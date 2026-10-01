#!/usr/bin/env python3
"""Run frozen one-worker NNUE gates with protocol, process and chess controls.

Uses quickmatch's spec and paired report, plus purpose (aa/smoke/gate/anchor),
limit_seconds, and a checked control_report for gate/anchor runs. A/A must use
the same instrument, clock, concurrency, affinity and one participating role.
Existing candidate-match helpers provide bounded process supervision, live
executable/environment/affinity witnesses and strict operational trace audit.
"""
from __future__ import annotations

import argparse
import json
import os
import re
import shutil
import subprocess
import sys
import time
from pathlib import Path

import quickmatch

HELPERS = Path(__file__).resolve().parent / 'candidate-match'
sys.path.insert(0, str(HELPERS))
from common import CandidateMatchError, atomic_json, require_wsl, safe_environment, sha256, supervisor_command, validate_supervisor_receipt
from trace_audit import TRACE_RE, audit_trace, crash_lines
from uci_preflight import EVAL_RE, Protocol, parse_advertised_options
import ownedstorage

COUNTER55_SHA256 = '6c48fb52934d49d3774633e32f0b4fb4796b1e2c63925f24167ef0f0c0d761c8'
# Unmodified Maelstrom v3.3.0, source b71b738509b431b8d1e7fb60f448f796b09d743b,
# built on WSL with Go 1.25.5, CGO=1, GOAMD64=v3 and -trimpath.
# Its UCI manager owns one Searcher and exposes Hash/Ponder only. The existing
# launcher and live process witness still require GOMAXPROCS=1 and one CPU.
MAELSTROM330_SHA256 = '66ea969530d63f5904114079f518ba618b0deb4e269eb0c95d7496960379ea53'
COUNTER55_BANNER = re.compile(
    r'^\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2} main\.go:39: Counter VersionName 5\.5 '
    r'BuildDate 2024-01-12 GitRevision 63c487ca724c620f71c129d62129c6fb9109c872 '
    r'RuntimeVersion go1\.21\.0 GOARCH amd64 GOOS linux NumCPU 1$')
COUNTER55_WEIGHTS = re.compile(r'^\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2} loaded embed nnue weights$')


def counter_stderr_kind(line: str) -> str | None:
    if COUNTER55_BANNER.fullmatch(line):
        return 'version'
    if COUNTER55_WEIGHTS.fullmatch(line):
        return 'embedded_weights'
    return None


class CheckedProtocol(Protocol):
    """Preserve the exact release's one benign startup banner in the receipt."""
    def __init__(self, process, transcript, counter55: bool):
        self.counter55 = counter55
        self.allowed_stderr = []
        self.allowed_stderr_kinds = set()
        super().__init__(process, transcript)

    def reader(self, stream, handle):
        if stream != 'stderr' or not self.counter55:
            return super().reader(stream, handle)
        while raw := handle.readline():
            line = raw.decode('utf-8', errors='replace').rstrip('\r\n')
            timestamp = time.monotonic()
            self.record('engine-stderr', line, timestamp)
            kind = counter_stderr_kind(line)
            if kind and kind not in self.allowed_stderr_kinds:
                self.allowed_stderr.append(line)
                self.allowed_stderr_kinds.add(kind)
                self.events.put(('allowed-startup-stderr', line, timestamp))
            else:
                self.stderr_lines.append(line)
                self.events.put(('stderr', line, timestamp))
        self.events.put(('stderr', '<EOF>', time.monotonic()))


def operational_trace(lines: list[str], trace_roles: list[dict], roles: list[dict]) -> dict:
    allowed_names = {role['name'] for role in roles if role['sha256'] == COUNTER55_SHA256}
    filtered, banners = [], []
    for number, line in enumerate(lines, 1):
        match = TRACE_RE.fullmatch(line)
        body = match.group('body').strip() if match else ''
        if ' ---> ' in body:
            actor, payload = body.split(' ---> ', 1)
            if actor.strip().startswith('<stderr>'):
                name = actor.strip()[len('<stderr>'):].strip()
                kind = counter_stderr_kind(payload.strip())
                if name in allowed_names and kind and match.group('level').strip().upper() in {'TRACE', 'DEBUG', 'INFO', 'ENGINE'}:
                    banners.append({'line': number, 'role': name, 'kind': kind, 'payload': payload.strip()})
                    continue
        filtered.append(line)
    for role in trace_roles:
        if role['display_name'] in allowed_names:
            for kind in ('version', 'embedded_weights'):
                count = sum(row['role'] == role['display_name'] and row['kind'] == kind for row in banners)
                if count != role['expected_processes']:
                    raise CandidateMatchError('native Counter startup-record count differs from process witness')
    report = audit_trace(filtered, trace_roles)
    report['allowed_exact_counter55_startup_banners'] = banners
    report['unfiltered_trace_lines'] = len(lines)
    return report


def instrument() -> dict:
    paths = [Path(__file__), Path(quickmatch.__file__), Path(ownedstorage.__file__)] + [HELPERS / name for name in
             ('common.py', 'uci_preflight.py', 'trace_audit.py', 'role_exec.py', 'run_match_stage.py', 'process_supervisor.py')]
    return {path.name: sha256(path) for path in paths}


def value_text(value) -> str:
    return str(value).lower() if isinstance(value, bool) else str(value)


def validate_one_worker_role(role: dict) -> None:
    options = dict(role['options'])
    if len(options) != len(role['options']):
        raise CandidateMatchError('duplicate role option')
    if role['engine']['sha256'] == MAELSTROM330_SHA256:
        if role.get('network') or set(options) != {'Hash', 'Ponder'} or options['Ponder'] is not False:
            raise CandidateMatchError('exact Maelstrom role requires Hash and Ponder=false only')
        if type(options['Hash']) is not int or not 1 <= options['Hash'] <= 4096:
            raise CandidateMatchError('Maelstrom Hash outside advertised domain')
    elif type(options.get('Threads')) is not int or options['Threads'] != 1:
        raise CandidateMatchError('this instrument admits Threads=1 or exact single-worker Maelstrom only')


def validate_maelstrom_handshake(lines: list[str], advertised: dict) -> None:
    if [line for line in lines if line.startswith('id name ')] != ['id name Maelstrom v3.3.0'] or \
       [line for line in lines if line.startswith('id author ')] != ['id author Saigautam Bonam']:
        raise CandidateMatchError('exact Maelstrom UCI identity differs')
    if set(advertised) != {'Hash', 'Ponder'} or advertised['Hash']['type'] != 'spin' or \
       advertised['Hash'].get('min') != 1 or advertised['Hash'].get('max') != 4096 or \
       advertised['Ponder']['type'] != 'check':
        raise CandidateMatchError('exact Maelstrom UCI option set differs')


def stage_role_inputs(role: dict, cwd: Path) -> tuple[Path, list]:
    binary = cwd / 'engine'
    shutil.copy2(quickmatch.checked(role['engine']), binary)
    network = None
    if role.get('network'):
        # The owned release loads this sibling before its first UCI command.
        network = cwd / 'ngn.nnue'
        shutil.copy2(quickmatch.checked(role['network']), network)
    options = [(name, str(network) if value == '$NETWORK' else value) for name, value in role['options']]
    if any(value is None for _, value in options):
        raise CandidateMatchError('$NETWORK without network')
    return binary, options


def role_identity(role: dict) -> dict:
    return {key: role.get(key) for key in ('engine', 'network', 'options')}


def configuration(spec: dict) -> dict:
    return {key: spec.get(key) for key in ('tc', 'nodes', 'concurrency', 'cpus', 'strict',
                                         'input_pool', 'trace_storage', 'maximum_trace_bytes')}


def admit_control(spec: dict, current_instrument: dict) -> None:
    purpose = spec['purpose']
    if purpose == 'aa':
        if role_identity(spec['roles'][0]) != role_identity(spec['roles'][1]):
            raise CandidateMatchError('A/A roles differ')
    elif purpose in {'gate', 'anchor'}:
        control_path = quickmatch.checked(spec['control_report'])
        control = json.loads(control_path.read_text())
        if control.get('purpose') != 'aa' or not control.get('pass'):
            raise CandidateMatchError('A/A control did not pass')
        if control['instrument'] != current_instrument or control['configuration'] != configuration(spec):
            raise CandidateMatchError('A/A instrument/clock/concurrency/affinity differ')
        if control['role_identity'] not in [role_identity(role) for role in spec['roles']]:
            raise CandidateMatchError('A/A role does not participate in this match')
    elif purpose != 'smoke':
        raise CandidateMatchError('invalid match purpose')


def stage(out: Path, label: str, command: list[str], spec: dict, limit: float, trace_path=None) -> None:
    command = supervisor_command(HELPERS / 'process_supervisor.py', label, out / label,
                                 limit, 10 * 1024 * 1024, ','.join(map(str, spec['cpus'])),
                                 command, 3, 1)
    process = subprocess.Popen(command, env=safe_environment())
    started = time.monotonic()
    cgroup = next((line.split(':', 2)[2] for line in Path('/proc/self/cgroup').read_text().splitlines()
                   if line.startswith('0::')), None)
    cgroup_path = Path('/sys/fs/cgroup') / cgroup.lstrip('/') if cgroup else None
    try:
        with (out / label / 'host-capacity.jsonl').open('x', buffering=1) as samples:
            while process.poll() is None:
                physical = os.statvfs('/mnt/c')
                free_gib = physical.f_bavail * physical.f_frsize / (1 << 30)
                memory = dict((line.split(':', 1)[0], line.split(':', 1)[1].strip())
                              for line in Path('/proc/meminfo').read_text().splitlines())
                sample = {'unix_time': time.time(), 'windows_free_gib': free_gib,
                          'memory_available_kib': int(memory['MemAvailable'].split()[0]),
                          'loadavg': Path('/proc/loadavg').read_text().strip(),
                          'cpu_stat': Path('/proc/stat').read_text().splitlines()[0]}
                if cgroup_path:
                    sample['cgroup_memory'] = {name: (cgroup_path / name).read_text().strip()
                                              for name in ('memory.current', 'memory.peak', 'memory.events')
                                              if (cgroup_path / name).is_file()}
                samples.write(json.dumps(sample) + '\n')
                if free_gib < spec.get('minimum_windows_free_gib', 25) or sample['memory_available_kib'] < 1 << 20:
                    raise CandidateMatchError('host capacity guard failed')
                if trace_path is not None and spec.get('trace_storage') == 'ram-gzip' and trace_path.exists() and \
                   trace_path.stat().st_size > spec['maximum_trace_bytes']:
                    raise CandidateMatchError('trace exceeds declared byte budget')
                if time.monotonic() - started > limit + 30:
                    raise CandidateMatchError('stage supervisor exceeded its outer deadline')
                try:
                    process.wait(timeout=30)
                except subprocess.TimeoutExpired:
                    pass
        if process.returncode:
            raise CandidateMatchError(f'{label} exited {process.returncode}')
    finally:
        if process.poll() is None:
            process.terminate()
            try:
                process.wait(timeout=15)
            except subprocess.TimeoutExpired:
                process.kill()
                process.wait()
    validate_supervisor_receipt(out / label / 'supervisor-receipt.json')


def preflight(launcher: Path, cwd: Path, options: list, role: dict, out: Path, cpu: int) -> dict:
    process = None
    with out.open('x', buffering=1) as transcript:
        try:
            process = subprocess.Popen(['taskset', '-c', str(cpu), str(launcher)], cwd=cwd, env=safe_environment(),
                                       stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
            counter55 = role['engine']['sha256'] == COUNTER55_SHA256
            protocol = CheckedProtocol(process, transcript, counter55)
            protocol.send('uci')
            uci_lines = protocol.until(lambda line: line == 'uciok', 30, 'uciok')
            advertised = parse_advertised_options(uci_lines)
            if role['engine']['sha256'] == MAELSTROM330_SHA256:
                validate_one_worker_role(role)
                validate_maelstrom_handshake(uci_lines, advertised)
            ngn = any(name == 'EvalBackend' for name, _ in options)
            if ngn:
                protocol.send('debug on')
                protocol.send('isready')
                protocol.until(lambda line: line == 'readyok', 30, 'debug ready')
            barriers = []
            for name, value in options:
                if name not in advertised:
                    raise CandidateMatchError(f'unadvertised option {name}')
                domain = advertised[name]
                if domain['type'] == 'spin' and not domain['min'] <= int(value) <= domain['max']:
                    raise CandidateMatchError(f'option {name} outside advertised domain')
                if domain['type'] == 'combo' and str(value) not in domain['vars']:
                    raise CandidateMatchError(f'option {name} outside advertised variants')
                text = value_text(value)
                protocol.send(f'setoption name {name} value {text}')
                protocol.send('isready')
                lines = protocol.until(lambda line: line == 'readyok', 30, name)
                if ngn and name in {'Threads', 'OwnBook', 'Hash', 'Move Overhead', 'K4EvalScale'}:
                    if lines.count(f'info string option set: {name} = {text}') != 1:
                        raise CandidateMatchError(f'{name} acknowledgement missing')
                barriers.append({'name': name, 'value': value, 'lines': lines})
            diagnostics = []
            if ngn:
                protocol.send('position startpos')
                protocol.send('eval')
                protocol.send('isready')
                diagnostics = [line for line in protocol.until(lambda line: line == 'readyok', 30, 'eval') if EVAL_RE.fullmatch(line)]
                backend = dict(options)['EvalBackend']
                if len(diagnostics) != 1 or EVAL_RE.fullmatch(diagnostics[0]).group(1) != backend:
                    raise CandidateMatchError('selected evaluator diagnostic differs')
            protocol.send('ucinewgame')
            protocol.send('isready')
            protocol.until(lambda line: line == 'readyok', 30, 'new game')
            protocol.send('position startpos')
            protocol.send('go depth 2')
            search = protocol.until(lambda line: line.startswith('bestmove '), 30, 'depth 2')
            if ngn and (not any(line.startswith('info depth 2 ') for line in search) or
                        any(re.fullmatch(r'info depth 1 score cp 50 nodes 1 time 0 nps 0 pv \S+', line) for line in search)):
                raise CandidateMatchError('book-free depth-2 witness failed')
            protocol.send('quit')
            if process.wait(timeout=30) or protocol.stderr_lines:
                raise CandidateMatchError('preflight exit/stderr failed')
            for thread in protocol.threads:
                thread.join(timeout=2)
                if thread.is_alive():
                    raise CandidateMatchError('preflight reader survived quit')
            if protocol.stderr_lines:
                raise CandidateMatchError('late preflight stderr')
            if counter55 and protocol.allowed_stderr_kinds != {'version', 'embedded_weights'}:
                raise CandidateMatchError('native Counter startup records absent')
            while not protocol.events.empty():
                stream, line, _ = protocol.events.get_nowait()
                if stream == 'stderr' and line != '<EOF>' or line.startswith('info string error'):
                    raise CandidateMatchError(f'late preflight error: {line}')
            return {'pass': True, 'uci_identity': [line for line in uci_lines if line.startswith('id ')],
                    'advertised': advertised, 'barriers': barriers,
                    'diagnostics': diagnostics, 'search': search, 'ngn': ngn,
                    'allowed_exact_counter55_startup_stderr': protocol.allowed_stderr}
        finally:
            if process is not None and process.poll() is None:
                process.terminate()
                try:
                    process.wait(timeout=2)
                except subprocess.TimeoutExpired:
                    process.kill()
                    process.wait()


def run(spec_path: Path, out: Path) -> None:
    require_wsl()
    spec = json.loads(spec_path.read_text())
    current_instrument = instrument()
    if ('tc' in spec) == ('nodes' in spec) or len(spec['roles']) != 2:
        raise CandidateMatchError('need two roles and exactly one budget')
    if spec['roles'][0]['name'] == spec['roles'][1]['name'] or spec['pairs'] < 1:
        raise CandidateMatchError('need distinct role names and at least one pair')
    if spec['concurrency'] > len(spec['cpus']) or len(set(spec['cpus'])) != len(spec['cpus']):
        raise CandidateMatchError('invalid one-worker concurrency/affinity')
    for role in spec['roles']:
        validate_one_worker_role(role)
    admit_control(spec, current_instrument)
    out.mkdir(parents=True, exist_ok=False)
    atomic_json(out / 'spec.json', spec)
    state = {'state': 'PREPARING', 'started': time.time()}
    atomic_json(out / 'STATE.json', state)
    frozen = ownedstorage.stage_inputs(spec, out, quickmatch.checked)
    roles = []
    crash_reports = []
    for index, role in enumerate(spec['roles']):
        cwd = out / f'role{index}'
        cwd.mkdir()
        binary, options = stage_role_inputs(role, cwd)
        launcher = cwd / 'role_exec.py'
        shutil.copy2(HELPERS / 'role_exec.py', launcher)
        launcher.chmod(0o555)
        atomic_json(cwd / 'role-config.json', {'schema': 'ngn-candidate-role-exec-v1', 'engine': str(binary),
                    'engine_sha256': role['engine']['sha256'], 'gomaxprocs': '1', 'expected_cwd': str(cwd)})
        receipt = preflight(launcher, cwd, options, role, out / f'preflight{index}.jsonl', spec['cpus'][0])
        atomic_json(out / f'preflight{index}.json', receipt)
        if receipt['ngn']:
            crash = cwd / 'ngn_crashes.log'
            crash_reports.append(crash_lines(crash, crash, 1))
            shutil.move(crash, out / f'preflight{index}-crashes.log')
        roles.append({'id': str(index), 'name': role['name'], 'engine': str(binary), 'launcher': str(launcher),
                      'cwd': str(cwd), 'options': options, 'ngn': receipt['ngn'], 'sha256': role['engine']['sha256']})
    storage = ownedstorage.TraceStorage(out, spec)
    command = [str(frozen['fastchess'])]
    for role in roles:
        command += ['-engine', f'cmd={role["launcher"]}', f'name={role["name"]}', f'dir={role["cwd"]}']
        command += [f'option.{name}={value_text(value)}' for name, value in role['options']]
    budget = [f'tc={spec["tc"]}', 'timemargin=0'] if 'tc' in spec else [f'nodes={spec["nodes"]}']
    command += ['-openings', f'file={frozen["openings_pgn"]}', 'format=pgn', 'order=sequential', 'start=1',
                f'plies={spec["opening_plies"]}', '-each', *budget, '-rounds', str(spec['pairs']), '-games', '2',
                '-repeat', '-concurrency', str(spec['concurrency']), '-use-affinity', ','.join(map(str, spec['cpus'])),
                '-srand', str(spec['seed']), *(['-strict'] if spec.get('strict', True) else []), '-show-latency',
                '-pgnout', f'file={out / "games.pgn"}', 'notation=uci', 'append=false', 'nodes=true',
                'seldepth=true', 'nps=true', 'hashfull=true', 'timeleft=true', 'latency=true', 'pv=true',
                '-epdout', f'file={out / "final.epd"}', 'append=false', '-log', f'file={storage.path}',
                'level=trace', 'engine=true', 'realtime=false', 'append=false']
    atomic_json(out / 'command.json', command)
    stage_config = out / 'match-stage.json'
    atomic_json(stage_config, {'schema': 'ngn-candidate-match-stage-v1', 'command': command, 'cwd': str(out),
        'environment': safe_environment(), 'sample_interval_seconds': 1,
        'roles': [{'id': role['id'], 'engine': role['engine'], 'engine_sha256': role['sha256'], 'cwd': role['cwd'],
                   'gomaxprocs': '1', 'allowed_cpu_masks': list(map(str, spec['cpus'])), 'launcher': role['launcher']} for role in roles],
        'match_stdout': str(out / 'match.stdout'), 'match_stderr': str(out / 'match.stderr'),
        'witness': str(out / 'child-processes.json')})
    state['state'] = 'MATCH_RUNNING'
    atomic_json(out / 'STATE.json', state)
    with storage:
        stage(out, 'supervised-match', [sys.executable, str(HELPERS / 'run_match_stage.py'), '--config', str(stage_config)], spec, spec['limit_seconds'], storage.path)
    witness = json.loads((out / 'child-processes.json').read_text())
    if witness['state'] != 'COMPLETE' or witness['violations'] or (out / 'match.stderr').stat().st_size:
        raise CandidateMatchError('match process witness failed')
    trace_roles = [{'display_name': role['name'], 'resolved_options': [{'name': name, 'value': value_text(value)} for name, value in role['options']],
                    'expected_refreshes': 2 * spec['pairs'], 'expected_processes': len(witness['observed_instances'][role['id']])} for role in roles]
    trace_lines, trace_sha256 = ownedstorage.read_trace(out, spec)
    trace = operational_trace(trace_lines, trace_roles, roles)
    for role in roles:
        if role['ngn']:
            crash = Path(role['cwd']) / 'ngn_crashes.log'
            crash_reports.append(crash_lines(crash, crash, len(witness['observed_instances'][role['id']])))
    atomic_json(out / 'audit-operational.json', {'pass': True, 'trace': trace, 'crash_logs': crash_reports,
                                             'unfiltered_trace_sha256': trace_sha256})
    audit = [sys.executable, str(frozen['auditor']), '--pgn', str(out / 'games.pgn'), '--final-epd', str(out / 'final.epd'),
             '--opening-prefixes', str(frozen['opening_prefixes']), '--opening-prefixes-sha256', spec['opening_prefixes']['sha256'],
             '--opening-pgn', str(frozen['openings_pgn']), '--opening-pgn-sha256', spec['openings_pgn']['sha256'],
             '--stockfish', str(frozen['stockfish']), '--stockfish-sha256', spec['stockfish']['sha256'],
             '--engine-a', roles[0]['name'], '--engine-b', roles[1]['name'], '--games', str(2 * spec['pairs']),
             '--pairs', str(spec['pairs']), '--output', str(out / 'audit.json')]
    state['state'] = 'AUDIT_RUNNING'
    atomic_json(out / 'STATE.json', state)
    stage(out, 'supervised-audit', audit, spec, 1800)
    if spec['purpose'] == 'smoke':
        _, audit_result = quickmatch.pair_scores(out)
        report = {'schema': 'ngn-owned-instrument-smoke-v1', 'purpose': 'smoke', 'pass': True,
                  'games': audit_result['games'], 'pairs': audit_result['pairs'],
                  'legal_plies': audit_result['legal_plies'], 'audit_sha256': sha256(out / 'audit.json'),
                  'pgn_sha256': audit_result['pgn_sha256']}
        atomic_json(out / 'report.json', report)
        print(json.dumps(report))
    else:
        quickmatch.report(out, None)
        report = json.loads((out / 'report.json').read_text())
    if spec['purpose'] == 'aa' and not report['score_interval_95'][0] <= .5 <= report['score_interval_95'][1]:
        raise CandidateMatchError('A/A interval excludes parity')
    control = {'purpose': spec['purpose'], 'pass': True, 'instrument': current_instrument,
               'configuration': configuration(spec), 'role_identity': role_identity(spec['roles'][0]),
               'report_sha256': sha256(out / 'report.json'), 'audit_sha256': sha256(out / 'audit.json'),
               'operational_sha256': sha256(out / 'audit-operational.json')}
    atomic_json(out / 'control.json', control)
    state.update(state='COMPLETE', completed=time.time())
    atomic_json(out / 'STATE.json', state)


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--spec', type=Path, required=True)
    parser.add_argument('--out', type=Path, required=True)
    args = parser.parse_args()
    try:
        run(args.spec.resolve(), args.out.resolve())
    except Exception as error:
        if args.out.is_dir():
            atomic_json(args.out / 'STATE.json', {'state': 'FAILED', 'error': str(error), 'completed': time.time()})
        raise


if __name__ == '__main__':
    main()
