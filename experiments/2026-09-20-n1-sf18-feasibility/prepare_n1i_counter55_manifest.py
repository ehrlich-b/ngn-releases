#!/usr/bin/env python3
"""Derive and approve the exact N1i SF18 BIG versus Counter 5.5 contract."""

from __future__ import annotations

import argparse
import hashlib
import json
import sys
from pathlib import Path


SOURCE_FILES = {
    "runner": "run_external_fixed_match.py",
    "runner_helpers": "run_candidate_match.py",
    "manifest_module": "external_match_manifest.py",
    "legacy_manifest": "manifest.py",
    "common": "common.py",
    "supervisor": "process_supervisor.py",
    "role_exec": "role_exec.py",
    "uci_preflight": "uci_preflight.py",
    "external_profiles": "external_profiles.py",
    "external_admission": "external_uci_admission.py",
    "external_report": "external_report.py",
    "match_stage": "run_match_stage.py",
    "trace_auditor": "external_trace_audit.py",
    "trace_helpers": "trace_audit.py",
    "chess_auditor": "audit_fastchess_match.py",
}

SOURCE_COMMIT = "629df0ed421cbb11020f6238fc59be46826805ba"
CANDIDATE_SHA256 = "f386ab74b9592763e276e155f010a3e9d887175fb72f285c957f272d9d7f4524"
MODEL_SHA256 = "c288c895ea924429ea9092e3f36b2b3c1f00f2a3a4c759ff7e57e79e3b43e4a7"


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for block in iter(lambda: handle.read(1 << 20), b""):
            digest.update(block)
    return digest.hexdigest()


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument(
        "--wsl-repo",
        default="/home/ehrli/repos/ngn-n1-sf18-n1i-match",
        help="absolute WSL checkout used by the frozen match",
    )
    args = parser.parse_args()

    experiment = Path(__file__).resolve().parent
    runner = experiment / "n1i-counter55-runner"
    sys.path.insert(0, str(runner))
    from external_match_manifest import (  # pylint: disable=import-outside-toplevel
        HELD_APPROVAL,
        review_subject_sha256,
        validate_manifest,
    )

    manifest = json.loads(args.base.read_text(encoding="utf-8"))
    wsl_repo = Path(args.wsl_repo)
    wsl_experiment = wsl_repo / "experiments/2026-09-20-n1-sf18-feasibility"
    wsl_runner = wsl_experiment / "n1i-counter55-runner"

    for key, name in SOURCE_FILES.items():
        local = runner / name
        manifest["inputs"][key] = {
            "path": str(wsl_runner / name),
            "sha256": sha256(local),
        }

    build_receipt = experiment / "n1i-build-receipt.json"
    manifest["candidate"] = {
        "id": "ngn_sf18_big",
        "display_name": "NGN-SF18-BIG-629df0e",
        "binary": {
            "path": str(wsl_repo / "output/n1i-match/ngn-sf18-big-v3"),
            "sha256": CANDIDATE_SHA256,
        },
        "build_receipt": {
            "path": str(wsl_experiment / "n1i-build-receipt.json"),
            "sha256": sha256(build_receipt),
        },
        "model": {
            "path": "/home/ehrli/repos/ngn-n1-sf18-feasibility-20260920/output/n1b/nn-c288c895ea92.nnue",
            "sha256": MODEL_SHA256,
        },
        "options": [
            {"name": "Threads", "value": 1},
            {"name": "OwnBook", "value": False},
            {"name": "EvalFile", "value": "$FROZEN_NETWORK"},
            {"name": "EvalBackend", "value": "sf18-big"},
            {"name": "Hash", "value": 128},
            {"name": "Move Overhead", "value": 100},
        ],
    }
    manifest["cell"]["id"] = "sf18-big-vs-counter-5.5-t1-30p0d3"
    manifest["purpose"] = (
        "direct one-thread 30+0.3 SF18 BIG comparison against the exact pinned "
        "Counter 5.5 anchor; no transitive Elo addition"
    )
    manifest["status"] = "HELD"
    manifest["approval"] = dict(HELD_APPROVAL)
    validate_manifest(manifest, verify_files=False)

    reviewed = review_subject_sha256(manifest)
    manifest["status"] = "APPROVED_TO_RUN"
    manifest["approval"] = {
        "authority": "root",
        "exclusive_match_window": True,
        "reviewed_manifest_sha256": reviewed,
        "token": f"root-sf18-big-counter55-{reviewed[:20]}",
    }
    validate_manifest(manifest, verify_files=False)

    args.output.write_text(
        json.dumps(manifest, indent=2, sort_keys=True) + "\n",
        encoding="utf-8",
    )
    print(json.dumps({
        "manifest": str(args.output),
        "manifest_sha256": sha256(args.output),
        "review_subject_sha256": reviewed,
        "approval_token": manifest["approval"]["token"],
        "candidate_source_commit": SOURCE_COMMIT,
    }, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
