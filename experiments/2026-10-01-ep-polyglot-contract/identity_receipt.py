#!/usr/bin/env python3
"""Emit the immutable inputs, source identities, and enforced task controls."""

from __future__ import annotations

import hashlib
import json
import os
import subprocess
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
TASK = ROOT.parent
BASELINE = TASK / "polyglot-baseline-proof8b"
PYCHESS = Path(
    "/home/ehrli/nnue-owned-continuation-20260928/"
    "counterdraw-deps/chess-1.11.2/chess"
)
CGROUP = Path(
    "/sys/fs/cgroup/user.slice/user-1000.slice/user@1000.service/app.slice/"
    "ngn-personal-polyglot-contract-20261001.service"
)


def sha256(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def git(path: Path, *args: str) -> str:
    return subprocess.check_output(["git", "-C", path, *args], text=True).strip()


oracle = TASK / "polyglot-independent-oracle.json"
fixture = ROOT / "engine/testdata/polyglot_special_move_oracle.json"
baseline_fixture = BASELINE / "engine/testdata/polyglot_special_move_oracle.json"
test = ROOT / "engine/polyglot_external_contract_test.go"
baseline_test = BASELINE / "engine/polyglot_external_contract_test.go"

receipt = {
    "schema": "ngn-ep-polyglot-contract-identity-v1",
    "candidate": {
        "branch": git(ROOT, "rev-parse", "--abbrev-ref", "HEAD"),
        "head": git(ROOT, "rev-parse", "HEAD"),
    },
    "baseline": {"head": git(BASELINE, "rev-parse", "HEAD")},
    "oracle": {"bytes": oracle.stat().st_size, "sha256": sha256(oracle)},
    "fixture": {"bytes": fixture.stat().st_size, "sha256": sha256(fixture)},
    "baseline_fixture_sha256": sha256(baseline_fixture),
    "test_sha256": sha256(test),
    "baseline_test_sha256": sha256(baseline_test),
    "python_chess": {
        "core_sha256": sha256(PYCHESS / "__init__.py"),
        "polyglot_sha256": sha256(PYCHESS / "polyglot.py"),
    },
    "resources": {
        "affinity": sorted(os.sched_getaffinity(0)),
        "nice": os.nice(0),
        "cpu_max": (CGROUP / "cpu.max").read_text().strip(),
        "memory_max": (CGROUP / "memory.max").read_text().strip(),
        "GOMAXPROCS": os.environ.get("GOMAXPROCS"),
        "GOFLAGS": os.environ.get("GOFLAGS"),
        "proc_cgroup": Path("/proc/self/cgroup").read_text().strip(),
    },
}

assert receipt["candidate"] == {
    "branch": "task/ep-polyglot-contract-20261001",
    "head": "7ce0b410a6e7919ff85eff7c38b84bd9f9d82d6e",
}
assert receipt["baseline"]["head"] == "8b41845aae29757c031e889f88de55a7064bc8b6"
assert receipt["oracle"]["sha256"] == "3e7acd53328562ab5fb6943441d586f8bb3723e269c1904c4f128ecee63d05fc"
assert receipt["oracle"] == receipt["fixture"]
assert receipt["fixture"]["sha256"] == receipt["baseline_fixture_sha256"]
assert receipt["test_sha256"] == receipt["baseline_test_sha256"]
assert receipt["python_chess"] == {
    "core_sha256": "1fde6a8e932508d14b31f6a584497b49c2969fc3c7cdc675347f819f0d667a5b",
    "polyglot_sha256": "8dc20733bdc1297a9e8993a76737ba4f328a99185ae94b311ddbb5342fce32fc",
}
assert receipt["resources"]["affinity"] == [0, 2]
assert receipt["resources"]["nice"] == 10
assert receipt["resources"]["cpu_max"] == "50000 100000"
assert receipt["resources"]["memory_max"] == "4294967296"

print(json.dumps(receipt, indent=2, sort_keys=True))
