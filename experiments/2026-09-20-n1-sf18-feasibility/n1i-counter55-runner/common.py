#!/usr/bin/env python3
"""Shared fail-closed utilities for the candidate match runner."""
from __future__ import annotations

import datetime
import hashlib
import json
import os
import signal
import subprocess
import sys
import time
from pathlib import Path
from typing import Any


class CandidateMatchError(RuntimeError):
    pass


class RunnerTermination(BaseException):
    def __init__(self, signum: int) -> None:
        self.signum = signum
        super().__init__(f"received {signal.Signals(signum).name}")


def install_termination_handlers() -> dict[int, Any]:
    previous = {signum: signal.getsignal(signum) for signum in (signal.SIGTERM, signal.SIGINT)}

    def controlled(signum: int, _frame: Any) -> None:
        raise RunnerTermination(signum)

    for signum in previous:
        signal.signal(signum, controlled)
    return previous


def restore_signal_handlers(previous: dict[int, Any]) -> None:
    for signum, handler in previous.items():
        signal.signal(signum, handler)


def utc_now() -> str:
    return datetime.datetime.now(datetime.timezone.utc).isoformat()


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for block in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def atomic_json(path: Path, value: Any) -> None:
    temporary = path.with_suffix(path.suffix + ".tmp")
    temporary.write_text(json.dumps(value, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    os.replace(temporary, path)


def require_wsl() -> None:
    if not sys.platform.startswith("linux"):
        raise CandidateMatchError(f"WSL Linux required, got {sys.platform}")
    markers = " ".join(
        path.read_text(errors="replace").lower()
        for path in (Path("/proc/sys/kernel/osrelease"), Path("/proc/version"))
        if path.exists()
    )
    if "microsoft" not in markers and "wsl" not in markers:
        raise CandidateMatchError("WSL kernel marker absent")


def cpus_allowed_list(pid: int | str = "self") -> str:
    status = Path(f"/proc/{pid}/status").read_text(encoding="utf-8")
    for line in status.splitlines():
        if line.startswith("Cpus_allowed_list:"):
            return line.split(":", 1)[1].strip()
    raise CandidateMatchError(f"Cpus_allowed_list absent for pid {pid}")


def verified(path: Path, expected: str, executable: bool = False) -> Path:
    resolved = path.resolve(strict=True)
    if not resolved.is_file() or sha256(resolved) != expected.lower():
        raise CandidateMatchError(f"identity check failed: {resolved}")
    if executable and not os.access(resolved, os.X_OK):
        raise CandidateMatchError(f"not executable: {resolved}")
    return resolved


def safe_environment() -> dict[str, str]:
    return {"PATH": "/usr/bin:/bin", "LANG": "C", "LC_ALL": "C", "TZ": "UTC"}


def supervisor_command(
    supervisor: Path,
    stage: str,
    stage_dir: Path,
    limit_seconds: float,
    memory_limit_kib: int,
    cpu_list: str,
    command: list[str],
    term_grace_seconds: float,
    sample_interval_seconds: float,
) -> list[str]:
    stage_dir.mkdir(parents=True, exist_ok=False)
    (stage_dir / "command.json").write_text(json.dumps(command, indent=2) + "\n")
    return [
        sys.executable,
        str(supervisor),
        "--label",
        stage,
        "--limit-seconds",
        str(limit_seconds),
        "--term-grace-seconds",
        str(term_grace_seconds),
        "--sample-interval-seconds",
        str(sample_interval_seconds),
        "--memory-limit-kib",
        str(memory_limit_kib),
        "--cpu-list",
        cpu_list,
        "--stdout",
        str(stage_dir / "stdout"),
        "--stderr",
        str(stage_dir / "stderr"),
        "--time-output",
        str(stage_dir / "time.txt"),
        "--samples",
        str(stage_dir / "process-tree.jsonl"),
        "--receipt",
        str(stage_dir / "supervisor-receipt.json"),
        "--",
        *command,
    ]


def validate_supervisor_receipt(path: Path) -> dict[str, Any]:
    try:
        receipt = json.loads(path.read_text(encoding="utf-8"))
    except Exception as error:
        raise CandidateMatchError(f"invalid supervisor receipt {path}: {error}") from error
    failures = {
        "state": receipt.get("state") != "COMPLETE",
        "supervisor_returncode": receipt.get("supervisor_returncode") != 0,
        "command_returncode": receipt.get("command_returncode") != 0,
        "valid_samples": not isinstance(receipt.get("valid_samples"), int)
        or receipt.get("valid_samples", 0) <= 0,
        "surviving_processes": bool(receipt.get("surviving_processes")),
        "orphan_detected": receipt.get("orphan_detected") is not False,
        "timed_out": receipt.get("timed_out") is not False,
        "monitor_error": receipt.get("monitor_error") is not None,
        "memory_limit_exceeded": receipt.get("memory_limit_exceeded") is not False,
    }
    failed = [name for name, value in failures.items() if value]
    if failed:
        raise CandidateMatchError(f"supervisor stage failed {path}: {failed}")
    return receipt


def _proc_parent_group(pid: int) -> tuple[int, int]:
    raw = Path(f"/proc/{pid}/stat").read_text(encoding="utf-8")
    close = raw.rfind(")")
    fields = raw[close + 2 :].split()
    return int(fields[1]), int(fields[2])


def _descendant_snapshot(root_pid: int) -> dict[int, int]:
    rows: dict[int, tuple[int, int]] = {}
    for entry in Path("/proc").iterdir():
        if not entry.name.isdigit():
            continue
        try:
            rows[int(entry.name)] = _proc_parent_group(int(entry.name))
        except (FileNotFoundError, ProcessLookupError, PermissionError, ValueError, IndexError):
            continue
    children: dict[int, list[int]] = {}
    for pid, (ppid, _pgid) in rows.items():
        children.setdefault(ppid, []).append(pid)
    descendants: dict[int, int] = {}
    pending = [root_pid]
    while pending:
        parent = pending.pop()
        for child in children.get(parent, []):
            if child not in descendants:
                descendants[child] = rows[child][1]
                pending.append(child)
    if root_pid in rows:
        descendants[root_pid] = rows[root_pid][1]
    return descendants


class ActiveSupervisor:
    """Run one accepted supervisor and retain its process tree until validation."""

    def __init__(self) -> None:
        self.process: subprocess.Popen[str] | None = None
        self.pgid: int | None = None
        self.last_cleanup: dict[str, Any] | None = None

    def run(self, argv: list[str], cwd: Path, env: dict[str, str]) -> int:
        if self.process is not None:
            raise CandidateMatchError("supervisor already active")
        self.process = subprocess.Popen(argv, cwd=cwd, env=env, text=True, start_new_session=True)
        self.pgid = os.getpgid(self.process.pid)
        return self.process.wait()

    def clear(self) -> None:
        self.process = None
        self.pgid = None

    def terminate(self, grace_seconds: float = 3.0) -> dict[str, Any]:
        process, supervisor_pgid = self.process, self.pgid
        if process is None or supervisor_pgid is None:
            report = {"attempted": False, "captured_pids": [], "captured_pgids": [], "surviving_pids": []}
            self.last_cleanup = report
            return report
        captured = _descendant_snapshot(process.pid)
        captured.setdefault(process.pid, supervisor_pgid)
        own_group = os.getpgrp()
        groups = sorted({pgid for pgid in captured.values() if pgid > 0 and pgid != own_group}, reverse=True)
        for pgid in groups:
            try:
                os.killpg(pgid, signal.SIGTERM)
            except ProcessLookupError:
                pass
        deadline = time.monotonic() + grace_seconds
        while time.monotonic() < deadline:
            if not any(Path(f"/proc/{pid}").exists() for pid in captured):
                break
            time.sleep(0.05)
        survivors = [pid for pid in captured if Path(f"/proc/{pid}").exists()]
        survivor_groups = sorted({captured[pid] for pid in survivors if captured[pid] > 0 and captured[pid] != own_group}, reverse=True)
        for pgid in survivor_groups:
            try:
                os.killpg(pgid, signal.SIGKILL)
            except ProcessLookupError:
                pass
        for pid in survivors:
            try:
                os.kill(pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
        try:
            process.wait(timeout=grace_seconds)
        except subprocess.TimeoutExpired:
            pass
        final_survivors = [pid for pid in captured if Path(f"/proc/{pid}").exists()]
        report = {
            "attempted": True,
            "supervisor_pid": process.pid,
            "supervisor_pgid": supervisor_pgid,
            "captured_pids": sorted(captured),
            "captured_pgids": groups,
            "sigkill_pgids": survivor_groups,
            "surviving_pids": final_survivors,
        }
        self.last_cleanup = report
        return report
