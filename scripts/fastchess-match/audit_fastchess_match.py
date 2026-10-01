#!/usr/bin/env python3
"""Fail-closed audit for the fixed NGN B0 fastchess A/A match."""

from __future__ import annotations

import argparse
import collections
import hashlib
import json
import os
import re
import select
import subprocess
import sys
import time
from pathlib import Path

MOVE = re.compile(r"^[a-h][1-8][a-h][1-8][qrbn]?$")
RESULTS = {"1-0", "0-1", "1/2-1/2", "*"}
NATURAL_REASONS = {
    "Draw by insufficient mating material",
    "Draw by 3-fold repetition",
    "Draw by fifty moves rule",
    "Draw by stalemate",
    "White mates",
    "Black mates",
}
OPERATIONAL_PATTERNS = (
    "loses on time", "makes an illegal move", "disconnects", "stalls",
    "abandoned", "unterminated", "adjudication",
)


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


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for block in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def canonical_fen(fen: str) -> str:
    fields = fen.split()
    if len(fields) < 4:
        raise ValueError(f"invalid FEN/EPD: {fen!r}")
    return " ".join(fields[:4])


def repetition_identity(position: dict) -> str:
    """FIDE position identity with EP retained only for a currently legal pawn capture."""
    board, side, castling, ep = position["fen"].split()[:4]
    if ep != "-":
        target_file = ord(ep[0]) - ord("a")
        target_rank = int(ep[1])
        source_rank = 5 if side == "w" else 4
        pawn = "P" if side == "w" else "p"
        squares: dict[str, str] = {}
        rank = 8
        file_index = 0
        for char in board:
            if char == "/":
                rank -= 1
                file_index = 0
            elif char.isdigit():
                file_index += int(char)
            else:
                squares[f"{chr(ord('a') + file_index)}{rank}"] = char
                file_index += 1
        candidates = []
        for source_file in (target_file - 1, target_file + 1):
            if 0 <= source_file < 8:
                source = f"{chr(ord('a') + source_file)}{source_rank}"
                move = source + ep
                if squares.get(source) == pawn and move in position["legal"]:
                    candidates.append(move)
        if not candidates:
            ep = "-"
    return " ".join((board, side, castling, ep))


class Stockfish:
    """Persistent unbuffered Stockfish oracle with explicit line buffering."""

    def __init__(self, binary: Path):
        self.process = subprocess.Popen(
            [str(binary)], stdin=subprocess.PIPE, stdout=subprocess.PIPE,
            stderr=subprocess.PIPE, bufsize=0, start_new_session=True,
        )
        self.buffer = b""
        try:
            self.write("uci")
            self.until("uciok")
            self.write("setoption name Threads value 1")
            self.write("isready")
            self.until("readyok")
        except BaseException:
            self.terminate()
            raise

    def write(self, line: str) -> None:
        if self.process.stdin is None:
            raise RuntimeError("Stockfish stdin unavailable")
        self.process.stdin.write((line + "\n").encode())
        self.process.stdin.flush()

    def until(self, marker: str, timeout: float = 10.0) -> list[str]:
        if self.process.stdout is None:
            raise RuntimeError("Stockfish stdout unavailable")
        lines: list[str] = []
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            if b"\n" in self.buffer:
                raw, self.buffer = self.buffer.split(b"\n", 1)
                line = raw.decode(errors="replace").rstrip("\r")
                lines.append(line)
                if line.startswith(marker):
                    return lines
                continue
            ready, _, _ = select.select([self.process.stdout], [], [], min(0.2, deadline - time.monotonic()))
            if ready:
                chunk = os.read(self.process.stdout.fileno(), 65536)
                if not chunk:
                    stderr = self.process.stderr.read().decode(errors="replace") if self.process.stderr else ""
                    raise RuntimeError(f"Stockfish exited before {marker!r}: {stderr!r}")
                self.buffer += chunk
        raise TimeoutError(f"Stockfish response timeout waiting for {marker!r}; tail={lines[-12:]}")

    def query(self, moves: list[str], fen: str | None = None) -> dict:
        command = "position " + (f"fen {fen}" if fen else "startpos")
        if moves:
            command += " moves " + " ".join(moves)
        self.write(command)
        self.write("d")
        self.write("go perft 1")
        lines = self.until("Nodes searched:")
        one = lambda prefix: [line[len(prefix):].strip() for line in lines if line.startswith(prefix)]
        fens, keys, checkers, nodes = one("Fen:"), one("Key:"), one("Checkers:"), one("Nodes searched:")
        legal = {
            match.group(1)
            for line in lines
            if (match := re.fullmatch(r"([a-h][1-8][a-h][1-8][qrbn]?):\s+[0-9]+", line))
        }
        if not (len(fens) == len(keys) == len(checkers) == len(nodes) == 1):
            raise RuntimeError(f"ambiguous Stockfish response: {lines[-25:]}")
        if len(legal) != int(nodes[0]):
            raise RuntimeError(f"Stockfish perft disagreement: {len(legal)} != {nodes[0]}")
        return {
            "fen": fens[0], "key": keys[0], "checkers": checkers[0],
            "legal": legal,
        }

    def close(self) -> None:
        if self.process.poll() is None:
            self.write("quit")
            try:
                self.process.wait(timeout=5)
            except subprocess.TimeoutExpired:
                self.terminate()
                raise
        stderr = self.process.stderr.read().decode(errors="replace") if self.process.stderr else ""
        if self.process.returncode != 0 or stderr:
            raise RuntimeError(f"Stockfish close rc={self.process.returncode} stderr={stderr!r}")

    def terminate(self) -> None:
        if self.process.poll() is None:
            self.process.kill()
            self.process.wait(timeout=5)


def parse_game(block: str) -> dict:
    parts = re.split(r"\r?\n\r?\n", block.strip(), maxsplit=1)
    if len(parts) != 2:
        raise ValueError("PGN game lacks header/movetext boundary")
    header_text, body = parts
    headers: dict[str, str] = {}
    for line in header_text.splitlines():
        match = re.fullmatch(r'\[([^ ]+) "(.*)"\]', line)
        if not match or match.group(1) in headers:
            raise ValueError(f"malformed or duplicate PGN header: {line!r}")
        headers[match.group(1)] = match.group(2)
    if "[Event " in body or re.search(r"[()]", body):
        raise ValueError("multiple games or PGN variations are unsupported")
    tokens = re.findall(r"\{[^{}]*\}|[^\s]+", re.sub(r";[^\r\n]*", " ", body))
    moves: list[str] = []
    comments: list[str] = []
    result_tokens: list[str] = []
    waiting_for_comment = False
    for token in tokens:
        if re.fullmatch(r"\d+\.(?:\.\.)?", token):
            continue
        if MOVE.fullmatch(token):
            if waiting_for_comment:
                raise ValueError(f"move {moves[-1]} has no telemetry comment")
            moves.append(token)
            waiting_for_comment = True
        elif token.startswith("{"):
            if not waiting_for_comment:
                raise ValueError("orphan PGN comment")
            comments.append(token[1:-1])
            waiting_for_comment = False
        elif token in RESULTS:
            if waiting_for_comment:
                raise ValueError(f"move {moves[-1]} has no telemetry comment")
            result_tokens.append(token)
        else:
            raise ValueError(f"unexpected UCI PGN token: {token!r}")
    if len(moves) != len(comments):
        raise ValueError("move/comment count mismatch")
    if len(result_tokens) != 1 or not body.rstrip().endswith(result_tokens[0]):
        raise ValueError(f"PGN game must end in one result: {result_tokens}")
    return {"headers": headers, "moves": moves, "comments": comments, "result": result_tokens[0]}


def parse_pgn(path: Path) -> list[dict]:
    text = path.read_text()
    blocks = [block for block in re.split(r"(?=^\[Event \")", text, flags=re.MULTILINE) if block.strip()]
    return [parse_game(block) for block in blocks]


def terminal_reason(comment: str) -> str:
    observed = [reason for reason in NATURAL_REASONS if reason in comment]
    operational = [fragment for fragment in OPERATIONAL_PATTERNS if fragment in comment.lower()]
    if operational:
        return "OPERATIONAL:" + ",".join(operational)
    if len(observed) != 1:
        raise ValueError(f"last comment has {len(observed)} natural reasons: {comment!r}")
    return observed[0]


def telemetry_ok(comment: str) -> bool:
    required = (
        r"^[^,]+/[0-9]+ [0-9]+(?:\.[0-9]+)?s",
        r"(?:^|, )tl=[-0-9.]+s(?:,|$)", r"(?:^|, )latency=[-0-9.]+s(?:,|$)",
        r"(?:^|, )n=[0-9]+(?:,|$)", r"(?:^|, )sd=[0-9]+(?:,|$)",
        r"(?:^|, )nps=[0-9]+(?:,|$)", r"(?:^|, )hashfull=[0-9]+(?:,|$)",
        r'(?:^|, )pv="[^"]*"(?:,|$)',
    )
    return all(re.search(pattern, comment) for pattern in required)


def probable_embedded_book_signature(comment: str, move: str) -> bool:
    """Recognize B0's fixed embedded-book UCI response without treating it as an error."""
    return all((
        re.search(r"^[+-]?0\.50/1 0(?:\.0+)?s(?:,| )", comment) is not None,
        re.search(r"(?:^|, )n=1(?:,|$)", comment) is not None,
        re.search(r"(?:^|, )nps=0(?:,|$)", comment) is not None,
        re.search(r"(?:^|, )hashfull=0(?:,|$)", comment) is not None,
        re.search(rf'(?:^|, )pv="{re.escape(move)}"(?:,|$)', comment) is not None,
    ))


def insufficient_material(fen: str) -> bool:
    board = fen.split()[0]
    pieces: list[tuple[str, int]] = []
    rank = 7
    file_index = 0
    for char in board:
        if char == "/":
            rank -= 1
            file_index = 0
        elif char.isdigit():
            file_index += int(char)
        else:
            if char.lower() != "k":
                pieces.append((char.lower(), (file_index + rank) & 1))
            file_index += 1
    if any(piece in "pqr" for piece, _ in pieces):
        return False
    if len(pieces) <= 1:
        return True
    return all(piece == "b" for piece, _ in pieces) and len({color for _, color in pieces}) == 1


def validate_ep_key_oracle(sf: Stockfish) -> dict:
    no_ep = sf.query([], "8/8/8/8/P7/8/8/K6k b - - 0 1")
    dead_ep = sf.query([], "8/8/8/8/P7/8/8/K6k b - a3 0 1")
    live_ep = sf.query([], "8/8/8/8/Pp6/8/8/K6k b - a3 0 1")
    live_no_ep = sf.query([], "8/8/8/8/Pp6/8/8/K6k b - - 0 1")
    pinned_ep = sf.query([], "k7/8/8/r4pPK/8/8/8/8 w - f6 0 1")
    pinned_no_ep = sf.query([], "k7/8/8/r4pPK/8/8/8/8 w - - 0 1")
    if repetition_identity(no_ep) != repetition_identity(dead_ep):
        raise RuntimeError("uncapturable en-passant field was retained")
    if repetition_identity(live_ep) == repetition_identity(live_no_ep) or "b4a3" not in live_ep["legal"]:
        raise RuntimeError("legal en-passant capture was not retained")
    if "g5f6" in pinned_ep["legal"] or repetition_identity(pinned_ep) != repetition_identity(pinned_no_ep):
        raise RuntimeError("pinned, illegal en-passant capture changed repetition identity")
    return {
        "dead_ep_identity_equal": True,
        "legal_ep_identity_distinct": True,
        "legal_ep_move": "b4a3",
        "pinned_ep_move_illegal": True,
        "pinned_ep_identity_equal": True,
        "stockfish_key_observed": {
            "dead_ep_equal": no_ep["key"] == dead_ep["key"],
            "legal_ep_distinct": live_ep["key"] != live_no_ep["key"],
            "pinned_ep_equal": pinned_ep["key"] == pinned_no_ep["key"],
        },
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--pgn", type=Path, required=True)
    parser.add_argument("--final-epd", type=Path, required=True)
    parser.add_argument("--opening-prefixes", type=Path, required=True)
    parser.add_argument("--opening-prefixes-sha256", required=True)
    parser.add_argument("--opening-pgn", type=Path, required=True)
    parser.add_argument("--opening-pgn-sha256", required=True)
    parser.add_argument("--stockfish", type=Path, required=True)
    parser.add_argument("--stockfish-sha256", required=True)
    parser.add_argument("--engine-a", required=True)
    parser.add_argument("--engine-b", required=True)
    parser.add_argument("--games", type=int, default=200)
    parser.add_argument("--pairs", type=int, default=100)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    require_wsl()
    for path, expected in (
        (args.opening_prefixes, args.opening_prefixes_sha256),
        (args.opening_pgn, args.opening_pgn_sha256),
        (args.stockfish, args.stockfish_sha256),
    ):
        if not path.is_file() or sha256(path) != expected.lower():
            raise SystemExit(f"identity check failed: {path}")
    games = parse_pgn(args.pgn)
    if len(games) != args.games:
        raise RuntimeError(f"game count {len(games)} != {args.games}")
    opening_lines = [line.split() for line in args.opening_prefixes.read_text().splitlines() if line.strip()]
    if len(opening_lines) < args.pairs:
        raise RuntimeError("opening file too short")
    selected = [tuple(line) for line in opening_lines[:args.pairs]]
    expected_prefixes = collections.Counter(selected)
    actual_prefixes: collections.Counter[tuple[str, ...]] = collections.Counter()
    orientations: collections.Counter[tuple[tuple[str, ...], str, str]] = collections.Counter()
    outcome = collections.Counter()
    reasons = collections.Counter()
    pair_scores: dict[str, list[float]] = collections.defaultdict(list)
    a_points = 0.0
    a_points_by_color = collections.Counter()
    total_plies = 0
    probable_embedded_book_plies = 0
    rows = []
    sf = Stockfish(args.stockfish)
    try:
        ep_key_control = validate_ep_key_oracle(sf)
        for game_number, game in enumerate(games, start=1):
            headers = game["headers"]
            moves = game["moves"]
            comments = game["comments"]
            result = game["result"]
            if headers.get("Result") != result or result not in RESULTS - {"*"}:
                raise RuntimeError(f"game {game_number}: header/movetext result mismatch")
            if headers.get("Termination") != "normal":
                raise RuntimeError(f"game {game_number}: operational termination {headers.get('Termination')!r}")
            white, black = headers.get("White"), headers.get("Black")
            if {white, black} != {args.engine_a, args.engine_b} or white == black:
                raise RuntimeError(f"game {game_number}: engine identity/orientation mismatch")
            if headers.get("SetUp") or headers.get("FEN"):
                raise RuntimeError(f"game {game_number}: unexpected non-startpos headers")
            matching = [opening for opening in expected_prefixes if tuple(moves[:len(opening)]) == opening]
            if len(matching) != 1:
                raise RuntimeError(f"game {game_number}: opening prefix matched {len(matching)} selected lines")
            opening = matching[0]
            actual_prefixes[opening] += 1
            orientations[(opening, white, black)] += 1
            for ply, comment in enumerate(comments):
                if ply < len(opening):
                    if comment.strip() != "book":
                        raise RuntimeError(f"game {game_number} ply {ply + 1}: opening move lacks book marker")
                elif not telemetry_ok(comment):
                    raise RuntimeError(f"game {game_number} ply {ply + 1}: telemetry incomplete: {comment!r}")
            game_embedded_book_plies = [
                ply + 1 for ply, (move, comment) in enumerate(zip(moves, comments))
                if ply >= len(opening) and probable_embedded_book_signature(comment, move)
            ]
            probable_embedded_book_plies += len(game_embedded_book_plies)
            keys = collections.Counter()
            prefix: list[str] = []
            position = sf.query(prefix)
            keys[repetition_identity(position)] += 1
            for ply, move in enumerate(moves, start=1):
                if not position["legal"]:
                    raise RuntimeError(f"game {game_number} ply {ply}: move follows an already-terminal position")
                if move not in position["legal"]:
                    raise RuntimeError(f"game {game_number} ply {ply}: illegal {move}; fen={position['fen']}")
                prefix.append(move)
                position = sf.query(prefix)
                keys[repetition_identity(position)] += 1
            reason = terminal_reason(comments[-1])
            if reason.startswith("OPERATIONAL:"):
                raise RuntimeError(f"game {game_number}: {reason}")
            legal_count = len(position["legal"])
            checked = bool(position["checkers"])
            side_to_move = position["fen"].split()[1]
            halfmove = int(position["fen"].split()[4])
            final_identity_occurrences = keys[repetition_identity(position)]
            if legal_count == 0 and checked:
                expected_result = "0-1" if side_to_move == "w" else "1-0"
                expected_reason = "Black mates" if side_to_move == "w" else "White mates"
                valid = result == expected_result and reason == expected_reason
            elif legal_count == 0:
                valid = result == "1/2-1/2" and reason == "Draw by stalemate"
            elif reason == "Draw by 3-fold repetition":
                valid = result == "1/2-1/2" and final_identity_occurrences >= 3
            elif reason == "Draw by fifty moves rule":
                valid = result == "1/2-1/2" and halfmove >= 100
            elif reason == "Draw by insufficient mating material":
                valid = result == "1/2-1/2" and insufficient_material(position["fen"])
            else:
                valid = False
            if not valid:
                raise RuntimeError(
                    f"game {game_number}: terminal mismatch result={result} reason={reason!r} "
                    f"legal={legal_count} check={checked} halfmove={halfmove} "
                    f"final_repetitions={final_identity_occurrences} "
                    f"fen={position['fen']}"
                )
            outcome[result] += 1
            reasons[reason] += 1
            white_points = {"1-0": 1.0, "0-1": 0.0, "1/2-1/2": 0.5}[result]
            a_score = white_points if white == args.engine_a else 1.0 - white_points
            a_points += a_score
            a_points_by_color["white" if white == args.engine_a else "black"] += a_score
            pair_scores[opening].append(a_score)
            total_plies += len(moves)
            rows.append({
                "game": game_number, "white": white, "black": black, "result": result,
                "reason": reason, "plies": len(moves), "final_fen": position["fen"],
                "final_legal_moves": legal_count, "final_check": checked,
                "final_position_occurrences": final_identity_occurrences,
                "max_position_key_occurrences": max(keys.values()),
                "probable_embedded_book_plies": game_embedded_book_plies,
            })
    finally:
        sf.close()
    if actual_prefixes != collections.Counter({key: count * 2 for key, count in expected_prefixes.items()}):
        raise RuntimeError("selected opening multiplicities are not exactly two games each")
    for opening, copies in expected_prefixes.items():
        if copies != 1 or len(pair_scores[opening]) != 2:
            raise RuntimeError(f"opening is not one exact two-game pair: {opening}")
        if orientations[(opening, args.engine_a, args.engine_b)] != copies:
            raise RuntimeError(f"opening lacks exact A-white orientation: {opening}")
        if orientations[(opening, args.engine_b, args.engine_a)] != copies:
            raise RuntimeError(f"opening lacks exact B-white orientation: {opening}")
    penta = [0, 0, 0, 0, 0]
    for scores in pair_scores.values():
        half_points = round(sum(scores) * 2)
        if half_points < 0 or half_points > 4:
            raise RuntimeError(f"invalid pair score: {scores}")
        penta[half_points] += 1
    final_epd = [canonical_fen(line) for line in args.final_epd.read_text().splitlines() if line.strip()]
    audited_finals = [canonical_fen(row["final_fen"]) for row in rows]
    if len(final_epd) != args.games or collections.Counter(final_epd) != collections.Counter(audited_finals):
        raise RuntimeError("final EPD does not reconcile exactly with all audited PGN final boards")
    report = {
        "status": "PASS", "games": len(games), "pairs": args.pairs,
        "legal_plies": total_plies, "results": dict(outcome), "reasons": dict(reasons),
        "probable_embedded_book_signature_plies": probable_embedded_book_plies,
        "engine_a_points": a_points, "engine_a_score_rate": a_points / len(games),
        "engine_a_points_by_color": dict(a_points_by_color), "penta_0_to_4": penta,
        "ep_normalization_control": ep_key_control,
        "pgn_sha256": sha256(args.pgn),
        "opening_prefixes_sha256": sha256(args.opening_prefixes),
        "opening_pgn_sha256": sha256(args.opening_pgn),
        "final_epd_sha256": sha256(args.final_epd),
        "stockfish_sha256": sha256(args.stockfish), "game_audit": rows,
    }
    args.output.write_text(json.dumps(report, indent=2) + "\n")
    print(json.dumps({key: value for key, value in report.items() if key != "game_audit"}, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
