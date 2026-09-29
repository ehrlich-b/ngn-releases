#!/usr/bin/env python3
"""Verify actual owned-profile Linux and native Windows executables on WSL."""
from __future__ import annotations

import argparse
import base64
import json
import os
import shutil
import subprocess
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent / 'candidate-match'))
from common import atomic_json, require_wsl, safe_environment, sha256
from uci_preflight import Protocol, parse_advertised_options


def windows_command(binary: Path, version_only: bool = False) -> list[str]:
    path = subprocess.check_output(['wslpath', '-w', str(binary)], text=True).strip().replace("'", "''")
    script = "$ErrorActionPreference='Stop'; $ProgressPreference='SilentlyContinue'; $env:GOMAXPROCS=$null; (Get-Process -Id $PID).ProcessorAffinity=15; & '" + path + "'" + (' -version' if version_only else '') + '; exit $LASTEXITCODE'
    return ['/mnt/c/Windows/System32/WindowsPowerShell/v1.0/powershell.exe', '-NoProfile', '-NonInteractive',
            '-OutputFormat', 'Text', '-EncodedCommand', base64.b64encode(script.encode('utf-16le')).decode()]


def search_result(lines: list[str]) -> dict:
    fields = next(line.split() for line in reversed(lines) if line.startswith('info depth ') and ' nodes ' in line)
    return {'depth': int(fields[fields.index('depth') + 1]), 'nodes': int(fields[fields.index('nodes') + 1]),
            'score': ' '.join(fields[fields.index('score') + 1:fields.index('score') + 3]),
            'bestmove': lines[-1].split()[1]}


def verify(root: Path, build: Path, network: Path, out: Path) -> None:
    require_wsl()
    manifest = json.loads((build / 'build-manifest.json').read_text())
    for name, artifact in manifest['builds'].items():
        if sha256(build / name) != artifact['sha256']:
            raise RuntimeError('release binary differs from build manifest')
    out.mkdir(exist_ok=False)
    linux = out / 'linux owned'
    windows = Path('/mnt/c/Users/ehrli/AppData/Local/Temp') / ('ngn owned ' + out.name)
    linux.mkdir(); windows.mkdir(exist_ok=False)
    for directory, name in [(linux, 'ngn'), (windows, 'ngn.exe')]:
        shutil.copy2(build / name, directory / name)
        shutil.copy2(network, directory / 'ngn.nnue')
        if sha256(directory / 'ngn.nnue') != sha256(network):
            raise RuntimeError('QA network copy differs')
    outside = out / 'outside'; outside.mkdir()
    win_outside = windows / 'outside'; win_outside.mkdir()
    env = safe_environment(); env.pop('GOMAXPROCS', None)
    roles = [('linux', linux, 'ngn', ['taskset', '-c', '0,1,2,3', str(linux / 'ngn')], outside),
             ('windows', windows, 'ngn.exe', windows_command(windows / 'ngn.exe'), win_outside)]
    legal = {f'{file}2{file}{rank}' for file in 'abcdefgh' for rank in '34'} | {'b1a3', 'b1c3', 'g1f3', 'g1h3'}
    reports = []
    for platform, directory, name, command, cwd in roles:
        trace = out / (platform + '.jsonl')
        with trace.open('x', buffering=1) as transcript:
            process = subprocess.Popen(command, cwd=cwd, env=env, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
            try:
                protocol = Protocol(process, transcript)
                protocol.send('uci'); lines = protocol.until(lambda line: line == 'uciok', 30, 'uci')
                options = parse_advertised_options(lines)
                assert 'id name ngn ' + manifest['version'] in lines, lines[:3]
                for option, value in [('Threads', 1), ('Hash', 128), ('Move Overhead', 100), ('OwnBook', False),
                                      ('K4EvalScale', 60), ('EvalBackend', 'ngn-k4-768-v1')]:
                    assert options[option]['default'] == value, (option, options[option])
                model_path = options['EvalFile']['default']
                if platform == 'windows':
                    model_path = subprocess.check_output(['wslpath', '-u', model_path], text=True).strip()
                assert Path(model_path).resolve() == (directory / 'ngn.nnue').resolve()
                protocol.send('isready'); protocol.until(lambda line: line == 'readyok', 30, 'ready')
                protocol.send('position startpos'); protocol.send('go depth 8')
                first = search_result(protocol.until(lambda line: line.startswith('bestmove '), 30, 'default search'))
                assert first['depth'] == 8 and first['bestmove'] in legal
                protocol.send('debug on'); protocol.send('isready'); protocol.until(lambda line: line == 'readyok', 30, 'debug')
                for option, value in [('Threads', 8), ('Hash', 32)]:
                    protocol.send(f'setoption name {option} value {value}'); protocol.send('isready')
                    lines = protocol.until(lambda line: line == 'readyok', 30, option)
                    assert f'info string option set: {option} = {value}' in lines
                searches = []
                for label, go in [('nodes', 'go nodes 80000'), ('stop', 'go infinite'), ('restart', 'go nodes 40000')]:
                    protocol.send('ucinewgame'); protocol.send('isready'); protocol.until(lambda line: line == 'readyok', 30, 'newgame')
                    protocol.send('position startpos'); protocol.send(go)
                    prefix = []
                    if label == 'stop':
                        prefix = protocol.until(lambda line: line.startswith('info depth 4 '), 30, 'live search'); protocol.send('stop')
                    result = search_result(prefix + protocol.until(lambda line: line.startswith('bestmove '), 30, label))
                    assert result['bestmove'] in legal
                    searches.append(dict(case=label, **result))
                for option, value in [('Threads', 1), ('K4EvalScale', 100), ('K4EvalScale', 60), ('Threads', 8)]:
                    protocol.send(f'setoption name {option} value {value}'); protocol.send('isready')
                    lines = protocol.until(lambda line: line == 'readyok', 30, option)
                    assert f'info string option set: {option} = {value}' in lines
                protocol.send('position startpos'); protocol.send('go nodes 40000')
                result = search_result(protocol.until(lambda line: line.startswith('bestmove '), 30, 'reconfigured'))
                assert result['bestmove'] in legal
                searches.append(dict(case='reconfigured', **result))
                protocol.send('quit'); assert process.wait(timeout=30) == 0
                for thread in protocol.threads:
                    thread.join(timeout=5); assert not thread.is_alive()
                assert not protocol.stderr_lines
                rows = [json.loads(line) for line in trace.read_text().splitlines()]
                assert sum(row['direction'] == 'engine-stdout' and row['line'].startswith('bestmove ') for row in rows) == 5
            finally:
                if process.poll() is None:
                    process.kill(); process.wait()
        missing = directory / 'missing network'; missing.mkdir(); shutil.copy2(directory / name, missing / name)
        fail_command = windows_command(missing / name) if platform == 'windows' else ['taskset', '-c', '0,2', str(missing / name)]
        failure = subprocess.run(fail_command, cwd=cwd, env=env, input=b'uci\nquit\n', capture_output=True, timeout=30)
        assert failure.returncode == 2 and not failure.stdout and b'ngn: evaluator startup configuration failed:' in failure.stderr
        version_command = windows_command(missing / name, True) if platform == 'windows' else fail_command + ['-version']
        version = subprocess.run(version_command, cwd=cwd, env=env, capture_output=True, timeout=30)
        assert version.returncode == 0 and version.stdout.decode().strip() == manifest['version'] and not version.stderr
        reports.append({'platform': platform, 'pass': True, 'outside_cwd': str(cwd), 'defaults': options,
                        'default_search': first, 'width8_searches': searches, 'missing_network_exit': failure.returncode,
                        'version_without_network': True, 'transcript_sha256': sha256(trace), 'binary_sha256': sha256(directory / name)})
    assert reports[0]['default_search'] == reports[1]['default_search'], 'Linux/Windows default search differs'
    atomic_json(out / 'startup.json', {'schema': 'ngn-owned-release-startup-v1', 'pass': True, 'version': manifest['version'],
                'network_sha256': sha256(network), 'gomaxprocs_environment': None, 'linux_affinity': [0, 1, 2, 3],
                'windows_parent_affinity': [0, 1, 2, 3], 'claim': 'startup and lifecycle verification; no SMP Elo claim', 'reports': reports})
    night = Path('/home/ehrli/nnue-owned-k4-20260920/night-20260927')
    command = ['python3', str(root / 'source/scripts/k4identity.py'), '--base', str(root / 'build/ngn'),
               '--cand', str(build / 'ngn'), '--network', str(network), '--pgn', str(night / 'm11/games.pgn'),
               '--cpu', '0', '--depth', '12', '--games', '30', '--plies', '16,40,70', '--rounds', '3',
               '--scale', '60', '--out', str(out / 'identity.json')]
    # This isolated executable copy gives the profile its adjacent startup model.
    shutil.copy2(network, build / 'ngn.nnue')
    run = subprocess.run(command, capture_output=True, text=True, timeout=600)
    (out / 'identity.stdout').write_text(run.stdout); (out / 'identity.stderr').write_text(run.stderr)
    assert run.returncode == 0, run.stderr
    identity = json.loads((out / 'identity.json').read_text())
    assert identity['identical'] and identity['fixtures'] == 88
    atomic_json(out / 'verification.json', {'pass': True, 'version': manifest['version'], 'build_manifest_sha256': sha256(build / 'build-manifest.json'),
                'startup_sha256': sha256(out / 'startup.json'), 'identity_sha256': sha256(out / 'identity.json'),
                'network_sha256': sha256(network), 'identity': identity, 'startup': json.loads((out / 'startup.json').read_text())})
    print(json.dumps({'pass': True, 'version': manifest['version'], 'fixtures': identity['fixtures'], 'identical': identity['identical'],
                      'speedup': identity['speedup'], 'platforms': [row['platform'] for row in reports], 'out': str(out)}))


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ['root', 'build', 'network', 'out']:
        parser.add_argument('--' + name, type=Path, required=True)
    args = parser.parse_args()
    verify(args.root.resolve(), args.build.resolve(), args.network.resolve(), args.out.resolve())
