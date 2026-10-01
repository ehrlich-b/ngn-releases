#!/usr/bin/env python3
"""Audit pinned fastchess trace and NGN crash logs for operational failures."""
from __future__ import annotations

import argparse
import json
import re
import sys
from collections import Counter, defaultdict
from pathlib import Path
from typing import Any

from common import CandidateMatchError, atomic_json, sha256, utc_now

TRACE_RE = re.compile(r"^\[(?P<level>[^]]+)\] \[[^]]+\] <(?P<thread>[^>]*)>\s*(?P<body>.*)$")
NONZERO_STATUS_RE = re.compile(r"terminated with status:\s*(-?[0-9]+)", re.IGNORECASE)
THREADS_RE = re.compile(r"^info string threads configured ([0-9]+) effective ([0-9]+)$")
M4C_CAPABILITY = "ngn-m4c-time-controlled-smp-v1"
COMBINED_CAPABILITY = "ngn-m4c-counter55-vs-hce-longclock-v1"
THREADED_CAPABILITIES = {M4C_CAPABILITY, COMBINED_CAPABILITY}
COUNTER55_CAPABILITY = "ngn-counter55-inprocess-one-worker-v1"
FATAL_PHRASES = (
    "illegal move", "illegal pv", "disconnected", "disconnecting", "connection stalled",
    "timed out", "timeout waiting", "engine crashed", "engine stall", "panic recovered",
    "see ngn_crashes.log", "failed to start/refresh", "failed to send", "doesn't have option",
    "invalid value for option", "failed to set option",
)


def crash_lines(path: Path, logged_path: Path, expected_processes: int) -> dict[str, Any]:
    if expected_processes < 0:
        raise CandidateMatchError("negative expected crash-log process count")
    if not path.is_file():
        if expected_processes == 0:
            return {"path": str(path), "sha256": None, "expected_processes": 0, "pairs": 0}
        raise CandidateMatchError(f"required NGN crash log absent: {path}")
    lines = path.read_text(encoding="utf-8", errors="replace").splitlines()
    init_re = re.compile(r"^\[NGN CRASH\] \d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}\.\d{6} crash_handler\.go:64: === NGN Crash Handler Initialized ===$")
    path_re = re.compile(r"^\[NGN CRASH\] \d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}\.\d{6} crash_handler\.go:65: Crash log: " + re.escape(str(logged_path)) + r"$")
    if len(lines) != expected_processes * 2:
        raise CandidateMatchError(f"{path}: crash-log lines={len(lines)} expected={expected_processes * 2}")
    for index in range(expected_processes):
        first, second = lines[2 * index : 2 * index + 2]
        if not init_re.fullmatch(first) or not path_re.fullmatch(second):
            raise CandidateMatchError(f"{path}: non-benign crash-log pair {index}: {first!r}, {second!r}")
    return {"path": str(path), "sha256": sha256(path), "expected_processes": expected_processes, "pairs": expected_processes}


def clock_go_violation(command: str) -> str | None:
    tokens = command.split()
    if not tokens or tokens[0] != "go":
        return "not a go command"
    arguments = tokens[1:]
    if len(arguments) % 2 != 0:
        return "expected key/value clock arguments"
    allowed = {"wtime", "btime", "winc", "binc", "movestogo"}
    values: dict[str, int] = {}
    for index in range(0, len(arguments), 2):
        name, raw = arguments[index:index + 2]
        if name not in allowed:
            return f"forbidden or unknown go mode {name}"
        if name in values:
            return f"duplicate go field {name}"
        if re.fullmatch(r"[0-9]+", raw) is None:
            return f"invalid nonnegative {name} value {raw!r}"
        values[name] = int(raw)
    if not {"wtime", "btime"}.issubset(values):
        return "wtime and btime are required"
    if "movestogo" in values and values["movestogo"] == 0:
        return "movestogo must be positive"
    return None


def audit_trace(lines: list[str], roles: list[dict[str, Any]]) -> dict[str, Any]:
    by_name = {role["display_name"]: role for role in roles}
    failures: list[str] = []
    refreshes: dict[tuple[str, str], list[str]] = {}
    refresh_widths: Counter[tuple[str, str]] = Counter()
    completed_sequences: list[dict[str, Any]] = []
    normal_exits: Counter[str] = Counter()
    canonical_errors: list[dict[str, Any]] = []
    engine_lines = 0
    warning_lines = 0
    probable_book_signatures: list[dict[str, Any]] = []
    pending_go: dict[str, dict[str, Any]] = {}
    width_events: list[dict[str, Any]] = []
    configured_threads = {
        name: int(next(item["value"] for item in role["resolved_options"] if item["name"] == "Threads"))
        for name, role in by_name.items()
    }
    book_signature = re.compile(r"^info depth 1 score cp 50 nodes 1 time 0 nps 0 pv \S+$")

    for line_number, line in enumerate(lines, 1):
        if not line:
            continue
        match = TRACE_RE.fullmatch(line)
        if not match:
            failures.append(f"line {line_number}: unrecognized trace syntax")
            continue
        level = match.group("level").strip()
        thread = match.group("thread").strip()
        body = match.group("body").strip()
        lower = body.lower()
        normalized_level = level.upper()
        if normalized_level == "WARN" or "warning;" in lower:
            warning_lines += 1
            failures.append(f"line {line_number}: warning: {body}")
        if normalized_level in {"ERROR", "FATAL", "CRITICAL"}:
            failures.append(f"line {line_number}: fatal log level {normalized_level}: {body}")
        for phrase in FATAL_PHRASES:
            if phrase in lower:
                failures.append(f"line {line_number}: operational phrase {phrase!r}: {body}")
                break
        status = NONZERO_STATUS_RE.search(body)
        if status and int(status.group(1)) != 0:
            failures.append(f"line {line_number}: nonzero engine status: {body}")

        prefix = "fastchess --- Refreshing engine "
        if body.startswith(prefix):
            name = body[len(prefix):]
            if name not in by_name:
                failures.append(f"line {line_number}: unknown refreshed role {name!r}")
            else:
                key = (thread, name)
                if key in refreshes:
                    failures.append(f"line {line_number}: nested refresh for {name}")
                refreshes[key] = []
                refresh_widths[key] = 0
            continue
        prefix = "fastchess --- Sending setoption to engine "
        if body.startswith(prefix):
            remainder = body[len(prefix):]
            names = [name for name in by_name if remainder.startswith(name + " ")]
            if len(names) != 1:
                failures.append(f"line {line_number}: cannot identify setoption role: {body}")
                continue
            name = names[0]
            key = (thread, name)
            if key not in refreshes:
                failures.append(f"line {line_number}: setoption outside refresh for {name}")
            else:
                refreshes[key].append(remainder[len(name) + 1:])
            continue
        prefix = "fastchess --- Engine "
        suffix = " refreshed."
        if body.startswith(prefix) and body.endswith(suffix):
            name = body[len(prefix):-len(suffix)]
            key = (thread, name)
            actual = refreshes.pop(key, None)
            configured_receipts = refresh_widths.pop(key, 0)
            if actual is None:
                failures.append(f"line {line_number}: refresh completion without start for {name}")
            elif name not in by_name:
                failures.append(f"line {line_number}: refresh completion unknown role {name}")
            else:
                expected = [f"{item['name']} {item['value']}" for item in by_name[name]["resolved_options"]]
                if actual != expected:
                    failures.append(f"line {line_number}: option sequence for {name} actual={actual!r} expected={expected!r}")
                if by_name[name]["capability_profile"] in THREADED_CAPABILITIES and configured_receipts != 1:
                    failures.append(
                        f"line {line_number}: configured Threads receipts={configured_receipts} for {name} expected=1"
                    )
                sequence = {"role": name, "thread": thread, "options": actual}
                if by_name[name]["capability_profile"] in THREADED_CAPABILITIES:
                    sequence["configured_threads_receipts"] = configured_receipts
                completed_sequences.append(sequence)
            continue

        for name, role in by_name.items():
            prefix = name + " <--- "
            if not body.startswith(prefix):
                continue
            command = body[len(prefix):].strip()
            if command == "go" or command.startswith("go "):
                if role["capability_profile"] in THREADED_CAPABILITIES:
                    violation = clock_go_violation(command)
                    if name in pending_go:
                        failures.append(f"line {line_number}: duplicate go before bestmove from {name}")
                    elif violation is not None:
                        failures.append(
                            f"line {line_number}: non-clock go is outside M4c time-controlled profile ({violation}): {command}"
                        )
                    else:
                        pending_go[name] = {"line": line_number, "width_lines": 0}
            break

        # Fastchess tags engine stdout/stderr inside the trace body.
        if " ---> " in body:
            actor, payload = body.split(" ---> ", 1)
            actor = actor.strip()
            payload = payload.strip()
            is_stderr = actor.startswith("<stderr>")
            name = actor[len("<stderr>"):].strip() if is_stderr else actor
            if name in by_name:
                engine_lines += 1
                if payload.startswith("info string error"):
                    canonical_errors.append({"line": line_number, "role": name, "payload": payload})
                    failures.append(f"line {line_number}: canonical engine error from {name}: {payload}")
                if not is_stderr and book_signature.fullmatch(payload):
                    probable_book_signatures.append({"line": line_number, "role": name, "payload": payload})
                    failures.append(f"line {line_number}: opening-book signature from OwnBook=false role {name}")
                width_match = THREADS_RE.fullmatch(payload) if not is_stderr else None
                if not is_stderr and payload.startswith("info string threads configured") and width_match is None:
                    failures.append(f"line {line_number}: malformed Threads width receipt from {name}: {payload}")
                if width_match is not None:
                    configured, effective = map(int, width_match.groups())
                    refresh_key = (thread, name)
                    phase = "configuration" if refresh_key in refreshes else "go"
                    event = {
                        "line": line_number, "role": name, "phase": phase,
                        "configured": configured, "effective": effective,
                    }
                    width_events.append(event)
                    if by_name[name]["capability_profile"] in THREADED_CAPABILITIES:
                        expected = configured_threads[name]
                        if configured != expected or effective != expected:
                            failures.append(
                                f"line {line_number}: Threads width {configured}/{effective} from {name} expected {expected}/{expected}"
                            )
                        if phase == "configuration":
                            refresh_widths[refresh_key] += 1
                            if refresh_widths[refresh_key] != 1:
                                failures.append(f"line {line_number}: duplicate configured Threads receipt from {name}")
                        else:
                            pending = pending_go.get(name)
                            if pending is None:
                                failures.append(f"line {line_number}: stale Threads width receipt from {name}")
                            else:
                                pending["width_lines"] += 1
                                if pending["width_lines"] != 1:
                                    failures.append(f"line {line_number}: duplicate Threads width receipt from {name}")
                if not is_stderr and payload.startswith("bestmove ") and by_name[name]["capability_profile"] in THREADED_CAPABILITIES:
                    pending = pending_go.pop(name, None)
                    if pending is None:
                        failures.append(f"line {line_number}: bestmove without pending go from {name}")
                    else:
                        expected_count = 1 if configured_threads[name] > 1 else 0
                        if pending["width_lines"] != expected_count:
                            failures.append(
                                f"line {line_number}: Threads width receipts={pending['width_lines']} from {name} expected={expected_count}"
                            )
                if is_stderr:
                    if payload == "Process exited normally with status 0":
                        normal_exits[name] += 1
                    else:
                        failures.append(f"line {line_number}: non-allowlisted stderr from {name}: {payload}")

    for (thread, name), options in refreshes.items():
        failures.append(f"unterminated refresh role={name} thread={thread} options={options!r}")
    for name, pending in pending_go.items():
        failures.append(f"pending go without bestmove role={name} line={pending['line']}")
    for name, role in by_name.items():
        sequence_count = sum(item["role"] == name for item in completed_sequences)
        expected_refreshes = role["expected_refreshes"]
        expected_processes = role["expected_processes"]
        if sequence_count != expected_refreshes:
            failures.append(f"role {name}: option refreshes={sequence_count} expected={expected_refreshes}")
        if normal_exits[name] != expected_processes:
            failures.append(f"role {name}: normal exits={normal_exits[name]} expected={expected_processes}")
    if failures:
        raise CandidateMatchError("trace audit failures: " + " | ".join(failures[:20]))
    report = {
        "trace_lines": len(lines), "engine_lines": engine_lines, "warning_lines": warning_lines,
        "canonical_errors": canonical_errors, "probable_embedded_book_signature_plies": len(probable_book_signatures),
        "completed_option_sequences": completed_sequences, "normal_exits": dict(normal_exits), "pass": True,
    }
    if any(role["capability_profile"] in THREADED_CAPABILITIES for role in roles):
        report["threads_width_events"] = width_events
    return report


def load_config(path: Path) -> dict[str, Any]:
    value = json.loads(path.read_text(encoding="utf-8"))
    if set(value) != {"schema", "trace", "match_exit", "match_stderr", "roles", "crash_logs", "output"} or value["schema"] != "ngn-candidate-trace-audit-config-v1":
        raise CandidateMatchError("invalid trace-audit config")
    if not isinstance(value["roles"], list) or len(value["roles"]) != 2:
        raise CandidateMatchError("trace-audit requires two roles")
    for role in value["roles"]:
        if set(role) != {"id", "display_name", "capability_profile", "resolved_options", "expected_processes", "expected_refreshes"}:
            raise CandidateMatchError("invalid trace role")
        if role["capability_profile"] not in {"ngn-pre-m4c-pinned-one-worker-v1", M4C_CAPABILITY, COMBINED_CAPABILITY, COUNTER55_CAPABILITY}:
            raise CandidateMatchError("invalid trace capability profile")
    if not isinstance(value["crash_logs"], list):
        raise CandidateMatchError("invalid crash log list")
    return value


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--config", type=Path, required=True)
    args = parser.parse_args()
    output: Path | None = None
    receipt = {"schema": "ngn-candidate-trace-audit-v1", "state": "STARTING", "started_utc": utc_now()}
    try:
        config = load_config(args.config.resolve(strict=True))
        output = Path(config["output"])
        atomic_json(output, receipt)
        match_exit = Path(config["match_exit"])
        match_stderr = Path(config["match_stderr"])
        if match_exit.read_text(encoding="utf-8").strip() != "0":
            raise CandidateMatchError("fastchess exit is nonzero")
        if match_stderr.stat().st_size != 0:
            raise CandidateMatchError("fastchess process stderr is nonempty")
        trace = Path(config["trace"])
        report = audit_trace(trace.read_text(encoding="utf-8", errors="replace").splitlines(), config["roles"])
        crash_reports = []
        for item in config["crash_logs"]:
            if set(item) != {"path", "logged_path", "expected_processes", "role_id", "phase"}:
                raise CandidateMatchError("invalid crash log config")
            crash_reports.append({"role_id": item["role_id"], "phase": item["phase"], **crash_lines(Path(item["path"]), Path(item["logged_path"]), item["expected_processes"])})
        receipt.update({"state": "COMPLETE", "ended_utc": utc_now(), "trace_sha256": sha256(trace), "trace": report, "crash_logs": crash_reports, "pass": True})
        atomic_json(output, receipt)
        print(json.dumps({"state": "COMPLETE", "pass": True, "trace_lines": report["trace_lines"]}, sort_keys=True))
        return 0
    except Exception as error:
        receipt.update({"state": "FAILED", "ended_utc": utc_now(), "error": str(error), "pass": False})
        if output is not None:
            atomic_json(output, receipt)
        print(f"trace audit failed: {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
