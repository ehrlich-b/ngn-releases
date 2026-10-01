#!/usr/bin/env python3
"""Exact manifest contract for fixed NGN SF18 BIG versus pinned Counter 5.5."""
from __future__ import annotations

import hashlib
import json
import math
import re
from pathlib import Path
from typing import Any

from common import CandidateMatchError, sha256
from external_profiles import (
    COUNTER55, EIGHT_PHYSICAL_CPUS, ONE_CPU, profile_for, resolved_options,
    validate_advertised_options,
)

SCHEMA = "ngn-external-fixed-match-v1"
OPENING_SELECTION_RECEIPT_SHA256 = "b289ac730134a2da90f0e71655ed1ccc2c85f22b3673efa7d8df39d46f2301a8"
OPENING_PGN_SHA256 = "b1d5d8503c2ee9d9c82c6df73138041f0a093109c44926dcaf10f58545c1b377"
OPENING_UCI_SHA256 = "05da45f7653ae0682133af7e813ab08f9f50a7677b0a28b921931ead4b28c9f4"
OPENING_ROOT_REVIEW_SHA256 = "9f6dddebb61d92f497d8ab99feecd362ddf5ee791d72fa4a88380153132f6335"
SF18BIG_MODEL_SHA256 = "c288c895ea924429ea9092e3f36b2b3c1f00f2a3a4c759ff7e57e79e3b43e4a7"
SHA_RE = re.compile(r"^[0-9a-f]{64}$")
ROLE_RE = re.compile(r"^[A-Za-z0-9_]{1,32}$")
HELD_APPROVAL = {"authority": None, "token": None, "exclusive_match_window": False, "reviewed_manifest_sha256": None}
COUNTER55_WARNING_POLICY_ID = "counter55-pv-after-fifty-v1"
REJECT_ALL_WARNING_POLICY_ID = "reject-all-fastchess-warnings-v1"


def warning_policy_for(profile_id: str) -> dict[str, Any]:
    if profile_id == COUNTER55.id:
        return {
            "id": COUNTER55_WARNING_POLICY_ID,
            "fastchess_strict": False,
            "allowed_fastchess_warnings": [{
                "kind": "pv-continues-after-fifty-move-rule",
                "level": "WARN",
                "actor": "fastchess",
                "role": "opponent",
                "profile_id": COUNTER55.id,
                "display_name": COUNTER55.display_name,
                "move_format": "uci-coordinate-lowercase-v1",
            }],
        }
    return {
        "id": REJECT_ALL_WARNING_POLICY_ID,
        "fastchess_strict": True,
        "allowed_fastchess_warnings": [],
    }


def exact_keys(value: dict[str, Any], names: set[str], where: str) -> None:
    if set(value) != names:
        raise CandidateMatchError(f"{where}: missing={sorted(names - value.keys())} unknown={sorted(value.keys() - names)}")


def obj(value: Any, where: str) -> dict[str, Any]:
    if not isinstance(value, dict):
        raise CandidateMatchError(f"{where}: expected object")
    return value


def integer(value: Any, where: str, low: int = 0) -> int:
    if isinstance(value, bool) or not isinstance(value, int) or value < low:
        raise CandidateMatchError(f"{where}: expected integer >= {low}")
    return value


def finite(value: Any, where: str, low: float = 0.0) -> float:
    if isinstance(value, bool) or not isinstance(value, (int, float)) or not math.isfinite(value) or value <= low:
        raise CandidateMatchError(f"{where}: expected finite number > {low}")
    return float(value)


def file_spec(value: Any, where: str) -> dict[str, str]:
    value = obj(value, where)
    exact_keys(value, {"path", "sha256"}, where)
    if not isinstance(value["path"], str) or not value["path"] or "\0" in value["path"]:
        raise CandidateMatchError(f"{where}.path: invalid")
    if not isinstance(value["sha256"], str) or SHA_RE.fullmatch(value["sha256"]) is None:
        raise CandidateMatchError(f"{where}.sha256: invalid")
    return value


def verify_file(spec: dict[str, str], where: str) -> Path:
    path = Path(spec["path"]).resolve(strict=True)
    if not path.is_file() or sha256(path) != spec["sha256"]:
        raise CandidateMatchError(f"{where}: file identity mismatch")
    return path


def review_subject_sha256(value: dict[str, Any]) -> str:
    subject = dict(value)
    subject["status"] = "HELD"
    subject["approval"] = dict(HELD_APPROVAL)
    encoded = json.dumps(subject, sort_keys=True, separators=(",", ":"), ensure_ascii=True).encode()
    return hashlib.sha256(encoded).hexdigest()


def validate_admission(profile_id: str, width: int, receipt_spec: dict[str, str], transcript_spec: dict[str, str]) -> dict[str, Any]:
    if isinstance(width, bool) or width not in (1, 8):
        raise CandidateMatchError("external admission width must be exact integer 1 or 8")
    receipt_path = verify_file(receipt_spec, "opponent.admission_receipt")
    verify_file(transcript_spec, "opponent.admission_transcript")
    receipt = json.loads(receipt_path.read_text(encoding="utf-8"))
    required = {
        "schema", "state", "started_utc", "ended_utc", "elapsed_seconds", "pass",
        "profile", "width", "cpus_allowed_list", "artifact", "identity", "option_domain",
        "barriers", "book_control", "timing_control", "depth_search", "stop_search",
        "multicore_runtime_witness", "transcript_sha256", "stderr", "trailing_events",
        "reader_threads_alive_after_cleanup", "process_returncode",
    }
    exact_keys(receipt, required, "external admission receipt")
    expected_mask = ",".join(str(cpu) for cpu in (ONE_CPU if width == 1 else EIGHT_PHYSICAL_CPUS))
    if (
        receipt["schema"] != "ngn-external-uci-admission-receipt-v1"
        or receipt["state"] != "COMPLETE"
        or receipt["pass"] is not True
        or isinstance(receipt["width"], bool)
        or receipt["width"] != width
        or receipt["profile"] != profile_id
        or receipt["cpus_allowed_list"] != expected_mask
        or receipt["transcript_sha256"] != transcript_spec["sha256"]
        or isinstance(receipt["process_returncode"], bool)
        or receipt["process_returncode"] != 0
        or receipt["reader_threads_alive_after_cleanup"] != []
    ):
        raise CandidateMatchError("external admission state/identity/width/mask/process differs")
    profile = profile_for(profile_id)
    artifact = obj(receipt["artifact"], "admission.artifact")
    exact_keys(artifact, {"profile", "binary_sha256", "binary_size", "identity_receipt_sha256"}, "admission.artifact")
    if (
        artifact["profile"] != profile.id
        or artifact["binary_sha256"] != profile.binary_sha256
        or integer(artifact["binary_size"], "admission.artifact.binary_size", 1) != profile.binary_size
        or artifact["identity_receipt_sha256"] != profile.identity_receipt_sha256
    ):
        raise CandidateMatchError("external admission artifact differs")
    identity = obj(receipt["identity"], "admission.identity")
    if identity != {
        "name": profile.uci_name,
        "author": profile.uci_author,
        "loaded_model_line": profile.loaded_model_line,
    }:
        raise CandidateMatchError("external admission UCI/model identity differs")
    domain = obj(receipt["option_domain"], "admission.option_domain")
    exact_keys(domain, {"advertised", "requested", "thread_basis"}, "admission.option_domain")
    if domain["requested"] != resolved_options(profile, width) or domain["thread_basis"] != profile.thread_basis:
        raise CandidateMatchError("external admission option request/basis differs")
    if not isinstance(domain["advertised"], dict):
        raise CandidateMatchError("external admission advertised options invalid")
    if validate_advertised_options(profile, domain["advertised"], width) != domain:
        raise CandidateMatchError("external admission advertised option domains differ")
    barriers = receipt["barriers"]
    if not isinstance(barriers, list) or len(barriers) != len(resolved_options(profile, width)):
        raise CandidateMatchError("external admission barrier count differs")
    for actual, requested in zip(barriers, resolved_options(profile, width)):
        actual = obj(actual, "admission.barrier")
        exact_keys(actual, {"option", "value", "lines"}, "admission.barrier")
        if (actual["option"] != requested["name"] or actual["value"] != requested["value"]
            or not isinstance(actual["lines"], list) or not all(isinstance(line, str) for line in actual["lines"])):
            raise CandidateMatchError("external admission barrier differs")
    book = obj(receipt["book_control"], "admission.book_control")
    exact_keys(book, {"ownbook_option_advertised", "engine_cwd_empty_pre_and_post", "depth_search_nodes_gt_one"}, "admission.book_control")
    if book["ownbook_option_advertised"] is not False or book["engine_cwd_empty_pre_and_post"] is not True or book["depth_search_nodes_gt_one"] is not True:
        raise CandidateMatchError("external admission book/cwd/search claims differ")
    for name in ("depth_search", "stop_search"):
        search = obj(receipt[name], f"admission.{name}")
        exact_keys(search, {"bestmove", "max_reported_nodes", "lines"}, f"admission.{name}")
        if not isinstance(search["lines"], list) or not all(isinstance(line, str) for line in search["lines"]):
            raise CandidateMatchError(f"external admission {name} lines invalid")
        if isinstance(search.get("max_reported_nodes"), bool) or not isinstance(search.get("max_reported_nodes"), int) or search["max_reported_nodes"] <= 1:
            raise CandidateMatchError(f"external admission {name} lacks nodes>1")
        if re.fullmatch(r"[a-h][1-8][a-h][1-8][nbrq]?", str(search.get("bestmove"))) is None:
            raise CandidateMatchError(f"external admission {name} bestmove invalid")
    timing = obj(receipt["timing_control"], "admission.timing_control")
    exact_keys(timing, {"command", "elapsed_seconds", "search"}, "admission.timing_control")
    if timing["command"] != "go wtime 2000 btime 2000 winc 200 binc 200 movestogo 16":
        raise CandidateMatchError("external admission timed-go command differs")
    finite(timing["elapsed_seconds"], "admission.timing_control.elapsed_seconds")
    timed_search = obj(timing["search"], "admission.timing_control.search")
    exact_keys(timed_search, {"bestmove", "max_reported_nodes", "lines"}, "admission.timing_control.search")
    if (not isinstance(timed_search["lines"], list) or not all(isinstance(line, str) for line in timed_search["lines"])
        or re.fullmatch(r"[a-h][1-8][a-h][1-8][nbrq]?", str(timed_search.get("bestmove"))) is None):
        raise CandidateMatchError("external admission timed search invalid")
    if isinstance(timed_search.get("max_reported_nodes"), bool) or not isinstance(timed_search.get("max_reported_nodes"), int) or timed_search["max_reported_nodes"] <= 1:
        raise CandidateMatchError("external admission timed search lacks nodes>1")
    if not isinstance(receipt["stderr"], list):
        raise CandidateMatchError("external admission stderr receipt invalid")
    from external_profiles import validate_stderr
    validate_stderr(profile, receipt["stderr"], width)
    witness = obj(receipt["multicore_runtime_witness"], "admission.multicore_runtime_witness")
    exact_claim = (
        "observed multicore activity at requested Threads=8 under exact full mask; not exact active-worker count"
        if width == 8 else
        "observed single-core activity at requested Threads=1 under exact one-CPU mask"
    )
    exact_keys(witness, {"claim", "wall_seconds", "process_cpu_seconds",
        "cpu_seconds_per_wall_second", "measured_core_equivalents", "minimum_wall_seconds",
        "minimum_ratio_for_width_8", "before", "after"}, "admission.multicore_runtime_witness")
    if witness["claim"] != exact_claim:
        raise CandidateMatchError("external admission multicore claim differs")
    wall = finite(witness["wall_seconds"], "admission.multicore_runtime_witness.wall_seconds")
    cpu_seconds = witness["process_cpu_seconds"]
    ratio = witness["cpu_seconds_per_wall_second"]
    measured = witness["measured_core_equivalents"]
    numeric = (cpu_seconds, ratio, measured)
    if (any(isinstance(item, bool) or not isinstance(item, (int, float)) or not math.isfinite(item) or item < 0 for item in numeric)
        or float(measured) != float(ratio) or witness["minimum_wall_seconds"] != 2.0
        or witness["minimum_ratio_for_width_8"] != 1.5 or wall < 2.0
        or abs(float(cpu_seconds) / wall - float(ratio)) > 1e-9 * max(1.0, float(ratio))):
        raise CandidateMatchError("external admission multicore timing/equivalent differs")
    trailing = receipt["trailing_events"]
    if not isinstance(trailing, list):
        raise CandidateMatchError("external admission trailing event list invalid")
    for event in trailing:
        event = obj(event, "external admission trailing event")
        exact_keys(event, {"stream", "line", "monotonic"}, "external admission trailing event")
        if event["stream"] not in {"stdout", "stderr"} or not isinstance(event["line"], str):
            raise CandidateMatchError("external admission trailing event invalid")
        finite(event["monotonic"], "external admission trailing event timestamp")
    observed_rows = {}
    for label in ("before", "after"):
        observed = obj(witness.get(label), f"admission.multicore_runtime_witness.{label}")
        exact_keys(observed, {"pid", "ppid", "pgid", "exe", "cwd", "cmdline", "cpus_allowed_list", "gomaxprocs", "task_count"}, f"admission.multicore_runtime_witness.{label}")
        for number in ("pid", "ppid", "pgid"):
            integer(observed[number], f"admission.multicore_runtime_witness.{label}.{number}")
        if (not isinstance(observed["exe"], str) or not observed["exe"]
            or not isinstance(observed["cwd"], str) or not observed["cwd"]
            or not isinstance(observed["cmdline"], list) or not all(isinstance(item, str) for item in observed["cmdline"])):
            raise CandidateMatchError(f"external admission {label} process identity invalid")
        if observed.get("cpus_allowed_list") != expected_mask or observed.get("gomaxprocs") != str(width):
            raise CandidateMatchError(f"external admission {label} mask/GOMAX differs")
        tasks = observed.get("task_count")
        if isinstance(tasks, bool) or not isinstance(tasks, int) or tasks < 1:
            raise CandidateMatchError(f"external admission {label} task count invalid")
        observed_rows[label] = observed
    for field in ("pid", "ppid", "pgid", "exe", "cwd", "cmdline"):
        if observed_rows["before"][field] != observed_rows["after"][field]:
            raise CandidateMatchError(f"external admission process identity changed across witness: {field}")
    if width == 8 and float(ratio) < 1.5:
        raise CandidateMatchError("external admission lacks observed multicore activity")
    return receipt


def validate_manifest(value: Any, verify_files: bool = True) -> dict[str, Any]:
    value = obj(value, "manifest")
    exact_keys(value, {"schema", "status", "purpose", "approval", "inputs", "candidate", "opponent", "cell", "resources", "limits", "acceptance"}, "manifest")
    if value["schema"] != SCHEMA or value["status"] not in {"HELD", "APPROVED_TO_RUN"}:
        raise CandidateMatchError("unsupported schema/status")
    approval = obj(value["approval"], "approval")
    exact_keys(approval, set(HELD_APPROVAL), "approval")
    if value["status"] == "HELD":
        if approval != HELD_APPROVAL:
            raise CandidateMatchError("HELD approval must be empty")
    elif (approval.get("authority") != "root" or approval.get("exclusive_match_window") is not True
          or not isinstance(approval.get("token"), str) or not approval["token"] or "\0" in approval["token"]
          or approval.get("reviewed_manifest_sha256") != review_subject_sha256(value)):
        raise CandidateMatchError("approved manifest lacks exact root subject binding")

    inputs = obj(value["inputs"], "inputs")
    input_names = {"runner", "runner_helpers", "manifest_module", "legacy_manifest", "common", "supervisor", "role_exec", "uci_preflight",
        "external_profiles", "external_admission", "external_report", "match_stage",
        "trace_auditor", "trace_helpers", "chess_auditor",
        "fastchess", "stockfish", "opening_pgn", "opening_uci", "opening_selection_receipt",
        "opening_history_audit", "opening_stockfish_admission", "opening_root_review"}
    exact_keys(inputs, input_names, "inputs")
    for name in input_names:
        file_spec(inputs[name], f"inputs.{name}")
    fixed = {"opening_pgn": OPENING_PGN_SHA256, "opening_uci": OPENING_UCI_SHA256,
             "opening_selection_receipt": OPENING_SELECTION_RECEIPT_SHA256,
             "opening_root_review": OPENING_ROOT_REVIEW_SHA256}
    for name, expected in fixed.items():
        if inputs[name]["sha256"] != expected:
            raise CandidateMatchError(f"inputs.{name}: not accepted opening artifact")

    cell = obj(value["cell"], "cell")
    exact_keys(cell, {"id", "width", "games", "pairs", "time_control", "opening_plies", "concurrency",
        "hash_mib_per_engine", "move_overhead_ms", "ponder", "book", "tablebase",
        "adjudication", "recover", "strict", "warning_policy"}, "cell")
    width = integer(cell["width"], "cell.width", 1)
    if (width != 1 or integer(cell["games"], "cell.games", 1) != 100
        or integer(cell["pairs"], "cell.pairs", 1) != 50
        or integer(cell["opening_plies"], "cell.opening_plies", 1) != 16):
        raise CandidateMatchError("SF18 BIG anchor cell must use width 1 and 100 games/50 pairs/16 plies")

    candidate = obj(value["candidate"], "candidate")
    exact_keys(candidate, {"id", "display_name", "binary", "build_receipt", "model", "options"}, "candidate")
    if not isinstance(candidate["id"], str) or ROLE_RE.fullmatch(candidate["id"]) is None:
        raise CandidateMatchError("candidate.id invalid")
    if not isinstance(candidate["display_name"], str) or not candidate["display_name"].strip() or "\0" in candidate["display_name"]:
        raise CandidateMatchError("candidate.display_name invalid")
    for name in ("binary", "build_receipt", "model"):
        file_spec(candidate[name], f"candidate.{name}")
    if candidate["model"]["sha256"] != SF18BIG_MODEL_SHA256:
        raise CandidateMatchError("candidate.model is not the admitted SF18 BIG model")
    expected_candidate_options = [
        {"name": "Threads", "value": width}, {"name": "OwnBook", "value": False},
        {"name": "EvalFile", "value": "$FROZEN_NETWORK"}, {"name": "EvalBackend", "value": "sf18-big"},
        {"name": "Hash", "value": 128}, {"name": "Move Overhead", "value": 100},
    ]
    if candidate["options"] != expected_candidate_options:
        raise CandidateMatchError("candidate.options differs")

    opponent = obj(value["opponent"], "opponent")
    exact_keys(opponent, {"id", "display_name", "profile_id", "binary", "identity_receipt",
        "admission_receipt", "admission_transcript", "options"}, "opponent")
    if not isinstance(opponent["id"], str) or ROLE_RE.fullmatch(opponent["id"]) is None:
        raise CandidateMatchError("opponent.id invalid")
    profile = profile_for(opponent["profile_id"])
    if opponent["display_name"] != profile.display_name:
        raise CandidateMatchError("opponent display name differs")
    if opponent["id"] == candidate["id"] or opponent["display_name"] == candidate["display_name"]:
        raise CandidateMatchError("candidate and opponent identities must differ")
    for name in ("binary", "identity_receipt", "admission_receipt", "admission_transcript"):
        file_spec(opponent[name], f"opponent.{name}")
    if opponent["binary"]["sha256"] != profile.binary_sha256 or opponent["identity_receipt"]["sha256"] != profile.identity_receipt_sha256:
        raise CandidateMatchError("opponent artifact identity differs")
    if opponent["options"] != resolved_options(profile, width):
        raise CandidateMatchError("opponent options differ")

    allowed_tc = {"30+0.3", "60+0.6"} if profile.id == "rodent-v1.2-nontal-testers" else {"30+0.3"}
    if cell["time_control"] not in allowed_tc:
        raise CandidateMatchError("cell time control outside schedule")
    if (integer(cell["concurrency"], "cell.concurrency", 1) != 1
        or integer(cell["hash_mib_per_engine"], "cell.hash_mib_per_engine", 1) != 128
        or integer(cell["move_overhead_ms"], "cell.move_overhead_ms", 0) != 100):
        raise CandidateMatchError("cell execution values differ")
    expected_warning_policy = warning_policy_for(profile.id)
    if (any(cell[name] is not False for name in ("ponder", "book", "tablebase", "adjudication", "recover"))
        or cell["strict"] is not expected_warning_policy["fastchess_strict"]
        or cell["warning_policy"] != expected_warning_policy):
        raise CandidateMatchError("cell safety/warning controls differ")

    resources = obj(value["resources"], "resources")
    exact_keys(resources, {"cpus", "cpus_allowed_list", "gomaxprocs", "fastchess_use_affinity", "memory_limit_kib"}, "resources")
    cpus = list(ONE_CPU if width == 1 else EIGHT_PHYSICAL_CPUS)
    if (resources["cpus"] != cpus or resources["cpus_allowed_list"] != ",".join(map(str, cpus))
        or integer(resources["gomaxprocs"], "resources.gomaxprocs", 1) != width
        or resources["fastchess_use_affinity"] is not False
        or integer(resources["memory_limit_kib"], "resources.memory_limit_kib", 1) != 12 * 1024 * 1024):
        raise CandidateMatchError("resource contract differs")

    limits = obj(value["limits"], "limits")
    exact_keys(limits, {"preflight_seconds", "match_seconds", "audit_seconds", "term_grace_seconds", "sample_interval_seconds"}, "limits")
    for name, limit in limits.items():
        finite(limit, f"limits.{name}")
    expected_acceptance = {"fixed_games_no_early_stop": True, "require_complete_pairs": True,
        "require_external_admission": True, "require_independent_chess_audit": True,
        "require_trace_audit": True, "require_zero_operational_errors": True,
        "require_zero_survivors": True,
        "interpretation": "matched-local-external-comparison-not-world-best-claim"}
    if value["acceptance"] != expected_acceptance:
        raise CandidateMatchError("acceptance contract differs")

    if verify_files:
        for name in input_names:
            verify_file(inputs[name], f"inputs.{name}")
        for name in ("binary", "build_receipt", "model"):
            verify_file(candidate[name], f"candidate.{name}")
        for name in ("binary", "identity_receipt", "admission_receipt", "admission_transcript"):
            verify_file(opponent[name], f"opponent.{name}")
        validate_admission(profile.id, width, opponent["admission_receipt"], opponent["admission_transcript"])
    return value


def load_manifest(path: Path, verify_files: bool = True) -> dict[str, Any]:
    return validate_manifest(json.loads(path.read_text(encoding="utf-8")), verify_files=verify_files)
