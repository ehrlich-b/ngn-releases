#!/usr/bin/env python3
"""Strict manifest validation for reviewed NGN candidate-match profiles."""
from __future__ import annotations

import hashlib
import json
import math
import re
from pathlib import Path
from typing import Any

SCHEMA = "ngn-candidate-match-v1"
CAPABILITY = "ngn-pre-m4c-pinned-one-worker-v1"
M4C_CAPABILITY = "ngn-m4c-time-controlled-smp-v1"
COMBINED_CAPABILITY = "ngn-m4c-counter55-vs-hce-longclock-v1"
THREADED_CAPABILITIES = {M4C_CAPABILITY, COMBINED_CAPABILITY}
COUNTER55_CAPABILITY = "ngn-counter55-inprocess-one-worker-v1"
COUNTER55_BACKEND = "counter-5.5"
COUNTER55_FORMAT = "counter-5.5"
COUNTER55_MODEL_SHA256 = "3488baed71f4d432d028e05b65e2ff13f8c9afe7f91974d1a3c34a0691d6670c"
COUNTER55_MODEL_SIZE = 1_576_988
COUNTER55_SOURCE_COMMIT = "63c487ca724c620f71c129d62129c6fb9109c872"
COUNTER55_MODEL_BLOB = "63c91e0bdab463fa20fe6da542e4158d99a92ea9"
COUNTER55_REVIEW_RECEIPT_SHA256 = "344d8c412c022860b5bf84a7756008425677ad14f3bc879eb1bf9d9af18fa599"
M4C_WIDTHS = {1, 2, 4, 8}
M4C_CPUS = [0, 2, 4, 6, 8, 10, 12, 14]
M4C_CPU_LIST = "0,2,4,6,8,10,12,14"
M4C_THREADS_BASIS = "m4c-uci-configured-and-per-go-effective-receipts"
SPRT_CLASSIFICATION = "prospective-strength"
OPTION_NAMES = ["Threads", "OwnBook", "EvalFile", "EvalBackend", "Hash", "Move Overhead"]
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
    backend = value["backend"]
    if backend not in {"hce", "ngn-v1", COUNTER55_BACKEND}:
        raise ManifestError(f"{location}.backend: expected hce, ngn-v1, or {COUNTER55_BACKEND}")
    network = value["network"]
    if network is not None:
        network = obj(network, f"{location}.network")
        exact_keys(network, {"path", "sha256", "format"}, {"size_bytes", "provenance"}, f"{location}.network")
        file_spec({"path": network["path"], "sha256": network["sha256"]}, f"{location}.network.file")
    if backend == "hce":
        if network is not None:
            raise ManifestError(f"{location}: hce forbids network")
    elif backend == "ngn-v1":
        if network is None or network["format"] != "ngn-v1":
            raise ManifestError(f"{location}: ngn-v1 requires an ngn-v1 network")
        if "size_bytes" in network or "provenance" in network:
            raise ManifestError(f"{location}: ngn-v1 v1 network shape changed")
    else:
        if network is None or network["format"] != COUNTER55_FORMAT:
            raise ManifestError(f"{location}: {COUNTER55_BACKEND} requires a {COUNTER55_FORMAT} network")
        if network.get("sha256") != COUNTER55_MODEL_SHA256 or network.get("size_bytes") != COUNTER55_MODEL_SIZE:
            raise ManifestError(f"{location}: {COUNTER55_BACKEND} requires the exact Counter 5.5 model")
        provenance = obj(network.get("provenance"), f"{location}.network.provenance")
        exact_keys(provenance, {"source_commit", "model_blob", "review_receipt"}, set(), f"{location}.network.provenance")
        if provenance["source_commit"] != COUNTER55_SOURCE_COMMIT or provenance["model_blob"] != COUNTER55_MODEL_BLOB:
            raise ManifestError(f"{location}.network.provenance: wrong Counter 5.5 source identity")
        receipt = file_spec(provenance["review_receipt"], f"{location}.network.provenance.review_receipt")
        if receipt["sha256"] != COUNTER55_REVIEW_RECEIPT_SHA256:
            raise ManifestError(f"{location}.network.provenance.review_receipt: unreviewed receipt")
    capability = value["capability_profile"]
    if capability not in {CAPABILITY, M4C_CAPABILITY, COMBINED_CAPABILITY, COUNTER55_CAPABILITY}:
        raise ManifestError(f"{location}.capability_profile: unsupported by v1")

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
    option_threads = integer(parsed.get("Threads"), f"{location}.uci_options.Threads", 1)
    expected_file = "$FROZEN_NETWORK" if backend != "hce" else "<empty>"
    if parsed != {
        "Threads": option_threads,
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
    if requested_threads != option_threads:
        raise ManifestError(f"{location}: requested Threads and option Threads differ")
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
    environment = obj(value["environment"], f"{location}.environment")
    exact_keys(environment, {"GOMAXPROCS"}, set(), f"{location}.environment")
    if capability in {CAPABILITY, COUNTER55_CAPABILITY}:
        if requested_threads != 1 or effective != {
            "Threads": 1,
            "GOMAXPROCS": 1,
            "OwnBook": False,
            "Hash": hash_mb,
            "Move Overhead": overhead,
            "threads_basis": "pinned-source-single-worker-plus-one-cpu-mask",
        }:
            raise ManifestError(f"{location}.effective: unsupported or inconsistent one-worker claim")
        if environment["GOMAXPROCS"] != "1":
            raise ManifestError(f"{location}.environment.GOMAXPROCS: one-worker v1 requires string 1")
    else:
        if capability == M4C_CAPABILITY and (value["backend"] != "hce" or value["network"] is not None):
            raise ManifestError(f"{location}: M4c profile is HCE-only")
        if capability == COMBINED_CAPABILITY and value["backend"] not in {COUNTER55_BACKEND, "hce"}:
            raise ManifestError(f"{location}: combined profile requires Counter 5.5 or HCE")
        if requested_threads not in M4C_WIDTHS:
            raise ManifestError(f"{location}: M4c manifest Threads must be one of {sorted(M4C_WIDTHS)}")
        if effective != {
            "Threads": requested_threads,
            "GOMAXPROCS": requested_threads,
            "OwnBook": False,
            "Hash": hash_mb,
            "Move Overhead": overhead,
            "threads_basis": M4C_THREADS_BASIS,
        }:
            raise ManifestError(f"{location}.effective: inconsistent M4c width or option claim")
        if environment["GOMAXPROCS"] != str(requested_threads):
            raise ManifestError(f"{location}.environment.GOMAXPROCS: must equal requested Threads")
    return value


def validate_manifest(value: Any) -> dict[str, Any]:
    value = obj(value, "manifest")
    exact_keys(value, {"schema", "status", "purpose", "classification", "approval", "inputs", "match", "resources", "limits", "operational_profile", "acceptance"}, {"sprt"}, "manifest")
    if value["schema"] != SCHEMA:
        raise ManifestError(f"manifest.schema: expected {SCHEMA}")
    if value["status"] not in {"HELD", "APPROVED_TO_RUN"}:
        raise ManifestError("manifest.status: expected HELD or APPROVED_TO_RUN")
    text(value["purpose"], "manifest.purpose")
    if value["classification"] not in {"diagnostic", SPRT_CLASSIFICATION}:
        raise ManifestError("manifest.classification: unsupported")
    sprt_mode = value["classification"] == SPRT_CLASSIFICATION

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
    input_names = {"runner", "schema", "common", "manifest_module", "supervisor", "uci_preflight", "match_stage", "trace_auditor", "role_exec", "chess_auditor", "fastchess", "stockfish", "opening_pgn", "opening_prefixes", "roles"}
    if sprt_mode:
        input_names.add("sprt_auditor")
    exact_keys(inputs, input_names, set(), "inputs")
    source_names = ["runner", "schema", "common", "manifest_module", "supervisor", "uci_preflight", "match_stage", "trace_auditor", "role_exec", "chess_auditor", "fastchess", "stockfish", "opening_pgn", "opening_prefixes"]
    if sprt_mode:
        source_names.append("sprt_auditor")
    for name in source_names:
        file_spec(inputs[name], f"inputs.{name}")
    roles = inputs["roles"]
    if not isinstance(roles, list) or len(roles) != 2:
        raise ManifestError("inputs.roles: exactly two roles required")
    roles = [role(item, index) for index, item in enumerate(roles)]
    if len({item["id"] for item in roles}) != 2 or len({item["display_name"] for item in roles}) != 2:
        raise ManifestError("inputs.roles: IDs and display names must be unique")
    capabilities = {item["capability_profile"] for item in roles}
    if len(capabilities) != 1:
        raise ManifestError("inputs.roles: capability profiles must match")
    capability = next(iter(capabilities))
    if capability == COMBINED_CAPABILITY and not sprt_mode:
        raise ManifestError("classification: combined long-clock capability is prospective-strength only")
    if capability == CAPABILITY and {item["backend"] for item in roles} != {"hce", "ngn-v1"}:
        raise ManifestError("inputs.roles: one-worker v1 requires one HCE and one NGN-v1 role")
    if capability == COUNTER55_CAPABILITY:
        if [item["backend"] for item in roles] != [COUNTER55_BACKEND, "hce"]:
            raise ManifestError("inputs.roles: Counter profile requires ordered counter-5.5 candidate then HCE reference")
        if len({(item["requested"]["Hash"], item["requested"]["Move Overhead"]) for item in roles}) != 1:
            raise ManifestError("inputs.roles: Counter profile requires equal Hash and Move Overhead")
    if capability == M4C_CAPABILITY and {item["backend"] for item in roles} != {"hce"}:
        raise ManifestError("inputs.roles: M4c profile requires two HCE roles")
    if capability == COMBINED_CAPABILITY and [item["backend"] for item in roles] != [COUNTER55_BACKEND, "hce"]:
        raise ManifestError("inputs.roles: combined profile requires ordered Counter 5.5 candidate then HCE reference")
    if sprt_mode:
        requested_threads = [item["requested"]["Threads"] for item in roles]
        if capability == M4C_CAPABILITY:
            if requested_threads != [8, 1]:
                raise ManifestError("inputs.roles: M4c prospective SPRT requires ordered HCE8 candidate then HCE1 reference")
        elif capability == COMBINED_CAPABILITY:
            if requested_threads != [8, 8]:
                raise ManifestError("inputs.roles: combined prospective SPRT requires equal eight-worker roles")
        elif capability == COUNTER55_CAPABILITY:
            if requested_threads != [1, 1]:
                raise ManifestError("inputs.roles: Counter prospective SPRT requires equal one-worker roles")
        else:
            raise ManifestError("inputs.roles: capability does not support prospective SPRT")
        if any(item["requested"]["Hash"] != 128 or item["requested"]["Move Overhead"] != 100 for item in roles):
            raise ManifestError("inputs.roles: prospective SPRT fixes Hash=128 and Move Overhead=100")
    binary_identities = {(item["binary"]["sha256"], item["binary"]["provenance"]["source_commit"], item["binary"]["provenance"]["source_tree"]) for item in roles}
    if len(binary_identities) != 1:
        raise ManifestError("inputs.roles: v1 requires same-code binary/source identities")

    match = obj(value["match"], "match")
    exact_keys(match, {"games", "pairs", "time_control", "concurrency", "seed", "opening_plies", "use_affinity", "affinity_cpus", "runner_strict", "recover", "ponder", "book", "tablebase", "score_adjudication", "resign_adjudication", "draw_adjudication", "maxmoves_adjudication"}, set(), "match")
    games = integer(match["games"], "match.games", 2)
    pairs = integer(match["pairs"], "match.pairs", 1)
    if games != 2 * pairs:
        raise ManifestError("match: games must equal 2*pairs")
    time_control = text(match["time_control"], "match.time_control")
    expected_time_control = "60+0.6" if capability == COMBINED_CAPABILITY else "30+0.3"
    if sprt_mode and (games != 400 or pairs != 200 or time_control != expected_time_control):
        raise ManifestError(f"match: prospective SPRT fixes 400-game/200-pair cap at {expected_time_control}")
    concurrency = integer(match["concurrency"], "match.concurrency", 1)
    integer(match["seed"], "match.seed", 0)
    opening_plies = integer(match["opening_plies"], "match.opening_plies", 0)
    if capability == COMBINED_CAPABILITY and opening_plies != 16:
        raise ManifestError("match.opening_plies: combined long-clock profile requires exactly 16 plies")
    use_affinity = boolean(match["use_affinity"], "match.use_affinity")
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
    if capability in {CAPABILITY, COUNTER55_CAPABILITY}:
        if not use_affinity:
            raise ManifestError("match.use_affinity: one-worker v1 requires fastchess affinity")
        if resources["affinity_mode"] != "fastchess-per-game-one-worker":
            raise ManifestError("resources.affinity_mode: unsupported by one-worker v1")
    else:
        if use_affinity or concurrency != 1:
            raise ManifestError("match: M4c profile requires concurrency=1 and use_affinity=false")
        if affinity != M4C_CPUS or runner_cpus != M4C_CPUS:
            raise ManifestError(f"resources: M4c profile requires exact full physical-core group {M4C_CPUS}")
        if resources["required_cpus_allowed_list"] != M4C_CPU_LIST:
            raise ManifestError(f"resources.required_cpus_allowed_list: M4c profile requires {M4C_CPU_LIST}")
        if resources["affinity_mode"] != "parent-full-mask-no-fastchess-affinity":
            raise ManifestError("resources.affinity_mode: unsupported by M4c profile")
        role_hashes = {item["effective"]["Hash"] for item in roles}
        if role_hashes != {128}:
            raise ManifestError("inputs.roles: M4c profile requires equal total Hash=128 per role")

    if sprt_mode and capability == COUNTER55_CAPABILITY:
        if concurrency != 1 or affinity != [12]:
            raise ManifestError("match: Counter prospective SPRT requires concurrency=1 and exact CPU 12 affinity")
        if runner_cpus != [12] or resources["required_cpus_allowed_list"] != "12":
            raise ManifestError("resources: Counter prospective SPRT requires exact runner CPU 12")

    if sprt_mode:
        sprt = obj(value.get("sprt"), "sprt")
        exact_keys(sprt, {"model", "statistics", "elo0", "elo1", "alpha", "beta", "report_penta", "rating_interval", "first_crossing_required"}, set(), "sprt")
        for name in ("elo0", "elo1", "alpha", "beta"):
            number = sprt[name]
            if isinstance(number, bool) or not isinstance(number, (int, float)) or not math.isfinite(number):
                raise ManifestError(f"sprt.{name}: finite number required")
        if sprt != {
            "model": "normalized", "statistics": "pentanomial-complete-pairs",
            "elo0": 0, "elo1": 20, "alpha": 0.05, "beta": 0.05,
            "report_penta": True, "rating_interval": 1, "first_crossing_required": True,
        }:
            raise ManifestError("sprt: unsupported prospective normalized-Elo contract")
    elif "sprt" in value:
        raise ManifestError("sprt: diagnostic manifests forbid SPRT configuration")

    limits = obj(value["limits"], "limits")
    limit_names = {"preflight_seconds", "match_seconds", "trace_audit_seconds", "chess_audit_seconds", "term_grace_seconds", "sample_interval_seconds"}
    if sprt_mode:
        limit_names.add("sprt_audit_seconds")
    exact_keys(limits, limit_names, set(), "limits")
    for name, limit in limits.items():
        if isinstance(limit, bool) or not isinstance(limit, (int, float)) or not math.isfinite(limit) or limit <= 0:
            raise ManifestError(f"limits.{name}: finite positive number required")

    profile = obj(value["operational_profile"], "operational_profile")
    exact_keys(profile, {"name", "allow_fastchess_normal_exit", "ngn_crash_handler"}, set(), "operational_profile")
    profile_name = "ngn-m4c-known-benign-v1" if capability in THREADED_CAPABILITIES else "ngn-pre-m4c-known-benign-v1"
    if profile != {"name": profile_name, "allow_fastchess_normal_exit": True, "ngn_crash_handler": "required-two-line-init-per-observed-process"}:
        raise ManifestError("operational_profile: unsupported exact profile")

    acceptance = obj(value["acceptance"], "acceptance")
    if sprt_mode:
        exact_keys(acceptance, {"minimum_completed_games", "maximum_completed_games", "completed_games_even", "require_independent_chess_audit", "require_independent_sprt_audit", "require_first_boundary_crossing_at_final_pair", "require_zero_operational_errors", "require_zero_survivors", "strength_claim"}, set(), "acceptance")
        if acceptance != {
            "minimum_completed_games": 2, "maximum_completed_games": games, "completed_games_even": True,
            "require_independent_chess_audit": True, "require_independent_sprt_audit": True,
            "require_first_boundary_crossing_at_final_pair": True,
            "require_zero_operational_errors": True, "require_zero_survivors": True,
            "strength_claim": "prospective-normalized-Elo-SPRT-verdict-only",
        }:
            raise ManifestError("acceptance: must bind prospective SPRT gates and maximum count")
    else:
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
