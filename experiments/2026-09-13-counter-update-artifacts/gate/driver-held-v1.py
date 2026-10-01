#!/usr/bin/env python3
"""Held WSL-only Counter accumulator-fusion performance gate."""
import argparse
import hashlib
import json
import math
import os
from pathlib import Path
import re
import shutil
import statistics
import subprocess
import sys
import threading
import time

COLD_FIXTURES = ["start_white", "start_black", "kiwipete_white", "kiwipete_black", "asymmetric_white", "asymmetric_black"]
PERSISTENT_FIXTURES = ["game_1", "game_2", "game_3"]
CORRECTNESS = "^(TestCounterTransitionFixedNodeSnapshot|TestCounterTransitionAdapterSpecialMovesAndDirectBoards|TestCounterTransitionAdapterRejectsInvalidState|TestCounterMoveDeltaDirectMatchesSemanticReferenceForIllegalAndMalformedMoves|TestCounterProfilePersistentSnapshot)$"
CORRECTNESS_NAMES = {
    "TestCounterTransitionFixedNodeSnapshot",
    "TestCounterTransitionAdapterSpecialMovesAndDirectBoards",
    "TestCounterTransitionAdapterRejectsInvalidState",
    "TestCounterMoveDeltaDirectMatchesSemanticReferenceForIllegalAndMalformedMoves",
    "TestCounterProfilePersistentSnapshot",
}


def die(message):
    raise SystemExit(f"counter-fusion-gate: {message}")


def sha256(path):
    digest = hashlib.sha256()
    with open(path, "rb") as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def atomic_json(path, value):
    temporary = path.with_suffix(path.suffix + ".tmp")
    temporary.write_text(json.dumps(value, indent=2, sort_keys=True) + "\n")
    os.replace(temporary, path)


def frozen(spec, label):
    path = Path(spec["path"]).resolve()
    expected = spec.get("sha256", "")
    if not path.is_file() or not re.fullmatch(r"[0-9a-f]{64}", expected):
        die(f"{label} is absent or not frozen: {path}")
    actual = sha256(path)
    if actual != expected:
        die(f"{label} hash mismatch: {actual} != {expected}")
    return path


def copy_frozen(source, destination, expected):
    shutil.copy2(source, destination)
    if sha256(destination) != expected:
        die(f"copied input changed: {destination}")
    destination.chmod(destination.stat().st_mode | 0o100)


def proc_snapshot():
    ticks, parents, names = {}, {}, {}
    for entry in Path("/proc").iterdir():
        if not entry.name.isdigit():
            continue
        try:
            raw = (entry / "stat").read_text()
            close = raw.rfind(")")
            if close < 0:
                continue
            rest = raw[close + 2:].split()
            pid = int(entry.name)
            ticks[pid] = int(rest[11]) + int(rest[12])
            parents[pid] = int(rest[1])
            names[pid] = raw[raw.find("(") + 1:close]
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
    hz = os.sysconf(os.sysconf_names["SC_CLK_TCK"])
    owned = descendants(owned_root, parents) if owned_root is not None else set()
    rows = []
    for pid, ticks in new.items():
        delta = ticks - old.get(pid, ticks)
        if delta > 0 and pid not in owned:
            rows.append({"pid": pid, "name": names.get(pid, "?"), "cores": delta / hz / elapsed})
    rows.sort(key=lambda row: row["cores"], reverse=True)
    return {"elapsed_seconds": elapsed, "non_owned_cores": sum(row["cores"] for row in rows), "top": rows[:20]}


class InterferenceMonitor:
    def __init__(self, output):
        self.output = output
        self.samples = []
        self.rejected = False
        self.error = None
        self.stop_event = threading.Event()
        self.thread = threading.Thread(target=self.run, daemon=True)

    def run(self):
        try:
            previous = proc_snapshot()
            while not self.stop_event.wait(5.0):
                current = proc_snapshot()
                sample = cpu_delta(previous, current, os.getpid())
                sample["unix_time"] = time.time()
                self.samples.append(sample)
                self.rejected = self.rejected or sample["non_owned_cores"] > 1.0
                previous = current
        except Exception as error:
            self.error = f"{type(error).__name__}: {error}"

    def start(self):
        self.thread.start()

    def finish(self):
        self.stop_event.set()
        self.thread.join()
        atomic_json(self.output, {"schema": "ngn-counter-fusion-interference-v1", "sample_seconds": 5,
                                  "threshold_cores": 1.0, "timing_evidence_rejected": self.rejected,
                                  "monitor_error": self.error,
                                  "samples": self.samples})


def stage(monitor_path, root, label, executable, env, argv, cpu_monitor):
    directory = root / "stages" / label
    directory.mkdir(parents=True)
    command = [
        "/usr/bin/taskset", "-c", "6", sys.executable, str(monitor_path), "--label", label, "--limit-seconds", "300",
        "--term-grace-seconds", "3", "--sample-interval-seconds", "0.25",
        "--memory-limit-kib", "1048576", "--cpu-list", "4",
        "--stdout", str(directory / "stdout.txt"), "--stderr", str(directory / "stderr.txt"),
        "--time-output", str(directory / "time.txt"), "--samples", str(directory / "samples.jsonl"),
        "--receipt", str(directory / "receipt.json"), "--", "/usr/bin/env", "-i",
        *[f"{key}={value}" for key, value in env.items()], str(executable), *argv,
    ]
    atomic_json(directory / "command.json", command)
    result = subprocess.run(command, cwd=directory, check=False)
    if result.returncode:
        die(f"stage {label} failed rc={result.returncode}")
    receipt = json.loads((directory / "receipt.json").read_text())
    if (receipt.get("state") != "COMPLETE" or receipt.get("command_returncode") != 0
            or receipt.get("supervisor_returncode") != 0 or receipt.get("termination_reason") is not None
            or receipt.get("monitor_error") is not None or receipt.get("surviving_processes") != []):
        die(f"stage {label} has non-clean supervisor receipt")
    if (directory / "stderr.txt").stat().st_size:
        die(f"stage {label} wrote stderr")
    if cpu_monitor.rejected:
        die("non-owned CPU exceeded one core over a five-second sample")
    if cpu_monitor.error:
        die(f"interference monitor failed: {cpu_monitor.error}")
    return directory


def verify_correctness_stdout(path):
    lines = path.read_text().splitlines()
    if any(line.lstrip().startswith("--- SKIP:") or line.lstrip().startswith("--- FAIL:") for line in lines):
        die(f"correctness test skipped or failed: {path}")
    runs, passes = set(), set()
    for line in lines:
        match = re.fullmatch(r"=== RUN   (Test[A-Za-z0-9_]+)", line)
        if match:
            runs.add(match.group(1))
        match = re.fullmatch(r"--- PASS: (Test[A-Za-z0-9_]+) \([^)]*\)", line)
        if match:
            passes.add(match.group(1))
    if runs != CORRECTNESS_NAMES or passes != CORRECTNESS_NAMES or not lines or lines[-1] != "PASS":
        die(f"wrong correctness RUN/PASS set: {path}: runs={sorted(runs)} passes={sorted(passes)}")


def parse_benchmark_family(path, expected_family):
    lines = path.read_text().splitlines()
    if (not lines or lines[-1] != "PASS"
            or any(line.startswith("=== RUN") or line.startswith("--- PASS:")
                   or line.startswith("--- FAIL:") or line.startswith("--- SKIP:") for line in lines)):
        die(f"benchmark stage did not contain benchmark-only PASS output: {path}")
    pattern = re.compile(r"^(BenchmarkCounterProfile(?:Cold|Persistent))/([a-z0-9_]+)(?:-\d+)?\s+(\d+)\s+(.+)$")
    found = {}
    for line in lines:
        if not line.startswith("Benchmark"):
            continue
        match = pattern.fullmatch(line.strip())
        if not match:
            die(f"unexpected benchmark row: {line}")
        benchmark, fixture, iterations, metrics_text = match.groups()
        if iterations != "1":
            die(f"benchmark row did not run exactly once: {line}")
        tokens = metrics_text.split()
        if len(tokens) % 2:
            die(f"malformed benchmark metrics: {line}")
        metrics = {}
        for index in range(0, len(tokens), 2):
            value_text, unit = tokens[index:index + 2]
            if unit in metrics:
                die(f"duplicate benchmark metric {unit}: {line}")
            try:
                value = float(value_text)
            except ValueError:
                die(f"non-numeric benchmark metric {value_text}: {line}")
            if not math.isfinite(value) or value < 0:
                die(f"invalid benchmark metric {value_text}: {line}")
            metrics[unit] = value
        family = "cold" if benchmark.endswith("Cold") else "persistent"
        if family != expected_family:
            die(f"unexpected benchmark family {family}, expected {expected_family}: {line}")
        expected_metrics = {"ns/op", "minimum_nodes/op", "B/op", "allocs/op"}
        if family == "persistent":
            expected_metrics.add("searches/op")
        if set(metrics) != expected_metrics or metrics["ns/op"] <= 0:
            die(f"wrong benchmark metrics for {family}/{fixture}: {metrics}")
        expected_nodes = 400000 if family == "cold" else 800000
        if metrics["minimum_nodes/op"] != expected_nodes:
            die(f"wrong node floor for {family}/{fixture}: {metrics['minimum_nodes/op']}")
        if family == "persistent" and metrics["searches/op"] != 2:
            die(f"wrong search count for persistent/{fixture}: {metrics['searches/op']}")
        if not metrics["B/op"].is_integer() or not metrics["allocs/op"].is_integer():
            die(f"non-integral allocation metric for {family}/{fixture}: {metrics}")
        if fixture in found:
            die(f"duplicate benchmark row {family}/{fixture}: {path}")
        found[fixture] = {"ns": metrics["ns/op"], "B/op": int(metrics["B/op"]),
                          "allocs/op": int(metrics["allocs/op"])}
    expected_fixtures = COLD_FIXTURES if expected_family == "cold" else PERSISTENT_FIXTURES
    if list(found) != expected_fixtures:
        die(f"wrong {expected_family} benchmark rows/order: {path}: {list(found)}")
    return found


def summarize_family(rows, fixtures):
    fixture_medians = {fixture: statistics.median(row["ratios"][fixture] for row in rows) for fixture in fixtures}
    equal_weight = statistics.median(fixture_medians.values())
    order_medians = {order: statistics.median(row["ratios"][fixture]
                     for row in rows if row["order"] == order for fixture in fixtures)
                     for order in ("base-first", "candidate-first")}
    passed = (equal_weight <= 0.97 and all(value < 1 for value in fixture_medians.values())
              and all(value < 1 for value in order_medians.values()))
    return {"fixture_medians": fixture_medians, "equal_weight_median": equal_weight,
            "order_medians": order_medians, "passed": passed}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("manifest", type=Path)
    parser.add_argument("--check-held", action="store_true")
    args = parser.parse_args()
    if sys.platform != "linux" or "microsoft" not in Path("/proc/version").read_text().lower():
        die("execution is restricted to WSL")
    try:
        os.sched_setaffinity(0, {6})
        affinity = os.sched_getaffinity(0)
    except (AttributeError, OSError) as error:
        die(f"cannot pin driver to supervisor CPU 6: {error}")
    if affinity != {6}:
        die(f"driver affinity is {sorted(affinity)}, expected [6]")
    manifest = json.loads(args.manifest.read_text())
    if manifest.get("schema") != "ngn-counter-fusion-gate-v1":
        die("wrong manifest schema")
    if sha256(Path(__file__).resolve()) != manifest.get("driver_sha256"):
        die("driver hash mismatch")
    if manifest.get("protocol") != {"blocks": 10, "benchtime": "1x", "child_cpu": "4",
                                      "supervisor_cpu": "6", "gomaxprocs": "1",
                                      "test_selector": "CounterFusionGateNoTests"}:
        die("protocol changed")
    family_gate = {"max_equal_weight_median_ratio": 0.97,
                   "each_fixture_median_below": 1.0,
                   "each_order_stratum_below": 1.0}
    if manifest.get("gates") != {"cold": family_gate, "persistent": family_gate}:
        die("gates changed")
    sources = {name: frozen(spec, name) for name, spec in manifest["frozen_inputs"].items()}
    if args.check_held:
        print(json.dumps({"state": "HELD_STATIC_CHECK_COMPLETE", "frozen_inputs": len(sources)}, sort_keys=True))
        return
    if manifest.get("release_state") != "RELEASED_AFTER_INDEPENDENT_REVIEW":
        die("manifest is HELD")
    run_root = Path(manifest["run_root"]).resolve()
    if run_root.exists():
        die(f"run root exists: {run_root}")
    before = proc_snapshot()
    time.sleep(5)
    prelaunch = cpu_delta(before, proc_snapshot(), None)
    prelaunch_path = args.manifest.resolve().with_suffix(".prelaunch-cpu.json")
    atomic_json(prelaunch_path, prelaunch)
    if prelaunch["non_owned_cores"] > 1.0:
        die(f"prelaunch non-owned CPU exceeds one core: {prelaunch['non_owned_cores']}")
    run_root.mkdir(parents=True)
    (run_root / "inputs").mkdir()
    (run_root / "stages").mkdir()
    atomic_json(run_root / "frozen-manifest.json", manifest)
    shutil.copy2(prelaunch_path, run_root / "prelaunch-cpu.json")
    copy_frozen(Path(__file__).resolve(), run_root / "inputs/driver.py", manifest["driver_sha256"])
    copied = {}
    for name, source in sources.items():
        destination = run_root / "inputs" / (name.replace("_", "-") + source.suffix)
        copy_frozen(source, destination, manifest["frozen_inputs"][name]["sha256"])
        copied[name] = destination
    model = copied["model"]
    oracle = copied["oracle_json"]
    prefixes = copied["profile_prefixes_json"]
    common_env = {"PATH": "/usr/bin:/bin", "LANG": "C", "LC_ALL": "C", "TZ": "UTC",
                  "GOMAXPROCS": "1", "GOGC": "100", "GOMEMLIMIT": "off", "GODEBUG": "",
                  "GOTOOLCHAIN": "local", "GOFLAGS": "-mod=readonly", "TMPDIR": str(run_root / "tmp"),
                  "COUNTER_MODEL": str(model), "COUNTER_ORACLE_JSON": str(oracle),
                  "NGN_COUNTER_PROFILE_PREFIXES": str(prefixes)}
    (run_root / "tmp").mkdir()
    cpu_monitor = InterferenceMonitor(run_root / "non-owned-cpu.json")
    cpu_monitor.start()
    outcome = {"schema": "ngn-counter-fusion-gate-decision-v1", "state": "RUNNING"}
    execution_error = None
    try:
        correctness = {"cold": {}, "persistent": {}}
        for role in ("base", "candidate"):
            cold_output = run_root / f"correctness-cold-{role}.json"
            persistent_output = run_root / f"correctness-persistent-{role}.json"
            env = {**common_env, "NGN_COUNTER55_PROFILE_MODEL": str(model),
                   "NGN_COUNTER55_PROFILE_OUTPUT": str(cold_output),
                   "NGN_COUNTER_PROFILE_OUTPUT": str(persistent_output)}
            directory = stage(copied["process_monitor"], run_root, f"correctness-{role}", copied[f"{role}_engine_test"], env,
                              ["-test.run", CORRECTNESS, "-test.count=1", "-test.v"], cpu_monitor)
            verify_correctness_stdout(directory / "stdout.txt")
            correctness["cold"][role] = cold_output
            correctness["persistent"][role] = persistent_output
        for family in ("cold", "persistent"):
            if correctness[family]["base"].read_bytes() != correctness[family]["candidate"].read_bytes():
                die(f"base/candidate {family} correctness receipts differ")

        results = {"cold": [], "persistent": []}
        for block in range(1, 11):
            order = ("base", "candidate") if block % 2 else ("candidate", "base")
            for family, benchmark, fixtures in (
                    ("cold", "BenchmarkCounterProfileCold", COLD_FIXTURES),
                    ("persistent", "BenchmarkCounterProfilePersistent", PERSISTENT_FIXTURES)):
                values = {}
                for slot, role in enumerate(order, 1):
                    label = f"{family}-b{block:02d}-s{slot}-{role}"
                    env = {**common_env, "NGN_COUNTER55_PROFILE_MODEL": str(model)}
                    directory = stage(copied["process_monitor"], run_root, label, copied[f"{role}_engine_test"], env,
                                      ["-test.run", "CounterFusionGateNoTests", "-test.bench", benchmark,
                                       "-test.benchtime=1x", "-test.count=1", "-test.benchmem"], cpu_monitor)
                    values[role] = parse_benchmark_family(directory / "stdout.txt", family)
                ratios = {fixture: values["candidate"][fixture]["ns"] / values["base"][fixture]["ns"]
                          for fixture in fixtures}
                for fixture in fixtures:
                    if (values["candidate"][fixture]["B/op"] != values["base"][fixture]["B/op"]
                            or values["candidate"][fixture]["allocs/op"] != values["base"][fixture]["allocs/op"]):
                        die(f"{family} allocation identity failed for {fixture} block {block}")
                results[family].append({"block": block, "order": f"{order[0]}-first",
                                        "ratios": ratios, "raw": values})
        summaries = {family: summarize_family(results[family], fixtures) for family, fixtures in
                     (("cold", COLD_FIXTURES), ("persistent", PERSISTENT_FIXTURES))}
        decision = "PASS_PERFORMANCE" if all(summary["passed"] for summary in summaries.values()) else "SHELVE_PERFORMANCE"
        outcome = {"schema": "ngn-counter-fusion-gate-decision-v1", "state": "COMPLETE", "decision": decision,
                   "families": {family: {"runs": results[family], **summaries[family]}
                                for family in ("cold", "persistent")},
                   "correctness_sha256": {family: sha256(correctness[family]["base"])
                                           for family in ("cold", "persistent")}}
    except BaseException as error:
        execution_error = error
        outcome.update({"state": "FAILED", "decision": "FAILED_EXECUTION",
                        "error": f"{type(error).__name__}: {error}"})
    finally:
        cpu_monitor.finish()
    if cpu_monitor.error:
        outcome.update({"state": "FAILED", "decision": "INVALID_MONITOR", "monitor_error": cpu_monitor.error})
    elif cpu_monitor.rejected:
        outcome.update({"state": "FAILED", "decision": "INVALID_INTERFERENCE"})
    receipts = list((run_root / "stages").glob("*/receipt.json"))
    if outcome["state"] == "COMPLETE":
        expected_receipts = 42
        if len(receipts) != expected_receipts:
            outcome.update({"state": "FAILED", "decision": "FAILED_RECEIPT_COUNT",
                            "expected_stage_receipts": expected_receipts,
                            "actual_stage_receipts": len(receipts)})
    decision_path = run_root / "decision.json"
    atomic_json(decision_path, outcome)
    inventory = {}
    for path in sorted(run_root.rglob("*")):
        if path.is_file() and path.name not in {"inventory.json", "terminal.json"}:
            inventory[str(path.relative_to(run_root))] = sha256(path)
    inventory_path = run_root / "inventory.json"
    atomic_json(inventory_path, {"schema": "ngn-counter-fusion-inventory-v1", "files": inventory})
    atomic_json(run_root / "terminal.json", {"schema": "ngn-counter-fusion-terminal-v1", "state": outcome["state"],
                "decision": outcome["decision"], "driver_sha256": manifest["driver_sha256"],
                "manifest_sha256": sha256(run_root / "frozen-manifest.json"), "decision_sha256": sha256(decision_path),
                "inventory_sha256": sha256(inventory_path), "verified_clean_stage_receipts": len(receipts),
                "timing_evidence_rejected": cpu_monitor.rejected})
    if outcome["state"] != "COMPLETE":
        if execution_error is not None:
            raise execution_error
        die(f"measurement failed closed: {outcome['decision']}")


if __name__ == "__main__":
    main()
