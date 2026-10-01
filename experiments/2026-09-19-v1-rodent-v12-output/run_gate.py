#!/usr/bin/env python3
"""Alternating WSL-only performance gate for the Rodent V1.2 output kernel."""

import argparse
import hashlib
import json
import math
import os
from pathlib import Path
import re
import statistics
import subprocess
import threading
import time


FIXTURES = [
    "start_white",
    "start_black",
    "kiwipete_white",
    "kiwipete_black",
    "asymmetric_white",
    "asymmetric_black",
]


def sha256(path):
    digest = hashlib.sha256()
    with path.open("rb") as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def atomic_json(path, value):
    temporary = path.with_suffix(path.suffix + ".tmp")
    temporary.write_text(json.dumps(value, indent=2, sort_keys=True) + "\n")
    os.replace(temporary, path)


def proc_snapshot():
    ticks, parents, names = {}, {}, {}
    for entry in Path("/proc").iterdir():
        if not entry.name.isdigit():
            continue
        try:
            raw = (entry / "stat").read_text()
            close = raw.rfind(")")
            rest = raw[close + 2 :].split()
            pid = int(entry.name)
            ticks[pid] = int(rest[11]) + int(rest[12])
            parents[pid] = int(rest[1])
            names[pid] = raw[raw.find("(") + 1 : close]
        except (FileNotFoundError, PermissionError, ValueError, IndexError):
            pass
    return time.monotonic(), ticks, parents, names


def descendants(root, parents):
    owned = {root}
    changed = True
    while changed:
        changed = False
        for pid, parent in parents.items():
            if parent in owned and pid not in owned:
                owned.add(pid)
                changed = True
    return owned


def cpu_delta(before, after, owned_root):
    t0, old, _, _ = before
    t1, new, parents, names = after
    elapsed = max(t1 - t0, 1e-9)
    hertz = os.sysconf(os.sysconf_names["SC_CLK_TCK"])
    owned = descendants(owned_root, parents) if owned_root is not None else set()
    rows = []
    for pid, ticks in new.items():
        delta = ticks - old.get(pid, ticks)
        if delta > 0 and pid not in owned:
            rows.append(
                {
                    "pid": pid,
                    "name": names.get(pid, "?"),
                    "cores": delta / hertz / elapsed,
                }
            )
    rows.sort(key=lambda row: row["cores"], reverse=True)
    return {
        "elapsed_seconds": elapsed,
        "non_owned_cores": sum(row["cores"] for row in rows),
        "top": rows[:20],
    }


class InterferenceMonitor:
    def __init__(self, interval_seconds=0.25):
        self.samples = []
        self.error = None
        self.rejected = False
        self.interval_seconds = interval_seconds
        self.stop = threading.Event()
        self.thread = threading.Thread(target=self.run, daemon=True)

    def run(self):
        try:
            previous = proc_snapshot()
            while not self.stop.wait(self.interval_seconds):
                current = proc_snapshot()
                sample = cpu_delta(previous, current, os.getpid())
                sample["unix_time"] = time.time()
                self.samples.append(sample)
                self.rejected = self.rejected or sample["non_owned_cores"] > 1.0
                previous = current
        except Exception as error:
            self.error = f"{type(error).__name__}: {error}"


def run_stage(root, label, executable, environment, arguments, monitor_timing=False):
    directory = root / "stages" / label
    directory.mkdir(parents=True)
    command = [
        "/usr/bin/taskset",
        "-c",
        "4",
        "/usr/bin/nice",
        "-n",
        "10",
        str(executable),
        *arguments,
    ]
    atomic_json(directory / "command.json", command)
    monitor = InterferenceMonitor() if monitor_timing else None
    if monitor is not None:
        monitor.thread.start()
    started = time.monotonic()
    try:
        result = subprocess.run(
            command,
            cwd=directory,
            env=environment,
            capture_output=True,
            text=True,
            timeout=120,
            check=False,
        )
    finally:
        if monitor is not None:
            monitor.stop.set()
            monitor.thread.join()
    elapsed = time.monotonic() - started
    (directory / "stdout.txt").write_text(result.stdout)
    (directory / "stderr.txt").write_text(result.stderr)
    receipt = {
        "returncode": result.returncode,
        "elapsed_seconds": elapsed,
        "stdout_sha256": sha256(directory / "stdout.txt"),
        "stderr_sha256": sha256(directory / "stderr.txt"),
        "interference": None
        if monitor is None
        else {
            "threshold_cores": 1.0,
            "rejected": monitor.rejected,
            "error": monitor.error,
            "samples": monitor.samples,
        },
    }
    atomic_json(directory / "receipt.json", receipt)
    if result.returncode != 0 or result.stderr:
        raise RuntimeError(f"stage {label} failed: {receipt}")
    return directory / "stdout.txt", receipt["interference"]


def parse_direct(path, role):
    pattern = re.compile(
        rf"^BenchmarkRodentV12OutputDot/{role}(?:-\d+)?\s+\d+\s+"
        r"([0-9]+(?:\.[0-9]+)?) ns/op\s+(\d+) B/op\s+(\d+) allocs/op$"
    )
    rows = [pattern.fullmatch(line.strip()) for line in path.read_text().splitlines()]
    rows = [row for row in rows if row]
    if len(rows) != 1:
        raise RuntimeError(f"expected one direct row for {role}: {path}")
    ns, bytes_op, allocs_op = rows[0].groups()
    value = float(ns)
    if not math.isfinite(value) or value <= 0 or bytes_op != "0" or allocs_op != "0":
        raise RuntimeError(f"invalid direct row for {role}: {rows[0].group(0)}")
    return value


def parse_search(path):
    pattern = re.compile(
        r"^BenchmarkRodentV12OutputGateColdFixedNodes/([a-z_]+)(?:-\d+)?\s+1\s+"
        r"([0-9]+(?:\.[0-9]+)?) ns/op\s+400000 minimum_nodes/op\s+"
        r"(\d+) B/op\s+(\d+) allocs/op$"
    )
    found = {}
    for line in path.read_text().splitlines():
        match = pattern.fullmatch(line.strip())
        if not match:
            continue
        fixture, ns, bytes_op, allocs_op = match.groups()
        if fixture in found:
            raise RuntimeError(f"duplicate search row {fixture}: {path}")
        found[fixture] = {
            "ns": float(ns),
            "B/op": int(bytes_op),
            "allocs/op": int(allocs_op),
        }
    if list(found) != FIXTURES:
        raise RuntimeError(f"wrong search rows/order: {path}: {list(found)}")
    return found


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--output", required=True, type=Path)
    parser.add_argument("--base", required=True, type=Path)
    parser.add_argument("--candidate", required=True, type=Path)
    parser.add_argument("--direct", required=True, type=Path)
    parser.add_argument("--model", required=True, type=Path)
    args = parser.parse_args()

    if "microsoft" not in Path("/proc/version").read_text().lower():
        raise SystemExit("this gate is restricted to WSL")
    os.sched_setaffinity(0, {6})
    if args.output.exists():
        raise SystemExit(f"output already exists: {args.output}")
    for path in (args.base, args.candidate, args.direct, args.model):
        if not path.is_file():
            raise SystemExit(f"input does not exist: {path}")

    preflight_start = proc_snapshot()
    time.sleep(5)
    preflight = cpu_delta(preflight_start, proc_snapshot(), None)
    if preflight["non_owned_cores"] > 1.0:
        raise SystemExit(f"preflight interference exceeds one core: {preflight}")

    args.output.mkdir(parents=True)
    (args.output / "stages").mkdir()
    atomic_json(args.output / "preflight-cpu.json", preflight)
    manifest = {
        "schema": "ngn-rodent-v12-output-gate-v1",
        "inputs": {
            "base": {"path": str(args.base.resolve()), "sha256": sha256(args.base)},
            "candidate": {
                "path": str(args.candidate.resolve()),
                "sha256": sha256(args.candidate),
            },
            "direct": {"path": str(args.direct.resolve()), "sha256": sha256(args.direct)},
            "model": {"path": str(args.model.resolve()), "sha256": sha256(args.model)},
            "driver": {"path": str(Path(__file__).resolve()), "sha256": sha256(Path(__file__).resolve())},
        },
        "protocol": {
            "blocks": 10,
            "search_nodes": 400000,
            "search_fixtures": FIXTURES,
            "search_benchtime": "1x",
            "direct_benchtime": "250ms",
            "gomaxprocs": 1,
            "child_cpu": 4,
            "driver_cpu": 6,
            "timing_interference_sample_seconds": 0.25,
            "interference_retry_policy": "discard_pair_and_retry_same_order",
            "maximum_attempts_per_ten_blocks": 30,
        },
        "gates": {
            "identity": "byte_exact_cold_and_warm_trajectory_receipts",
            "allocation": "exact_identity",
            "direct_median_ratio_max": 2 / 3,
            "direct_each_order_stratum_below": 1.0,
            "search_equal_weight_median_ratio_max": 0.97,
            "search_each_fixture_below": 1.0,
            "search_each_order_stratum_max": 0.97,
            "interference_max_non_owned_cores": 1.0,
        },
    }
    atomic_json(args.output / "manifest.json", manifest)

    environment = {
        "PATH": "/usr/bin:/bin",
        "LANG": "C",
        "LC_ALL": "C",
        "TZ": "UTC",
        "GOMAXPROCS": "1",
        "GOGC": "100",
        "GOMEMLIMIT": "off",
        "GODEBUG": "",
        "RODENT_V12_DEFAULT_MODEL": str(args.model.resolve()),
    }
    decision = {"schema": "ngn-rodent-v12-output-gate-decision-v1", "state": "RUNNING"}
    discarded_timing_attempts = []
    try:
        identity = {}
        for role, binary in (("base", args.base), ("candidate", args.candidate)):
            receipt = args.output / f"search-identity-{role}.json"
            run_stage(
                args.output,
                f"identity-{role}",
                binary,
                {**environment, "NGN_RODENT_V12_OUTPUT_GATE_RECEIPT": str(receipt)},
                ["-test.run=^TestRodentV12OutputGateSnapshot$", "-test.count=1", "-test.v"],
            )
            identity[role] = receipt
        if identity["base"].read_bytes() != identity["candidate"].read_bytes():
            raise RuntimeError("base/candidate cold+warm search receipts differ")

        direct = []
        direct_attempt = 0
        while len(direct) < 10:
            direct_attempt += 1
            if direct_attempt > 30:
                raise RuntimeError("could not collect ten interference-free direct blocks")
            block = len(direct) + 1
            order = ("portable", "selected") if block % 2 else ("selected", "portable")
            values = {}
            stage_interference = []
            for slot, role in enumerate(order, 1):
                output, interference = run_stage(
                    args.output,
                    f"direct-b{block:02d}-a{direct_attempt:02d}-s{slot}-{role}",
                    args.direct,
                    environment,
                    [
                        "-test.run=V1NoTests",
                        f"-test.bench=^BenchmarkRodentV12OutputDot/{role}$",
                        "-test.benchtime=250ms",
                        "-test.count=1",
                        "-test.benchmem",
                    ],
                    monitor_timing=True,
                )
                values[role] = parse_direct(output, role)
                stage_interference.append(interference)
            if any(row["rejected"] or row["error"] for row in stage_interference):
                discarded_timing_attempts.append(
                    {
                        "kind": "direct",
                        "block": block,
                        "attempt": direct_attempt,
                        "order": f"{order[0]}-first",
                        "interference": stage_interference,
                    }
                )
                continue
            direct.append(
                {
                    "block": block,
                    "order": f"{order[0]}-first",
                    "portable_ns": values["portable"],
                    "selected_ns": values["selected"],
                    "ratio": values["selected"] / values["portable"],
                }
            )
        direct_median = statistics.median(row["ratio"] for row in direct)
        direct_order_medians = {
            order: statistics.median(row["ratio"] for row in direct if row["order"] == order)
            for order in ("portable-first", "selected-first")
        }
        direct_pass = direct_median <= 2 / 3 and all(
            value < 1.0 for value in direct_order_medians.values()
        )

        search = []
        if direct_pass:
            search_attempt = 0
            while len(search) < 10:
                search_attempt += 1
                if search_attempt > 30:
                    raise RuntimeError("could not collect ten interference-free search blocks")
                block = len(search) + 1
                order = ("base", "candidate") if block % 2 else ("candidate", "base")
                values = {}
                stage_interference = []
                for slot, role in enumerate(order, 1):
                    binary = args.base if role == "base" else args.candidate
                    output, interference = run_stage(
                        args.output,
                        f"search-b{block:02d}-a{search_attempt:02d}-s{slot}-{role}",
                        binary,
                        environment,
                        [
                            "-test.run=V1NoTests",
                            "-test.bench=^BenchmarkRodentV12OutputGateColdFixedNodes$",
                            "-test.benchtime=1x",
                            "-test.count=1",
                            "-test.benchmem",
                        ],
                        monitor_timing=True,
                    )
                    values[role] = parse_search(output)
                    stage_interference.append(interference)
                if any(row["rejected"] or row["error"] for row in stage_interference):
                    discarded_timing_attempts.append(
                        {
                            "kind": "search",
                            "block": block,
                            "attempt": search_attempt,
                            "order": f"{order[0]}-first",
                            "interference": stage_interference,
                        }
                    )
                    continue
                ratios = {}
                for fixture in FIXTURES:
                    base = values["base"][fixture]
                    candidate = values["candidate"][fixture]
                    if (base["B/op"], base["allocs/op"]) != (
                        candidate["B/op"],
                        candidate["allocs/op"],
                    ):
                        raise RuntimeError(
                            f"allocation mismatch block={block} fixture={fixture}: "
                            f"base={base} candidate={candidate}"
                        )
                    ratios[fixture] = candidate["ns"] / base["ns"]
                search.append(
                    {
                        "block": block,
                        "order": f"{order[0]}-first",
                        "ratios": ratios,
                        "raw": values,
                    }
                )

        fixture_medians = {
            fixture: statistics.median(row["ratios"][fixture] for row in search)
            for fixture in FIXTURES
        }
        equal_weight_median = statistics.median(fixture_medians.values())
        search_order_medians = {
            order: statistics.median(
                row["ratios"][fixture]
                for row in search
                if row["order"] == order
                for fixture in FIXTURES
            )
            for order in ("base-first", "candidate-first")
        }
        improving_cells = sum(
            ratio < 1.0 for row in search for ratio in row["ratios"].values()
        )
        search_pass = (
            len(search) == 10
            and equal_weight_median <= 0.97
            and all(value < 1.0 for value in fixture_medians.values())
            and all(value <= 0.97 for value in search_order_medians.values())
        )
        decision.update(
            {
                "state": "COMPLETE",
                "decision": "PASS_PERFORMANCE" if direct_pass and search_pass else "SHELVE",
                "search_identity_sha256": sha256(identity["base"]),
                "direct": direct,
                "direct_median_ratio": direct_median,
                "direct_order_medians": direct_order_medians,
                "search": search,
                "search_fixture_medians": fixture_medians,
                "search_equal_weight_median_ratio": equal_weight_median,
                "search_order_medians": search_order_medians,
                "search_improving_cells": improving_cells,
                "search_total_cells": len(search) * len(FIXTURES),
                "discarded_timing_attempts": discarded_timing_attempts,
            }
        )
    except BaseException as error:
        decision.update(
            {
                "state": "FAILED",
                "decision": "FAILED_EXECUTION",
                "error": f"{type(error).__name__}: {error}",
            }
        )
    atomic_json(
        args.output / "interference.json",
        {
            "threshold_cores": 1.0,
            "policy": "discard_entire_pair_if_either_stage_has_a_rejected_sample",
            "discarded_timing_attempts": discarded_timing_attempts,
        },
    )
    atomic_json(args.output / "decision.json", decision)
    print(json.dumps(decision, indent=2, sort_keys=True))
    if decision.get("decision") != "PASS_PERFORMANCE":
        raise SystemExit(1)


if __name__ == "__main__":
    main()
