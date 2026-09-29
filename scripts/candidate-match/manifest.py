#!/usr/bin/env python3
"""Strict same-code one-worker candidate match manifest validation."""
from __future__ import annotations

import hashlib
import json
import math
import re
from pathlib import Path
from typing import Any

SCHEMA = "ngn-candidate-match-v1"
SCHEMA_BORROWED = "ngn-candidate-match-borrowed-v1"
SCHEMA_BORROWED_FIXED_NODES = "ngn-candidate-match-borrowed-fixed-nodes-v1"
CAPABILITY = "ngn-pre-m4c-pinned-one-worker-v1"
OPTION_NAMES = ["Threads", "OwnBook", "EvalFile", "EvalBackend", "Hash", "Move Overhead"]
NETWORK_BACKENDS = {"ngn-v1", "ngn-k4-768-v1", "rodent-v1.1-anand"}
HELD_APPROVAL = {
    "authority": None,
    "token": None,
    "exclusive_match_window": False,
    "reviewed_manifest_sha256": None,
}
SHA_RE = re.compile(r"^[0-9a-f]{64}$")
ROLE_RE = re.compile(r"^[A-Za-z0-9_]{1,32}$")


class ManifestError(ValueError):
    pass


def review_subject_sha256(value: dict[str, Any]) -> str:
    """Hash every run field while excluding the later root approval decision."""
    subject = dict(value)
    subject["status"] = "HELD"
    subject["approval"] = dict(HELD_APPROVAL)
    encoded = json.dumps(subject, sort_keys=True, separators=(",", ":"), ensure_ascii=True).encode()
    return hashlib.sha256(encoded).hexdigest()


def obj(value: Any, location: str) -> dict[str, Any]:
    if not isinstance(value, dict):
        raise ManifestError(f"{location}: expected object")
    return value


def exact_keys(value: dict[str, Any], required: set[str], optional: set[str], location: str) -> None:
    missing = required - value.keys()
    unknown = value.keys() - required - optional
    if missing or unknown:
        raise ManifestError(f"{location}: missing={sorted(missing)} unknown={sorted(unknown)}")


def integer(value: Any, location: str, minimum: int = 0) -> int:
    if isinstance(value, bool) or not isinstance(value, int) or value < minimum:
        raise ManifestError(f"{location}: expected integer >= {minimum}")
    return value


def text(value: Any, location: str) -> str:
    if not isinstance(value, str) or not value:
        raise ManifestError(f"{location}: expected nonempty string")
    if "\x00" in value or "\n" in value or "\r" in value:
        raise ManifestError(f"{location}: control character")
    return value


def boolean(value: Any, location: str) -> bool:
    if not isinstance(value, bool):
        raise ManifestError(f"{location}: expected boolean")
    return value


def file_spec(value: Any, location: str, provenance: bool = False) -> dict[str, Any]:
    value = obj(value, location)
    optional = {"provenance"} if provenance else set()
    exact_keys(value, {"path", "sha256"}, optional, location)
    text(value["path"], f"{location}.path")
    if not isinstance(value["sha256"], str) or not SHA_RE.fullmatch(value["sha256"]):
        raise ManifestError(f"{location}.sha256: expected lowercase SHA-256")
    if provenance:
        p = obj(value.get("provenance"), f"{location}.provenance")
        exact_keys(p, {"source_commit", "source_tree", "build_receipt", "build_command"}, set(), f"{location}.provenance")
        for name in ("source_commit", "source_tree"):
            text(p[name], f"{location}.provenance.{name}")
        file_spec(p["build_receipt"], f"{location}.provenance.build_receipt")
        if not isinstance(p["build_command"], list) or not p["build_command"]:
            raise ManifestError(f"{location}.provenance.build_command: expected nonempty argv")
        for index, arg in enumerate(p["build_command"]):
            text(arg, f"{location}.provenance.build_command[{index}]")
    return value


def role(value: Any, index: int) -> dict[str, Any]:
    location = f"inputs.roles[{index}]"
    value = obj(value, location)
    exact_keys(
        value,
        {"id", "display_name", "binary", "network", "capability_profile", "backend", "uci_options", "requested", "effective", "environment"},
        set(),
        location,
    )
    role_id = text(value["id"], f"{location}.id")
    if not ROLE_RE.fullmatch(role_id):
        raise ManifestError(f"{location}.id: unsafe role identifier")
    text(value["display_name"], f"{location}.display_name")
    file_spec(value["binary"], f"{location}.binary", provenance=True)
    if value["network"] is not None:
        network = obj(value["network"], f"{location}.network")
        exact_keys(network, {"path", "sha256", "format"}, set(), f"{location}.network")
        file_spec({"path": network["path"], "sha256": network["sha256"]}, f"{location}.network.file")
        if network["format"] not in NETWORK_BACKENDS:
            raise ManifestError(f"{location}.network.format: unsupported exact NGN format")
    if value["capability_profile"] != CAPABILITY:
        raise ManifestError(f"{location}.capability_profile: unsupported by v1")
    if value["backend"] not in {"hce", *NETWORK_BACKENDS}:
        raise ManifestError(f"{location}.backend: expected hce or an exact NGN backend")
    if value["backend"] in NETWORK_BACKENDS and value["network"] is None:
        raise ManifestError(f"{location}: network backend requires network")
    if value["backend"] == "hce" and value["network"] is not None:
        raise ManifestError(f"{location}: hce forbids network")
    if value["network"] is not None and value["network"]["format"] != value["backend"]:
        raise ManifestError(f"{location}: network format must equal backend")

    options = value["uci_options"]
    if not isinstance(options, list) or len(options) != len(OPTION_NAMES):
        raise ManifestError(f"{location}.uci_options: expected exactly {OPTION_NAMES}")
    parsed: dict[str, Any] = {}
    for option_index, option in enumerate(options):
        option = obj(option, f"{location}.uci_options[{option_index}]")
        exact_keys(option, {"name", "value"}, set(), f"{location}.uci_options[{option_index}]")
        name = text(option["name"], f"{location}.uci_options[{option_index}].name")
        parsed[name] = option["value"]
    if [option["name"] for option in options] != OPTION_NAMES:
        raise ManifestError(f"{location}.uci_options: canonical order is {OPTION_NAMES}")
    expected_file = "$FROZEN_NETWORK" if value["backend"] in NETWORK_BACKENDS else "<empty>"
    if parsed != {
        "Threads": 1,
        "OwnBook": False,
        "EvalFile": expected_file,
        "EvalBackend": value["backend"],
        "Hash": parsed.get("Hash"),
        "Move Overhead": parsed.get("Move Overhead"),
    }:
        raise ManifestError(f"{location}.uci_options: backend/thread/file mismatch")

    requested = obj(value["requested"], f"{location}.requested")
    exact_keys(requested, {"Threads", "OwnBook", "Hash", "Move Overhead"}, set(), f"{location}.requested")
    requested_threads = integer(requested["Threads"], f"{location}.requested.Threads", 1)
    option_threads = integer(parsed["Threads"], f"{location}.uci_options.Threads", 1)
    if requested_threads != 1 or option_threads != 1:
        raise ManifestError(f"{location}: v1 supports requested Threads=1 only")
    if requested["OwnBook"] is not False or parsed["OwnBook"] is not False:
        raise ManifestError(f"{location}: candidate matches require OwnBook=false")
    hash_mb = integer(requested["Hash"], f"{location}.requested.Hash", 1)
    overhead = integer(requested["Move Overhead"], f"{location}.requested.Move Overhead", 0)
    if hash_mb > 1024 or overhead > 5000 or parsed["Hash"] != hash_mb or parsed["Move Overhead"] != overhead:
        raise ManifestError(f"{location}: requested and option values differ or exceed advertised bounds")

    effective = obj(value["effective"], f"{location}.effective")
    exact_keys(effective, {"Threads", "GOMAXPROCS", "OwnBook", "Hash", "Move Overhead", "threads_basis"}, set(), f"{location}.effective")
    integer(effective["Threads"], f"{location}.effective.Threads", 1)
    integer(effective["GOMAXPROCS"], f"{location}.effective.GOMAXPROCS", 1)
    integer(effective["Hash"], f"{location}.effective.Hash", 1)
    integer(effective["Move Overhead"], f"{location}.effective.Move Overhead", 0)
    boolean(effective["OwnBook"], f"{location}.effective.OwnBook")
    if effective != {
        "Threads": 1,
        "GOMAXPROCS": 1,
        "OwnBook": False,
        "Hash": hash_mb,
        "Move Overhead": overhead,
        "threads_basis": "pinned-source-single-worker-plus-one-cpu-mask",
    }:
        raise ManifestError(f"{location}.effective: unsupported or inconsistent one-worker claim")
    environment = obj(value["environment"], f"{location}.environment")
    exact_keys(environment, {"GOMAXPROCS"}, set(), f"{location}.environment")
    if environment["GOMAXPROCS"] != "1":
        raise ManifestError(f"{location}.environment.GOMAXPROCS: v1 requires string 1")
    return value


def validate_manifest(value: Any) -> dict[str, Any]:
    value = obj(value, "manifest")
    exact_keys(value, {"schema", "status", "purpose", "classification", "approval", "inputs", "match", "resources", "limits", "operational_profile", "acceptance"}, set(), "manifest")
    if value["schema"] not in {SCHEMA, SCHEMA_BORROWED, SCHEMA_BORROWED_FIXED_NODES}:
        raise ManifestError("manifest.schema: unsupported candidate-match contract")
    if value["status"] not in {"HELD", "APPROVED_TO_RUN"}:
        raise ManifestError("manifest.status: expected HELD or APPROVED_TO_RUN")
    text(value["purpose"], "manifest.purpose")
    if value["classification"] != "diagnostic":
        raise ManifestError("manifest.classification: v1 is diagnostic only")

    approval = obj(value["approval"], "approval")
    exact_keys(approval, {"authority", "token", "exclusive_match_window", "reviewed_manifest_sha256"}, set(), "approval")
    if value["status"] == "APPROVED_TO_RUN":
        if approval["authority"] != "root" or not approval["exclusive_match_window"]:
            raise ManifestError("approval: root exclusive approval required")
        text(approval["token"], "approval.token")
        reviewed = approval["reviewed_manifest_sha256"]
        if not isinstance(reviewed, str) or not SHA_RE.fullmatch(reviewed):
            raise ManifestError("approval.reviewed_manifest_sha256: lowercase SHA-256 required")
        if reviewed != review_subject_sha256(value):
            raise ManifestError("approval.reviewed_manifest_sha256: does not bind the normalized HELD review subject")
    else:
        if approval != HELD_APPROVAL:
            raise ManifestError("approval: HELD manifest must have null authority/token/hash and false exclusive flag")

    inputs = obj(value["inputs"], "inputs")
    exact_keys(inputs, {"runner", "schema", "common", "manifest_module", "supervisor", "uci_preflight", "match_stage", "trace_auditor", "role_exec", "chess_auditor", "fastchess", "stockfish", "opening_pgn", "opening_prefixes", "roles"}, set(), "inputs")
    for name in ("runner", "schema", "common", "manifest_module", "supervisor", "uci_preflight", "match_stage", "trace_auditor", "role_exec", "chess_auditor", "fastchess", "stockfish", "opening_pgn", "opening_prefixes"):
        file_spec(inputs[name], f"inputs.{name}")
    roles = inputs["roles"]
    if not isinstance(roles, list) or len(roles) != 2:
        raise ManifestError("inputs.roles: exactly two roles required")
    roles = [role(item, index) for index, item in enumerate(roles)]
    if len({item["id"] for item in roles}) != 2 or len({item["display_name"] for item in roles}) != 2:
        raise ManifestError("inputs.roles: IDs and display names must be unique")
    backends = {item["backend"] for item in roles}
    if value["schema"] == SCHEMA:
        if "hce" not in backends or len(backends) != 2 or not (backends - {"hce"}).issubset({"ngn-v1", "ngn-k4-768-v1"}):
            raise ManifestError("inputs.roles: v1 requires one HCE and one exact NGN-network role")
    elif backends != {"ngn-k4-768-v1", "rodent-v1.1-anand"}:
        raise ManifestError("inputs.roles: borrowed schema requires owned K4 and Rodent V1.1 Anand")
    binary_identities = {(item["binary"]["sha256"], item["binary"]["provenance"]["source_commit"], item["binary"]["provenance"]["source_tree"]) for item in roles}
    if len(binary_identities) != 1:
        raise ManifestError("inputs.roles: v1 requires same-code binary/source identities")

    match = obj(value["match"], "match")
    fixed_nodes = value["schema"] == SCHEMA_BORROWED_FIXED_NODES
    budget_key = "node_limit" if fixed_nodes else "time_control"
    exact_keys(match, {"games", "pairs", budget_key, "concurrency", "seed", "opening_plies", "use_affinity", "affinity_cpus", "runner_strict", "recover", "ponder", "book", "tablebase", "score_adjudication", "resign_adjudication", "draw_adjudication", "maxmoves_adjudication"}, set(), "match")
    games = integer(match["games"], "match.games", 2)
    pairs = integer(match["pairs"], "match.pairs", 1)
    if games != 2 * pairs:
        raise ManifestError("match: games must equal 2*pairs")
    if fixed_nodes:
        integer(match["node_limit"], "match.node_limit", 1)
    else:
        text(match["time_control"], "match.time_control")
    concurrency = integer(match["concurrency"], "match.concurrency", 1)
    integer(match["seed"], "match.seed", 0)
    integer(match["opening_plies"], "match.opening_plies", 0)
    if match["use_affinity"] is not True:
        raise ManifestError("match.use_affinity: one-worker v1 requires fastchess affinity")
    affinity = match["affinity_cpus"]
    if not isinstance(affinity, list) or len(affinity) < concurrency or len(set(affinity)) != len(affinity):
        raise ManifestError("match.affinity_cpus: distinct CPU list must cover concurrency")
    for index, cpu in enumerate(affinity): integer(cpu, f"match.affinity_cpus[{index}]", 0)
    for name in ("runner_strict",):
        if boolean(match[name], f"match.{name}") is not True: raise ManifestError(f"match.{name}: must be true")
    for name in ("recover", "ponder", "book", "tablebase", "score_adjudication", "resign_adjudication", "draw_adjudication", "maxmoves_adjudication"):
        if boolean(match[name], f"match.{name}") is not False: raise ManifestError(f"match.{name}: must be false")

    resources = obj(value["resources"], "resources")
    exact_keys(resources, {"runner_cpus", "required_cpus_allowed_list", "memory_limit_kib", "affinity_mode"}, set(), "resources")
    runner_cpus = resources["runner_cpus"]
    if not isinstance(runner_cpus, list) or len(set(runner_cpus)) != len(runner_cpus): raise ManifestError("resources.runner_cpus: distinct list required")
    for index, cpu in enumerate(runner_cpus): integer(cpu, f"resources.runner_cpus[{index}]", 0)
    if not set(affinity).issubset(runner_cpus): raise ManifestError("resources: affinity CPUs outside runner CPUs")
    text(resources["required_cpus_allowed_list"], "resources.required_cpus_allowed_list")
    integer(resources["memory_limit_kib"], "resources.memory_limit_kib", 1)
    if resources["affinity_mode"] != "fastchess-per-game-one-worker": raise ManifestError("resources.affinity_mode: unsupported by v1")

    limits = obj(value["limits"], "limits")
    exact_keys(limits, {"preflight_seconds", "match_seconds", "trace_audit_seconds", "chess_audit_seconds", "term_grace_seconds", "sample_interval_seconds"}, set(), "limits")
    for name, limit in limits.items():
        if isinstance(limit, bool) or not isinstance(limit, (int, float)) or not math.isfinite(limit) or limit <= 0:
            raise ManifestError(f"limits.{name}: finite positive number required")

    profile = obj(value["operational_profile"], "operational_profile")
    exact_keys(profile, {"name", "allow_fastchess_normal_exit", "ngn_crash_handler"}, set(), "operational_profile")
    if profile != {"name": "ngn-pre-m4c-known-benign-v1", "allow_fastchess_normal_exit": True, "ngn_crash_handler": "required-two-line-init-per-observed-process"}:
        raise ManifestError("operational_profile: unsupported exact profile")

    acceptance = obj(value["acceptance"], "acceptance")
    exact_keys(acceptance, {"expected_games", "expected_pairs", "require_independent_audit", "require_zero_operational_errors", "require_zero_survivors", "strength_claim"}, set(), "acceptance")
    if acceptance != {"expected_games": games, "expected_pairs": pairs, "require_independent_audit": True, "require_zero_operational_errors": True, "require_zero_survivors": True, "strength_claim": "none"}:
        raise ManifestError("acceptance: must bind game counts and diagnostic-only gates")
    return value


def load_manifest(path: Path) -> dict[str, Any]:
    try:
        value = json.loads(path.read_text(encoding="utf-8"))
    except Exception as error:
        raise ManifestError(f"cannot parse {path}: {error}") from error
    return validate_manifest(value)
