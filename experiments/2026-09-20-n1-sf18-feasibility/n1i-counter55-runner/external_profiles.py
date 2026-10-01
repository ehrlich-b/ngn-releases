#!/usr/bin/env python3
"""Pinned external-engine identities and fail-closed UCI admission contracts."""
from __future__ import annotations

import json
import re
from dataclasses import dataclass
from pathlib import Path
from typing import Any

from common import CandidateMatchError, sha256

ONE_CPU = (12,)
EIGHT_PHYSICAL_CPUS = (0, 2, 4, 6, 8, 10, 12, 14)
SUPPORTED_WIDTHS = {1: ONE_CPU, 8: EIGHT_PHYSICAL_CPUS}
RODENT11_T8_PROBE_ROOT = Path("/home/ehrli/repos/ngn-external-opponents-v1/output/rodent11-t8-capability-probe-20260907/attempt-003")
RODENT11_T8_PROBE_FILES = {
    "receipt.json": ("artifacts/receipt.json", "932523cfd7f92a25ed6adad0ee2eca8d303640dcfaab68028cb822f66cd2be05"),
    "transcript.jsonl": ("artifacts/transcript.jsonl", "a057d533373aeb2df225a0915a8e1e15ea56c90044e48a9b7d4e54d15480bf6d"),
    "engine.stdout": ("artifacts/engine.stdout", "24541b4e86341e0607d4742d2674d767d686323ff2dde2ecf32ba84a0610ad8e"),
    "engine.stderr": ("artifacts/engine.stderr", "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"),
    "origin-before.json": ("origin-before.json", "628af006ed7467970fe9335910f6870f2b926dfdf640c37e6368df5c1e026a76"),
    "origin-after.json": ("origin-after.json", "628af006ed7467970fe9335910f6870f2b926dfdf640c37e6368df5c1e026a76"),
    "supervisor-receipt.json": ("command/supervisor-receipt.json", "7ccfe509a01f25613f01c583b9a650ee03a37f340f97b6d7a6f6cb93747aa9d8"),
}
RODENT11_T8_ROOT_REVIEW = (
    Path("/home/ehrli/repos/ngn-external-opponents-v1/output/rodent11-t8-capability-probe-20260907/root-attempt003-capability-review-v1.json"),
    "6c60ea77b267ea13a6c6f5ee1ecd75bf843ac165c7942e83a9fedd4f8ec3de3f",
)


@dataclass(frozen=True)
class ExternalProfile:
    id: str
    display_name: str
    binary_sha256: str
    binary_size: int
    repository: str
    tag: str
    source_commit: str
    identity_receipt: str
    identity_receipt_sha256: str
    uci_name: str
    uci_author: str
    loaded_model_line: str | None
    launch_args: tuple[str, ...]
    ordered_options: tuple[tuple[str, Any], ...]
    exact_advertised_names: frozenset[str]
    spin_domains: dict[str, tuple[int, int, int | None]]
    check_defaults: dict[str, bool]
    stdout_banner_version: str | None
    stderr_contract: str
    thread_basis: str
    rated_anchor: bool
    supported_widths: tuple[int, ...]


COUNTER55 = ExternalProfile(
    id="counter-5.5-v1.55.0",
    display_name="Counter 5.5",
    binary_sha256="6c48fb52934d49d3774633e32f0b4fb4796b1e2c63925f24167ef0f0c0d761c8",
    binary_size=4316021,
    repository="ChizhovVadim/CounterGo",
    tag="v1.55.0",
    source_commit="63c487ca724c620f71c129d62129c6fb9109c872",
    identity_receipt="/home/ehrli/repos/ngn-next/output/go-opponent-preflight-20260906/handshake.json",
    identity_receipt_sha256="3d48a88f2dd0eac637c7a546af9eb37f38d4d017df0bc3e22bcc3a0ef7ceeef9",
    uci_name="Counter 5.5",
    uci_author="Vadim Chizhov",
    loaded_model_line=None,
    launch_args=(),
    ordered_options=(("Threads", "$WIDTH"), ("Hash", 128), ("ExperimentSettings", False)),
    exact_advertised_names=frozenset({"Hash", "Threads", "ExperimentSettings"}),
    spin_domains={"Hash": (16, 4, 65536), "Threads": (1, 1, None)},
    check_defaults={"ExperimentSettings": False},
    stdout_banner_version=None,
    stderr_contract="counter55-two-line-embedded-network-v1",
    thread_basis="runtime NumCPU/Threads domain under exact mask plus pinned Counter Lazy SMP source",
    rated_anchor=True,
    supported_widths=(1, 8),
)

RODENT11 = ExternalProfile(
    id="rodent-v1.1-anand-testers",
    display_name="Rodent V 1.1 Anand testers",
    binary_sha256="9c68d7b39dc933eff5fd2da88bd4be1b0d009f6d2add7485c121cfc0d1308531",
    binary_size=4013485,
    repository="nescitus/Rodent-V",
    tag="Rodent_v_1_1",
    source_commit="5689d0babebe95d87592eaaaee73ea555ef9345c",
    identity_receipt="/home/ehrli/rodent-v1.1-public-preflight-20260906/identity.json",
    identity_receipt_sha256="0241655b8b79ffec11a6af2e32ec775b82d08817743d2ca74fa2e195f601e10e",
    uci_name="Rodent V 1.1 AVX2",
    uci_author="Naman Thanki, Pawel Koziol, based on Sungorus by Pablo Vazquez",
    loaded_model_line="info string Loaded NNUE network: nets/rodent_anand_512hl.bin",
    launch_args=(),
    ordered_options=(("Hash", 128), ("UCI_LimitStrength", False), ("UCI_Elo", 3000)),
    exact_advertised_names=frozenset({"Hash", "Clear Hash", "UCI_LimitStrength", "UCI_Elo"}),
    spin_domains={"Hash": (16, 1, 4096), "UCI_Elo": (3000, 800, 3000)},
    check_defaults={"UCI_LimitStrength": False},
    stdout_banner_version="1.1",
    stderr_contract="empty",
    thread_basis="release Testers artifact omits Threads; admitted only at T1 under exact one-CPU mask",
    rated_anchor=True,
    supported_widths=(1,),
)

RODENT12 = ExternalProfile(
    id="rodent-v1.2-nontal-testers",
    display_name="Rodent V 1.2 non-Tal testers",
    binary_sha256="9cfb8195207ee5695c1973a89664ab73b34b5bcbc10ad3bc0f0afe28b9713cbc",
    binary_size=7311522,
    repository="nescitus/Rodent-V",
    tag="Rodent_v_1.2",
    source_commit="b53ffaf670590932957cb63b7b6d871f6f33b7d8",
    identity_receipt="/home/ehrli/rodent-v1.2-public-preflight-20260906/identity.json",
    identity_receipt_sha256="be2e9b2a4f142017a795e10c5726733074acef0412446eec2ead7a18d35e432f",
    uci_name="Rodent V 1.2 AVX2",
    uci_author="Naman Thanki, Pawel Koziol, based on Sungorus by Pablo Vazquez",
    loaded_model_line="info string Loaded NNUE network: nets/rodent_4kb_768hl_8ob_v2.bin",
    launch_args=(),
    ordered_options=(("Threads", "$WIDTH"), ("Hash", 128), ("UCI_Chess960", False), ("UCI_LimitStrength", False), ("UCI_Elo", 3000)),
    exact_advertised_names=frozenset({"Hash", "Clear Hash", "Threads", "UCI_Chess960", "UCI_LimitStrength", "UCI_Elo"}),
    spin_domains={"Hash": (16, 1, 4096), "Threads": (1, 1, 256), "UCI_Elo": (3000, 800, 3000)},
    check_defaults={"UCI_Chess960": False, "UCI_LimitStrength": False},
    stdout_banner_version="1.2",
    stderr_contract="empty",
    thread_basis="corrected V1.2 testers runtime Threads domain under exact mask plus pinned Rodent Lazy SMP source",
    rated_anchor=False,
    supported_widths=(1, 8),
)

PROFILES = {profile.id: profile for profile in (COUNTER55, RODENT11, RODENT12)}


def profile_for(profile_id: str) -> ExternalProfile:
    try:
        return PROFILES[profile_id]
    except KeyError as error:
        raise CandidateMatchError(f"unsupported external profile {profile_id!r}") from error


def resolved_options(profile: ExternalProfile, width: int) -> list[dict[str, Any]]:
    if isinstance(width, bool) or width not in SUPPORTED_WIDTHS:
        raise CandidateMatchError("external comparison width must be exactly 1 or 8")
    if width not in profile.supported_widths:
        raise CandidateMatchError(f"{profile.id}: width {width} unavailable in pinned release artifact")
    return [
        {"name": name, "value": width if value == "$WIDTH" else value}
        for name, value in profile.ordered_options
    ]



def validate_unavailable_width_evidence(profile: ExternalProfile, width: int) -> dict[str, Any]:
    """Validate the exact transcript-backed proof for a predeclared unavailable width."""
    if profile is not RODENT11 or width != 8:
        raise CandidateMatchError(f"{profile.id}: no transcript-backed unavailable-width evidence for T{width}")
    files: dict[str, dict[str, Any]] = {}
    for name, (relative, expected_sha) in RODENT11_T8_PROBE_FILES.items():
        path = (RODENT11_T8_PROBE_ROOT / relative).resolve(strict=True)
        actual_sha = sha256(path)
        if actual_sha != expected_sha:
            raise CandidateMatchError(f"{profile.id}: T8 capability proof {name} differs")
        files[name] = {"path": str(path), "sha256": actual_sha, "size": path.stat().st_size}
    review_path, review_sha = RODENT11_T8_ROOT_REVIEW
    review_path = review_path.resolve(strict=True)
    if sha256(review_path) != review_sha:
        raise CandidateMatchError(f"{profile.id}: T8 capability root review differs")
    receipt = json.loads(Path(files["receipt.json"]["path"]).read_text(encoding="utf-8"))
    supervisor = json.loads(Path(files["supervisor-receipt.json"]["path"]).read_text(encoding="utf-8"))
    expected_options = ["Hash", "Clear Hash", "UCI_LimitStrength", "UCI_Elo"]
    if (
        receipt.get("schema") != "ngn-rodent11-t8-uci-capability-probe-v2"
        or receipt.get("state") != "COMPLETE"
        or receipt.get("pass") is not True
        or receipt.get("search_commands_sent") != []
        or receipt.get("threads_option_present") is not False
        or receipt.get("advertised_option_names") != expected_options
        or receipt.get("binary_sha256_before") != profile.binary_sha256
        or receipt.get("binary_sha256_after") != profile.binary_sha256
        or receipt.get("origin_binary_sha256_before") != profile.binary_sha256
        or receipt.get("origin_binary_sha256_after") != profile.binary_sha256
        or receipt.get("process_returncode") != 0
        or receipt.get("process_reaped") is not True
        or receipt.get("surviving_exact_binary_processes") != []
        or receipt.get("uciok_count") != 1
        or receipt.get("readyok_count") != 1
    ):
        raise CandidateMatchError(f"{profile.id}: T8 capability receipt contract differs")
    for key in ("process_identity_before_uci", "process_identity_after_readyok"):
        identity = receipt.get(key, {})
        if identity.get("cpus_allowed_list") != ",".join(map(str, EIGHT_PHYSICAL_CPUS)) or identity.get("gomaxprocs") != "8":
            raise CandidateMatchError(f"{profile.id}: T8 capability process identity differs at {key}")
    proof_files = receipt.get("proof_files")
    for name in ("engine.stdout", "engine.stderr", "transcript.jsonl", "origin-before.json", "origin-after.json"):
        expected = files[name]
        recorded = proof_files.get(name, {}) if isinstance(proof_files, dict) else {}
        if recorded.get("sha256") != expected["sha256"] or recorded.get("size") != expected["size"]:
            raise CandidateMatchError(f"{profile.id}: T8 capability receipt binding differs for {name}")
    transcript_lines = Path(files["transcript.jsonl"]["path"]).read_text(encoding="utf-8").splitlines()
    transcript = [json.loads(line) for line in transcript_lines]
    sent = [row.get("line") for row in transcript if row.get("direction") == "runner-to-engine"]
    if sent != ["uci", "isready", "quit"] or any(str(line).startswith("go") for line in sent):
        raise CandidateMatchError(f"{profile.id}: T8 capability transcript command sequence differs")
    if (
        supervisor.get("state") != "COMPLETE"
        or supervisor.get("command_returncode") != 0
        or supervisor.get("supervisor_returncode") != 0
        or supervisor.get("cpu_list") != ",".join(map(str, EIGHT_PHYSICAL_CPUS))
        or supervisor.get("surviving_processes") != []
    ):
        raise CandidateMatchError(f"{profile.id}: T8 capability supervisor receipt differs")
    return {
        "profile": profile.id,
        "width": width,
        "availability": False,
        "reason": "exact pinned full-mask GOMAXPROCS=8 UCI transcript advertises no Threads option",
        "capability_probe_engine_launched": True,
        "search_commands_sent": [],
        "files": files,
        "root_review": {"path": str(review_path), "sha256": review_sha, "size": review_path.stat().st_size},
    }

def validate_artifact(profile: ExternalProfile, binary: Path, identity_receipt: Path) -> dict[str, Any]:
    if not binary.is_file() or binary.stat().st_size != profile.binary_size:
        raise CandidateMatchError(f"{profile.id}: binary size differs")
    if sha256(binary) != profile.binary_sha256:
        raise CandidateMatchError(f"{profile.id}: binary SHA-256 differs")
    # A run-local immutable copy is required; identity is by the pinned content hash.
    if not identity_receipt.is_file() or sha256(identity_receipt) != profile.identity_receipt_sha256:
        raise CandidateMatchError(f"{profile.id}: identity receipt differs")
    return {
        "profile": profile.id,
        "binary_sha256": profile.binary_sha256,
        "binary_size": profile.binary_size,
        "identity_receipt_sha256": profile.identity_receipt_sha256,
    }


def validate_identity(profile: ExternalProfile, lines: list[str]) -> dict[str, Any]:
    names = [line[8:] for line in lines if line.startswith("id name ")]
    authors = [line[10:] for line in lines if line.startswith("id author ")]
    if names != [profile.uci_name] or authors != [profile.uci_author]:
        raise CandidateMatchError(f"{profile.id}: UCI identity differs names={names!r} authors={authors!r}")
    if any("nnue not loaded" in line.lower() for line in lines):
        raise CandidateMatchError(f"{profile.id}: NNUE load failure reported")
    if profile.loaded_model_line is not None and lines.count(profile.loaded_model_line) != 1:
        raise CandidateMatchError(f"{profile.id}: loaded model receipt differs")
    if profile.stdout_banner_version is not None:
        wanted = f"Rodent V {profile.stdout_banner_version}"
        if sum(line.strip() == wanted for line in lines) != 1:
            raise CandidateMatchError(f"{profile.id}: startup banner differs")
    return {"name": names[0], "author": authors[0], "loaded_model_line": profile.loaded_model_line}


def validate_advertised_options(
    profile: ExternalProfile,
    advertised: dict[str, dict[str, Any]],
    width: int,
) -> dict[str, Any]:
    if set(advertised) != set(profile.exact_advertised_names):
        raise CandidateMatchError(
            f"{profile.id}: option names differ actual={sorted(advertised)} "
            f"expected={sorted(profile.exact_advertised_names)}"
        )
    for name, (default, minimum, maximum) in profile.spin_domains.items():
        option = advertised.get(name, {})
        expected_max = width if name == "Threads" and profile is COUNTER55 else maximum
        if option.get("type") != "spin":
            raise CandidateMatchError(f"{profile.id}: {name} is not spin")
        actual = (option.get("default"), option.get("min"), option.get("max"))
        expected = (default, minimum, expected_max)
        if actual != expected:
            raise CandidateMatchError(f"{profile.id}: {name} domain={actual!r} expected={expected!r}")
    for name, default in profile.check_defaults.items():
        option = advertised.get(name, {})
        if option.get("type") != "check" or option.get("default") is not default:
            raise CandidateMatchError(f"{profile.id}: {name} check/default differs")
    if "Clear Hash" in profile.exact_advertised_names and advertised["Clear Hash"].get("type") != "button":
        raise CandidateMatchError(f"{profile.id}: Clear Hash is not a button")
    requested = resolved_options(profile, width)
    for item in requested:
        option = advertised[item["name"]]
        if option["type"] == "spin" and not option["min"] <= item["value"] <= option["max"]:
            raise CandidateMatchError(f"{profile.id}: requested {item['name']} outside domain")
    return {"advertised": advertised, "requested": requested, "thread_basis": profile.thread_basis}


def validate_stderr(profile: ExternalProfile, lines: list[str], width: int) -> list[str]:
    if profile.stderr_contract == "empty":
        if lines:
            raise CandidateMatchError(f"{profile.id}: stderr must be empty: {lines!r}")
        return []
    stamp = r"[0-9]{4}/[0-9]{2}/[0-9]{2} [0-9]{2}:[0-9]{2}:[0-9]{2}"
    startup = re.compile(
        rf"^{stamp} main\.go:39: Counter VersionName 5\.5 BuildDate 2024-01-12 "
        rf"GitRevision {COUNTER55.source_commit} RuntimeVersion go1\.21\.0 "
        rf"GOARCH amd64 GOOS linux NumCPU {width}$"
    )
    loaded = re.compile(rf"^{stamp} loaded embed nnue weights$")
    if len(lines) != 2 or startup.fullmatch(lines[0]) is None or loaded.fullmatch(lines[1]) is None:
        raise CandidateMatchError(f"{profile.id}: Counter startup stderr differs: {lines!r}")
    return lines


def validate_empty_cwd(profile: ExternalProfile, cwd: Path) -> None:
    entries = list(cwd.iterdir())
    if entries:
        raise CandidateMatchError(f"{profile.id}: engine cwd is not empty: {[entry.name for entry in entries]}")
