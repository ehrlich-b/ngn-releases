#!/usr/bin/env python3
"""Independent, WSL-only verifier for frozen Counter fusion run-001."""
import hashlib
import json
import math
import os
from pathlib import Path
import re
import statistics
import sys

RUN = Path("/home/ehrli/repos/ngn-counter-fusion-gate-20260913/run-001")
COLD = ["start_white", "start_black", "kiwipete_white", "kiwipete_black", "asymmetric_white", "asymmetric_black"]
PERSISTENT = ["game_1", "game_2", "game_3"]
TESTS = {
    "TestCounterTransitionFixedNodeSnapshot",
    "TestCounterTransitionAdapterSpecialMovesAndDirectBoards",
    "TestCounterTransitionAdapterRejectsInvalidState",
    "TestCounterMoveDeltaDirectMatchesSemanticReferenceForIllegalAndMalformedMoves",
    "TestCounterProfilePersistentSnapshot",
}


def sha(path):
    h = hashlib.sha256()
    with path.open("rb") as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b""):
            h.update(block)
    return h.hexdigest()


def require(condition, message):
    if not condition:
        raise SystemExit(f"fusion-review: {message}")


def load(path):
    return json.loads(path.read_text())


def parse_rows(path, family):
    fixtures = COLD if family == "cold" else PERSISTENT
    name = "BenchmarkCounterProfileCold" if family == "cold" else "BenchmarkCounterProfilePersistent"
    pattern = re.compile(rf"^{name}/([a-z0-9_]+)(?:-\d+)?\s+(\d+)\s+(.+)$")
    lines = path.read_text().splitlines()
    require(lines and lines[-1] == "PASS", f"no terminal PASS in {path}")
    require(not any(line.startswith(("=== RUN", "--- PASS:", "--- FAIL:", "--- SKIP:")) for line in lines),
            f"ordinary test event in benchmark output {path}")
    rows = {}
    for line in lines:
        if not line.startswith("Benchmark"):
            continue
        match = pattern.fullmatch(line.strip())
        require(match is not None, f"unexpected benchmark row {line}")
        fixture, iterations, rest = match.groups()
        require(iterations == "1" and fixture not in rows, f"iteration/duplicate violation {line}")
        tokens = rest.split()
        require(len(tokens) % 2 == 0, f"odd metric token count {line}")
        metrics = {}
        for index in range(0, len(tokens), 2):
            value, unit = float(tokens[index]), tokens[index + 1]
            require(unit not in metrics and math.isfinite(value) and value >= 0, f"bad metric {line}")
            metrics[unit] = value
        expected = {"ns/op", "minimum_nodes/op", "B/op", "allocs/op"}
        if family == "persistent":
            expected.add("searches/op")
        require(set(metrics) == expected and metrics["ns/op"] > 0, f"metric set/value mismatch {line}")
        require(metrics["minimum_nodes/op"] == (400000 if family == "cold" else 800000), f"node count mismatch {line}")
        require(family == "cold" or metrics["searches/op"] == 2, f"search count mismatch {line}")
        require(metrics["B/op"].is_integer() and metrics["allocs/op"].is_integer(), f"allocation not integral {line}")
        rows[fixture] = {"ns": metrics["ns/op"], "B/op": int(metrics["B/op"]),
                         "allocs/op": int(metrics["allocs/op"])}
    require(list(rows) == fixtures, f"wrong {family} row order/set in {path}: {list(rows)}")
    return rows


def close(a, b):
    return math.isclose(a, b, rel_tol=1e-15, abs_tol=0.0)


def main():
    require(sys.platform == "linux" and "microsoft" in Path("/proc/version").read_text().lower(), "WSL only")
    manifest = load(RUN / "frozen-manifest.json")
    inventory = load(RUN / "inventory.json")
    terminal = load(RUN / "terminal.json")
    recorded = inventory["files"]
    actual = {str(path.relative_to(RUN)) for path in RUN.rglob("*") if path.is_file()}
    require(actual == set(recorded) | {"inventory.json", "terminal.json"},
            f"inventory file-set mismatch extra={sorted(actual-set(recorded)-{'inventory.json','terminal.json'})}")
    for relative, digest in recorded.items():
        require(sha(RUN / relative) == digest, f"inventory digest mismatch {relative}")
    require(sha(RUN / "decision.json") == terminal["decision_sha256"], "decision digest mismatch")
    require(sha(RUN / "inventory.json") == terminal["inventory_sha256"], "inventory digest mismatch")
    require(sha(RUN / "frozen-manifest.json") == terminal["manifest_sha256"], "manifest digest mismatch")
    require(sha(RUN / "inputs/driver.py") == terminal["driver_sha256"] == manifest["driver_sha256"], "driver digest mismatch")

    copied = {}
    for key, spec in manifest["frozen_inputs"].items():
        source = Path(spec["path"])
        destination = RUN / "inputs" / (key.replace("_", "-") + source.suffix)
        require(sha(source) == spec["sha256"], f"external frozen input changed {key}")
        require(sha(destination) == spec["sha256"], f"run-local frozen input mismatch {key}")
        copied[key] = destination
    expected_inputs = {path.relative_to(RUN).as_posix() for path in copied.values()} | {"inputs/driver.py", "inputs/ngn_crashes.log"}
    present_inputs = {path.relative_to(RUN).as_posix() for path in (RUN / "inputs").iterdir() if path.is_file()}
    require(present_inputs == expected_inputs, f"unexpected input inventory: {sorted(present_inputs-expected_inputs)}")

    for role in ("base", "candidate"):
        directory = RUN / "stages" / f"correctness-{role}"
        receipt = load(directory / "receipt.json")
        require(receipt["state"] == "COMPLETE" and receipt["command_returncode"] == 0
                and receipt["supervisor_returncode"] == 0 and receipt["termination_reason"] is None
                and receipt["monitor_error"] is None and receipt["surviving_processes"] == [], f"bad correctness receipt {role}")
        lines = (directory / "stdout.txt").read_text().splitlines()
        runs = {m.group(1) for line in lines if (m := re.fullmatch(r"=== RUN   (Test[A-Za-z0-9_]+)", line))}
        passes = {m.group(1) for line in lines if (m := re.fullmatch(r"--- PASS: (Test[A-Za-z0-9_]+) \([^)]*\)", line))}
        require(runs == TESTS and passes == TESTS and lines[-1] == "PASS", f"wrong correctness set {role}")
    require((RUN / "correctness-cold-base.json").read_bytes() == (RUN / "correctness-cold-candidate.json").read_bytes(),
            "cold snapshots differ")
    require((RUN / "correctness-persistent-base.json").read_bytes() == (RUN / "correctness-persistent-candidate.json").read_bytes(),
            "persistent snapshots differ")

    reconstructed = {}
    all_receipts = list((RUN / "stages").glob("*/receipt.json"))
    require(len(all_receipts) == 42, f"receipt count {len(all_receipts)}")
    for family, fixtures in (("cold", COLD), ("persistent", PERSISTENT)):
        runs = []
        benchmark = "BenchmarkCounterProfileCold" if family == "cold" else "BenchmarkCounterProfilePersistent"
        for block in range(1, 11):
            order = ("base", "candidate") if block % 2 else ("candidate", "base")
            values = {}
            for slot, role in enumerate(order, 1):
                label = f"{family}-b{block:02d}-s{slot}-{role}"
                directory = RUN / "stages" / label
                receipt = load(directory / "receipt.json")
                require(receipt["state"] == "COMPLETE" and receipt["command_returncode"] == 0
                        and receipt["supervisor_returncode"] == 0 and receipt["termination_reason"] is None
                        and receipt["monitor_error"] is None and receipt["surviving_processes"] == []
                        and receipt["cpu_list"] == "4" and receipt["stage"] == label, f"bad receipt {label}")
                command = receipt["command"]
                require(command[-8:] == [str(copied[f"{role}_engine_test"]), "-test.run", "CounterFusionGateNoTests",
                                         "-test.bench", benchmark, "-test.benchtime=1x", "-test.count=1", "-test.benchmem"],
                        f"wrong inner argv {label}")
                require((directory / "stderr.txt").stat().st_size == 0, f"stderr nonempty {label}")
                values[role] = parse_rows(directory / "stdout.txt", family)
            ratios = {}
            for fixture in fixtures:
                require(values["candidate"][fixture]["B/op"] == values["base"][fixture]["B/op"]
                        and values["candidate"][fixture]["allocs/op"] == values["base"][fixture]["allocs/op"],
                        f"allocation mismatch {family}/{fixture}/b{block}")
                ratios[fixture] = values["candidate"][fixture]["ns"] / values["base"][fixture]["ns"]
            runs.append({"block": block, "order": f"{order[0]}-first", "ratios": ratios})
        fixture_medians = {fixture: statistics.median(run["ratios"][fixture] for run in runs) for fixture in fixtures}
        equal_weight = statistics.median(fixture_medians.values())
        order_medians = {order: statistics.median(run["ratios"][fixture] for run in runs if run["order"] == order for fixture in fixtures)
                         for order in ("base-first", "candidate-first")}
        reconstructed[family] = {"equal_weight_median": equal_weight, "fixture_medians": fixture_medians,
                                 "order_medians": order_medians,
                                 "all_fixture_medians_improve": all(value < 1 for value in fixture_medians.values()),
                                 "all_order_medians_improve": all(value < 1 for value in order_medians.values()),
                                 "passes_0_97": equal_weight <= 0.97}

    decision = load(RUN / "decision.json")
    for family in ("cold", "persistent"):
        got, recorded_family = reconstructed[family], decision["families"][family]
        require(close(got["equal_weight_median"], recorded_family["equal_weight_median"]), f"aggregate mismatch {family}")
        for key in got["fixture_medians"]:
            require(close(got["fixture_medians"][key], recorded_family["fixture_medians"][key]), f"fixture mismatch {family}/{key}")
        for key in got["order_medians"]:
            require(close(got["order_medians"][key], recorded_family["order_medians"][key]), f"order mismatch {family}/{key}")
    require(decision["decision"] == terminal["decision"] == "SHELVE_PERFORMANCE", "wrong decision")
    require(all(not value["passes_0_97"] for value in reconstructed.values()), "a family unexpectedly passed")
    require(all(value["all_fixture_medians_improve"] and value["all_order_medians_improve"] for value in reconstructed.values()),
            "reported subordinate improvements not reproduced")

    interference = load(RUN / "non-owned-cpu.json")
    prelaunch = load(RUN / "prelaunch-cpu.json")
    require(interference["monitor_error"] is None and not interference["timing_evidence_rejected"]
            and max(sample["non_owned_cores"] for sample in interference["samples"]) <= 1.0
            and prelaunch["non_owned_cores"] <= 1.0, "interference receipt rejected")
    live = []
    frozen_binary_names = {copied["base_engine_test"].name, copied["candidate_engine_test"].name}
    for proc in Path("/proc").iterdir():
        if not proc.name.isdigit():
            continue
        try:
            if (proc / "exe").resolve().name in frozen_binary_names:
                live.append(int(proc.name))
        except (FileNotFoundError, PermissionError):
            pass
    require(not live, f"frozen benchmark process remains live: {live}")

    result = {
        "schema": "ngn-counter-fusion-independent-review-v1",
        "state": "PASS_AUDIT",
        "decision": "SHELVE_PERFORMANCE",
        "run_root": str(RUN),
        "terminal_sha256": sha(RUN / "terminal.json"),
        "manifest_sha256": sha(RUN / "frozen-manifest.json"),
        "inventory_sha256": sha(RUN / "inventory.json"),
        "decision_sha256": sha(RUN / "decision.json"),
        "stage_receipts": len(all_receipts),
        "snapshots_byte_equal": {"cold": True, "persistent": True},
        "allocations_equal": True,
        "timing_evidence_rejected": False,
        "cleanup_live_frozen_processes": live,
        "families": reconstructed,
        "note": "Both families improve in every per-fixture median and order stratum, but independently miss the predeclared <=0.97 aggregate gate; no rounding, pooling, extension, or rerun is valid.",
        "generated_crash_log": {"path": "inputs/ngn_crashes.log", "sha256": sha(RUN / "inputs/ngn_crashes.log"),
                                "classification": "42 process-start initialization messages; inventory-bound, outside timed benchmark regions"},
    }
    output = Path(sys.argv[1]) if len(sys.argv) > 1 else RUN.parent / "review-001" / "review-v1.json"
    output.parent.mkdir(parents=True, exist_ok=False)
    output.write_text(json.dumps(result, indent=2, sort_keys=True) + "\n")
    print(json.dumps({"state": result["state"], "decision": result["decision"], "output": str(output)}, sort_keys=True))


if __name__ == "__main__":
    main()
