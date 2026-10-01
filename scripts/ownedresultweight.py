#!/usr/bin/env python3
"""Build, smoke and train the separately frozen WDL15 candidate on WSL."""
from __future__ import annotations

import argparse
import datetime
import json
import os
import shutil
import subprocess
import sys
import time
from pathlib import Path

import ownednight

sys.path.insert(0, str(Path(__file__).resolve().parent / 'candidate-match'))
from common import atomic_json, require_wsl, sha256


def training(root: Path, plan_path: Path) -> None:
    require_wsl()
    plan = json.loads(plan_path.read_text())
    out = root / 'wdl15'; out.mkdir(exist_ok=False)
    atomic_json(out / 'STATE.json', {'state': 'BUILDING', 'plan_sha256': sha256(plan_path), 'started': time.time()})
    try:
        if ownednight.capacity() < plan['admission_windows_free_gib']:
            raise RuntimeError('insufficient physical Windows disk headroom')
        deadline = datetime.datetime.fromisoformat(plan['training_deadline_utc'].replace('Z', '+00:00')).timestamp()
        if deadline - time.time() < 14400:
            raise RuntimeError('insufficient time for the full training cap')
        master = root / 'openings/master.txt'
        if sha256(master) != plan['opening_master_sha256'] or len(master.read_text().splitlines()) < plan['matches'][-1]['opening_lines'][-1]:
            raise RuntimeError('frozen opening source differs or is too short')
        source = root / 'source/training/nnue/bullet/ngn_k4_train.rs'
        if sha256(source) != plan['harness_sha256']:
            raise RuntimeError('training harness differs')
        old = Path('/home/ehrli/nnue-owned-k4-20260920/night-20260927/bullet-wdl25')
        identity = dict(line.split('=', 1) for line in (old / 'NGN_K4_EXECUTION_IDENTITY').read_text().splitlines())
        if identity['bullet_commit'] != plan['bullet_commit'] or identity['patch_sha256'] != plan['bullet_patch_sha256']:
            raise RuntimeError('pinned Bullet source differs')
        if sha256(old / 'crates/bullet_lib/examples/ngn_k4_train.rs') != plan['parent_harness_sha256']:
            raise RuntimeError('preserved parent training harness differs')
        build = out / 'bullet'
        shutil.copytree(old, build, ignore=shutil.ignore_patterns('target', '.git'))
        shutil.copy2(source, build / 'crates/bullet_lib/examples/ngn_k4_train.rs')
        tool = Path('/home/ehrli/nnue-public-toolchain-20260906/install')
        env = dict(os.environ, PATH=str(tool / 'rust-1.88.0/bin') + ':' + str(tool / 'cuda-12.8.1/bin') + ':/usr/bin:/bin',
                   CUDA_PATH=str(tool / 'cuda-12.8.1'), LD_LIBRARY_PATH=str(tool / 'cuda-12.8.1/lib64') + ':/usr/lib/wsl/lib',
                   CARGO_HOME='/home/ehrli/nnue-owned-k4-20260920/night-20260927/cargo-home-k8', CARGO_BUILD_JOBS='2', NGN_WDL15='1')
        for key in ['NGN_WDL25', 'NGN_WDL40', 'NGN_K8', 'NGN_H1024', 'NGN_W1024']:
            env.pop(key, None)
        command = [str(tool / 'rust-1.88.0/bin/cargo'), 'build', '--release', '--offline', '-p', 'bullet_lib', '--features', 'cuda', '--example', 'ngn_k4_train']
        receipt = {'plan_sha256': sha256(plan_path), 'harness_sha256': sha256(source), 'parent_execution': identity,
                   'source_files': {str(path.relative_to(build)): sha256(path) for path in build.rglob('*') if path.is_file()},
                   'rust_version': subprocess.check_output([str(tool / 'rust-1.88.0/bin/rustc'), '--version'], env=env, text=True).strip(),
                   'command': command, 'result_weight_end': 0.15, 'started': time.time()}
        with (out / 'build.stdout').open('xb') as stdout, (out / 'build.stderr').open('xb') as stderr:
            run = subprocess.run(['taskset', '-c', '0,2', *command], cwd=build, env=env, stdout=stdout, stderr=stderr, timeout=1800)
        if run.returncode:
            raise RuntimeError('WDL15 build failed')
        trainer = out / 'ngn-k4-wdl15'; shutil.copy2(build / 'target/release/examples/ngn_k4_train', trainer); trainer.chmod(0o555)
        receipt.update(trainer_sha256=sha256(trainer), completed=time.time()); atomic_json(out / 'build.json', receipt)
        for other in ['NGN_WDL25', 'NGN_WDL40']:
            negative = subprocess.run(['taskset', '-c', '0,2', *command], cwd=build, env=dict(env, **{other: '1'}), capture_output=True, text=True, timeout=300)
            (out / (other + '-compile-negative.stderr')).write_text(negative.stderr)
            if negative.returncode == 0 or 'select only one result-weight target' not in negative.stderr:
                raise RuntimeError('mutually exclusive result targets were not rejected')
        atomic_json(out / 'STATE.json', {'state': 'SMOKE', 'started': time.time()})
        sampler = out / 'synthetic-sampler.json'; atomic_json(sampler, {'schema': 'synthetic-smoke-validation-only', 'production': False})
        smoke = out / 'smoke'
        smoke_command = ['/bin/bash', str(root / 'source/training/nnue/bullet/smoke_resume_gate.sh'), str(trainer),
                         '/home/ehrli/nnue-owned-k4-20260920/data/smoke-public-1m.bf', str(sampler), str(smoke)]
        ownednight.run_bounded(smoke_command, out / 'smoke-run', min(deadline, time.time() + 900), env)
        attempt = json.loads((smoke / 'full/attempt.json').read_text())
        if abs(attempt['result_weight_end'] - 0.15) > 1e-7 or attempt['hidden'] != 768 or attempt['input_buckets'] != 4:
            raise RuntimeError('smoke result target/graph differs')
        negative_dir = out / 'negative-result-weight'; negative_dir.mkdir()
        wrong = dict(attempt, result_weight_end=0.25); atomic_json(negative_dir / 'attempt.json', wrong)
        item = negative_dir / 'attempt.json'; parent = json.loads((smoke / 'full/candidates/candidate-128/receipt.json').read_text())
        parent['attempt_contract'] = {'path': str(item), 'bytes': item.stat().st_size, 'sha256': sha256(item)}
        parent_path = negative_dir / 'receipt.json'; atomic_json(parent_path, parent)
        negative = subprocess.run(['taskset', '-c', '0,2', str(trainer), 'resume', 'smoke', str(smoke / 'fixture/finalized-smoke-fixture.json'),
                                   str(parent_path), str(negative_dir / 'forbidden-resume')], env=env, capture_output=True, text=True, timeout=60)
        (out / 'negative-result-weight.stderr').write_text(negative.stderr)
        if negative.returncode == 0 or 'parent attempt contract mismatch' not in negative.stderr:
            raise RuntimeError('mismatched result-weight resume was not rejected')
        atomic_json(out / 'smoke.json', {'pass': True, 'smoke_resume_pass': True, 'mutually_exclusive_flags_rejected': True,
                    'mismatched_weight_resume_rejected': True, 'trainer_sha256': sha256(trainer), 'result_weight_end': 0.15})
        manifest = Path('/home/ehrli/nnue-owned-k4-20260920/archive-a1/corpus-a1/manifest.json')
        if sha256(manifest) != plan['corpus_manifest_sha256']:
            raise RuntimeError('training corpus manifest differs')
        atomic_json(out / 'STATE.json', {'state': 'TRAINING', 'started': time.time()})
        train = out / 'train'
        ownednight.run_bounded([str(trainer), 'start', 'archive-a1-e10', str(manifest), str(train)], out / 'training-run', deadline, env)
        completion = json.loads((train / 'completion.json').read_text())
        if completion['completed_updates'] != plan['updates']:
            raise RuntimeError('completed update cap differs')
        checkpoint = train / 'candidates' / ('candidate-' + str(plan['updates']))
        bridge = root / 'build/ngnk4bridge'; network = out / 'wdl15-e10.nnue'
        if sha256(bridge) != plan['bridge_sha256']:
            raise RuntimeError('conversion/parity bridge differs')
        ownednight.run_bounded([str(bridge), 'convert', '-in', str(checkpoint / 'quantised.bin'), '-manifest', str(manifest), '-out', str(network)],
                               out / 'convert', deadline, env)
        fens = Path('/home/ehrli/nnue-owned-k4-20260920/night-20260927/k4-parity.fens'); probe = out / 'probe.tsv'
        with probe.open('x') as stdout:
            subprocess.run(['taskset', '-c', '0,2', str(trainer), 'probe', str(checkpoint), str(fens)], env=env, stdout=stdout, check=True, timeout=120)
        ownednight.run_bounded([str(bridge), 'parity', '-raw', str(checkpoint / 'raw.bin'), '-model', str(network), '-fens', str(fens), '-probe', str(probe)],
                               out / 'parity', deadline, env)
        if not json.loads((out / 'parity.stdout').read_text())['pass']:
            raise RuntimeError('final export parity failed')
        atomic_json(out / 'ready.json', {'state': 'COMPLETE', 'network': {'path': str(network), 'sha256': sha256(network)},
                    'trainer_sha256': sha256(trainer), 'plan_sha256': sha256(plan_path), 'completion_sha256': sha256(train / 'completion.json'),
                    'checkpoint_sha256': sha256(checkpoint / 'receipt.json'), 'parity_sha256': sha256(out / 'parity.stdout'), 'completed': time.time()})
        atomic_json(out / 'STATE.json', {'state': 'COMPLETE', 'completed': time.time()})
    except Exception as error:
        atomic_json(out / 'STATE.json', {'state': 'FAILED', 'error': str(error), 'completed': time.time()}); raise


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--root', type=Path, required=True); parser.add_argument('--plan', type=Path, required=True)
    args = parser.parse_args(); training(args.root.resolve(), args.plan.resolve())
