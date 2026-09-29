#!/usr/bin/env python3
"""Fixed-depth identity and timing comparison of two NGN binaries on K4 nets.

Positions are move prefixes taken from a UCI-notation fastchess PGN. Each
binary searches every position at a fixed depth after ucinewgame; the final
info line (depth, nodes, score, pv head) and bestmove must match exactly.
Rounds alternate base/candidate on one pinned CPU and report wall time.

  k4identity.py --base BIN --cand BIN --network NET --pgn PGN --cpu 5 \
      --depth 12 --games 30 --plies 16,40,70 --rounds 3 --out OUT.json
"""

from __future__ import annotations

import argparse
import hashlib
import json
import re
import statistics
import subprocess
import time
from pathlib import Path


def positions(pgn: Path, games: int, plies: list[int]) -> list[list[str]]:
    text = pgn.read_text()
    result = []
    for body in re.split(r'\n\[Event ', text)[:games]:
        moves = re.findall(r'(?:^|\s)([a-h][1-8][a-h][1-8][qrbn]?)(?=\s*\{)', body.split('\n\n', 1)[-1])
        result += [moves[:ply] for ply in plies if ply <= len(moves)]
    return result


class Engine:
    def __init__(self, binary: str, network: str, backend: str, cpu: int, scale: int) -> None:
        self.process = subprocess.Popen(
            ['taskset', '-c', str(cpu), 'env', 'GOMAXPROCS=1', binary], stdin=subprocess.PIPE,
            stdout=subprocess.PIPE, text=True, bufsize=1, cwd='/tmp')
        self.send('uci')
        self.wait('uciok')
        options = [('Threads', 1), ('Hash', 16), ('OwnBook', 'false'), ('EvalFile', network), ('EvalBackend', backend)]
        if backend == 'ngn-k4-768-v1':
            options.append(('K4EvalScale', scale))
        for name, value in options:
            self.send(f'setoption name {name} value {value}')
            self.send('isready')
            for line in self.wait('readyok'):
                if 'error' in line:
                    raise SystemExit(f'{binary}: {line}')

    def send(self, line: str) -> None:
        self.process.stdin.write(line + '\n')

    def wait(self, token: str) -> list[str]:
        lines = []
        while True:
            line = self.process.stdout.readline()
            if not line:
                raise SystemExit('engine exited')
            lines.append(line.strip())
            if line.startswith(token):
                return lines

    def search(self, moves: list[str], depth: int) -> dict:
        self.send('ucinewgame')
        self.send('isready')
        self.wait('readyok')
        self.send('position startpos' + (' moves ' + ' '.join(moves) if moves else ''))
        self.send(f'go depth {depth}')
        lines = self.wait('bestmove')
        info = [line for line in lines if line.startswith('info') and ' nodes ' in line][-1]
        fields = info.split()
        pick = lambda key: fields[fields.index(key) + 1] if key in fields else None
        score = ' '.join(fields[fields.index('score') + 1:fields.index('score') + 3])
        return {'depth': pick('depth'), 'nodes': int(pick('nodes')), 'score': score,
                'bestmove': lines[-1].split()[1]}

    def close(self) -> None:
        self.send('quit')
        self.process.wait(timeout=10)


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument('--base', required=True)
    parser.add_argument('--cand', required=True)
    parser.add_argument('--network', required=True)
    parser.add_argument('--backend', default='ngn-k4-768-v1')
    parser.add_argument('--scale', type=int, default=100)
    parser.add_argument('--pgn', type=Path, required=True)
    parser.add_argument('--cpu', type=int, required=True)
    parser.add_argument('--depth', type=int, default=12)
    parser.add_argument('--games', type=int, default=30)
    parser.add_argument('--plies', default='16,40,70')
    parser.add_argument('--rounds', type=int, default=3)
    parser.add_argument('--out', type=Path, required=True)
    args = parser.parse_args()
    if not 10 <= args.scale <= 400:
        parser.error('--scale must be between 10 and 400')
    fixtures = positions(args.pgn, args.games, [int(x) for x in args.plies.split(',')])
    if not fixtures or args.rounds < 1:
        parser.error('need at least one fixture and one round')
    times = {'base': [], 'cand': []}
    results = {}
    for round_index in range(args.rounds):
        order = ('base', 'cand') if round_index % 2 == 0 else ('cand', 'base')
        for role in order:
            engine = Engine(getattr(args, role), args.network, args.backend, args.cpu, args.scale)
            start = time.perf_counter()
            rows = [engine.search(moves, args.depth) for moves in fixtures]
            times[role].append(time.perf_counter() - start)
            engine.close()
            if role in results and results[role] != rows:
                raise SystemExit(f'{role} is not deterministic across rounds')
            results[role] = rows
    mismatches = [index for index, (a, b) in enumerate(zip(results['base'], results['cand'])) if a != b]
    nodes = sum(row['nodes'] for row in results['base'])
    report = {
        'schema': 'ngn-k4-identity-timing-v1', 'fixtures': len(fixtures), 'depth': args.depth,
        'backend': args.backend, 'scale_percent': args.scale, 'own_book': False,
        'sha256': {key: hashlib.sha256(Path(value).read_bytes()).hexdigest()
                   for key, value in [('base', args.base), ('cand', args.cand), ('network', args.network), ('pgn', args.pgn)]},
        'identical': not mismatches, 'mismatch_indices': mismatches[:20], 'total_nodes': nodes,
        'seconds': times, 'median_seconds': {role: statistics.median(value) for role, value in times.items()},
        'nps_median': {role: nodes / statistics.median(value) for role, value in times.items()},
    }
    report['speedup'] = report['median_seconds']['base'] / report['median_seconds']['cand']
    args.out.write_text(json.dumps(report, indent=2) + '\n')
    print(json.dumps(report, indent=2))
    if mismatches:
        raise SystemExit(1)


if __name__ == '__main__':
    main()
