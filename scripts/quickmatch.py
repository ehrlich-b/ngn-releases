#!/usr/bin/env python3
"""Hash-checked fastchess match with independent chess audit and paired report.

A lighter instrument than scripts/candidate-match: the same fastchess argv
shape (sequential frozen openings, -repeat pairs, per-game affinity, strict,
trace log), the same independent chess auditor, and a report paired by round.

  quickmatch.py run --spec SPEC.json --out DIR
  quickmatch.py report --out DIR [--paired-with OTHER_DIR]

Spec keys: fastchess, auditor, stockfish, openings_pgn, opening_prefixes (each
{"path", "sha256"}), opening_plies, pairs, concurrency, cpus, seed, and exactly
one of tc / nodes; optional strict (default true) controls fastchess -strict.
roles: two entries {"name", "engine": {"path", "sha256"},
"options": [[UCI name, value], ...] sent in order, "network": {"path", "sha256"}
| null}; an option value "$NETWORK" becomes the checked network path. The first
role is the candidate; scores are reported from its side.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import math
import os
import random
import re
import subprocess
import sys
import time
from pathlib import Path

RESAMPLES = 20_000


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with open(path, 'rb') as handle:
        for block in iter(lambda: handle.read(1 << 20), b''):
            digest.update(block)
    return digest.hexdigest()


def checked(item: dict) -> Path:
    path = Path(item['path'])
    actual = sha256(path)
    if actual != item['sha256']:
        raise SystemExit(f'hash mismatch {path}: {actual} != {item["sha256"]}')
    return path


def run(spec_path: Path, out: Path) -> None:
    spec = json.loads(spec_path.read_text())
    out.mkdir(parents=True, exist_ok=False)
    (out / 'spec.json').write_text(json.dumps(spec, indent=2) + '\n')
    for key in ('fastchess', 'auditor', 'stockfish', 'openings_pgn', 'opening_prefixes'):
        checked(spec[key])
    if ('tc' in spec) == ('nodes' in spec):
        raise SystemExit('spec needs exactly one of tc or nodes')
    roles = spec['roles']
    if len(roles) != 2 or roles[0]['name'] == roles[1]['name']:
        raise SystemExit('need two distinctly named roles')
    command = [spec['fastchess']['path']]
    for index, role in enumerate(roles):
        engine = checked(role['engine'])
        network = str(checked(role['network'])) if role.get('network') else None
        options = [(name, network if value == '$NETWORK' else value) for name, value in role['options']]
        if None in (value for _, value in options):
            raise SystemExit('$NETWORK used without a network')
        role_dir = out / f'role{index}'
        role_dir.mkdir()
        launcher = role_dir / 'engine.sh'
        launcher.write_text(f'#!/bin/sh\nexec env GOMAXPROCS=1 {engine}\n')
        launcher.chmod(0o555)
        command += ['-engine', f'cmd={launcher}', f'name={role["name"]}', f'dir={role_dir}']
        command += [f'option.{name}={str(value).lower() if isinstance(value, bool) else value}' for name, value in options]
    budget = [f'tc={spec["tc"]}', 'timemargin=0'] if 'tc' in spec else [f'nodes={spec["nodes"]}']
    command += [
        '-openings', f'file={spec["openings_pgn"]["path"]}', 'format=pgn', 'order=sequential', 'start=1',
        f'plies={spec["opening_plies"]}',
        '-each', *budget,
        '-rounds', str(spec['pairs']), '-games', '2', '-repeat', '-concurrency', str(spec['concurrency']),
        '-use-affinity', ','.join(str(cpu) for cpu in spec['cpus']),
        '-srand', str(spec['seed']), *(['-strict'] if spec.get('strict', True) else []), '-show-latency',
        '-pgnout', f'file={out / "games.pgn"}', 'notation=uci', 'append=false', 'nodes=true', 'seldepth=true',
        'nps=true', 'hashfull=true', 'timeleft=true', 'latency=true', 'pv=true',
        '-epdout', f'file={out / "final.epd"}', 'append=false',
        '-log', f'file={out / "fastchess.log"}', 'level=warn', 'engine=false', 'realtime=false', 'append=false',
    ]
    (out / 'command.json').write_text(json.dumps(command, indent=2) + '\n')
    state = {'state': 'MATCH_RUNNING', 'started': time.time()}
    (out / 'STATE.json').write_text(json.dumps(state) + '\n')
    with open(out / 'match.stdout', 'wb') as stdout, open(out / 'match.stderr', 'wb') as stderr:
        code = subprocess.run(command, cwd=out, stdout=stdout, stderr=stderr).returncode
    state.update(match_returncode=code, match_seconds=time.time() - state['started'])
    if code != 0:
        state['state'] = 'MATCH_FAILED'
        (out / 'STATE.json').write_text(json.dumps(state) + '\n')
        raise SystemExit(f'fastchess exited {code}')
    audit = [
        sys.executable, spec['auditor']['path'], '--pgn', str(out / 'games.pgn'), '--final-epd', str(out / 'final.epd'),
        '--opening-prefixes', spec['opening_prefixes']['path'], '--opening-prefixes-sha256', spec['opening_prefixes']['sha256'],
        '--opening-pgn', spec['openings_pgn']['path'], '--opening-pgn-sha256', spec['openings_pgn']['sha256'],
        '--stockfish', spec['stockfish']['path'], '--stockfish-sha256', spec['stockfish']['sha256'],
        '--engine-a', roles[0]['name'], '--engine-b', roles[1]['name'],
        '--games', str(2 * spec['pairs']), '--pairs', str(spec['pairs']), '--output', str(out / 'audit.json'),
    ]
    state['state'] = 'AUDIT_RUNNING'
    (out / 'STATE.json').write_text(json.dumps(state) + '\n')
    with open(out / 'audit.stdout', 'wb') as stdout, open(out / 'audit.stderr', 'wb') as stderr:
        code = subprocess.run(audit, cwd=out, stdout=stdout, stderr=stderr).returncode
    state.update(audit_returncode=code, state='COMPLETE' if code == 0 else 'AUDIT_FAILED')
    (out / 'STATE.json').write_text(json.dumps(state) + '\n')
    report(out, None)


def pair_scores(out: Path) -> tuple[dict[int, float], dict]:
    spec = json.loads((out / 'spec.json').read_text())
    audit = json.loads((out / 'audit.json').read_text())
    pgn = (out / 'games.pgn').read_bytes()
    if audit['status'] != 'PASS' or audit['pgn_sha256'] != hashlib.sha256(pgn).hexdigest():
        raise SystemExit('audit did not pass or PGN differs from audit')
    if audit['probable_embedded_book_signature_plies'] != 0:
        raise SystemExit('embedded-book signature present')
    rounds = [int(value) for value in re.findall(rb'^\[Round "(\d+)"\]', pgn, re.M)]
    games = audit['game_audit']
    if len(rounds) != len(games) or len(games) != 2 * spec['pairs']:
        raise SystemExit('game count differs')
    candidate = spec['roles'][0]['name']
    by_round: dict[int, list[tuple[dict, float]]] = {}
    for game, round_id in zip(games, rounds):
        if game['result'] == '1/2-1/2':
            points = 0.5
        elif game['result'] == '1-0':
            points = float(game['white'] == candidate)
        elif game['result'] == '0-1':
            points = float(game['black'] == candidate)
        else:
            raise SystemExit(f'unknown result {game["result"]}')
        by_round.setdefault(round_id, []).append((game, points))
    scores = {}
    for round_id, pair in by_round.items():
        if len(pair) != 2 or pair[0][0]['white'] != pair[1][0]['black']:
            raise SystemExit(f'round {round_id} is not a reversed-color pair')
        scores[round_id] = (pair[0][1] + pair[1][1]) / 2
    if len(scores) != spec['pairs'] or abs(sum(scores.values()) * 2 - audit['engine_a_points']) > 1e-9:
        raise SystemExit('pair scores disagree with audit')
    return scores, audit


def elo(score: float) -> float:
    score = min(max(score, 1e-6), 1 - 1e-6)
    return -400 * math.log10(1 / score - 1)


def bootstrap(values: list[float], seed: int) -> list[float]:
    rng = random.Random(seed)
    n = len(values)
    means = sorted(sum(values[rng.randrange(n)] for _ in range(n)) / n for _ in range(RESAMPLES))
    return [means[int(0.025 * RESAMPLES)], means[int(0.975 * RESAMPLES) - 1]]


def report(out: Path, paired_with: Path | None) -> None:
    scores, audit = pair_scores(out)
    values = [scores[key] for key in sorted(scores)]
    mean = sum(values) / len(values)
    interval = bootstrap(values, 2026092701)
    result = {
        'schema': 'ngn-quickmatch-report-v1',
        'candidate': json.loads((out / 'spec.json').read_text())['roles'][0]['name'],
        'games': audit['games'], 'pairs': audit['pairs'], 'legal_plies': audit['legal_plies'],
        'results': audit['results'], 'reasons': audit['reasons'],
        'penta_0_to_4': audit['penta_0_to_4'],
        'score': mean, 'score_interval_95': interval,
        'elo': elo(mean), 'elo_interval_95': [elo(x) for x in interval],
        'audit_sha256': sha256(out / 'audit.json'), 'pgn_sha256': audit['pgn_sha256'],
        'bootstrap': f'percentile over pair scores, {RESAMPLES} resamples, seed 2026092701',
    }
    if paired_with is not None:
        other, _ = pair_scores(paired_with)
        if other.keys() != scores.keys():
            raise SystemExit('paired runs have different rounds')
        delta = [scores[key] - other[key] for key in sorted(scores)]
        result['paired_with'] = str(paired_with)
        result['paired_delta_score'] = sum(delta) / len(delta)
        result['paired_delta_interval_95'] = bootstrap(delta, 2026092702)
    name = 'report.json' if paired_with is None else f'report-paired-{paired_with.name}.json'
    (out / name).write_text(json.dumps(result, indent=2) + '\n')
    print(json.dumps(result, indent=2))


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    sub = parser.add_subparsers(dest='command', required=True)
    run_parser = sub.add_parser('run')
    run_parser.add_argument('--spec', type=Path, required=True)
    run_parser.add_argument('--out', type=Path, required=True)
    report_parser = sub.add_parser('report')
    report_parser.add_argument('--out', type=Path, required=True)
    report_parser.add_argument('--paired-with', type=Path)
    args = parser.parse_args()
    os.umask(0o022)
    if args.command == 'run':
        run(args.spec, args.out)
    else:
        report(args.out, args.paired_with)


if __name__ == '__main__':
    main()
