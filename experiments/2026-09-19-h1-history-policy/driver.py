#!/usr/bin/env python3
"""Frozen H1 quiet-history factorial pilot and independent confirmation."""

from __future__ import annotations

import argparse
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import random
import shutil
import sys


SCHEMA = "ngn-h1-history-policy-gate-v1"
HELD = "HELD_PENDING_STATIC_REVIEW"
RELEASED = "RELEASED_AFTER_STATIC_REVIEW"
PROTOCOL = {
    "aa_pairs": 50,
    "pilot_pairs": 64,
    "confirmation_pairs": 200,
    "tc": "10+0.1",
    "concurrency": 4,
    "physical_cpu_mask": ["0", "2", "4", "6"],
    "seed": 20260919,
    "aa_bootstrap_seed": 2026091901,
    "pilot_bootstrap_seed": 2026091902,
    "confirmation_bootstrap_seed": 2026091903,
    "bootstrap_replicates": 100000,
}
RUN_ROOT = "/home/ehrli/h1-history-policy-gate-20260919/run-001"
DECISION_RULE = (
    "After a clean policy-00 A/A whose paired-bootstrap 95% interval includes zero, "
    "run fixed 64-pair 10/00, 11/01 and 11/00 pilot cells on canonical openings "
    "1-64. SHELVE before confirmation only if the direct 11/00 upper95 Elo is below "
    "zero. Otherwise run fixed 200-pair 11/00 confirmation on independent canonical "
    "source lines 201-400 and ACCEPT only if its lower95 Elo is strictly positive; "
    "otherwise SHELVE; never extend, select another cell or tune after scores"
)
CLAIM_BOUNDARY = (
    "Relative Threads=1 Rodent-V1.2 quiet-history policy evidence at 10+0.1 on the "
    "shared WSL host; not an absolute rating, 3300 proof, default change, installation "
    "or release approval"
)
SHARED_HOST_CONTRACT = (
    "the Lean hopper remains running; ten-second prelaunch and five-second "
    "continuous non-owned CPU samples are retained as telemetry, but background "
    "CPU alone does not reject the sample; scheduler and time-management "
    "interference remain a disclosed limitation"
)
EXPECTED_SOURCES = {
    "e0_driver.py", "role_exec.py", "run_match_stage.py", "process_supervisor.py",
    "trace_audit.py", "audit_fastchess_match.py", "uci_preflight.py", "common.py",
    "fastchess", "stockfish", "pilot_openings", "pilot_prefixes",
    "confirmation_openings", "confirmation_prefixes", "v12_model",
}
EXPECTED_TAGS = {
    "00": [],
    "01": ["h1consumer"],
    "10": ["h1producer"],
    "11": ["h1producer", "h1consumer"],
}


def die(message: str) -> None:
    raise SystemExit(f"h1-history-policy: {message}")


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def atomic_json(path: Path, value: object) -> None:
    temporary = path.with_suffix(path.suffix + ".tmp")
    temporary.write_text(json.dumps(value, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    os.replace(temporary, path)


def checked(spec: dict, label: str) -> Path:
    if set(spec) != {"path", "sha256"}:
        die(f"{label}: unexpected identity keys")
    path = Path(spec["path"]).resolve()
    if not path.is_file() or sha256(path) != spec["sha256"]:
        die(f"{label}: identity mismatch at {path}")
    return path


def checked_engine(spec: dict, label: str) -> Path:
    """Check an engine record after its immutable build tags were validated."""
    if set(spec) != {"path", "sha256", "build_tags"}:
        die(f"{label}: unexpected identity keys")
    return checked({"path": spec["path"], "sha256": spec["sha256"]}, label)


def load_module(path: Path):
    spec = importlib.util.spec_from_file_location("h1_frozen_e0_driver", path)
    if spec is None or spec.loader is None:
        die("cannot load frozen E0 helper")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def validate_manifest(manifest: dict, driver_hash: str) -> None:
    expected = {
        "schema", "driver_sha256", "release_state", "run_root", "protocol",
        "decision_rule", "claim_boundary", "shared_host_contract", "source",
        "engines", "frozen_sources",
    }
    if set(manifest) != expected or manifest.get("schema") != SCHEMA:
        die("manifest keys/schema differ from contract")
    if manifest["driver_sha256"] != driver_hash:
        die("driver hash differs from manifest")
    if manifest["release_state"] not in {HELD, RELEASED}:
        die("invalid release state")
    if manifest["protocol"] != PROTOCOL:
        die("protocol differs from contract")
    contracts = {
        "decision_rule": DECISION_RULE,
        "claim_boundary": CLAIM_BOUNDARY,
        "shared_host_contract": SHARED_HOST_CONTRACT,
    }
    for key, expected_value in contracts.items():
        if manifest[key] != expected_value:
            die(f"{key} differs from contract")
    if manifest["run_root"] != RUN_ROOT:
        die("run root differs from contract")
    if set(manifest["frozen_sources"]) != EXPECTED_SOURCES:
        die("frozen source set differs")
    if set(manifest["engines"]) != set(EXPECTED_TAGS):
        die("engine policy set differs")
    hashes = set()
    for policy, tags in EXPECTED_TAGS.items():
        engine = manifest["engines"][policy]
        if set(engine) != {"path", "sha256", "build_tags"} or engine["build_tags"] != tags:
            die(f"policy {policy} build contract differs")
        hashes.add(engine["sha256"])
    if len(hashes) != 4:
        die("four policies must have distinct binary hashes")
    source = manifest["source"]
    if set(source) != {"commit", "tree", "go_version", "goamd64", "cgo_enabled", "build_flags"}:
        die("source provenance keys differ")
    if source["goamd64"] != "v3" or source["cgo_enabled"] != "0" or source["build_flags"] != ["-trimpath"]:
        die("build contract differs")
    if source["go_version"] != "go1.25.5 linux/amd64":
        die("Go version differs from contract")
    for key in ("commit", "tree"):
        value = source[key]
        if len(value) != 40 or any(character not in "0123456789abcdef" for character in value):
            die(f"source {key} is not a full lowercase Git object ID")
    if Path(manifest["run_root"]).resolve().exists():
        die("immutable run root already exists")


def paired_interaction(audit_a: dict, audit_b: dict, seed: int, replicates: int, elo_fn) -> dict:
    rows_a = audit_a["pair_audit"]
    rows_b = audit_b["pair_audit"]
    identities_a = [(row["pair"], row["opening_identity"]) for row in rows_a]
    identities_b = [(row["pair"], row["opening_identity"]) for row in rows_b]
    expected_indices = list(range(1, len(rows_a) + 1))
    if [pair for pair, _ in identities_a] != expected_indices or identities_a != identities_b:
        die("interaction cells do not have identical sequential opening identities")
    left = [row["half_points"] / 4.0 for row in rows_a]
    right = [row["half_points"] / 4.0 for row in rows_b]
    if len(left) != len(right) or not left:
        die("interaction cells do not share a nonempty opening index set")
    point = elo_fn(sum(right) / len(right)) - elo_fn(sum(left) / len(left))
    rng = random.Random(seed)
    estimates = []
    for _ in range(replicates):
        indices = [rng.randrange(len(left)) for _ in left]
        effect_left = elo_fn(sum(left[index] for index in indices) / len(indices))
        effect_right = elo_fn(sum(right[index] for index in indices) / len(indices))
        estimates.append(effect_right - effect_left)
    estimates.sort()
    lower = estimates[int(0.025 * replicates)]
    upper = estimates[int(0.975 * replicates) - 1]
    material = abs(point) >= 20 and not (lower <= 0 <= upper)
    return {
        "method": "joint opening-index nonparametric percentile bootstrap",
        "definition": "Elo(11/01) - Elo(10/00)",
        "seed": seed,
        "replicates": replicates,
        "pairs": len(left),
        "elo_difference": point,
        "lower95_elo_difference": lower,
        "upper95_elo_difference": upper,
        "classification": "MATERIAL_CONSUMER_DEPENDENCE" if material else "NO_MATERIAL_INTERACTION_RESOLVED",
    }


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("manifest", type=Path)
    parser.add_argument("--check-held", action="store_true")
    args = parser.parse_args()
    if sys.platform != "linux" or "microsoft" not in Path("/proc/version").read_text(encoding="utf-8").lower():
        die("WSL execution required")
    manifest = json.loads(args.manifest.read_text(encoding="utf-8"))
    driver_hash = sha256(Path(__file__).resolve())
    validate_manifest(manifest, driver_hash)
    sources = {name: checked(spec, name) for name, spec in manifest["frozen_sources"].items()}
    engines = {
        policy: checked_engine(spec, f"engine {policy}")
        for policy, spec in manifest["engines"].items()
    }
    helper = load_module(sources["e0_driver.py"])
    if args.check_held:
        print(json.dumps({
            "state": "HELD_STATIC_CHECK_COMPLETE", "driver_sha256": driver_hash,
            "engines": {key: sha256(value) for key, value in engines.items()},
            "frozen_sources": len(sources), "run_root": "absent",
        }, sort_keys=True))
        return
    if manifest["release_state"] != RELEASED:
        die("manifest is held")

    run_root = Path(manifest["run_root"]).resolve()
    run_root.mkdir(parents=True)
    helper.baseline_cpu(10, run_root / "prelaunch-non-owned-cpu.json")
    atomic_json(run_root / "frozen-manifest.json", manifest)
    source_dir, tools_dir, inputs_dir = (run_root / "source", run_root / "tools", run_root / "inputs")
    for directory in (source_dir, tools_dir, inputs_dir):
        directory.mkdir()
    shutil.copy2(Path(__file__).resolve(), source_dir / "driver.py")
    for name in (
        "e0_driver.py", "role_exec.py", "run_match_stage.py", "process_supervisor.py",
        "trace_audit.py", "audit_fastchess_match.py", "uci_preflight.py", "common.py",
    ):
        helper.copy_frozen(sources[name], source_dir / name, manifest["frozen_sources"][name]["sha256"], executable=True)
    for name in ("fastchess", "stockfish"):
        helper.copy_frozen(sources[name], tools_dir / name, manifest["frozen_sources"][name]["sha256"], executable=True)
    copied = {}
    for name in ("pilot_openings", "pilot_prefixes", "confirmation_openings", "confirmation_prefixes", "v12_model"):
        destination = inputs_dir / Path(sources[name]).name
        helper.copy_frozen(sources[name], destination, manifest["frozen_sources"][name]["sha256"])
        copied[name] = destination
    frozen_engines = {}
    for policy, source in engines.items():
        destination = inputs_dir / f"ngn-h1-{policy}"
        helper.copy_frozen(source, destination, manifest["engines"][policy]["sha256"], executable=True)
        frozen_engines[policy] = destination

    phase_results = {}
    phase_audits = {}

    def run_phase(name: str, policy_a: str, policy_b: str, pairs: int, book: str) -> None:
        phase = run_root / name
        phase.mkdir()
        roles = []
        for side, policy in (("a", policy_a), ("b", policy_b)):
            role_dir = phase / f"role-{side}"
            role_dir.mkdir()
            engine = role_dir / "ngn"
            helper.copy_frozen(frozen_engines[policy], engine, manifest["engines"][policy]["sha256"], executable=True)
            launcher = role_dir / "launch"
            helper.copy_frozen(source_dir / "role_exec.py", launcher, manifest["frozen_sources"]["role_exec.py"]["sha256"], executable=True)
            helper.role_config(role_dir / "role-config.json", engine, manifest["engines"][policy]["sha256"], role_dir)
            roles.append({
                "id": f"{name}-{side}", "name": f"H1-{policy}-{side.upper()}", "semantic": policy,
                "backend": "rodent-v1.2-default", "model": copied["v12_model"], "cwd": role_dir,
                "engine": engine, "sha256": manifest["engines"][policy]["sha256"], "launcher": launcher,
            })
        for role in roles:
            helper.preflight(source_dir, role, phase)
        openings = copied[f"{book}_openings"]
        prefixes = copied[f"{book}_prefixes"]
        command = helper.phase_command(tools_dir, phase, roles, openings, pairs, manifest["protocol"])
        atomic_json(phase / "command.json", {"argv": command})
        helper.supervise(source_dir, command, phase, roles, manifest["protocol"]["physical_cpu_mask"], 3600 if pairs <= 64 else 7200)
        (phase / "match.exit").write_text("0\n", encoding="utf-8")
        audited = helper.audit(source_dir, tools_dir, phase, roles, openings, prefixes, pairs * 2, pairs)
        if name == "aa":
            bootstrap_seed = manifest["protocol"]["aa_bootstrap_seed"]
        elif book == "confirmation":
            bootstrap_seed = manifest["protocol"]["confirmation_bootstrap_seed"]
        else:
            bootstrap_seed = manifest["protocol"]["pilot_bootstrap_seed"]
        phase_audits[name] = audited
        phase_results[name] = {
            "engine_a": policy_a, "engine_b": policy_b,
            "games": pairs * 2, "pairs": pairs, "legal_plies": audited["legal_plies"],
            "engine_a_points": audited["engine_a_points"], "engine_a_score_rate": audited["engine_a_score_rate"],
            "penta_0_to_4": audited["penta_0_to_4"],
            "paired_normal_ci": helper.paired_ci(audited["penta_0_to_4"]),
            "paired_bootstrap_ci": helper.paired_bootstrap(audited["pair_audit"], bootstrap_seed, manifest["protocol"]["bootstrap_replicates"]),
            "evidence_sha256": {
                artifact: sha256(phase / artifact) for artifact in (
                    "audit-match.json", "audit-operational.json", "match-witness.json", "supervisor.json",
                    "games.pgn", "final.epd", "match.stdout", "match.stderr", "fastchess.log", "non-owned-cpu.json",
                )
            },
        }

    run_phase("aa", "00", "00", manifest["protocol"]["aa_pairs"], "pilot")
    aa_ci = phase_results["aa"]["paired_bootstrap_ci"]
    if not aa_ci["lower95_elo"] <= 0 <= aa_ci["upper95_elo"]:
        atomic_json(run_root / "decision.json", {"schema": SCHEMA + "-decision", "decision": "INVALID_AA", "results": phase_results})
        die("A/A interval excludes zero")

    run_phase("pilot-10-vs-00", "10", "00", manifest["protocol"]["pilot_pairs"], "pilot")
    run_phase("pilot-11-vs-01", "11", "01", manifest["protocol"]["pilot_pairs"], "pilot")
    run_phase("pilot-11-vs-00", "11", "00", manifest["protocol"]["pilot_pairs"], "pilot")
    interaction = paired_interaction(
        phase_audits["pilot-10-vs-00"], phase_audits["pilot-11-vs-01"],
        manifest["protocol"]["pilot_bootstrap_seed"], manifest["protocol"]["bootstrap_replicates"], helper.elo,
    )
    if phase_results["pilot-11-vs-00"]["paired_bootstrap_ci"]["upper95_elo"] < 0:
        decision = "SHELVE_CLEARLY_BAD_PILOT"
    else:
        run_phase("confirmation-11-vs-00", "11", "00", manifest["protocol"]["confirmation_pairs"], "confirmation")
        decision = (
            "ACCEPT" if phase_results["confirmation-11-vs-00"]["paired_bootstrap_ci"]["lower95_elo"] > 0
            else "SHELVE"
        )

    decision_path = run_root / "decision.json"
    atomic_json(decision_path, {
        "schema": SCHEMA + "-decision", "decision": decision, "rule": DECISION_RULE,
        "interaction": interaction, "results": phase_results,
        "frozen_manifest_sha256": sha256(run_root / "frozen-manifest.json"),
    })
    inventory = {}
    for path in sorted(run_root.rglob("*")):
        if path.is_file() and path.name not in {"inventory.sha256.json", "terminal.json"}:
            inventory[str(path.relative_to(run_root))] = sha256(path)
    inventory_path = run_root / "inventory.sha256.json"
    atomic_json(inventory_path, {"schema": SCHEMA + "-inventory", "files": inventory})
    atomic_json(run_root / "terminal.json", {
        "schema": SCHEMA + "-terminal", "state": "COMPLETE", "decision": decision,
        "decision_sha256": sha256(decision_path), "inventory_sha256": sha256(inventory_path),
        "no_extension_after_observation": True, "shared_host_observational_evidence": True,
    })


if __name__ == "__main__":
    main()
