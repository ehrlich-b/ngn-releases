#!/usr/bin/env python3
"""Read-only audit of completed-info tails in an existing fastchess log.

This is the parser independently run on 2026-09-19, preserved without a rerun.
The only post-run changes are formatting, this header and an optional log-path
argument. It launches no engines and writes nothing. The default path is the
WSL run-003 candidate log, not a Mac-local path.
"""

import collections
import json
import re
import statistics
import sys
import time


started = time.monotonic()
path = sys.argv[1] if len(sys.argv) > 1 else (
    "/home/ehrli/repos/ngn-external-counter-checkpoint-gate-20260913/"
    "run-003/candidate/fastchess.log"
)
pattern = re.compile(
    r"^\[Engine\] \[(\d+):(\d+):(\d+\.\d+)\] <\s*(\d+)>\s+"
    r"(NGN-RODENT-A|Counter 5\.5) (<---|--->) (.*)$"
)
active = {}
results = collections.defaultdict(list)
anomalies = collections.Counter()
directions = collections.Counter()
samples = collections.defaultdict(list)

with open(path) as log:
    for line in log:
        match = pattern.match(line)
        if not match:
            continue
        hour, minute, second, thread, role, direction, message = match.groups()
        timestamp = (
            int(hour) * 3600 + int(minute) * 60 + float(second)
        ) * 1000
        key = (role, thread)
        if message.startswith("go "):
            directions[role + " go " + direction] += 1
            if key in active:
                anomalies["duplicate_go"] += 1
            active[key] = {"go": timestamp, "infos": [], "stops": 0}
        elif message == "stop":
            if key in active:
                active[key]["stops"] += 1
            else:
                anomalies["stop_without_active"] += 1
        elif message.startswith("info depth "):
            if key not in active:
                anomalies["info_without_go"] += 1
                continue
            depth = re.search(r"\bdepth (\d+)", message)
            nodes = re.search(r"\bnodes (\d+)", message)
            reported = re.search(r"\btime (\d+)", message)
            score = re.search(r"\bscore (cp|mate) (-?\d+)", message)
            pv = re.search(r"\bpv (\S+)", message)
            bounded = " lowerbound" in message or " upperbound" in message
            if bounded:
                anomalies[role + " bounded_info"] += 1
            if not (depth and nodes and reported and score):
                anomalies[role + " unparsed_info"] += 1
                continue
            active[key]["infos"].append({
                "t": timestamp,
                "depth": int(depth.group(1)),
                "nodes": int(nodes.group(1)),
                "reported": int(reported.group(1)),
                "bounded": bounded,
                "pv": pv.group(1) if pv else None,
            })
        elif message.startswith("bestmove "):
            directions[role + " bestmove " + direction] += 1
            search = active.pop(key, None)
            if search is None:
                anomalies["bestmove_without_go"] += 1
                continue
            infos = [item for item in search["infos"] if not item["bounded"]]
            if not infos:
                anomalies[role + " no_exact_info"] += 1
                continue
            last = infos[-1]
            total = timestamp - search["go"]
            tail = timestamp - last["t"]
            best = message.split()[1]
            if total < 0 or tail < 0:
                anomalies[role + " negative_timestamp"] += 1
                continue
            row = {
                "total": total,
                "tail": tail,
                "depth": last["depth"],
                "infos": len(infos),
                "repeat": len(infos) > 1 and infos[-2]["depth"] == last["depth"],
                "stops": search["stops"],
                "pv_mismatch": best != last["pv"],
                "report_gap": last["t"] - search["go"] - last["reported"],
            }
            results[role].append(row)
            if row["pv_mismatch"] and len(samples[role]) < 5:
                samples[role].append({
                    "thread": thread, "best": best, "pv": last["pv"],
                    "total": total, "tail": tail,
                })


def summarize(rows):
    if not rows:
        return {}
    totals = sum(row["total"] for row in rows)
    tails = sum(row["tail"] for row in rows)
    ratios = sorted(row["tail"] / row["total"] for row in rows if row["total"] > 0)
    return {
        "n": len(rows),
        "sum_total_ms": round(totals, 3),
        "sum_tail_ms": round(tails, 3),
        "weighted_tail_pct": round(100 * tails / totals, 5),
        "median_tail_pct": round(100 * statistics.median(ratios), 5),
        "p90_tail_pct": round(100 * ratios[int(.9 * (len(ratios) - 1))], 5),
        "stopped": sum(bool(row["stops"]) for row in rows),
        "last_depth_repeat": sum(row["repeat"] for row in rows),
        "pv_mismatch": sum(row["pv_mismatch"] for row in rows),
        "median_report_gap_ms": round(statistics.median(row["report_gap"] for row in rows), 3),
    }


output = {
    "elapsed_sec": round(time.monotonic() - started, 3),
    "anomalies": dict(anomalies),
    "pending": len(active),
    "directions": dict(directions),
    "roles": {},
}
for role, rows in results.items():
    output["roles"][role] = {
        "all": summarize(rows),
        "ge10ms": summarize([row for row in rows if row["total"] >= 10]),
        "ge100ms": summarize([row for row in rows if row["total"] >= 100]),
        "ge1000ms": summarize([row for row in rows if row["total"] >= 1000]),
        "depth_ge10": summarize([row for row in rows if row["depth"] >= 10]),
        "no_stop": summarize([row for row in rows if not row["stops"]]),
        "mismatch_samples": samples[role],
    }
print(json.dumps(output, indent=2))
