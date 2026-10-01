#!/usr/bin/env python3
"""Execute one approved fixed NGN SF18 BIG versus pinned Counter 5.5 cell."""
from __future__ import annotations

import argparse
import json
import os
import shutil
import sys
from pathlib import Path

from common import (
    ActiveSupervisor,
    CandidateMatchError,
    atomic_json,
    cpus_allowed_list,
    install_termination_handlers,
    require_wsl,
    restore_signal_handlers,
    safe_environment,
    sha256,
    utc_now,
    verified,
)
from external_match_manifest import load_manifest, review_subject_sha256
from external_profiles import profile_for, resolved_options
from external_report import report as comparison_report
from run_candidate_match import (
    DEFAULT_BOOK_PATHS,
    hash_tree,
    known_live_processes,
    run_stage,
    write_role_launcher,
    write_state,
)
from trace_audit import crash_lines

SOURCE_KEYS = (
    "runner", "runner_helpers", "manifest_module", "legacy_manifest", "common", "supervisor",
    "role_exec", "uci_preflight", "external_profiles", "external_admission",
    "external_report", "match_stage", "trace_auditor", "trace_helpers",
    "chess_auditor",
)


def stage_manifest(manifest: dict) -> dict:
    return {
        "limits": {
            "preflight_seconds": manifest["limits"]["preflight_seconds"],
            "match_seconds": manifest["limits"]["match_seconds"],
            "trace_audit_seconds": manifest["limits"]["audit_seconds"],
            "chess_audit_seconds": manifest["limits"]["audit_seconds"],
            "term_grace_seconds": manifest["limits"]["term_grace_seconds"],
            "sample_interval_seconds": manifest["limits"]["sample_interval_seconds"],
        },
        "resources": {
            "runner_cpus": manifest["resources"]["cpus"],
            "memory_limit_kib": manifest["resources"]["memory_limit_kib"],
        },
    }


def option_strings(options: list[dict], network: Path | None = None) -> list[dict[str, str]]:
    result = []
    for item in options:
        value = item["value"]
        if value == "$FROZEN_NETWORK":
            if network is None:
                raise CandidateMatchError("candidate network is absent")
            value = str(network)
        elif isinstance(value, bool):
            value = "true" if value else "false"
        else:
            value = str(value)
        result.append({"name": item["name"], "value": value})
    return result


def copy_checked(spec: dict, destination: Path, executable: bool = False) -> Path:
    original = verified(Path(spec["path"]), spec["sha256"], executable)
    destination.parent.mkdir(parents=True, exist_ok=True)
    shutil.copy2(original, destination)
    if sha256(destination) != spec["sha256"]:
        raise CandidateMatchError(f"copy differs: {destination}")
    destination.chmod(0o555 if executable else 0o444)
    return destination


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
        started_manifest_sha256 = sha256(manifest_path)
        manifest = load_manifest(manifest_path, verify_files=True)
        approval = manifest["approval"]
        if (
            manifest["status"] != "APPROVED_TO_RUN"
            or approval["authority"] != "root"
            or args.approval_token != approval["token"]
            or review_subject_sha256(manifest) != approval["reviewed_manifest_sha256"]
        ):
            raise CandidateMatchError("exact root execution approval is absent")
        if Path(manifest["inputs"]["runner"]["path"]).resolve(strict=True) != Path(__file__).resolve():
            raise CandidateMatchError("running external launcher differs from manifest")
        if cpus_allowed_list() != manifest["resources"]["cpus_allowed_list"]:
            raise CandidateMatchError("runner CPU mask differs")
        core_ids = [
            Path(f"/sys/devices/system/cpu/cpu{cpu}/topology/core_id").read_text().strip()
            for cpu in manifest["resources"]["cpus"]
        ]
        if len(set(core_ids)) != len(core_ids):
            raise CandidateMatchError("runner CPU mask contains SMT siblings")

        executable_input_names = {"runner", "supervisor", "role_exec", "uci_preflight",
            "external_admission", "match_stage", "trace_auditor", "chess_auditor",
            "fastchess", "stockfish"}
        for name, spec in manifest["inputs"].items():
            path = verified(Path(spec["path"]), spec["sha256"], name in executable_input_names)
            if name in {"fastchess", "stockfish"}:
                frozen_paths.add(str(path))
                known_hashes.add(spec["sha256"])
        for role_name in ("candidate", "opponent"):
            spec = manifest[role_name]["binary"]
            path = verified(Path(spec["path"]), spec["sha256"], True)
            frozen_paths.add(str(path))
            known_hashes.add(spec["sha256"])
        before = known_live_processes(frozen_paths, known_hashes)
        atomic_json(output / "receipts/pre-run-processes.json", {"matches": before})
        if before:
            raise CandidateMatchError(f"known pre-run chess processes exist: {before}")

        source_dir = output / "source"
        inputs_dir = output / "inputs"
        configs_dir = output / "configs"
        artifacts_dir = output / "artifacts"
        commands_dir = output / "commands"
        for directory in (source_dir, inputs_dir, configs_dir, artifacts_dir, commands_dir):
            directory.mkdir()
        source = {}
        source_map = []
        for name in SOURCE_KEYS:
            spec = manifest["inputs"][name]
            original = Path(spec["path"]).resolve(strict=True)
            destination = source_dir / original.name
            if destination.exists():
                raise CandidateMatchError(f"source basename collision: {destination.name}")
            copy_checked(spec, destination, name in executable_input_names)
            source[name] = destination
            source_map.append({"kind": name, "original": str(original), "copy": str(destination), "sha256": sha256(destination)})
        manifest_copy = source_dir / "manifest.json"
        shutil.copy2(manifest_path, manifest_copy)
        source_map.append({"kind": "manifest", "original": str(manifest_path), "copy": str(manifest_copy), "sha256": sha256(manifest_copy)})
        atomic_json(source_dir / "source-map.json", source_map)

        frozen_fastchess = copy_checked(manifest["inputs"]["fastchess"], inputs_dir / "tools/fastchess", True)
        frozen_stockfish = copy_checked(manifest["inputs"]["stockfish"], inputs_dir / "tools/stockfish", True)
        frozen_openings = copy_checked(manifest["inputs"]["opening_pgn"], inputs_dir / "openings.pgn")
        frozen_prefixes = copy_checked(manifest["inputs"]["opening_uci"], inputs_dir / "opening-prefixes.txt")
        evidence_names = (
            "opening_selection_receipt", "opening_history_audit",
            "opening_stockfish_admission", "opening_root_review",
        )
        evidence_copies = {}
        for name in evidence_names:
            evidence_copies[name] = copy_checked(
                manifest["inputs"][name], inputs_dir / "evidence" / f"{name}.json",
            )

        candidate_engine = copy_checked(manifest["candidate"]["binary"], inputs_dir / "candidate/engine", True)
        candidate_model = copy_checked(manifest["candidate"]["model"], inputs_dir / "candidate/sf18-big.nnue")
        candidate_build = copy_checked(manifest["candidate"]["build_receipt"], inputs_dir / "candidate/build-receipt.json")
        opponent_engine = copy_checked(manifest["opponent"]["binary"], inputs_dir / "opponent/engine", True)
        opponent_identity = copy_checked(manifest["opponent"]["identity_receipt"], inputs_dir / "opponent/identity.json")
        bound_admission = copy_checked(manifest["opponent"]["admission_receipt"], inputs_dir / "opponent/bound-admission.json")
        bound_transcript = copy_checked(manifest["opponent"]["admission_transcript"], inputs_dir / "opponent/bound-admission-transcript.jsonl")
        post_checks = [
            (frozen_fastchess, manifest["inputs"]["fastchess"]["sha256"]),
            (frozen_stockfish, manifest["inputs"]["stockfish"]["sha256"]),
            (frozen_openings, manifest["inputs"]["opening_pgn"]["sha256"]),
            (frozen_prefixes, manifest["inputs"]["opening_uci"]["sha256"]),
            (candidate_engine, manifest["candidate"]["binary"]["sha256"]),
            (candidate_model, manifest["candidate"]["model"]["sha256"]),
            (candidate_build, manifest["candidate"]["build_receipt"]["sha256"]),
            (opponent_engine, manifest["opponent"]["binary"]["sha256"]),
            (opponent_identity, manifest["opponent"]["identity_receipt"]["sha256"]),
            (bound_admission, manifest["opponent"]["admission_receipt"]["sha256"]),
            (bound_transcript, manifest["opponent"]["admission_transcript"]["sha256"]),
            *[(path, manifest["inputs"][name]["sha256"]) for name, path in evidence_copies.items()],
        ]
        post_checks.extend(
            (source[name], manifest["inputs"][name]["sha256"]) for name in SOURCE_KEYS
        )
        post_checks.append((manifest_copy, started_manifest_sha256))
        frozen_paths.update({str(frozen_fastchess), str(frozen_stockfish), str(candidate_engine), str(opponent_engine)})

        candidate_cwd = output / "engine-cwd/candidate"
        opponent_cwd = output / "engine-cwd/opponent"
        candidate_cwd.mkdir(parents=True)
        opponent_cwd.mkdir(parents=True)
        width = manifest["cell"]["width"]
        candidate_options = option_strings(manifest["candidate"]["options"], candidate_model)
        opponent_options = option_strings(resolved_options(profile_for(manifest["opponent"]["profile_id"]), width))
        candidate_launcher = write_role_launcher(
            output / "launchers/candidate", source["role_exec"],
            {"schema": "ngn-candidate-role-exec-v1", "engine": str(candidate_engine),
             "engine_sha256": manifest["candidate"]["binary"]["sha256"],
             "gomaxprocs": str(width), "expected_cwd": str(candidate_cwd)},
        )
        opponent_launcher = write_role_launcher(
            output / "launchers/opponent", source["role_exec"],
            {"schema": "ngn-candidate-role-exec-v1", "engine": str(opponent_engine),
             "engine_sha256": manifest["opponent"]["binary"]["sha256"],
             "gomaxprocs": str(width), "expected_cwd": str(opponent_cwd)},
        )
        frozen_paths.update({str(candidate_launcher.resolve()), str(opponent_launcher.resolve())})
        candidate_crash_log = candidate_engine.parent / "ngn_crashes.log"
        opponent_crash_log = opponent_engine.parent / "ngn_crashes.log"
        if candidate_crash_log.exists() or opponent_crash_log.exists():
            raise CandidateMatchError("run-local crash log exists before preflight")

        context = stage_manifest(manifest)
        write_state(output, "PREFLIGHT_RUNNING")
        capability = "ngn-sf18-big-one-worker-v1"
        candidate_preflight_config = configs_dir / "candidate-preflight.json"
        atomic_json(candidate_preflight_config, {
            "schema": "ngn-candidate-uci-preflight-v1", "role_id": manifest["candidate"]["id"],
            "launcher": str(candidate_launcher), "cwd": str(candidate_cwd),
            "backend": "sf18-big", "capability_profile": capability,
            "options": candidate_options,
            "barrier_timeout_seconds": min(30.0, float(manifest["limits"]["preflight_seconds"])),
        })
        candidate_preflight_out = artifacts_dir / "candidate-preflight"
        run_stage(active, source, output, context, "candidate-preflight", "preflight_seconds",
                  [sys.executable, str(source["uci_preflight"]), "--config", str(candidate_preflight_config), "--output", str(candidate_preflight_out)])
        if not candidate_crash_log.is_file():
            raise CandidateMatchError("candidate crash-handler initialization absent")
        preflight_crash = candidate_preflight_out / "ngn_crashes.log"
        shutil.move(candidate_crash_log, preflight_crash)
        atomic_json(output / "receipts/candidate-preflight-crash.json",
                    crash_lines(preflight_crash, candidate_crash_log, 1))

        external_config = configs_dir / "external-admission.json"
        external_out = artifacts_dir / "external-admission"
        atomic_json(external_config, {
            "schema": "ngn-external-uci-admission-v1",
            "profile_id": manifest["opponent"]["profile_id"],
            "binary": str(opponent_engine), "identity_receipt": str(opponent_identity),
            "cwd": str(opponent_cwd), "width": width,
            "cpus_allowed_list": manifest["resources"]["cpus_allowed_list"],
            "barrier_timeout_seconds": min(30.0, float(manifest["limits"]["preflight_seconds"])),
        })
        run_stage(active, source, output, context, "external-admission", "preflight_seconds",
                  [sys.executable, str(source["external_admission"]), "--config", str(external_config), "--output", str(external_out)])
        if opponent_crash_log.exists():
            raise CandidateMatchError("external opponent unexpectedly created NGN crash log")

        book_receipt = {"paths": list(DEFAULT_BOOK_PATHS),
            "pre": {"candidate": [x for x in DEFAULT_BOOK_PATHS if (candidate_cwd / x).exists()],
                    "opponent": [x for x in DEFAULT_BOOK_PATHS if (opponent_cwd / x).exists()]}}
        if any(book_receipt["pre"].values()):
            raise CandidateMatchError("default book path exists before match")

        cell = manifest["cell"]
        pgn = output / "games.pgn"
        final_epd = output / "final.epd"
        trace = output / "fastchess.log"
        command = [str(frozen_fastchess)]
        for launcher, name, cwd, options in (
            (candidate_launcher, manifest["candidate"]["display_name"], candidate_cwd, candidate_options),
            (opponent_launcher, manifest["opponent"]["display_name"], opponent_cwd, opponent_options),
        ):
            command.extend(["-engine", f"cmd={launcher}", f"name={name}", f"dir={cwd}"])
            command.extend(f"option.{item['name']}={item['value']}" for item in options)
        command.extend([
            "-openings", f"file={frozen_openings}", "format=pgn", "order=sequential", "start=1", "plies=16",
            "-each", f"tc={cell['time_control']}", "timemargin=0",
            "-rounds", "50", "-games", "2", "-repeat", "-concurrency", "1",
            "-srand", "20260906",
        ])
        if cell["strict"]:
            command.append("-strict")
        command.extend([
            "-show-latency",
            "-pgnout", f"file={pgn}", "notation=uci", "append=false", "nodes=true",
            "seldepth=true", "nps=true", "hashfull=true", "timeleft=true", "latency=true", "pv=true",
            "-epdout", f"file={final_epd}", "append=false",
            "-log", f"file={trace}", "level=trace", "engine=true", "realtime=false", "append=false",
        ])
        atomic_json(output / "match.command.json", command)
        child_witness = output / "receipts/child-processes.json"
        match_stage_config = configs_dir / "match-stage.json"
        atomic_json(match_stage_config, {
            "schema": "ngn-candidate-match-stage-v1", "command": command,
            "cwd": str(output), "environment": safe_environment(),
            "roles": [
                {"id": manifest["candidate"]["id"], "engine": str(candidate_engine),
                 "engine_sha256": manifest["candidate"]["binary"]["sha256"], "cwd": str(candidate_cwd),
                 "gomaxprocs": str(width), "allowed_cpu_masks": [manifest["resources"]["cpus_allowed_list"]],
                 "launcher": str(candidate_launcher)},
                {"id": manifest["opponent"]["id"], "engine": str(opponent_engine),
                 "engine_sha256": manifest["opponent"]["binary"]["sha256"], "cwd": str(opponent_cwd),
                 "gomaxprocs": str(width), "allowed_cpu_masks": [manifest["resources"]["cpus_allowed_list"]],
                 "launcher": str(opponent_launcher)},
            ],
            "sample_interval_seconds": min(0.05, float(manifest["limits"]["sample_interval_seconds"])),
            "match_stdout": str(output / "match.stdout"), "match_stderr": str(output / "match.stderr"),
            "witness": str(child_witness),
        })
        write_state(output, "MATCH_RUNNING", command_sha256=sha256(output / "match.command.json"))
        run_stage(active, source, output, context, "match", "match_seconds",
                  [sys.executable, str(source["match_stage"]), "--config", str(match_stage_config)])
        witness = json.loads(child_witness.read_text())
        if witness.get("state") != "COMPLETE" or witness.get("violations") or witness.get("fastchess_returncode") != 0:
            raise CandidateMatchError("child process witness failed")
        (output / "match.exit").write_text("0\n")
        if (output / "match.stderr").stat().st_size:
            raise CandidateMatchError("fastchess stderr is nonempty")

        candidate_processes = len(witness["observed_instances"][manifest["candidate"]["id"]])
        opponent_processes = len(witness["observed_instances"][manifest["opponent"]["id"]])
        trace_config = configs_dir / "trace-audit.json"
        trace_output = output / "audit-operational.json"
        atomic_json(trace_config, {
            "trace": str(trace), "match_exit": str(output / "match.exit"),
            "match_stderr": str(output / "match.stderr"), "output": str(trace_output),
            "warning_policy": cell["warning_policy"],
            "candidate": {"display_name": manifest["candidate"]["display_name"], "width": width,
                          "options": candidate_options, "expected_processes": candidate_processes,
                          "expected_refreshes": 100,
                          "crash_log": {"path": str(candidate_crash_log), "logged_path": str(candidate_crash_log),
                                        "expected_processes": candidate_processes}},
            "opponent": {"display_name": manifest["opponent"]["display_name"],
                         "profile_id": manifest["opponent"]["profile_id"], "width": width,
                         "options": opponent_options, "expected_processes": opponent_processes,
                         "expected_refreshes": 100},
        })
        run_stage(active, source, output, context, "trace-audit", "trace_audit_seconds",
                  [sys.executable, str(source["trace_auditor"]), "--config", str(trace_config)])
        trace_receipt = json.loads(trace_output.read_text())
        trace_report = trace_receipt.get("trace", {})
        if (trace_receipt.get("pass") is not True
            or trace_report.get("warning_policy") != cell["warning_policy"]
            or trace_report.get("forbidden_warning_count") != 0
            or not isinstance(trace_report.get("allowed_fastchess_warnings"), list)):
            raise CandidateMatchError("external trace audit failed")

        audit_output = output / "audit.json"
        audit_command = [
            sys.executable, str(source["chess_auditor"]), "--pgn", str(pgn),
            "--final-epd", str(final_epd), "--opening-prefixes", str(frozen_prefixes),
            "--opening-prefixes-sha256", manifest["inputs"]["opening_uci"]["sha256"],
            "--opening-pgn", str(frozen_openings), "--opening-pgn-sha256", manifest["inputs"]["opening_pgn"]["sha256"],
            "--stockfish", str(frozen_stockfish), "--stockfish-sha256", manifest["inputs"]["stockfish"]["sha256"],
            "--engine-a", manifest["candidate"]["display_name"], "--engine-b", manifest["opponent"]["display_name"],
            "--games", "100", "--pairs", "50", "--output", str(audit_output),
        ]
        atomic_json(output / "audit.command.json", audit_command)
        run_stage(active, source, output, context, "chess-audit", "chess_audit_seconds", audit_command)
        audit = json.loads(audit_output.read_text())
        if audit.get("status") != "PASS" or audit.get("games") != 100 or audit.get("pairs") != 50:
            raise CandidateMatchError("independent chess audit failed")
        if audit.get("probable_embedded_book_signature_plies") != 0:
            raise CandidateMatchError("no-book roles emitted probable book signature")
        pair_scores = [item["half_points"] for item in audit["pair_audit"]]
        comparison = comparison_report(pair_scores)
        atomic_json(output / "comparison.json", comparison)

        book_receipt["post"] = {
            "candidate": [x for x in DEFAULT_BOOK_PATHS if (candidate_cwd / x).exists()],
            "opponent": [x for x in DEFAULT_BOOK_PATHS if (opponent_cwd / x).exists()],
        }
        atomic_json(output / "receipts/book-paths.json", book_receipt)
        if any(book_receipt["post"].values()):
            raise CandidateMatchError("default book path appeared")
        for path, expected_hash in post_checks:
            if sha256(path) != expected_hash:
                raise CandidateMatchError(f"post-run input mutation: {path}")

        after = known_live_processes(frozen_paths, known_hashes)
        atomic_json(output / "receipts/terminal-processes.json", {"matches": after})
        if after:
            raise CandidateMatchError(f"known terminal chess processes exist: {after}")
        atomic_json(output / "terminal.json", {
            "schema": "ngn-external-fixed-match-terminal-v1", "state": "COMPLETE",
            "ended_utc": utc_now(), "manifest_sha256": sha256(manifest_copy),
            "review_subject_sha256": review_subject_sha256(manifest),
            "games": 100, "pairs": 50, "comparison": comparison,
            "warning_policy": cell["warning_policy"],
            "allowed_fastchess_warning_count": len(trace_report["allowed_fastchess_warnings"]),
            "forbidden_warning_count": trace_report["forbidden_warning_count"],
            "operational_pass": True, "chess_audit_pass": True, "trace_audit_pass": True,
        })
        final_receipt = hash_tree(output)
        write_state(output, "COMPLETE", final_files_sha256=sha256(final_receipt))
        success = True
        print(json.dumps({"state": "COMPLETE", "output": str(output),
                          "final_files_sha256": sha256(final_receipt)}, sort_keys=True))
        return 0
    except BaseException as error:
        restore_signal_handlers(previous_handlers)
        cleanup = active.terminate()
        try:
            after = known_live_processes(frozen_paths, known_hashes)
        except Exception as process_error:
            after = [{"process_check_error": str(process_error)}]
        atomic_json(output / "receipts/terminal-processes.json", {"matches": after, "during_failure": True})
        atomic_json(output / "terminal.json", {
            "schema": "ngn-external-fixed-match-terminal-v1", "state": "FAILED",
            "ended_utc": utc_now(), "error": str(error), "cleanup": cleanup,
            "remaining_processes": after,
        })
        try:
            final_receipt = hash_tree(output)
            write_state(output, "FAILED", error=str(error), final_files_sha256=sha256(final_receipt))
        except Exception as final_error:
            write_state(output, "FAILED_EVIDENCE", error=str(error), finalization_error=str(final_error))
        print(f"external fixed match failed: {error}", file=sys.stderr)
        return 1
    finally:
        restore_signal_handlers(previous_handlers)
        if not success and active.process is not None and active.last_cleanup is None:
            active.terminate()


if __name__ == "__main__":
    raise SystemExit(main())
