#!/usr/bin/env python3
"""Compare candidate/baseline focused Polyglot diagnostic receipts."""

from __future__ import annotations

import hashlib
import json
import re
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
TASK = ROOT.parent
EVIDENCE = Path(__file__).resolve().parent / "evidence"
BASELINE = TASK / "polyglot-baseline-proof8b"
SUMMARY = re.compile(
    r"histories=(\d+) frozen_states=(\d+) frozen_perft2_nodes=(\d+) "
    r"book_checks=(\d+) mismatches=(\d+) actual_sequence_sha256=([0-9a-f]{64})"
)
START = re.compile(r"standard-startpos: got=([0-9a-f]{16}) want=([0-9a-f]{16})")


def sha256(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def parse(name: str) -> dict[str, object]:
    stdout = (EVIDENCE / f"{name}-focus.stdout").read_text()
    summary = SUMMARY.search(stdout)
    start = START.search(stdout)
    assert summary and start, name
    exit_code = int((EVIDENCE / f"{name}-focus.exit").read_text())
    assert exit_code == 1, (name, exit_code)
    return {
        "exit": exit_code,
        "histories": int(summary.group(1)),
        "frozen_states": int(summary.group(2)),
        "frozen_perft2_nodes": int(summary.group(3)),
        "book_checks": int(summary.group(4)),
        "mismatches": int(summary.group(5)),
        "actual_sequence_sha256": summary.group(6),
        "startpos_got": start.group(1),
        "startpos_want": start.group(2),
    }


candidate = parse("candidate")
baseline = parse("baseline")
assert candidate == baseline
assert candidate == {
    "exit": 1,
    "histories": 29,
    "frozen_states": 99,
    "frozen_perft2_nodes": 10836,
    "book_checks": 302,
    "mismatches": 302,
    "actual_sequence_sha256": "3f89792dd707913f365e2f0554b862c816469466d20cf6df47452097c52b14f7",
    "startpos_got": "4b9aa9e5e768fe35",
    "startpos_want": "463b96181691fc9c",
}

candidate_test = ROOT / "engine/polyglot_external_contract_test.go"
baseline_test = BASELINE / "engine/polyglot_external_contract_test.go"
candidate_fixture = ROOT / "engine/testdata/polyglot_special_move_oracle.json"
baseline_fixture = BASELINE / "engine/testdata/polyglot_special_move_oracle.json"
assert candidate_test.read_bytes() == baseline_test.read_bytes()
assert candidate_fixture.read_bytes() == baseline_fixture.read_bytes()
assert sha256(candidate_fixture) == "3e7acd53328562ab5fb6943441d586f8bb3723e269c1904c4f128ecee63d05fc"

print(
    json.dumps(
        {
            "state": "CANDIDATE_AND_BASELINE_IDENTICAL_NONSTANDARD_BOOK_KEYS",
            "candidate": candidate,
            "baseline": baseline,
            "test_sha256": sha256(candidate_test),
            "fixture_sha256": sha256(candidate_fixture),
        },
        indent=2,
        sort_keys=True,
    )
)
