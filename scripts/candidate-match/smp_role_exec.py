#!/usr/bin/env python3
"""Exec a hash-bound role on the CPU team selected by fastchess's slot anchor."""
import hashlib
import json
import os
import sys
from pathlib import Path


def main():
    try:
        config = json.loads(Path(__file__).with_name('role-config.json').read_text())
        if set(config) != {'schema', 'engine', 'engine_sha256', 'gomaxprocs', 'expected_cwd', 'cpu_teams'}:
            raise ValueError('unexpected role config keys')
        if config['schema'] != 'ngn-smp-role-exec-v1' or config['gomaxprocs'] not in ['1', '2', '4', '8']:
            raise ValueError('invalid schema/width')
        engine = Path(config['engine']).resolve(strict=True)
        if Path.cwd().resolve() != Path(config['expected_cwd']).resolve(strict=True):
            raise ValueError('cwd mismatch')
        if hashlib.sha256(engine.read_bytes()).hexdigest() != config['engine_sha256'] or not os.access(engine, os.X_OK):
            raise ValueError('engine hash/executable mismatch')
        teams = config['cpu_teams']
        if not teams or any(not isinstance(team, list) or len(set(team)) != len(team) or
                            any(type(cpu) is not int or cpu < 0 for cpu in team) or
                            len(team) < int(config['gomaxprocs']) for team in teams):
            raise ValueError('invalid CPU teams')
        flat = [cpu for team in teams for cpu in team]
        if len(set(flat)) != len(flat):
            raise ValueError('overlapping CPU teams')
        inherited = os.sched_getaffinity(0)
        matches = [team for team in teams if inherited == {team[0]} or inherited == set(team)]
        if len(matches) != 1:
            raise ValueError('inherited affinity is not a declared slot/team')
        os.sched_setaffinity(0, matches[0])
        environment = dict(os.environ, GOMAXPROCS=config['gomaxprocs'])
        os.execve(engine, [str(engine)], environment)
    except Exception as error:
        print('SMP role launcher error: ' + str(error), file=sys.stderr, flush=True)
        raise SystemExit(111)


if __name__ == '__main__':
    main()
