#!/usr/bin/env python3
"""Measure owned NNUE widths with disjoint physical CPU teams and full audits.

The original one-worker instrument and helpers remain unchanged. Each fastchess
slot inherits one anchor CPU; a separate launcher expands it to that slot's
declared team before engine startup. Both roles receive the same team, with
GOMAXPROCS matching their declared UCI width. A/A binds instrument, clock,
concurrency, CPU teams and one participating role exactly.
"""
import argparse
import collections
import json
import os
import re
import shutil
import sys
import time
from pathlib import Path

import ownedmatch as base
import quickmatch


def instrument():
    paths = [Path(__file__), base.HELPERS / 'smp_role_exec.py', base.HELPERS / 'smp_match_stage.py']
    return dict(base.instrument(), **{path.name: base.sha256(path) for path in paths})


def configuration(spec):
    return {key: spec.get(key) for key in ('tc', 'nodes', 'concurrency', 'cpus', 'cpu_teams', 'strict', 'opening_plies', 'minimum_full_width_fraction',
                                         'input_pool', 'trace_storage', 'maximum_trace_bytes')}


def mask_text(team):
    return ','.join(str(cpu) for cpu in sorted(team))


def validate_layout(spec):
    # SearchFixed's node budget deliberately selects the primary worker only.
    # Such a run cannot witness Lazy SMP, even if UCI Threads advertises >1.
    external = spec.get('schema') == 'ngn-owned-multicore-counter-spec-v1'
    if spec.get('schema') not in ('ngn-owned-multicore-spec-v1', 'ngn-owned-multicore-counter-spec-v1') or 'tc' not in spec or 'nodes' in spec:
        raise base.CandidateMatchError('invalid multicore schema/budget')
    minimum = spec.get('minimum_full_width_fraction')
    if type(minimum) not in [int, float] or not 0 < minimum <= 1:
        raise base.CandidateMatchError('invalid full-width admission fraction')
    teams = spec['cpu_teams']
    if not isinstance(teams, list) or not teams or spec['concurrency'] != len(teams):
        raise base.CandidateMatchError('concurrency must equal team count')
    flat = [cpu for team in teams for cpu in team]
    if any(not isinstance(team, list) or not team or team != sorted(team) for team in teams) or any(type(cpu) is not int or cpu < 0 for cpu in flat):
        raise base.CandidateMatchError('invalid CPU teams')
    if len(set(flat)) != len(flat) or sorted(flat) != spec['cpus'] or len({len(team) for team in teams}) != 1:
        raise base.CandidateMatchError('overlapping, inconsistent or unequal CPU teams')
    if len(spec['roles']) != 2 or spec['roles'][0]['name'] == spec['roles'][1]['name'] or spec['pairs'] < 1:
        raise base.CandidateMatchError('invalid roles/pairs')
    for role in spec['roles']:
        options = dict(role['options'])
        if len(options) != len(role['options']) or type(options.get('Threads')) is not int or options['Threads'] not in [1, 2, 4, 8]:
            raise base.CandidateMatchError('invalid or duplicate width options')
        if options['Threads'] > len(teams[0]):
            raise base.CandidateMatchError('width exceeds CPU team')
        counter = role.get('engine', {}).get('sha256') == base.COUNTER55_SHA256
        if counter:
            if not external or options['Threads'] != 1 or role.get('network') or set(options) != {'Threads', 'Hash', 'ExperimentSettings'} or options['ExperimentSettings'] is not False:
                raise base.CandidateMatchError('external mode admits only exact Counter 5.5 at width one')
        elif options.get('EvalBackend') != 'ngn-k4-768-v1' or not role.get('network') or options.get('OwnBook') is not False:
            raise base.CandidateMatchError('require book-free owned K4 role')
    if spec.get('strict') is not (not external):
        raise base.CandidateMatchError('strict setting differs from declared match mode')
    if external and spec['purpose'] == 'anchor' and sum(role.get('engine', {}).get('sha256') == base.COUNTER55_SHA256 for role in spec['roles']) != 1:
        raise base.CandidateMatchError('external anchor requires one exact Counter role')


def admit_control(spec, hashes):
    if spec['purpose'] == 'aa':
        if base.role_identity(spec['roles'][0]) != base.role_identity(spec['roles'][1]):
            raise base.CandidateMatchError('A/A roles differ')
    elif spec['purpose'] in ['gate', 'anchor']:
        control = json.loads(quickmatch.checked(spec['control_report']).read_text())
        if not control.get('pass') or control.get('purpose') != 'aa' or control['instrument'] != hashes or control['configuration'] != configuration(spec):
            raise base.CandidateMatchError('A/A instrument or resource configuration differs')
        if control['role_identity'] not in [base.role_identity(role) for role in spec['roles']]:
            raise base.CandidateMatchError('A/A role does not participate')
    elif spec['purpose'] != 'smoke':
        raise base.CandidateMatchError('invalid purpose')


def audit_width_receipts(lines, roles, minimum_fraction):
    widths = {role['name']: int(dict(role['options'])['Threads']) for role in roles}
    owned = {role['name'] for role in roles if role.get('ngn', True)}
    receipt_re = re.compile(r'info string threads configured (\d+) effective (\d+)')
    pending, pending_options = {}, {}
    counts = {name: collections.Counter() for name in widths}
    slot_full, option_counts = collections.Counter(), collections.Counter()
    for number, line in enumerate(lines, 1):
        match = base.TRACE_RE.fullmatch(line)
        if not match:
            continue  # The independent operational auditor rejects malformed syntax.
        body = match.group('body').strip()
        thread = match.group('thread').strip()
        if ' <--- ' in body:
            name, payload = body.split(' <--- ', 1)
            key = (thread, name)
            if name in owned and payload.startswith('setoption name Threads value '):
                if key in pending or key in pending_options or int(payload.rsplit(' ', 1)[1]) != widths[name]:
                    raise base.CandidateMatchError('invalid/nested Threads configuration')
                pending_options[key] = widths[name]
            elif name in widths and payload.startswith('go '):
                if key in pending:
                    raise base.CandidateMatchError('nested go in width receipt trace')
                if key in pending_options:
                    raise base.CandidateMatchError('go before Threads configuration acknowledgement')
                pending[key] = None
        elif ' ---> ' in body:
            name, payload = body.split(' ---> ', 1)
            width = receipt_re.fullmatch(payload)
            if name not in widths:
                if width:
                    raise base.CandidateMatchError('foreign worker receipt')
                continue
            key = (thread, name)
            if width:
                configured, effective = map(int, width.groups())
                if key in pending_options:
                    if configured != pending_options.pop(key) or effective != configured:
                        raise base.CandidateMatchError('Threads configuration acknowledgement differs')
                    option_counts[name] += 1
                    continue
                if key not in pending or pending[key] is not None:
                    raise base.CandidateMatchError('worker receipt outside go or duplicated')
                if configured != widths[name] or configured <= 1 or not 1 <= effective <= configured:
                    raise base.CandidateMatchError('configured/effective worker receipt differs')
                pending[key] = effective
            elif payload.startswith('bestmove '):
                if key not in pending:
                    raise base.CandidateMatchError('bestmove without a witnessed go')
                effective = pending.pop(key)
                if widths[name] > 1 and effective is None:
                    raise base.CandidateMatchError('missing per-go worker receipt')
                counts[name][effective if effective is not None else 1] += 1
                slot_full[key] += int(effective == widths[name] or widths[name] == 1)
    if pending or pending_options:
        raise base.CandidateMatchError('unfinished go or Threads configuration in worker receipt trace')
    if not 0 <= minimum_fraction <= 1:
        raise base.CandidateMatchError('invalid full-width admission fraction')
    report = {}
    for name, width in widths.items():
        total = sum(counts[name].values())
        if not total or (name in owned and not option_counts[name]):
            raise base.CandidateMatchError('role has no witnessed searches/configuration')
        fraction = counts[name][width] / total
        if width > 1 and (fraction < minimum_fraction or any(count == 0 for (thread, actor), count in slot_full.items() if actor == name)):
            raise base.CandidateMatchError('declared SMP width did not run sufficiently')
        report[name] = {'configured': width, 'searches': total, 'configuration_acknowledgements': option_counts[name],
                        'effective_width_counts': dict(counts[name]), 'full_width_fraction': fraction}
    return {'pass': True, 'minimum_full_width_fraction': minimum_fraction, 'roles': report}


def run(spec_path, out):
    base.require_wsl()
    spec = json.loads(spec_path.read_text())
    validate_layout(spec)
    external = spec['schema'] == 'ngn-owned-multicore-counter-spec-v1'
    if external:
        # Counter's exact release emits a post-draw PV warning under fastchess.
        # The independently validated wrapper accepts only that proven case.
        import ownedmatch_counterdraw  # noqa: F401
        base.COUNTER55_BANNER = re.compile(base.COUNTER55_BANNER.pattern.replace(
            'NumCPU 1$', f'NumCPU {len(spec["cpu_teams"][0])}$'))
    if not set(spec['cpus']).issubset(os.sched_getaffinity(0)):
        raise base.CandidateMatchError('declared CPUs unavailable')
    siblings = [(Path('/sys/devices/system/cpu') / f'cpu{cpu}' / 'topology/thread_siblings_list').read_text().strip() for cpu in spec['cpus']]
    if len(set(siblings)) != len(siblings):
        raise base.CandidateMatchError('CPU teams must use distinct physical cores')
    hashes = instrument()
    admit_control(spec, hashes)
    out.mkdir(parents=True, exist_ok=False)
    base.atomic_json(out / 'spec.json', spec)
    state = {'state': 'PREPARING', 'started': time.time()}
    base.atomic_json(out / 'STATE.json', state)
    frozen = base.ownedstorage.stage_inputs(spec, out, quickmatch.checked)
    roles, crashes = [], []
    for index, role in enumerate(spec['roles']):
        cwd = out / f'role{index}'; cwd.mkdir()
        binary = cwd / 'engine'; shutil.copy2(quickmatch.checked(role['engine']), binary)
        network = None
        if role.get('network'):
            network = cwd / 'ngn.nnue'; shutil.copy2(quickmatch.checked(role['network']), network)
        options = [(name, str(network) if value == '$NETWORK' else value) for name, value in role['options']]
        if any(value is None for _, value in options):
            raise base.CandidateMatchError('$NETWORK without network')
        launcher = cwd / 'smp_role_exec.py'; shutil.copy2(base.HELPERS / 'smp_role_exec.py', launcher); launcher.chmod(0o555)
        width = str(dict(options)['Threads'])
        base.atomic_json(cwd / 'role-config.json', {'schema': 'ngn-smp-role-exec-v1', 'engine': str(binary),
            'engine_sha256': role['engine']['sha256'], 'gomaxprocs': width, 'expected_cwd': str(cwd), 'cpu_teams': spec['cpu_teams']})
        receipt = base.preflight(launcher, cwd, options, role, out / f'preflight{index}.jsonl', mask_text(spec['cpu_teams'][0]))
        if receipt['ngn'] == (role['engine']['sha256'] == base.COUNTER55_SHA256):
            raise base.CandidateMatchError('evaluator preflight identity differs')
        base.atomic_json(out / f'preflight{index}.json', receipt)
        if receipt['ngn']:
            crash = cwd / 'ngn_crashes.log'; crashes.append(base.crash_lines(crash, crash, 1)); shutil.move(crash, out / f'preflight{index}-crashes.log')
        roles.append({'id': str(index), 'name': role['name'], 'engine': str(binary), 'launcher': str(launcher),
                      'cwd': str(cwd), 'options': options, 'ngn': receipt['ngn'], 'sha256': role['engine']['sha256'], 'gomaxprocs': width})
    storage = base.ownedstorage.TraceStorage(out, spec)
    command = [str(frozen['fastchess'])]
    for role in roles:
        command += ['-engine', f'cmd={role["launcher"]}', f'name={role["name"]}', f'dir={role["cwd"]}']
        command += [f'option.{name}={base.value_text(value)}' for name, value in role['options']]
    budget = [f'tc={spec["tc"]}', 'timemargin=0']
    command += ['-openings', f'file={frozen["openings_pgn"]}', 'format=pgn', 'order=sequential', 'start=1', f'plies={spec["opening_plies"]}',
        '-each', *budget, '-rounds', str(spec['pairs']), '-games', '2', '-repeat', '-concurrency', str(spec['concurrency']),
        '-use-affinity', ','.join(str(team[0]) for team in spec['cpu_teams']), '-srand', str(spec['seed']),
        *(['-strict'] if spec['strict'] else []), '-show-latency',
        '-pgnout', f'file={out / "games.pgn"}', 'notation=uci', 'append=false', 'nodes=true', 'seldepth=true', 'nps=true', 'hashfull=true',
        'timeleft=true', 'latency=true', 'pv=true', '-epdout', f'file={out / "final.epd"}', 'append=false',
        '-log', f'file={storage.path}', 'level=trace', 'engine=true', 'realtime=false', 'append=false']
    base.atomic_json(out / 'command.json', command)
    stage_path = out / 'match-stage.json'
    masks = [mask_text(team) for team in spec['cpu_teams']]
    base.atomic_json(stage_path, {'schema': 'ngn-smp-match-stage-v1', 'command': command, 'cwd': str(out), 'environment': base.safe_environment(),
        'sample_interval_seconds': 1, 'roles': [{'id': role['id'], 'engine': role['engine'], 'engine_sha256': role['sha256'],
            'cwd': role['cwd'], 'gomaxprocs': role['gomaxprocs'], 'allowed_cpu_masks': masks, 'launcher': role['launcher']} for role in roles],
        'match_stdout': str(out / 'match.stdout'), 'match_stderr': str(out / 'match.stderr'), 'witness': str(out / 'child-processes.json')})
    state['state'] = 'MATCH_RUNNING'; base.atomic_json(out / 'STATE.json', state)
    with storage:
        base.stage(out, 'supervised-match', [sys.executable, str(base.HELPERS / 'smp_match_stage.py'), '--config', str(stage_path)], spec, spec['limit_seconds'], storage.path)
    witness = json.loads((out / 'child-processes.json').read_text())
    if witness['schema'] != 'ngn-smp-child-process-witness-v1' or witness['state'] != 'COMPLETE' or witness['violations'] or (out / 'match.stderr').stat().st_size:
        raise base.CandidateMatchError('SMP process witness failed')
    for role in roles:
        if len(witness['observed_instances'][role['id']]) != spec['concurrency']:
            raise base.CandidateMatchError('observed engine instance count differs from slot count')
        observed = {row['cpus_allowed_list'] for row in witness['observations'] if row['role_id'] == role['id']}
        if observed != set(masks):
            raise base.CandidateMatchError('a declared CPU team was never observed')
    trace_roles = [{'display_name': role['name'], 'resolved_options': [{'name': name, 'value': base.value_text(value)} for name, value in role['options']],
        'expected_refreshes': 2 * spec['pairs'], 'expected_processes': len(witness['observed_instances'][role['id']])} for role in roles]
    trace_lines, trace_sha256 = base.ownedstorage.read_trace(out, spec)
    trace = base.operational_trace(trace_lines, trace_roles, roles)
    widths = audit_width_receipts(trace_lines, roles, spec['minimum_full_width_fraction'])
    if any(row['configuration_acknowledgements'] != 2 * spec['pairs'] for name, row in widths['roles'].items() if name in {role['name'] for role in roles if role['ngn']}):
        raise base.CandidateMatchError('Threads acknowledgement count differs from game refreshes')
    for role in roles:
        if role['ngn']:
            crash = Path(role['cwd']) / 'ngn_crashes.log'; crashes.append(base.crash_lines(crash, crash, spec['concurrency']))
    base.atomic_json(out / 'audit-operational.json', {'pass': True, 'trace': trace, 'crash_logs': crashes,
        'unfiltered_trace_sha256': trace_sha256, 'process_witness_sha256': base.sha256(out / 'child-processes.json'),
        'effective_widths': widths})
    audit_command = [sys.executable, str(frozen['auditor']), '--pgn', str(out / 'games.pgn'), '--final-epd', str(out / 'final.epd'),
        '--opening-prefixes', str(frozen['opening_prefixes']), '--opening-prefixes-sha256', spec['opening_prefixes']['sha256'],
        '--opening-pgn', str(frozen['openings_pgn']), '--opening-pgn-sha256', spec['openings_pgn']['sha256'],
        '--stockfish', str(frozen['stockfish']), '--stockfish-sha256', spec['stockfish']['sha256'],
        '--engine-a', roles[0]['name'], '--engine-b', roles[1]['name'], '--games', str(2 * spec['pairs']), '--pairs', str(spec['pairs']), '--output', str(out / 'audit.json')]
    state['state'] = 'AUDIT_RUNNING'; base.atomic_json(out / 'STATE.json', state)
    base.stage(out, 'supervised-audit', audit_command, spec, 1800)
    legal_audit = json.loads((out / 'audit.json').read_text())
    if sum(row['searches'] for row in widths['roles'].values()) != legal_audit['legal_plies'] - spec['opening_plies'] * 2 * spec['pairs']:
        raise base.CandidateMatchError('search count differs from independently audited non-book plies')
    if spec['purpose'] == 'smoke':
        _, audit = quickmatch.pair_scores(out)
        report = {'schema': 'ngn-owned-multicore-smoke-v1', 'pass': True, 'games': audit['games'], 'legal_plies': audit['legal_plies']}
        base.atomic_json(out / 'report.json', report)
    else:
        quickmatch.report(out, None); report = json.loads((out / 'report.json').read_text())
    if spec['purpose'] == 'aa' and not report['score_interval_95'][0] <= 0.5 <= report['score_interval_95'][1]:
        raise base.CandidateMatchError('A/A interval excludes parity')
    base.atomic_json(out / 'control.json', {'purpose': spec['purpose'], 'pass': True, 'instrument': hashes, 'configuration': configuration(spec),
        'role_identity': base.role_identity(spec['roles'][0]), 'report_sha256': base.sha256(out / 'report.json'),
        'audit_sha256': base.sha256(out / 'audit.json'), 'operational_sha256': base.sha256(out / 'audit-operational.json')})
    state.update(state='COMPLETE', completed=time.time()); base.atomic_json(out / 'STATE.json', state)


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__); parser.add_argument('--spec', type=Path, required=True); parser.add_argument('--out', type=Path, required=True)
    args = parser.parse_args()
    try:
        run(args.spec.resolve(), args.out.resolve())
    except Exception as error:
        if args.out.is_dir():
            base.atomic_json(args.out / 'STATE.json', {'state': 'FAILED', 'error': str(error), 'completed': time.time()})
        raise
