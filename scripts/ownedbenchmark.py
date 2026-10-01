#!/usr/bin/env python3
"""Run a frozen native opponent benchmark after fresh instrument controls."""
import argparse
import datetime
import json
import os
import subprocess
import sys
import time
from pathlib import Path

worker_source = Path(__file__).resolve().parent / 'benchmark-source/scripts'
if worker_source.is_dir():
    sys.path.insert(0, str(worker_source))
import ownedmatch as match


def checked(item):
    return match.quickmatch.checked(item)


def free_gib():
    capacity = os.statvfs('/mnt/c')
    return capacity.f_bavail * capacity.f_frsize / (1 << 30)


def activate_compatibility(plan):
    compatibility = plan.get('compatibility')
    if compatibility is None:
        return Path(match.__file__)
    kind = compatibility.get('kind')
    if kind not in {'exact-maelstrom330-post-draw-pv-v1', 'exact-zahak10-post-draw-pv-v1'} or plan['common']['strict'] is not False:
        raise match.CandidateMatchError('unsupported native compatibility instrument')
    runner = checked(compatibility['runner'])
    filename = 'ownedmatch_maelstromdraw.py' if kind == 'exact-maelstrom330-post-draw-pv-v1' else 'ownedmatch_zahakdraw.py'
    if runner != Path(__file__).with_name(filename):
        raise match.CandidateMatchError('native compatibility runner differs')
    if kind == 'exact-maelstrom330-post-draw-pv-v1':
        import ownedmatch_maelstromdraw as wrapper
    else:
        import ownedmatch_zahakdraw as wrapper
    controls = json.loads(checked(compatibility['controls']).read_text())
    if controls.get('pass') is not True or controls['instrument'] != match.instrument() or \
       controls['independently_validated_warning_count'] < 1 or len(controls['negative_fixtures']) < 10 or \
       not all(row['rejected'] for row in controls['negative_fixtures']):
        raise match.CandidateMatchError('native compatibility controls differ or failed')
    return Path(wrapper.__file__)


def run(root, plan_path):
    match.require_wsl()
    plan = json.loads(plan_path.read_text())
    if plan['schema'] != 'ngn-owned-external-benchmark-plan-v1':
        raise match.CandidateMatchError('external benchmark plan schema differs')
    runner = activate_compatibility(plan)
    checked(plan['controller'])
    checked(plan['source_receipt'])
    if plan['instrument'] != match.instrument():
        raise match.CandidateMatchError('external benchmark instrument differs')
    validation = json.loads(checked(plan['validation']).read_text())
    if not validation['pass'] or validation['instrument'] != plan['instrument']:
        raise match.CandidateMatchError('external benchmark controls differ or failed')
    openings = json.loads(checked(plan['opening_receipt']).read_text())
    for role in plan['roles']:
        match.validate_one_worker_role(role)
        checked(role['engine'])
        if role.get('network'):
            checked(role['network'])
    deadline = datetime.datetime.fromisoformat(plan['deadline_utc'].replace('Z', '+00:00')).timestamp()
    if free_gib() < plan['minimum_windows_free_gib'] or deadline - time.time() < plan['initial_remaining_seconds']:
        raise match.CandidateMatchError('external benchmark admission failed before start')
    out = root / 'matches'
    out.mkdir(exist_ok=False)
    match.atomic_json(out / 'STATE.json', {'state': 'RUNNING', 'started': time.time(),
                                          'plan_sha256': match.sha256(plan_path)})
    completed = []
    for cell in plan['cells']:
        if free_gib() < plan['minimum_windows_free_gib'] or deadline - time.time() < cell['minimum_remaining_seconds']:
            match.atomic_json(out / 'STATE.json', {'state': 'SKIPPED', 'reason': 'declared time/capacity admission',
                                                  'completed_cells': completed, 'completed': time.time()})
            return
        name = cell['name']
        roles = plan['roles']
        if cell['purpose'] == 'aa':
            roles = [dict(roles[0], name='Owned-1-A'), dict(roles[0], name='Owned-1-B')]
        spec = dict(plan['common'], schema='ngn-owned-match-spec-v1', purpose=cell['purpose'],
                    pairs=cell['pairs'], roles=roles, openings_pgn=openings[name]['pgn'],
                    opening_prefixes=openings[name]['prefixes'], limit_seconds=cell['limit_seconds'])
        if cell['purpose'] == 'anchor':
            control = out / 'aa/control.json'
            spec['control_report'] = {'path': str(control), 'sha256': match.sha256(control)}
        spec_path = out / (name + '.json')
        match.atomic_json(spec_path, spec)
        match.atomic_json(out / 'STATE.json', {'state': 'RUNNING', 'name': name,
                                              'started': time.time(), 'completed_cells': completed})
        command = [sys.executable, str(runner), '--spec', str(spec_path), '--out', str(out / name)]
        with (out / (name + '.stdout')).open('wb') as stdout, (out / (name + '.stderr')).open('wb') as stderr:
            result = subprocess.run(command, cwd=root, env=match.safe_environment(), stdout=stdout, stderr=stderr,
                                    timeout=max(1, int(deadline - time.time() - 60)))
        if result.returncode or json.loads((out / name / 'STATE.json').read_text())['state'] != 'COMPLETE':
            raise match.CandidateMatchError('external benchmark cell lacks accepted terminal: ' + name)
        completed.append(name)
    report_path = out / 'anchor/report.json'
    report = json.loads(report_path.read_text())
    if report['games'] != 2 * plan['cells'][-1]['pairs']:
        raise match.CandidateMatchError('external benchmark fixed game cap differs')
    match.atomic_json(out / 'verdict.json', {
        'state': 'COMPLETE', 'games': report['games'], 'elo': report['elo'],
        'elo_interval_95': report['elo_interval_95'], 'score_interval_95': report['score_interval_95'],
        'decision': 'WIN_PROVED' if report['score_interval_95'][0] > 0.5 else 'WIN_UNPROVEN',
        'report': {'path': str(report_path), 'sha256': match.sha256(report_path)},
        'claim': plan['claim_boundary'],
    })
    match.atomic_json(out / 'STATE.json', {'state': 'COMPLETE', 'completed': time.time(), 'completed_cells': completed})


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--root', type=Path, required=True)
    parser.add_argument('--plan', type=Path, required=True)
    args = parser.parse_args()
    try:
        run(args.root.resolve(), args.plan.resolve())
    except Exception as error:
        out = args.root / 'matches'
        if out.is_dir():
            match.atomic_json(out / 'STATE.json', {'state': 'FAILED', 'error': str(error), 'completed': time.time()})
        raise
