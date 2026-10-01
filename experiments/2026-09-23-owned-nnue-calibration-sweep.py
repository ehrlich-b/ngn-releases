#!/usr/bin/env python3
"""Read-only scalar calibration diagnostic on the frozen 100k holdout."""

from __future__ import annotations

import argparse
import csv
import hashlib
import json
import math
from pathlib import Path

EXPECTED_SHA256 = '165a33043fb9f0b305a9d2362388bf4997024364c38b23c8b3817a594f18da36'
ALPHAS = [value / 100 for value in range(30, 121, 2)]
PREDICTION_BANDS = [(-10_000, -200), (-200, -100), (-100, -50), (-50, 0),
                    (0, 50), (50, 100), (100, 200), (200, 10_000)]


def sigmoid(score: float) -> float:
    scaled = score / 400
    return 1 / (1 + math.exp(-scaled))


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--predictions', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    if args.output.exists():
        raise ValueError('refusing existing output')
    data = args.predictions.read_bytes()
    if hashlib.sha256(data).hexdigest() != EXPECTED_SHA256:
        raise ValueError('calibration predictions SHA-256 mismatch')
    rows = list(csv.DictReader(data.decode().splitlines()))
    if len(rows) != 100_000:
        raise ValueError('expected 100k calibration positions')
    count = len(rows)
    central_count = 0
    results = {alpha: {'mae_sum': 0.0, 'wdl_mse_sum': 0.0, 'central_mae_sum': 0.0} for alpha in ALPHAS}
    rodent_mae = 0.0
    rodent_central_mae = 0.0
    zero_central_mae = 0.0
    conditioned = {role: [{'count': 0, 'teacher_sum': 0, 'abs_error_sum': 0}
                          for _ in PREDICTION_BANDS] for role in ('k4', 'rodent')}
    for row in rows:
        teacher = int(row['teacher_cp'])
        k4 = int(row['k4_cp'])
        rodent = int(row['rodent_cp'])
        central = -100 <= teacher < 100
        target = sigmoid(teacher)
        rodent_mae += abs(rodent - teacher)
        for role, prediction in (('k4', k4), ('rodent', rodent)):
            index = next((i for i, (lo, hi) in enumerate(PREDICTION_BANDS)
                          if lo <= prediction < hi), None)
            if index is None:
                raise ValueError(f'prediction outside declared bands: {prediction}')
            bucket = conditioned[role][index]
            bucket['count'] += 1
            bucket['teacher_sum'] += teacher
            bucket['abs_error_sum'] += abs(prediction - teacher)
        if central:
            central_count += 1
            rodent_central_mae += abs(rodent - teacher)
            zero_central_mae += abs(teacher)
        for alpha, metrics in results.items():
            scaled = alpha * k4
            metrics['mae_sum'] += abs(scaled - teacher)
            metrics['wdl_mse_sum'] += (sigmoid(scaled) - target) ** 2
            if central:
                metrics['central_mae_sum'] += abs(scaled - teacher)
    candidates = [{
        'scale': alpha,
        'overall_mae_cp': metrics['mae_sum'] / count,
        'overall_wdl_mse': metrics['wdl_mse_sum'] / count,
        'central_mae_cp': metrics['central_mae_sum'] / central_count,
    } for alpha, metrics in results.items()]
    report = {
        'schema': 'ngn-k4-calibration-scalar-sweep-v1',
        'predictions_sha256': EXPECTED_SHA256,
        'positions': count,
        'central_teacher_band': '-100 <= teacher_cp < 100',
        'central_positions': central_count,
        'rodent_overall_mae_cp': rodent_mae / count,
        'rodent_central_mae_cp': rodent_central_mae / central_count,
        'zero_central_mae_cp': zero_central_mae / central_count,
        'best_overall_mae_scale': min(candidates, key=lambda row: row['overall_mae_cp']),
        'best_overall_wdl_mse_scale': min(candidates, key=lambda row: row['overall_wdl_mse']),
        'best_central_mae_scale': min(candidates, key=lambda row: row['central_mae_cp']),
        'baseline_scale_1': next(row for row in candidates if row['scale'] == 1.0),
        'prediction_conditioned_bands': {
            role: [{
                'prediction_cp_range': f'{lo}:{hi - 1}',
                'count': bucket['count'],
                'teacher_mean_cp': bucket['teacher_sum'] / bucket['count'] if bucket['count'] else None,
                'teacher_mae_cp': bucket['abs_error_sum'] / bucket['count'] if bucket['count'] else None,
            } for (lo, hi), bucket in zip(PREDICTION_BANDS, buckets)]
            for role, buckets in conditioned.items()
        },
        'candidates': candidates,
    }
    args.output.write_text(json.dumps(report, indent=2) + '\n')
    print(json.dumps({key: value for key, value in report.items() if key != 'candidates'}, indent=2))


if __name__ == '__main__':
    main()
