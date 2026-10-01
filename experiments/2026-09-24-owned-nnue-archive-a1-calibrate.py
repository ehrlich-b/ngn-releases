#!/usr/bin/env python3
"""Board-set packing parity and a monotone raw->NGN score map for the archive."""
import bisect, csv, glob, json, math, struct, sys

probe_csv, labels_glob, calib_bf, out_json = sys.argv[1:5]

teacher = {}
for fn in sorted(glob.glob(labels_glob)):
    with open(fn) as f:
        for line in f:
            r = json.loads(line)
            if r.get("type") == "label" and r.get("status") == "accepted" and "ngn_score" in r:
                teacher[r["k4_input_sha256"]] = int(r["ngn_score"])

probe = {}
with open(probe_csv) as f:
    for row in csv.DictReader(f):
        probe.setdefault(row["k4_input_sha256"], row)

def board_key(rec):
    return rec[:24] + rec[27:29]

# Parity: every Go-packed calibration record's board must equal one of our packed
# probe boards, and our board's teacher score must equal the Go record's score.
ours = {}
for key, row in probe.items():
    if key in teacher:
        ours.setdefault(board_key(bytes.fromhex(row["board_hex"])), set()).add(teacher[key])
with open(calib_bf, "rb") as f:
    bf = f.read()
found = score_match = 0
for i in range(len(bf) // 32):
    rec = bf[32 * i: 32 * i + 32]
    scores = ours.get(board_key(rec))
    if scores is not None:
        found += 1
        if struct.unpack_from("<h", rec, 24)[0] in scores:
            score_match += 1

pairs = sorted((int(probe[k]["raw_score"]), c) for k, c in teacher.items() if k in probe)
raw = [p[0] for p in pairs]
n = len(pairs)

# Quantile bins on raw: median teacher score per bin.
bins = []
q = 40
for b in range(q):
    lo, hi = b * n // q, (b + 1) * n // q
    chunk = pairs[lo:hi]
    cs = sorted(c for _, c in chunk)
    rs = [r for r, _ in chunk]
    bins.append({"raw_lo": rs[0], "raw_hi": rs[-1], "raw_median": rs[len(rs) // 2],
                 "ngn_median": cs[len(cs) // 2], "ngn_mean": sum(cs) / len(cs), "n": len(chunk)})

sig = lambda v: 1 / (1 + math.exp(-max(-60.0, min(60.0, v))))

def mse(mapper):
    return sum((sig(mapper(r) / 400) - sig(c / 400)) ** 2 for r, c in pairs) / n

def linear(f):
    return lambda r: f * r

def tanh_map(scale, knee):
    return lambda r: scale * math.tanh(r / knee)

best_linear = min(((mse(linear(f)), f) for f in [0.40 + 0.01 * i for i in range(41)]))
best_tanh = min(((mse(tanh_map(s, k)), s, k)
                 for s in [400, 500, 600, 700, 800, 900, 1000, 1200, 1500, 2000]
                 for k in [300, 400, 500, 600, 700, 800, 1000, 1200, 1500, 2000]))
_, s0, k0 = best_tanh
fine = min(((mse(tanh_map(s, k)), s, k)
            for s in [s0 + 25 * i for i in range(-4, 5)]
            for k in [k0 + 25 * i for i in range(-4, 5)]))
central = [(r, c) for r, c in pairs if abs(r) <= 300]
slope0 = sum(r * c for r, c in central) / sum(r * r for r, c in central)

result = {
    "joined": n,
    "calibration_bf_records": len(bf) // 32,
    "parity_boards_found": found,
    "parity_scores_match": score_match,
    "best_linear": {"wdl_mse": best_linear[0], "factor": best_linear[1]},
    "best_tanh": {"wdl_mse": fine[0], "scale": fine[1], "knee": fine[2],
                  "slope_at_zero": fine[1] / fine[2]},
    "central_slope_abs_raw_le_300": slope0,
    "quantile_bins": bins,
}
with open(out_json, "w") as f:
    json.dump(result, f, indent=1)
print(json.dumps({k: v for k, v in result.items() if k != "quantile_bins"}, indent=1))
for b in bins:
    print(f'{b["raw_lo"]:>7} {b["raw_hi"]:>7} med_raw={b["raw_median"]:>7} med_ngn={b["ngn_median"]:>6} mean_ngn={b["ngn_mean"]:8.1f}')
