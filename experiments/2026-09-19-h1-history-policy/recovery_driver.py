#!/usr/bin/env python3
"""Recover the frozen H1 gate after a telemetry-only procfs race."""

from __future__ import annotations

import argparse
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import shutil
import sys
import time


SCHEMA = "ngn-h1-history-policy-recovery-v1"
HELD = "HELD_PENDING_STATIC_REVIEW"
RELEASED = "RELEASED_AFTER_STATIC_REVIEW"
RUN_ROOT = Path("/home/ehrli/h1-history-policy-gate-20260919/run-001")
RECOVERY_ROOT = RUN_ROOT / "recovery-v1"
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
DECISION_RULE = (
    "After a clean policy-00 A/A whose paired-bootstrap 95% interval includes zero, "
    "run fixed 64-pair 10/00, 11/01 and 11/00 pilot cells on canonical openings "
    "1-64. SHELVE before confirmation only if the direct 11/00 upper95 Elo is below "
    "zero. Otherwise run fixed 200-pair 11/00 confirmation on independent canonical "
    "source lines 201-400 and ACCEPT only if its lower95 Elo is strictly positive; "
    "otherwise SHELVE; never extend, select another cell or tune after scores"
)
RECOVERY_RULE = (
    "Preserve the already-completed policy-11 versus policy-00 pilot exactly when its "
    "frozen supervisor and process-witness receipts prove a zero-exit, violation-free "
    "match with no survivors. Audit those existing games without replay. The missing "
    "non-owned-CPU sample is disclosed as a telemetry-only deviation and cannot select "
    "or reject the result because background CPU was predeclared non-gating. Apply the "
    "unchanged decision rule, and if confirmation is required run it once with the same "
    "frozen inputs after making procfs disappearance non-fatal to telemetry sampling"
)
EXPECTED_FAILURE_ARTIFACTS = {
    "launcher_stderr", "direct_supervisor", "direct_witness", "direct_games",
    "direct_final_epd", "direct_fastchess_log",
}
EVIDENCE_FILES = (
    "audit-match.json", "audit-operational.json", "match-witness.json",
    "supervisor.json", "games.pgn", "final.epd", "match.stdout",
    "match.stderr", "fastchess.log", "non-owned-cpu.json",
)


def die(message: str) -> None:
    raise SystemExit(f"h1-history-recovery: {message}")


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


def atomic_text(path: Path, value: str) -> None:
    temporary = path.with_suffix(path.suffix + ".tmp")
    temporary.write_text(value, encoding="utf-8")
    os.replace(temporary, path)


def read_json(path: Path) -> dict:
    return json.loads(path.read_text(encoding="utf-8"))


def checked(spec: dict, label: str) -> Path:
    if set(spec) != {"path", "sha256"}:
        die(f"{label}: unexpected identity keys")
    path = Path(spec["path"]).resolve()
    if not path.is_file() or sha256(path) != spec["sha256"]:
        die(f"{label}: identity mismatch at {path}")
    return path


def load_module(name: str, path: Path):
    spec = importlib.util.spec_from_file_location(name, path)
    if spec is None or spec.loader is None:
        die(f"cannot load {path}")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def read_proc_entry(entry: Path) -> tuple[int, int, int, str] | None:
    try:
        fields = (entry / "stat").read_text(encoding="utf-8").split()
        status = (entry / "status").read_text(encoding="utf-8").splitlines()
        return (
            int(fields[0]),
            int(fields[13]) + int(fields[14]),
            int(fields[3]),
            next((line.split(":", 1)[1].strip() for line in status if line.startswith("Name:")), "?"),
        )
    except (FileNotFoundError, ProcessLookupError, PermissionError, ValueError, IndexError):
        return None


def safe_proc_snapshot() -> tuple[float, dict[int, int], dict[int, int], dict[int, str]]:
    ticks: dict[int, int] = {}
    parents: dict[int, int] = {}
    names: dict[int, str] = {}
    for entry in Path("/proc").iterdir():
        if not entry.name.isdigit():
            continue
        record = read_proc_entry(entry)
        if record is None:
            continue
        pid, tick_count, parent, name = record
        ticks[pid] = tick_count
        parents[pid] = parent
        names[pid] = name
    return time.monotonic(), ticks, parents, names


def decide(direct_ci: dict, confirmation_ci: dict | None = None) -> str:
    if direct_ci["upper95_elo"] < 0:
        return "SHELVE_CLEARLY_BAD_PILOT"
    if confirmation_ci is None:
        return "NEED_CONFIRMATION"
    return "ACCEPT" if confirmation_ci["lower95_elo"] > 0 else "SHELVE"


def validate_recovery_manifest(manifest: dict, driver_hash: str) -> None:
    expected = {
        "schema", "driver_sha256", "release_state", "run_root", "recovery_root",
        "protocol", "decision_rule", "recovery_rule", "original_manifest_sha256",
        "original_driver_sha256", "original_helper_sha256", "failure_artifacts",
    }
    if set(manifest) != expected or manifest.get("schema") != SCHEMA:
        die("recovery manifest keys/schema differ")
    if manifest["driver_sha256"] != driver_hash:
        die("recovery driver hash differs")
    if manifest["release_state"] not in {HELD, RELEASED}:
        die("invalid recovery release state")
    if manifest["run_root"] != str(RUN_ROOT) or manifest["recovery_root"] != str(RECOVERY_ROOT):
        die("recovery paths differ")
    if manifest["protocol"] != PROTOCOL or manifest["decision_rule"] != DECISION_RULE:
        die("protocol or decision rule differs")
    if manifest["recovery_rule"] != RECOVERY_RULE:
        die("recovery rule differs")
    if set(manifest["failure_artifacts"]) != EXPECTED_FAILURE_ARTIFACTS:
        die("failure artifact set differs")
    for key in ("original_manifest_sha256", "original_driver_sha256", "original_helper_sha256"):
        value = manifest[key]
        if len(value) != 64 or any(character not in "0123456789abcdef" for character in value):
            die(f"invalid {key}")


def validate_supervision(phase: Path) -> tuple[dict, dict]:
    supervisor = read_json(phase / "supervisor.json")
    witness = read_json(phase / "match-witness.json")
    if (
        supervisor.get("state") != "COMPLETE"
        or supervisor.get("command_returncode") != 0
        or supervisor.get("supervisor_returncode") != 0
        or supervisor.get("termination_reason") is not None
        or supervisor.get("monitor_error") is not None
        or supervisor.get("surviving_processes") != []
    ):
        die(f"{phase.name}: supervisor receipt is not clean")
    if (
        witness.get("state") != "COMPLETE"
        or witness.get("fastchess_returncode") != 0
        or witness.get("violations") != []
    ):
        die(f"{phase.name}: process witness is not clean")
    return supervisor, witness


def validate_audit(phase: Path, pairs: int) -> dict:
    audit = read_json(phase / "audit-match.json")
    operational = read_json(phase / "audit-operational.json")
    if audit.get("status") != "PASS" or operational.get("state") != "COMPLETE" or operational.get("pass") is not True:
        die(f"{phase.name}: retained audit is not clean")
    rows = audit.get("pair_audit", [])
    if len(rows) != pairs or [row.get("pair") for row in rows] != list(range(1, pairs + 1)):
        die(f"{phase.name}: pair audit differs")
    if sum(audit.get("penta_0_to_4", [])) != pairs:
        die(f"{phase.name}: pentanomial count differs")
    return audit


def roles_for_phase(phase: Path, phase_name: str, policy_a: str, policy_b: str, original_manifest: dict) -> list[dict]:
    model = RUN_ROOT / "inputs" / "rodent_4kb_768hl_8ob_v2.bin"
    roles = []
    for side, policy in (("a", policy_a), ("b", policy_b)):
        role_dir = phase / f"role-{side}"
        roles.append({
            "id": f"{phase_name}-{side}", "name": f"H1-{policy}-{side.upper()}",
            "semantic": policy, "backend": "rodent-v1.2-default", "model": model,
            "cwd": role_dir, "engine": role_dir / "ngn",
            "sha256": original_manifest["engines"][policy]["sha256"],
            "launcher": role_dir / "launch",
        })
    return roles


def phase_result(helper, phase: Path, policy_a: str, policy_b: str, pairs: int, seed: int) -> tuple[dict, dict]:
    validate_supervision(phase)
    audit = validate_audit(phase, pairs)
    result = {
        "engine_a": policy_a,
        "engine_b": policy_b,
        "games": pairs * 2,
        "pairs": pairs,
        "legal_plies": audit["legal_plies"],
        "engine_a_points": audit["engine_a_points"],
        "engine_a_score_rate": audit["engine_a_score_rate"],
        "penta_0_to_4": audit["penta_0_to_4"],
        "paired_normal_ci": helper.paired_ci(audit["penta_0_to_4"]),
        "paired_bootstrap_ci": helper.paired_bootstrap(audit["pair_audit"], seed, PROTOCOL["bootstrap_replicates"]),
        "evidence_sha256": {name: sha256(phase / name) for name in EVIDENCE_FILES},
    }
    return result, audit


def run_confirmation(helper, original_manifest: dict) -> None:
    phase_name = "confirmation-11-vs-00"
    phase = RUN_ROOT / phase_name
    if phase.exists():
        die("confirmation phase already exists")
    phase.mkdir()
    roles = []
    for side, policy in (("a", "11"), ("b", "00")):
        role_dir = phase / f"role-{side}"
        role_dir.mkdir()
        engine = role_dir / "ngn"
        engine_hash = original_manifest["engines"][policy]["sha256"]
        helper.copy_frozen(RUN_ROOT / "inputs" / f"ngn-h1-{policy}", engine, engine_hash, executable=True)
        launcher = role_dir / "launch"
        role_exec_hash = original_manifest["frozen_sources"]["role_exec.py"]["sha256"]
        helper.copy_frozen(RUN_ROOT / "source" / "role_exec.py", launcher, role_exec_hash, executable=True)
        helper.role_config(role_dir / "role-config.json", engine, engine_hash, role_dir)
        roles.append({
            "id": f"{phase_name}-{side}", "name": f"H1-{policy}-{side.upper()}",
            "semantic": policy, "backend": "rodent-v1.2-default",
            "model": RUN_ROOT / "inputs" / "rodent_4kb_768hl_8ob_v2.bin",
            "cwd": role_dir, "engine": engine, "sha256": engine_hash, "launcher": launcher,
        })
    for role in roles:
        helper.preflight(RUN_ROOT / "source", role, phase)
    openings = RUN_ROOT / "inputs" / "confirmation-openings-201-400.pgn"
    command = helper.phase_command(RUN_ROOT / "tools", phase, roles, openings, PROTOCOL["confirmation_pairs"], PROTOCOL)
    atomic_json(phase / "command.json", {"argv": command})
    helper.supervise(
        RUN_ROOT / "source", command, phase, roles,
        PROTOCOL["physical_cpu_mask"], 7200,
    )
    atomic_text(phase / "match.exit", "0\n")
    helper.audit(
        RUN_ROOT / "source", RUN_ROOT / "tools", phase, roles, openings,
        RUN_ROOT / "inputs" / "confirmation-prefixes-201-400.txt",
        PROTOCOL["confirmation_pairs"] * 2, PROTOCOL["confirmation_pairs"],
    )


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("manifest", type=Path)
    parser.add_argument("--check-held", action="store_true")
    args = parser.parse_args()
    if sys.platform != "linux" or "microsoft" not in Path("/proc/version").read_text(encoding="utf-8").lower():
        die("WSL execution required")
    manifest = read_json(args.manifest.resolve(strict=True))
    driver_path = Path(__file__).resolve()
    driver_hash = sha256(driver_path)
    validate_recovery_manifest(manifest, driver_hash)
    if not RUN_ROOT.is_dir() or RECOVERY_ROOT.exists():
        die("run root absent or recovery root already exists")
    frozen_manifest_path = RUN_ROOT / "frozen-manifest.json"
    original_driver_path = RUN_ROOT / "source" / "driver.py"
    helper_path = RUN_ROOT / "source" / "e0_driver.py"
    identities = {
        frozen_manifest_path: manifest["original_manifest_sha256"],
        original_driver_path: manifest["original_driver_sha256"],
        helper_path: manifest["original_helper_sha256"],
    }
    for path, expected in identities.items():
        if not path.is_file() or sha256(path) != expected:
            die(f"original frozen identity mismatch: {path}")
    for label, spec in manifest["failure_artifacts"].items():
        checked(spec, label)
    for forbidden in (RUN_ROOT / "decision.json", RUN_ROOT / "terminal.json", RUN_ROOT / "confirmation-11-vs-00"):
        if forbidden.exists():
            die(f"unexpected post-failure artifact: {forbidden}")
    direct = RUN_ROOT / "pilot-11-vs-00"
    for forbidden in (direct / "match.exit", direct / "audit-match.json", direct / "audit-operational.json", direct / "non-owned-cpu.json"):
        if forbidden.exists():
            die(f"direct phase was already recovered: {forbidden}")
    validate_supervision(direct)
    original_manifest = read_json(frozen_manifest_path)
    if original_manifest["protocol"] != PROTOCOL or original_manifest["decision_rule"] != DECISION_RULE:
        die("original frozen protocol differs")
    helper = load_module("h1_recovery_e0_helper", helper_path)
    original = load_module("h1_recovery_original_driver", original_driver_path)
    helper.proc_snapshot = safe_proc_snapshot
    prior_specs = (
        ("aa", "00", "00", PROTOCOL["aa_pairs"], PROTOCOL["aa_bootstrap_seed"]),
        ("pilot-10-vs-00", "10", "00", PROTOCOL["pilot_pairs"], PROTOCOL["pilot_bootstrap_seed"]),
        ("pilot-11-vs-01", "11", "01", PROTOCOL["pilot_pairs"], PROTOCOL["pilot_bootstrap_seed"]),
    )
    for name, _, _, pairs, _ in prior_specs:
        validate_supervision(RUN_ROOT / name)
        validate_audit(RUN_ROOT / name, pairs)
    if args.check_held:
        print(json.dumps({
            "state": "HELD_RECOVERY_CHECK_COMPLETE",
            "driver_sha256": driver_hash,
            "original_manifest_sha256": manifest["original_manifest_sha256"],
            "preserved_direct_artifacts": len(manifest["failure_artifacts"]),
            "completed_prior_phases": 3,
            "direct_supervision": "CLEAN",
            "direct_score_observed": False,
            "recovery_root": "absent",
        }, sort_keys=True))
        return
    if manifest["release_state"] != RELEASED:
        die("recovery manifest is held")

    RECOVERY_ROOT.mkdir()
    shutil.copy2(driver_path, RECOVERY_ROOT / "recovery_driver.py")
    shutil.copy2(args.manifest.resolve(), RECOVERY_ROOT / "recovery-manifest.json")
    atomic_json(RECOVERY_ROOT / "incident.json", {
        "schema": SCHEMA + "-incident",
        "failure": "ProcessLookupError while collecting telemetry-only non-owned CPU data",
        "failed_controller_phase": "pilot-11-vs-00",
        "match_supervision": "completed cleanly after controller exit",
        "salvage_disposition": "retain and audit existing games without replay",
        "missing_evidence": "continuous non-owned CPU sample for pilot-11-vs-00",
        "selection_effect": "none; background CPU was predeclared non-gating and the direct score was not inspected before recovery release",
        "launcher_stderr_sha256": manifest["failure_artifacts"]["launcher_stderr"]["sha256"],
    })
    atomic_text(direct / "match.exit", "0\n")
    atomic_json(direct / "non-owned-cpu.json", {
        "schema": SCHEMA + "-telemetry-gap",
        "disposition": "DISCLOSED_TELEMETRY_ONLY_DEVIATION",
        "background_cpu_rejects_sample": False,
        "samples": [],
        "reason": "original controller exited on a transient procfs ProcessLookupError; the independently supervised match continued to a clean zero exit",
        "supervisor_sha256": sha256(direct / "supervisor.json"),
        "match_witness_sha256": sha256(direct / "match-witness.json"),
    })
    direct_roles = roles_for_phase(direct, "pilot-11-vs-00", "11", "00", original_manifest)
    helper.audit(
        RUN_ROOT / "source", RUN_ROOT / "tools", direct, direct_roles,
        RUN_ROOT / "inputs" / "pilot-openings-1-64.pgn",
        RUN_ROOT / "inputs" / "pilot-prefixes-1-64.txt",
        PROTOCOL["pilot_pairs"] * 2, PROTOCOL["pilot_pairs"],
    )

    phase_results: dict[str, dict] = {}
    phase_audits: dict[str, dict] = {}
    for name, policy_a, policy_b, pairs, seed in prior_specs + (
        ("pilot-11-vs-00", "11", "00", PROTOCOL["pilot_pairs"], PROTOCOL["pilot_bootstrap_seed"]),
    ):
        phase_results[name], phase_audits[name] = phase_result(
            helper, RUN_ROOT / name, policy_a, policy_b, pairs, seed,
        )
    aa_ci = phase_results["aa"]["paired_bootstrap_ci"]
    if not aa_ci["lower95_elo"] <= 0 <= aa_ci["upper95_elo"]:
        die("retained A/A interval excludes zero")
    interaction = original.paired_interaction(
        phase_audits["pilot-10-vs-00"], phase_audits["pilot-11-vs-01"],
        PROTOCOL["pilot_bootstrap_seed"], PROTOCOL["bootstrap_replicates"], helper.elo,
    )
    decision = decide(phase_results["pilot-11-vs-00"]["paired_bootstrap_ci"])
    if decision == "NEED_CONFIRMATION":
        run_confirmation(helper, original_manifest)
        phase_results["confirmation-11-vs-00"], phase_audits["confirmation-11-vs-00"] = phase_result(
            helper, RUN_ROOT / "confirmation-11-vs-00", "11", "00",
            PROTOCOL["confirmation_pairs"], PROTOCOL["confirmation_bootstrap_seed"],
        )
        decision = decide(
            phase_results["pilot-11-vs-00"]["paired_bootstrap_ci"],
            phase_results["confirmation-11-vs-00"]["paired_bootstrap_ci"],
        )
    decision_path = RUN_ROOT / "decision.json"
    atomic_json(decision_path, {
        "schema": "ngn-h1-history-policy-gate-v1-decision",
        "decision": decision,
        "rule": DECISION_RULE,
        "recovery_rule": RECOVERY_RULE,
        "interaction": interaction,
        "results": phase_results,
        "frozen_manifest_sha256": sha256(frozen_manifest_path),
        "recovery_manifest_sha256": sha256(RECOVERY_ROOT / "recovery-manifest.json"),
        "telemetry_limitation": "pilot-11-vs-00 non-owned CPU sample unavailable after controller telemetry race; supervisor and process witness clean",
    })
    inventory = {}
    for path in sorted(RUN_ROOT.rglob("*")):
        if path.is_file() and path.name not in {"inventory.sha256.json", "terminal.json"}:
            inventory[str(path.relative_to(RUN_ROOT))] = sha256(path)
    inventory_path = RUN_ROOT / "inventory.sha256.json"
    atomic_json(inventory_path, {"schema": "ngn-h1-history-policy-gate-v1-inventory", "files": inventory})
    atomic_json(RUN_ROOT / "terminal.json", {
        "schema": "ngn-h1-history-policy-gate-v1-terminal",
        "state": "COMPLETE",
        "decision": decision,
        "decision_sha256": sha256(decision_path),
        "inventory_sha256": sha256(inventory_path),
        "no_extension_after_observation": True,
        "shared_host_observational_evidence": "PARTIAL_DIRECT_PHASE_TELEMETRY_GAP_DISCLOSED",
        "recovered_without_match_replay": True,
    })


if __name__ == "__main__":
    main()
