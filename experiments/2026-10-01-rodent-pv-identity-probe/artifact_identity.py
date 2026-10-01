#!/usr/bin/env python3
"""Verify the immutable Rodent probe inputs and print a compact JSON receipt."""

from __future__ import annotations

import hashlib
import json
import os
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
TASK = ROOT.parent
RUN = Path(
    "/home/ehrli/repos/ngn-external-opponents-v1/output/"
    "external-fixed-comparisons-20260908-attempt4/"
    "rodent-v1.2-nontal-testers-t1-30p0d3"
)
BINARY = RUN / "inputs/opponent/engine"
PYCHESS = Path(
    "/home/ehrli/nnue-owned-continuation-20260928/"
    "counterdraw-deps/chess-1.11.2/chess/__init__.py"
)
SOURCE = TASK / "rodent-pinned-renderer-source"
CGROUP = Path(
    "/sys/fs/cgroup/user.slice/user-1000.slice/user@1000.service/app.slice/"
    "ngn-personal-rodent-probe-20261001.service"
)


def digest(path: Path) -> str:
    h = hashlib.sha256()
    with path.open("rb") as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b""):
            h.update(chunk)
    return h.hexdigest()


def checked(path: Path, expected: str, expected_bytes: int | None = None) -> dict:
    actual = digest(path)
    size = path.stat().st_size
    assert actual == expected, (path, actual, expected)
    if expected_bytes is not None:
        assert size == expected_bytes, (path, size, expected_bytes)
    return {"path": str(path), "bytes": size, "sha256": actual}


binary = checked(
    BINARY,
    "9cfb8195207ee5695c1973a89664ab73b34b5bcbc10ad3bc0f0afe28b9713cbc",
    7_311_522,
)
with BINARY.open("rb") as stream:
    stream.seek(2_538_080)
    network_bytes = stream.read(4_744_768)
assert len(network_bytes) == 4_744_768
network_sha = hashlib.sha256(network_bytes).hexdigest()
assert network_sha == "c35a1abc1b8c1cb1d5f4221454d494c1a6da1ed9088fd51ab27038bfa74b5053"

warning_path = TASK / "rodent-probe-inputs/warning.json"
warning = json.loads(warning_path.read_text())
trace = checked(RUN / "fastchess.log.zst", warning["compressed_trace_sha256"])
config = json.loads((RUN / "configs/match-stage.json").read_text())
opponent = next(role for role in config["roles"] if role["id"] == "external")
assert opponent["engine_sha256"] == binary["sha256"]
manifest_line = next(
    line
    for line in (RUN / "receipts/final-files.sha256").read_text().splitlines()
    if line.endswith("/inputs/opponent/engine")
)
assert manifest_line.split()[0] == binary["sha256"]

frozen = {}
for name in ("warning.json", "frontier-mechanisms.json", "notation-source.json"):
    path = TASK / "rodent-probe-inputs" / name
    frozen[name] = {"bytes": path.stat().st_size, "sha256": digest(path)}

source_identity = json.loads(
    (
        ROOT
        / "experiments/2026-10-01-rodent-pv-identity-probe/"
        "evidence/source-identity.json"
    ).read_text()
)
assert all(r["blob_matches"] and r["size_matches"] for r in source_identity["records"])

receipt = {
    "schema": "ngn-rodent-pv-artifact-identity-v1",
    "source_head": os.popen(f"git -C {ROOT} rev-parse HEAD").read().strip(),
    "source_branch": os.popen(f"git -C {ROOT} rev-parse --abbrev-ref HEAD").read().strip(),
    "binary": binary,
    "embedded_network": {
        "offset": 2_538_080,
        "bytes": len(network_bytes),
        "sha256": network_sha,
    },
    "historical_trace": trace,
    "historical_binding": {
        "match_config_engine_sha256": opponent["engine_sha256"],
        "final_manifest_line": manifest_line,
        "warning_trace_sha256": warning["compressed_trace_sha256"],
    },
    "frozen_inputs": frozen,
    "python_chess": checked(
        PYCHESS,
        "1fde6a8e932508d14b31f6a584497b49c2969fc3c7cdc675347f819f0d667a5b",
    ),
    "official_source": {
        "commit": source_identity["source_commit"],
        "tree": source_identity["tree_id"],
        "checked_blob_count": len(source_identity["records"]),
        "all_checked_blobs_match": True,
        "binding_limit": source_identity["binding_limit"],
    },
    "runtime_controls": {
        "proc_cgroup": Path("/proc/self/cgroup").read_text().strip(),
        "cpus_allowed_list": next(
            line.split(":", 1)[1].strip()
            for line in Path("/proc/self/status").read_text().splitlines()
            if line.startswith("Cpus_allowed_list:")
        ),
        "cpu_max": (CGROUP / "cpu.max").read_text().strip(),
        "memory_max": (CGROUP / "memory.max").read_text().strip(),
        "nice": os.nice(0),
        "GOMAXPROCS": os.environ.get("GOMAXPROCS"),
        "GOFLAGS": os.environ.get("GOFLAGS"),
    },
}

assert receipt["source_head"] == "cc0846a4da60a2c7172fab3301ba719970ce910e"
assert receipt["source_branch"] == "task/rodent-pv-identity-probe-20261001"
assert receipt["runtime_controls"]["cpus_allowed_list"] == "0,2"
assert receipt["runtime_controls"]["cpu_max"] == "50000 100000"
assert receipt["runtime_controls"]["memory_max"] == "4294967296"
assert receipt["runtime_controls"]["nice"] == 10
print(json.dumps(receipt, indent=2, sort_keys=True))
