#!/usr/bin/env python3
"""Frozen E0 Rodent V1.2 versus V1.1 Anand strength gate. WSL only."""

from __future__ import annotations

import argparse
import hashlib
import json
import math
import os
from pathlib import Path
import random
import shutil
import subprocess
import sys
import time


SCHEMA = "ngn-e0-v12-anand-fixed-gate-v1"
EXPECTED_MANIFEST_KEYS = {
    "schema", "driver_sha256", "release_state", "run_root", "protocol",
    "decision_rule", "claim_boundary", "startup_contract",
    "shared_host_contract", "engine", "frozen_sources",
}
EXPECTED_FROZEN_SOURCES = {
    "role_exec.py", "run_match_stage.py", "process_supervisor.py",
    "trace_audit.py", "audit_fastchess_match.py", "uci_preflight.py",
    "common.py", "fastchess", "stockfish", "openings", "prefixes",
    "anand_model", "v12_model",
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
RELEASE_STATE = "RELEASED_AFTER_STATIC_REVIEW"
HELD_STATE = "HELD_PENDING_STATIC_REVIEW"
DECISION_RULE = (
    "A/A uses Rodent V1.1 Anand in both roles and its paired-bootstrap 95% "
    "interval must include zero; then ADOPT Rodent V1.2 iff the fixed 400-game "
    "V1.2-A versus Anand-B paired-bootstrap lower95 relative Elo is strictly "
    "positive; otherwise SHELVE; never extend or rerun after observing score"
)
CLAIM_BOUNDARY = (
    "Rodent V1.2 default versus optimized Rodent V1.1 Anand relative Elo at "
    "Threads=1, Hash=128 and 10+0.1 on the shared WSL host only; not an absolute "
    "rating, 3300 proof, wider-thread claim, default change or installation"
)
STARTUP_CONTRACT = (
    "both roles use one exact executable starting at default HCE; options are "
    "sent in exact order Threads, OwnBook=false, EvalFile, EvalBackend, Hash, "
    "Move Overhead with an isready barrier after every option; EvalFile stages "
    "the role network before backend selection"
)
SHARED_HOST_CONTRACT = (
    "the Lean hopper remains running; ten-second prelaunch and five-second "
    "continuous non-owned CPU samples are retained as telemetry, but background "
    "CPU alone does not reject the sample; scheduler and time-management "
    "interference remain a disclosed limitation"
)


def die(message: str) -> None:
    raise SystemExit(f"e0-v12-anand-gate: {message}")


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


def checked_file(spec: dict, label: str) -> Path:
    path = Path(spec["path"]).resolve()
    expected = spec.get("sha256")
    if not path.is_file() or not expected or expected.startswith("PENDING"):
        die(f"{label} is absent or not frozen: {path}")
    actual = sha256(path)
    if actual != expected:
        die(f"{label} SHA-256 mismatch: expected {expected}, got {actual}")
    return path


def run_checked(argv: list[str], *, cwd: Path | None = None) -> None:
    result = subprocess.run(argv, cwd=cwd, check=False)
    if result.returncode:
        die(f"command failed rc={result.returncode}: {' '.join(map(str, argv))}")


def copy_frozen(source: Path, destination: Path, expected: str, *, executable: bool = False) -> None:
    shutil.copy2(source, destination)
    if sha256(destination) != expected:
        die(f"copied artifact changed: {destination}")
    if executable:
        destination.chmod(destination.stat().st_mode | 0o100)


def proc_snapshot() -> tuple[float, dict[int, int], dict[int, int], dict[int, str]]:
    ticks: dict[int, int] = {}
    parents: dict[int, int] = {}
    names: dict[int, str] = {}
    for entry in Path("/proc").iterdir():
        if not entry.name.isdigit():
            continue
        try:
            fields = (entry / "stat").read_text(encoding="utf-8").split()
            status = (entry / "status").read_text(encoding="utf-8").splitlines()
            pid = int(fields[0])
            ticks[pid] = int(fields[13]) + int(fields[14])
            parents[pid] = int(fields[3])
            names[pid] = next(
                (line.split(":", 1)[1].strip() for line in status if line.startswith("Name:")),
                "?",
            )
        except (FileNotFoundError, PermissionError, ValueError, IndexError):
            continue
    return time.monotonic(), ticks, parents, names


def descendants(root: int, parents: dict[int, int]) -> set[int]:
    owned = {root}
    changed = True
    while changed:
        changed = False
        for pid, parent in parents.items():
            if parent in owned and pid not in owned:
                owned.add(pid)
                changed = True
    return owned


def cpu_delta(before: tuple, after: tuple, owned_root: int | None = None) -> dict:
    t0, ticks0, _, _ = before
    t1, ticks1, parents1, names1 = after
    elapsed = max(t1 - t0, 1e-9)
    hz = os.sysconf(os.sysconf_names["SC_CLK_TCK"])
    owned = descendants(owned_root, parents1) if owned_root else set()
    rows = []
    for pid, value in ticks1.items():
        delta = value - ticks0.get(pid, value)
        if delta > 0 and pid not in owned:
            rows.append({"pid": pid, "name": names1.get(pid, "?"), "cores": delta / hz / elapsed})
    rows.sort(key=lambda row: row["cores"], reverse=True)
    return {
        "elapsed_seconds": elapsed,
        "non_owned_cores": sum(row["cores"] for row in rows),
        "top": rows[:20],
    }


def baseline_cpu(seconds: int, output: Path) -> dict:
    before = proc_snapshot()
    time.sleep(seconds)
    sample = cpu_delta(before, proc_snapshot())
    atomic_json(output, {
        "schema": "ngn-e0-shared-host-prelaunch-cpu-v1",
        "disposition": "TELEMETRY_ONLY",
        "background_cpu_rejects_sample": False,
        **sample,
    })
    return sample


def role_config(path: Path, engine: Path, engine_hash: str, cwd: Path) -> None:
    atomic_json(path, {
        "schema": "ngn-candidate-role-exec-v1",
        "engine": str(engine),
        "engine_sha256": engine_hash,
        "gomaxprocs": "1",
        "expected_cwd": str(cwd),
    })


def options(model: Path, backend: str) -> list[dict[str, str]]:
    return [
        {"name": "Threads", "value": "1"},
        {"name": "OwnBook", "value": "false"},
        {"name": "EvalFile", "value": str(model)},
        {"name": "EvalBackend", "value": backend},
        {"name": "Hash", "value": "128"},
        {"name": "Move Overhead", "value": "100"},
    ]


def preflight(source: Path, role: dict, phase: Path) -> None:
    config = phase / f"preflight-{role['id']}.config.json"
    crash = role["cwd"] / "ngn_crashes.log"
    crash.unlink(missing_ok=True)
    atomic_json(config, {
        "schema": "ngn-candidate-uci-preflight-v1",
        "role_id": role["id"],
        "launcher": str(role["launcher"]),
        "cwd": str(role["cwd"]),
        "backend": role["backend"],
        "capability_profile": "ngn-pre-m4c-pinned-one-worker-v1",
        "options": options(role["model"], role["backend"]),
        "barrier_timeout_seconds": 30,
    })
    output = phase / f"preflight-{role['id']}"
    run_checked([
        sys.executable,
        str(source / "uci_preflight.py"),
        "--config",
        str(config),
        "--output",
        str(output),
    ])
    retained = phase / f"preflight-{role['id']}.crash_handler.log"
    if not crash.exists():
        die(f"preflight crash-handler log absent for {role['id']}")
    shutil.move(crash, retained)


def phase_command(
    tools: Path,
    phase: Path,
    roles: list[dict],
    openings: Path,
    rounds: int,
    protocol: dict,
) -> list[str]:
    argv = [str(tools / "fastchess")]
    for role in roles:
        argv += [
            "-engine",
            f"cmd={role['launcher']}",
            f"name={role['name']}",
            f"dir={role['cwd']}",
        ]
        for option in options(role["model"], role["backend"]):
            argv.append(f"option.{option['name']}={option['value']}")
    argv += [
        "-openings", f"file={openings}", "format=pgn", "order=sequential", "start=1", "plies=6",
        "-each", f"tc={protocol['tc']}", "timemargin=0",
        "-rounds", str(rounds), "-games", "2", "-repeat",
        "-concurrency", str(protocol["concurrency"]),
        "-report", "penta=true",
        "-use-affinity", ",".join(protocol["physical_cpu_mask"]),
        "-srand", str(protocol["seed"]), "-strict", "-show-latency",
        "-pgnout", f"file={phase / 'games.pgn'}", "notation=uci", "append=false", "nodes=true",
        "seldepth=true", "nps=true", "hashfull=true", "timeleft=true", "latency=true", "pv=true",
        "-epdout", f"file={phase / 'final.epd'}", "append=false",
        "-log", f"file={phase / 'fastchess.log'}", "level=trace", "engine=true",
        "realtime=false", "append=false",
    ]
    return argv


def supervise(
    source: Path,
    command: list[str],
    phase: Path,
    roles: list[dict],
    masks: list[str],
    max_seconds: int,
) -> dict:
    match_cfg = phase / "match-stage.config.json"
    match_stdout = phase / "match.stdout"
    match_stderr = phase / "match.stderr"
    witness_path = phase / "match-witness.json"
    atomic_json(match_cfg, {
        "schema": "ngn-candidate-match-stage-v1",
        "command": command,
        "cwd": str(phase),
        "environment": {
            "HOME": str(phase),
            "LANG": "C",
            "LC_ALL": "C",
            "PATH": "/usr/bin:/bin",
            "TZ": "UTC",
        },
        "roles": [
            {
                "id": role["id"],
                "engine": str(role["engine"]),
                "engine_sha256": role["sha256"],
                "cwd": str(role["cwd"]),
                "gomaxprocs": "1",
                "allowed_cpu_masks": masks,
                "launcher": str(role["launcher"]),
            }
            for role in roles
        ],
        "sample_interval_seconds": 0.05,
        "match_stdout": str(match_stdout),
        "match_stderr": str(match_stderr),
        "witness": str(witness_path),
    })
    stage_cmd = [sys.executable, str(source / "run_match_stage.py"), "--config", str(match_cfg)]
    supervisor = [
        sys.executable, str(source / "process_supervisor.py"),
        "--label", phase.name,
        "--limit-seconds", str(max_seconds),
        "--term-grace-seconds", "5",
        "--sample-interval-seconds", "0.25",
        "--memory-limit-kib", "8388608",
        "--cpu-list", ",".join(masks),
        "--stdout", str(phase / "supervisor.stdout"),
        "--stderr", str(phase / "supervisor.stderr"),
        "--time-output", str(phase / "time.txt"),
        "--samples", str(phase / "process-samples.jsonl"),
        "--receipt", str(phase / "supervisor.json"),
        "--",
        *stage_cmd,
    ]
    process = subprocess.Popen(supervisor, cwd=phase)
    interference = []
    consecutive = 0
    counterfactual_triggered = False
    previous = proc_snapshot()
    while process.poll() is None:
        time.sleep(5)
        current = proc_snapshot()
        sample = cpu_delta(previous, current, owned_root=os.getpid())
        sample["unix_time"] = time.time()
        interference.append(sample)
        consecutive = consecutive + 1 if sample["non_owned_cores"] > 1.0 else 0
        counterfactual_triggered = counterfactual_triggered or consecutive >= 12
        previous = current
    atomic_json(phase / "non-owned-cpu.json", {
        "schema": "ngn-e0-shared-host-cpu-v1",
        "disposition": "TELEMETRY_ONLY",
        "background_cpu_rejects_sample": False,
        "counterfactual_threshold_cores": 1.0,
        "counterfactual_required_consecutive_samples": 12,
        "counterfactual_gate_triggered": counterfactual_triggered,
        "sample_seconds": 5,
        "samples": interference,
    })
    if process.returncode:
        die(f"{phase.name} supervisor failed rc={process.returncode}")
    receipt = json.loads((phase / "supervisor.json").read_text(encoding="utf-8"))
    if (
        receipt.get("state") != "COMPLETE"
        or receipt.get("command_returncode") != 0
        or receipt.get("supervisor_returncode") != 0
        or receipt.get("termination_reason") is not None
        or receipt.get("monitor_error") is not None
        or receipt.get("surviving_processes") != []
    ):
        die(f"{phase.name} supervisor receipt is not clean")
    witness = json.loads(witness_path.read_text(encoding="utf-8"))
    if (
        witness.get("state") != "COMPLETE"
        or witness.get("fastchess_returncode") != 0
        or witness.get("violations") != []
        or set(witness.get("observed_instances", {})) != {role["id"] for role in roles}
        or any(not witness["observed_instances"][role["id"]] for role in roles)
    ):
        die(f"{phase.name} process witness is not clean")
    return witness


def elo(score: float) -> float:
    bounded = min(max(score, 1e-12), 1 - 1e-12)
    return -400.0 * math.log10(1.0 / bounded - 1.0)


def paired_ci(penta: list[int]) -> dict:
    pairs = sum(penta)
    values = [0.0, 0.5, 1.0, 1.5, 2.0]
    mean = sum(count * value for count, value in zip(penta, values)) / pairs
    second = sum(count * value * value for count, value in zip(penta, values)) / pairs
    variance_mean = max(second - mean * mean, 0.0) / pairs
    score = mean / 2.0
    score_error = 1.959963984540054 * math.sqrt(variance_mean) / 2.0
    return {
        "pairs": pairs,
        "score": score,
        "elo": elo(score),
        "lower95_elo": elo(score - score_error),
        "upper95_elo": elo(score + score_error),
    }


def audit(
    source: Path,
    tools: Path,
    phase: Path,
    roles: list[dict],
    openings: Path,
    prefixes: Path,
    games: int,
    pairs: int,
) -> dict:
    audit_output = phase / "audit-match.json"
    run_checked([
        sys.executable,
        str(source / "audit_fastchess_match.py"),
        "--pgn", str(phase / "games.pgn"),
        "--final-epd", str(phase / "final.epd"),
        "--opening-prefixes", str(prefixes),
        "--opening-prefixes-sha256", sha256(prefixes),
        "--opening-pgn", str(openings),
        "--opening-pgn-sha256", sha256(openings),
        "--stockfish", str(tools / "stockfish"),
        "--stockfish-sha256", sha256(tools / "stockfish"),
        "--engine-a", roles[0]["name"],
        "--engine-b", roles[1]["name"],
        "--games", str(games),
        "--pairs", str(pairs),
        "--output", str(audit_output),
    ])
    witness = json.loads((phase / "match-witness.json").read_text(encoding="utf-8"))
    crash_logs = []
    for role in roles:
        crash_logs += [
            {
                "path": str(phase / f"preflight-{role['id']}.crash_handler.log"),
                "logged_path": str(role["cwd"] / "ngn_crashes.log"),
                "expected_processes": 1,
                "role_id": role["id"],
                "phase": "preflight",
            },
            {
                "path": str(role["cwd"] / "ngn_crashes.log"),
                "logged_path": str(role["cwd"] / "ngn_crashes.log"),
                "expected_processes": len(witness["observed_instances"][role["id"]]),
                "role_id": role["id"],
                "phase": "match",
            },
        ]
    trace_cfg = phase / "trace-audit.config.json"
    atomic_json(trace_cfg, {
        "schema": "ngn-candidate-trace-audit-config-v1",
        "trace": str(phase / "fastchess.log"),
        "match_exit": str(phase / "match.exit"),
        "match_stderr": str(phase / "match.stderr"),
        "roles": [
            {
                "id": role["id"],
                "display_name": role["name"],
                "resolved_options": options(role["model"], role["backend"]),
                "expected_processes": len(witness["observed_instances"][role["id"]]),
                "expected_refreshes": games,
            }
            for role in roles
        ],
        "crash_logs": crash_logs,
        "output": str(phase / "audit-operational.json"),
    })
    run_checked([sys.executable, str(source / "trace_audit.py"), "--config", str(trace_cfg)])
    chess = json.loads(audit_output.read_text(encoding="utf-8"))
    operational = json.loads((phase / "audit-operational.json").read_text(encoding="utf-8"))
    if chess.get("status") != "PASS" or operational.get("state") != "COMPLETE" or operational.get("pass") is not True:
        die(f"{phase.name} retained audit receipts are not clean")
    return chess


def paired_bootstrap(pair_audit: list[dict], seed: int, replicates: int) -> dict:
    points = [row["half_points"] / 4.0 for row in pair_audit]
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


def validate_manifest(manifest: dict, actual_driver_hash: str) -> None:
    if set(manifest) != EXPECTED_MANIFEST_KEYS or manifest.get("schema") != SCHEMA:
        die("manifest keys or schema differ from the reviewed contract")
    if manifest.get("driver_sha256") != actual_driver_hash:
        die(
            f"driver SHA-256 mismatch: manifest {manifest.get('driver_sha256')}, "
            f"actual {actual_driver_hash}"
        )
    if set(manifest.get("frozen_sources", {})) != EXPECTED_FROZEN_SOURCES:
        die("frozen source key set differs from the reviewed contract")
    if manifest.get("protocol") != EXPECTED_PROTOCOL:
        die("protocol differs from the predeclared fixed gate")
    if manifest.get("release_state") not in {HELD_STATE, RELEASE_STATE}:
        die("unknown release state")
    contracts = {
        "decision_rule": DECISION_RULE,
        "claim_boundary": CLAIM_BOUNDARY,
        "startup_contract": STARTUP_CONTRACT,
        "shared_host_contract": SHARED_HOST_CONTRACT,
    }
    for key, expected in contracts.items():
        if manifest.get(key) != expected:
            die(f"{key} differs from the reviewed contract")
    engine = manifest.get("engine", {})
    required_engine = {
        "path", "sha256", "source_commit", "source_tree", "go_version",
        "goamd64", "cgo_enabled", "build_flags",
    }
    if set(engine) != required_engine:
        die("engine provenance differs from the reviewed contract")
    if engine["goamd64"] != "v3" or engine["cgo_enabled"] != "0" or engine["build_flags"] != ["-trimpath"]:
        die("engine build contract differs from GOAMD64=v3 CGO_ENABLED=0 go build -trimpath")
    run_root = Path(manifest["run_root"]).resolve()
    if run_root.exists():
        die(f"immutable run_root already exists: {run_root}")


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("manifest", type=Path)
    parser.add_argument("--check-held", action="store_true", help="verify frozen inputs without engine execution")
    args = parser.parse_args()
    if sys.platform != "linux" or "microsoft" not in Path("/proc/version").read_text(encoding="utf-8").lower():
        die("execution is restricted to WSL")
    manifest = json.loads(args.manifest.read_text(encoding="utf-8"))
    actual_driver_hash = sha256(Path(__file__).resolve())
    validate_manifest(manifest, actual_driver_hash)
    sources = {
        name: checked_file(spec, name)
        for name, spec in manifest["frozen_sources"].items()
    }
    engine_source = checked_file(manifest["engine"], "shared engine")
    if args.check_held:
        print(json.dumps({
            "state": "HELD_STATIC_CHECK_COMPLETE",
            "driver_sha256": actual_driver_hash,
            "frozen_sources": len(sources),
            "shared_engine": "hash-checked",
            "run_root": "absent",
        }, sort_keys=True))
        return
    if manifest.get("release_state") != RELEASE_STATE:
        die("manifest is HELD; reviewed release manifest is required")

    run_root = Path(manifest["run_root"]).resolve()
    run_root.mkdir(parents=True)
    prelaunch_path = run_root / "prelaunch-non-owned-cpu.json"
    baseline_cpu(10, prelaunch_path)
    atomic_json(run_root / "frozen-manifest.json", manifest)
    source_dir = run_root / "source"
    tools_dir = run_root / "tools"
    inputs_dir = run_root / "inputs"
    for directory in (source_dir, tools_dir, inputs_dir):
        directory.mkdir()
    copy_frozen(Path(__file__).resolve(), source_dir / "driver.py", actual_driver_hash, executable=True)
    for name in (
        "role_exec.py", "run_match_stage.py", "process_supervisor.py",
        "trace_audit.py", "audit_fastchess_match.py", "uci_preflight.py", "common.py",
    ):
        copy_frozen(
            sources[name],
            source_dir / name,
            manifest["frozen_sources"][name]["sha256"],
            executable=name.endswith(".py"),
        )
    for name in ("fastchess", "stockfish"):
        copy_frozen(
            sources[name],
            tools_dir / name,
            manifest["frozen_sources"][name]["sha256"],
            executable=True,
        )

    openings = inputs_dir / "openings-first200.pgn"
    prefixes = inputs_dir / "opening-prefixes-first200.txt"
    anand_model = inputs_dir / "rodent-v1.1-anand.bin"
    v12_model = inputs_dir / "rodent-v1.2-default.bin"
    for key, destination in (
        ("openings", openings),
        ("prefixes", prefixes),
        ("anand_model", anand_model),
        ("v12_model", v12_model),
    ):
        copy_frozen(sources[key], destination, manifest["frozen_sources"][key]["sha256"])
    engine_frozen = inputs_dir / "ngn-shared"
    copy_frozen(engine_source, engine_frozen, manifest["engine"]["sha256"], executable=True)

    phase_specs = [
        (
            "aa",
            50,
            ("anand", "rodent-v1.1-anand", anand_model),
            ("anand", "rodent-v1.1-anand", anand_model),
        ),
        (
            "candidate",
            200,
            ("v12", "rodent-v1.2-default", v12_model),
            ("anand", "rodent-v1.1-anand", anand_model),
        ),
    ]
    results = {}
    for phase_name, rounds, left, right in phase_specs:
        phase = run_root / phase_name
        phase.mkdir()
        roles = []
        for side, (semantic, backend, model) in (("a", left), ("b", right)):
            role_id = f"{phase_name}-{side}"
            role_dir = phase / f"role-{side}"
            role_dir.mkdir()
            engine = role_dir / "ngn"
            engine_hash = sha256(engine_frozen)
            copy_frozen(engine_frozen, engine, engine_hash, executable=True)
            launcher = role_dir / "launch"
            copy_frozen(
                source_dir / "role_exec.py",
                launcher,
                manifest["frozen_sources"]["role_exec.py"]["sha256"],
                executable=True,
            )
            role_config(role_dir / "role-config.json", engine, engine_hash, role_dir)
            roles.append({
                "id": role_id,
                "name": f"{semantic.upper()}-{side.upper()}",
                "semantic": semantic,
                "backend": backend,
                "model": model,
                "cwd": role_dir,
                "engine": engine,
                "sha256": engine_hash,
                "launcher": launcher,
            })
        if phase_name == "aa" and [role["backend"] for role in roles] != [
            "rodent-v1.1-anand", "rodent-v1.1-anand"
        ]:
            die("A/A invariant failed: both roles must use Anand")
        if phase_name == "candidate" and [role["backend"] for role in roles] != [
            "rodent-v1.2-default", "rodent-v1.1-anand"
        ]:
            die("candidate orientation invariant failed: audited engine A must be V1.2")
        for role in roles:
            preflight(source_dir, role, phase)
        command = phase_command(
            tools_dir,
            phase,
            roles,
            openings,
            rounds,
            manifest["protocol"],
        )
        atomic_json(phase / "command.json", {"argv": command})
        supervise(
            source_dir,
            command,
            phase,
            roles,
            manifest["protocol"]["physical_cpu_mask"],
            2700 if phase_name == "aa" else 7200,
        )
        (phase / "match.exit").write_text("0\n", encoding="utf-8")
        audited = audit(
            source_dir,
            tools_dir,
            phase,
            roles,
            openings,
            prefixes,
            rounds * 2,
            rounds,
        )
        normal_ci = paired_ci(audited["penta_0_to_4"])
        bootstrap = paired_bootstrap(
            audited["pair_audit"],
            manifest["protocol"]["bootstrap_seed"],
            manifest["protocol"]["bootstrap_replicates"],
        )
        evidence_files = [
            "audit-match.json", "audit-operational.json", "match-witness.json",
            "supervisor.json", "games.pgn", "final.epd", "match.stdout",
            "match.stderr", "fastchess.log", "non-owned-cpu.json",
        ]
        results[phase_name] = {
            "engine_a": roles[0]["semantic"],
            "engine_b": roles[1]["semantic"],
            "backend_a": roles[0]["backend"],
            "backend_b": roles[1]["backend"],
            "audit": str(phase / "audit-match.json"),
            "paired_normal_ci": normal_ci,
            "paired_bootstrap_ci": bootstrap,
            "evidence_sha256": {
                name: sha256(phase / name)
                for name in evidence_files
            },
        }
        if phase_name == "aa" and not (
            bootstrap["lower95_elo"] <= 0 <= bootstrap["upper95_elo"]
        ):
            atomic_json(run_root / "decision.json", {
                "schema": "ngn-e0-v12-anand-fixed-gate-decision-v1",
                "decision": "INVALID_AA",
                "results": results,
            })
            die("A/A 95% CI excludes zero; candidate phase not run")

    decision = "ADOPT" if results["candidate"]["paired_bootstrap_ci"]["lower95_elo"] > 0 else "SHELVE"
    decision_path = run_root / "decision.json"
    atomic_json(decision_path, {
        "schema": "ngn-e0-v12-anand-fixed-gate-decision-v1",
        "decision": decision,
        "rule": (
            "ADOPT V1.2 iff the fixed 400-game paired-bootstrap lower95 relative "
            "Elo is strictly positive; otherwise SHELVE; never extend after observing score"
        ),
        "frozen_manifest_sha256": sha256(run_root / "frozen-manifest.json"),
        "results": results,
    })
    inventory = {}
    for path in sorted(run_root.rglob("*")):
        if path.is_file() and path.name not in {"inventory.sha256.json", "terminal.json"}:
            inventory[str(path.relative_to(run_root))] = sha256(path)
    inventory_path = run_root / "inventory.sha256.json"
    atomic_json(inventory_path, {
        "schema": "ngn-e0-v12-anand-fixed-gate-inventory-v1",
        "files": inventory,
    })
    atomic_json(run_root / "terminal.json", {
        "schema": "ngn-e0-v12-anand-fixed-gate-terminal-v1",
        "state": "COMPLETE",
        "decision": decision,
        "decision_sha256": sha256(decision_path),
        "inventory_sha256": sha256(inventory_path),
        "no_extension_after_observation": True,
        "shared_host_observational_evidence": True,
    })


if __name__ == "__main__":
    main()
