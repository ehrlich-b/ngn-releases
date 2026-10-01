#!/usr/bin/env python3
"""Read one existing log; conservatively exclude hard/emergency deadlines.

No engine launch or file writes. Intended for the original WSL host. Source
engine/time.go is identical at match 52ee629 and review f75b578 (Git blob
3cb408b6fae48d4d7098c0f9216a0df8c418714d). Abort after 30 seconds.
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
rows = []
anomalies = collections.Counter()
excluded = collections.Counter()
overheads = collections.Counter()
increments = collections.Counter()
go_examples = []

with open(path) as log:
    for line in log:
        match = pattern.match(line)
        if not match:
            continue
        hour, minute, second, thread, direction, message = match.groups()
        timestamp = (int(hour) * 3600 + int(minute) * 60 + float(second)) * 1000
        if message.startswith("setoption name Move Overhead value "):
            overheads[message.rsplit(" ", 1)[1]] += 1
        elif message.startswith("go "):
            if thread in active:
                anomalies["duplicate_go"] += 1
            if direction != "<---":
                anomalies["wrong_go_direction"] += 1
            tokens = message.split()[1:]
            allowed = {"wtime", "btime", "winc", "binc"}
            reason = None
            values = {}
            if len(tokens) % 2:
                reason = "unsupported_or_odd_go_tokens"
            else:
                for index in range(0, len(tokens), 2):
                    key, value = tokens[index:index + 2]
                    if key not in allowed or key in values:
                        reason = "unsupported_or_duplicate_go_key:" + key
                        break
                    try:
                        values[key] = int(value)
                    except ValueError:
                        reason = "noninteger_go_value"
                        break
                if reason is None and set(values) != allowed:
                    reason = "missing_time_or_increment"
                if reason is None and min(values.values()) < 0:
                    reason = "negative_time_or_increment"
            if len(go_examples) < 3:
                go_examples.append(message)
            active[thread] = {
                "go": timestamp, "values": values, "excluded": reason,
                "last": None, "stops": 0,
            }
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
            nodes = re.search(r"\bnodes (\d+)", message)
            reported = re.search(r"\btime (\d+)", message)
            if " lowerbound" in message or " upperbound" in message:
                anomalies["bounded_info"] += 1
                continue
            if not (depth and score and nodes and reported):
                anomalies["unparsed_info"] += 1
                continue
            active[thread]["last"] = {
                "timestamp": timestamp, "depth": int(depth.group(1)),
                "mate": score.group(1) == "mate", "nodes": int(nodes.group(1)),
            }
        elif message.startswith("bestmove "):
            if direction != "--->":
                anomalies["wrong_bestmove_direction"] += 1
            search = active.pop(thread, None)
            if search is None:
                anomalies["bestmove_without_go"] += 1
                continue
            if search["excluded"]:
                excluded[search["excluded"]] += 1
                continue
            last = search["last"]
            if last is None:
                anomalies["missing_exact_info"] += 1
                continue
            duration = timestamp - search["go"]
            tail = timestamp - last["timestamp"]
            if duration < 0 or tail < 0:
                anomalies["negative_timestamp"] += 1
                continue
            values = search["values"]
            bank = min(values["wtime"], values["btime"])
            increment = min(values["winc"], values["binc"])
            increments[str(increment)] += 1
            usable = max(bank - 100 - 100, 0)
            soft_min = usable / 40.0 + .8 * increment
            hard_min = max(soft_min, min(4 * soft_min, .3 * usable))
            emergency_min = bank - 100
            rows.append({
                "duration": duration, "tail": tail, "hard_min": hard_min,
                "emergency_min": emergency_min, "mate": last["mate"],
                "depth100": last["depth"] >= 100, "stops": search["stops"],
                "nodes1": last["nodes"] <= 1,
            })


def summarize(selected):
    duration = sum(row["duration"] for row in selected)
    tail = sum(row["tail"] for row in selected)
    result = {
        "n": len(selected), "duration_ms": round(duration, 3),
        "tail_ms": round(tail, 3),
        "last_score_mate": sum(row["mate"] for row in selected),
        "last_depth_ge100": sum(row["depth100"] for row in selected),
        "last_nodes_le1": sum(row["nodes1"] for row in selected),
        "explicit_stop": sum(bool(row["stops"]) for row in selected),
        "deadline_exclusions": {},
    }
    for margin in (0, 5):
        ruled_out = [row for row in selected if
                      row["duration"] + margin < row["hard_min"] and
                      row["duration"] + margin < row["emergency_min"]]
        excluded_tail = sum(row["tail"] for row in ruled_out)
        result["deadline_exclusions"][str(margin) + "ms_margin"] = {
            "n": len(ruled_out),
            "search_pct": round(100 * len(ruled_out) / len(selected), 6) if selected else None,
            "tail_ms": round(excluded_tail, 3),
            "tail_pct": round(100 * excluded_tail / tail, 6) if tail else None,
            "last_score_mate": sum(row["mate"] for row in ruled_out),
            "last_depth_ge100": sum(row["depth100"] for row in ruled_out),
        }
    return result


print(json.dumps({
    "elapsed_sec": round(time.monotonic() - started, 3),
    "input": path,
    "source_time_go_blob": "3cb408b6fae48d4d7098c0f9216a0df8c418714d",
    "anomalies": dict(anomalies), "excluded_go": dict(excluded),
    "pending": len(active), "move_overhead_commands": dict(overheads),
    "minimum_increment_counts": dict(increments), "go_examples": go_examples,
    "all": summarize(rows),
    "substantial_tail_ge5ms": summarize([row for row in rows if row["tail"] >= 5]),
    "substantial_nonmate_below_depth100_no_stop": summarize([
        row for row in rows if row["tail"] >= 5 and not row["mate"]
        and not row["depth100"] and not row["stops"]
    ]),
}, indent=2))
