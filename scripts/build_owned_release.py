#!/usr/bin/env python3
"""Build hash-bound owned-profile executables on the authorized WSL host."""
import argparse
import hashlib
import json
import os
import re
import subprocess
import sys
from pathlib import Path


def digest(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--version',required=True);parser.add_argument('--source-commit',required=True)
    parser.add_argument('--verification',type=Path,required=True);parser.add_argument('--out',type=Path,required=True)
    args=parser.parse_args()
    if not sys.platform.startswith('linux') or 'microsoft' not in Path('/proc/sys/kernel/osrelease').read_text().lower():
        parser.error('build on the authorized WSL host')
    if not re.fullmatch(r'[0-9]+\.[0-9]+\.[0-9]+(?:-[A-Za-z0-9.-]+)?',args.version) or not re.fullmatch(r'[0-9a-f]{40}',args.source_commit):
        parser.error('invalid release version/source identity')
    verification=json.loads(args.verification.read_text())
    steps={row['name']:row for row in verification['steps']}
    if any(name not in steps or steps[name]['returncode']!=0 for name in ['focused','short','race','oracle']):
        parser.error('required verification is incomplete')
    source=Path(__file__).resolve().parent.parent;out=args.out.resolve();out.mkdir(parents=True,exist_ok=False)
    source_files={str(path.relative_to(source)):digest(path) for path in source.rglob('*')
                  if path.is_file() and (path.suffix in ['.go','.s','.h','.c','.cpp'] or path.name in ['go.mod','go.sum'])
                  and not any(part in ['.git','output','build','vendor'] for part in path.relative_to(source).parts)}
    env=dict(os.environ,PATH='/usr/local/go/bin:/usr/bin:/bin',CGO_ENABLED='0',GOARCH='amd64',GOAMD64='v3',
             GOFLAGS='-mod=readonly',GOMAXPROCS='2')
    flags=f'-s -w -X main.releaseProfile=owned -X main.releaseVersion={args.version}'
    builds={}
    for platform,name in [('linux','ngn'),('windows','ngn.exe')]:
        env['GOOS']=platform
        command=['go','build','-trimpath','-buildvcs=false','-ldflags',flags,'-o',str(out/name),'.']
        subprocess.run(['taskset','-c','0,2',*command],cwd=source,env=env,check=True,timeout=600)
        builds[name]={'sha256':digest(out/name),'bytes':(out/name).stat().st_size,'command':command}
    manifest={'schema':'ngn-owned-release-build-v1','version':args.version,'source_commit':args.source_commit,
              'source_files':source_files,'go_version':subprocess.check_output(['go','version'],env=env,text=True).strip(),
              'goamd64':'v3','cgo_enabled':0,'profile':'owned','network_filename':'ngn.nnue',
              'startup':{'backend':'ngn-k4-768-v1','scale_percent':60,'own_book':False,'threads':1,'hash_mb':128,'move_overhead_ms':100,
                         'scheduler':'follow UCI Threads unless GOMAXPROCS is explicitly set'},
              'verification_sha256':digest(args.verification),'builds':builds}
    (out/'build-manifest.json').write_text(json.dumps(manifest,indent=2)+'\n')
    print(json.dumps({'out':str(out),'version':args.version,'builds':builds,'manifest_sha256':digest(out/'build-manifest.json')}))


if __name__=='__main__':main()
