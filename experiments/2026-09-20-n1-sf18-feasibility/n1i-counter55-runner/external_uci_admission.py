#!/usr/bin/env python3
"""Run one pinned external opponent through sequential, fail-closed UCI admission."""
from __future__ import annotations

import argparse
import json
import os
import queue
import re
import subprocess
import sys
import time
from pathlib import Path

from common import CandidateMatchError, atomic_json, cpus_allowed_list, safe_environment, sha256, utc_now
from external_profiles import (
    SUPPORTED_WIDTHS, profile_for, resolved_options, validate_advertised_options,
    validate_artifact, validate_empty_cwd, validate_identity, validate_stderr,
)
from run_match_stage import proc_stat, read_process
from uci_preflight import Protocol, parse_advertised_options

START_LEGAL = {
    f"{file_name}2{file_name}{rank}" for file_name in "abcdefgh" for rank in ("3", "4")
} | {"b1a3", "b1c3", "g1f3", "g1h3"}
BESTMOVE_RE = re.compile(r"^bestmove ([a-h][1-8][a-h][1-8][nbrq]?)(?: ponder [a-h][1-8][a-h][1-8][nbrq]?)?$")
NODE_RE = re.compile(r"(?:^| )nodes ([0-9]+)(?: |$)")
POSITIVE_DEPTH_RE = re.compile(r"^info depth [1-9][0-9]*(?: |$)")
FATAL_OUTPUT = re.compile(r"(?:panic|fatal|error|invalid move|nnue not loaded)", re.IGNORECASE)


class ExternalProtocol(Protocol):
    """Accepted queue reader with stderr retained for profile validation, not rejected early."""

    def until_events(self, predicate, timeout: float, label: str, forbid_bestmove: bool = False) -> list[dict]:
        deadline = time.monotonic() + timeout
        events = []
        while True:
            remaining = deadline - time.monotonic()
            if remaining <= 0:
                raise CandidateMatchError(f"timeout waiting for {label}; tail={events[-8:]}")
            try:
                stream, line, timestamp = self.events.get(timeout=remaining)
            except queue.Empty as error:
                raise CandidateMatchError(f"timeout waiting for {label}; tail={events[-8:]}") from error
            if line == "<EOF>":
                if self.process.poll() is not None:
                    raise CandidateMatchError(f"engine EOF waiting for {label} rc={self.process.returncode}")
                continue
            event = {"stream": stream, "line": line, "monotonic": timestamp}
            events.append(event)
            if stream == "stderr":
                continue
            if line.startswith("info string error"):
                raise CandidateMatchError(f"engine operational error during {label}: {line}")
            if forbid_bestmove and line.startswith("bestmove "):
                raise CandidateMatchError(f"engine returned premature bestmove before stop during {label}: {line}")
            if predicate(line):
                return events

    def until(self, predicate, timeout: float, label: str) -> list[str]:
        return [event["line"] for event in self.until_events(predicate, timeout, label) if event["stream"] == "stdout"]

    def send_marked(self, line: str) -> float:
        if self.process.poll() is not None:
            raise CandidateMatchError(f"engine exited before send rc={self.process.returncode}")
        assert self.process.stdin is not None
        self.record("runner-to-engine", line)
        self.process.stdin.write((line + "\n").encode())
        self.process.stdin.flush()
        return time.monotonic()

    def drain_events(self) -> list[dict]:
        result = []
        while True:
            try:
                stream, line, timestamp = self.events.get_nowait()
            except queue.Empty:
                return result
            if line != "<EOF>":
                result.append({"stream": stream, "line": line, "monotonic": timestamp})


def load_config(path: Path) -> dict:
    value = json.loads(path.read_text(encoding="utf-8"))
    required = {"schema", "profile_id", "binary", "identity_receipt", "cwd", "width", "cpus_allowed_list", "barrier_timeout_seconds"}
    if set(value) != required or value["schema"] != "ngn-external-uci-admission-v1":
        raise CandidateMatchError("invalid external admission config")
    width = value["width"]
    if isinstance(width, bool) or not isinstance(width, int) or width not in SUPPORTED_WIDTHS:
        raise CandidateMatchError("width must be exact 1 or 8")
    expected_mask = ",".join(str(cpu) for cpu in SUPPORTED_WIDTHS[width])
    if value["cpus_allowed_list"] != expected_mask:
        raise CandidateMatchError(f"cpuset={value['cpus_allowed_list']!r} expected={expected_mask!r}")
    timeout = value["barrier_timeout_seconds"]
    if isinstance(timeout, bool) or not isinstance(timeout, (int, float)) or not 1 <= timeout <= 60:
        raise CandidateMatchError("barrier timeout outside [1,60]")
    profile_for(value["profile_id"])
    return value


def process_cpu_seconds(pid: int) -> float:
    raw = Path(f"/proc/{pid}/stat").read_text(encoding="utf-8")
    close = raw.rfind(")")
    fields = raw[close + 2:].split()
    return (int(fields[11]) + int(fields[12])) / os.sysconf("SC_CLK_TCK")


def runtime_process_witness(process: subprocess.Popen[bytes]) -> dict:
    ppid, pgid = proc_stat(process.pid)
    return read_process(process.pid, {"pid": process.pid, "ppid": ppid, "pgid": pgid})


def assert_search(lines: list[str], startpos: bool) -> dict:
    bestmoves = [BESTMOVE_RE.fullmatch(line) for line in lines if line.startswith("bestmove ")]
    if len(bestmoves) != 1 or bestmoves[0] is None:
        raise CandidateMatchError(f"expected one syntactic bestmove: {lines[-8:]}")
    bestmove = bestmoves[0].group(1)
    if startpos and bestmove not in START_LEGAL:
        raise CandidateMatchError(f"illegal start-position bestmove {bestmove}")
    nodes = [int(match.group(1)) for line in lines if (match := NODE_RE.search(line))]
    if not nodes or max(nodes) <= 1:
        raise CandidateMatchError("search lacks a real nodes>1 witness")
    return {"bestmove": bestmove, "max_reported_nodes": max(nodes), "lines": lines}


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--config", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    output = args.output.resolve()
    output.mkdir(parents=True, exist_ok=False)
    receipt = {"schema": "ngn-external-uci-admission-receipt-v1", "state": "STARTING", "started_utc": utc_now()}
    atomic_json(output / "receipt.json", receipt)
    process = None
    protocol = None
    transcript_path = output / "transcript.jsonl"
    trailing_events = []
    try:
        config = load_config(args.config.resolve(strict=True))
        profile = profile_for(config["profile_id"])
        binary = Path(config["binary"]).resolve(strict=True)
        identity_receipt = Path(config["identity_receipt"]).resolve(strict=True)
        cwd = Path(config["cwd"]).resolve(strict=True)
        validate_empty_cwd(profile, cwd)
        if cpus_allowed_list() != config["cpus_allowed_list"]:
            raise CandidateMatchError(f"runtime cpuset={cpus_allowed_list()!r} expected={config['cpus_allowed_list']!r}")
        artifact = validate_artifact(profile, binary, identity_receipt)
        started = time.monotonic()
        operation_error = None
        with transcript_path.open("x", encoding="utf-8", buffering=1) as transcript:
            process = subprocess.Popen(
                [str(binary), *profile.launch_args], cwd=cwd, env={**safe_environment(), "GOMAXPROCS": str(config["width"])},
                stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE, bufsize=0,
            )
            protocol = ExternalProtocol(process, transcript)
            try:
                timeout = float(config["barrier_timeout_seconds"])
                protocol.send("uci")
                uci_lines = protocol.until(lambda line: line == "uciok", timeout, "uciok")
                identity = validate_identity(profile, uci_lines)
                advertised = parse_advertised_options(uci_lines)
                domain = validate_advertised_options(profile, advertised, config["width"])
                barriers = []
                for option in resolved_options(profile, config["width"]):
                    value = str(option["value"]).lower() if isinstance(option["value"], bool) else str(option["value"])
                    protocol.send(f"setoption name {option['name']} value {value}")
                    protocol.send("isready")
                    lines = protocol.until(lambda line: line == "readyok", timeout, f"{option['name']} readyok")
                    barriers.append({"option": option["name"], "value": option["value"], "lines": lines})
                if "Clear Hash" in advertised:
                    protocol.send("setoption name Clear Hash")
                    protocol.send("isready")
                    protocol.until(lambda line: line == "readyok", timeout, "Clear Hash readyok")
                protocol.send("ucinewgame")
                protocol.send("isready")
                protocol.until(lambda line: line == "readyok", timeout, "ucinewgame readyok")

                protocol.send("position startpos")
                protocol.send("go depth 2")
                depth_search = assert_search(protocol.until(lambda line: line.startswith("bestmove "), timeout, "depth-2 bestmove"), True)

                protocol.send("position startpos")
                timed_go = "go wtime 2000 btime 2000 winc 200 binc 200 movestogo 16"
                protocol.send(timed_go)
                timed_started = time.monotonic()
                timed_search = assert_search(protocol.until(lambda line: line.startswith("bestmove "), timeout, "clock bestmove"), True)
                timed_elapsed = time.monotonic() - timed_started

                protocol.send("position startpos")
                before_process = runtime_process_witness(process)
                cpu_before = process_cpu_seconds(process.pid)
                witness_started = time.monotonic()
                protocol.send("go infinite")
                infinite_events = protocol.until_events(
                    lambda line: POSITIVE_DEPTH_RE.match(line) is not None, timeout,
                    "infinite positive-depth witness", forbid_bestmove=True,
                )
                minimum_runtime = 2.0
                remaining = minimum_runtime - (time.monotonic() - witness_started)
                if remaining > 0:
                    time.sleep(remaining)
                pre_stop_events = protocol.drain_events()
                queued_premature = [
                    event for event in pre_stop_events
                    if event["stream"] == "stdout" and event["line"].startswith("bestmove ")
                ]
                if queued_premature:
                    raise CandidateMatchError(f"engine returned premature bestmove before stop: {queued_premature}")
                cpu_after = process_cpu_seconds(process.pid)
                witness_elapsed = time.monotonic() - witness_started
                after_process = runtime_process_witness(process)
                stop_sent = protocol.send_marked("stop")
                stop_events = protocol.until_events(lambda line: line.startswith("bestmove "), timeout, "post-stop bestmove")
                premature = [
                    event for event in infinite_events + pre_stop_events + stop_events
                    if event["stream"] == "stdout" and event["line"].startswith("bestmove ")
                    and event["monotonic"] < stop_sent
                ]
                if premature:
                    raise CandidateMatchError(f"bestmove preceded causal stop boundary: {premature}")
                stopped_lines = [
                    event["line"] for event in infinite_events + pre_stop_events + stop_events
                    if event["stream"] == "stdout"
                ]
                stopped_search = assert_search(stopped_lines, True)
                cpu_ratio = (cpu_after - cpu_before) / witness_elapsed
                witness_claim = (
                    "observed multicore activity at requested Threads=8 under exact full mask; not exact active-worker count"
                    if config["width"] == 8 else
                    "observed single-core activity at requested Threads=1 under exact one-CPU mask"
                )
                multicore_witness = {
                    "claim": witness_claim,
                    "wall_seconds": witness_elapsed, "process_cpu_seconds": cpu_after - cpu_before,
                    "cpu_seconds_per_wall_second": cpu_ratio, "measured_core_equivalents": cpu_ratio,
                    "minimum_wall_seconds": minimum_runtime, "minimum_ratio_for_width_8": 1.5,
                    "before": before_process, "after": after_process,
                }
                expected_mask = config["cpus_allowed_list"]
                for label, observed in (("before", before_process), ("after", after_process)):
                    if observed["cpus_allowed_list"] != expected_mask or observed["gomaxprocs"] != str(config["width"]):
                        raise CandidateMatchError(f"{label} process mask/GOMAX differs")
                    if isinstance(observed["task_count"], bool) or not isinstance(observed["task_count"], int) or observed["task_count"] < 1:
                        raise CandidateMatchError(f"{label} process task count invalid")
                if config["width"] == 8 and cpu_ratio < 1.5:
                    raise CandidateMatchError(f"{profile.id}: Threads=8 CPU/wall={cpu_ratio:.3f} lacks multicore activity")

                protocol.send("ucinewgame")
                protocol.send("isready")
                protocol.until(lambda line: line == "readyok", timeout, "post-stop newgame readyok")
                protocol.send("quit")
                process.wait(timeout=timeout)
            except BaseException as error:
                operation_error = error
            finally:
                if process.poll() is None:
                    process.terminate()
                    try:
                        process.wait(timeout=3)
                    except subprocess.TimeoutExpired:
                        process.kill()
                        process.wait(timeout=3)
                if process.stdin is not None and not process.stdin.closed:
                    process.stdin.close()
                for thread in protocol.threads:
                    thread.join(timeout=2)
                readers_alive = [thread.name for thread in protocol.threads if thread.is_alive()]
                trailing_events = protocol.drain_events()
                if readers_alive and operation_error is None:
                    operation_error = CandidateMatchError(f"protocol readers survived cleanup: {readers_alive}")
            if operation_error is not None:
                raise operation_error

        validate_empty_cwd(profile, cwd)
        stderr = validate_stderr(profile, protocol.stderr_lines, config["width"])
        transcript_failures = [
            event for event in trailing_events
            if event["stream"] == "stdout"
            and (FATAL_OUTPUT.search(event["line"]) or event["line"].startswith("bestmove "))
        ]
        if transcript_failures:
            raise CandidateMatchError(f"fatal trailing engine output: {transcript_failures[:4]}")
        if process.returncode != 0:
            raise CandidateMatchError(f"engine exit={process.returncode}")
        receipt.update({
            "state": "COMPLETE", "ended_utc": utc_now(), "elapsed_seconds": time.monotonic() - started,
            "profile": profile.id, "width": config["width"], "cpus_allowed_list": config["cpus_allowed_list"],
            "artifact": artifact, "identity": identity, "option_domain": domain, "barriers": barriers,
            "book_control": {"ownbook_option_advertised": "OwnBook" in advertised,
                "engine_cwd_empty_pre_and_post": True, "depth_search_nodes_gt_one": True},
            "timing_control": {"command": timed_go, "elapsed_seconds": timed_elapsed, "search": timed_search},
            "depth_search": depth_search, "stop_search": stopped_search,
            "multicore_runtime_witness": multicore_witness,
            "transcript_sha256": sha256(transcript_path), "stderr": stderr,
            "reader_threads_alive_after_cleanup": [], "trailing_events": trailing_events,
            "process_returncode": process.returncode, "pass": True,
        })
        atomic_json(output / "receipt.json", receipt)
        print(json.dumps({"state": "COMPLETE", "profile": profile.id, "width": config["width"]}, sort_keys=True))
        return 0
    except BaseException as error:
        receipt.update({
            "state": "FAILED", "ended_utc": utc_now(), "error": str(error), "pass": False,
            "process_returncode": None if process is None else process.poll(),
            "reader_threads_alive_after_cleanup": [] if protocol is None else [t.name for t in protocol.threads if t.is_alive()],
            "trailing_events": trailing_events,
            "transcript_sha256": sha256(transcript_path) if transcript_path.is_file() else None,
        })
        atomic_json(output / "receipt.json", receipt)
        print(f"external UCI admission failed: {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
