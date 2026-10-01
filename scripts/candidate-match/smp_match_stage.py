#!/usr/bin/env python3
"""Separate SMP process witness, including every observed OS thread's mask."""
import json
import os
import sys
from pathlib import Path

import run_match_stage as base
from common import CandidateMatchError, atomic_json

original_read = base.read_process
original_violations = base.observation_violations


def parse_mask(value):
    result = set()
    for part in value.split(','):
        ends = part.split('-')
        if len(ends) == 1:
            result.add(int(ends[0]))
        elif len(ends) == 2 and int(ends[0]) <= int(ends[1]):
            result.update(range(int(ends[0]), int(ends[1]) + 1))
        else:
            raise CandidateMatchError('invalid CPU mask')
    if not result or min(result) < 0:
        raise CandidateMatchError('invalid CPU mask')
    return result


def load_config(path):
    value = json.loads(path.read_text())
    keys = {'schema', 'command', 'cwd', 'environment', 'roles', 'sample_interval_seconds', 'match_stdout', 'match_stderr', 'witness'}
    if set(value) != keys or value['schema'] != 'ngn-smp-match-stage-v1':
        raise CandidateMatchError('invalid SMP stage config')
    if not isinstance(value['command'], list) or not value['command'] or any(not isinstance(x, str) or not x for x in value['command']):
        raise CandidateMatchError('invalid stage command')
    if not isinstance(value['environment'], dict) or any(not isinstance(k, str) or not isinstance(v, str) for k, v in value['environment'].items()):
        raise CandidateMatchError('invalid environment')
    if not isinstance(value['sample_interval_seconds'], (int, float)) or not 0 < value['sample_interval_seconds'] <= 1:
        raise CandidateMatchError('invalid sample interval')
    if not isinstance(value['roles'], list) or len(value['roles']) != 2 or len({x['id'] for x in value['roles']}) != 2:
        raise CandidateMatchError('need two distinct roles')
    for role in value['roles']:
        if set(role) != {'id', 'engine', 'engine_sha256', 'cwd', 'gomaxprocs', 'allowed_cpu_masks', 'launcher'}:
            raise CandidateMatchError('invalid role keys')
        if role['gomaxprocs'] not in ['1', '2', '4', '8'] or not isinstance(role['allowed_cpu_masks'], list) or not role['allowed_cpu_masks']:
            raise CandidateMatchError('invalid width/masks')
        masks = [parse_mask(mask) for mask in role['allowed_cpu_masks']]
        if any(len(mask) < int(role['gomaxprocs']) for mask in masks):
            raise CandidateMatchError('width exceeds CPU team')
    return value


def read_process(pid, initial):
    row = original_read(pid, initial)
    tasks = []
    for path in Path(f'/proc/{pid}/task').iterdir():
        if not path.name.isdigit():
            continue
        try:
            fields = (path / 'stat').read_text().rsplit(')', 1)[1].split()
            mask = next(line.split(':', 1)[1].strip() for line in (path / 'status').read_text().splitlines() if line.startswith('Cpus_allowed_list:'))
            tasks.append({'tid': int(path.name), 'cpus_allowed_list': mask,
                          'cpu_ticks': int(fields[11]) + int(fields[12]), 'last_processor': int(fields[36])})
        except (FileNotFoundError, ProcessLookupError):
            continue
    fields = Path(f'/proc/{pid}/stat').read_text().rsplit(')', 1)[1].split()
    row['cpu_ticks'] = int(fields[11]) + int(fields[12])
    row['tasks'] = tasks
    return row


def observation_violations(role, row):
    problems = original_violations(role, row)
    if not row['tasks']:
        problems.append('no observed OS threads')
    for task in row['tasks']:
        if task['cpus_allowed_list'] != row['cpus_allowed_list']:
            problems.append(f"thread {task['tid']} CPU mask differs from process team")
    return problems


base.load_config = load_config
base.read_process = read_process
base.observation_violations = observation_violations

if __name__ == '__main__':
    code = base.main()
    config = load_config(Path(sys.argv[sys.argv.index('--config') + 1]))
    path = Path(config['witness'])
    receipt = json.loads(path.read_text())
    receipt['schema'] = 'ngn-smp-child-process-witness-v1'
    receipt['clock_ticks_per_second'] = os.sysconf('SC_CLK_TCK')
    atomic_json(path, receipt)
    raise SystemExit(code)
