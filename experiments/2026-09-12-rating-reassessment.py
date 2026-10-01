#!/usr/bin/env python3
"""Recompute NGN's accepted external-rating evidence from frozen WSL receipts."""

from __future__ import annotations

import argparse
import hashlib
import json
import math
import os
import random
from collections import Counter
from datetime import datetime, timezone
from pathlib import Path


BOOTSTRAP_RESAMPLES = 100_000
BOOTSTRAP_SEED = 0x4E474E4558543236
CANDIDATE_SHA256 = "437981db2753e2e9b46fc7185754da63969f284792ed0a6b5182dc192565f910"
MODEL_SHA256 = "3488baed71f4d432d028e05b65e2ff13f8c9afe7f91974d1a3c34a0691d6670c"
MODEL_PATH = Path(
    "/home/ehrli/repos/ngn/build/releases/"
    "ngn-a51233b-counter55-v1/n-30-5268.nn"
)

BUILD_RECEIPT = Path(
    "/home/ehrli/repos/ngn-final-integration-gate/output/"
    "final-integration-gate-20260906/attempt-001/terminal-receipt.json"
)
CANDIDATE_BINARY = BUILD_RECEIPT.parent / "build/ngn_linux_amd64_v3"

DOCS = {
    "counter_3333_author_readme_orientation": {
        "path": Path("/home/ehrli/repos/ngn/experiments/2026-09-05-next-stage-roadmap.md"),
        "sha256": "7403bd5f1abc9ff9ed0a1f8e03d8dcdca79921628ecc63047fbdf27d1d2eaff3",
        "required_text": "Counter 5.5 around 3333 in 40/15",
        "rating": 3333,
        "kind": "author-README orientation recorded by the project; no local anchor interval",
    },
    "ccrl_blitz_snapshot": {
        "path": Path("/home/ehrli/repos/ngn/experiments/2026-09-06-rodent-v1.1-release-confirmation.md"),
        "sha256": "33cdfdd01f1e2c5ebaf86da34a98411236f0995fe8f7a505c6ccfc917dfaaafe",
        "required_text": "Counter 5.5 64-bit` at 3359 +10/-10",
        "ratings": {
            "counter_5_5": {"rating": 3359, "reported_half_interval": 10},
            "rodent_v1_1": {"rating": 3522, "reported_half_interval": 16},
        },
        "kind": "September 5 CCRL Blitz snapshot recorded by the project",
    },
    "historical_classical": {
        "path": Path("/home/ehrli/repos/ngn/experiments/2026-09-05-2800-result.md"),
        "sha256": "ad3d5c54edad30b4f8eff410ec81ae990b4d78411c0787f3c2d7bee6210841bc",
        "required_text": "2884.1429896655936",
        "rating": 2884,
        "interval_95": [2834, 2934],
        "kind": "separate historical classical five-anchor calibration",
    },
}

BASE = Path("/home/ehrli/repos/ngn-external-opponents-v1/output")
CELLS = {
    "counter_5_5_t1": {
        "width": 1,
        "audit": BASE / "external-counter-t1-v16-offline-audit-20260908/audit-chess-v16.json",
        "audit_sha256": "fa44168bad1ee10c115981a1fbf8b2ef1234164f596a9a8b0b0be6382c655ecc",
        "comparison": BASE / "external-counter-t1-v16-offline-audit-20260908/comparison-v16.json",
        "comparison_sha256": "21b86fdae120fc5dca3e90bd00a1e7414f233425ba6506b011beb0c2355dead4",
        "acceptance": BASE / "root-counter-t1-v16-recovery-acceptance-20260908-v1.json",
        "acceptance_sha256": "1a8700e89bc134ae64e37e7e9bbc6d05f3ff39cae17912b7dcc54212a0c9b819",
        "manifest": BASE / "external-fixed-comparisons-20260908-attempt2/counter-5.5-v1.55.0-t1-30p0d3/source/manifest.json",
        "manifest_sha256": "2ac57c2696ab66b9bf3c2a7050f7a9fb4ac7e29202030673548f014e3472dc12",
    },
    "counter_5_5_t8": {
        "width": 8,
        "audit": BASE / "external-counter-t8-v17-offline-audit-20260908/audit-chess-v17.json",
        "audit_sha256": "3dc61bb6e8f0eb7c72a62fdc2f9d71b62216f6b3a30109eb57a49cf640d9bd46",
        "comparison": BASE / "external-counter-t8-v17-offline-audit-20260908/comparison-v17.json",
        "comparison_sha256": "9efe01ae9747721ee211671a6204fb6d7e71669074681c0762359e175d5567ab",
        "acceptance": BASE / "root-counter-t8-v17-recovery-acceptance-20260908-v1.json",
        "acceptance_sha256": "6b0d2d22e88a96ed799145fc2f82149781034a5e2a7cd78646fceec53b4eac95",
        "manifest": BASE / "external-fixed-comparisons-20260908-attempt3/counter-5.5-v1.55.0-t8-30p0d3/source/manifest.json",
        "manifest_sha256": "a2d11e9ccd4f71a85fb144440bb6bdc65ff6ebd5cbc22e2f3bbadf2d6c63a6dc",
    },
    "rodent_v1_1_t1": {
        "width": 1,
        "audit": BASE / "external-fixed-comparisons-20260908-attempt4/rodent-v1.1-anand-testers-t1-30p0d3/audit.json",
        "audit_sha256": "ecf9a328c211bf38820699c5be7e5400f0ef2e6dbd3f0e0ad75210e7f0c58436",
        "comparison": BASE / "external-fixed-comparisons-20260908-attempt4/rodent-v1.1-anand-testers-t1-30p0d3/comparison.json",
        "comparison_sha256": "67b4379295f465b26e7fdcece412f4f25dd6e9fd6253ff711875ae15f334ae98",
        "terminal": BASE / "external-fixed-comparisons-20260908-attempt4/rodent-v1.1-anand-testers-t1-30p0d3/terminal.json",
        "terminal_sha256": "fec280cd16a92e6458ca686ebeabb075044d084791c39fb9bcf856c2f84a23d0",
        "manifest": BASE / "external-fixed-comparisons-20260908-attempt4/rodent-v1.1-anand-testers-t1-30p0d3/source/manifest.json",
        "manifest_sha256": "25749c38e7e21f44373b4b4a32cf1cd00001ecfa4034c0c0c3c3f039208d8ebb",
    },
}


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def verified(path: Path, expected: str) -> dict:
    actual = sha256(path)
    if actual != expected:
        raise RuntimeError(f"hash mismatch: {path}: {actual} != {expected}")
    return {"path": str(path), "sha256": actual}


def load(path: Path) -> dict:
    with path.open("r", encoding="utf-8") as handle:
        return json.load(handle)


def elo(score: float) -> float:
    return -400.0 * math.log10(1.0 / score - 1.0)


def percentile(values: list[float], quantile: float) -> float:
    index = quantile * (len(values) - 1)
    lower = math.floor(index)
    upper = math.ceil(index)
    if lower == upper:
        return values[lower]
    fraction = index - lower
    return values[lower] * (1.0 - fraction) + values[upper] * fraction


def paired_report(half_points: list[int]) -> dict:
    if len(half_points) != 50 or any(v not in range(5) for v in half_points):
        raise RuntimeError("accepted cell must contain exactly 50 complete pair outcomes")
    scores = [value / 4.0 for value in half_points]
    mean = sum(scores) / len(scores)
    rng = random.Random(BOOTSTRAP_SEED)
    samples = sorted(
        sum(scores[rng.randrange(len(scores))] for _ in scores) / len(scores)
        for _ in range(BOOTSTRAP_RESAMPLES)
    )
    bounds = [percentile(samples, 0.025), percentile(samples, 0.975)]
    return {
        "pairs": 50,
        "penta_0_to_4": [Counter(half_points)[i] for i in range(5)],
        "score": mean,
        "elo": elo(mean),
        "bootstrap": {
            "method": "deterministic-paired-nonparametric-percentile",
            "pairing_unit": "one opening played with colors reversed",
            "resamples": BOOTSTRAP_RESAMPLES,
            "seed_decimal": BOOTSTRAP_SEED,
            "seed_hex": hex(BOOTSTRAP_SEED),
            "score_interval_95": bounds,
            "elo_interval_95": [elo(bounds[0]), elo(bounds[1])],
        },
    }


def close(a: object, b: object, tolerance: float = 1e-12) -> bool:
    if isinstance(a, dict) and isinstance(b, dict):
        return a.keys() == b.keys() and all(close(a[k], b[k], tolerance) for k in a)
    if isinstance(a, list) and isinstance(b, list):
        return len(a) == len(b) and all(close(x, y, tolerance) for x, y in zip(a, b))
    if isinstance(a, (int, float)) and isinstance(b, (int, float)):
        return abs(float(a) - float(b)) <= tolerance
    return a == b


def anchored(relative: dict, rating: int, anchor_half_interval: int | None = None) -> dict:
    match_interval = [rating + value for value in relative["bootstrap"]["elo_interval_95"]]
    result = {
        "anchor_rating": rating,
        "point": rating + relative["elo"],
        "match_sampling_interval_95": match_interval,
    }
    if anchor_half_interval is not None:
        result["anchor_reported_half_interval"] = anchor_half_interval
        result["outer_sum_with_anchor_reported_interval"] = [
            match_interval[0] - anchor_half_interval,
            match_interval[1] + anchor_half_interval,
        ]
    return result


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--output", required=True, type=Path)
    args = parser.parse_args()

    kernel_release = Path("/proc/sys/kernel/osrelease")
    kernel_text = kernel_release.read_text(encoding="utf-8") if kernel_release.exists() else ""
    if not (os.environ.get("WSL_DISTRO_NAME") and "microsoft" in kernel_text.lower()):
        raise RuntimeError("this evidence audit is restricted to the authorized WSL environment")

    build_binding = verified(
        BUILD_RECEIPT,
        "831349ef0161b1bcea010122aadfcaad36bdfde17fc3e38c5a652294e28d9ea2",
    )
    build = load(BUILD_RECEIPT)
    if not (
        build["state"] == "COMPLETE"
        and build["source_commit"].startswith("a51233b")
    ):
        raise RuntimeError("unexpected candidate build receipt")
    if build["ngn_linux_amd64_v3"]["sha256"] != CANDIDATE_SHA256:
        raise RuntimeError("build receipt does not bind candidate binary")
    binary_binding = verified(CANDIDATE_BINARY, CANDIDATE_SHA256)
    model_binding = verified(MODEL_PATH, MODEL_SHA256)

    provenance = {}
    for name, spec in DOCS.items():
        binding = verified(spec["path"], spec["sha256"])
        if spec["required_text"] not in spec["path"].read_text(encoding="utf-8"):
            raise RuntimeError(f"anchor text missing: {name}")
        provenance[name] = {**binding, **{k: v for k, v in spec.items() if k not in {"path", "sha256", "required_text"}}}

    results = {}
    for name, spec in CELLS.items():
        bindings = {
            key: verified(spec[key], spec[f"{key}_sha256"])
            for key in ("audit", "comparison", "manifest")
        }
        if "acceptance" in spec:
            bindings["acceptance"] = verified(spec["acceptance"], spec["acceptance_sha256"])
        if "terminal" in spec:
            bindings["terminal"] = verified(spec["terminal"], spec["terminal_sha256"])

        audit = load(spec["audit"])
        comparison = load(spec["comparison"])
        manifest = load(spec["manifest"])
        if audit["status"] != "PASS" or audit["games"] != 100 or audit["pairs"] != 50:
            raise RuntimeError(f"cell is not a full passing 100-game/50-pair audit: {name}")
        if len(audit["pair_audit"]) != 50:
            raise RuntimeError(f"per-opening pair evidence incomplete: {name}")
        if manifest["cell"]["width"] != spec["width"]:
            raise RuntimeError(f"width mismatch: {name}")
        if manifest["candidate"]["binary"]["sha256"] != CANDIDATE_SHA256:
            raise RuntimeError(f"candidate identity mismatch: {name}")
        if manifest["candidate"]["model"]["sha256"] != MODEL_SHA256:
            raise RuntimeError(f"model identity mismatch: {name}")
        recomputed = paired_report([row["half_points"] for row in audit["pair_audit"]])
        stored_core = {
            "pairs": comparison["pairs"],
            "penta_0_to_4": comparison["penta_0_to_4"],
            "score": comparison["score"],
            "elo": comparison["elo"],
            "bootstrap": {
                "method": comparison["bootstrap"]["method"],
                "resamples": comparison["bootstrap"]["resamples"],
                "seed_decimal": comparison["bootstrap"]["seed"],
                "seed_hex": hex(comparison["bootstrap"]["seed"]),
                "score_interval_95": comparison["bootstrap"]["score_interval_95"],
                "elo_interval_95": comparison["bootstrap"]["elo_interval_95"],
            },
        }
        recomputed_core = json.loads(json.dumps(recomputed))
        del recomputed_core["bootstrap"]["pairing_unit"]
        if not close(recomputed_core, stored_core):
            raise RuntimeError(f"paired bootstrap does not reproduce stored comparison: {name}")

        if "acceptance" in spec:
            acceptance = load(spec["acceptance"])
            facts = acceptance["accepted_facts"]
            if not (
                acceptance["state"] == "ACCEPTED_RECOVERED_TERMINAL"
                and facts["games"] == 100
                and facts["pairs"] == 50
                and facts["penta_0_to_4"] == recomputed["penta_0_to_4"]
            ):
                raise RuntimeError(f"recovery acceptance does not bind full result: {name}")
        else:
            terminal = load(spec["terminal"])
            if not (
                terminal["state"] == "COMPLETE"
                and terminal["games"] == 100
                and terminal["pairs"] == 50
                and terminal["comparison"]["penta_0_to_4"] == recomputed["penta_0_to_4"]
            ):
                raise RuntimeError(f"terminal receipt does not bind full result: {name}")

        results[name] = {
            "width": spec["width"],
            "bindings": bindings,
            "candidate_binary_sha256": CANDIDATE_SHA256,
            "model_sha256": MODEL_SHA256,
            "full_game_receipt_verified": True,
            "per_opening_pair_rows_verified": 50,
            "recomputed": recomputed,
        }

    c1 = results["counter_5_5_t1"]["recomputed"]
    c8 = results["counter_5_5_t8"]["recomputed"]
    r1 = results["rodent_v1_1_t1"]["recomputed"]
    result = {
        "schema": "ngn-rating-reassessment-v2",
        "generated_utc": datetime.now(timezone.utc).isoformat(),
        "state": "PASS",
        "scope": "accepted full external cells only; no engines, matches, partial cells, or internal SPRTs",
        "candidate": {
            "binary": binary_binding,
            "build_receipt": build_binding,
            "source_commit": build["source_commit"],
            "source_tree": build["source_tree"],
            "model": model_binding,
        },
        "anchor_provenance": provenance,
        "cells": results,
        "anchored_sensitivities": {
            "counter_t1_on_3333_orientation": anchored(c1, 3333),
            "counter_t1_on_3359_ccrl_blitz": anchored(c1, 3359, 10),
            "rodent_t1_on_3522_ccrl_blitz": anchored(r1, 3522, 16),
            "counter_t8_on_3333_orientation": anchored(c8, 3333),
            "counter_t8_on_3359_ccrl_blitz": anchored(c8, 3359, 10),
        },
        "conclusions": {
            "one_thread": "roughly 3000-3100 as a cross-list orientation, not an official rating",
            "one_thread_match_interval_union_counter3333_and_rodent3522": [
                anchored(c1, 3333)["match_sampling_interval_95"][0],
                anchored(r1, 3522)["match_sampling_interval_95"][1],
            ],
            "eight_thread": "25.5% vs matched Counter8: -186 Elo, paired 95% [-230,-143]; absolute 3147 map is orientation only",
            "historical_classical": "2884 [2834,2934] is a separate 53e4/native-Windows/five-anchor calibration and is not pooled",
            "mixed_width_3130": "not defensible: it averages Rodent t1 and Counter t8 mappings",
            "additional_rating_games_now": 0,
            "reason": "more repeats narrow sampling error but do not resolve cross-list, settings, personality, or thread-count anchor systematics",
        },
        "next_fixed400_rule_review": {
            "rule": "accept candidate only if the completed 400-game (200-pair) paired 95% interval lower bound is > 0",
            "assessment": "valid predeclared fixed-sample evidence of positive mean paired Elo",
            "required_guardrails": [
                "finish all 400 games and all 200 color-reversed opening pairs",
                "no early stopping, extension, or score-conditioned rerun",
                "zero operational failures and independent legal/terminal audit",
                "report point estimate and full paired interval; do not claim a positive minimum margin beyond zero",
            ],
        },
    }
    args.output.parent.mkdir(parents=True, exist_ok=False)
    args.output.write_text(json.dumps(result, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    print(json.dumps({"state": "PASS", "output": str(args.output), "sha256": sha256(args.output)}, sort_keys=True))


if __name__ == "__main__":
    main()
