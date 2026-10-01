#!/usr/bin/env python3
"""Deterministic UCI engine used only to test an external tournament runner."""

from __future__ import annotations

import argparse
import json
import os
import sys
import time
from pathlib import Path


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--id", required=True)
    parser.add_argument("--moves", default="")
    parser.add_argument("--delay-ms", type=int, default=0)
    parser.add_argument("--score-cp", type=int, default=0)
    parser.add_argument("--transcript", type=Path, required=True)
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    moves = [move for move in args.moves.split(",") if move]
    search = 0
    start = time.monotonic_ns()
    args.transcript.parent.mkdir(parents=True, exist_ok=True)

    def record(direction: str, line: str) -> None:
        row = {
            "elapsed_ns": time.monotonic_ns() - start,
            "pid": os.getpid(),
            "id": args.id,
            "direction": direction,
            "line": line,
        }
        with args.transcript.open("a", encoding="utf-8") as handle:
            handle.write(json.dumps(row, sort_keys=True) + "\n")

    def send(line: str) -> None:
        record("send", line)
        print(line, flush=True)

    allowed = "unknown"
    try:
        for status_line in Path("/proc/self/status").read_text().splitlines():
            if status_line.startswith("Cpus_allowed_list:"):
                allowed = status_line.split(":", 1)[1].strip()
                break
    except OSError as error:
        allowed = f"error:{error}"
    record("meta", f"cpus_allowed_list={allowed}")

    for raw in sys.stdin:
        line = raw.rstrip("\r\n")
        record("recv", line)
        command = line.split(maxsplit=1)[0] if line else ""
        if command == "uci":
            send(f"id name NGN B0 fixture {args.id}")
            send("id author NGN test fixture")
            send("uciok")
        elif command == "isready":
            send("readyok")
        elif command == "go":
            move = moves[search] if search < len(moves) else "0000"
            search += 1
            if args.delay_ms:
                time.sleep(args.delay_ms / 1000.0)
            send(f"info depth 1 score cp {args.score_cp} time {max(args.delay_ms, 1)} nodes 1 pv {move}")
            send(f"bestmove {move}")
        elif command == "quit":
            return 0
        elif command in {"debug", "setoption", "register", "ucinewgame", "position", "stop", "ponderhit"}:
            continue
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
