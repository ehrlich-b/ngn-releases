#!/usr/bin/env python3
"""One read-only, 30-second-bounded scan of completed iteration cost growth.

Filters were declared before reading the log. No engines, files written, or
counterfactual completion simulation. Final tails include finalization and are
only conditional lower-bound proxies for unfinished-iteration cost.
"""

import collections
import json
import re
import signal
import sys
import time

signal.alarm(30)
started = time.monotonic()
path = sys.argv[1] if len(sys.argv) > 1 else (
    "/home/ehrli/repos/ngn-external-counter-checkpoint-gate-20260913/"
    "run-003/candidate/fastchess.log"
)
pattern = re.compile(
    r"^\[Engine\] \[(\d+):(\d+):(\d+\.\d+)\] <\s*(\d+)>\s+"
    r"NGN-RODENT-A (<---|--->) (.*)$"
)
active = {}
anomalies = collections.Counter()
all_growth = []
last_eligible_growth = []
tail_rows = []
searches = 0

with open(path) as log:
    for line in log:
        match = pattern.match(line)
        if not match:
            continue
        hour, minute, second, thread, direction, message = match.groups()
        timestamp = (int(hour) * 3600 + int(minute) * 60 + float(second)) * 1000
        if message.startswith("go "):
            if thread in active:
                anomalies["duplicate_go"] += 1
            if direction != "<---":
                anomalies["wrong_go_direction"] += 1
            tokens = message.split()[1:]
            values = {}
            valid = len(tokens) % 2 == 0
            if valid:
                for index in range(0, len(tokens), 2):
                    key, value = tokens[index:index + 2]
                    if key not in {"wtime", "btime", "winc", "binc"} or key in values:
                        valid = False
                        break
                    try:
                        values[key] = int(value)
                    except ValueError:
                        valid = False
                        break
                valid = valid and set(values) == {"wtime", "btime", "winc", "binc"}
                valid = valid and min(values.values(), default=-1) >= 0
            if not valid:
                anomalies["unsupported_go"] += 1
            active[thread] = {"go": timestamp, "values": values if valid else None,
                              "infos": [], "stops": 0}
        elif message == "stop":
            if thread in active:
                active[thread]["stops"] += 1
            else:
                anomalies["stop_without_active"] += 1
        elif message.startswith("info depth "):
            if thread not in active:
                anomalies["info_without_go"] += 1
                continue
            depth = re.search(r"\bdepth (\d+)", message)
            score = re.search(r"\bscore (cp|mate) (-?\d+)", message)
            if " lowerbound" in message or " upperbound" in message:
                anomalies["bounded_info"] += 1
                continue
            if not (depth and score):
                anomalies["unparsed_info"] += 1
                continue
            active[thread]["infos"].append({
                "t": timestamp, "d": int(depth.group(1)), "mate": score.group(1) == "mate",
            })
        elif message.startswith("bestmove "):
            if direction != "--->":
                anomalies["wrong_bestmove_direction"] += 1
            search = active.pop(thread, None)
            if search is None:
                anomalies["bestmove_without_go"] += 1
                continue
            searches += 1
            infos = search["infos"]
            eligible_growth = []
            for index in range(2, len(infos)):
                before, previous, current = infos[index - 2:index + 1]
                if not (current["d"] >= 8 and before["d"] + 1 == previous["d"]
                        and previous["d"] + 1 == current["d"]):
                    continue
                if any(item["mate"] for item in (before, previous, current)):
                    continue
                prior_cost = previous["t"] - before["t"]
                current_cost = current["t"] - previous["t"]
                if prior_cost < 2 or current_cost <= 0:
                    continue
                row = {"prior": prior_cost, "next": current_cost,
                       "ratio": current_cost / prior_cost, "d": current["d"]}
                eligible_growth.append(row)
                all_growth.append(row)
            if eligible_growth:
                last_eligible_growth.append(eligible_growth[-1])
            if len(infos) < 2:
                continue
            previous, last = infos[-2:]
            tail = timestamp - last["t"]
            duration = timestamp - search["go"]
            last_cost = last["t"] - previous["t"]
            if duration < 0 or tail < 0:
                anomalies["negative_timestamp"] += 1
                continue
            if (not 8 <= last["d"] < 100 or previous["d"] + 1 != last["d"]
                    or last["mate"] or previous["mate"] or last_cost < 2
                    or tail < 5 or search["stops"]):
                continue
            bound_pass = False
            values = search["values"]
            if values is not None:
                bank = min(values["wtime"], values["btime"])
                increment = min(values["winc"], values["binc"])
                usable = max(bank - 200, 0)
                soft_min = usable / 40.0 + .8 * increment
                hard_min = max(soft_min, min(4 * soft_min, .3 * usable))
                bound_pass = duration + 5 < hard_min and duration + 5 < bank - 100
            tail_rows.append({"prior": last_cost, "next": tail, "ratio": tail / last_cost,
                              "d": last["d"], "bound_pass": bound_pass})


def summarize(rows):
    if not rows:
        return {"n": 0}
    ratios = sorted(row["ratio"] for row in rows)
    quantiles = {str(percent): round(ratios[int(percent / 100 * (len(ratios) - 1))], 6)
                 for percent in (10, 25, 50, 75, 90, 95)}
    return {
        "n": len(rows), "ratio_quantiles_lower_order_statistic": quantiles,
        "fraction_percent_above": {
            str(threshold): round(100 * sum(row["ratio"] > threshold for row in rows) / len(rows), 6)
            for threshold in (1, 2, 4)
        },
        "sum_prior_ms": round(sum(row["prior"] for row in rows), 3),
        "sum_next_or_tail_ms": round(sum(row["next"] for row in rows), 3),
        "ratio_of_sums": round(sum(row["next"] for row in rows) / sum(row["prior"] for row in rows), 6),
        "min_completed_depth": min(row["d"] for row in rows),
        "max_completed_depth": max(row["d"] for row in rows),
    }


print(json.dumps({
    "elapsed_sec": round(time.monotonic() - started, 3), "input": path,
    "searches": searches, "anomalies": dict(anomalies), "pending": len(active),
    "completed_growth_all_eligible": summarize(all_growth),
    "completed_growth_last_eligible_per_search": summarize(last_eligible_growth),
    "final_tail_over_last_completed_cost": summarize(tail_rows),
    "final_tail_below_conservative_deadlines_5ms": summarize([
        row for row in tail_rows if row["bound_pass"]
    ]),
}, indent=2))
