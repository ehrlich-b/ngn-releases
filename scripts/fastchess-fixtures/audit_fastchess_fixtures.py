#!/usr/bin/env python3
"""Independently audit fastchess B0 fixture PGNs and final positions with Stockfish."""

from __future__ import annotations

import argparse
import collections
import hashlib
import json
import os
import re
import subprocess
import sys
from pathlib import Path

UCI_MOVE = re.compile(r"^[a-h][1-8][a-h][1-8][qrbn]?$")
RESULTS = {"1-0", "0-1", "1/2-1/2", "*"}
KNOWN_REASONS = {
    "White mates", "Black mates", "Draw by stalemate", "Draw by fifty moves rule",
    "Draw by 3-fold repetition", "Draw by adjudication", "White loses on time",
    "Black loses on time",
}


def require_wsl() -> None:
    if sys.platform != "linux":
        raise SystemExit(f"WSL Linux required, got {sys.platform}")
    text = " ".join(
        path.read_text(errors="replace").lower()
        for path in (Path("/proc/sys/kernel/osrelease"), Path("/proc/version"))
        if path.exists()
    )
    if "microsoft" not in text and "wsl" not in text:
        raise SystemExit("WSL kernel marker absent")


def digest(path: Path) -> str:
    value = hashlib.sha256()
    with path.open("rb") as handle:
        for block in iter(lambda: handle.read(1024 * 1024), b""):
            value.update(block)
    return value.hexdigest()


class Stockfish:
    def __init__(self, binary: Path):
        self.binary = binary

    def position(self, fen: str, moves: list[str]) -> dict:
        command = "position fen " + fen
        if moves:
            command += " moves " + " ".join(moves)
        script = "\n".join([
            "uci", "setoption name Threads value 1", "isready", command,
            "d", "go perft 1", "quit", "",
        ])
        completed = subprocess.run(
            [str(self.binary)], input=script, text=True, capture_output=True,
            timeout=10, check=False, start_new_session=True,
        )
        if completed.returncode or completed.stderr:
            raise RuntimeError(
                f"Stockfish query failed rc={completed.returncode} stderr={completed.stderr!r}"
            )
        lines = completed.stdout.splitlines()
        if "uciok" not in lines or "readyok" not in lines:
            raise RuntimeError(f"Stockfish handshake absent: {lines[:12]}")
        fens = [line.split(":", 1)[1].strip() for line in lines if line.startswith("Fen:")]
        checkers = [line.split(":", 1)[1].strip() for line in lines if line.startswith("Checkers:")]
        nodes = [line.split(":", 1)[1].strip() for line in lines if line.startswith("Nodes searched:")]
        legal = {
            match.group(1)
            for line in lines
            if (match := re.fullmatch(r"([a-h][1-8][a-h][1-8][qrbn]?):\s+\d+", line))
        }
        if len(fens) != 1 or len(checkers) != 1 or len(nodes) != 1:
            raise RuntimeError(f"unexpected Stockfish query response: {lines[-20:]}")
        if len(legal) != int(nodes[0]):
            raise RuntimeError(f"perft move/count disagreement: moves={len(legal)} nodes={nodes[0]}")
        return {
            "fen": fens[0], "checkers": checkers[0],
            "legal_moves": len(legal), "legal_move_set": sorted(legal),
        }


def parse_fixture_pgn(pgn: str) -> tuple[dict[str, str], list[str], str, list[str]]:
    """Parse the deliberately narrow UCI-notation, one-game fixture PGN."""
    if len(re.findall(r'^\[Event "', pgn, re.MULTILINE)) != 1:
        raise ValueError("fixture PGN must contain exactly one game")
    split = re.split(r"\r?\n\r?\n", pgn, maxsplit=1)
    if len(split) != 2:
        raise ValueError("fixture PGN lacks header/movetext boundary")
    header_text, body = split
    headers: dict[str, str] = {}
    for line in header_text.splitlines():
        match = re.fullmatch(r'\[([^ ]+) "(.*)"\]', line)
        if not match or match.group(1) in headers:
            raise ValueError(f"malformed or duplicate PGN header: {line!r}")
        headers[match.group(1)] = match.group(2)
    if "[Event " in body or re.search(r"[()]", body):
        raise ValueError("multiple games or variations are outside fixture PGN scope")
    comments = re.findall(r"\{([^{}]*)\}", body, re.DOTALL)
    stripped = re.sub(r"\{[^{}]*\}", " ", body, flags=re.DOTALL)
    stripped = re.sub(r";[^\r\n]*", " ", stripped)
    moves: list[str] = []
    result_tokens: list[str] = []
    for token in stripped.split():
        if re.fullmatch(r"\d+\.(?:\.\.)?", token):
            continue
        if token in RESULTS:
            result_tokens.append(token)
        elif UCI_MOVE.fullmatch(token):
            moves.append(token)
        else:
            raise ValueError(f"unexpected fixture PGN token: {token!r}")
    if len(result_tokens) != 1 or not stripped.rstrip().endswith(result_tokens[0]):
        raise ValueError(f"fixture PGN must end in exactly one result: {result_tokens}")
    return headers, moves, result_tokens[0], comments


def load_transcript(path: Path) -> list[dict]:
    return [json.loads(line) for line in path.read_text().splitlines() if line.strip()]


def canonical(fen: str) -> str:
    return " ".join(fen.split()[:4])


def parse_go_clock(line: str) -> dict[str, int]:
    tokens = line.split()
    values: dict[str, int] = {}
    for name in ("wtime", "btime", "winc", "binc"):
        if name in tokens:
            index = tokens.index(name)
            if index + 1 >= len(tokens) or not tokens[index + 1].isdigit():
                raise ValueError(f"bad {name} in go command: {line!r}")
            values[name] = int(tokens[index + 1])
    return values


def audit_case(sf: Stockfish, case: dict, case_dir: Path) -> dict:
    pgn_path = case_dir / "games.pgn"
    epd_path = case_dir / "final.epd"
    pgn = pgn_path.read_text()
    failures: list[str] = []

    def check(condition: bool, message: str) -> None:
        if not condition:
            failures.append(message)

    try:
        headers, played_moves, movetext_result, comments = parse_fixture_pgn(pgn)
    except ValueError as error:
        headers, played_moves, movetext_result, comments = {}, [], "", []
        failures.append(str(error))
    check(headers.get("White") == "FixtureWhite", f"White header={headers.get('White')!r}")
    check(headers.get("Black") == "FixtureBlack", f"Black header={headers.get('Black')!r}")
    check(headers.get("Result") == case["result"], f"Result={headers.get('Result')!r}")
    check(movetext_result == case["result"], f"movetext result={movetext_result!r}")
    check(headers.get("Termination") == case["termination"], f"Termination={headers.get('Termination')!r}")
    check(played_moves == case["applied_moves"],
          f"played moves {played_moves} != expected {case['applied_moves']}")
    observed_reasons = sorted(reason for reason in KNOWN_REASONS if any(reason in comment for comment in comments))
    expected_reasons = [] if case["classification"] == "timeout" else [case["reason"]]
    check(observed_reasons == expected_reasons,
          f"PGN terminal reasons {observed_reasons} != expected {expected_reasons}")
    check(case["reason"] in (case_dir / "stdout.txt").read_text(), "reason missing from runner stdout")
    check((case_dir / "stderr.txt").stat().st_size == 0, "runner emitted stderr")
    check((case_dir / "fastchess.log").stat().st_size > 0, "empty fastchess engine log")

    transcripts: dict[str, list[dict]] = {}
    for color in ("white", "black"):
        rows = load_transcript(case_dir / f"{color}-uci.jsonl")
        transcripts[color] = rows
        received = [row["line"] for row in rows if row["direction"] == "recv"]
        sent = [row["line"] for row in rows if row["direction"] == "send"]
        check("uci" in received and "isready" in received and "quit" in received,
              f"{color} incomplete UCI lifecycle")
        expected = case[f"{color}_moves"]
        actual = [line.split()[1] for line in sent if line.startswith("bestmove ")]
        check(actual == expected, f"{color} bestmoves {actual} != {expected}")

        expected_delay = case.get(f"{color}_delay_ms", 0)
        if expected_delay:
            go_times = [row["elapsed_ns"] for row in rows
                        if row["direction"] == "recv" and row["line"].startswith("go")]
            best_times = [row["elapsed_ns"] for row in rows
                          if row["direction"] == "send" and row["line"].startswith("bestmove")]
            check(len(go_times) == len(best_times) == len(expected),
                  f"{color} delay transcript count go={len(go_times)} best={len(best_times)} expected={len(expected)}")
            for index, (go_time, best_time) in enumerate(zip(go_times, best_times)):
                observed_ms = (best_time - go_time) / 1_000_000
                check(observed_ms >= expected_delay * 0.9,
                      f"{color} delay {index} only {observed_ms:.1f}ms < {expected_delay * 0.9:.1f}ms")
        if case.get("require_increment_ms") is not None:
            go_lines = [line for line in received if line.startswith("go")]
            check(len(go_lines) == len(case[f"{color}_moves"]),
                  f"{color} increment go count {len(go_lines)}")
            for line in go_lines:
                try:
                    clocks = parse_go_clock(line)
                except ValueError as error:
                    failures.append(str(error))
                    continue
                required = case["require_increment_ms"]
                check(clocks.get("winc") == required and clocks.get("binc") == required,
                      f"{color} go lacks {required}ms increments: {line!r}")

    positions = [sf.position(case["fen"], [])]
    prefix: list[str] = []
    legal_sequence = True
    for ply, move in enumerate(played_moves, start=1):
        before = positions[-1]
        if move not in before["legal_move_set"]:
            failures.append(f"illegal PGN move at ply {ply}: {move}; legal={before['legal_move_set']}")
            legal_sequence = False
            break
        prefix.append(move)
        positions.append(sf.position(case["fen"], prefix))
    if not legal_sequence:
        final = positions[-1]
    else:
        final = positions[-1]
    epd = epd_path.read_text().strip()
    check(legal_sequence and canonical(epd) == canonical(final["fen"]),
          f"EPD final {epd!r} != legal Stockfish replay {final['fen']!r}")
    occurrences = collections.Counter(canonical(position["fen"]) for position in positions)
    max_occurrences = max(occurrences.values())
    halfmove = int(final["fen"].split()[4])
    kind = case["classification"]
    if kind == "checkmate":
        check(final["legal_moves"] == 0 and bool(final["checkers"]), f"not checkmate: {final}")
    elif kind == "stalemate":
        check(final["legal_moves"] == 0 and not final["checkers"], f"not stalemate: {final}")
    elif kind == "fifty_move":
        check(final["legal_moves"] > 0 and halfmove >= 100, f"not fifty-move boundary: {final}")
    elif kind == "repetition":
        check(final["legal_moves"] > 0 and max_occurrences >= 3,
              f"not independently triplicated: max={max_occurrences} final={final}")
    elif kind in {"ordinary_cap", "increment_control"}:
        check(final["legal_moves"] > 0 and halfmove < 100 and max_occurrences < 3,
              f"cap control accidentally terminal: max={max_occurrences} final={final}")
    elif kind == "timeout":
        check(final["legal_moves"] > 0 and final["fen"] == case["fen"],
              f"timed-out move altered board: {final}")
    else:
        failures.append(f"unknown classification {kind}")
    final_public = {key: value for key, value in final.items() if key != "legal_move_set"}
    return {
        "name": case["name"], "result": headers.get("Result"),
        "termination": headers.get("Termination"), "reason": case["reason"],
        "played_moves": played_moves, "final": final_public,
        "max_position_occurrences": max_occurrences,
        "pgn_sha256": digest(pgn_path), "epd_sha256": digest(epd_path),
        "uci_sha256": {color: digest(case_dir / f"{color}-uci.jsonl") for color in ("white", "black")},
        "failures": failures,
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--cases", type=Path, required=True)
    parser.add_argument("--results", type=Path, required=True)
    parser.add_argument("--stockfish", type=Path, required=True)
    parser.add_argument("--stockfish-sha256", required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    require_wsl()
    stockfish = args.stockfish.resolve(strict=True)
    if digest(stockfish) != args.stockfish_sha256.lower() or not os.access(stockfish, os.X_OK):
        raise SystemExit("Stockfish identity or executable check failed")
    cases = json.loads(args.cases.read_text())
    sf = Stockfish(stockfish)
    rows = [audit_case(sf, case, args.results / case["name"]) for case in cases["cases"]]
    failures = [{"name": row["name"], "failures": row["failures"]} for row in rows if row["failures"]]
    report = {
        "status": "PASS" if not failures else "FAIL",
        "cases_sha256": digest(args.cases),
        "stockfish_sha256": digest(stockfish),
        "case_count": len(rows), "failures": failures, "cases": rows,
        "scope": "Fixture-specific PGN/UCI legality, terminal, cap, score-adjudication, increment, and timeout controls; not a general PGN validator."
    }
    args.output.write_text(json.dumps(report, indent=2) + "\n")
    print(json.dumps({key: value for key, value in report.items() if key != "cases"}, indent=2))
    return 0 if not failures else 1


if __name__ == "__main__":
    raise SystemExit(main())
