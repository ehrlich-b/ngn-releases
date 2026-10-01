#!/usr/bin/env python3
"""Fail-closed same-code NGN evaluator candidate match runner."""
from __future__ import annotations

import argparse
import json
import os
import re
import shutil
import signal
import stat
import subprocess
import sys
from pathlib import Path
from typing import Any

from common import (
    ActiveSupervisor,
    CandidateMatchError,
    atomic_json,
    cpus_allowed_list,
    require_wsl,
    restore_signal_handlers,
    safe_environment,
    sha256,
    supervisor_command,
    install_termination_handlers,
    utc_now,
    validate_supervisor_receipt,
    verified,
)
from audit_fastchess_match import Stockfish, canonical_fen, repetition_identity
from manifest import COMBINED_CAPABILITY, M4C_CAPABILITY, SPRT_CLASSIFICATION, load_manifest, review_subject_sha256

BASE_SOURCE_KEYS = (
    "runner", "schema", "common", "manifest_module", "supervisor", "uci_preflight",
    "match_stage", "trace_auditor", "role_exec", "chess_auditor",
)
DEFAULT_BOOK_PATHS = ("book.bin", "opening.bin", "openings.bin", "book/book.bin", "book/opening.bin")


def write_state(output: Path, state: str, **extra: Any) -> None:
    atomic_json(output / "STATE.json", {"state": state, "pid": os.getpid(), "time_utc": utc_now(), **extra})


def process_executables() -> list[dict[str, Any]]:
    rows = []
    for item in Path("/proc").iterdir():
        if not item.name.isdigit() or int(item.name) == os.getpid():
            continue
        try:
            exe = Path(f"/proc/{item.name}/exe").resolve(strict=True)
            comm = Path(f"/proc/{item.name}/comm").read_text(encoding="utf-8").strip()
            rows.append({"pid": int(item.name), "exe": str(exe), "comm": comm})
        except (FileNotFoundError, ProcessLookupError, PermissionError):
            continue
    return rows


def known_live_processes(exact_paths: set[str], known_hashes: set[str]) -> list[dict[str, Any]]:
    found = []
    for row in process_executables():
        if row["exe"] in exact_paths:
            found.append({**row, "basis": "exact-path"})
            continue
        if row["comm"].lower() not in {"fastchess", "ngn", "engine", "stockfish", "stockfish18"}:
            continue
        try:
            digest = sha256(Path(row["exe"]))
        except (FileNotFoundError, PermissionError, OSError):
            continue
        if digest in known_hashes:
            found.append({**row, "sha256": digest, "basis": "known-hash-and-chess-comm"})
    return found


def source_keys(manifest: dict[str, Any]) -> tuple[str, ...]:
    return BASE_SOURCE_KEYS + (("sprt_auditor",) if manifest["classification"] == SPRT_CLASSIFICATION else ())


def artifact_specs(manifest: dict[str, Any]) -> list[tuple[str, dict[str, Any], bool]]:
    inputs = manifest["inputs"]
    result = [(name, inputs[name], name in {"runner", "supervisor", "uci_preflight", "match_stage", "trace_auditor", "role_exec", "sprt_auditor", "fastchess", "stockfish"}) for name in source_keys(manifest) + ("fastchess", "stockfish", "opening_pgn", "opening_prefixes")]
    for role in inputs["roles"]:
        result.append((f"role-{role['id']}-binary", role["binary"], True))
        result.append((f"role-{role['id']}-build-receipt", role["binary"]["provenance"]["build_receipt"], False))
        if role["network"] is not None:
            result.append((f"role-{role['id']}-network", role["network"], False))
            provenance = role["network"].get("provenance")
            if provenance is not None:
                result.append((f"role-{role['id']}-model-review", provenance["review_receipt"], False))
    return result


def verified_artifact(name: str, spec: dict[str, Any], executable: bool) -> Path:
    path = verified(Path(spec["path"]), spec["sha256"], executable)
    if "size_bytes" in spec and path.stat().st_size != spec["size_bytes"]:
        raise CandidateMatchError(f"{name}: bytes={path.stat().st_size} expected={spec['size_bytes']}")
    return path


def frozen_network_name(network: dict[str, Any]) -> str:
    names = {"ngn-v1": "network.ngn", "counter-5.5": "counter-5.5.nn"}
    try:
        return names[network["format"]]
    except KeyError as error:
        raise CandidateMatchError(f"unsupported network format {network.get('format')!r}") from error


def verified_staged_network(name: str, path: Path, spec: dict[str, Any]) -> dict[str, Any]:
    observed = {
        "sha256": sha256(path),
        "bytes": path.stat().st_size,
        "mode": f"{stat.S_IMODE(path.stat().st_mode):04o}",
    }
    if observed["sha256"] != spec["sha256"]:
        raise CandidateMatchError(f"{name}: staged SHA-256 differs")
    if "size_bytes" in spec and observed["bytes"] != spec["size_bytes"]:
        raise CandidateMatchError(
            f"{name}: staged bytes={observed['bytes']} expected={spec['size_bytes']}"
        )
    if observed["mode"] != "0444":
        raise CandidateMatchError(f"{name}: staged mode={observed['mode']} expected=0444")
    return observed


def resolve_options(role: dict[str, Any], frozen_network: Path | None) -> list[dict[str, str]]:
    result = []
    for option in role["uci_options"]:
        value = option["value"]
        if value == "$FROZEN_NETWORK":
            if frozen_network is None:
                raise CandidateMatchError(f"role {role['id']}: missing frozen network")
            value = str(frozen_network)
        elif isinstance(value, bool):
            value = "true" if value else "false"
        else:
            value = str(value)
        result.append({"name": option["name"], "value": value})
    return result


def threaded_profile(manifest: dict[str, Any]) -> bool:
    return manifest["inputs"]["roles"][0]["capability_profile"] in {M4C_CAPABILITY, COMBINED_CAPABILITY}


def fastchess_affinity_arguments(manifest: dict[str, Any]) -> list[str]:
    if threaded_profile(manifest):
        return []
    return ["-use-affinity", ",".join(str(cpu) for cpu in manifest["match"]["affinity_cpus"])]


def role_allowed_cpu_masks(manifest: dict[str, Any]) -> list[str]:
    if threaded_profile(manifest):
        return [manifest["resources"]["required_cpus_allowed_list"]]
    return [str(cpu) for cpu in manifest["match"]["affinity_cpus"]]


def validate_sprt_opening_inputs(
    opening_pgn: Path, prefixes: Path, pairs: int, opening_plies: int,
    stockfish_binary: Path | None = None, *, require_legal_unique_finals: bool = False,
    oracle_factory=Stockfish,
) -> dict[str, Any]:
    lines = [line.split() for line in prefixes.read_text(encoding="utf-8").splitlines() if line.strip()]
    if len(lines) < pairs:
        raise CandidateMatchError(f"SPRT prefix input has {len(lines)} rows, needs {pairs}")
    if require_legal_unique_finals and len(lines) != pairs:
        raise CandidateMatchError(f"combined prefix input has {len(lines)} rows, requires exactly {pairs}")
    selected = [tuple(line) for line in lines[:pairs]]
    if len(set(selected)) != pairs:
        raise CandidateMatchError("SPRT opening prefixes must be unique")
    move_pattern = re.compile(r"^[a-h][1-8][a-h][1-8][qrbn]?$")
    if any(len(line) != opening_plies or any(not move_pattern.fullmatch(token) for token in line) for line in selected):
        raise CandidateMatchError("SPRT opening prefixes do not match the fixed UCI ply contract")
    pgn_text = opening_pgn.read_text(encoding="utf-8")
    event_count = len(re.findall(r'^\[Event "', pgn_text, flags=re.MULTILINE))
    rounds = [int(value) for value in re.findall(r'^\[Round "([0-9]+)"\]$', pgn_text, flags=re.MULTILINE)]
    results = re.findall(r'^\[Result "([^"]*)"\]$', pgn_text, flags=re.MULTILINE)
    setup_headers = re.findall(r'^\[SetUp "([^"]*)"\]$', pgn_text, flags=re.MULTILINE)
    fen_headers = re.findall(r'^\[FEN "([^"]*)"\]$', pgn_text, flags=re.MULTILINE)
    if event_count < pairs:
        raise CandidateMatchError(f"SPRT opening PGN has {event_count} games, needs {pairs}")
    basic = {
        "schema": "ngn-opening-input-audit-v1", "opening_pgn_games": event_count,
        "opening_prefixes_selected": len(selected), "opening_plies": opening_plies,
        "legal_final_validation": False,
    }
    if not require_legal_unique_finals:
        return basic
    if event_count != pairs or rounds != list(range(1, pairs + 1)):
        raise CandidateMatchError("combined opening PGN must contain exactly one sequentially numbered game per pair")
    if results != ["*"] * pairs or setup_headers or fen_headers:
        raise CandidateMatchError("combined opening PGN must contain startpos games with exactly one Result * header each")
    if stockfish_binary is None:
        raise CandidateMatchError("combined opening validation requires pinned Stockfish")

    oracle = oracle_factory(stockfish_binary)
    finals = []
    try:
        for opening_index, opening in enumerate(selected, start=1):
            played: list[str] = []
            position = oracle.query(played)
            for ply, move in enumerate(opening, start=1):
                if not position["legal"]:
                    raise CandidateMatchError(f"opening {opening_index} ply {ply}: move follows terminal position")
                if move not in position["legal"]:
                    raise CandidateMatchError(f"opening {opening_index} ply {ply}: illegal {move}; fen={position['fen']}")
                played.append(move)
                position = oracle.query(played)
            if not position["legal"]:
                raise CandidateMatchError(f"opening {opening_index}: fixed prefix ends in terminal position")
            finals.append({
                "opening": opening_index, "prefix": " ".join(opening),
                "final_fen": canonical_fen(position["fen"]),
                "final_identity": repetition_identity(position),
            })
    finally:
        oracle.close()
    identities = [item["final_identity"] for item in finals]
    if len(set(identities)) != pairs:
        raise CandidateMatchError("SPRT opening prefixes do not reach unique legal final positions")
    return {
        **basic, "legal_final_validation": True,
        "all_moves_legal": True, "all_final_positions_nonterminal": True,
        "unique_final_position_identities": len(set(identities)), "records": finals,
    }


def sprt_fastchess_arguments(manifest: dict[str, Any]) -> list[str]:
    if manifest["classification"] != SPRT_CLASSIFICATION:
        return []
    sprt = manifest["sprt"]
    return [
        "-report", "penta=true", "-sprt", f"elo0={sprt['elo0']}", f"elo1={sprt['elo1']}",
        f"alpha={sprt['alpha']}", f"beta={sprt['beta']}", f"model={sprt['model']}",
        "-ratinginterval", str(sprt["rating_interval"]),
    ]


def write_role_launcher(launcher_dir: Path, source: Path, config: dict[str, Any]) -> Path:
    launcher_dir.mkdir(parents=True, exist_ok=False)
    launcher = launcher_dir / "role_exec.py"
    shutil.copy2(source, launcher)
    launcher.chmod(0o555)
    atomic_json(launcher_dir / "role-config.json", config)
    return launcher


def run_stage(
    active: ActiveSupervisor,
    source: dict[str, Path],
    output: Path,
    manifest: dict[str, Any],
    label: str,
    limit_key: str,
    command: list[str],
) -> dict[str, Any]:
    stage_dir = output / "commands" / label
    limits = manifest["limits"]
    resources = manifest["resources"]
    cpu_list = ",".join(str(cpu) for cpu in resources["runner_cpus"])
    argv = supervisor_command(
        source["supervisor"], label, stage_dir, float(limits[limit_key]),
        resources["memory_limit_kib"], cpu_list, command,
        float(limits["term_grace_seconds"]), float(limits["sample_interval_seconds"]),
    )
    rc = active.run(argv, output, safe_environment())
    try:
        receipt = validate_supervisor_receipt(stage_dir / "supervisor-receipt.json")
        if rc != 0:
            raise CandidateMatchError(f"supervisor {label} rc={rc}")
        if (stage_dir / "stderr").stat().st_size:
            raise CandidateMatchError(f"supervised stage {label} emitted stderr")
    except Exception:
        raise
    else:
        active.clear()
    return receipt


def hash_tree(output: Path) -> Path:
    receipt = output / "receipts" / "final-files.sha256"
    temporary = receipt.with_suffix(".sha256.tmp")
    excluded = {receipt, temporary, output / "STATE.json"}
    with temporary.open("x", encoding="utf-8") as handle:
        for path in sorted(item for item in output.rglob("*") if item.is_file() and item not in excluded):
            handle.write(f"{sha256(path)}  {path}\n")
    os.replace(temporary, receipt)
    return receipt


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--manifest", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--approval-token", required=True)
    args = parser.parse_args()
    require_wsl()
    output = args.output.resolve()
    output.mkdir(parents=True, exist_ok=False)
    (output / "receipts").mkdir()
    write_state(output, "PREPARING")
    active = ActiveSupervisor()
    previous_handlers = install_termination_handlers()
    frozen_paths: set[str] = set()
    known_hashes: set[str] = set()
    success = False
    try:
        manifest_path = args.manifest.resolve(strict=True)
        manifest = load_manifest(manifest_path)
        approval = manifest["approval"]
        if manifest["status"] != "APPROVED_TO_RUN" or approval["authority"] != "root" or args.approval_token != approval["token"]:
            raise CandidateMatchError("manifest is held or approval token differs")
        if review_subject_sha256(manifest) != approval["reviewed_manifest_sha256"]:
            raise CandidateMatchError("root approval does not bind normalized HELD review subject")
        specs = artifact_specs(manifest)
        originals: dict[str, Path] = {}
        for name, spec, executable in specs:
            path = verified_artifact(name, spec, executable)
            originals[name] = path
            frozen_paths.add(str(path))
            known_hashes.add(spec["sha256"])
        if originals["runner"] != Path(__file__).resolve():
            raise CandidateMatchError("running launcher differs from reviewed manifest")
        if cpus_allowed_list() != manifest["resources"]["required_cpus_allowed_list"]:
            raise CandidateMatchError(f"runner CPU mask {cpus_allowed_list()} differs from manifest")
        core_ids = [Path(f"/sys/devices/system/cpu/cpu{cpu}/topology/core_id").read_text().strip() for cpu in manifest["resources"]["runner_cpus"]]
        if len(set(core_ids)) != len(core_ids):
            raise CandidateMatchError(f"runner CPUs are not distinct physical cores: {core_ids}")
        before = known_live_processes(frozen_paths, known_hashes)
        atomic_json(output / "receipts" / "pre-run-processes.json", {"matches": before, "scope": "exact executable path plus known hash on chess process names"})
        if before:
            raise CandidateMatchError(f"known pre-run chess processes exist: {before}")

        source_dir = output / "source"
        inputs_dir = output / "inputs"
        commands_dir = output / "commands"
        configs_dir = output / "configs"
        artifacts_dir = output / "artifacts"
        for directory in (source_dir, inputs_dir, commands_dir, configs_dir, artifacts_dir):
            directory.mkdir()
        source_map = []
        source: dict[str, Path] = {}
        for name in source_keys(manifest):
            original = originals[name]
            destination = source_dir / original.name
            if destination.exists():
                raise CandidateMatchError(f"source basename collision: {destination.name}")
            shutil.copy2(original, destination)
            source[name] = destination
            copied_sha = sha256(destination)
            if copied_sha != sha256(original):
                raise CandidateMatchError(f"source copy differs: {name}")
            source_map.append({"kind": name, "original": str(original), "copy": str(destination), "sha256": copied_sha})
            if os.access(destination, os.X_OK):
                frozen_paths.add(str(destination.resolve()))
        manifest_copy = source_dir / "manifest.json"
        shutil.copy2(manifest_path, manifest_copy)
        source_map.append({"kind": "manifest", "original": str(manifest_path), "copy": str(manifest_copy), "sha256": sha256(manifest_copy)})
        evidence_dir = source_dir / "evidence"
        evidence_dir.mkdir()
        for role in manifest["inputs"]["roles"]:
            original = originals[f"role-{role['id']}-build-receipt"]
            destination = evidence_dir / f"{role['id']}-build-receipt{original.suffix}"
            shutil.copy2(original, destination)
            source_map.append({"kind": f"role-{role['id']}-build-receipt", "original": str(original), "copy": str(destination), "sha256": sha256(destination)})
            if role["network"] is not None and role["network"].get("provenance") is not None:
                original = originals[f"role-{role['id']}-model-review"]
                destination = evidence_dir / f"{role['id']}-model-review{original.suffix}"
                shutil.copy2(original, destination)
                source_map.append({"kind": f"role-{role['id']}-model-review", "original": str(original), "copy": str(destination), "sha256": sha256(destination)})
        atomic_json(source_dir / "source-map.json", source_map)

        tools_dir = inputs_dir / "tools"
        tools_dir.mkdir()
        frozen_fastchess = tools_dir / "fastchess"
        frozen_stockfish = tools_dir / "stockfish"
        shutil.copy2(originals["fastchess"], frozen_fastchess)
        shutil.copy2(originals["stockfish"], frozen_stockfish)
        frozen_fastchess.chmod(0o555)
        frozen_stockfish.chmod(0o555)
        frozen_paths.update({str(frozen_fastchess.resolve()), str(frozen_stockfish.resolve())})
        frozen_openings = inputs_dir / "openings.pgn"
        frozen_prefixes = inputs_dir / "opening-prefixes.txt"
        shutil.copy2(originals["opening_pgn"], frozen_openings)
        shutil.copy2(originals["opening_prefixes"], frozen_prefixes)
        loaded = []
        role_runtime = []
        for index, role in enumerate(manifest["inputs"]["roles"]):
            role_input = inputs_dir / "roles" / role["id"]
            role_input.mkdir(parents=True)
            engine = role_input / "engine"
            shutil.copy2(originals[f"role-{role['id']}-binary"], engine)
            engine.chmod(0o555)
            network = None
            network_observation = None
            if role["network"] is not None:
                network_name = frozen_network_name(role["network"])
                network = role_input / network_name
                shutil.copy2(originals[f"role-{role['id']}-network"], network)
                network.chmod(0o444)
                network_observation = verified_staged_network(
                    f"role-{role['id']}-network", network, role["network"],
                )
            cwd = output / "engine-cwd" / role["id"]
            cwd.mkdir(parents=True)
            options = resolve_options(role, network)
            launcher = write_role_launcher(
                output / "launchers" / role["id"], source["role_exec"],
                {"schema": "ngn-candidate-role-exec-v1", "engine": str(engine), "engine_sha256": role["binary"]["sha256"], "gomaxprocs": role["environment"]["GOMAXPROCS"], "expected_cwd": str(cwd)},
            )
            frozen_paths.update({str(engine.resolve()), str(launcher.resolve())})
            crash_log = engine.parent / "ngn_crashes.log"
            if crash_log.exists():
                raise CandidateMatchError(f"role {role['id']}: crash log exists before execution")
            role_runtime.append({"manifest": role, "engine": engine, "network": network, "cwd": cwd, "launcher": launcher, "options": options, "crash_log": crash_log, "index": index})
            loaded.append({"kind": f"role-{role['id']}-binary", "original": role["binary"]["path"], "copy": str(engine), "sha256": sha256(engine)})
            if network is not None:
                loaded.append({
                    "kind": f"role-{role['id']}-network", "original": role["network"]["path"],
                    "copy": str(network), "format": role["network"]["format"],
                    **network_observation,
                })
        for kind, original, copy in (("fastchess", originals["fastchess"], frozen_fastchess), ("stockfish", originals["stockfish"], frozen_stockfish), ("opening_pgn", originals["opening_pgn"], frozen_openings), ("opening_prefixes", originals["opening_prefixes"], frozen_prefixes)):
            loaded.append({"kind": kind, "original": str(original), "copy": str(copy), "sha256": sha256(copy)})
        atomic_json(output / "receipts" / "loaded-inputs.json", loaded)
        for item in loaded:
            expected = next(spec["sha256"] for name, spec, _ in specs if name == item["kind"])
            if item["sha256"] != expected:
                raise CandidateMatchError(f"loaded input differs: {item['kind']}")
        if manifest["classification"] == SPRT_CLASSIFICATION:
            opening_audit = validate_sprt_opening_inputs(
                frozen_openings, frozen_prefixes, manifest["match"]["pairs"], manifest["match"]["opening_plies"],
                frozen_stockfish,
                require_legal_unique_finals=(
                    manifest["inputs"]["roles"][0]["capability_profile"] == COMBINED_CAPABILITY
                ),
            )
            atomic_json(output / "receipts" / "opening-input-audit.json", opening_audit)

        environment_receipt = {"fastchess": safe_environment(), "role_overrides": {item["manifest"]["id"]: item["manifest"]["environment"] for item in role_runtime}, "policy": "fixed C/UTC environment; inherited NGN_* and Go debug/tuning variables absent"}
        atomic_json(output / "receipts" / "environment.json", environment_receipt)
        book_receipt = {"paths": list(DEFAULT_BOOK_PATHS), "pre": {item["manifest"]["id"]: [path for path in DEFAULT_BOOK_PATHS if (item["cwd"] / path).exists()] for item in role_runtime}}
        if any(book_receipt["pre"].values()):
            raise CandidateMatchError("external default book path exists before run")

        write_state(output, "PREFLIGHT_RUNNING")
        preflight_crashes = []
        for item in role_runtime:
            role = item["manifest"]
            config_path = configs_dir / f"preflight-{role['id']}.json"
            preflight_output = artifacts_dir / "preflight" / role["id"]
            atomic_json(config_path, {"schema": "ngn-candidate-uci-preflight-v1", "role_id": role["id"], "launcher": str(item["launcher"]), "cwd": str(item["cwd"]), "backend": role["backend"], "capability_profile": role["capability_profile"], "options": item["options"], "barrier_timeout_seconds": min(30.0, float(manifest["limits"]["preflight_seconds"]))})
            run_stage(active, source, output, manifest, f"preflight-{role['id']}", "preflight_seconds", [sys.executable, str(source["uci_preflight"]), "--config", str(config_path), "--output", str(preflight_output)])
            if not item["crash_log"].is_file():
                raise CandidateMatchError(f"role {role['id']}: expected crash-handler initialization absent in preflight")
            frozen_crash = preflight_output / "ngn_crashes.log"
            shutil.move(item["crash_log"], frozen_crash)
            preflight_crashes.append({"path": str(frozen_crash), "logged_path": str(item["crash_log"]), "expected_processes": 1, "role_id": role["id"], "phase": "preflight"})

        match = manifest["match"]
        pgn = output / "games.pgn"
        final_epd = output / "final.epd"
        trace = output / "fastchess.log"
        command = [str(frozen_fastchess)]
        for item in role_runtime:
            role = item["manifest"]
            command.extend(["-engine", f"cmd={item['launcher']}", f"name={role['display_name']}", f"dir={item['cwd']}"])
            command.extend(f"option.{option['name']}={option['value']}" for option in item["options"])
        command.extend([
            "-openings", f"file={frozen_openings}", "format=pgn", "order=sequential", "start=1", f"plies={match['opening_plies']}",
            "-each", f"tc={match['time_control']}", "timemargin=0",
            "-rounds", str(match["pairs"]), "-games", "2", "-repeat", "-concurrency", str(match["concurrency"]),
        ])
        sprt_mode = manifest["classification"] == SPRT_CLASSIFICATION
        command.extend(sprt_fastchess_arguments(manifest))
        command.extend(fastchess_affinity_arguments(manifest))
        command.extend([
            "-srand", str(match["seed"]), "-strict", "-show-latency",
            "-pgnout", f"file={pgn}", "notation=uci", "append=false", "nodes=true", "seldepth=true", "nps=true", "hashfull=true", "timeleft=true", "latency=true", "pv=true",
            "-epdout", f"file={final_epd}", "append=false",
            "-log", f"file={trace}", "level=trace", "engine=true", "realtime=false", "append=false",
        ])
        atomic_json(output / "match.command.json", command)
        match_stage_config = configs_dir / "match-stage.json"
        allowed_masks = role_allowed_cpu_masks(manifest)
        child_witness = output / "receipts" / "child-processes.json"
        atomic_json(match_stage_config, {
            "schema": "ngn-candidate-match-stage-v1", "command": command, "cwd": str(output), "environment": safe_environment(),
            "roles": [{"id": item["manifest"]["id"], "engine": str(item["engine"]), "engine_sha256": item["manifest"]["binary"]["sha256"], "cwd": str(item["cwd"]), "gomaxprocs": item["manifest"]["environment"]["GOMAXPROCS"], "allowed_cpu_masks": allowed_masks, "launcher": str(item["launcher"])} for item in role_runtime],
            "sample_interval_seconds": min(0.05, float(manifest["limits"]["sample_interval_seconds"])),
            "match_stdout": str(output / "match.stdout"), "match_stderr": str(output / "match.stderr"), "witness": str(child_witness),
        })
        write_state(output, "MATCH_RUNNING", command_sha256=sha256(output / "match.command.json"))
        run_stage(active, source, output, manifest, "match", "match_seconds", [sys.executable, str(source["match_stage"]), "--config", str(match_stage_config)])
        witness = json.loads(child_witness.read_text(encoding="utf-8"))
        if witness.get("state") != "COMPLETE" or witness.get("violations") or witness.get("fastchess_returncode") != 0:
            raise CandidateMatchError("child-process witness failed")
        (output / "match.exit").write_text(str(witness["fastchess_returncode"]) + "\n", encoding="utf-8")
        if (output / "match.stderr").stat().st_size:
            raise CandidateMatchError("fastchess process stderr nonempty")

        def run_chess_audit() -> dict[str, Any]:
            audit_output = output / "audit.json"
            audit_command = [
                sys.executable, str(source["chess_auditor"]), "--pgn", str(pgn), "--final-epd", str(final_epd),
                "--opening-prefixes", str(frozen_prefixes), "--opening-prefixes-sha256", manifest["inputs"]["opening_prefixes"]["sha256"],
                "--opening-pgn", str(frozen_openings), "--opening-pgn-sha256", manifest["inputs"]["opening_pgn"]["sha256"],
                "--stockfish", str(frozen_stockfish), "--stockfish-sha256", manifest["inputs"]["stockfish"]["sha256"],
                "--engine-a", role_runtime[0]["manifest"]["display_name"], "--engine-b", role_runtime[1]["manifest"]["display_name"],
            ]
            if sprt_mode:
                audit_command.extend(["--max-games", str(match["games"]), "--max-pairs", str(match["pairs"])])
            else:
                audit_command.extend(["--games", str(match["games"]), "--pairs", str(match["pairs"])])
            audit_command.extend(["--output", str(audit_output)])
            atomic_json(output / "audit.command.json", audit_command)
            write_state(output, "CHESS_AUDIT_RUNNING")
            run_stage(active, source, output, manifest, "chess-audit", "chess_audit_seconds", audit_command)
            result = json.loads(audit_output.read_text(encoding="utf-8"))
            if result.get("status") != "PASS":
                raise CandidateMatchError("independent chess audit status differs")
            if sprt_mode and result.get("count_mode") != "maximum":
                raise CandidateMatchError("independent SPRT chess audit count mode differs")
            if not sprt_mode and (result.get("games") != match["games"] or result.get("pairs") != match["pairs"]):
                raise CandidateMatchError("independent exact audit counts differ")
            if sprt_mode and (
                not isinstance(result.get("games"), int) or result["games"] < 2 or result["games"] > match["games"]
                or result["games"] % 2 or result.get("pairs") != result["games"] // 2
            ):
                raise CandidateMatchError("independent SPRT audit completed counts are invalid")
            if result.get("probable_embedded_book_signature_plies") != 0:
                raise CandidateMatchError("no-book candidate audit found probable embedded-book signature plies")
            return result

        audit = run_chess_audit() if sprt_mode else None
        completed_games = audit["games"] if sprt_mode else match["games"]
        trace_config = configs_dir / "trace-audit.json"
        trace_roles = []
        match_crashes = []
        for item in role_runtime:
            role_id = item["manifest"]["id"]
            expected_processes = len(witness["observed_instances"][role_id])
            trace_roles.append({
                "id": role_id, "display_name": item["manifest"]["display_name"],
                "capability_profile": item["manifest"]["capability_profile"],
                "resolved_options": item["options"], "expected_processes": expected_processes,
                "expected_refreshes": completed_games,
            })
            match_crashes.append({"path": str(item["crash_log"]), "logged_path": str(item["crash_log"]), "expected_processes": expected_processes, "role_id": role_id, "phase": "match"})
        trace_output = output / "audit-operational.json"
        atomic_json(trace_config, {"schema": "ngn-candidate-trace-audit-config-v1", "trace": str(trace), "match_exit": str(output / "match.exit"), "match_stderr": str(output / "match.stderr"), "roles": trace_roles, "crash_logs": preflight_crashes + match_crashes, "output": str(trace_output)})
        write_state(output, "TRACE_AUDIT_RUNNING")
        run_stage(active, source, output, manifest, "trace-audit", "trace_audit_seconds", [sys.executable, str(source["trace_auditor"]), "--config", str(trace_config)])
        operational = json.loads(trace_output.read_text(encoding="utf-8"))
        if not operational.get("pass"):
            raise CandidateMatchError("operational trace audit did not pass")

        if not sprt_mode:
            audit = run_chess_audit()

        sprt_audit = None
        if sprt_mode:
            sprt_output = output / "audit-sprt.json"
            sprt = manifest["sprt"]
            sprt_command = [
                sys.executable, str(source["sprt_auditor"]), "--chess-audit", str(output / "audit.json"),
                "--match-stdout", str(output / "match.stdout"), "--elo0", str(sprt["elo0"]),
                "--elo1", str(sprt["elo1"]), "--alpha", str(sprt["alpha"]), "--beta", str(sprt["beta"]),
                "--max-pairs", str(match["pairs"]), "--output", str(sprt_output),
            ]
            atomic_json(output / "sprt-audit.command.json", sprt_command)
            write_state(output, "SPRT_AUDIT_RUNNING")
            run_stage(active, source, output, manifest, "sprt-audit", "sprt_audit_seconds", sprt_command)
            sprt_audit = json.loads(sprt_output.read_text(encoding="utf-8"))
            if sprt_audit.get("status") != "PASS" or sprt_audit.get("completed_games") != audit["games"]:
                raise CandidateMatchError("independent SPRT audit status/counts differ")

        book_receipt["post"] = {item["manifest"]["id"]: [path for path in DEFAULT_BOOK_PATHS if (item["cwd"] / path).exists()] for item in role_runtime}
        atomic_json(output / "receipts" / "external-book-paths.json", book_receipt)
        if any(book_receipt["post"].values()):
            raise CandidateMatchError("external default book path appeared")
        for item in loaded:
            if sha256(Path(item["copy"])) != item["sha256"]:
                raise CandidateMatchError(f"post-run input mutation: {item['kind']}")
        after = known_live_processes({str(frozen_fastchess), *(str(item["engine"]) for item in role_runtime)}, known_hashes)
        atomic_json(output / "receipts" / "terminal-processes.json", {"matches": after, "scope": "exact frozen paths plus known hash on chess process names"})
        if after:
            raise CandidateMatchError(f"known post-run chess processes exist: {after}")
        terminal = {"schema": "ngn-candidate-match-terminal-v1", "state": "COMPLETE", "started_manifest_sha256": sha256(manifest_copy), "review_subject_sha256": review_subject_sha256(manifest), "ended_utc": utc_now(), "operational_pass": True, "independent_chess_audit_pass": True, "games": audit["games"], "pairs": audit["pairs"]}
        if sprt_mode:
            terminal.update({
                "requested_max_games": match["games"], "requested_max_pairs": match["pairs"],
                "independent_sprt_audit_pass": True, "penta_0_to_4": sprt_audit["final_penta_0_to_4"],
                "llr": sprt_audit["final_llr"], "lower_bound": sprt_audit["lower_bound"],
                "upper_bound": sprt_audit["upper_bound"], "stop_reason": sprt_audit["stop_reason"],
                "verdict": sprt_audit["verdict"], "first_boundary_crossing": sprt_audit["first_boundary_crossing"],
            })
        atomic_json(output / "terminal.json", terminal)
        final_receipt = hash_tree(output)
        write_state(output, "COMPLETE", final_files_sha256=sha256(final_receipt))
        success = True
        print(json.dumps({"state": "COMPLETE", "output": str(output), "final_files_sha256": sha256(final_receipt)}, indent=2))
        return 0
    except BaseException as error:
        restore_signal_handlers(previous_handlers)
        cleanup_report = active.terminate()
        try:
            after = known_live_processes(frozen_paths, known_hashes)
            atomic_json(output / "receipts" / "terminal-processes.json", {"matches": after, "scope": "exact paths plus known hash on chess process names", "during_failure": True})
        except Exception as process_error:
            after = [{"process_check_error": str(process_error)}]
        atomic_json(output / "terminal.json", {"schema": "ngn-candidate-match-terminal-v1", "state": "FAILED", "ended_utc": utc_now(), "error": str(error), "supervisor_cleanup": cleanup_report, "remaining_processes": after})
        try:
            final_receipt = hash_tree(output)
            write_state(output, "FAILED", error=str(error), final_files_sha256=sha256(final_receipt))
        except Exception as final_error:
            write_state(output, "FAILED_EVIDENCE", error=str(error), finalization_error=str(final_error))
        print(f"candidate match failed: {error}", file=sys.stderr)
        return 1
    finally:
        restore_signal_handlers(previous_handlers)
        if not success and active.process is not None and active.last_cleanup is None:
            active.terminate()


if __name__ == "__main__":
    raise SystemExit(main())
