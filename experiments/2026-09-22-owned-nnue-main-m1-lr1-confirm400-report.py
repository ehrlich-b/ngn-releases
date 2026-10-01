#!/usr/bin/env python3
"""Report the frozen 400-game K4 milestone from complete independent audits."""

from __future__ import annotations

import argparse
import hashlib
import json
import math
import random
from pathlib import Path

GAMES = 400
PAIRS = 200
SEED = 2026092201
RESAMPLES = 100_000
HELD_SHA256 = '8054dac3d04f0d71f621bc18792a2167d9d1d6830159372f915e262c565b4536'
K4_SHA256 = 'cebec29cbab676af6a2ade2b67da5c9a01ddb7882ed604238c0e7d67893f72d6'
REVIEW_SUBJECT = '59e35609c7b3d1ef57d49c8fcdb6d9bc42069ef44c476f005745ba617fc4df7f'


def sha256(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def percentile(sorted_values: list[float], quantile: float) -> float:
    index = quantile * (len(sorted_values) - 1)
    lower = math.floor(index)
    upper = math.ceil(index)
    fraction = index - lower
    return sorted_values[lower] * (1 - fraction) + sorted_values[upper] * fraction


def elo(score: float) -> float | str:
    if score <= 0:
        return '-infinity'
    if score >= 1:
        return 'infinity'
    return -400 * math.log10(1 / score - 1)


def k4_pair_scores(audit: dict, pairs: int) -> list[float]:
    counts = audit['penta_0_to_4']
    if not isinstance(counts, list) or len(counts) != 5 or any(type(x) is not int or x < 0 for x in counts):
        raise ValueError('invalid penta outcome counts')
    if sum(counts) != pairs:
        raise ValueError('penta count differs from opening pairs')
    hce_points = sum(half_points * count for half_points, count in enumerate(counts)) / 2
    if hce_points != audit['engine_a_points']:
        raise ValueError('penta orientation disagrees with engine-A points')
    return [(4 - half_points) / 4 for half_points, count in enumerate(counts) for _ in range(count)]


def bootstrap(scores: list[float]) -> dict:
    rng = random.Random(SEED)
    size = len(scores)
    samples = sorted(
        sum(scores[rng.randrange(size)] for _ in range(size)) / size
        for _ in range(RESAMPLES)
    )
    mean = sum(scores) / size
    lower = percentile(samples, 0.025)
    upper = percentile(samples, 0.975)
    return {
        'method': 'paired nonparametric percentile bootstrap by opening pair',
        'seed': SEED,
        'resamples': RESAMPLES,
        'score': mean,
        'score_interval_95': [lower, upper],
        'elo': elo(mean),
        'elo_interval_95': [elo(lower), elo(upper)],
        'lower_above_half': lower > 0.5,
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
    if args.output.exists():
        raise ValueError(f'refusing existing report {args.output}')
    if sha256(args.held) != HELD_SHA256:
        raise ValueError('HELD manifest hash mismatch')
    held = json.loads(args.held.read_text())
    setup = json.loads(args.setup.read_text())
    audit = json.loads(args.audit.read_text())
    operational = json.loads(args.operational.read_text())
    terminal = json.loads(args.terminal.read_text())

    if held['status'] != 'HELD' or held['match']['games'] != GAMES or held['match']['pairs'] != PAIRS:
        raise ValueError('match contract mismatch')
    if held['match']['time_control'] != '30+0.3' or held['match']['concurrency'] != 2:
        raise ValueError('clock or worker contract mismatch')
    if [role['id'] for role in held['inputs']['roles']] != ['hce', 'k4']:
        raise ValueError('role order differs from engine-A HCE assumption')
    if held['inputs']['roles'][1]['network']['sha256'] != K4_SHA256:
        raise ValueError('K4 model identity mismatch')
    if setup['held_manifest_sha256'] != HELD_SHA256 or setup['review_subject_sha256'] != REVIEW_SUBJECT:
        raise ValueError('setup receipt differs from frozen subject')
    if setup['new_indices'] != [4041, 4240] or setup['prefix_sets_disjoint'] is not True:
        raise ValueError('opening set differs from frozen setup')
    if audit['status'] != 'PASS' or audit['games'] != GAMES or audit['pairs'] != PAIRS:
        raise ValueError('independent chess audit is incomplete')
    if audit['probable_embedded_book_signature_plies'] != 0:
        raise ValueError('embedded-book signature found')
    if operational['state'] != 'COMPLETE' or operational['pass'] is not True:
        raise ValueError('operational audit is incomplete')
    if terminal['state'] != 'COMPLETE' or terminal['games'] != GAMES or terminal['pairs'] != PAIRS:
        raise ValueError('match terminal receipt is incomplete')
    if terminal['review_subject_sha256'] != REVIEW_SUBJECT:
        raise ValueError('terminal review subject mismatch')
    if terminal['operational_pass'] is not True or terminal['independent_chess_audit_pass'] is not True:
        raise ValueError('terminal audits failed')

    scores = k4_pair_scores(audit, PAIRS)
    result = bootstrap(scores)
    if abs(result['score'] * GAMES - (GAMES - audit['engine_a_points'])) > 1e-9:
        raise ValueError('K4 score differs from audited engine-A score')
    report = {
        'schema': 'ngn-k4-main20m-confirm400-report-v1',
        'held_sha256': HELD_SHA256,
        'setup_sha256': sha256(args.setup),
        'chess_audit_sha256': sha256(args.audit),
        'operational_audit_sha256': sha256(args.operational),
        'terminal_sha256': sha256(args.terminal),
        'k4_model_sha256': K4_SHA256,
        'games': GAMES,
        'pairs': PAIRS,
        'k4_points': GAMES - audit['engine_a_points'],
        'k4_pair_halfpoint_counts': list(reversed(audit['penta_0_to_4'])),
        'bootstrap': result,
        'owned_network_milestone': 'PASS' if result['lower_above_half'] else 'NOT_ESTABLISHED',
        'absolute_rating_3k': 'NOT_ESTABLISHED',
    }
    args.output.write_text(json.dumps(report, indent=2) + '\n')
    print(json.dumps(report, indent=2))


if __name__ == '__main__':
    main()
