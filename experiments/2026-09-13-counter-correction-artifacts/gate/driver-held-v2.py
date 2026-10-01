#!/usr/bin/env python3
"""Held, fixed-sample Counter policy strength gate. Linux/WSL only."""

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

BASE_COMMIT = "270c73563139996d03f3c34b2840015b8e130df9"
EXPECTED_FROZEN_SOURCES = {
    "role_exec.py", "run_match_stage.py", "process_supervisor.py", "trace_audit.py",
    "audit_fastchess_match.py", "uci_preflight.py", "common.py", "fastchess", "stockfish",
    "openings", "prefixes", "model", "candidate_production_patch",
    "candidate_regression_patch", "build_test_receipt",
}
EXPECTED_PROTOCOL = {
    "aa_games": 100, "candidate_games": 400, "tc": "10+0.1", "concurrency": 4,
    "physical_cpu_mask": ["0", "2", "4", "6"], "seed": 20260912,
    "bootstrap_seed": 2026091201, "bootstrap_replicates": 100000,
}


def die(message):
    raise SystemExit(f"counter-policy-gate: {message}")


def sha256(path):
    digest = hashlib.sha256()
    with open(path, "rb") as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def atomic_json(path, value):
    temporary = path.with_suffix(path.suffix + ".tmp")
    temporary.write_text(json.dumps(value, indent=2, sort_keys=True) + "\n")
    os.replace(temporary, path)


def checked_file(spec, label):
    path = Path(spec["path"]).resolve()
    expected = spec.get("sha256")
    if not path.is_file() or not expected or expected.startswith("PENDING"):
        die(f"{label} is absent or not frozen: {path}")
    actual = sha256(path)
    if actual != expected:
        die(f"{label} SHA-256 mismatch: expected {expected}, got {actual}")
    return path


def run_checked(argv, *, cwd=None, stdout=None, stderr=None):
    result = subprocess.run(argv, cwd=cwd, stdout=stdout, stderr=stderr, check=False)
    if result.returncode:
        die(f"command failed rc={result.returncode}: {' '.join(map(str, argv))}")


def copy_frozen(source, destination, expected):
    shutil.copy2(source, destination)
    if sha256(destination) != expected:
        die(f"copied artifact changed: {destination}")
    destination.chmod(destination.stat().st_mode | 0o100)


def proc_snapshot():
    ticks = {}
    parents = {}
    names = {}
    for entry in Path("/proc").iterdir():
        if not entry.name.isdigit():
            continue
        try:
            fields = (entry / "stat").read_text().split()
            status = (entry / "status").read_text().splitlines()
            pid = int(fields[0])
            ticks[pid] = int(fields[13]) + int(fields[14])
            parents[pid] = int(fields[3])
            names[pid] = next((x.split(":", 1)[1].strip() for x in status if x.startswith("Name:")), "?")
        except (FileNotFoundError, PermissionError, ValueError, IndexError):
            continue
    return time.monotonic(), ticks, parents, names


def descendants(root, parents):
    owned = {root}
    changed = True
    while changed:
        changed = False
        for pid, ppid in parents.items():
            if ppid in owned and pid not in owned:
                owned.add(pid)
                changed = True
    return owned


def cpu_delta(before, after, owned_root=None):
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
    return {"elapsed_seconds": elapsed, "non_owned_cores": sum(x["cores"] for x in rows), "top": rows[:20]}


def baseline_cpu(seconds, output):
    before = proc_snapshot()
    time.sleep(seconds)
    sample = cpu_delta(before, proc_snapshot())
    atomic_json(output, sample)
    return sample


def role_config(path, engine, engine_hash, cwd):
    atomic_json(path, {
        "schema": "ngn-candidate-role-exec-v1",
        "engine": str(engine), "engine_sha256": engine_hash,
        "gomaxprocs": "1", "expected_cwd": str(cwd),
    })


def options(model):
    return [
        {"name": "Threads", "value": "1"},
        {"name": "OwnBook", "value": "false"},
        {"name": "EvalFile", "value": str(model)},
        {"name": "EvalBackend", "value": "counter-5.5"},
        {"name": "Hash", "value": "128"},
        {"name": "Move Overhead", "value": "100"},
    ]


def preflight(source, role, launcher, cwd, model, phase):
    config = phase / f"preflight-{role['id']}.config.json"
    crash = cwd / "ngn_crashes.log"
    crash.unlink(missing_ok=True)
    atomic_json(config, {
        "schema": "ngn-candidate-uci-preflight-v1", "role_id": role["id"],
        "launcher": str(launcher), "cwd": str(cwd), "backend": "counter-5.5",
        "capability_profile": "ngn-counter55-inprocess-one-worker-v1",
        "options": options(model), "barrier_timeout_seconds": 30,
    })
    output = phase / f"preflight-{role['id']}.json"
    run_checked([sys.executable, str(source / "uci_preflight.py"), "--config", str(config), "--output", str(output)])
    retained = phase / f"preflight-{role['id']}.crash_handler.log"
    if crash.exists():
        shutil.move(crash, retained)
    else:
        die(f"preflight crash-handler log absent for {role['id']}")
    return retained


def phase_command(tools, phase, roles, model, openings, rounds, seed, masks):
    argv = [str(tools / "fastchess")]
    for role in roles:
        argv += ["-engine", f"cmd={role['launcher']}", f"name={role['name']}", f"dir={role['cwd']}"]
        for option in options(model):
            argv.append(f"option.{option['name']}={option['value']}")
    argv += [
        "-openings", f"file={openings}", "format=pgn", "order=sequential", "start=1", "plies=6",
        "-each", "tc=10+0.1", "timemargin=0",
        "-rounds", str(rounds), "-games", "2", "-repeat", "-concurrency", "4",
        "-report", "penta=true", "-use-affinity", ",".join(masks),
        "-srand", str(seed), "-strict", "-show-latency",
        "-pgnout", f"file={phase / 'games.pgn'}", "notation=uci", "append=false", "nodes=true",
        "seldepth=true", "nps=true", "hashfull=true", "timeleft=true", "latency=true", "pv=true",
        "-epdout", f"file={phase / 'final.epd'}", "append=false",
        "-log", f"file={phase / 'fastchess.log'}", "level=trace", "engine=true", "realtime=false", "append=false",
    ]
    return argv


def supervise(source, command, phase, roles, masks, max_seconds):
    match_cfg = phase / "match-stage.config.json"
    match_stdout = phase / "match.stdout"
    match_stderr = phase / "match.stderr"
    witness = phase / "match-witness.json"
    atomic_json(match_cfg, {
        "schema": "ngn-candidate-match-stage-v1", "command": command, "cwd": str(phase),
        "environment": {"HOME": str(phase), "LANG": "C", "LC_ALL": "C", "PATH": "/usr/bin:/bin", "TZ": "UTC"},
        "roles": [{"id": r["id"], "engine": str(r["engine"]), "engine_sha256": r["sha256"],
                   "cwd": str(r["cwd"]), "gomaxprocs": "1", "allowed_cpu_masks": masks,
                   "launcher": str(r["launcher"])} for r in roles],
        "sample_interval_seconds": 0.05, "match_stdout": str(match_stdout),
        "match_stderr": str(match_stderr), "witness": str(witness),
    })
    stage_cmd = [sys.executable, str(source / "run_match_stage.py"), "--config", str(match_cfg)]
    supervisor = [
        sys.executable, str(source / "process_supervisor.py"), "--label", phase.name,
        "--limit-seconds", str(max_seconds), "--term-grace-seconds", "5",
        "--sample-interval-seconds", "0.25", "--memory-limit-kib", "8388608",
        "--cpu-list", ",".join(masks), "--stdout", str(phase / "supervisor.stdout"),
        "--stderr", str(phase / "supervisor.stderr"), "--time-output", str(phase / "time.txt"),
        "--samples", str(phase / "process-samples.jsonl"), "--receipt", str(phase / "supervisor.json"),
        "--", *stage_cmd,
    ]
    process = subprocess.Popen(supervisor, cwd=phase)
    interference = []
    consecutive = 0
    rejected = False
    previous = proc_snapshot()
    while process.poll() is None:
        time.sleep(5)
        current = proc_snapshot()
        sample = cpu_delta(previous, current, owned_root=os.getpid())
        sample["unix_time"] = time.time()
        interference.append(sample)
        consecutive = consecutive + 1 if sample["non_owned_cores"] > 1.0 else 0
        rejected = rejected or consecutive >= 12
        previous = current
    atomic_json(phase / "non-owned-cpu.json", {
        "schema": "ngn-counter-policy-non-owned-cpu-v1", "threshold_cores": 1.0,
        "required_consecutive_samples": 12, "sample_seconds": 5,
        "timing_evidence_rejected": rejected, "samples": interference,
    })
    if process.returncode:
        die(f"{phase.name} supervisor failed rc={process.returncode}")
    receipt = json.loads((phase / "supervisor.json").read_text())
    if (receipt.get("state") != "COMPLETE" or receipt.get("command_returncode") != 0
            or receipt.get("supervisor_returncode") != 0 or receipt.get("termination_reason") is not None
            or receipt.get("monitor_error") is not None or receipt.get("surviving_processes") != []):
        die(f"{phase.name} supervisor receipt is not clean")
    witness = json.loads((phase / "match-witness.json").read_text())
    if (witness.get("state") != "COMPLETE" or witness.get("fastchess_returncode") != 0
            or witness.get("violations") != [] or set(witness.get("observed_instances", {})) != {r["id"] for r in roles}
            or any(not witness["observed_instances"][r["id"]] for r in roles)):
        die(f"{phase.name} process witness is not clean")
    if rejected:
        die(f"{phase.name} timing evidence rejected: non-owned CPU exceeded one core for >=60 seconds")
    return witness


def paired_ci(penta):
    # Fastchess pentanomial normal approximation: each pair score is 0,.5,1,1.5,2.
    pairs = sum(penta)
    values = [0.0, 0.5, 1.0, 1.5, 2.0]
    mean = sum(n * x for n, x in zip(penta, values)) / pairs
    second = sum(n * x * x for n, x in zip(penta, values)) / pairs
    variance_mean = max(second - mean * mean, 0.0) / pairs
    score = mean / 2.0
    score_error = 1.959963984540054 * math.sqrt(variance_mean) / 2.0
    def elo(s):
        s = min(max(s, 1e-12), 1 - 1e-12)
        return -400.0 * math.log10(1.0 / s - 1.0)
    return {"pairs": pairs, "score": score, "elo": elo(score),
            "lower95_elo": elo(score - score_error), "upper95_elo": elo(score + score_error)}


def audit(source, tools, phase, roles, model, openings, prefixes, games, pairs):
    audit_output = phase / "audit-match.json"
    run_checked([
        sys.executable, str(source / "audit_fastchess_match.py"), "--pgn", str(phase / "games.pgn"),
        "--final-epd", str(phase / "final.epd"), "--opening-prefixes", str(prefixes),
        "--opening-prefixes-sha256", sha256(prefixes), "--opening-pgn", str(openings),
        "--opening-pgn-sha256", sha256(openings), "--stockfish", str(tools / "stockfish"),
        "--stockfish-sha256", sha256(tools / "stockfish"), "--engine-a", roles[0]["name"],
        "--engine-b", roles[1]["name"], "--games", str(games), "--pairs", str(pairs),
        "--output", str(audit_output),
    ])
    witness = json.loads((phase / "match-witness.json").read_text())
    crash_logs = []
    for role in roles:
        crash_logs += [
            {"path": str(phase / f"preflight-{role['id']}.crash_handler.log"), "logged_path": str(role["cwd"] / "ngn_crashes.log"),
             "expected_processes": 1, "role_id": role["id"], "phase": "preflight"},
            {"path": str(role["cwd"] / "ngn_crashes.log"), "logged_path": str(role["cwd"] / "ngn_crashes.log"),
             "expected_processes": len(witness["observed_instances"][role["id"]]), "role_id": role["id"], "phase": "match"},
        ]
    trace_cfg = phase / "trace-audit.config.json"
    atomic_json(trace_cfg, {
        "schema": "ngn-candidate-trace-audit-config-v1", "trace": str(phase / "fastchess.log"),
        "match_exit": str(phase / "match.exit"), "match_stderr": str(phase / "match.stderr"),
        "roles": [{"id": r["id"], "display_name": r["name"],
                   "capability_profile": "ngn-counter55-inprocess-one-worker-v1",
                   "resolved_options": options(model),
                   "expected_processes": len(witness["observed_instances"][r["id"]]),
                   "expected_refreshes": games} for r in roles],
        "crash_logs": crash_logs, "output": str(phase / "audit-operational.json"),
    })
    run_checked([sys.executable, str(source / "trace_audit.py"), "--config", str(trace_cfg)])
    chess = json.loads(audit_output.read_text())
    operational = json.loads((phase / "audit-operational.json").read_text())
    if chess.get("status") != "PASS" or operational.get("state") != "COMPLETE" or operational.get("pass") is not True:
        die(f"{phase.name} retained audit receipts are not clean")
    return chess


def paired_bootstrap(pair_audit, seed, replicates=100000):
    points = [row["half_points"] / 4.0 for row in pair_audit]
    rng = random.Random(seed)
    estimates = []
    for _ in range(replicates):
        score = sum(rng.choice(points) for _ in points) / len(points)
        score = min(max(score, 1e-12), 1 - 1e-12)
        estimates.append(-400.0 * math.log10(1.0 / score - 1.0))
    estimates.sort()
    lower = estimates[int(0.025 * replicates)]
    upper = estimates[int(0.975 * replicates) - 1]
    score = sum(points) / len(points)
    score = min(max(score, 1e-12), 1 - 1e-12)
    return {"method": "paired nonparametric percentile bootstrap", "seed": seed,
            "replicates": replicates, "pairs": len(points),
            "elo": -400.0 * math.log10(1.0 / score - 1.0),
            "lower95_elo": lower, "upper95_elo": upper}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("manifest", type=Path)
    parser.add_argument("--check-held", action="store_true", help="verify frozen inputs without engines or execution")
    args = parser.parse_args()
    if sys.platform != "linux" or "microsoft" not in Path("/proc/version").read_text().lower():
        die("execution is restricted to WSL")
    manifest = json.loads(args.manifest.read_text())
    if manifest.get("schema") != "ngn-counter-policy-fixed-gate-v1":
        die("wrong manifest schema")
    actual_driver_hash = sha256(Path(__file__).resolve())
    if manifest.get("driver_sha256") != actual_driver_hash:
        die(f"driver SHA-256 mismatch: manifest {manifest.get('driver_sha256')}, actual {actual_driver_hash}")
    run_root = Path(manifest["run_root"]).resolve()
    if run_root.exists():
        die(f"immutable run_root already exists: {run_root}")
    if set(manifest.get("frozen_sources", {})) != EXPECTED_FROZEN_SOURCES:
        die("frozen source key set differs from the reviewed contract")
    sources = {name: checked_file(spec, name) for name, spec in manifest["frozen_sources"].items()}
    base_source = checked_file(manifest["engines"]["base"], "base engine")
    candidate_source = checked_file(manifest["engines"]["candidate"], "candidate engine")
    if manifest["engines"]["base"]["source_commit"] != BASE_COMMIT:
        die("base source commit is not frozen immediate base")
    if manifest["engines"]["candidate"]["base_commit"] != BASE_COMMIT:
        die("candidate is not based on frozen immediate base")
    if manifest.get("protocol") != EXPECTED_PROTOCOL:
        die("protocol differs from the predeclared fixed gate")
    masks = manifest["protocol"]["physical_cpu_mask"]
    if args.check_held:
        print(json.dumps({"state": "HELD_STATIC_CHECK_COMPLETE", "driver_sha256": actual_driver_hash,
                          "frozen_sources": len(sources), "engines": "hash-checked"}, sort_keys=True))
        return
    if manifest.get("release_state") != "RELEASED_AFTER_INDEPENDENT_REVIEW":
        die("manifest is HELD; independent reviewer must explicitly release it")
    prelaunch_path = args.manifest.resolve().with_suffix(".prelaunch-cpu.json")
    prelaunch = baseline_cpu(10, prelaunch_path)
    if prelaunch["non_owned_cores"] > 1.0:
        die(f"prelaunch non-owned CPU exceeds one core; evidence retained at {prelaunch_path}")
    run_root.mkdir(parents=True)
    atomic_json(run_root / "frozen-manifest.json", manifest)
    shutil.copy2(prelaunch_path, run_root / "prelaunch-non-owned-cpu.json")
    source_dir = run_root / "source"
    tools_dir = run_root / "tools"
    inputs_dir = run_root / "inputs"
    for directory in (source_dir, tools_dir, inputs_dir):
        directory.mkdir()
    copy_frozen(Path(__file__).resolve(), source_dir / "counter_policy_gate_driver.py", actual_driver_hash)
    for name in ("role_exec.py", "run_match_stage.py", "process_supervisor.py", "trace_audit.py", "audit_fastchess_match.py", "uci_preflight.py", "common.py"):
        copy_frozen(sources[name], source_dir / name, manifest["frozen_sources"][name]["sha256"])
    for name in ("fastchess", "stockfish"):
        copy_frozen(sources[name], tools_dir / name, manifest["frozen_sources"][name]["sha256"])
    openings = inputs_dir / "openings-first200.pgn"
    prefixes = inputs_dir / "opening-prefixes-first200.txt"
    model = inputs_dir / "n-30-5268.nn"
    for key, dst in (("openings", openings), ("prefixes", prefixes), ("model", model)):
        copy_frozen(sources[key], dst, manifest["frozen_sources"][key]["sha256"])
    for key in ("candidate_production_patch", "candidate_regression_patch", "build_test_receipt"):
        copy_frozen(sources[key], inputs_dir / sources[key].name, manifest["frozen_sources"][key]["sha256"])
    base_frozen = inputs_dir / "base-ngn"
    candidate_frozen = inputs_dir / "candidate-ngn"
    copy_frozen(base_source, base_frozen, manifest["engines"]["base"]["sha256"])
    copy_frozen(candidate_source, candidate_frozen, manifest["engines"]["candidate"]["sha256"])
    phase_specs = [
        ("aa", 50, ("base-a", base_frozen), ("base-b", base_frozen)),
        ("candidate", 200, ("candidate", candidate_frozen), ("base", base_frozen)),
    ]
    results = {}
    for phase_name, rounds, left, right in phase_specs:
        phase = run_root / phase_name
        phase.mkdir()
        roles = []
        for side, (semantic, engine_source) in (("a", left), ("b", right)):
            role_id = f"{phase_name}-{side}"
            role_dir = phase / f"role-{side}"
            role_dir.mkdir()
            engine = role_dir / "ngn"
            engine_hash = sha256(engine_source)
            copy_frozen(engine_source, engine, engine_hash)
            launcher = role_dir / "launch"
            shutil.copy2(source_dir / "role_exec.py", launcher)
            launcher.chmod(0o755)
            role_config(role_dir / "role-config.json", engine, engine_hash, role_dir)
            roles.append({"id": role_id, "name": f"{semantic.upper()}-{side.upper()}", "semantic": semantic,
                          "cwd": role_dir, "engine": engine, "sha256": engine_hash, "launcher": launcher})
        if phase_name == "candidate" and [role["semantic"] for role in roles] != ["candidate", "base"]:
            die("candidate orientation invariant failed: audited engine A must be candidate")
        for role in roles:
            preflight(source_dir, role, role["launcher"], role["cwd"], model, phase)
        command = phase_command(tools_dir, phase, roles, model, openings, rounds, manifest["protocol"]["seed"], masks)
        atomic_json(phase / "command.json", {"argv": command})
        supervise(source_dir, command, phase, roles, masks, 2700 if phase_name == "aa" else 7200)
        (phase / "match.exit").write_text("0\n")
        audited = audit(source_dir, tools_dir, phase, roles, model, openings, prefixes, rounds * 2, rounds)
        ci = paired_ci(audited["penta_0_to_4"])
        bootstrap = paired_bootstrap(audited["pair_audit"], manifest["protocol"]["bootstrap_seed"],
                                     manifest["protocol"]["bootstrap_replicates"])
        evidence_files = ["audit-match.json", "audit-operational.json", "match-witness.json", "supervisor.json",
                          "games.pgn", "final.epd", "match.stdout", "match.stderr", "fastchess.log",
                          "non-owned-cpu.json"]
        results[phase_name] = {"engine_a": roles[0]["semantic"], "engine_b": roles[1]["semantic"],
                               "audit": str(phase / "audit-match.json"), "paired_normal_ci": ci,
                               "paired_bootstrap_ci": bootstrap,
                               "evidence_sha256": {name: sha256(phase / name) for name in evidence_files}}
        if phase_name == "aa" and not (bootstrap["lower95_elo"] <= 0 <= bootstrap["upper95_elo"]):
            atomic_json(run_root / "decision.json", {"decision": "INVALID_AA", "results": results})
            die("A/A 95% CI excludes zero; candidate phase not run")
    decision = "ADOPT" if results["candidate"]["paired_bootstrap_ci"]["lower95_elo"] > 0 else "SHELVE"
    decision_path = run_root / "decision.json"
    atomic_json(decision_path, {
        "schema": "ngn-counter-policy-fixed-gate-decision-v1", "decision": decision,
        "rule": "ADOPT iff fixed 400-game paired lower95 Elo > 0; never extend after observing score",
        "frozen_manifest_sha256": sha256(run_root / "frozen-manifest.json"), "results": results,
    })
    inventory = {}
    for path in sorted(run_root.rglob("*")):
        if path.is_file() and path.name not in {"inventory.sha256.json", "terminal.json"}:
            inventory[str(path.relative_to(run_root))] = sha256(path)
    inventory_path = run_root / "inventory.sha256.json"
    atomic_json(inventory_path, {"schema": "ngn-counter-policy-fixed-gate-inventory-v1", "files": inventory})
    atomic_json(run_root / "terminal.json", {
        "schema": "ngn-counter-policy-fixed-gate-terminal-v1", "state": "COMPLETE", "decision": decision,
        "decision_sha256": sha256(decision_path), "inventory_sha256": sha256(inventory_path),
        "no_extension_after_observation": True,
    })


if __name__ == "__main__":
    main()
