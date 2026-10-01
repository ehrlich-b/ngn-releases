#!/usr/bin/env python3
"""Create a separate, hash-bound owned-NNUE candidate bundle on WSL."""
from __future__ import annotations

import argparse
import hashlib
import json
import shutil
import sys
from pathlib import Path


def digest(path: Path) -> str:
    result = hashlib.sha256()
    with path.open('rb') as handle:
        for block in iter(lambda: handle.read(1 << 20), b''):
            result.update(block)
    return result.hexdigest()


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--linux', type=Path, required=True)
    parser.add_argument('--windows', type=Path, required=True)
    parser.add_argument('--network', type=Path, required=True)
    parser.add_argument('--verification', type=Path, required=True)
    parser.add_argument('--identity', type=Path, required=True)
    parser.add_argument('--source-commit', required=True)
    parser.add_argument('--out', type=Path, required=True)
    args = parser.parse_args()
    if not sys.platform.startswith('linux') or 'microsoft' not in Path('/proc/sys/kernel/osrelease').read_text().lower():
        parser.error('run on the authorized WSL build host')
    verification = json.loads(args.verification.read_text())
    identity = json.loads(args.identity.read_text())
    if not all(step['returncode'] == 0 for step in verification['steps']):
        parser.error('verification did not pass')
    if not identity['identical'] or identity['scale_percent'] != 60 or identity['fixtures'] < 80:
        parser.error('configured-60 search identity did not pass')
    inputs = {'ngn': args.linux, 'ngn.exe': args.windows, 'wdl25-e10.nnue': args.network}
    for name, expected in [('ngn', verification['builds']['ngn']), ('ngn.exe', verification['builds']['ngn.exe']),
                           ('wdl25-e10.nnue', identity['sha256']['network'])]:
        if digest(inputs[name]) != expected:
            parser.error(f'input hash differs: {name}')
    if digest(args.linux) != identity['sha256']['cand']:
        parser.error('Linux binary differs from the identity candidate')
    out = args.out.resolve()
    out.mkdir(parents=True, exist_ok=False)
    for name, path in inputs.items():
        shutil.copy2(path, out / name)
    linux_launcher = '''#!/bin/sh
set -eu
bundle_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
exec env GOMAXPROCS="${GOMAXPROCS:-1}" "$bundle_dir/ngn" \\
  -eval-backend ngn-k4-768-v1 -eval-file "$bundle_dir/wdl25-e10.nnue" \\
  -k4-eval-scale 60 -own-book=false "$@"
'''
    (out / 'ngn-owned').write_text(linux_launcher)
    (out / 'ngn-owned').chmod(0o555)
    windows_launcher = '''@echo off
setlocal
if not defined GOMAXPROCS set "GOMAXPROCS=1"
"%~dp0ngn.exe" -eval-backend ngn-k4-768-v1 -eval-file "%~dp0wdl25-e10.nnue" -k4-eval-scale 60 -own-book=false %*
'''
    (out / 'ngn-owned.cmd').write_bytes(windows_launcher.replace('\n', '\r\n').encode())
    (out / 'README.txt').write_text(
        'NGN owned NNUE candidate — September 28, 2026\n\n'
        'Launch ngn-owned on Linux, or ngn-owned.cmd on Windows. Keep all bundle files together.\n'
        'The launchers select WDL25-e10, K4EvalScale 60 and OwnBook false. UCI advertises these defaults.\n'
        'Hash defaults to 128 MiB, Threads to 1, Move Overhead to 100 ms, and GOMAXPROCS to 1.\n'
        'An explicit GOMAXPROCS environment value is preserved; Threads is configured through UCI.\n'
        'The playing evidence is for one worker. Width/clock transfer must be measured separately.\n'
        'Both binaries are GOAMD64=v3 builds requiring the corresponding x86-64 instruction set.\n'
        'This is a separate research candidate; the historical installs and ordinary HCE startup are held.\n'
        'The net has +34.9 Elo [+20.0,+49.4] versus borrowed Rodent V1.2 inside NGN at 10+0.1.\n'
        'This is a relative match result, not an absolute rating or broad competitor ranking.\n')
    files = {path.name: {'bytes': path.stat().st_size, 'sha256': digest(path)} for path in out.iterdir()}
    manifest = {'schema': 'ngn-owned-candidate-bundle-v1', 'source_commit': args.source_commit,
                'go_version': verification['go_version'], 'goamd64': 'v3', 'cgo_enabled': 0,
                'startup': {'backend': 'ngn-k4-768-v1', 'scale_percent': 60, 'own_book': False,
                            'threads': 1, 'hash_mb': 128, 'move_overhead_ms': 100, 'gomaxprocs_default': 1},
                'verification_sha256': digest(args.verification), 'identity_sha256': digest(args.identity), 'files': files}
    (out / 'manifest.json').write_text(json.dumps(manifest, indent=2) + '\n')
    (out / 'SHA256SUMS').write_text(''.join(f'{digest(path)}  {path.name}\n' for path in sorted(out.iterdir())))
    print(json.dumps({'bundle': str(out), 'files': len(files), 'manifest_sha256': digest(out / 'manifest.json')}))


if __name__ == '__main__':
    main()
