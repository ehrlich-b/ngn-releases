#!/usr/bin/env python3
"""Report the audited fixed-node match and paired difference from 10+0.1."""

from __future__ import annotations

import argparse
import collections
import hashlib
import json
import math
import random
import re
import tarfile
from pathlib import Path

GAMES = 200
PAIRS = 100
RESAMPLES = 100_000
HELD_SHA256 = 'c8ebf895a25b144813b977ab9523f46113e3ec4f5ac0ae6eb4bc624511c328ce'
REVIEW_SUBJECT = '0b602e6b7026e1477b9da2beba405b3d7fa92d6f6cbeaa0a276b8a209fd144a2'
CLOCK_AUDIT_SHA256 = '5a491ff7186ef3efeee41ae6e5808b9c2e5e8a8901a0202683e1ce816900bc9e'
CLOCK_ARCHIVE_SHA256 = '7262da90d008d39456778628b7aaeddbfc829d7955c9f7f5995d463ca19c4507'
K4_SHA256 = 'cebec29cbab676af6a2ade2b67da5c9a01ddb7882ed604238c0e7d67893f72d6'
RODENT_SHA256 = '5f7480b56538e9e64ee02fb5b4842bf0394ee44b707dae3a63ec7f902abd0afb'
UCI_MOVE = re.compile(r'\b([a-h][1-8][a-h][1-8][qrbn]?) \{')


def sha256(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def percentile(sorted_values: list[float], quantile: float) -> float:
    index = quantile * (len(sorted_values) - 1)
    lower = math.floor(index)
    upper = math.ceil(index)
    return sorted_values[lower] * (upper - index) + sorted_values[upper] * (index - lower) if upper != lower else sorted_values[lower]


def elo(score: float) -> float | str:
    if score <= 0:
        return '-infinity'
    if score >= 1:
        return 'infinity'
    return -400 * math.log10(1 / score - 1)


def opening_ids(pgn: bytes) -> list[tuple[str, ...]]:
    games = ['[Event "Fastchess Tournament"]' + text
             for text in pgn.decode().split('[Event "Fastchess Tournament"]') if text.strip()]
    if len(games) != GAMES:
        raise ValueError('PGN game count differs')
    result = []
    for game in games:
        moves = UCI_MOVE.findall(game.split('\n\n', 1)[1])
        if len(moves) < 6:
            raise ValueError('PGN game shorter than six opening plies')
        result.append(tuple(moves[:6]))
    return result


def pair_scores(audit: dict, pgn: bytes, k4_name: str, rodent_name: str) -> dict[tuple[str, ...], float]:
    if audit['status'] != 'PASS' or audit['games'] != GAMES or audit['pairs'] != PAIRS:
        raise ValueError('chess audit incomplete')
    if audit['probable_embedded_book_signature_plies'] != 0:
        raise ValueError('embedded-book signature present')
    games = audit['game_audit']
    if len(games) != GAMES:
        raise ValueError('incomplete per-game audit')
    if hashlib.sha256(pgn).hexdigest() != audit['pgn_sha256']:
        raise ValueError('PGN differs from chess audit')
    openings = opening_ids(pgn)
    by_opening: dict[tuple[str, ...], list[tuple[dict, float]]] = collections.defaultdict(list)
    for index, (game, opening) in enumerate(zip(games, openings), 1):
        if game['game'] != index or {game['white'], game['black']} != {k4_name, rodent_name}:
            raise ValueError('game order or roles differ')
        if game['result'] == '1/2-1/2':
            points = 0.5
        elif game['result'] == '1-0':
            points = float(game['white'] == k4_name)
        elif game['result'] == '0-1':
            points = float(game['black'] == k4_name)
        else:
            raise ValueError('unknown game result')
        by_opening[opening].append((game, points))
    if len(by_opening) != PAIRS:
        raise ValueError('opening-pair count differs')
    scores = {}
    for opening, pair in by_opening.items():
        if len(pair) != 2 or pair[0][0]['white'] != pair[1][0]['black'] or pair[0][0]['black'] != pair[1][0]['white']:
            raise ValueError('opening does not have reversed-color pair')
        scores[opening] = (pair[0][1] + pair[1][1]) / 2
    if sum(scores.values()) * 2 != audit['engine_a_points']:
        raise ValueError('pair points disagree with chess audit')
    counts = [sum(score * 4 == i for score in scores.values()) for i in range(5)]
    if counts != audit['penta_0_to_4']:
        raise ValueError('pair outcomes disagree with penta counts')
    return scores


def bootstrap(scores: list[float], seed: int) -> list[float]:
    rng = random.Random(seed)
    size = len(scores)
    samples = sorted(sum(scores[rng.randrange(size)] for _ in range(size)) / size for _ in range(RESAMPLES))
    return [percentile(samples, 0.025), percentile(samples, 0.975)]


def outcomes(audit: dict, k4_name: str) -> dict[str, int]:
    result = {'wins': 0, 'draws': 0, 'losses': 0}
    for game in audit['game_audit']:
        if game['result'] == '1/2-1/2':
            result['draws'] += 1
        elif (game['result'] == '1-0' and game['white'] == k4_name) or (game['result'] == '0-1' and game['black'] == k4_name):
            result['wins'] += 1
        else:
            result['losses'] += 1
    if result['wins'] + result['draws'] + result['losses'] != GAMES:
        raise ValueError('outcome count differs')
    if result['wins'] + result['draws'] / 2 != audit['engine_a_points']:
        raise ValueError('outcome points differ from audit')
    return result


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--held', type=Path, required=True)
    parser.add_argument('--audit', type=Path, required=True)
    parser.add_argument('--operational', type=Path, required=True)
    parser.add_argument('--terminal', type=Path, required=True)
    parser.add_argument('--clock-audit', type=Path, required=True)
    parser.add_argument('--clock-archive', type=Path, required=True)
    parser.add_argument('--fixed-pgn', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    if args.output.exists() or sha256(args.held) != HELD_SHA256:
        raise ValueError('output exists or HELD manifest mismatch')
    if sha256(args.clock_audit) != CLOCK_AUDIT_SHA256:
        raise ValueError('clock audit hash mismatch')
    if sha256(args.clock_archive) != CLOCK_ARCHIVE_SHA256:
        raise ValueError('clock archive hash mismatch')
    with tarfile.open(args.clock_archive) as archive:
        clock_pgn = archive.extractfile('games.pgn').read()
    fixed_pgn = args.fixed_pgn.read_bytes()
    held = json.loads(args.held.read_text())
    audit = json.loads(args.audit.read_text())
    operational = json.loads(args.operational.read_text())
    terminal = json.loads(args.terminal.read_text())
    clock = json.loads(args.clock_audit.read_text())
    if held['schema'] != 'ngn-candidate-match-borrowed-fixed-nodes-v1' or held['status'] != 'HELD':
        raise ValueError('fixed-node contract mismatch')
    if held['match']['games'] != GAMES or held['match']['pairs'] != PAIRS or held['match']['node_limit'] != 160_000:
        raise ValueError('game count or node limit mismatch')
    if [role['id'] for role in held['inputs']['roles']] != ['k4', 'rodent']:
        raise ValueError('role order mismatch')
    roles = held['inputs']['roles']
    if [role['network']['sha256'] for role in roles] != [K4_SHA256, RODENT_SHA256]:
        raise ValueError('evaluator identity mismatch')
    if audit['opening_pgn_sha256'] != held['inputs']['opening_pgn']['sha256'] or clock['opening_pgn_sha256'] != audit['opening_pgn_sha256']:
        raise ValueError('opening input mismatch between conditions')
    if operational['state'] != 'COMPLETE' or operational['pass'] is not True:
        raise ValueError('operational audit incomplete')
    if terminal['state'] != 'COMPLETE' or terminal['games'] != GAMES or terminal['pairs'] != PAIRS:
        raise ValueError('terminal receipt incomplete')
    if terminal['review_subject_sha256'] != REVIEW_SUBJECT or terminal['operational_pass'] is not True or terminal['independent_chess_audit_pass'] is not True:
        raise ValueError('terminal binding or audits failed')
    names = [role['display_name'] for role in roles]
    fixed_scores = pair_scores(audit, fixed_pgn, *names)
    clock_scores = pair_scores(clock, clock_pgn, *names)
    if fixed_scores.keys() != clock_scores.keys():
        raise ValueError('opening pairs differ between conditions')
    fixed_interval = bootstrap(list(fixed_scores.values()), 2026092302)
    delta_scores = [fixed_scores[key] - clock_scores[key] for key in sorted(fixed_scores)]
    delta_interval = bootstrap(delta_scores, 2026092303)
    fixed_score = sum(fixed_scores.values()) / PAIRS
    clock_score = sum(clock_scores.values()) / PAIRS
    report = {
        'schema': 'ngn-k4-vs-rodent200-fixednodes-report-v1',
        'held_sha256': HELD_SHA256,
        'chess_audit_sha256': sha256(args.audit),
        'operational_audit_sha256': sha256(args.operational),
        'terminal_sha256': sha256(args.terminal),
        'clock_audit_sha256': CLOCK_AUDIT_SHA256,
        'games': GAMES,
        'pairs': PAIRS,
        'fixed_node_limit_per_move': 160_000,
        'fixed_nodes_k4_outcomes': outcomes(audit, names[0]),
        'fixed_nodes_k4_points': audit['engine_a_points'],
        'fixed_nodes_k4_score': fixed_score,
        'fixed_nodes_score_interval_95': fixed_interval,
        'fixed_nodes_relative_elo': elo(fixed_score),
        'fixed_nodes_relative_elo_interval_95': [elo(x) for x in fixed_interval],
        'clock_k4_score': clock_score,
        'fixed_minus_clock_score': fixed_score - clock_score,
        'fixed_minus_clock_score_interval_95': delta_interval,
        'bootstrap': {'method': 'nonparametric percentile, paired by exact six-ply opening prefix',
                      'score_seed': 2026092302, 'delta_seed': 2026092303,
                      'resamples': RESAMPLES},
        'absolute_rating_3k': 'NOT_ESTABLISHED',
    }
    args.output.write_text(json.dumps(report, indent=2) + '\n')
    print(json.dumps(report, indent=2))


if __name__ == '__main__':
    main()
