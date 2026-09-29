#!/usr/bin/env python3
"""Assemble and read back verified owned NNUE release-candidate archives."""
import argparse
import hashlib
import json
import re
import shutil
import tarfile
import zipfile
from pathlib import Path


def digest(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def package(args):
    build = json.loads(args.build_manifest.read_text())
    verify = json.loads(args.verification.read_text())
    if not re.fullmatch(r'[0-9]+\.[0-9]+\.[0-9]+(?:-[A-Za-z0-9.-]+)?', build['version']):
        raise ValueError('invalid version')
    if not verify['pass'] or verify['version'] != build['version'] or verify['build_manifest_sha256'] != digest(args.build_manifest):
        raise ValueError('executable verification is not bound to this build')
    network_sha = digest(args.network)
    if verify['network_sha256'] != network_sha or verify['identity']['sha256']['network'] != network_sha:
        raise ValueError('network differs from executable verification')
    if not verify['identity']['identical'] or verify['identity']['fixtures'] != 88 or verify['identity']['scale_percent'] != 60:
        raise ValueError('required configured search identity failed')
    binaries = {'linux': args.linux, 'windows': args.windows}
    names = {'linux': 'ngn', 'windows': 'ngn.exe'}
    reports = {row['platform']: row for row in verify['startup']['reports']}
    for platform, path in binaries.items():
        expected = build['builds'][names[platform]]['sha256']
        if digest(path) != expected or not reports[platform]['pass'] or reports[platform]['binary_sha256'] != expected:
            raise ValueError('platform executable differs or did not pass')
    with tarfile.open(args.source, 'r:*') as archive:
        files = {member.name.split('/', 1)[-1]: member for member in archive.getmembers() if member.isfile()}
        for relative, expected in build['source_files'].items():
            if relative not in files or hashlib.sha256(archive.extractfile(files[relative]).read()).hexdigest() != expected:
                raise ValueError('corresponding source differs: ' + relative)
        if args.approved_distribution:
            for name in ['LICENSE', 'LICENSE-GRANT.txt', 'README.md']:
                if name not in files or archive.extractfile(files[name]).read() != (args.approved_distribution / name).read_bytes():
                    raise ValueError('approved source notice differs: ' + name)
    method = json.loads(args.training_method_receipt.read_text())
    if digest(args.training_method) != method['sha256'] or method['recipe']['network_sha256'] != network_sha:
        raise ValueError('training method differs from the selected network receipt')
    with tarfile.open(args.training_method, 'r:*') as archive:
        files = {member.name.split('/', 1)[-1]: member for member in archive.getmembers() if member.isfile()}
        for relative, expected in method['recipe']['files'].items():
            if relative not in files:
                raise ValueError('training-method input missing: ' + relative)
            data = archive.extractfile(files[relative]).read()
            if len(data) != expected['bytes'] or hashlib.sha256(data).hexdigest() != expected['sha256']:
                raise ValueError('training-method input differs: ' + relative)
    notices = {path.name: path for path in args.notices.iterdir() if path.is_file()}
    for name in ['Zahak-MIT.txt', 'Go-LICENSE.txt', 'CounterGo-GPL.txt', 'Rodent-V-GPL.txt', 'Stockfish-GPL.txt', 'TRAINING-DATA.txt']:
        if name not in notices:
            raise ValueError('required attribution missing: ' + name)
    out = args.out.resolve(); out.mkdir(exist_ok=False, parents=True)
    version = build['version']
    shutil.copy2(args.source, out / ('ngn-' + version + '-source.tar.gz'))
    shutil.copy2(args.training_method, out / ('ngn-' + version + '-training-method.tar.gz'))
    readme = (
        f'NGN {version} — owned NNUE release candidate\n\n'
        'Install ngn.exe in a Windows UCI chess GUI, or ngn on Linux. Keep ngn.nnue beside it.\n'
        'The executable starts with its owned network automatically, including from another working directory.\n'
        'Set Threads and Hash in your GUI. Defaults: Threads 1, Hash 128 MiB, OwnBook false,\n'
        'K4EvalScale 60 and Move Overhead 100 ms. Pondering is unsupported.\n'
        'Threads controls Go scheduling automatically; an explicit GOMAXPROCS setting takes precedence.\n'
        'These amd64-v3 binaries require the corresponding x86-64 CPU features.\n'
        f'Network: {args.network_name}\nSHA-256: {network_sha}\n\n'
        'The playing measurements are for one worker. Threads=8 lifecycle checks establish reliability,\n'
        'and do not establish an SMP Elo gain. No official CCRL rating or >3500 certification is claimed.\n'
        'Source, the complete training alteration method and its inputs, training-data attribution,\n'
        'applicable notices and integrity records accompany this candidate.\n'
        'This prepared candidate is pending the project/model license decision and release approval.\n'
    )
    archives = []
    for platform, path in binaries.items():
        folder_name = 'ngn-' + version + '-' + platform + '-amd64-v3'; folder = out / folder_name; folder.mkdir()
        shutil.copy2(path, folder / names[platform]); shutil.copy2(args.network, folder / 'ngn.nnue')
        if platform == 'linux':
            (folder / 'ngn').chmod(0o755)
        if args.approved_distribution:
            for name in ['README.md', 'LICENSE', 'LICENSE-GRANT.txt']:
                shutil.copy2(args.approved_distribution / name, folder / name)
        else:
            (folder / 'README.txt').write_text(readme)
        for name, notice in notices.items():
            shutil.copy2(notice, folder / name)
        files = {p.name: {'bytes': p.stat().st_size, 'sha256': digest(p)} for p in folder.iterdir()}
        manifest = {'schema': 'ngn-owned-release-package-v1', 'version': version, 'platform': platform,
                    'application_source_commit': build['source_commit'], 'corresponding_source_sha256': digest(args.source),
                    'training_method_sha256': method['sha256'],
                    'build_manifest_sha256': digest(args.build_manifest), 'verification_sha256': digest(args.verification),
                    'network_name': args.network_name, 'startup': build['startup'], 'files': files,
                    'publication_state': 'APPROVED_RELEASE_CANDIDATE' if args.approved_distribution else 'DRAFT_PENDING_LICENSE_AND_APPROVAL',
                    'project_license': 'GPL-3.0-only' if args.approved_distribution else 'NOASSERTION',
                    'model_license': 'GPL-3.0-only' if args.approved_distribution else 'NOASSERTION', 'training_archive_license': 'ODbL-1.0'}
        (folder / 'manifest.json').write_text(json.dumps(manifest, indent=2) + '\n')
        (folder / 'SHA256SUMS').write_text(''.join(f'{digest(p)}  {p.name}\n' for p in sorted(folder.iterdir())))
        expected = {p.name: digest(p) for p in folder.iterdir()}
        if platform == 'windows':
            asset = out / (folder_name + '.zip')
            with zipfile.ZipFile(asset, 'w', zipfile.ZIP_DEFLATED) as archive:
                for p in sorted(folder.iterdir()):
                    archive.write(p, folder_name + '/' + p.name)
            with zipfile.ZipFile(asset) as archive:
                actual = {Path(name).name: hashlib.sha256(archive.read(name)).hexdigest() for name in archive.namelist()}
        else:
            asset = out / (folder_name + '.tar.gz')
            with tarfile.open(asset, 'w:gz') as archive:
                archive.add(folder, arcname=folder_name)
            with tarfile.open(asset, 'r:gz') as archive:
                actual = {Path(member.name).name: hashlib.sha256(archive.extractfile(member).read()).hexdigest()
                          for member in archive.getmembers() if member.isfile()}
        if actual != expected:
            raise ValueError('archive readback differs')
        archives.append({'file': asset.name, 'bytes': asset.stat().st_size, 'sha256': digest(asset), 'member_hashes_verified': len(expected)})
    receipt = {'schema': 'ngn-owned-release-assets-v1', 'version': version, 'pass': True, 'source_files_verified': len(build['source_files']),
               'network_sha256': network_sha, 'source_asset': {'file': 'ngn-' + version + '-source.tar.gz', 'sha256': digest(args.source)},
               'training_method_asset': {'file': 'ngn-' + version + '-training-method.tar.gz', 'bytes': args.training_method.stat().st_size,
                                         'sha256': method['sha256'], 'member_hashes_verified': len(method['recipe']['files'])},
               'assets': archives, 'publication_state': 'APPROVED_RELEASE_CANDIDATE' if args.approved_distribution else 'DRAFT_PENDING_LICENSE_AND_APPROVAL',
               'project_license': 'GPL-3.0-only' if args.approved_distribution else 'NOASSERTION',
               'model_license': 'GPL-3.0-only' if args.approved_distribution else 'NOASSERTION'}
    (out / 'assets.json').write_text(json.dumps(receipt, indent=2) + '\n')
    (out / 'SHA256SUMS').write_text(''.join(f'{digest(p)}  {p.name}\n' for p in sorted(out.iterdir()) if p.is_file()))
    print(json.dumps(receipt))


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ['build-manifest', 'verification', 'linux', 'windows', 'network', 'source', 'notices', 'out',
                 'training-method', 'training-method-receipt']:
        parser.add_argument('--' + name, type=Path, required=True)
    parser.add_argument('--network-name', required=True)
    parser.add_argument('--approved-distribution', type=Path, help='Directory containing the explicitly approved LICENSE, LICENSE-GRANT.txt and README.md.')
    package(parser.parse_args())
