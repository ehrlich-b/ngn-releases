#!/usr/bin/env python3
"""Independent terminal verification of the frozen NGN qpromo run-002."""

from __future__ import annotations

import argparse
import hashlib
import json
import math
import os
import random
import sys
from collections import Counter
from datetime import datetime, timezone
from pathlib import Path


EXPECTED_ROOT = Path("/home/ehrli/repos/ngn-qpromo-gate-20260912/run-002")
EXPECTED_DRIVER_SHA256 = "9c35fe89ef99a854deba2919e230af4decf6d7f93e55cb10c96444e8c51b2455"
EXTERNAL_PROVENANCE = {
    "resource_amendment_receipt": {
        "path": "/home/ehrli/repos/ngn-qpromo-gate-20260912/qpromo_gate_resource_amendment_v2.json",
        "sha256": "a672be9ff284171c5dcd79ee69f770fab438d26635bfde029220d3ba20e404c8",
    },
    "run001_supervisor_receipt": {
        "path": "/home/ehrli/repos/ngn-qpromo-gate-20260912/run-001/aa/supervisor.json",
        "sha256": "fdc3d73fee2eb47ef4dacab26f8c5748d51570ef6eedff4dd4128496a1098bab",
    },
}
EXPECTED_PROTOCOL = {
    "aa_games": 100,
    "candidate_games": 400,
    "tc": "10+0.1",
    "concurrency": 4,
    "physical_cpu_mask": ["0", "2", "4", "6"],
    "seed": 20260912,
    "bootstrap_seed": 2026091201,
    "bootstrap_replicates": 100000,
}


class VerificationError(RuntimeError):
    pass


def require(condition: bool, message: str) -> None:
    if not condition:
        raise VerificationError(message)


def load(path: Path) -> dict:
    with path.open("r", encoding="utf-8") as handle:
        return json.load(handle)


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for block in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def atomic_json(path: Path, value: dict) -> None:
    temporary = path.with_suffix(path.suffix + ".tmp")
    temporary.write_text(json.dumps(value, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    os.replace(temporary, path)


def elo(score: float) -> float:
    score = min(max(score, 1e-12), 1.0 - 1e-12)
    return -400.0 * math.log10(1.0 / score - 1.0)


def bootstrap(pair_half_points: list[int], seed: int, replicates: int) -> dict:
    points = [value / 4.0 for value in pair_half_points]
    rng = random.Random(seed)
    estimates = []
    for _ in range(replicates):
        score = sum(rng.choice(points) for _ in points) / len(points)
        estimates.append(elo(score))
    estimates.sort()
    score = sum(points) / len(points)
    return {
        "method": "paired nonparametric percentile bootstrap",
        "seed": seed,
        "replicates": replicates,
        "pairs": len(points),
        "elo": elo(score),
        "lower95_elo": estimates[int(0.025 * replicates)],
        "upper95_elo": estimates[int(0.975 * replicates) - 1],
    }


def close(left: object, right: object, tolerance: float = 1e-12) -> bool:
    if isinstance(left, dict) and isinstance(right, dict):
        return left.keys() == right.keys() and all(close(left[key], right[key], tolerance) for key in left)
    if isinstance(left, list) and isinstance(right, list):
        return len(left) == len(right) and all(close(a, b, tolerance) for a, b in zip(left, right))
    if isinstance(left, (int, float)) and isinstance(right, (int, float)):
        return abs(float(left) - float(right)) <= tolerance
    return left == right


def verify_inventory(root: Path, terminal: dict) -> dict:
    inventory_path = root / "inventory.sha256.json"
    decision_path = root / "decision.json"
    require(sha256(inventory_path) == terminal.get("inventory_sha256"), "terminal inventory digest mismatch")
    require(sha256(decision_path) == terminal.get("decision_sha256"), "terminal decision digest mismatch")
    inventory = load(inventory_path)
    require(inventory.get("schema") == "ngn-qpromo-fixed-gate-inventory-v1", "inventory schema mismatch")
    recorded = inventory.get("files")
    require(isinstance(recorded, dict), "inventory files map absent")
    actual_paths = {
        str(path.relative_to(root)): path
        for path in root.rglob("*")
        if path.is_file() and path.name not in {"inventory.sha256.json", "terminal.json"}
    }
    require(set(recorded) == set(actual_paths), "inventory membership differs from terminal tree")
    mismatches = []
    for relative, path in sorted(actual_paths.items()):
        actual = sha256(path)
        if recorded[relative] != actual:
            mismatches.append({"path": relative, "recorded": recorded[relative], "actual": actual})
    require(not mismatches, f"inventory file digest mismatches: {mismatches}")
    return {
        "inventory": {"path": str(inventory_path), "sha256": sha256(inventory_path), "files": len(recorded)},
        "decision": {"path": str(decision_path), "sha256": sha256(decision_path)},
    }


def frozen_copy_path(key: str, original: Path) -> Path:
    if key in {"role_exec.py", "run_match_stage.py", "process_supervisor.py", "trace_audit.py", "audit_fastchess_match.py", "uci_preflight.py", "common.py"}:
        return EXPECTED_ROOT / "source" / key
    if key in {"fastchess", "stockfish"}:
        return EXPECTED_ROOT / "tools" / key
    mapping = {
        "openings": "openings-first200.pgn",
        "prefixes": "opening-prefixes-first200.txt",
        "model": "n-30-5268.nn",
    }
    return EXPECTED_ROOT / "inputs" / mapping.get(key, original.name)


def verify_frozen_inputs(root: Path, manifest: dict) -> dict:
    driver = root / "source/qpromo_gate_driver.py"
    require(sha256(driver) == EXPECTED_DRIVER_SHA256, "frozen driver differs from archived reviewed driver")
    require(manifest.get("driver_sha256") == EXPECTED_DRIVER_SHA256, "manifest driver binding mismatch")
    bindings = {"qpromo_gate_driver.py": EXPECTED_DRIVER_SHA256}
    for key, spec in manifest["frozen_sources"].items():
        if key in EXTERNAL_PROVENANCE:
            require(spec == EXTERNAL_PROVENANCE[key], f"external provenance specification changed: {key}")
            external = Path(spec["path"])
            require(external.is_file(), f"manifest-bound external provenance is absent: {key}")
            actual = sha256(external)
            require(actual == spec["sha256"], f"external provenance hash mismatch: {key}")
            bindings[f"external:{external}"] = actual
            continue
        copy = frozen_copy_path(key, Path(spec["path"]))
        require(copy.is_file(), f"frozen input copy absent: {key}")
        actual = sha256(copy)
        require(actual == spec["sha256"], f"frozen input hash mismatch: {key}")
        bindings[str(copy.relative_to(root))] = actual
    for role, filename in (("base", "base-ngn"), ("candidate", "candidate-ngn")):
        actual = sha256(root / "inputs" / filename)
        require(actual == manifest["engines"][role]["sha256"], f"{role} binary hash mismatch")
        bindings[f"inputs/{filename}"] = actual

    receipt = load(root / "inputs/qpromo-build-test-receipt-20260912.json")
    require(receipt.get("schema") == "ngn-qpromo-build-test-receipt-v1", "build receipt schema mismatch")
    builds = {row["role"]: row for row in receipt["builds"]}
    require(builds["base"]["sha256"] == manifest["engines"]["base"]["sha256"], "base build receipt mismatch")
    require(builds["candidate"]["sha256"] == manifest["engines"]["candidate"]["sha256"], "candidate build receipt mismatch")
    require(all(row["exit_code"] == 0 for row in receipt["builds"]), "a build receipt exit is nonzero")
    require(all(row["exit_code"] == 0 for row in receipt["tests"]), "a test receipt exit is nonzero")
    source = receipt["source"]
    require(source["base_commit"] == manifest["engines"]["base"]["source_commit"], "base commit binding mismatch")
    require(source["production_diff_sha256"] == manifest["engines"]["candidate"]["production_patch_sha256"], "production patch binding mismatch")
    require(source["regression_diff_sha256"] == manifest["engines"]["candidate"]["regression_test_patch_sha256"], "regression patch binding mismatch")
    return {"count": len(bindings), "sha256": bindings, "build_receipt_reconciled": True}


def verify_interference(path: Path) -> dict:
    receipt = load(path)
    samples = receipt.get("samples", [])
    require(receipt.get("timing_evidence_rejected") is False, f"timing interference rejected: {path}")
    require(receipt.get("threshold_cores") == 1.0, f"interference threshold mismatch: {path}")
    require(receipt.get("required_consecutive_samples") == 12, f"interference consecutive rule mismatch: {path}")
    require(samples, f"interference samples absent: {path}")
    consecutive = maximum = 0
    for sample in samples:
        consecutive = consecutive + 1 if sample["non_owned_cores"] > 1.0 else 0
        maximum = max(maximum, consecutive)
    require(maximum < 12, f"independently reconstructed interference rejection: {path}")
    return {"samples": len(samples), "maximum_consecutive_over_one_core": maximum}


def verify_phase(root: Path, name: str, games: int, pairs: int, engine_a: str, engine_b: str,
                 bootstrap_seed: int, bootstrap_replicates: int) -> dict:
    phase = root / name
    chess = load(phase / "audit-match.json")
    operational = load(phase / "audit-operational.json")
    witness = load(phase / "match-witness.json")
    supervisor = load(phase / "supervisor.json")
    require(chess.get("status") == "PASS", f"{name}: chess audit did not pass")
    require(chess.get("count_mode") == "exact", f"{name}: audit was not fixed exact-count mode")
    require(chess.get("games") == games and chess.get("pairs") == pairs, f"{name}: count mismatch")
    require(len(chess.get("game_audit", [])) == games, f"{name}: game audit is incomplete")
    require(len(chess.get("pair_audit", [])) == pairs, f"{name}: pair audit is incomplete")
    require(chess.get("legal_plies", 0) > 0, f"{name}: no legally replayed plies recorded")
    require(chess.get("probable_embedded_book_signature_plies") == 0, f"{name}: embedded book signature")
    require(operational.get("state") == "COMPLETE" and operational.get("pass") is True, f"{name}: operational audit failed")
    require(witness.get("state") == "COMPLETE" and witness.get("fastchess_returncode") == 0, f"{name}: process witness failed")
    require(witness.get("violations") == [], f"{name}: process witness violations")
    require(all(witness.get("observed_instances", {}).values()), f"{name}: an engine role was not observed")
    require(supervisor.get("state") == "COMPLETE", f"{name}: supervisor incomplete")
    require(supervisor.get("command_returncode") == 0 and supervisor.get("supervisor_returncode") == 0, f"{name}: supervisor returncode failure")
    require(supervisor.get("termination_reason") is None and supervisor.get("monitor_error") is None, f"{name}: supervisor termination/monitor failure")
    require(supervisor.get("surviving_processes") == [], f"{name}: supervisor retained survivors")
    require((phase / "match.exit").read_text(encoding="utf-8").strip() == "0", f"{name}: match exit nonzero")
    require((phase / "match.stderr").stat().st_size == 0, f"{name}: match stderr nonempty")

    expected_names = {engine_a, engine_b}
    wdl = Counter()
    derived_game_scores = []
    known_reasons = {"White mates", "Black mates", "Draw by 3-fold repetition", "Draw by fifty moves rule", "Draw by insufficient mating material", "Draw by stalemate"}
    for row in chess["game_audit"]:
        require({row["white"], row["black"]} == expected_names, f"{name}: game role orientation mismatch")
        require(row["reason"] in known_reasons, f"{name}: unknown or operational terminal reason")
        white_points = {"1-0": 1.0, "0-1": 0.0, "1/2-1/2": 0.5}.get(row["result"])
        require(white_points is not None, f"{name}: invalid result token")
        if row["white"] == engine_a:
            score = white_points
        else:
            require(row["black"] == engine_a, f"{name}: engine A absent from game")
            score = 1.0 - white_points
        require(row.get("engine_a_score") == score, f"{name}: reported engine-A score disagrees with White/Black/Result")
        derived_game_scores.append(score)
        wdl[{1.0: "wins", 0.5: "draws", 0.0: "losses"}[score]] += 1
    require(sum(wdl.values()) == games, f"{name}: WDL does not reconcile")
    require(abs(chess["engine_a_points"] - (wdl["wins"] + 0.5 * wdl["draws"])) < 1e-12, f"{name}: points do not reconcile")

    penta = [0, 0, 0, 0, 0]
    derived_pair_half_points = []
    for index, row in enumerate(chess["pair_audit"], start=1):
        require(row["pair"] == index, f"{name}: pair indices not sequential")
        game_rows = chess["game_audit"][2 * (index - 1):2 * index]
        require(len(game_rows) == 2, f"{name}: pair lacks two corresponding game rows")
        require(all(game["opening_identity"] == row["opening_identity"] for game in game_rows), f"{name}: pair/game opening identity mismatch")
        require(game_rows[0]["white"] == engine_a and game_rows[1]["white"] == engine_b, f"{name}: pair color order mismatch")
        scores = derived_game_scores[2 * (index - 1):2 * index]
        reconstructed = round(sum(scores) * 2)
        require(row.get("engine_a_scores") == scores, f"{name}: reported pair scores disagree with game headers/results")
        require(row["half_points"] == reconstructed and reconstructed in range(5), f"{name}: pair score mismatch")
        derived_pair_half_points.append(reconstructed)
        penta[reconstructed] += 1
    require(penta == chess["penta_0_to_4"] and sum(penta) == pairs, f"{name}: penta does not reconcile")
    reproduced = bootstrap(derived_pair_half_points, bootstrap_seed, bootstrap_replicates)
    interference = verify_interference(phase / "non-owned-cpu.json")
    return {
        "games": games,
        "pairs": pairs,
        "engine_a": engine_a,
        "engine_b": engine_b,
        "engine_a_wdl": [wdl["wins"], wdl["draws"], wdl["losses"]],
        "penta_0_to_4": penta,
        "legal_plies": chess["legal_plies"],
        "bootstrap": reproduced,
        "interference": interference,
        "receipt_sha256": {
            filename: sha256(phase / filename)
            for filename in ("audit-match.json", "audit-operational.json", "match-witness.json", "supervisor.json", "non-owned-cpu.json", "games.pgn", "final.epd")
        },
    }


def live_run_processes(root: Path) -> list[dict]:
    found = []
    marker = str(root).encode()
    ancestors = set()
    pid = os.getpid()
    while pid > 0 and pid not in ancestors:
        ancestors.add(pid)
        try:
            raw = Path(f"/proc/{pid}/stat").read_text(encoding="utf-8")
            close_parenthesis = raw.rfind(")")
            pid = int(raw[close_parenthesis + 2:].split()[1])
        except (FileNotFoundError, PermissionError, ValueError, IndexError):
            break
    for entry in Path("/proc").iterdir():
        if not entry.name.isdigit():
            continue
        if int(entry.name) in ancestors:
            continue
        try:
            cmdline = (entry / "cmdline").read_bytes()
            executable = str((entry / "exe").resolve(strict=True))
        except (FileNotFoundError, ProcessLookupError, PermissionError):
            continue
        if marker in cmdline or executable.startswith(str(root) + "/"):
            found.append({"pid": int(entry.name), "exe": executable, "cmdline": cmdline.replace(b"\0", b" ").decode(errors="replace")})
    return found


def verify(root: Path) -> dict:
    require(root == EXPECTED_ROOT, f"wrong run root: {root}")
    terminal_path = root / "terminal.json"
    # This check intentionally precedes inventory traversal and all large-file hashing.
    require(terminal_path.is_file(), "terminal.json is absent; do not verify a live run")
    terminal = load(terminal_path)
    require(terminal.get("schema") == "ngn-qpromo-fixed-gate-terminal-v1", "terminal schema mismatch")
    require(terminal.get("state") == "COMPLETE", "terminal state is not COMPLETE")
    require(terminal.get("no_extension_after_observation") is True, "fixed-sample no-extension receipt absent")

    bindings = verify_inventory(root, terminal)
    manifest = load(root / "frozen-manifest.json")
    require(manifest.get("schema") == "ngn-qpromo-fixed-gate-v1", "manifest schema mismatch")
    require(manifest.get("release_state") == "RELEASED_AFTER_INDEPENDENT_REVIEW", "run manifest was not released")
    require(manifest.get("protocol") == EXPECTED_PROTOCOL, "frozen protocol mismatch")
    decision = load(root / "decision.json")
    require(decision.get("schema") == "ngn-qpromo-fixed-gate-decision-v1", "decision schema mismatch")
    require(decision.get("frozen_manifest_sha256") == sha256(root / "frozen-manifest.json"), "decision manifest digest mismatch")
    frozen = verify_frozen_inputs(root, manifest)

    aa = verify_phase(root, "aa", 100, 50, "BASE-A-A", "BASE-B-B", 2026091201, 100000)
    candidate = verify_phase(root, "candidate", 400, 200, "CANDIDATE-A", "BASE-B", 2026091201, 100000)
    require(aa["bootstrap"]["lower95_elo"] <= 0 <= aa["bootstrap"]["upper95_elo"], "A/A paired interval excludes zero")
    expected_decision = "ADOPT" if candidate["bootstrap"]["lower95_elo"] > 0 else "SHELVE"
    require(decision.get("rule") == "ADOPT iff fixed 400-game paired lower95 Elo > 0; never extend after observing score", "decision rule changed")
    require(decision.get("results", {}).get("candidate", {}).get("engine_a") == "candidate", "decision is not candidate-as-engine-A")
    require(close(decision["results"]["aa"]["paired_bootstrap_ci"], aa["bootstrap"]), "stored A/A bootstrap differs from independent reproduction")
    require(close(decision["results"]["candidate"]["paired_bootstrap_ci"], candidate["bootstrap"]), "stored candidate bootstrap differs from independent reproduction")
    for phase_name, phase_result in (("aa", aa), ("candidate", candidate)):
        recorded_hashes = decision["results"][phase_name].get("evidence_sha256", {})
        for filename, actual in phase_result["receipt_sha256"].items():
            require(recorded_hashes.get(filename) == actual, f"{phase_name}: decision evidence digest mismatch for {filename}")
    require(decision.get("decision") == expected_decision == terminal.get("decision"), "terminal decision does not follow the fixed rule")

    prelaunch = load(root / "prelaunch-non-owned-cpu.json")
    require(prelaunch.get("non_owned_cores", math.inf) <= 1.0, "prelaunch interference exceeds one core")
    live = live_run_processes(root)
    require(not live, f"run-owned processes remain live: {live}")
    return {
        "schema": "ngn-qpromo-run002-independent-terminal-review-v4",
        "state": "PASS",
        "run_root": str(root),
        "terminal": {"path": str(terminal_path), "sha256": sha256(terminal_path), "decision": terminal["decision"]},
        "bindings": bindings,
        "frozen_inputs": frozen,
        "prelaunch_non_owned_cores": prelaunch["non_owned_cores"],
        "live_run_processes": live,
        "aa": aa,
        "candidate": candidate,
        "independently_reproduced_decision": expected_decision,
    }


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--run-root", type=Path, default=EXPECTED_ROOT)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    if sys.platform != "linux" or "microsoft" not in Path("/proc/version").read_text(encoding="utf-8").lower():
        raise SystemExit("review: execution is restricted to WSL")
    output = args.output.resolve()
    require(not output.exists(), f"review result already exists: {output}")
    require(EXPECTED_ROOT not in output.parents, "review output must be outside the frozen run")
    output.parent.mkdir(parents=True, exist_ok=True)
    try:
        result = verify(args.run_root.resolve())
        result["verified_utc"] = datetime.now(timezone.utc).isoformat()
        result["verifier_sha256"] = sha256(Path(__file__).resolve())
        atomic_json(output, result)
        print(json.dumps({"state": "PASS", "output": str(output), "sha256": sha256(output)}, sort_keys=True))
        return 0
    except Exception as error:
        failure = {
            "schema": "ngn-qpromo-run002-independent-terminal-review-v4",
            "state": "FAILED",
            "run_root": str(args.run_root.resolve()),
            "verified_utc": datetime.now(timezone.utc).isoformat(),
            "verifier_sha256": sha256(Path(__file__).resolve()),
            "error": f"{type(error).__name__}: {error}",
        }
        atomic_json(output, failure)
        print(json.dumps(failure, sort_keys=True), file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
