#!/usr/bin/env python3
"""Run deterministic terminal and clock controls against a pinned fastchess."""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import signal
import shutil
import subprocess
import sys
from pathlib import Path

HERE = Path(__file__).resolve().parent


def require_wsl() -> None:
    if sys.platform != "linux":
        raise SystemExit(f"WSL Linux required, got {sys.platform}")
    markers = []
    for path in (Path("/proc/sys/kernel/osrelease"), Path("/proc/version")):
        if path.exists():
            markers.append(path.read_text(errors="replace").lower())
    if not any("microsoft" in marker or "wsl" in marker for marker in markers):
        raise SystemExit("WSL kernel marker absent")


def digest(path: Path) -> str:
    value = hashlib.sha256()
    with path.open("rb") as handle:
        for block in iter(lambda: handle.read(1024 * 1024), b""):
            value.update(block)
    return value.hexdigest()


def pinned(path: Path, expected: str, executable: bool = False) -> Path:
    path = path.resolve(strict=True)
    if not path.is_file() or (executable and not os.access(path, os.X_OK)):
        raise SystemExit(f"invalid pinned file: {path}")
    actual = digest(path)
    if actual != expected.lower():
        raise SystemExit(f"SHA-256 mismatch for {path}: {actual} != {expected}")
    return path


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--fastchess", type=Path, required=True)
    parser.add_argument("--fastchess-sha256", required=True)
    parser.add_argument("--stockfish", type=Path, required=True)
    parser.add_argument("--stockfish-sha256", required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    require_wsl()
    fastchess = pinned(args.fastchess, args.fastchess_sha256, True)
    stockfish = pinned(args.stockfish, args.stockfish_sha256, True)
    output = args.output
    output.mkdir(parents=True, exist_ok=False)
    source = output / "source"
    source.mkdir()
    source_names = [
        "run_fastchess_fixtures.py", "audit_fastchess_fixtures.py",
        "mutation_check_fastchess_audit.py", "uci_fixture_engine.py", "cases.json",
    ]
    for name in source_names:
        shutil.copy2(HERE / name, source / name)
    engine = source / "uci_fixture_engine.py"
    cases_path = source / "cases.json"
    cases = json.loads(cases_path.read_text())
    if cases.get("schema") != 1:
        raise SystemExit("unsupported fixture schema")
    manifest = {
        "status": "RUNNING",
        "fastchess": {"path": str(fastchess), "sha256": digest(fastchess)},
        "stockfish": {"path": str(stockfish), "sha256": digest(stockfish)},
        "frozen_source_sha256": {name: digest(source / name) for name in source_names},
        "cases": [],
    }
    (output / "manifest.running.json").write_text(json.dumps(manifest, indent=2) + "\n")
    for case in cases["cases"]:
        case_dir = output / case["name"]
        case_dir.mkdir()
        white_args = (
            f"--id={case['name']}-white --moves={','.join(case['white_moves'])} "
            f"--delay-ms={case.get('white_delay_ms', 0)} --score-cp=0 "
            f"--transcript={case_dir / 'white-uci.jsonl'}"
        )
        black_args = (
            f"--id={case['name']}-black --moves={','.join(case['black_moves'])} "
            f"--delay-ms={case.get('black_delay_ms', 0)} --score-cp=0 "
            f"--transcript={case_dir / 'black-uci.jsonl'}"
        )
        command = [
            str(fastchess),
            "-engine", f"cmd={engine}", "name=FixtureWhite", f"args={white_args}",
            "-engine", f"cmd={engine}", "name=FixtureBlack", f"args={black_args}",
            "-openings", f"file={case_dir / 'opening.epd'}", "format=epd", "order=sequential",
            "-each", f"tc={case['tc']}", "timemargin=0",
            "-rounds", "1", "-games", "1", "-noswap", "-concurrency", "1", "-srand", "424242",
            "-pgnout", f"file={case_dir / 'games.pgn'}", "notation=uci", "append=false",
            "nodes=true", "timeleft=true", "latency=true", "pv=true",
            "-epdout", f"file={case_dir / 'final.epd'}", "append=false",
            "-log", f"file={case_dir / 'fastchess.log'}", "level=trace", "engine=true",
            "realtime=false", "append=false",
            *case["extra_args"],
        ]
        (case_dir / "opening.epd").write_text(case["fen"] + "\n")
        (case_dir / "command.json").write_text(json.dumps(command, indent=2) + "\n")
        with (case_dir / "stdout.txt").open("x") as stdout, (case_dir / "stderr.txt").open("x") as stderr:
            process = subprocess.Popen(command, cwd=case_dir, stdout=stdout, stderr=stderr, start_new_session=True)
            try:
                returncode = process.wait(timeout=30)
            except subprocess.TimeoutExpired:
                os.killpg(process.pid, signal.SIGKILL)
                process.wait()
                (case_dir / "exit.txt").write_text("TIMEOUT\n")
                raise RuntimeError(f"fixture {case['name']} timed out")
        (case_dir / "exit.txt").write_text(str(returncode) + "\n")
        if returncode != 0:
            raise RuntimeError(f"fixture {case['name']} exited {returncode}")
        manifest["cases"].append({"name": case["name"], "returncode": returncode})
        (output / "manifest.running.json").write_text(json.dumps(manifest, indent=2) + "\n")
    audit = source / "audit_fastchess_fixtures.py"
    audit_command = [
        sys.executable, str(audit), "--cases", str(cases_path), "--results", str(output),
        "--stockfish", str(stockfish), "--stockfish-sha256", args.stockfish_sha256,
        "--output", str(output / "audit.json"),
    ]
    (output / "audit.command.json").write_text(json.dumps(audit_command, indent=2) + "\n")
    with (output / "audit.stdout").open("x") as stdout, (output / "audit.stderr").open("x") as stderr:
        completed = subprocess.run(audit_command, stdout=stdout, stderr=stderr, timeout=120, check=False)
    (output / "audit.exit").write_text(str(completed.returncode) + "\n")
    if completed.returncode:
        raise RuntimeError("independent fixture audit failed")
    mutation_tool = source / "mutation_check_fastchess_audit.py"
    mutation_command = [
        sys.executable, str(mutation_tool), "--auditor", str(audit),
        "--cases", str(cases_path), "--results", str(output),
        "--stockfish", str(stockfish), "--stockfish-sha256", args.stockfish_sha256,
        "--output", str(output / "mutations.json"),
    ]
    (output / "mutations.command.json").write_text(json.dumps(mutation_command, indent=2) + "\n")
    with (output / "mutations.stdout").open("x") as stdout, (output / "mutations.stderr").open("x") as stderr:
        completed = subprocess.run(mutation_command, stdout=stdout, stderr=stderr, timeout=900, check=False)
    (output / "mutations.exit").write_text(str(completed.returncode) + "\n")
    if completed.returncode:
        raise RuntimeError("fixture auditor mutation checks failed")
    manifest["mutation_checker_sha256"] = digest(mutation_tool)
    manifest["mutation_report_sha256"] = digest(output / "mutations.json")
    manifest["status"] = "PASS"
    manifest["audit_sha256"] = digest(output / "audit.json")
    manifest["artifact_sha256"] = {
        str(path.relative_to(output)): digest(path)
        for path in sorted(output.rglob("*")) if path.is_file() and path.name != "manifest.running.json"
    }
    (output / "manifest.json").write_text(json.dumps(manifest, indent=2) + "\n")
    (output / "DONE").write_text("DONE_EXIT_0\n")
    print(json.dumps({"status": "PASS", "cases": len(cases["cases"]), "output": str(output)}, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
