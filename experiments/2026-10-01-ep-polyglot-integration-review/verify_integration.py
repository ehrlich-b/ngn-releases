#!/usr/bin/env python3
"""Verify component identity and retained combined integration receipts."""

from __future__ import annotations

import hashlib
import json
import os
import re
import subprocess
from collections import Counter
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
TASK = ROOT.parent
EVIDENCE = Path(__file__).resolve().parent / "evidence"
MODEL = Path("/home/ehrli/nnue-owned-k4-20260920/night-20260927/runs/wdl25-e10.nnue")


def sha256(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def output(name: str) -> str:
    return (EVIDENCE / f"{name}.stdout").read_text()


def exit_code(name: str) -> int:
    return int((EVIDENCE / f"{name}.exit").read_text())


materialization = json.loads((EVIDENCE / "integration-materialization.json").read_text())
assert materialization["base"] == "28d66322debef23d01645a1bc73d7e942ada9813"
assert materialization["book_component_commit"] == "780c89dfb3177c115e682a8c1524ee8101ccaaec"
assert subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=ROOT, text=True).strip() == materialization["base"]
assert subprocess.check_output(["git", "branch", "--show-current"], cwd=ROOT, text=True).strip() == "task/ep-polyglot-integration-review-20261001"

component_hashes = {}
for row in materialization["files"]:
    path = ROOT / row["path"]
    worktree_hash = sha256(path)
    blob = subprocess.check_output(
        ["git", "show", f"{materialization['book_component_commit']}:{row['path']}"],
        cwd=ROOT,
    )
    blob_hash = hashlib.sha256(blob).hexdigest()
    assert worktree_hash == blob_hash == row["sha256"], row["path"]
    component_hashes[row["path"]] = worktree_hash

base_production_hashes = {}
for name in ("engine/hash.go", "engine/position.go"):
    path = ROOT / name
    blob = subprocess.check_output(["git", "show", f"{materialization['base']}:{name}"], cwd=ROOT)
    assert path.read_bytes() == blob, name
    base_production_hashes[name] = sha256(path)

tracked_changes = subprocess.check_output(["git", "diff", "--name-only", "HEAD"], cwd=ROOT, text=True).splitlines()
assert tracked_changes == [
    "engine/book_test.go",
    "engine/enpassant_repetition_correctness_test.go",
    "engine/polyglot.go",
    "engine/polyglot_external_contract_test.go",
]

assert sha256(TASK / "integration-independent-key-transition.json") == sha256(EVIDENCE / "integration-independent-key-transition.json") == "172796acc6b60cc450fe5614734d03c5068d3b5fa4be208e51dc4a44d284623d"
assert sha256(MODEL) == "1ec8fc1737ddfdd5f8b6ff0b4e26778085fb563f7c5e82c1fc5d3ea454b6ec29"
assert sha256(ROOT / "engine/testdata/polyglot_special_move_oracle.json") == "3e7acd53328562ab5fb6943441d586f8bb3723e269c1904c4f128ecee63d05fc"
assert sha256(ROOT / "engine/testdata/polyglot_book_move_oracle.json") == "8289efed4fd50a58a6bff4a816c9b0ecdc7cea7d7d7acc7dcf0a17d26de993ba"

before = output("failing-before-combined")
assert exit_code("failing-before-combined") == 1
failure_pairs = re.findall(
    r"Polyglot hash = ([0-9A-F]{16}), want proof-baseline ([0-9A-F]{16})",
    before,
)
transition_oracle = json.loads((EVIDENCE / "integration-independent-key-transition.json").read_text())
ep_pairs = Counter(
    (row["standard_key"].upper(), row["legacy_key"].upper())
    for row in transition_oracle["rows"]
    if row["value_field"] == "wantPolyglotEP"
)
assert Counter(failure_pairs) == Counter({pair: 2 for pair in ep_pairs})
assert len(failure_pairs) == 12
assert "book_checks=302 mismatches=0" in before
assert "cases=23 exact_decodes=23 decode_failures=0" in before
assert "--- PASS: TestIllegalEnPassantPlayedThreefold" in before
assert "--- PASS: TestEnPassantKeyTransitionRestoration" in before
assert "--- PASS: TestLegalEnPassantPredicateEdgesAndPurity" in before
assert exit_code("before-nonnumeric-ep-controls") == 0

focused = output("passing-focused-combined")
assert exit_code("passing-focused-combined") == 0
assert "--- FAIL:" not in focused and "--- SKIP:" not in focused
for test in (
    "TestLegalEnPassantOrdinaryHashAndPinnedOracle",
    "TestIllegalEnPassantPlayedThreefold",
    "TestEnPassantKeyTransitionRestoration",
    "TestLegalEnPassantPredicateEdgesAndPurity",
    "TestEnPassantSecondCapturerCanBeLegal",
    "TestEnPassantNullDescendantRestoresKey",
    "TestOwnedK4EnPassantRepetitionReadiness",
    "TestCanonicalPolyglotRandomTable",
    "TestExternalPolyglotSpecialMoveContract",
    "TestExternalPolyglotBookMoveContract",
    "TestPolyglotOrthodoxCastleDestinationCompatibility",
):
    assert f"--- PASS: {test}" in focused, test
assert "book_checks=302 mismatches=0" in focused
assert "cases=23 exact_decodes=23 decode_failures=0" in focused
assert "runtime_perft2_nodes=7651 oracle_perft2_nodes=7651" in focused
owned_leaf_passes = [
    line
    for line in focused.splitlines()
    if "--- PASS: TestOwnedK4EnPassantRepetitionReadiness/" in line
    and line.count("/") == 2
]
assert len(owned_leaf_passes) == 16, len(owned_leaf_passes)
assert focused.count("depth-2 search restored caller after") == 4

suite_names = (
    "short-engine-actual-net",
    "short-race-engine-actual-net",
    "short-all-actual-net",
    "short-race-all-actual-net",
)
suite_exits = {}
for name in suite_names:
    suite_exits[name] = exit_code(name)
    assert suite_exits[name] == 0, name
    command = (EVIDENCE / f"{name}.command").read_text()
    assert f"NGN_EP_OWNED_NNUE={MODEL}" in command
    assert "GOTOOLCHAIN=local" in command and "GOMAXPROCS=1" in command and "GOFLAGS=-p=1" in command

cgroup = Path("/sys/fs/cgroup") / Path("/proc/self/cgroup").read_text().strip().split("::", 1)[1].lstrip("/")
resources = {
    "affinity": sorted(os.sched_getaffinity(0)),
    "nice": os.getpriority(os.PRIO_PROCESS, 0),
    "cpu_max": (cgroup / "cpu.max").read_text().strip(),
    "memory_max": (cgroup / "memory.max").read_text().strip(),
    "GOMAXPROCS": os.environ.get("GOMAXPROCS"),
    "GOFLAGS": os.environ.get("GOFLAGS"),
    "proc_cgroup": Path("/proc/self/cgroup").read_text().strip(),
}
assert resources["affinity"] == [0, 2]
assert resources["nice"] == 10
assert resources["cpu_max"] == "50000 100000"
assert resources["memory_max"] == "4294967296"
assert resources["GOMAXPROCS"] == "1" and resources["GOFLAGS"] == "-p=1"

print(
    json.dumps(
        {
            "state": "EP_POLYGLOT_COMBINED_REVIEW_PASS",
            "source": {
                "base": materialization["base"],
                "book_component": materialization["book_component_commit"],
                "component_hashes": component_hashes,
                "base_production_hashes": base_production_hashes,
                "active_ep_fixture_sha256": sha256(ROOT / "engine/enpassant_repetition_correctness_test.go"),
            },
            "inputs": {
                "transition_oracle_sha256": sha256(EVIDENCE / "integration-independent-key-transition.json"),
                "owned_model_sha256": sha256(MODEL),
            },
            "baseline": {
                "exit": exit_code("failing-before-combined"),
                "legacy_numeric_assertion_failures": len(failure_pairs),
                "nonnumeric_ep_controls_exit": exit_code("before-nonnumeric-ep-controls"),
            },
            "focused": {
                "exit": exit_code("passing-focused-combined"),
                "owned_nnue_leaf_cases_non_skipped": len(owned_leaf_passes),
                "external_book_key_observations": 302,
                "external_book_move_cases": 23,
                "external_book_move_perft2_nodes": 7651,
            },
            "suite_exits": suite_exits,
            "resources": resources,
        },
        indent=2,
        sort_keys=True,
    )
)
