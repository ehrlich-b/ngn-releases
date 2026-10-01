#!/usr/bin/env python3
"""Held WSL-only Rodent accumulator-update AVX2 performance gate."""
import argparse
import hashlib
import importlib.util
import json
import math
import os
from pathlib import Path
import re
import shutil
import statistics
import sys
import threading
import time

FIXTURES = ["start_white", "start_black", "kiwipete_white", "kiwipete_black", "asymmetric_white", "asymmetric_black"]
DIRECT_CLASSES = ["all", "as", "ass", "asas"]
SNAPSHOT_TEST = "TestRodentOutputFixedNodeSnapshot"


def die(message):
    raise SystemExit(f"rodent-update-gate: {message}")


def sha256(path):
    digest = hashlib.sha256()
    with open(path, "rb") as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def load_gate_library(path, expected):
    if not path.is_file() or not re.fullmatch(r"[0-9a-f]{64}", expected) or sha256(path) != expected:
        die(f"gate library is absent or changed: {path}")
    spec = importlib.util.spec_from_file_location("counter_output_gate_library", path)
    if spec is None or spec.loader is None:
        die(f"cannot load gate library: {path}")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    module.die = die
    return module


def verify_named_test_stdout(path, expected):
    lines = path.read_text().splitlines()
    if any(line.lstrip().startswith("--- SKIP:") or line.lstrip().startswith("--- FAIL:") for line in lines):
        die(f"required test skipped or failed: {path}")
    runs, passes = set(), set()
    for line in lines:
        run = re.fullmatch(r"=== RUN   (Test[A-Za-z0-9_]+)", line)
        if run:
            runs.add(run.group(1))
        passed = re.fullmatch(r"--- PASS: (Test[A-Za-z0-9_]+) \([^)]*\)", line)
        if passed:
            passes.add(passed.group(1))
    expected = set(expected)
    if runs != expected or passes != expected or not lines or lines[-1] != "PASS":
        die(f"wrong required RUN/PASS set: {path}: runs={sorted(runs)} passes={sorted(passes)} expected={sorted(expected)}")


def parse_direct(path, role):
    pattern = re.compile(
        rf"^BenchmarkRodentApplyUpdatesMicrokernel/{role}/(all|as|ass|asas)(?:-\d+)?\s+\d+\s+"
        r"([0-9]+(?:\.[0-9]+)?) ns/op\s+(\d+) B/op\s+(\d+) allocs/op$"
    )
    found = {}
    for line in path.read_text().splitlines():
        match = pattern.fullmatch(line.strip())
        if not match:
            continue
        record_class, ns, bytes_op, allocs_op = match.groups()
        if record_class in found:
            die(f"duplicate direct class {role}/{record_class}: {path}")
        value = float(ns)
        if record_class not in DIRECT_CLASSES or not math.isfinite(value) or value <= 0:
            die(f"invalid direct row {role}/{record_class} ns/op={ns}: {path}")
        if bytes_op != "0" or allocs_op != "0":
            die(f"direct row allocated {role}/{record_class}: B/op={bytes_op} allocs/op={allocs_op}")
        found[record_class] = value
    if list(found) != DIRECT_CLASSES:
        die(f"wrong direct class rows/order for {role}: {path}: {list(found)}")
    return found


def parse_search(path):
    pattern = re.compile(
        r"^BenchmarkRodentV11ColdFixedNodes/([a-z_]+)(?:-\d+)?\s+1\s+"
        r"([0-9]+(?:\.[0-9]+)?) ns/op\s+400000 minimum_nodes/op\s+(\d+) B/op\s+(\d+) allocs/op$"
    )
    found = {}
    for line in path.read_text().splitlines():
        match = pattern.fullmatch(line.strip())
        if not match:
            continue
        fixture, ns, bytes_op, allocs_op = match.groups()
        if fixture in found:
            die(f"duplicate search fixture {fixture}: {path}")
        value = float(ns)
        if fixture not in FIXTURES or not math.isfinite(value) or value <= 0:
            die(f"invalid search row {fixture} ns/op={ns}: {path}")
        found[fixture] = {"ns": value, "B/op": int(bytes_op), "allocs/op": int(allocs_op)}
    if list(found) != FIXTURES:
        die(f"wrong search fixture rows/order: {path}: {list(found)}")
    return found


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
    if manifest.get("schema") != "ngn-rodent-update-gate-v1":
        die("wrong manifest schema")
    if sha256(Path(__file__).resolve()) != manifest.get("driver_sha256"):
        die("driver hash mismatch")
    protocol = {
        "blocks": 10, "direct_benchtime": "250ms", "search_benchtime": "1x",
        "child_cpu": "4", "supervisor_cpu": "6", "gomaxprocs": "1",
    }
    gates = {
        "direct_each_class_below": 1.0,
        "direct_each_order_stratum_below": 1.0,
        "search_max_equal_weight_median_ratio": 0.97,
        "search_each_fixture_below": 1.0,
        "search_each_order_stratum_below": 1.0,
        "allocation_policy": "exact_identity",
    }
    if manifest.get("protocol") != protocol or manifest.get("gates") != gates:
        die("protocol or gates changed")
    required_tests = manifest.get("required_tests", {})
    engine_names = required_tests.get("engine", [])
    rodent_names = required_tests.get("rodenteval", {})
    base_rodent_names = rodent_names.get("base", [])
    candidate_rodent_names = rodent_names.get("candidate", [])
    if (engine_names != [SNAPSHOT_TEST]
            or not base_rodent_names or len(set(base_rodent_names)) != len(base_rodent_names)
            or not candidate_rodent_names or len(set(candidate_rodent_names)) != len(candidate_rodent_names)
            or not set(base_rodent_names).issubset(candidate_rodent_names)):
        die("required test names are absent, duplicated, or changed")

    library_path = Path(__file__).with_name("counter_output_gate_library.py").resolve()
    gate = load_gate_library(library_path, manifest.get("gate_library_sha256", ""))
    sources = {name: gate.frozen(spec, name) for name, spec in manifest["frozen_inputs"].items()}
    if sources.get("gate_library") != library_path:
        die("frozen gate library path is not the imported sibling")
    if args.check_held:
        print(json.dumps({"state": "HELD_STATIC_CHECK_COMPLETE", "frozen_inputs": len(sources)}, sort_keys=True))
        return
    if manifest.get("release_state") != "RELEASED_AFTER_INDEPENDENT_REVIEW":
        die("manifest is HELD")

    run_root = Path(manifest["run_root"]).resolve()
    if run_root.exists():
        die(f"run root exists: {run_root}")
    before = gate.proc_snapshot()
    time.sleep(5)
    prelaunch = gate.cpu_delta(before, gate.proc_snapshot(), None)
    prelaunch_path = args.manifest.resolve().with_suffix(".prelaunch-cpu.json")
    gate.atomic_json(prelaunch_path, prelaunch)
    if prelaunch["non_owned_cores"] > 1.0:
        die(f"prelaunch non-owned CPU exceeds one core: {prelaunch['non_owned_cores']}")

    run_root.mkdir(parents=True)
    (run_root / "inputs").mkdir()
    (run_root / "stages").mkdir()
    (run_root / "tmp").mkdir()
    gate.atomic_json(run_root / "frozen-manifest.json", manifest)
    shutil.copy2(prelaunch_path, run_root / "prelaunch-cpu.json")
    gate.copy_frozen(Path(__file__).resolve(), run_root / "inputs/driver.py", manifest["driver_sha256"])
    copied = {}
    for name, source in sources.items():
        destination = run_root / "inputs" / (name.replace("_", "-") + source.suffix)
        gate.copy_frozen(source, destination, manifest["frozen_inputs"][name]["sha256"])
        copied[name] = destination

    common_env = {
        "PATH": "/usr/bin:/bin", "LANG": "C", "LC_ALL": "C", "TZ": "UTC",
        "GOMAXPROCS": "1", "GOGC": "100", "GOMEMLIMIT": "off", "GODEBUG": "",
        "GOTOOLCHAIN": "local", "GOFLAGS": "-mod=readonly", "TMPDIR": str(run_root / "tmp"),
        "RODENT_V11_ANAND_MODEL": str(copied["model"]),
        "RODENT_V11_ANAND_ORACLE_JSON": str(copied["release_oracle"]),
        "RODENT_V11_ANAND_TRANSITION_FIXTURES": str(copied["transition_fixtures"]),
        "RODENT_V11_ANAND_TAGGED_TRANSITION_ORACLE": str(copied["tagged_transition_oracle"]),
        "RODENT_V11_ANAND_RELEASE_TRANSITION_ORACLE": str(copied["release_transition_oracle"]),
    }

    class RodentInterferenceMonitor(gate.InterferenceMonitor):
        def finish(self):
            self.stop_event.set()
            self.thread.join()
            gate.atomic_json(self.output, {
                "schema": "ngn-rodent-update-interference-v1", "sample_seconds": 5,
                "threshold_cores": 1.0, "timing_evidence_rejected": self.rejected,
                "monitor_error": self.error, "samples": self.samples,
            })

    cpu_monitor = RodentInterferenceMonitor(run_root / "non-owned-cpu.json")
    cpu_monitor.start()
    outcome = {"schema": "ngn-rodent-update-gate-decision-v1", "state": "RUNNING"}
    execution_error = None
    try:
        snapshots = {}
        engine_regex = "^(" + "|".join(re.escape(name) for name in engine_names) + ")$"
        for role in ("base", "candidate"):
            output = run_root / f"search-identity-{role}.json"
            env = {**common_env, "NGN_RODENT_OUTPUT_GATE_RECEIPT": str(output)}
            directory = gate.stage(copied["process_monitor"], run_root, f"engine-correctness-{role}",
                                   copied[f"{role}_engine_test"], env,
                                   ["-test.run", engine_regex, "-test.count=1", "-test.v"], cpu_monitor)
            verify_named_test_stdout(directory / "stdout.txt", engine_names)
            snapshots[role] = output
        if snapshots["base"].read_bytes() != snapshots["candidate"].read_bytes():
            die("base/candidate complete cold+warm search identity differs")
        for role in ("base", "candidate"):
            role_names = base_rodent_names if role == "base" else candidate_rodent_names
            rodent_regex = "^(" + "|".join(re.escape(name) for name in role_names) + ")$"
            directory = gate.stage(copied["process_monitor"], run_root, f"rodenteval-correctness-{role}",
                                   copied[f"{role}_rodenteval_test"], common_env,
                                   ["-test.run", rodent_regex, "-test.count=1", "-test.v"], cpu_monitor)
            verify_named_test_stdout(directory / "stdout.txt", role_names)

        direct = []
        for block in range(1, 11):
            order = ("original_generic", "selected") if block % 2 else ("selected", "original_generic")
            order_label = "original-first" if order[0] == "original_generic" else "selected-first"
            values = {}
            for slot, role in enumerate(order, 1):
                label = f"direct-b{block:02d}-s{slot}-{role}"
                env = {**common_env, "NGN_RODENT_UPDATE_BENCH_ORDER": order_label}
                directory = gate.stage(copied["process_monitor"], run_root, label,
                                       copied["candidate_rodenteval_test"], env,
                                       ["-test.run", "RodentUpdateGateNoTestsSelected", "-test.bench",
                                        f"BenchmarkRodentApplyUpdatesMicrokernel/{role}",
                                        "-test.benchtime=250ms", "-test.count=1", "-test.benchmem"], cpu_monitor)
                values[role] = parse_direct(directory / "stdout.txt", role)
            ratios = {record_class: values["selected"][record_class] / values["original_generic"][record_class]
                      for record_class in DIRECT_CLASSES}
            direct.append({"block": block, "order": order_label, "ratios": ratios, "raw": values})

        direct_medians = {record_class: statistics.median(row["ratios"][record_class] for row in direct)
                          for record_class in DIRECT_CLASSES}
        direct_order_medians = {
            order: statistics.median(row["ratios"]["all"] for row in direct if row["order"] == order)
            for order in ("original-first", "selected-first")
        }
        direct_pass = (all(direct_medians[name] < 1.0 for name in DIRECT_CLASSES)
                       and all(value < 1.0 for value in direct_order_medians.values()))
        if not direct_pass:
            outcome.update({
                "state": "COMPLETE", "decision": "SHELVE_DIRECT_KERNEL",
                "search_identity_sha256": sha256(snapshots["base"]), "direct": direct,
                "direct_medians": direct_medians, "direct_order_medians": direct_order_medians,
            })
        else:
            search = []
            for block in range(1, 11):
                order = ("base", "candidate") if block % 2 else ("candidate", "base")
                values = {}
                for slot, role in enumerate(order, 1):
                    label = f"search-b{block:02d}-s{slot}-{role}"
                    directory = gate.stage(copied["process_monitor"], run_root, label,
                                           copied[f"{role}_engine_test"], common_env,
                                           ["-test.run", "RodentUpdateGateNoTestsSelected", "-test.bench",
                                            "BenchmarkRodentV11ColdFixedNodes", "-test.benchtime=1x",
                                            "-test.count=1", "-test.benchmem"], cpu_monitor)
                    values[role] = parse_search(directory / "stdout.txt")
                ratios = {fixture: values["candidate"][fixture]["ns"] / values["base"][fixture]["ns"]
                          for fixture in FIXTURES}
                for fixture in FIXTURES:
                    base_alloc = (values["base"][fixture]["B/op"], values["base"][fixture]["allocs/op"])
                    candidate_alloc = (values["candidate"][fixture]["B/op"], values["candidate"][fixture]["allocs/op"])
                    if candidate_alloc != base_alloc:
                        die(f"unexplained search allocation change for {fixture} block {block}: {candidate_alloc} != {base_alloc}")
                search.append({"block": block, "order": f"{order[0]}-first", "ratios": ratios, "raw": values})
            fixture_medians = {fixture: statistics.median(row["ratios"][fixture] for row in search) for fixture in FIXTURES}
            equal_weight = statistics.median(fixture_medians.values())
            order_medians = {
                order: statistics.median(row["ratios"][fixture] for row in search if row["order"] == order for fixture in FIXTURES)
                for order in ("base-first", "candidate-first")
            }
            passed = (equal_weight <= 0.97 and all(value < 1.0 for value in fixture_medians.values())
                      and all(value < 1.0 for value in order_medians.values()))
            outcome.update({
                "state": "COMPLETE", "decision": "PASS_PERFORMANCE" if passed else "SHELVE_SEARCH",
                "search_identity_sha256": sha256(snapshots["base"]), "direct": direct,
                "direct_medians": direct_medians, "direct_order_medians": direct_order_medians,
                "search": search, "fixture_medians": fixture_medians,
                "equal_weight_median": equal_weight, "order_medians": order_medians,
            })
    except BaseException as error:
        execution_error = error
        outcome.update({"state": "FAILED", "decision": "FAILED_EXECUTION", "error": f"{type(error).__name__}: {error}"})
    finally:
        cpu_monitor.finish()

    if cpu_monitor.error:
        outcome.update({"state": "FAILED", "decision": "INVALID_MONITOR", "monitor_error": cpu_monitor.error})
    elif cpu_monitor.rejected:
        outcome.update({"state": "FAILED", "decision": "INVALID_INTERFERENCE"})
    receipts = list((run_root / "stages").glob("*/receipt.json"))
    if outcome["state"] == "COMPLETE":
        expected_receipts = 24 if outcome["decision"] == "SHELVE_DIRECT_KERNEL" else 44
        if len(receipts) != expected_receipts:
            outcome.update({"state": "FAILED", "decision": "FAILED_RECEIPT_COUNT",
                            "expected_stage_receipts": expected_receipts, "actual_stage_receipts": len(receipts)})
    decision_path = run_root / "decision.json"
    gate.atomic_json(decision_path, outcome)
    inventory = {}
    for path in sorted(run_root.rglob("*")):
        if path.is_file() and path.name not in {"inventory.json", "terminal.json"}:
            inventory[str(path.relative_to(run_root))] = sha256(path)
    inventory_path = run_root / "inventory.json"
    gate.atomic_json(inventory_path, {"schema": "ngn-rodent-update-inventory-v1", "files": inventory})
    gate.atomic_json(run_root / "terminal.json", {
        "schema": "ngn-rodent-update-terminal-v1", "state": outcome["state"],
        "decision": outcome["decision"], "driver_sha256": manifest["driver_sha256"],
        "manifest_sha256": sha256(run_root / "frozen-manifest.json"),
        "decision_sha256": sha256(decision_path), "inventory_sha256": sha256(inventory_path),
        "verified_clean_stage_receipts": len(receipts), "timing_evidence_rejected": cpu_monitor.rejected,
    })
    if outcome["state"] != "COMPLETE":
        if execution_error is not None:
            raise execution_error
        die(f"measurement failed closed: {outcome['decision']}")


if __name__ == "__main__":
    main()
