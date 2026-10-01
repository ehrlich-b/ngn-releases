#!/usr/bin/env python3
"""Sequentially verify one frozen NGN role through UCI ready barriers."""
from __future__ import annotations

import argparse
import json
import os
import queue
import re
import subprocess
import sys
import threading
import time
from pathlib import Path
from typing import Callable

from common import CandidateMatchError, atomic_json, safe_environment, utc_now

OPTION_RE = re.compile(r"^option name (.+?) type (spin|combo|string|check|button)(?:\s+(.*))?$")
EVAL_RE = re.compile(
    r"^info string eval backend (hce|ngn-v1|sf18-big|counter-5\.5) score_cp -?[0-9]+ pov side-to-move "
    r"policy base-SearchSTM rule50-and-backend-adapter correction-history excluded$"
)
THREADS_RE = re.compile(r"^info string threads configured ([0-9]+) effective ([0-9]+)$")
LEGACY_CAPABILITY = "ngn-pre-m4c-pinned-one-worker-v1"
M4C_CAPABILITY = "ngn-m4c-time-controlled-smp-v1"
COMBINED_CAPABILITY = "ngn-m4c-counter55-vs-hce-longclock-v1"
THREADED_CAPABILITIES = {M4C_CAPABILITY, COMBINED_CAPABILITY}
COUNTER55_CAPABILITY = "ngn-counter55-inprocess-one-worker-v1"
SF18BIG_CAPABILITY = "ngn-sf18-big-one-worker-v1"


class Protocol:
    def __init__(self, process: subprocess.Popen[bytes], transcript) -> None:
        self.process = process
        self.transcript = transcript
        self.events: queue.Queue[tuple[str, str, float]] = queue.Queue()
        self.transcript_lock = threading.Lock()
        self.stderr_lines: list[str] = []
        self.threads = [
            threading.Thread(target=self.reader, args=("stdout", process.stdout), daemon=True),
            threading.Thread(target=self.reader, args=("stderr", process.stderr), daemon=True),
        ]
        for thread in self.threads:
            thread.start()

    def record(self, direction: str, line: str, timestamp: float | None = None) -> None:
        with self.transcript_lock:
            self.transcript.write(json.dumps({"monotonic": timestamp if timestamp is not None else time.monotonic(), "direction": direction, "line": line}, sort_keys=True) + "\n")
            self.transcript.flush()

    def reader(self, stream: str, handle) -> None:
        assert handle is not None
        while True:
            raw = handle.readline()
            if not raw:
                break
            line = raw.decode("utf-8", errors="replace").rstrip("\r\n")
            timestamp = time.monotonic()
            self.record(f"engine-{stream}", line, timestamp)
            if stream == "stderr":
                self.stderr_lines.append(line)
            self.events.put((stream, line, timestamp))
        self.events.put((stream, "<EOF>", time.monotonic()))

    def send(self, line: str) -> None:
        if self.process.poll() is not None:
            raise CandidateMatchError(f"engine exited before send rc={self.process.returncode}")
        assert self.process.stdin is not None
        self.record("runner-to-engine", line)
        self.process.stdin.write((line + "\n").encode())
        self.process.stdin.flush()

    def until(self, predicate: Callable[[str], bool], timeout: float, label: str) -> list[str]:
        deadline = time.monotonic() + timeout
        lines: list[str] = []
        while True:
            remaining = deadline - time.monotonic()
            if remaining <= 0:
                raise CandidateMatchError(f"timeout waiting for {label}; tail={lines[-8:]}")
            try:
                stream, line, _ = self.events.get(timeout=remaining)
            except queue.Empty as error:
                raise CandidateMatchError(f"timeout waiting for {label}; tail={lines[-8:]}") from error
            if line == "<EOF>":
                if self.process.poll() is not None:
                    raise CandidateMatchError(f"engine EOF waiting for {label} rc={self.process.returncode}")
                continue
            if stream == "stderr":
                raise CandidateMatchError(f"engine stderr during {label}: {line}")
            lines.append(line)
            if line.startswith("info string error"):
                raise CandidateMatchError(f"engine operational error during {label}: {line}")
            if predicate(line):
                return lines


def parse_advertised_options(lines: list[str]) -> dict[str, dict]:
    advertised: dict[str, dict] = {}
    for line in lines:
        match = OPTION_RE.fullmatch(line)
        if match is None:
            continue
        name, kind, tail = match.group(1), match.group(2), match.group(3) or ""
        if name in advertised:
            raise CandidateMatchError(f"duplicate UCI option advertisement: {name}")
        record: dict = {"type": kind, "line": line}
        if kind == "spin":
            values = re.fullmatch(r"default (-?[0-9]+) min (-?[0-9]+) max (-?[0-9]+)", tail)
            if values is None:
                raise CandidateMatchError(f"malformed spin option advertisement: {line}")
            record.update({"default": int(values.group(1)), "min": int(values.group(2)), "max": int(values.group(3))})
            if record["min"] > record["max"] or not record["min"] <= record["default"] <= record["max"]:
                raise CandidateMatchError(f"invalid spin option domain: {line}")
        elif kind == "combo":
            combo = re.fullmatch(r"default (\S+)((?: var \S+)+)", tail)
            if combo is None:
                raise CandidateMatchError(f"malformed combo option advertisement: {line}")
            variants = re.findall(r" var (\S+)", combo.group(2))
            record.update({"default": combo.group(1), "vars": variants})
            if record["default"] not in variants or len(variants) != len(set(variants)):
                raise CandidateMatchError(f"invalid combo option domain: {line}")
        elif kind == "check":
            checked = re.fullmatch(r"default (true|false)", tail)
            if checked is None:
                raise CandidateMatchError(f"malformed check option advertisement: {line}")
            record["default"] = checked.group(1) == "true"
        elif kind == "string":
            string = re.fullmatch(r"default (.*)", tail)
            if string is None:
                raise CandidateMatchError(f"malformed string option advertisement: {line}")
            record["default"] = string.group(1)
        elif tail:
            raise CandidateMatchError(f"malformed button option advertisement: {line}")
        advertised[name] = record
    return advertised


def validate_option_domains(advertised: dict[str, dict], options: list[dict]) -> None:
    expected_types = {"Threads": "spin", "OwnBook": "check", "EvalFile": "string", "EvalBackend": "combo", "Hash": "spin", "Move Overhead": "spin"}
    missing = [name for name in expected_types if name not in advertised]
    if missing:
        raise CandidateMatchError(f"required UCI options absent: {missing}")
    wrong = {name: advertised[name]["type"] for name, kind in expected_types.items() if advertised[name]["type"] != kind}
    if wrong:
        raise CandidateMatchError(f"required UCI option types differ: {wrong}")
    requested = {item["name"]: item["value"] for item in options}
    for name in ("Threads", "Hash", "Move Overhead"):
        try:
            value = int(requested[name])
        except (TypeError, ValueError) as error:
            raise CandidateMatchError(f"requested {name} is not an integer") from error
        domain = advertised[name]
        if not domain["min"] <= value <= domain["max"]:
            raise CandidateMatchError(f"requested {name}={value} outside advertised [{domain['min']},{domain['max']}]")
    backend = str(requested["EvalBackend"])
    if backend not in advertised["EvalBackend"]["vars"]:
        raise CandidateMatchError(f"requested EvalBackend={backend!r} absent from advertised vars {advertised['EvalBackend']['vars']}")


def load_config(path: Path) -> dict:
    value = json.loads(path.read_text(encoding="utf-8"))
    required = {"schema", "role_id", "launcher", "cwd", "backend", "capability_profile", "options", "barrier_timeout_seconds"}
    if set(value) != required or value["schema"] != "ngn-candidate-uci-preflight-v1":
        raise CandidateMatchError("invalid preflight config")
    if value["backend"] not in {"hce", "ngn-v1", "sf18-big", "counter-5.5"}:
        raise CandidateMatchError("invalid backend")
    if value["capability_profile"] not in {LEGACY_CAPABILITY, M4C_CAPABILITY, COMBINED_CAPABILITY, COUNTER55_CAPABILITY, SF18BIG_CAPABILITY}:
        raise CandidateMatchError("invalid capability profile")
    if not isinstance(value["options"], list) or [x.get("name") for x in value["options"]] != ["Threads", "OwnBook", "EvalFile", "EvalBackend", "Hash", "Move Overhead"]:
        raise CandidateMatchError("invalid ordered options")
    requested = {item["name"]: item["value"] for item in value["options"]}
    try:
        threads = int(requested["Threads"])
    except (TypeError, ValueError) as error:
        raise CandidateMatchError("invalid Threads width") from error
    if value["capability_profile"] in {LEGACY_CAPABILITY, COUNTER55_CAPABILITY, SF18BIG_CAPABILITY} and threads != 1:
        raise CandidateMatchError("one-worker preflight requires Threads=1")
    if value["capability_profile"] == LEGACY_CAPABILITY and value["backend"] not in {"hce", "ngn-v1"}:
        raise CandidateMatchError("legacy preflight requires HCE or NGN-v1")
    if value["capability_profile"] == COUNTER55_CAPABILITY and value["backend"] not in {"counter-5.5", "hce"}:
        raise CandidateMatchError("Counter diagnostic preflight requires counter-5.5 or HCE")
    if value["capability_profile"] == SF18BIG_CAPABILITY and value["backend"] != "sf18-big":
        raise CandidateMatchError("SF18 BIG preflight requires sf18-big")
    if value["capability_profile"] == M4C_CAPABILITY and (value["backend"] != "hce" or threads not in {1, 2, 4, 8}):
        raise CandidateMatchError("M4c preflight requires HCE and Threads in {1,2,4,8}")
    if value["capability_profile"] == COMBINED_CAPABILITY and (value["backend"] not in {"counter-5.5", "hce"} or threads != 8):
        raise CandidateMatchError("combined preflight requires Counter 5.5 or HCE at Threads=8")
    if not isinstance(value["barrier_timeout_seconds"], (int, float)) or value["barrier_timeout_seconds"] <= 0:
        raise CandidateMatchError("invalid barrier timeout")
    return value


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--config", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    output = args.output.resolve()
    output.mkdir(parents=True, exist_ok=False)
    receipt = {"schema": "ngn-candidate-uci-preflight-receipt-v1", "state": "STARTING", "started_utc": utc_now()}
    atomic_json(output / "receipt.json", receipt)
    process: subprocess.Popen[bytes] | None = None
    try:
        config = load_config(args.config.resolve(strict=True))
        launcher = Path(config["launcher"]).resolve(strict=True)
        cwd = Path(config["cwd"]).resolve(strict=True)
        if not launcher.is_file() or not os.access(launcher, os.X_OK) or not cwd.is_dir():
            raise CandidateMatchError("launcher/cwd invalid")
        transcript_path = output / "transcript.jsonl"
        with transcript_path.open("x", encoding="utf-8", buffering=1) as transcript:
            process = subprocess.Popen(
                [str(launcher)], cwd=cwd, env=safe_environment(), stdin=subprocess.PIPE,
                stdout=subprocess.PIPE, stderr=subprocess.PIPE, bufsize=0,
            )
            protocol = Protocol(process, transcript)
            timeout = float(config["barrier_timeout_seconds"])
            protocol.send("uci")
            uci_lines = protocol.until(lambda line: line == "uciok", timeout, "uciok")
            advertised = parse_advertised_options(uci_lines)
            validate_option_domains(advertised, config["options"])
            protocol.send("debug on")
            protocol.send("isready")
            protocol.until(lambda line: line == "readyok", timeout, "debug readyok")
            barriers = []
            configured_width_receipt = None
            for option in config["options"]:
                value = str(option["value"]).lower() if isinstance(option["value"], bool) else str(option["value"])
                protocol.send(f"setoption name {option['name']} value {value}")
                protocol.send("isready")
                lines = protocol.until(lambda line: line == "readyok", timeout, f"{option['name']} readyok")
                if option["name"] in {"Threads", "OwnBook", "Hash", "Move Overhead"}:
                    expected_ack = f"info string option set: {option['name']} = {value}"
                    if lines.count(expected_ack) != 1:
                        raise CandidateMatchError(f"{option['name']} acknowledgement mismatch: {lines}")
                if option["name"] == "Threads" and config["capability_profile"] in THREADED_CAPABILITIES:
                    expected_width = f"info string threads configured {value} effective {value}"
                    width_lines = [line for line in lines if line.startswith("info string threads configured")]
                    if width_lines != [expected_width]:
                        raise CandidateMatchError(f"Threads configured-width receipt mismatch: {width_lines}")
                    configured_width_receipt = expected_width
                barriers.append({"option": option["name"], "value": option["value"], "lines": lines})
            protocol.send("position startpos")
            protocol.send("eval")
            protocol.send("isready")
            eval_lines = protocol.until(lambda line: line == "readyok", timeout, "eval diagnostic")
            diagnostics = [line for line in eval_lines if EVAL_RE.fullmatch(line)]
            if len(diagnostics) != 1 or EVAL_RE.fullmatch(diagnostics[0]).group(1) != config["backend"]:
                raise CandidateMatchError(f"selected evaluator diagnostic mismatch: {diagnostics}")
            protocol.send("ucinewgame")
            protocol.send("isready")
            protocol.until(lambda line: line == "readyok", timeout, "ucinewgame readyok")
            protocol.send("position startpos")
            protocol.send("go depth 2")
            search_lines = protocol.until(lambda line: line.startswith("bestmove "), timeout, "OwnBook=false depth-2 bestmove")
            book_signature = re.compile(r"^info depth 1 score cp 50 nodes 1 time 0 nps 0 pv \S+$")
            if any(book_signature.fullmatch(line) for line in search_lines):
                raise CandidateMatchError("OwnBook=false emitted fixed opening-book signature")
            depth_two = [line for line in search_lines if line.startswith("info depth 2 ")]
            if len(depth_two) != 1:
                raise CandidateMatchError(f"OwnBook=false did not complete exactly one depth-2 iteration: {depth_two}")
            node_match = re.search(r"(?:^| )nodes ([0-9]+)(?: |$)", depth_two[0])
            if node_match is None or int(node_match.group(1)) <= 1:
                raise CandidateMatchError(f"OwnBook=false depth-2 node witness invalid: {depth_two[0]}")
            per_go_width_receipts = [line for line in search_lines if line.startswith("info string threads configured")]
            if config["capability_profile"] in THREADED_CAPABILITIES:
                threads = int(next(item["value"] for item in config["options"] if item["name"] == "Threads"))
                expected_go_width = f"info string threads configured {threads} effective {threads}"
                expected_receipts = [expected_go_width] if threads > 1 else []
                if per_go_width_receipts != expected_receipts:
                    raise CandidateMatchError(f"per-go Threads width receipt mismatch: {per_go_width_receipts}")
            no_book_search = {"lines": search_lines, "depth_two": depth_two[0], "nodes": int(node_match.group(1))}
            if config["capability_profile"] in THREADED_CAPABILITIES:
                no_book_search["threads_width_receipts"] = per_go_width_receipts
            protocol.send("quit")
            try:
                returncode = process.wait(timeout=timeout)
            except subprocess.TimeoutExpired as error:
                raise CandidateMatchError("engine did not exit after quit") from error
            for thread in protocol.threads:
                thread.join(timeout=1.0)
                if thread.is_alive():
                    raise CandidateMatchError("engine output reader did not terminate")
            trailing_events = []
            while True:
                try:
                    stream, line, timestamp = protocol.events.get_nowait()
                except queue.Empty:
                    break
                if line != "<EOF>":
                    trailing_events.append({"stream": stream, "line": line, "monotonic": timestamp})
            trailing_errors = [event for event in trailing_events if event["stream"] == "stdout" and event["line"].startswith("info string error")]
            if trailing_errors:
                raise CandidateMatchError(f"engine operational error after bestmove: {trailing_errors}")
            if returncode != 0:
                raise CandidateMatchError(f"engine quit rc={returncode}")
            if protocol.stderr_lines:
                raise CandidateMatchError(f"engine emitted stderr: {protocol.stderr_lines}")
        receipt.update({
            "state": "COMPLETE", "ended_utc": utc_now(), "role_id": config["role_id"],
            "backend": config["backend"], "advertised_options": advertised,
            "ordered_option_barriers": barriers, "eval_diagnostic": diagnostics[0],
            "ownbook_false_search": no_book_search, "trailing_events": trailing_events,
            "returncode": returncode,
        })
        if config["capability_profile"] in THREADED_CAPABILITIES:
            receipt["configured_threads_receipt"] = configured_width_receipt
        atomic_json(output / "receipt.json", receipt)
        return 0
    except Exception as error:
        receipt.update({"state": "FAILED", "ended_utc": utc_now(), "error": str(error)})
        atomic_json(output / "receipt.json", receipt)
        if process is not None and process.poll() is None:
            process.terminate()
            try:
                process.wait(timeout=1.0)
            except subprocess.TimeoutExpired:
                process.kill()
                process.wait(timeout=1.0)
        print(f"uci preflight failed: {error}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
