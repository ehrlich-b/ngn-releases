#!/usr/bin/env python3
"""Fit per-source raw-score scales from ngnk4archive scatter calibration bins.

Each scatter receipt holds eligible_score_result_bins: bin -> [loss, draw, win]
with bin = floor(clamp(raw, -2000, 2000) / 25). For each source the expected STM
score E(raw) is fitted as 1 / (1 + exp(-raw / k)) by weighted least squares over
bins with |center| <= 1000; the source's raw scale is k_reference / k_source,
so scaled raw scores carry the reference archive's score-to-result relation.

  fit_source_scale.py --reference T80/scatter.json --source NAME=DIR/scatter.json ... --out scales.json
"""

import argparse
import json
import math


def fit(path):
    bins = json.load(open(path))['counts']['eligible_score_result_bins']
    rows = []
    for key, (loss, draw, win) in bins.items():
        center = int(key) * 25 + 12.5
        total = loss + draw + win
        if abs(center) <= 1000 and total:
            rows.append((center, (draw * 0.5 + win) / total, total))

    def error(k):
        return sum(w * (e - 1 / (1 + math.exp(-x / k))) ** 2 for x, e, w in rows) / sum(w for _, _, w in rows)

    low, high = 20.0, 5000.0
    for _ in range(200):
        a, b = low + (high - low) / 3, high - (high - low) / 3
        if error(a) < error(b):
            high = b
        else:
            low = a
    k = (low + high) / 2
    return {'k': k, 'rmse': math.sqrt(error(k)), 'bins': len(rows), 'records': sum(w for _, _, w in rows)}


def main():
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument('--reference', required=True)
    parser.add_argument('--source', action='append', default=[])
    parser.add_argument('--out', required=True)
    args = parser.parse_args()
    reference = fit(args.reference)
    result = {'reference': {'path': args.reference, **reference}, 'sources': {}}
    for item in args.source:
        name, path = item.split('=', 1)
        source = fit(path)
        result['sources'][name] = {'path': path, **source, 'raw_scale': reference['k'] / source['k']}
    json.dump(result, open(args.out, 'x'), indent=2)
    print(json.dumps(result, indent=2))


if __name__ == '__main__':
    main()
