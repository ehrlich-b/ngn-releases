#!/usr/bin/env python3
"""Execute the preregistered released-binary owned SMP comparison queue."""
import argparse
import datetime
import json
import os
import subprocess
import sys
import time
from pathlib import Path

import ownedmultimatch as match


def checked(item):
    path = Path(item['path'])
    if match.base.sha256(path) != item['sha256']:
        raise match.base.CandidateMatchError('frozen queue input differs: ' + str(path))
    return path


def capacity():
    value = os.statvfs('/mnt/c')
    return value.f_bavail * value.f_frsize / (1 << 30)


def initial_width(plan):
    width = plan.get('initial_width', 1)
    if type(width) is not int or width not in (1, 2):
        raise match.base.CandidateMatchError('unsupported initial width')
    if width == 1:
        return width
    selection = plan['initial_selection']
    verdict = json.loads(checked(selection['verdict']).read_text())
    control = json.loads(checked(selection['control']).read_text())
    report = json.loads(checked(verdict['report']).read_text())
    spec = json.loads(checked(selection['spec']).read_text())
    audit = checked(selection['audit'])
    audit_record = json.loads(audit.read_text())
    operational = checked(selection['operational'])
    expected = dict(plan['role'], options=[[name, width if name == 'Threads' else value]
                                         for name, value in plan['role']['options']])
    if verdict.get('state') != 'COMPLETE' or verdict.get('decision') != 'SELECT_WIDTH' or \
       verdict.get('selected_width') != width or verdict.get('candidate_width') != width or \
       verdict.get('baseline_width') != 1 or report['games'] != 800 or report['pairs'] != 400 or report['elo_interval_95'][0] <= 0 or \
       control.get('pass') is not True or control.get('purpose') != 'gate' or \
       control.get('role_identity') != match.base.role_identity(expected) or \
       control['report_sha256'] != verdict['report']['sha256'] or \
       control['audit_sha256'] != match.base.sha256(audit) or \
       control['operational_sha256'] != match.base.sha256(operational) or \
       audit_record.get('status') != 'PASS' or audit_record.get('games') != 800 or audit_record.get('pairs') != 400 or \
       not json.loads(operational.read_text())['pass'] or spec.get('purpose') != 'gate' or \
       match.base.role_identity(spec['roles'][0]) != match.base.role_identity(expected):
        raise match.base.CandidateMatchError('initial width lacks the accepted same-role gate')
    return width


def run(root, plan_path):
    match.base.require_wsl()
    plan = json.loads(plan_path.read_text())
    if plan['schema'] != 'ngn-owned-smp-plan-v1' or plan['instrument'] != match.instrument():
        raise match.base.CandidateMatchError('plan/instrument differs')
    checked(plan['controller'])
    if not json.loads(checked(plan['validation']).read_text())['pass']:
        raise match.base.CandidateMatchError('validation did not pass')
    selected_width = initial_width(plan)
    for item in plan['smokes']:
        if json.loads(checked(item).read_text())['state'] != 'COMPLETE':
            raise match.base.CandidateMatchError('smoke did not complete')
    deadline = datetime.datetime.fromisoformat(plan['deadline_utc'].replace('Z', '+00:00')).timestamp()
    out = root / 'matches'; out.mkdir(exist_ok=False)
    match.base.atomic_json(out / 'STATE.json', {'state': 'RUNNING', 'started': time.time(), 'pid': os.getpid(), 'plan_sha256': match.base.sha256(plan_path)})
    decisions = []
    for phase in plan['phases']:
        if capacity() < plan['minimum_windows_free_gib'] or deadline - time.time() < phase['minimum_remaining_seconds']:
            decisions.append({'phase': phase['name'], 'state': 'SKIPPED', 'reason': 'predeclared time/capacity admission'})
            break
        candidate_width = phase['candidate_width']
        baseline_width = selected_width
        role_base = plan['role']
        def role(name, width):
            value = dict(role_base, name=name)
            value['options'] = [[name, width if name == 'Threads' else option] for name, option in role_base['options']]
            return value
        for purpose, pair_count, opening_key in [('aa', phase['aa_pairs'], 'aa_openings'), ('gate', phase['gate_pairs'], 'gate_openings')]:
            name = phase['name'] + '-' + purpose
            opening = phase[opening_key]
            spec = dict(plan['common'], schema='ngn-owned-multicore-spec-v1', purpose=purpose, pairs=pair_count,
                        cpu_teams=phase['cpu_teams'], cpus=sorted(cpu for team in phase['cpu_teams'] for cpu in team),
                        concurrency=len(phase['cpu_teams']), openings_pgn=opening['pgn'], opening_prefixes=opening['prefixes'],
                        limit_seconds=min(phase['match_limit_seconds'], int(deadline - time.time() - 1800)))
            spec['roles'] = [role(f'Owned-{candidate_width}-A', candidate_width), role(f'Owned-{candidate_width}-B', candidate_width)] if purpose == 'aa' else [role(f'Owned-{candidate_width}', candidate_width), role(f'Owned-{baseline_width}', baseline_width)]
            if purpose == 'gate':
                control = out / (phase['name'] + '-aa/control.json')
                spec['control_report'] = {'path': str(control), 'sha256': match.base.sha256(control)}
            spec_path = out / (name + '.json'); match.base.atomic_json(spec_path, spec)
            match.base.atomic_json(out / 'STATE.json', {'state': 'RUNNING', 'name': name, 'started': time.time(), 'pid': os.getpid()})
            command = [sys.executable, str(Path(match.__file__)), '--spec', str(spec_path), '--out', str(out / name)]
            with (out / (name + '.stdout')).open('wb') as stdout, (out / (name + '.stderr')).open('wb') as stderr:
                result = subprocess.run(command, cwd=root, env=match.base.safe_environment(), stdout=stdout, stderr=stderr,
                                        timeout=max(1, int(deadline - time.time() - 60)))
            if result.returncode or json.loads((out / name / 'STATE.json').read_text())['state'] != 'COMPLETE':
                raise match.base.CandidateMatchError('cell did not produce an accepted terminal: ' + name)
        report_path = out / (phase['name'] + '-gate/report.json')
        report = json.loads(report_path.read_text())
        if report['games'] != 2 * phase['gate_pairs']:
            raise match.base.CandidateMatchError('fixed gate cap differs')
        promoted = report['elo_interval_95'][0] > 0
        if promoted:
            selected_width = candidate_width
        decision = {'phase': phase['name'], 'state': 'COMPLETE', 'candidate_width': candidate_width,
                    'baseline_width': baseline_width, 'selected_width': selected_width,
                    'decision': 'SELECT_WIDTH' if promoted else 'RETAIN_BASELINE_WIDTH', 'rule': plan['decision_rule'],
                    'report': {'path': str(report_path), 'sha256': match.base.sha256(report_path)}}
        decisions.append(decision); match.base.atomic_json(out / (phase['name'] + '-verdict.json'), decision)
    match.base.atomic_json(out / 'verdict.json', {'state': 'COMPLETE', 'selected_width': selected_width, 'decisions': decisions,
        'claim': 'Owned width comparison only. Do not infer an absolute rating or pool these cells with prior external anchors.'})
    match.base.atomic_json(out / 'STATE.json', {'state': 'COMPLETE', 'completed': time.time(), 'selected_width': selected_width})


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--root', type=Path, required=True); parser.add_argument('--plan', type=Path, required=True)
    args = parser.parse_args()
    try:
        run(args.root.resolve(), args.plan.resolve())
    except Exception as error:
        out = args.root / 'matches'
        if out.is_dir():
            match.base.atomic_json(out / 'STATE.json', {'state': 'FAILED', 'error': str(error), 'completed': time.time()})
        raise
