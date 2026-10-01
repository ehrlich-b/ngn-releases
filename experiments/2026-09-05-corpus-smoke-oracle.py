#!/usr/bin/env python3
"""Check generated corpus moves, sample states and terminal outcomes with SF.

Usage: python3 experiments/2026-09-05-corpus-smoke-oracle.py corpus.jsonl report.json
STOCKFISH may override the default /opt/homebrew/bin/stockfish executable.
"""
import collections
import json
import os
from pathlib import Path
import re
import select
import subprocess
import sys
import time


class Stockfish:
    def __init__(self):
        self.p = subprocess.Popen([os.environ.get("STOCKFISH", "/opt/homebrew/bin/stockfish")], stdin=subprocess.PIPE, stdout=subprocess.PIPE, bufsize=0)
        self.buf = b""

    def position(self, moves):
        self.p.stdin.write(("position startpos" + (" moves " + " ".join(moves) if moves else "") + "\nd\ngo perft 1\n").encode())
        lines, deadline = [], time.monotonic() + 15
        while time.monotonic() < deadline:
            if b"\n" in self.buf:
                line, self.buf = self.buf.split(b"\n", 1)
                line = line.decode().strip()
                lines.append(line)
                if line.startswith("Nodes searched:"):
                    fen = next(x[5:] for x in lines if x.startswith("Fen: "))
                    check = bool(next(x[9:] for x in lines if x.startswith("Checkers:")).strip())
                    legal = {x.split(":")[0] for x in lines if re.fullmatch(r"[a-h][1-8][a-h][1-8][qrbn]?: [0-9]+", x)}
                    return fen, check, legal
            elif select.select([self.p.stdout], [], [], .2)[0]:
                chunk = os.read(self.p.stdout.fileno(), 65536)
                assert chunk, "Stockfish exited"
                self.buf += chunk
        raise TimeoutError("Stockfish perft")

    def close(self):
        self.p.stdin.write(b"quit\n")
        self.p.wait(timeout=5)


def squares(placement):
    out = {}
    for rank, row in zip(range(8, 0, -1), placement.split("/")):
        file = 0
        for c in row:
            if c.isdigit():
                file += int(c)
            else:
                out[chr(97+file) + str(rank)] = c
                file += 1
    return out


def insufficient(placement):
    material = [(sq, p.lower()) for sq, p in squares(placement).items() if p.lower() != "k"]
    if not material:
        return True
    if len(material) == 1 and material[0][1] in "bn":
        return True
    if all(p == "b" for _, p in material):
        return len({(ord(sq[0])-97 + int(sq[1])-1) % 2 for sq, _ in material}) == 1
    return False


def main():
    records = [json.loads(line) for line in Path(sys.argv[1]).read_text().splitlines() if line.strip()]
    assert records, "empty corpus"
    sf = Stockfish()
    plies = samples = 0
    try:
        for record in records:
            moves = record["moves"]
            seen = collections.Counter()
            sample_at = {s["ply"]: s["fen"] for s in record["samples"]}
            for i in range(len(moves)+1):
                fen, check, legal = sf.position(moves[:i])
                fields = fen.split()
                key = " ".join(fields[:4])
                seen[key] += 1
                if i in sample_at:
                    actual = sample_at[i].split()
                    assert actual[:3] == fields[:3] and actual[4:] == fields[4:], (record["game_id"], i, "sample FEN", actual, fields)
                    if actual[3] != fields[3]:
                        # SF normalizes uncapturable EP targets; standard FEN
                        # may retain them. All other fields must match exactly.
                        board = squares(fields[0])
                        assert fields[3] == "-" and not any(m[2:4] == actual[3] and m[0] != m[2] and board[m[:2]].lower() == "p" for m in legal)
                    samples += 1
                if i < len(moves):
                    assert moves[i] in legal, (record["game_id"], i, moves[i], fen)
                    assert int(fields[4]) < 100 and seen[key] < 3 and not insufficient(fields[0]), (record["game_id"], i, "continued after draw")
                    plies += 1
            reason, result = record["terminal_reason"], record.get("result")
            if reason == "checkmate":
                assert not legal and check and result == (0 if fields[1] == "w" else 1)
            elif reason == "stalemate":
                assert not legal and not check and result == .5
            elif reason == "fifty_move":
                assert legal and int(fields[4]) >= 100 and result == .5
            elif reason == "threefold":
                assert legal and seen[key] >= 3 and result == .5
            elif reason == "insufficient_material":
                assert legal and insufficient(fields[0]) and result == .5
            else:
                assert reason == "maxplies" and not record["completed"] and result is None
                assert len(moves) == record["generation"]["maxplies"] and legal
                assert int(fields[4]) < 100 and seen[key] < 3 and not insufficient(fields[0])
        report = {"games": len(records), "legal_plies": plies, "checked_samples": samples,
                  "terminal_reasons": dict(collections.Counter(r["terminal_reason"] for r in records)), "verdict": "PASS"}
        Path(sys.argv[2]).write_text(json.dumps(report, indent=2) + "\n")
        print(json.dumps(report, indent=2))
    finally:
        sf.close()


if __name__ == "__main__":
    main()
