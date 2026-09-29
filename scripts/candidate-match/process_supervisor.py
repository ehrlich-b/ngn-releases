#!/usr/bin/env python3
"""Bound one stage, own its process group, sample descendants, and retain termination evidence."""
import argparse
import json
import os
from pathlib import Path
import signal
import subprocess
import sys
import time


def atomic_json(path: Path, value: dict) -> None:
    temporary = path.with_suffix(path.suffix + ".tmp")
    temporary.write_text(json.dumps(value, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    os.replace(temporary, path)


def normalized_returncode(value):
    if value is None:
        return None
    return 128 - value if value < 0 else value

parser = argparse.ArgumentParser()
parser.add_argument("--label", required=True)
parser.add_argument("--limit-seconds", type=float, required=True)
parser.add_argument("--term-grace-seconds", type=float, default=3.0)
parser.add_argument("--sample-interval-seconds", type=float, default=0.25)
parser.add_argument("--memory-limit-kib", type=int, required=True)
parser.add_argument("--cpu-list", required=True)
parser.add_argument("--stdout", type=Path, required=True)
parser.add_argument("--stderr", type=Path, required=True)
parser.add_argument("--time-output", type=Path, required=True)
parser.add_argument("--samples", type=Path, required=True)
parser.add_argument("--receipt", type=Path, required=True)
parser.add_argument("--ps-command", default="/usr/bin/ps")
parser.add_argument("command", nargs=argparse.REMAINDER)
args = parser.parse_args()
if args.command and args.command[0] == "--":
    args.command = args.command[1:]
if not args.command or args.limit_seconds <= 0 or args.term_grace_seconds <= 0 \
        or args.sample_interval_seconds <= 0 or args.memory_limit_kib <= 0:
    parser.error("command and positive limits are required")

started_wall = time.time()
started = time.monotonic()
termination_reason = None
term_sent_at = None
term_sent = False
kill_sent = False
kill_sent_at = None
monitor_error = None
orphan_detected = False
memory_exceeded = False
timed_out = False
interrupted_signal = None
valid_samples = 0
peak_tree_rss_kib = 0
peak_processes = 0
known_descendants = set()
last_live = []
process = None
pgid = None

state = {
    "stage": args.label,
    "state": "STARTING",
    "command": args.command,
    "started_unix_time": started_wall,
    "limit_seconds": args.limit_seconds,
    "term_grace_seconds": args.term_grace_seconds,
    "sample_interval_seconds": args.sample_interval_seconds,
    "memory_limit_kib": args.memory_limit_kib,
    "cpu_list": args.cpu_list,
}
atomic_json(args.receipt, state)


def requested(signum, _frame):
    global interrupted_signal
    interrupted_signal = signum

signal.signal(signal.SIGINT, requested)
signal.signal(signal.SIGTERM, requested)


def scan_processes():
    result = subprocess.run(
        [args.ps_command, "-e", "-o", "pid=,ppid=,pgid=,rss=,comm="],
        text=True,
        capture_output=True,
        timeout=2,
        check=False,
    )
    if result.returncode != 0:
        raise RuntimeError(f"ps failed rc={result.returncode} stderr={result.stderr!r}")
    rows = {}
    children = {}
    for line in result.stdout.splitlines():
        fields = line.split(None, 4)
        if len(fields) != 5:
            continue
        try:
            pid, ppid, row_pgid, rss = map(int, fields[:4])
        except ValueError:
            continue
        rows[pid] = {"pid": pid, "ppid": ppid, "pgid": row_pgid, "rss_kib": rss, "command": fields[4]}
        children.setdefault(ppid, []).append(pid)
    tree = set()
    frontier = [process.pid]
    while frontier:
        pid = frontier.pop()
        if pid in tree:
            continue
        if pid in rows:
            tree.add(pid)
            frontier.extend(children.get(pid, ()))
    same_group = {pid for pid, row in rows.items() if row["pgid"] == pgid}
    known_descendants.update(tree | same_group)
    tracked = (tree | same_group | known_descendants).intersection(rows)
    return rows, tracked


def final_procfs_survivors():
    survivors = []
    for entry in Path("/proc").iterdir():
        if not entry.name.isdigit():
            continue
        pid = int(entry.name)
        try:
            row_pgid = os.getpgid(pid)
        except (ProcessLookupError, PermissionError):
            continue
        if row_pgid != pgid and pid not in known_descendants:
            continue
        rss_kib = None
        command = None
        try:
            command = (entry / "comm").read_text().strip()
        except (FileNotFoundError, PermissionError):
            pass
        try:
            for line in (entry / "status").read_text().splitlines():
                if line.startswith("VmRSS:"):
                    rss_kib = int(line.split()[1])
                    break
        except (FileNotFoundError, PermissionError, ValueError):
            pass
        survivors.append({"pid": pid, "pgid": row_pgid, "rss_kib": rss_kib, "command": command})
    return sorted(survivors, key=lambda row: row["pid"])

def signal_scope(sig):
    try:
        os.killpg(pgid, sig)
    except ProcessLookupError:
        pass
    for pid in list(known_descendants):
        if pid == os.getpid():
            continue
        try:
            os.kill(pid, sig)
        except ProcessLookupError:
            pass
        except PermissionError:
            pass

full_command = [
    "/usr/bin/time", "-v", "-o", str(args.time_output),
    "taskset", "-c", args.cpu_list,
    *args.command,
]
args.stdout.parent.mkdir(parents=True, exist_ok=True)
with args.stdout.open("wb") as stdout, args.stderr.open("wb") as stderr, args.samples.open("w", encoding="utf-8") as samples:
    process = subprocess.Popen(full_command, stdout=stdout, stderr=stderr, start_new_session=True)
    pgid = os.getpgid(process.pid)
    state.update({"state": "RUNNING", "supervised_root_pid": process.pid, "process_group_id": pgid, "full_command": full_command})
    atomic_json(args.receipt, state)
    absolute_deadline = started + args.limit_seconds + 2 * args.term_grace_seconds + 10
    while True:
        leader_rc = process.poll()
        now = time.monotonic()
        try:
            rows, tracked = scan_processes()
            last_live = [rows[pid] for pid in sorted(tracked)]
            if tracked:
                tree_rss = sum(rows[pid]["rss_kib"] for pid in tracked)
                valid_samples += 1
                peak_tree_rss_kib = max(peak_tree_rss_kib, tree_rss)
                peak_processes = max(peak_processes, len(tracked))
                samples.write(json.dumps({
                    "elapsed_seconds": now - started,
                    "processes": last_live,
                    "tree_rss_kib": tree_rss,
                }, sort_keys=True) + "\n")
                samples.flush()
                if tree_rss > args.memory_limit_kib and termination_reason is None:
                    memory_exceeded = True
                    termination_reason = "sampled-process-tree-memory-limit"
            if leader_rc is not None and tracked and termination_reason is None:
                orphan_detected = True
                termination_reason = "descendants-survived-leader"
        except Exception as error:
            tracked = set()
            if monitor_error is None:
                monitor_error = f"{type(error).__name__}: {error}"
            if termination_reason is None:
                termination_reason = "process-monitor-failure"
        if interrupted_signal is not None and termination_reason is None:
            termination_reason = f"supervisor-signal-{interrupted_signal}"
        if now - started >= args.limit_seconds and leader_rc is None and termination_reason is None:
            timed_out = True
            termination_reason = "stage-timeout"
        if termination_reason is not None and not term_sent:
            signal_scope(signal.SIGTERM)
            term_sent = True
            term_sent_at = now
        if term_sent and not kill_sent and now - term_sent_at >= args.term_grace_seconds:
            try:
                _, still_tracked = scan_processes()
            except Exception:
                still_tracked = set(known_descendants)
            if monitor_error is not None or process.poll() is None or still_tracked:
                signal_scope(signal.SIGKILL)
                kill_sent = True
                kill_sent_at = now
        if process.poll() is not None and not tracked and monitor_error is None:
            break
        if monitor_error is not None and kill_sent and now - kill_sent_at >= args.term_grace_seconds:
            break
        if now >= absolute_deadline:
            if termination_reason is None:
                termination_reason = "supervisor-absolute-deadline"
            signal_scope(signal.SIGKILL)
            kill_sent = True
            kill_sent_at = now
            break
        time.sleep(args.sample_interval_seconds)
    try:
        raw_rc = process.wait(timeout=args.term_grace_seconds)
    except subprocess.TimeoutExpired:
        signal_scope(signal.SIGKILL)
        kill_sent = True
        kill_sent_at = time.monotonic()
        try:
            raw_rc = process.wait(timeout=args.term_grace_seconds)
        except subprocess.TimeoutExpired:
            raw_rc = None
    try:
        survivor_deadline = time.monotonic() + args.term_grace_seconds
        surviving_rows = final_procfs_survivors()
        while surviving_rows and time.monotonic() < survivor_deadline:
            signal_scope(signal.SIGKILL)
            kill_sent = True
            kill_sent_at = time.monotonic()
            time.sleep(min(0.05, args.sample_interval_seconds))
            surviving_rows = final_procfs_survivors()
    except Exception as error:
        surviving_rows = last_live
        if monitor_error is None:
            monitor_error = f"final procfs {type(error).__name__}: {error}"

command_rc = normalized_returncode(raw_rc)
if valid_samples == 0 and monitor_error is None:
    monitor_error = "no valid process-tree sample"
pass_state = command_rc == 0 and termination_reason is None and monitor_error is None \
    and valid_samples > 0 and not surviving_rows
if timed_out:
    supervisor_rc = 124
elif command_rc not in (None, 0):
    supervisor_rc = command_rc if 0 < command_rc < 256 else 125
elif pass_state:
    supervisor_rc = 0
else:
    supervisor_rc = 125
receipt = {
    **state,
    "state": "COMPLETE" if pass_state else "FAILED",
    "ended_unix_time": time.time(),
    "elapsed_seconds": time.monotonic() - started,
    "command_returncode": command_rc,
    "supervisor_returncode": supervisor_rc,
    "termination_reason": termination_reason,
    "timed_out": timed_out,
    "memory_limit_exceeded": memory_exceeded,
    "orphan_detected": orphan_detected,
    "term_sent": term_sent,
    "sigkill_sent": kill_sent,
    "interrupted_signal": interrupted_signal,
    "monitor_error": monitor_error,
    "valid_samples": valid_samples,
    "peak_sampled_process_tree_rss_kib": peak_tree_rss_kib,
    "peak_sampled_process_count": peak_processes,
    "surviving_processes": surviving_rows,
    "final_survivor_check_scope": "procfs PGID plus every previously observed descendant PID",
    "sampling_scope": "leader process group plus observed descendants, including observed reparented descendants",
}
atomic_json(args.receipt, receipt)
raise SystemExit(supervisor_rc)
