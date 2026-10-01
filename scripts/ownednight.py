#!/usr/bin/env python3
"""Run the frozen owned-NNUE training or match queue on the authorized WSL host."""
from __future__ import annotations

import argparse
import datetime
import json
import os
import signal
import subprocess
import sys
import time
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent / 'candidate-match'))
from common import atomic_json, require_wsl, sha256


def capacity() -> float:
    disk = os.statvfs('/mnt/c')
    return disk.f_bavail * disk.f_frsize / (1 << 30)


def run_bounded(command: list[str], out: Path, deadline: float, env: dict, cpus: str = '0,2') -> None:
    atomic_json(out.with_suffix('.command.json'), {'command': command, 'cpus': cpus, 'started': time.time()})
    with out.with_suffix('.stdout').open('xb') as stdout, out.with_suffix('.stderr').open('xb') as stderr:
        process = subprocess.Popen(['taskset', '-c', cpus, *command], env=env, stdout=stdout, stderr=stderr, start_new_session=True)
        try:
            with out.with_suffix('.capacity.jsonl').open('x', buffering=1) as samples:
                while process.poll() is None:
                    free = capacity()
                    samples.write(json.dumps({'unix_time': time.time(), 'windows_free_gib': free}) + '\n')
                    if free < 25 or time.time() >= deadline:
                        raise RuntimeError('capacity/deadline guard stopped the job; partial work is unscored')
                    try:
                        process.wait(timeout=30)
                    except subprocess.TimeoutExpired:
                        pass
            if process.returncode:
                raise RuntimeError(f'job exited {process.returncode}; see {out.with_suffix(".stderr")}')
        finally:
            if process.poll() is None:
                os.killpg(process.pid, signal.SIGTERM)
                try:
                    process.wait(timeout=10)
                except subprocess.TimeoutExpired:
                    os.killpg(process.pid, signal.SIGKILL)
                    process.wait()


def training(root: Path, plan: dict, deadline: float) -> None:
    if capacity() < 35:
        raise RuntimeError('need 35 GiB physical C: headroom for the small-write queue')
    source = root / 'source'
    tool = Path('/home/ehrli/nnue-public-toolchain-20260906/install')
    env = dict(os.environ, PATH='/usr/bin:/bin', LD_LIBRARY_PATH=str(tool / 'cuda-12.8.1/lib64') + ':/usr/lib/wsl/lib')
    manifest = Path('/home/ehrli/nnue-owned-k4-20260920/archive-a1/corpus-a1/manifest.json')
    if sha256(manifest) != '5119735957a271252f1521b95999ee6b05ddb04be3e37161f2d6222643b97c50':
        raise RuntimeError('archive manifest differs')
    bridge = root / 'build/ngnk4bridge'
    build_env = dict(env, GOMAXPROCS='2', GOAMD64='v3', CGO_ENABLED='0', GOFLAGS='-mod=readonly', GOCACHE=str(root / 'gocache'))
    subprocess.run(['/usr/local/go/bin/go', 'build', '-trimpath', '-o', str(bridge), './cmd/ngnk4bridge'], cwd=source, env=build_env, check=True)
    fens = Path('/home/ehrli/nnue-owned-k4-20260920/night-20260927/k4-parity.fens')
    for spec in plan['training']:
        name = spec['name']
        out = root / 'training' / name
        if name == 'wdl40-e10':
            trainer = root / 'training/ngn-k4-wdl40'
            if sha256(trainer) != spec['trainer_sha256']:
                raise RuntimeError('WDL40 trainer differs')
            validation = json.loads((root / 'training/wdl40-smoke-report.json').read_text())
            if not validation['smoke_resume_pass'] or not validation['mismatched_weight_resume_rejected']:
                raise RuntimeError('WDL40 smoke/resume validation absent')
            command = [str(trainer), 'start', spec['mode'], str(manifest), str(out)]
        else:
            trainer = Path('/home/ehrli/nnue-owned-k4-20260920/night-20260927/bullet-e20/target/release/examples/ngn_k4_train')
            if sha256(trainer) != '6be85b6026b136332041ce6a4f5a2085ab0489f5b5c8e669311a38a953a77f26':
                raise RuntimeError('original e20 trainer differs')
            command = [str(trainer), 'resume', spec['mode'], str(manifest), spec['parent'], str(out)]
        atomic_json(root / 'training/STATE.json', {'state': 'TRAINING', 'name': name, 'started': time.time()})
        run_bounded(command, root / 'training' / (name + '-run'), min(deadline, time.time() + 14400), env)
        completion = json.loads((out / 'completion.json').read_text())
        if completion['completed_updates'] != spec['updates']:
            raise RuntimeError('training completion differs from frozen cap')
        checkpoint = out / 'candidates' / ('candidate-' + str(spec['updates']))
        network = root / 'training' / (name + '.nnue')
        run_bounded([str(bridge), 'convert', '-in', str(checkpoint / 'quantised.bin'), '-manifest', str(manifest), '-out', str(network)],
                    root / 'training' / (name + '-convert'), deadline, env)
        probe = root / 'training' / (name + '-probe.tsv')
        with probe.open('x') as stdout:
            subprocess.run(['taskset', '-c', '0,2', str(trainer), 'probe', str(checkpoint), str(fens)], env=env, stdout=stdout, check=True, timeout=120)
        run_bounded([str(bridge), 'parity', '-raw', str(checkpoint / 'raw.bin'), '-model', str(network), '-fens', str(fens), '-probe', str(probe)],
                    root / 'training' / (name + '-parity'), deadline, env)
        parity = json.loads((root / 'training' / (name + '-parity.stdout')).read_text())
        if not parity['pass']:
            raise RuntimeError('export parity failed')
        ready = {'state': 'COMPLETE', 'name': name, 'network': {'path': str(network), 'sha256': sha256(network)},
                 'completion_sha256': sha256(out / 'completion.json'), 'checkpoint_sha256': sha256(checkpoint / 'receipt.json'),
                 'parity_sha256': sha256(root / 'training' / (name + '-parity.stdout')), 'completed': time.time()}
        atomic_json(root / 'training' / (name + '-ready.json'), ready)
        print(json.dumps(ready), flush=True)
    atomic_json(root / 'training/STATE.json', {'state': 'COMPLETE', 'completed': time.time()})


def matches(root: Path, plan: dict, deadline: float) -> None:
    env = dict(os.environ, PATH='/usr/bin:/bin')
    def run(name: str, control: str | None = None) -> None:
        spec_path = root / 'specs' / (name + '.json')
        spec = json.loads(spec_path.read_text())
        if control:
            control_path = root / 'matches' / control / 'control.json'
            spec['control_report'] = {'path': str(control_path), 'sha256': sha256(control_path)}
        spec['limit_seconds'] = min(spec['limit_seconds'], int(deadline - time.time() - 1800))
        if spec['limit_seconds'] <= 0:
            raise RuntimeError('deadline leaves no time for another audited cell')
        atomic_json(spec_path, spec)
        atomic_json(root / 'matches/STATE.json', {'state': 'RUNNING', 'name': name, 'started': time.time()})
        command = ['python3', str(root / 'source/scripts/ownedmatch.py'), '--spec', str(spec_path), '--out', str(root / 'matches' / name)]
        run_bounded(command, root / 'matches' / (name + '-run'), deadline, env, '4,6,8,10,12,14')
        state = json.loads((root / 'matches' / name / 'STATE.json').read_text())
        if state['state'] != 'COMPLETE':
            raise RuntimeError('match has no accepted terminal')
        report = json.loads((root / 'matches' / name / 'report.json').read_text())
        print(json.dumps({'name': name, 'report': report}), flush=True)
    run('aa-short')
    run('reference', 'aa-short')
    ready_path = root / 'training/wdl40-e10-ready.json'
    while not ready_path.is_file():
        training_state = json.loads((root / 'training/STATE.json').read_text())
        if training_state['state'] == 'FAILED' or time.time() >= deadline - 1800:
            raise RuntimeError('WDL40 is unavailable for its frozen gate')
        # An hour-scale training job gets a native, silent ten-minute backoff.
        time.sleep(min(600, max(1, deadline - time.time() - 1800)))
    ready = json.loads(ready_path.read_text())
    if ready['state'] != 'COMPLETE':
        raise RuntimeError('WDL40 export is not eligible')
    spec = json.loads((root / 'specs/reference.json').read_text())
    baseline = spec['roles'][0]
    candidate = dict(baseline, name='Owned-WDL40', network=ready['network'])
    spec.update(roles=[candidate, baseline], pairs=800)
    opening = root / 'openings/wdl40.pgn'
    prefixes = root / 'openings/wdl40.txt'
    spec.update(openings_pgn={'path': str(opening), 'sha256': sha256(opening)},
                opening_prefixes={'path': str(prefixes), 'sha256': sha256(prefixes)})
    atomic_json(root / 'specs/wdl40.json', spec)
    run('wdl40', 'aa-short')
    report = json.loads((root / 'matches/wdl40/report.json').read_text())
    atomic_json(root / 'matches/wdl40-verdict.json', {'state': 'COMPLETE', 'decision': 'PROMOTE' if report['elo_interval_95'][0] > 0 else 'RETAIN_WDL25',
                'rule': 'positive paired 95% Elo lower bound at fixed 1600 games; no extension', 'report_sha256': sha256(root / 'matches/wdl40/report.json')})
    run('aa-long')
    run('external-long', 'aa-long')
    atomic_json(root / 'matches/STATE.json', {'state': 'COMPLETE', 'completed': time.time()})


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('queue', choices=['training', 'matches'])
    parser.add_argument('--root', type=Path, required=True)
    parser.add_argument('--plan', type=Path, required=True)
    args = parser.parse_args()
    require_wsl()
    plan = json.loads(args.plan.read_text())
    deadline = datetime.datetime.fromisoformat(plan['deadline_utc'].replace('Z', '+00:00')).timestamp()
    try:
        (training if args.queue == 'training' else matches)(args.root.resolve(), plan, deadline)
    except Exception as error:
        atomic_json(args.root / args.queue / 'STATE.json', {'state': 'FAILED', 'error': str(error), 'completed': time.time()})
        raise


if __name__ == '__main__':
    main()
