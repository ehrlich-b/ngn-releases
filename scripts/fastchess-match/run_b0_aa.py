#!/usr/bin/env python3
"""Fail-closed launcher for the reviewed, fixed 200-game B0 A/A control."""

from __future__ import annotations

import argparse
import datetime
import hashlib
import json
import os
import re
import shutil
import subprocess
import sys
from pathlib import Path


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for block in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def require_wsl() -> None:
    if not sys.platform.startswith("linux"):
        raise SystemExit(f"WSL Linux required, got {sys.platform}")
    markers = " ".join(
        path.read_text(errors="replace").lower()
        for path in (Path("/proc/sys/kernel/osrelease"), Path("/proc/version"))
        if path.exists()
    )
    if "microsoft" not in markers and "wsl" not in markers:
        raise SystemExit("WSL kernel marker absent")


def verified(path: Path, expected: str, executable: bool = False) -> Path:
    resolved = path.resolve(strict=True)
    if not resolved.is_file() or sha256(resolved) != expected.lower():
        raise RuntimeError(f"identity check failed: {resolved}")
    if executable and not os.access(resolved, os.X_OK):
        raise RuntimeError(f"not executable: {resolved}")
    return resolved


def live_workers() -> list[str]:
    completed = subprocess.run(
        ["ps", "-eo", "pid=,comm=,args="], text=True, capture_output=True,
        timeout=30, check=True,
    )
    pattern = re.compile(r"^(?:fastchess|ngn-b0|stockfish18)$")
    rows = []
    for line in completed.stdout.splitlines():
        fields = line.split(None, 2)
        if len(fields) >= 2 and pattern.fullmatch(Path(fields[1]).name):
            rows.append(line.strip())
    return rows


def write_state(output: Path, state: str, **extra) -> None:
    value = {
        "state": state, "pid": os.getpid(),
        "time_utc": datetime.datetime.now(datetime.timezone.utc).isoformat(), **extra,
    }
    (output / "STATE.json").write_text(json.dumps(value, indent=2) + "\n")


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--manifest", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--approval-token", required=True)
    args = parser.parse_args()
    require_wsl()
    manifest_path = args.manifest.resolve(strict=True)
    manifest = json.loads(manifest_path.read_text())
    approval = manifest.get("approval", {})
    if manifest.get("status") != "APPROVED_TO_RUN" or approval.get("authority") != "root":
        raise SystemExit("manifest remains held; root approval state is absent")
    if not approval.get("token") or args.approval_token != approval["token"]:
        raise SystemExit("approval token mismatch")
    inputs = manifest["inputs"]
    engine = verified(Path(inputs["engine"]["path"]), inputs["engine"]["sha256"], True)
    fastchess = verified(Path(inputs["fastchess"]["path"]), inputs["fastchess"]["sha256"], True)
    verified(Path(inputs["raw_opening_corpus"]["path"]), inputs["raw_opening_corpus"]["sha256"])
    opening_prefixes = verified(Path(inputs["opening_prefixes"]["path"]), inputs["opening_prefixes"]["sha256"])
    openings = verified(Path(inputs["openings_pgn"]["path"]), inputs["openings_pgn"]["sha256"])
    stockfish = verified(Path(inputs["stockfish"]["path"]), inputs["stockfish"]["sha256"], True)
    auditor = verified(Path(inputs["auditor"]["path"]), inputs["auditor"]["sha256"])
    converter = verified(Path(inputs["converter"]["path"]), inputs["converter"]["sha256"])
    launcher = verified(Path(inputs["launcher"]["path"]), inputs["launcher"]["sha256"])
    if launcher != Path(__file__).resolve():
        raise RuntimeError("launcher path differs from reviewed manifest")
    evidence = [verified(Path(item["path"]), item["sha256"]) for item in inputs["evidence"]]
    if openings.read_text().count('[Event "') != manifest["match"]["pairs"]:
        raise RuntimeError("converted PGN opening count mismatch")
    allowed = Path("/proc/self/status").read_text()
    allowed_line = next(line.split(":", 1)[1].strip() for line in allowed.splitlines()
                        if line.startswith("Cpus_allowed_list:"))
    if allowed_line != manifest["resources"]["required_cpus_allowed_list"]:
        raise RuntimeError(f"cpuset changed: {allowed_line}")
    selected_cpus = manifest["resources"]["selected_logical_cpus"]
    core_ids = [Path(f"/sys/devices/system/cpu/cpu{cpu}/topology/core_id").read_text().strip()
                for cpu in selected_cpus]
    if len(set(core_ids)) != len(core_ids):
        raise RuntimeError(f"selected CPUs are not distinct physical cores: {core_ids}")
    workers = live_workers()
    if workers:
        raise RuntimeError("pre-run chess workers exist: " + "; ".join(workers))

    output = args.output.resolve()
    output.mkdir(parents=True, exist_ok=False)
    write_state(output, "PREPARING")
    source = output / "source"
    frozen = output / "inputs"
    source.mkdir()
    frozen.mkdir()
    source_map = []
    for path in (launcher, auditor, converter, manifest_path):
        destination = source / path.name
        shutil.copy2(path, destination)
        source_map.append({"original": str(path), "copy": str(destination.relative_to(output)),
                           "sha256": sha256(destination)})
    evidence_dir = source / "evidence"
    evidence_dir.mkdir()
    for index, path in enumerate(evidence):
        destination = evidence_dir / f"{index:02d}-{path.parent.name}-{path.name}"
        shutil.copy2(path, destination)
        source_map.append({"original": str(path), "copy": str(destination.relative_to(output)),
                           "sha256": sha256(destination)})
    (source / "source-map.json").write_text(json.dumps(source_map, indent=2) + "\n")
    frozen_engine = frozen / "ngn-b0"
    frozen_fastchess = frozen / "fastchess"
    frozen_prefixes = frozen / "opening-prefixes.txt"
    frozen_openings = frozen / "openings-100.pgn"
    engine_work = output / "engine-cwd"
    engine_a_work = engine_work / "a"
    engine_b_work = engine_work / "b"
    engine_a_work.mkdir(parents=True)
    engine_b_work.mkdir()
    shutil.copy2(engine, frozen_engine)
    shutil.copy2(fastchess, frozen_fastchess)
    shutil.copy2(opening_prefixes, frozen_prefixes)
    shutil.copy2(openings, frozen_openings)
    for copied, expected in (
        (frozen_engine, inputs["engine"]["sha256"]),
        (frozen_fastchess, inputs["fastchess"]["sha256"]),
        (frozen_prefixes, inputs["opening_prefixes"]["sha256"]),
        (frozen_openings, inputs["openings_pgn"]["sha256"]),
    ):
        verified(copied, expected, copied in {frozen_engine, frozen_fastchess})

    match = manifest["match"]
    pgn = output / "games.pgn"
    final_epd = output / "final.epd"
    log = output / "fastchess.log"
    common_options = ["option.Threads=1", "option.Hash=64", "option.Move Overhead=100"]
    default_book_paths = ["book.bin", "opening.bin", "openings.bin", "book/book.bin", "book/opening.bin"]
    book_path_receipt = {
        "engine_a_cwd": str(engine_a_work), "engine_b_cwd": str(engine_b_work),
        "paths": default_book_paths,
        "pre_run_present": {
            "a": [name for name in default_book_paths if (engine_a_work / name).exists()],
            "b": [name for name in default_book_paths if (engine_b_work / name).exists()],
        },
    }
    if book_path_receipt["pre_run_present"]["a"] or book_path_receipt["pre_run_present"]["b"]:
        raise RuntimeError("external default book path exists in an engine cwd")
    sanitized_environment = dict(os.environ)
    removed_environment = sorted(name for name in sanitized_environment if name.startswith("NGN_"))
    for name in removed_environment:
        sanitized_environment.pop(name)
    (output / "environment-sanitization.json").write_text(json.dumps({
        "removed_variable_names": removed_environment,
        "policy": "All inherited NGN_* variables removed; compiled B0 defaults and discarded debug logging apply.",
    }, indent=2) + "\n")
    command = [
        str(frozen_fastchess),
        "-engine", f"cmd={frozen_engine}", f"name={match['engine_a']}", f"dir={engine_a_work}", *common_options,
        "-engine", f"cmd={frozen_engine}", f"name={match['engine_b']}", f"dir={engine_b_work}", *common_options,
        "-openings", f"file={frozen_openings}", "format=pgn", "order=sequential", "start=1", "plies=6",
        "-each", f"tc={match['time_control']}", "timemargin=0",
        "-rounds", str(match["pairs"]), "-games", "2", "-repeat",
        "-concurrency", str(match["concurrency"]),
        "-use-affinity", ",".join(map(str, selected_cpus)),
        "-srand", str(match["seed"]), "-strict", "-show-latency",
        "-pgnout", f"file={pgn}", "notation=uci", "append=false", "nodes=true",
        "seldepth=true", "nps=true", "hashfull=true", "timeleft=true", "latency=true", "pv=true",
        "-epdout", f"file={final_epd}", "append=false",
        "-log", f"file={log}", "level=trace", "engine=true", "realtime=false", "append=false",
    ]
    (output / "match.command.json").write_text(json.dumps(command, indent=2) + "\n")
    write_state(output, "MATCH_RUNNING", command_sha256=sha256(output / "match.command.json"))
    with (output / "match.stdout").open("x") as stdout, (output / "match.stderr").open("x") as stderr:
        completed = subprocess.run(command, cwd=output, stdout=stdout, stderr=stderr,
                                   env=sanitized_environment, check=False, start_new_session=True)
    (output / "match.exit").write_text(str(completed.returncode) + "\n")
    if completed.returncode != 0 or (output / "match.stderr").stat().st_size:
        write_state(output, "MATCH_FAILED", returncode=completed.returncode)
        raise RuntimeError("fastchess failed or emitted stderr")
    book_path_receipt["post_run_present"] = {
        "a": [name for name in default_book_paths if (engine_a_work / name).exists()],
        "b": [name for name in default_book_paths if (engine_b_work / name).exists()],
    }
    (output / "external-book-paths.json").write_text(json.dumps(book_path_receipt, indent=2) + "\n")
    if book_path_receipt["post_run_present"]["a"] or book_path_receipt["post_run_present"]["b"]:
        raise RuntimeError("external default book path appeared in an engine cwd")
    write_state(output, "AUDIT_RUNNING")
    frozen_auditor = source / auditor.name
    audit_command = [
        sys.executable, str(frozen_auditor), "--pgn", str(pgn), "--final-epd", str(final_epd),
        "--opening-prefixes", str(frozen_prefixes),
        "--opening-prefixes-sha256", inputs["opening_prefixes"]["sha256"],
        "--opening-pgn", str(frozen_openings), "--opening-pgn-sha256", inputs["openings_pgn"]["sha256"],
        "--stockfish", str(stockfish), "--stockfish-sha256", inputs["stockfish"]["sha256"],
        "--engine-a", match["engine_a"], "--engine-b", match["engine_b"],
        "--games", str(match["games"]), "--pairs", str(match["pairs"]),
        "--output", str(output / "audit.json"),
    ]
    (output / "audit.command.json").write_text(json.dumps(audit_command, indent=2) + "\n")
    with (output / "audit.stdout").open("x") as stdout, (output / "audit.stderr").open("x") as stderr:
        audited = subprocess.run(audit_command, cwd=output, stdout=stdout, stderr=stderr,
                                 timeout=3600, check=False, start_new_session=True)
    (output / "audit.exit").write_text(str(audited.returncode) + "\n")
    if audited.returncode != 0 or (output / "audit.stderr").stat().st_size:
        write_state(output, "AUDIT_FAILED", returncode=audited.returncode)
        raise RuntimeError("independent match audit failed or emitted stderr")
    audit_report = json.loads((output / "audit.json").read_text())
    sanity = manifest["acceptance"]["aa_score_rate"]
    score_rate = audit_report.get("engine_a_score_rate")
    if not isinstance(score_rate, (int, float)) or not sanity["min"] <= score_rate <= sanity["max"]:
        write_state(output, "AA_SANITY_FAILED", engine_a_score_rate=score_rate)
        raise RuntimeError(f"prospective A/A score-rate gate failed: {score_rate}")
    workers = live_workers()
    (output / "terminal-processes.txt").write_text("\n".join(workers) + ("\n" if workers else ""))
    if workers:
        write_state(output, "PROCESS_LEAK", workers=workers)
        raise RuntimeError("post-run chess workers remain")
    artifacts = {
        str(path.relative_to(output)): sha256(path)
        for path in sorted(output.rglob("*"))
        if path.is_file() and path.name not in {"STATE.json", "manifest.final.json", "DONE"}
    }
    final = {
        "status": "PASS", "reviewed_manifest_sha256": sha256(source / manifest_path.name),
        "artifacts": artifacts,
    }
    (output / "manifest.final.json").write_text(json.dumps(final, indent=2) + "\n")
    write_state(output, "PASS")
    (output / "DONE").write_text("DONE_EXIT_0\n")
    print(json.dumps({"status": "PASS", "output": str(output)}, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
