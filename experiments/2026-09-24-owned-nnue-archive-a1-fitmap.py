#!/usr/bin/env python3
"""Fit a monotone piecewise-linear raw-archive -> NGN score map on SF18 labels.

Knots are (median raw, median SF18 ngn_score) over raw-quantile bins, excluding
the VALUE_NONE sentinel (|raw| >= 32000); ngn medians are made non-decreasing.
Outside the outer knots the map is clamped to the outer ngn values.
"""
import bisect, csv, glob, json, math, sys

probe_csv, labels_glob, out_json = sys.argv[1:4]
BINS = 64
SENTINEL = 32000

teacher = {}
for fn in sorted(glob.glob(labels_glob)):
    for line in open(fn):
        r = json.loads(line)
        if r.get("type") == "label" and r.get("status") == "accepted" and "ngn_score" in r:
            teacher[r["k4_input_sha256"]] = int(r["ngn_score"])
first = {}
with open(probe_csv) as f:
    for row in csv.DictReader(f):
        first.setdefault(row["k4_input_sha256"], int(row["raw_score"]))

all_pairs = [(first[k], c) for k, c in teacher.items() if k in first]
sentinel = [(r, c) for r, c in all_pairs if abs(r) >= SENTINEL]
pairs = sorted((r, c) for r, c in all_pairs if abs(r) < SENTINEL)
n = len(pairs)

knots = []
for b in range(BINS):
    chunk = pairs[b * n // BINS:(b + 1) * n // BINS]
    rs = sorted(r for r, _ in chunk)
    cs = sorted(c for _, c in chunk)
    knots.append([rs[len(rs) // 2], cs[len(cs) // 2]])
merged = []
for r, c in knots:
    if merged and r <= merged[-1][0]:
        merged[-1][1] = (merged[-1][1] + c) / 2
    else:
        merged.append([r, c])
for i in range(1, len(merged)):
    merged[i][1] = max(merged[i][1], merged[i - 1][1])
xs = [k[0] for k in merged]
ys = [float(k[1]) for k in merged]

def piecewise(r):
    if r <= xs[0]:
        return ys[0]
    if r >= xs[-1]:
        return ys[-1]
    i = bisect.bisect_right(xs, r)
    x0, x1, y0, y1 = xs[i - 1], xs[i], ys[i - 1], ys[i]
    return y0 + (y1 - y0) * (r - x0) / (x1 - x0)

sig = lambda v: 1 / (1 + math.exp(-max(-60.0, min(60.0, v))))

def mse(mapper, data):
    return sum((sig(mapper(r) / 400) - sig(c / 400)) ** 2 for r, c in data) / len(data)

best_linear = min((mse(lambda r, f=f: f * r, pairs), f) for f in [0.40 + 0.01 * i for i in range(51)])
best_tanh = min((mse(lambda r, s=s, k=k: s * math.tanh(r / k), pairs), s, k)
                for s in range(500, 2001, 100) for k in range(400, 3001, 200))
mean_c = lambda d: sum(c for _, c in d) / len(d)
result = {
    "joined": len(all_pairs),
    "sentinel_rows": len(sentinel),
    "sentinel_fraction": len(sentinel) / len(all_pairs),
    "sentinel_mean_ngn": mean_c(sentinel) if sentinel else None,
    "fit_rows": n,
    "knots": merged,
    "wdl_mse_piecewise": mse(piecewise, pairs),
    "wdl_mse_best_linear": {"mse": best_linear[0], "factor": best_linear[1]},
    "wdl_mse_best_tanh": {"mse": best_tanh[0], "scale": best_tanh[1], "knee": best_tanh[2]},
    "note": "Knots fitted on the same calibration rows they are scored on; in-sample.",
}
json.dump(result, open(out_json, "w"), indent=1)
print(json.dumps({k: v for k, v in result.items() if k != "knots"}, indent=1))
print("knots", json.dumps(merged))
