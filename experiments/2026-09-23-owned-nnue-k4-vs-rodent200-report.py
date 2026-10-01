#!/usr/bin/env python3
"""Audit and report the fixed owned-K4 versus borrowed-Rodent match."""

from __future__ import annotations

import argparse
import hashlib
import json
import math
import random
from pathlib import Path

GAMES = 200
PAIRS = 100
SEED = 2026092301
RESAMPLES = 100_000
HELD_SHA256 = '60e7df9d8c4eefefaa2fb15d28a718803430c02ec4a78ed1129d0ceacb60f919'
REVIEW_SUBJECT = '988c40972ea671e1dc3baaf42a366633d3732a53845fdf5f3fb3c1a4d5b39724'
K4_SHA256 = 'cebec29cbab676af6a2ade2b67da5c9a01ddb7882ed604238c0e7d67893f72d6'
RODENT_SHA256 = '5f7480b56538e9e64ee02fb5b4842bf0394ee44b707dae3a63ec7f902abd0afb'


def sha256(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def elo(score: float) -> float | str:
    if score <= 0:
        return '-infinity'
    if score >= 1:
        return 'infinity'
    return -400 * math.log10(1 / score - 1)


def percentile(sorted_values: list[float], quantile: float) -> float:
    index = quantile * (len(sorted_values) - 1)
    lower = math.floor(index)
    upper = math.ceil(index)
    fraction = index - lower
    return sorted_values[lower] * (1 - fraction) + sorted_values[upper] * fraction


def pair_scores(audit: dict, pairs: int) -> list[float]:
    counts = audit['penta_0_to_4']
    if not isinstance(counts, list) or len(counts) != 5 or any(type(x) is not int or x < 0 for x in counts):
        raise ValueError('invalid penta outcome counts')
    if sum(counts) != pairs:
        raise ValueError('penta count differs from opening pairs')
    points = sum(half_points * count for half_points, count in enumerate(counts)) / 2
    if points != audit['engine_a_points']:
        raise ValueError('penta orientation disagrees with engine-A points')
    return [half_points / 4 for half_points, count in enumerate(counts) for _ in range(count)]


def bootstrap(scores: list[float]) -> dict:
    rng = random.Random(SEED)
    size = len(scores)
    samples = sorted(
        sum(scores[rng.randrange(size)] for _ in range(size)) / size
        for _ in range(RESAMPLES)
    )
    mean = sum(scores) / size
    low, high = percentile(samples, 0.025), percentile(samples, 0.975)
    return {
        'method': 'paired nonparametric percentile bootstrap by opening pair',
        'seed': SEED,
        'resamples': RESAMPLES,
        'score': mean,
        'score_interval_95': [low, high],
        'relative_elo': elo(mean),
        'relative_elo_interval_95': [elo(low), elo(high)],
    }


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--held', type=Path, required=True)
    parser.add_argument('--setup', type=Path, required=True)
    parser.add_argument('--audit', type=Path, required=True)
    parser.add_argument('--operational', type=Path, required=True)
    parser.add_argument('--terminal', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    if args.output.exists() or sha256(args.held) != HELD_SHA256:
        raise ValueError('output exists or HELD manifest hash mismatch')
    held = json.loads(args.held.read_text())
    setup = json.loads(args.setup.read_text())
    audit = json.loads(args.audit.read_text())
    operational = json.loads(args.operational.read_text())
    terminal = json.loads(args.terminal.read_text())

    if held['schema'] != 'ngn-candidate-match-borrowed-v1' or held['status'] != 'HELD':
        raise ValueError('borrowed match contract mismatch')
    if held['match']['games'] != GAMES or held['match']['pairs'] != PAIRS or held['match']['time_control'] != '10+0.1':
        raise ValueError('game count or clock mismatch')
    roles = held['inputs']['roles']
    if [role['id'] for role in roles] != ['k4', 'rodent']:
        raise ValueError('role order differs from engine-A K4 assumption')
    if roles[0]['network']['sha256'] != K4_SHA256 or roles[1]['network']['sha256'] != RODENT_SHA256:
        raise ValueError('evaluator model identity mismatch')
    if setup['held_manifest_sha256'] != HELD_SHA256 or setup['review_subject_sha256'] != REVIEW_SUBJECT:
        raise ValueError('setup receipt mismatch')
    if setup['new_indices'] != [4241, 4340] or setup['prefix_sets_disjoint'] is not True:
        raise ValueError('opening set mismatch')
    if audit['status'] != 'PASS' or audit['games'] != GAMES or audit['pairs'] != PAIRS:
        raise ValueError('independent chess audit incomplete')
    if audit['probable_embedded_book_signature_plies'] != 0:
        raise ValueError('embedded-book signature found')
    if operational['state'] != 'COMPLETE' or operational['pass'] is not True:
        raise ValueError('operational audit incomplete')
    if terminal['state'] != 'COMPLETE' or terminal['games'] != GAMES or terminal['pairs'] != PAIRS:
        raise ValueError('terminal receipt incomplete')
    if terminal['review_subject_sha256'] != REVIEW_SUBJECT:
        raise ValueError('terminal review subject mismatch')
    if terminal['operational_pass'] is not True or terminal['independent_chess_audit_pass'] is not True:
        raise ValueError('terminal audits failed')

    scores = pair_scores(audit, PAIRS)
    interval = bootstrap(scores)
    if abs(interval['score'] * GAMES - audit['engine_a_points']) > 1e-9:
        raise ValueError('K4 score differs from chess audit')
    report = {
        'schema': 'ngn-k4-vs-rodent200-report-v1',
        'held_sha256': HELD_SHA256,
        'chess_audit_sha256': sha256(args.audit),
        'operational_audit_sha256': sha256(args.operational),
        'terminal_sha256': sha256(args.terminal),
        'games': GAMES,
        'pairs': PAIRS,
        'k4_points': audit['engine_a_points'],
        'k4_pair_halfpoint_counts': audit['penta_0_to_4'],
        'bootstrap': interval,
        'absolute_rating_3k': 'NOT_ESTABLISHED',
    }
    args.output.write_text(json.dumps(report, indent=2) + '\n')
    print(json.dumps(report, indent=2))


if __name__ == '__main__':
    main()
