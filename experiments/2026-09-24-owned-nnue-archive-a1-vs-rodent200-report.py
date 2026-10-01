#!/usr/bin/env python3
"""Report an archive-K4 vs Rodent match, paired by opening with the 20M K4 clock match."""

from __future__ import annotations

import argparse
import importlib.util
import json
from pathlib import Path

HERE = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location(
    'fixednodes', HERE / '2026-09-23-owned-nnue-k4-vs-rodent200-fixednodes-report.py')
base = importlib.util.module_from_spec(spec)
spec.loader.exec_module(base)

CLOCK_HELD_SHA256 = '60e7df9d8c4eefefaa2fb15d28a718803430c02ec4a78ed1129d0ceacb60f919'


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--integration', type=Path, required=True)
    parser.add_argument('--clock-integration', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    if args.output.exists():
        raise ValueError('output exists')
    held = json.loads((args.integration / 'manifest-held.json').read_text())
    setup = json.loads((args.integration / 'setup-receipt.json').read_text())
    run = args.integration / 'run-001'
    audit = json.loads((run / 'audit.json').read_text())
    operational = json.loads((run / 'audit-operational.json').read_text())
    pgn = (run / 'games.pgn').read_bytes()
    clock_run = args.clock_integration / 'run-001'
    if base.sha256(args.clock_integration / 'manifest-held.json') != CLOCK_HELD_SHA256:
        raise ValueError('clock HELD manifest mismatch')
    clock_audit_path = clock_run / 'audit.json'
    if base.sha256(clock_audit_path) != base.CLOCK_AUDIT_SHA256:
        raise ValueError('clock audit hash mismatch')
    clock_held = json.loads((args.clock_integration / 'manifest-held.json').read_text())
    clock_audit = json.loads(clock_audit_path.read_text())
    clock_pgn = (clock_run / 'games.pgn').read_bytes()
    if base.sha256(args.integration / 'manifest-held.json') != setup['held_manifest_sha256']:
        raise ValueError('HELD manifest differs from setup receipt')
    if operational.get('state') != 'COMPLETE' or operational.get('pass') is not True:
        raise ValueError('operational audit incomplete')
    if audit['opening_pgn_sha256'] != clock_audit['opening_pgn_sha256']:
        raise ValueError('opening input differs from the clock match')
    names = [role['display_name'] for role in held['inputs']['roles']]
    clock_names = [role['display_name'] for role in clock_held['inputs']['roles']]
    scores = base.pair_scores(audit, pgn, *names)
    clock_scores = base.pair_scores(clock_audit, clock_pgn, *clock_names)
    if scores.keys() != clock_scores.keys():
        raise ValueError('opening pairs differ between matches')
    score = sum(scores.values()) / base.PAIRS
    clock_score = sum(clock_scores.values()) / base.PAIRS
    interval = base.bootstrap(list(scores.values()), 2026092403)
    delta = [scores[key] - clock_scores[key] for key in sorted(scores)]
    delta_interval = base.bootstrap(delta, 2026092401)
    report = {
        'schema': 'ngn-k4-archive-vs-rodent200-report-v1',
        'k4_model_sha256': setup['k4_model_sha256'],
        'held_sha256': setup['held_manifest_sha256'],
        'chess_audit_sha256': base.sha256(run / 'audit.json'),
        'operational_audit_sha256': base.sha256(run / 'audit-operational.json'),
        'games': base.GAMES,
        'pairs': base.PAIRS,
        'archive_k4_outcomes': base.outcomes(audit, names[0]),
        'archive_k4_score': score,
        'archive_k4_score_interval_95': interval,
        'archive_k4_relative_elo_vs_rodent': base.elo(score),
        'archive_k4_relative_elo_interval_95': [base.elo(x) for x in interval],
        'k4_20m_clock_score_same_openings': clock_score,
        'archive_minus_20m_score': score - clock_score,
        'archive_minus_20m_score_interval_95': delta_interval,
        'decision_rule': setup['decision_rule'],
        'promote_over_20m': delta_interval[0] > 0,
        'bootstrap': {'method': 'nonparametric percentile, paired by exact six-ply opening prefix',
                      'score_seed': 2026092403, 'delta_seed': 2026092401,
                      'resamples': base.RESAMPLES},
        'absolute_rating': 'NOT_ESTABLISHED',
    }
    args.output.write_text(json.dumps(report, indent=2) + '\n')
    print(json.dumps(report, indent=2))


if __name__ == '__main__':
    main()
