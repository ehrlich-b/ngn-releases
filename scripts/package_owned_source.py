#!/usr/bin/env python3
"""Create corresponding release source from an immutable Git commit."""
import argparse
import gzip
import hashlib
import io
import json
import re
import subprocess
import tarfile
from pathlib import Path


ROOTS = ('cmd/', 'countereval/', 'engine/', 'internal/', 'nnue/', 'rodenteval/',
         'rodentv12eval/', 'training/', 'scripts/candidate-match/')
FILES = {'main.go', 'main_test.go', 'go.mod', 'Makefile',
         'docs/RELEASING.md', 'docs/THIRD_PARTY.md',
         'scripts/build_owned_release.py', 'scripts/verify_owned_release.py',
         'scripts/package_owned_release.py', 'scripts/package_owned_source.py',
         'scripts/k4identity.py'}
RECEIPTS = ('build', 'source-tests', 'provenance', 'executables')
OPTIONAL_FILES = {'go.sum'}
FILES.update('experiments/2026-09-29-owned-release-' + name + '.json' for name in RECEIPTS)
FILES.update(('experiments/2026-09-29-owned-dataset-license.json',
              'experiments/2026-09-29-owned-training-method.json'))


def git(repo, *args):
    return subprocess.check_output(['git', '-C', str(repo), *args])


def package(args):
    if not re.fullmatch('[0-9a-f]{40}', args.source_commit):
        raise ValueError('source commit must be an immutable 40-character hash')
    build = json.loads(args.build_manifest.read_text())
    version = build['version']
    if not re.fullmatch(r'[0-9]+\.[0-9]+\.[0-9]+(?:-[A-Za-z0-9.-]+)?', version):
        raise ValueError('invalid version')
    tree = git(args.repo, 'ls-tree', '-rz', '--full-tree', args.source_commit)
    files = {}
    for row in tree.split(b'\0'):
        if not row:
            continue
        header, raw_name = row.split(b'\t', 1)
        mode, kind, oid = header.decode().split()
        name = raw_name.decode()
        if kind != 'blob' or not (name in FILES or name in OPTIONAL_FILES
                                 or name in build['source_files'] or name.startswith(ROOTS)):
            continue
        if name.endswith(('.prof', '.out')) or name == 'cmd/tactical-test/tactical-test':
            continue
        if mode not in ('100644', '100755'):
            raise ValueError('unsupported source entry: ' + name)
        files[name] = (git(args.repo, 'cat-file', 'blob', oid), 0o755 if mode == '100755' else 0o644)
    for name in FILES:
        if name not in files:
            raise ValueError('required corresponding source or receipt missing: ' + name)
    for name, expected in build['source_files'].items():
        if name not in files or hashlib.sha256(files[name][0]).hexdigest() != expected:
            raise ValueError('application source differs from verified build: ' + name)
    if args.approved_distribution:
        for name in ('LICENSE', 'LICENSE-GRANT.txt', 'README.md'):
            files[name] = (git(args.repo, 'show', args.source_commit + ':' + name), 0o644)
        if b'GPL-3.0-only' not in files['LICENSE-GRANT.txt'][0]:
            raise ValueError('approved GPL grant is missing')
    for notice in sorted(args.notices.iterdir()):
        if notice.is_file():
            files['LICENSES/' + notice.name] = (notice.read_bytes(), 0o644)
    files['source-info.json'] = ((json.dumps({
        'version': version, 'archive_source_commit': args.source_commit,
        'application_source_commit': build['source_commit'],
        'build_manifest_sha256': hashlib.sha256(args.build_manifest.read_bytes()).hexdigest(),
        'application_source_files_verified': len(build['source_files']),
        'publication_state': 'APPROVED_RELEASE_CANDIDATE' if args.approved_distribution else 'DRAFT_PENDING_LICENSE_AND_APPROVAL',
        'project_license': 'GPL-3.0-only' if args.approved_distribution else 'NOASSERTION',
        'model_license': 'GPL-3.0-only' if args.approved_distribution else 'NOASSERTION',
    }, indent=2) + '\n').encode(), 0o644)
    readme = (
        f'NGN {version} corresponding source\n\n'
        'This archive contains the application source that produced the verified binaries,\n'
        'release tooling, compact verification receipts, and applicable third-party notices.\n'
        'The complete training alteration method and exact filter/map inputs are a separate asset.\n'
        'No borrowed neural-network weights are included.\n\n'
        'The application was built on WSL with Go 1.25.5, CGO_ENABLED=0 and GOAMD64=v3:\n'
        'go build -trimpath -buildvcs=false -ldflags="-s -w -X main.releaseProfile=owned '
        f'-X main.releaseVersion={version}" -o ngn .\n'
        'Set GOOS=windows and GOARCH=amd64 to reproduce the Windows build.\n'
        'Place the selected ngn.nnue next to the executable. Consult docs/RELEASING.md.\n\n'
        'This prepared candidate is pending the project/model license decision and release approval.\n'
    )
    files['README.txt'] = (readme.encode(), 0o644)
    if args.approved_distribution:
        files['README.txt'] = files['README.md']
    stamp = int(git(args.repo, 'show', '-s', '--format=%ct', args.source_commit))
    prefix = 'ngn-' + version + '-source'
    args.out.parent.mkdir(parents=True, exist_ok=True)
    with args.out.open('xb') as raw, gzip.GzipFile(fileobj=raw, mode='wb', filename='', mtime=0) as compressed:
        with tarfile.open(fileobj=compressed, mode='w') as archive:
            for name, (data, mode) in sorted(files.items()):
                member = tarfile.TarInfo(prefix + '/' + name)
                member.size = len(data); member.mode = mode; member.mtime = stamp
                member.uid = 0; member.gid = 0
                archive.addfile(member, io.BytesIO(data))
    with tarfile.open(args.out, 'r:gz') as archive:
        actual = {member.name.split('/', 1)[1]: hashlib.sha256(archive.extractfile(member).read()).hexdigest()
                  for member in archive.getmembers() if member.isfile()}
    expected = {name: hashlib.sha256(data).hexdigest() for name, (data, mode) in files.items()}
    if actual != expected:
        raise ValueError('corresponding-source archive readback differs')
    print(json.dumps({'pass': True, 'archive_source_commit': args.source_commit,
                      'application_source_commit': build['source_commit'],
                      'source_files_verified': len(build['source_files']), 'member_hashes_verified': len(expected),
                      'bytes': args.out.stat().st_size, 'sha256': hashlib.sha256(args.out.read_bytes()).hexdigest()}))


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--source-commit', required=True)
    parser.add_argument('--approved-distribution', action='store_true', help='Package the explicitly approved GPL release grant and README.')
    for name in ('repo', 'build-manifest', 'notices', 'out'):
        parser.add_argument('--' + name, type=Path, required=True)
    package(parser.parse_args())
