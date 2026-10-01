#!/usr/bin/env python3
"""Curated Stockfish 18 oracle for NGN en-passant repetition analysis.

This is intentionally small and deterministic: four illegal-EP fixtures (two
pin-discovery and two unrelated-check cases) plus two ordinary legal-EP
controls.  It does not search or play games; ``go perft`` is used only to read
bounded legal-move sets and depth-2 node counts.
"""

from __future__ import annotations

import argparse
import datetime as dt
import hashlib
import json
import os
import platform
import re
import subprocess
from dataclasses import dataclass
from pathlib import Path


EXPECTED_STOCKFISH_SHA256 = (
    "6b087694916228c905a5e14db74cca8c7e5643602226af1fa5d42353c455b9f9"
)


@dataclass(frozen=True)
class Fixture:
    name: str
    kind: str
    start_fen: str
    push: str
    ep_fen: str
    no_ep_fen: str
    ep_move: str
    cycle: tuple[str, ...] = ()


FIXTURES = (
    Fixture(
        "pinned_white_capturer",
        "illegal_pin",
        "6k1/3p4/8/K3P2r/8/8/8/8 b - - 0 1",
        "d7d5",
        "6k1/8/8/K2pP2r/8/8/8/8 w - d6 0 2",
        "6k1/8/8/K2pP2r/8/8/8/8 w - - 0 2",
        "e5d6",
        ("a5a4", "h5h6", "a4a5", "h6h5"),
    ),
    Fixture(
        "pinned_black_capturer",
        "illegal_pin",
        "8/8/8/8/R2p3k/8/4P3/1K6 w - - 0 1",
        "e2e4",
        "8/8/8/8/R2pP2k/8/8/1K6 b - e3 0 1",
        "8/8/8/8/R2pP2k/8/8/1K6 b - - 0 1",
        "d4e3",
        ("h4h5", "a4a3", "h5h4", "a3a4"),
    ),
    Fixture(
        "unrelated_check_white_capturer",
        "illegal_unrelated_check",
        "4b1k1/3p4/8/4P3/K7/8/8/8 b - - 0 1",
        "d7d5",
        "4b1k1/8/8/3pP3/K7/8/8/8 w - d6 0 2",
        "4b1k1/8/8/3pP3/K7/8/8/8 w - - 0 2",
        "e5d6",
        ("a4a3", "e8f7", "a3a4", "f7e8"),
    ),
    Fixture(
        "unrelated_check_black_capturer",
        "illegal_unrelated_check",
        "8/8/8/7k/3p4/8/4P3/1K1B4 w - - 0 1",
        "e2e4",
        "8/8/8/7k/3pP3/8/8/1K1B4 b - e3 0 1",
        "8/8/8/7k/3pP3/8/8/1K1B4 b - - 0 1",
        "d4e3",
        ("h5h6", "d1c2", "h6h5", "c2d1"),
    ),
    Fixture(
        "legal_white_capturer",
        "legal_control",
        "6k1/3p4/7r/K3P3/8/8/8/8 b - - 0 1",
        "d7d5",
        "6k1/8/7r/K2pP3/8/8/8/8 w - d6 0 2",
        "6k1/8/7r/K2pP3/8/8/8/8 w - - 0 2",
        "e5d6",
    ),
    Fixture(
        "legal_black_capturer",
        "legal_control",
        "8/8/8/8/3p3k/R7/4P3/1K6 w - - 0 1",
        "e2e4",
        "8/8/8/8/3pP2k/R7/8/1K6 b - e3 0 1",
        "8/8/8/8/3pP2k/R7/8/1K6 b - - 0 1",
        "d4e3",
    ),
)


class Stockfish:
    def __init__(self, executable: Path) -> None:
        self.proc = subprocess.Popen(
            [str(executable)],
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            bufsize=1,
        )
        assert self.proc.stdin is not None and self.proc.stdout is not None
        self.stdin = self.proc.stdin
        self.stdout = self.proc.stdout
        self.banner = self._line()
        self._send("uci")
        self.uci = self._until(lambda line: line == "uciok")
        self._send("setoption name Threads value 1")
        self._send("setoption name Hash value 16")
        self._send("isready")
        self.ready = self._until(lambda line: line == "readyok")

    def _send(self, line: str) -> None:
        self.stdin.write(line + "\n")
        self.stdin.flush()

    def _line(self) -> str:
        line = self.stdout.readline()
        if line == "":
            stderr = ""
            if self.proc.stderr is not None:
                stderr = self.proc.stderr.read()
            raise RuntimeError(f"Stockfish exited unexpectedly: {stderr}")
        return line.rstrip("\r\n")

    def _until(self, done) -> list[str]:
        lines: list[str] = []
        while True:
            line = self._line()
            lines.append(line)
            if done(line):
                return lines

    def inspect(self, position_command: str) -> dict:
        self._send(position_command)
        self._send("d")
        display = self._until(lambda line: line.startswith("Checkers:"))
        fen = next(line[5:].strip() for line in display if line.startswith("Fen:"))
        key = next(line[5:].strip() for line in display if line.startswith("Key:"))
        perft: dict[str, dict] = {}
        for depth in (1, 2):
            self._send(f"go perft {depth}")
            lines = self._until(lambda line: line.startswith("Nodes searched:"))
            moves: dict[str, int] = {}
            nodes = None
            for line in lines:
                match = re.fullmatch(r"([a-h][1-8][a-h][1-8][qrbn]?): (\d+)", line)
                if match:
                    moves[match.group(1)] = int(match.group(2))
                if line.startswith("Nodes searched:"):
                    nodes = int(line.split(":", 1)[1].strip())
            if nodes is None:
                raise AssertionError(f"missing perft total for {position_command}")
            perft[str(depth)] = {
                "nodes": nodes,
                "moves": {move: moves[move] for move in sorted(moves)},
            }
        return {"command": position_command, "fen": fen, "key": key, "perft": perft}

    def close(self) -> dict:
        self._send("quit")
        stdout_tail = self.proc.stdout.read() if self.proc.stdout is not None else ""
        stderr = self.proc.stderr.read() if self.proc.stderr is not None else ""
        returncode = self.proc.wait(timeout=10)
        return {"returncode": returncode, "stdout_tail": stdout_tail, "stderr": stderr}


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for block in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def legal_moves(state: dict) -> set[str]:
    return set(state["perft"]["1"]["moves"])


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("stockfish", type=Path)
    parser.add_argument("--source-head", required=True)
    args = parser.parse_args()

    binary = args.stockfish.resolve()
    binary_sha256 = sha256(binary)
    if binary_sha256 != EXPECTED_STOCKFISH_SHA256:
        raise SystemExit(
            f"Stockfish SHA256 {binary_sha256}, expected {EXPECTED_STOCKFISH_SHA256}"
        )

    sf = Stockfish(binary)
    results: list[dict] = []
    assertions: list[str] = []
    try:
        for fixture in FIXTURES:
            incremental_command = f"position fen {fixture.start_fen} moves {fixture.push}"
            incremental = sf.inspect(incremental_command)
            fen_ep = sf.inspect(f"position fen {fixture.ep_fen}")
            fen_no_ep = sf.inspect(f"position fen {fixture.no_ep_fen}")

            if fixture.kind.startswith("illegal_"):
                if fixture.ep_move in legal_moves(incremental):
                    raise AssertionError(f"{fixture.name}: illegal EP is legal after real push")
                if fixture.ep_move in legal_moves(fen_ep):
                    raise AssertionError(f"{fixture.name}: illegal EP is legal after EP FEN load")
                if legal_moves(incremental) != legal_moves(fen_ep) or legal_moves(fen_ep) != legal_moves(fen_no_ep):
                    raise AssertionError(f"{fixture.name}: illegal-EP legal move sets differ")
                for depth in ("1", "2"):
                    totals = {
                        incremental["perft"][depth]["nodes"],
                        fen_ep["perft"][depth]["nodes"],
                        fen_no_ep["perft"][depth]["nodes"],
                    }
                    if len(totals) != 1:
                        raise AssertionError(f"{fixture.name}: illegal-EP perft {depth} differs: {totals}")
                if incremental["fen"].split()[3] != "-":
                    raise AssertionError(f"{fixture.name}: Stockfish real push retained illegal EP")
                if fen_ep["fen"].split()[3] == "-":
                    raise AssertionError(f"{fixture.name}: expected FEN load to retain pseudo EP")
                if incremental["key"] != fen_no_ep["key"]:
                    raise AssertionError(f"{fixture.name}: real-push key != no-EP key")
                if fen_ep["key"] == fen_no_ep["key"]:
                    raise AssertionError(f"{fixture.name}: FEN EP/no-EP behavior unexpectedly identical")

                moves = [fixture.push, *(fixture.cycle * 2)]
                prefix: list[str] = []
                history: list[dict] = []
                for ply, move in enumerate(moves, start=1):
                    before = sf.inspect(
                        f"position fen {fixture.start_fen} moves {' '.join(prefix)}"
                        if prefix
                        else f"position fen {fixture.start_fen}"
                    )
                    if move not in legal_moves(before):
                        raise AssertionError(f"{fixture.name}: ply {ply} {move} not legal")
                    prefix.append(move)
                    after = sf.inspect(f"position fen {fixture.start_fen} moves {' '.join(prefix)}")
                    history.append({"ply": ply, "move": move, "after": after})
                final_key = history[-1]["after"]["key"]
                occurrences = [entry["ply"] for entry in history if entry["after"]["key"] == final_key]
                if occurrences != [1, 5, 9]:
                    raise AssertionError(f"{fixture.name}: repetition plies {occurrences}, want [1, 5, 9]")
                assertions.append(
                    f"{fixture.name}: all 9 played plies legal; normalized Stockfish key repeats at plies 1,5,9"
                )
            else:
                if fixture.ep_move not in legal_moves(incremental) or fixture.ep_move not in legal_moves(fen_ep):
                    raise AssertionError(f"{fixture.name}: legal EP missing")
                if fixture.ep_move in legal_moves(fen_no_ep):
                    raise AssertionError(f"{fixture.name}: EP exists without EP right")
                if incremental["key"] != fen_ep["key"]:
                    raise AssertionError(f"{fixture.name}: real-push key != legal-EP FEN key")
                if fen_ep["key"] == fen_no_ep["key"]:
                    raise AssertionError(f"{fixture.name}: legal EP failed to distinguish key")
                history = []
                assertions.append(
                    f"{fixture.name}: ordinary legal EP appears only with EP right and distinguishes key"
                )

            results.append(
                {
                    "name": fixture.name,
                    "kind": fixture.kind,
                    "fixture": fixture.__dict__,
                    "incremental": incremental,
                    "fen_ep": fen_ep,
                    "fen_no_ep": fen_no_ep,
                    "history": history,
                }
            )
    finally:
        close = sf.close()

    if close["returncode"] != 0 or close["stderr"]:
        raise AssertionError(f"Stockfish close failure: {close}")

    receipt = {
        "schema": "ngn-ep-repetition-stockfish-probe-v1",
        "generated_at_utc": dt.datetime.now(dt.timezone.utc).isoformat(),
        "source_head": args.source_head,
        "stockfish": {
            "path": str(binary),
            "sha256": binary_sha256,
            "banner": sf.banner,
            "uci_id": [line for line in sf.uci if line.startswith("id ")],
        },
        "environment": {
            "platform": platform.platform(),
            "python": platform.python_version(),
            "cgroup": Path("/proc/self/cgroup").read_text().strip(),
            "allowed_cpus": sorted(os.sched_getaffinity(0)),
        },
        "assertions": assertions,
        "fixtures": results,
        "process": close,
    }
    print(json.dumps(receipt, indent=2, sort_keys=True))


if __name__ == "__main__":
    main()
