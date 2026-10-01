#!/usr/bin/env python3
"""Run the fixed two-core released NGN versus exact Counter 5.5 calibration."""
import argparse
import datetime
import json
import os
import subprocess
import sys
import time
from pathlib import Path

worker_source = Path(__file__).resolve().parent / 'external-source-v2/scripts'
if worker_source.is_dir():
    sys.path.insert(0, str(worker_source))
import ownedmultimatch as match


def checked(item):
    path = Path(item['path'])
    if match.base.sha256(path) != item['sha256']:
        raise match.base.CandidateMatchError('frozen external input differs: ' + str(path))
    return path


def free_gib():
    value = os.statvfs('/mnt/c')
    return value.f_bavail * value.f_frsize / (1 << 30)


def run(root, plan_path):
    match.base.require_wsl()
    plan = json.loads(plan_path.read_text())
    if plan['schema'] != 'ngn-owned-smp-counter-width2-plan-v1':
        raise match.base.CandidateMatchError('external plan schema differs')
    os.environ['NGN_COUNTER_DRAW_DEPS'] = plan['counter_draw_dependency']
    import ownedmatch_counterdraw  # noqa: F401  Installs the exact-release diagnostic checker.
    if plan['instrument'] != match.instrument():
        raise match.base.CandidateMatchError('external instrument differs')
    checked(plan['controller'])
    if not json.loads(checked(plan['validation']).read_text())['pass']:
        raise match.base.CandidateMatchError('external controls did not pass')
    controls = json.loads(checked(plan['counter_controls']).read_text())
    if not controls['pass'] or not controls['negative_fixtures']:
        raise match.base.CandidateMatchError('Counter diagnostic controls did not pass')
    verdict = json.loads(checked(plan['width2_verdict']).read_text())
    if verdict['selected_width'] != 2 or verdict['decision'] != 'SELECT_WIDTH':
        raise match.base.CandidateMatchError('two-core precondition did not pass')
    openings = json.loads(checked(plan['opening_receipt']).read_text())
    deadline = datetime.datetime.fromisoformat(plan['deadline_utc'].replace('Z', '+00:00')).timestamp()
    if free_gib() < plan['minimum_windows_free_gib'] or deadline - time.time() < plan['initial_remaining_seconds']:
        raise match.base.CandidateMatchError('external admission failed before start')
    out = root / 'external-width2'; out.mkdir(exist_ok=False)
    match.base.atomic_json(out / 'STATE.json', {'state': 'RUNNING', 'started': time.time(), 'plan_sha256': match.base.sha256(plan_path)})
    completed = []
    for cell in plan['cells']:
        if free_gib() < plan['minimum_windows_free_gib'] or deadline - time.time() < cell['minimum_remaining_seconds']:
            match.base.atomic_json(out / 'STATE.json', {'state': 'SKIPPED', 'reason': 'declared time/capacity admission',
                'completed_cells': completed, 'completed': time.time()})
            return
        name = cell['name']
        if cell['purpose'] == 'aa':
            roles = [dict(plan['owned_role'], name='Owned-2-A'), dict(plan['owned_role'], name='Owned-2-B')]
        else:
            roles = [plan['owned_role'], plan['counter_role']]
        spec = dict(plan['common'], schema='ngn-owned-multicore-counter-spec-v1', purpose=cell['purpose'],
                    pairs=cell['pairs'], roles=roles, openings_pgn=openings[name]['pgn'],
                    opening_prefixes=openings[name]['prefixes'], limit_seconds=min(cell['limit_seconds'], int(deadline-time.time()-120)))
        if cell['purpose'] == 'anchor':
            control = out / 'aa/control.json'
            spec['control_report'] = {'path': str(control), 'sha256': match.base.sha256(control)}
        spec_path = out / (name + '.json'); match.base.atomic_json(spec_path, spec)
        match.base.atomic_json(out / 'STATE.json', {'state': 'RUNNING', 'name': name, 'started': time.time(), 'completed_cells': completed})
        env = dict(match.base.safe_environment(), NGN_COUNTER_DRAW_DEPS=plan['counter_draw_dependency'])
        command = [sys.executable, str(Path(match.__file__)), '--spec', str(spec_path), '--out', str(out / name)]
        with (out / (name + '.stdout')).open('wb') as stdout, (out / (name + '.stderr')).open('wb') as stderr:
            result = subprocess.run(command, cwd=root, env=env, stdout=stdout, stderr=stderr,
                                    timeout=max(1, int(deadline-time.time()-60)))
        if result.returncode or json.loads((out / name / 'STATE.json').read_text())['state'] != 'COMPLETE':
            raise match.base.CandidateMatchError('external cell did not produce accepted terminal: ' + name)
        completed.append(name)
    report = out / 'anchor/report.json'
    final = json.loads(report.read_text())
    if final['games'] != 2 * plan['cells'][-1]['pairs']:
        raise match.base.CandidateMatchError('external anchor game cap differs')
    match.base.atomic_json(out / 'verdict.json', {'state': 'COMPLETE', 'games': final['games'], 'elo': final['elo'],
        'elo_interval_95': final['elo_interval_95'], 'report': {'path': str(report), 'sha256': match.base.sha256(report)},
        'claim': 'Relative Elo versus exact Counter 5.5 at 60+0.6. Published CCRL rating transfer is an inference, not a direct rating.'})
    match.base.atomic_json(out / 'STATE.json', {'state': 'COMPLETE', 'completed': time.time(), 'completed_cells': completed})


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--root', type=Path, required=True)
    parser.add_argument('--plan', type=Path, required=True)
    args = parser.parse_args()
    try:
        run(args.root.resolve(), args.plan.resolve())
    except Exception as error:
        out = args.root / 'external-width2'
        if out.is_dir():
            match.base.atomic_json(out / 'STATE.json', {'state': 'FAILED', 'error': str(error), 'completed': time.time()})
        raise
