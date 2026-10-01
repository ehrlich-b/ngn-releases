#!/usr/bin/env python3
"""Bounded exact-binary Rodent promotion-PV probe; no match or scoring."""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import queue
import re
import shutil
import signal
import subprocess
import sys
import threading
import time
from pathlib import Path

BINARY_SHA256 = "9cfb8195207ee5695c1973a89664ab73b34b5bcbc10ad3bc0f0afe28b9713cbc"
BINARY_BYTES = 7_311_522
NET_OFFSET = 2_538_080
NET_BYTES = 4_744_768
NET_SHA256 = "c35a1abc1b8c1cb1d5f4221454d494c1a6da1ed9088fd51ab27038bfa74b5053"
CHESS_SHA256 = "1fde6a8e932508d14b31f6a584497b49c2969fc3c7cdc675347f819f0d667a5b"
ROOT_FEN = "8/7R/4K3/8/1p6/1P4p1/5k2/8 w - - 4 79"
PROMOTION_FEN = "8/6R1/8/1K6/1p6/1P3k2/6p1/8 b - - 5 83"
FORCED_PROMOTION_FEN = "k7/7R/8/4B3/8/8/6p1/K7 b - - 0 1"
LEGAL_PROMOTIONS = ["g2g1q", "g2g1r", "g2g1b", "g2g1n"]
OPTIONS = [
    ("Threads", "1"),
    ("Hash", "128"),
    ("UCI_Chess960", "false"),
    ("UCI_LimitStrength", "false"),
    ("UCI_Elo", "3000"),
]
MAX_SEARCHES = 9
MAX_DEPTH = 18
PER_SEARCH_TIMEOUT = 10.0
MAX_CPU_SECONDS = 120.0
MAX_SEARCH_WALL_SECONDS = 180.0


def sha256_path(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as stream:
        while block := stream.read(1024 * 1024):
            digest.update(block)
    return digest.hexdigest()


def embedded_sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as stream:
        stream.seek(NET_OFFSET)
        remaining = NET_BYTES
        while remaining:
            block = stream.read(min(1024 * 1024, remaining))
            if not block:
                raise RuntimeError("binary ended inside embedded network")
            digest.update(block)
            remaining -= len(block)
    return digest.hexdigest()


def process_cpu_seconds(pid: int) -> float:
    fields = Path(f"/proc/{pid}/stat").read_text().split()
    return (int(fields[13]) + int(fields[14])) / os.sysconf("SC_CLK_TCK")


def process_observation(pid: int) -> dict[str, object]:
    status = {}
    for line in Path(f"/proc/{pid}/status").read_text().splitlines():
        if ":" in line:
            key, value = line.split(":", 1)
            status[key] = value.strip()
    return {
        "pid": pid,
        "pgid": os.getpgid(pid),
        "exe": str(Path(f"/proc/{pid}/exe").resolve()),
        "cwd": str(Path(f"/proc/{pid}/cwd").resolve()),
        "cpus_allowed_list": status.get("Cpus_allowed_list"),
        "mems_allowed_list": status.get("Mems_allowed_list"),
        "threads": int(status.get("Threads", "0")),
        "nice": int(Path(f"/proc/{pid}/stat").read_text().split()[18]),
    }


class UCIProcess:
    def __init__(self, binary: Path, cwd: Path):
        self.input_lines: list[str] = []
        self.output_lines: list[str] = []
        self.stderr_lines: list[str] = []
        self.events: queue.Queue[tuple[str, str]] = queue.Queue()
        env = os.environ.copy()
        env["GOMAXPROCS"] = "1"
        self.proc = subprocess.Popen(
            [str(binary)],
            cwd=cwd,
            env=env,
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            bufsize=1,
            start_new_session=True,
        )
        assert self.proc.stdin and self.proc.stdout and self.proc.stderr
        self.stdout_thread = threading.Thread(
            target=self._reader, args=("stdout", self.proc.stdout), daemon=True
        )
        self.stderr_thread = threading.Thread(
            target=self._reader, args=("stderr", self.proc.stderr), daemon=True
        )
        self.stdout_thread.start()
        self.stderr_thread.start()

    def _reader(self, kind: str, stream) -> None:
        for raw in stream:
            line = raw.rstrip("\r\n")
            if kind == "stdout":
                self.output_lines.append(line)
            else:
                self.stderr_lines.append(line)
            self.events.put((kind, line))

    def send(self, line: str) -> None:
        assert self.proc.stdin
        self.input_lines.append(line)
        self.proc.stdin.write(line + "\n")
        self.proc.stdin.flush()

    def wait_stdout(self, predicate, timeout: float, description: str) -> str:
        deadline = time.monotonic() + timeout
        while True:
            remaining = deadline - time.monotonic()
            if remaining <= 0:
                raise TimeoutError(f"timeout waiting for {description}")
            try:
                kind, line = self.events.get(timeout=remaining)
            except queue.Empty as error:
                raise TimeoutError(f"timeout waiting for {description}") from error
            if kind == "stdout" and predicate(line):
                return line

    def terminate_owned_group(self) -> None:
        if self.proc.poll() is not None:
            return
        pgid = os.getpgid(self.proc.pid)
        os.killpg(pgid, signal.SIGTERM)
        try:
            self.proc.wait(timeout=1.0)
        except subprocess.TimeoutExpired:
            os.killpg(pgid, signal.SIGKILL)
            self.proc.wait(timeout=1.0)

    def close(self) -> int:
        if self.proc.poll() is None:
            self.send("quit")
        try:
            return self.proc.wait(timeout=2.0)
        except subprocess.TimeoutExpired:
            self.terminate_owned_group()
            return self.proc.returncode


def parse_pv(line: str) -> list[str]:
    marker = " pv "
    if not line.startswith("info ") or marker not in line:
        return []
    return line.split(marker, 1)[1].split()


def classify_tokens(chess, board, tokens: list[str]) -> dict[str, object]:
    walked = board.copy(stack=True)
    for index, token in enumerate(tokens):
        try:
            move = chess.Move.from_uci(token)
        except ValueError:
            return {"legal": False, "index": index, "token": token, "reason": "invalid UCI token"}
        if move not in walked.legal_moves:
            piece = walked.piece_at(move.from_square)
            alternatives = sorted(
                candidate.uci()
                for candidate in walked.legal_moves
                if candidate.uci().startswith(token[:4]) and candidate.promotion is not None
            )
            return {
                "legal": False,
                "index": index,
                "token": token,
                "fen": walked.fen(),
                "piece": piece.symbol() if piece else None,
                "suffixless_last_rank_pawn": bool(
                    piece
                    and piece.piece_type == chess.PAWN
                    and chess.square_rank(move.to_square) in (0, 7)
                    and len(token) == 4
                ),
                "legal_promotion_alternatives": alternatives,
            }
        walked.push(move)
    return {"legal": True, "plies": len(tokens), "final_fen": walked.fen()}


def board_for_spec(chess, spec: dict[str, object]):
    kind = spec["position_kind"]
    if kind == "fen":
        return chess.Board(str(spec["position"]))
    if kind == "startpos":
        board = chess.Board()
        for token in spec.get("history", []):
            move = chess.Move.from_uci(token)
            if move not in board.legal_moves:
                raise RuntimeError(f"frozen history move {token} is illegal at {board.fen()}")
            board.push(move)
        return board
    raise RuntimeError(f"unknown position kind {kind}")


def configure(proc: UCIProcess) -> dict[str, object]:
    proc.send("uci")
    proc.wait_stdout(lambda line: line == "uciok", 3.0, "uciok")
    uci_lines = list(proc.output_lines)
    barriers = []
    for name, value in OPTIONS:
        proc.send(f"setoption name {name} value {value}")
        proc.send("isready")
        barriers.append(proc.wait_stdout(lambda line: line == "readyok", 3.0, f"readyok after {name}"))
    proc.send("ucinewgame")
    proc.send("isready")
    barriers.append(proc.wait_stdout(lambda line: line == "readyok", 3.0, "readyok after ucinewgame"))
    advertised = [line for line in uci_lines if line.startswith("option name ")]
    for name, _ in OPTIONS:
        if not any(line.startswith(f"option name {name} ") for line in advertised):
            raise RuntimeError(f"required option {name} not advertised")
    return {
        "uci_lines": uci_lines,
        "barriers": barriers,
        "ownbook_advertised": any(line.startswith("option name OwnBook ") for line in advertised),
    }


def position_command(spec: dict[str, object]) -> str:
    if spec["position_kind"] == "fen":
        return f"position fen {spec['position']}"
    history = " ".join(spec.get("history", []))
    return "position startpos" + (f" moves {history}" if history else "")


def write_session(evidence: Path, name: str, proc: UCIProcess, receipt: dict[str, object]) -> None:
    (evidence / f"{name}.uci").write_text("\n".join(proc.input_lines) + "\n")
    (evidence / f"{name}.stdout").write_text("\n".join(proc.output_lines) + "\n")
    (evidence / f"{name}.stderr").write_text(
        "\n".join(proc.stderr_lines) + ("\n" if proc.stderr_lines else "")
    )
    (evidence / f"{name}.json").write_text(json.dumps(receipt, indent=2, sort_keys=True) + "\n")


def run_session(binary: Path, evidence: Path, runtime: Path, name: str, specs: list[dict[str, object]]) -> dict[str, object]:
    cwd = runtime / name
    if cwd.exists():
        if any(cwd.iterdir()):
            raise RuntimeError(f"refusing nonempty runtime cwd {cwd}")
        cwd.rmdir()
    cwd.mkdir(parents=True)
    proc = UCIProcess(binary, cwd)
    receipt: dict[str, object] = {
        "name": name,
        "started_utc": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        "cwd_empty_before": not any(cwd.iterdir()),
        "process_start": process_observation(proc.proc.pid),
        "searches": [],
    }
    try:
        receipt["configuration"] = configure(proc)
        for spec in specs:
            before_line = len(proc.output_lines)
            proc.send(position_command(spec))
            proc.send(f"go depth {spec['depth']}")
            cpu_before = process_cpu_seconds(proc.proc.pid)
            wall_before = time.monotonic()
            bestmove = proc.wait_stdout(lambda line: line.startswith("bestmove "), PER_SEARCH_TIMEOUT, "bestmove")
            wall = time.monotonic() - wall_before
            cpu = process_cpu_seconds(proc.proc.pid) - cpu_before
            lines = proc.output_lines[before_line:]
            info_lines = [line for line in lines if line.startswith("info ")]
            receipt["searches"].append(
                {
                    **spec,
                    "command": f"go depth {spec['depth']}",
                    "wall_seconds": wall,
                    "process_cpu_seconds": cpu,
                    "bestmove": bestmove,
                    "info_lines": info_lines,
                    "max_reported_depth": max(
                        [int(match.group(1)) for line in info_lines if (match := re.search(r"\bdepth (\d+)", line))],
                        default=0,
                    ),
                    "max_reported_nodes": max(
                        [int(match.group(1)) for line in info_lines if (match := re.search(r"\bnodes (\d+)", line))],
                        default=0,
                    ),
                }
            )
        receipt["process_end"] = process_observation(proc.proc.pid)
        receipt["returncode"] = proc.close()
    except BaseException as error:
        receipt["error"] = f"{type(error).__name__}: {error}"
        proc.terminate_owned_group()
        receipt["returncode"] = proc.proc.returncode
        raise
    finally:
        proc.stdout_thread.join(timeout=1.0)
        proc.stderr_thread.join(timeout=1.0)
        receipt["cwd_empty_after"] = not any(cwd.iterdir())
        receipt["finished_utc"] = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())
        write_session(evidence, name, proc, receipt)
    if receipt["returncode"] != 0:
        raise RuntimeError(f"{name} exited {receipt['returncode']}")
    if not receipt["cwd_empty_after"]:
        raise RuntimeError(f"{name} wrote into its empty cwd")
    return receipt


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--binary", required=True, type=Path)
    parser.add_argument("--inputs", required=True, type=Path)
    parser.add_argument("--evidence", required=True, type=Path)
    parser.add_argument("--runtime", required=True, type=Path)
    parser.add_argument(
        "--resume-seven",
        type=Path,
        help="reuse the seven preserved first-pass session receipts and run only the conditional warm pair",
    )
    args = parser.parse_args()
    args.evidence.mkdir(parents=True, exist_ok=True)
    args.runtime.mkdir(parents=True, exist_ok=True)

    binary = args.binary.resolve()
    if binary.stat().st_size != BINARY_BYTES or sha256_path(binary) != BINARY_SHA256:
        raise RuntimeError("frozen binary identity mismatch")
    if embedded_sha256(binary) != NET_SHA256:
        raise RuntimeError("embedded default network identity mismatch")

    import chess

    chess_path = Path(chess.__file__).resolve()
    if chess.__version__ != "1.11.2" or sha256_path(chess_path) != CHESS_SHA256:
        raise RuntimeError("python-chess identity mismatch")

    frontier_path = args.inputs / "frontier-mechanisms.json"
    frozen = json.loads(frontier_path.read_text())["rodent_v12"]["independent_chess"]
    fixtures = frozen["fixtures"]
    if len(fixtures) != 2 or any(fixture["root_fen"] != ROOT_FEN for fixture in fixtures):
        raise RuntimeError("unexpected frozen fixtures")
    if fixtures[0]["history"] != fixtures[1]["history"]:
        raise RuntimeError("saved fixtures do not share the frozen history")
    history = fixtures[0]["history"]
    root_from_history = chess.Board()
    for token in history:
        move = chess.Move.from_uci(token)
        if move not in root_from_history.legal_moves:
            raise RuntimeError(f"illegal frozen history move {token}")
        root_from_history.push(move)
    if root_from_history.fen() != ROOT_FEN:
        raise RuntimeError(f"full frozen history ends at {root_from_history.fen()}, want {ROOT_FEN}")

    prefix = parse_pv(fixtures[0]["info"])[:9]
    prefix_board = chess.Board(ROOT_FEN)
    prefix_classification = classify_tokens(chess, prefix_board, prefix)
    if not prefix_classification.get("legal"):
        raise RuntimeError(f"saved nine-ply prefix is not legal: {prefix_classification}")
    for token in prefix:
        prefix_board.push(chess.Move.from_uci(token))
    if prefix_board.fen() != PROMOTION_FEN:
        raise RuntimeError(f"nine-ply prefix ends at {prefix_board.fen()}, want {PROMOTION_FEN}")
    if sorted(move.uci() for move in prefix_board.legal_moves if move.from_square == chess.G2) != sorted(LEGAL_PROMOTIONS):
        raise RuntimeError("promotion node does not have the four frozen alternatives")

    specs = [
        {"name": "fresh-root-depth17", "position_kind": "fen", "position": ROOT_FEN, "depth": 17},
        {"name": "fresh-root-depth18", "position_kind": "fen", "position": ROOT_FEN, "depth": 18},
        {"name": "full-history-depth17", "position_kind": "startpos", "history": history, "depth": 17},
        {"name": "full-history-depth18", "position_kind": "startpos", "history": history, "depth": 18},
        {"name": "forced-promotion-positive", "position_kind": "fen", "position": FORCED_PROMOTION_FEN, "depth": 4},
        {"name": "ordinary-negative", "position_kind": "startpos", "history": [], "depth": 2},
        {"name": "promotion-node-control", "position_kind": "fen", "position": PROMOTION_FEN, "depth": 4},
    ]
    if args.resume_seven:
        resume = args.resume_seven.resolve()
        receipts = [json.loads((resume / f"{spec['name']}.json").read_text()) for spec in specs]
    else:
        receipts = [run_session(binary, args.evidence, args.runtime, spec["name"], [spec]) for spec in specs]

    def classify_receipts():
        classifications = []
        runtime_witness = False
        for receipt in receipts:
            for search in receipt["searches"]:
                board = board_for_spec(chess, search)
                line_results = []
                for line in search["info_lines"]:
                    tokens = parse_pv(line)
                    if not tokens:
                        continue
                    result = classify_tokens(chess, board, tokens)
                    line_results.append({"line": line, "tokens": tokens, "classification": result})
                    if (
                        search["name"].startswith(("fresh-root", "full-history", "warm-root"))
                        and not result.get("legal")
                        and result.get("token") == "g2g1"
                        and result.get("index") == 9
                        and result.get("suffixless_last_rank_pawn")
                        and sorted(result.get("legal_promotion_alternatives", [])) == sorted(LEGAL_PROMOTIONS)
                    ):
                        runtime_witness = True
                best_tokens = search["bestmove"].split()[1:2]
                best_result = classify_tokens(chess, board, best_tokens)
                classifications.append(
                    {"name": search["name"], "pv_lines": line_results, "bestmove_classification": best_result}
                )
        return classifications, runtime_witness

    classifications, runtime_witness = classify_receipts()
    warm_executed = False
    if not runtime_witness:
        warm_specs = [
            {"name": "warm-root-depth18", "position_kind": "fen", "position": ROOT_FEN, "depth": 18},
            {"name": "warm-root-repeat-depth17", "position_kind": "fen", "position": ROOT_FEN, "depth": 17},
        ]
        receipts.append(run_session(binary, args.evidence, args.runtime, "conditional-warm-root", warm_specs))
        warm_executed = True
        classifications, runtime_witness = classify_receipts()

    positive = next(item for item in classifications if item["name"] == "forced-promotion-positive")
    if not positive["bestmove_classification"].get("legal") or positive["bestmove_classification"].get("plies") != 1:
        raise RuntimeError("forced-promotion bestmove is not legal")
    positive_move = next(r["searches"][0]["bestmove"].split()[1] for r in receipts if r["name"] == "forced-promotion-positive")
    if positive_move[-1:] not in "qrbn" or len(positive_move) != 5:
        raise RuntimeError(f"forced-promotion renderer omitted suffix: {positive_move}")

    negative = next(item for item in classifications if item["name"] == "ordinary-negative")
    if not all(line["classification"].get("legal") for line in negative["pv_lines"]):
        raise RuntimeError("ordinary negative control emitted an illegal PV")

    promotion_control = next(item for item in classifications if item["name"] == "promotion-node-control")
    promotion_move = next(r["searches"][0]["bestmove"].split()[1] for r in receipts if r["name"] == "promotion-node-control")
    if not promotion_control["bestmove_classification"].get("legal"):
        raise RuntimeError(f"promotion-node bestmove is illegal: {promotion_move}")

    historical = []
    for fixture in fixtures:
        historical.append(
            {
                "recorded_info": fixture["info"],
                "classification": classify_tokens(chess, chess.Board(ROOT_FEN), parse_pv(fixture["info"])),
                "recorded_bestmove": fixture["actual_recorded_bestmove"],
                "recorded_bestmove_classification": classify_tokens(
                    chess, chess.Board(ROOT_FEN), [fixture["actual_recorded_bestmove"]]
                ),
                "promotion_completions": fixture["promotion_completions"],
            }
        )

    search_count = sum(len(receipt["searches"]) for receipt in receipts)
    total_cpu = sum(search["process_cpu_seconds"] for receipt in receipts for search in receipt["searches"])
    total_wall = sum(search["wall_seconds"] for receipt in receipts for search in receipt["searches"])
    if search_count > MAX_SEARCHES or total_cpu > MAX_CPU_SECONDS or total_wall > MAX_SEARCH_WALL_SECONDS:
        raise RuntimeError("aggregate search budget exceeded")
    for receipt in receipts:
        for search in receipt["searches"]:
            if search["depth"] > MAX_DEPTH or search["wall_seconds"] > PER_SEARCH_TIMEOUT:
                raise RuntimeError(f"per-search budget exceeded by {search['name']}")

    result = {
        "schema": "ngn-rodent-pv-identity-probe-v1",
        "state": "EXACT_BINARY_SUFFIXLESS_PROMOTION_REPRODUCED" if runtime_witness else "NOT_REPRODUCED_IN_BOUNDED_PLAN",
        "binary": {"path": str(binary), "bytes": BINARY_BYTES, "sha256": BINARY_SHA256},
        "embedded_network": {"offset": NET_OFFSET, "bytes": NET_BYTES, "sha256": NET_SHA256},
        "python_chess": {"version": chess.__version__, "path": str(chess_path), "sha256": CHESS_SHA256},
        "frozen_input_sha256": sha256_path(frontier_path),
        "root_fen": ROOT_FEN,
        "nine_ply_prefix": prefix,
        "promotion_fen": PROMOTION_FEN,
        "legal_promotions": LEGAL_PROMOTIONS,
        "recorded_historical": historical,
        "new_runtime": classifications,
        "runtime_witness": runtime_witness,
        "controls": {
            "forced_promotion_bestmove": positive_move,
            "ordinary_negative_all_pvs_legal": True,
            "promotion_node_bestmove": promotion_move,
        },
        "budget": {
            "searches": search_count,
            "maximum_searches": MAX_SEARCHES,
            "total_process_cpu_seconds": total_cpu,
            "maximum_cpu_seconds": MAX_CPU_SECONDS,
            "total_search_wall_seconds": total_wall,
            "maximum_search_wall_seconds": MAX_SEARCH_WALL_SECONDS,
            "per_search_timeout_seconds": PER_SEARCH_TIMEOUT,
            "maximum_depth": MAX_DEPTH,
            "conditional_warm_tt_executed": warm_executed,
            "conditional_warm_tt_reason": (
                "fresh/full-history probes did not reproduce, so the predeclared two-search warm control ran"
                if warm_executed
                else "fresh/full-history probes reproduced the witness; warm control was unnecessary"
            ),
        },
    }
    output = args.evidence / "probe-result.json"
    output.write_text(json.dumps(result, indent=2, sort_keys=True) + "\n")
    print(json.dumps({
        "state": result["state"],
        "runtime_witness": runtime_witness,
        "searches": search_count,
        "cpu_seconds": total_cpu,
        "search_wall_seconds": total_wall,
        "forced_promotion_bestmove": positive_move,
        "promotion_node_bestmove": promotion_move,
    }, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
