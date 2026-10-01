#!/usr/bin/env python3
"""Independent, read-only verifier for the frozen Counter output gate."""
import hashlib
import json
import math
from pathlib import Path
import re
import statistics
import sys

RUN = Path("/home/ehrli/repos/ngn-counter-output-gate-20260913/run-001")
OUT = Path("/home/ehrli/repos/ngn-counter-output-gate-20260913/review-001/review-v2.json")
FIXTURES = ["start_white", "start_black", "kiwipete_white", "kiwipete_black", "asymmetric_white", "asymmetric_black"]
DIRECT_RE = re.compile(r"^BenchmarkCounterOutputDotCapturedCorpus/(portable|selected)(?:-\d+)?\s+\d+\s+([0-9]+(?:\.[0-9]+)?) ns/op\s+(\d+) B/op\s+(\d+) allocs/op$")
SEARCH_RE = re.compile(r"^BenchmarkCounterTransitionFixedNodes/([a-z_]+)(?:-\d+)?\s+1\s+([0-9]+(?:\.[0-9]+)?) ns/op\s+400000 minimum_nodes/op\s+(\d+) B/op\s+(\d+) allocs/op$")


def fail(message):
    raise AssertionError(message)


def require(condition, message):
    if not condition:
        fail(message)


def sha256(path):
    digest = hashlib.sha256()
    with path.open("rb") as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def load(path):
    return json.loads(path.read_text())


def close(a, b):
    return math.isclose(a, b, rel_tol=0, abs_tol=1e-15)


def main():
    require(sys.platform == "linux" and "microsoft" in Path("/proc/version").read_text().lower(), "WSL only")
    require(RUN.is_dir() and OUT.parent.is_dir() and not OUT.exists(), "run/review paths not in expected state")
    terminal = load(RUN / "terminal.json")
    decision = load(RUN / "decision.json")
    manifest = load(RUN / "frozen-manifest.json")
    inventory_doc = load(RUN / "inventory.json")

    require(terminal["schema"] == "ngn-counter-output-terminal-v1", "terminal schema")
    require(terminal["state"] == "COMPLETE" and terminal["decision"] == "PASS_PERFORMANCE", "terminal decision")
    require(sha256(RUN / "decision.json") == terminal["decision_sha256"], "decision digest")
    require(sha256(RUN / "inventory.json") == terminal["inventory_sha256"], "inventory digest")
    require(sha256(RUN / "frozen-manifest.json") == terminal["manifest_sha256"], "manifest digest")
    require(sha256(RUN / "inputs/driver.py") == terminal["driver_sha256"] == manifest["driver_sha256"], "driver digest")

    inventory = inventory_doc["files"]
    actual = {}
    for path in sorted(RUN.rglob("*")):
        if path.is_file() and path.name not in {"inventory.json", "terminal.json"}:
            actual[str(path.relative_to(RUN))] = sha256(path)
    require(actual == inventory, "inventory membership or file digest mismatch")

    copied_inputs = {}
    external_inputs = {}
    for name, spec in manifest["frozen_inputs"].items():
        external = Path(spec["path"])
        require(external.is_file(), f"missing external input {name}")
        external_inputs[name] = sha256(external)
        require(external_inputs[name] == spec["sha256"], f"external input hash {name}")
        copied = RUN / "inputs" / (name.replace("_", "-") + external.suffix)
        require(copied.is_file(), f"missing copied input {name}: {copied}")
        copied_inputs[name] = sha256(copied)
        require(copied_inputs[name] == spec["sha256"], f"copied input hash {name}")

    candidate_receipt = load(RUN / "inputs/candidate-build-test-receipt.json")
    base_receipt = load(RUN / "inputs/base-build-receipt.json")
    require(candidate_receipt["base_commit"] == manifest["base_source"]["commit"], "candidate base commit")
    require(base_receipt["source"]["commit"] == manifest["base_source"]["commit"], "base commit")
    require(base_receipt["source"]["tree"] == manifest["base_source"]["tree"], "base tree")
    require(candidate_receipt["source"]["candidate_patch"]["sha256"] == manifest["frozen_inputs"]["candidate_patch"]["sha256"], "candidate receipt patch binding")
    require(candidate_receipt["binaries"]["engine_test"]["sha256"] == manifest["frozen_inputs"]["candidate_engine_test"]["sha256"], "candidate engine binding")
    require(candidate_receipt["binaries"]["countereval_test"]["sha256"] == manifest["frozen_inputs"]["candidate_countereval_test"]["sha256"], "candidate countereval binding")
    require(base_receipt["binary"]["sha256"] == manifest["frozen_inputs"]["base_engine_test"]["sha256"], "base engine binding")
    require(candidate_receipt["source"]["profile_fixture"]["sha256"] == manifest["frozen_inputs"]["profile_test_source"]["sha256"], "candidate profile source binding")
    require(manifest["frozen_inputs"]["candidate_profile_test_source"]["sha256"] == manifest["frozen_inputs"]["profile_test_source"]["sha256"], "base/candidate profile source differs")

    patch_paths = []
    for line in (RUN / "inputs/candidate-patch.patch").read_text().splitlines():
        match = re.fullmatch(r"diff --git a/(.+) b/(.+)", line)
        if match:
            require(match.group(1) == match.group(2), "patch rename")
            patch_paths.append(match.group(1))
    allowed_patch_paths = [
        "countereval/context.go", "countereval/model.go", "countereval/output_dot.go",
        "countereval/output_dot_amd64_v3.go", "countereval/output_dot_amd64_v3.s",
        "countereval/output_dot_amd64_v3_test.go", "countereval/output_dot_benchmark_test.go",
        "countereval/output_dot_fallback.go", "countereval/output_dot_fallback_test.go",
        "countereval/output_dot_test.go", "engine/counter_transition_profile_test.go",
    ]
    require(patch_paths == allowed_patch_paths, f"candidate patch scope {patch_paths}")

    correctness_base = (RUN / "correctness-base.json").read_bytes()
    correctness_candidate = (RUN / "correctness-candidate.json").read_bytes()
    require(correctness_base == correctness_candidate, "base/candidate correctness receipts differ")
    correctness_hash = hashlib.sha256(correctness_base).hexdigest()
    require(correctness_hash == decision["correctness_sha256"], "correctness hash differs from decision")

    expected_stages = {"correctness-base", "correctness-candidate"}
    for block in range(1, 11):
        direct_order = ("portable", "selected") if block % 2 else ("selected", "portable")
        search_order = ("base", "candidate") if block % 2 else ("candidate", "base")
        for slot, role in enumerate(direct_order, 1):
            expected_stages.add(f"direct-b{block:02d}-s{slot}-{role}")
        for slot, role in enumerate(search_order, 1):
            expected_stages.add(f"search-b{block:02d}-s{slot}-{role}")
    stage_dirs = {path.name: path for path in (RUN / "stages").iterdir() if path.is_dir()}
    require(set(stage_dirs) == expected_stages and len(stage_dirs) == 42, "stage set/count")
    require(terminal["verified_clean_stage_receipts"] == 42, "terminal receipt count")

    required_env = {"GOMAXPROCS=1", "GOGC=100", "GOMEMLIMIT=off", "GODEBUG=", "PATH=/usr/bin:/bin", "LANG=C", "LC_ALL=C", "TZ=UTC"}
    stage_intervals = []
    for label, directory in stage_dirs.items():
        receipt = load(directory / "receipt.json")
        require(receipt["stage"] == label and receipt["state"] == "COMPLETE", f"stage state {label}")
        require(receipt["command_returncode"] == 0 and receipt["supervisor_returncode"] == 0, f"stage rc {label}")
        require(receipt["termination_reason"] is None and receipt["monitor_error"] is None, f"stage error {label}")
        require(receipt["surviving_processes"] == [] and not receipt["orphan_detected"], f"stage cleanup {label}")
        require(receipt["cpu_list"] == "4" and receipt["valid_samples"] > 0, f"stage cpu/samples {label}")
        require((directory / "stderr.txt").stat().st_size == 0, f"stderr {label}")
        wrapper = load(directory / "command.json")
        require(wrapper[:3] == ["/usr/bin/taskset", "-c", "6"], f"supervisor affinity {label}")
        require("--" in wrapper, f"missing monitor delimiter {label}")
        delimiter = wrapper.index("--")
        require(wrapper[delimiter + 1:] == receipt["command"], f"inner command receipt mismatch {label}")
        command = receipt["command"]
        env_index = command.index("/usr/bin/env")
        require(command[env_index + 1] == "-i" and required_env.issubset(set(command[env_index + 2:])), f"environment {label}")
        stage_intervals.append((receipt["started_unix_time"], receipt["ended_unix_time"], label))
    stage_intervals.sort()
    for previous, current in zip(stage_intervals, stage_intervals[1:]):
        require(previous[1] <= current[0], f"overlapping stages {previous[2]} {current[2]}")

    prelaunch = load(RUN / "prelaunch-cpu.json")
    interference = load(RUN / "non-owned-cpu.json")
    require(prelaunch["non_owned_cores"] <= 1.0, "prelaunch interference")
    require(not interference["timing_evidence_rejected"] and interference["monitor_error"] is None, "runtime interference status")
    max_non_owned = max((sample["non_owned_cores"] for sample in interference["samples"]), default=0.0)
    require(max_non_owned <= 1.0 and not terminal["timing_evidence_rejected"], "runtime interference threshold")

    direct = []
    search = []
    for block in range(1, 11):
        direct_order = ("portable", "selected") if block % 2 else ("selected", "portable")
        direct_values = {}
        for slot, role in enumerate(direct_order, 1):
            stdout = stage_dirs[f"direct-b{block:02d}-s{slot}-{role}"] / "stdout.txt"
            matches = [DIRECT_RE.fullmatch(line.strip()) for line in stdout.read_text().splitlines()]
            matches = [match for match in matches if match]
            require(len(matches) == 1 and matches[0].group(1) == role, f"direct row {block} {role}")
            ns, bytes_op, allocs = float(matches[0].group(2)), int(matches[0].group(3)), int(matches[0].group(4))
            require(ns > 0 and bytes_op == 0 and allocs == 0, f"direct metrics {block} {role}")
            direct_values[role] = ns
        direct.append({"block": block, "order": f"{direct_order[0]}-first", **direct_values,
                       "ratio_selected_portable": direct_values["selected"] / direct_values["portable"]})

        search_order = ("base", "candidate") if block % 2 else ("candidate", "base")
        search_values = {}
        for slot, role in enumerate(search_order, 1):
            stdout = stage_dirs[f"search-b{block:02d}-s{slot}-{role}"] / "stdout.txt"
            rows = []
            for line in stdout.read_text().splitlines():
                match = SEARCH_RE.fullmatch(line.strip())
                if match:
                    rows.append(match.groups())
            require([row[0] for row in rows] == FIXTURES, f"search rows {block} {role}")
            require(len({row[0] for row in rows}) == 6, f"duplicate search rows {block} {role}")
            search_values[role] = {name: {"ns": float(ns), "B/op": int(b), "allocs/op": int(a)} for name, ns, b, a in rows}
        ratios = {}
        for fixture in FIXTURES:
            base = search_values["base"][fixture]
            candidate = search_values["candidate"][fixture]
            require(base["B/op"] == candidate["B/op"] and base["allocs/op"] == candidate["allocs/op"], f"allocation parity {block} {fixture}")
            ratios[fixture] = candidate["ns"] / base["ns"]
        search.append({"block": block, "order": f"{search_order[0]}-first", "ratios": ratios, "raw": search_values})

    direct_median = statistics.median(row["ratio_selected_portable"] for row in direct)
    direct_orders = {order: statistics.median(row["ratio_selected_portable"] for row in direct if row["order"] == order)
                     for order in ("portable-first", "selected-first")}
    fixture_medians = {fixture: statistics.median(row["ratios"][fixture] for row in search) for fixture in FIXTURES}
    equal_weight = statistics.median(fixture_medians.values())
    order_medians = {order: statistics.median(row["ratios"][fixture] for row in search if row["order"] == order for fixture in FIXTURES)
                     for order in ("base-first", "candidate-first")}
    require(close(direct_median, decision["direct_median_ratio"]), "direct median")
    require(all(close(direct_orders[key], decision["direct_order_medians"][key]) for key in direct_orders), "direct order medians")
    require(all(close(fixture_medians[key], decision["fixture_medians"][key]) for key in fixture_medians), "fixture medians")
    require(close(equal_weight, decision["equal_weight_median"]), "equal-weight median")
    require(all(close(order_medians[key], decision["order_medians"][key]) for key in order_medians), "search order medians")
    for got, recorded in zip(direct, decision["direct"]):
        require(got["block"] == recorded["block"] and got["order"] == recorded["order"] and
                close(got["portable"], recorded["portable"]) and close(got["selected"], recorded["selected"]) and
                close(got["ratio_selected_portable"], recorded["ratio_selected_portable"]), f"direct decision row {got['block']}")
    for got, recorded in zip(search, decision["search"]):
        require(got["block"] == recorded["block"] and got["order"] == recorded["order"], f"search decision row {got['block']}")
        for fixture in FIXTURES:
            require(close(got["ratios"][fixture], recorded["ratios"][fixture]), f"search decision ratio {got['block']} {fixture}")

    gates = manifest["gates"]
    passed = (direct_median <= gates["direct_max_median_ratio"] and all(value < gates["direct_each_order_stratum_below"] for value in direct_orders.values())
              and equal_weight <= gates["search_max_equal_weight_median_ratio"]
              and all(value < gates["search_each_fixture_below"] for value in fixture_medians.values())
              and all(value < gates["search_each_order_stratum_below"] for value in order_medians.values()))
    require(passed, "independent gate decision")
    all_search_ratios = [row["ratios"][fixture] for row in search for fixture in FIXTURES]
    result = {
        "schema": "ngn-counter-output-independent-review-v1",
        "state": "ACCEPT",
        "run": str(RUN),
        "terminal_sha256": sha256(RUN / "terminal.json"),
        "manifest_sha256": sha256(RUN / "frozen-manifest.json"),
        "decision_sha256": sha256(RUN / "decision.json"),
        "inventory_sha256": sha256(RUN / "inventory.json"),
        "verified_inventory_files": len(inventory),
        "verified_external_inputs": len(external_inputs),
        "verified_stage_receipts": len(stage_dirs),
        "correctness_receipts_byte_identical": True,
        "correctness_sha256": correctness_hash,
        "candidate_patch_paths": patch_paths,
        "direct": {"median_ratio": direct_median, "speedup": 1 / direct_median,
                   "order_medians": direct_orders, "ratio_range": [min(row["ratio_selected_portable"] for row in direct), max(row["ratio_selected_portable"] for row in direct)]},
        "search": {"equal_weight_median_ratio": equal_weight, "time_reduction": 1 - equal_weight,
                   "fixture_medians": fixture_medians, "order_medians": order_medians,
                   "all_pair_ratio_range": [min(all_search_ratios), max(all_search_ratios)]},
        "operations": {"prelaunch_non_owned_cores": prelaunch["non_owned_cores"],
                       "max_sampled_non_owned_cores": max_non_owned,
                       "interference_samples": len(interference["samples"]),
                       "stages_non_overlapping": True},
        "interpretation": "The direct 12-record corpus measures only the output fold; the fixed-node search ratio measures the whole engine. Their different effect sizes are expected and are not pooled.",
    }
    OUT.write_text(json.dumps(result, indent=2, sort_keys=True) + "\n")
    print(json.dumps(result, sort_keys=True))


if __name__ == "__main__":
    main()
