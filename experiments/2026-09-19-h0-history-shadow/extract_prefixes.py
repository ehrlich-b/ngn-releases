#!/usr/bin/env python3
"""Freeze result-blind H0 warm prefixes from the immutable Counter match PGN."""

import hashlib
import json
from pathlib import Path
import re
import sys


SOURCE_FIRST_GAME = 4
SOURCE_LAST_GAME = 11
TARGET_PLIES = [16, 32, 48, 64]


source = Path(sys.argv[1])
destination = Path(sys.argv[2])
encoded = source.read_bytes()
text = encoded.decode("utf-8")
games = re.split(r"(?m)(?=^\[Event )", text)
games = [game for game in games if game.startswith("[Event ")]
if len(games) < SOURCE_LAST_GAME:
    raise SystemExit(f"PGN has {len(games)} games, need {SOURCE_LAST_GAME}")

records = []
for game_index in range(SOURCE_FIRST_GAME, SOURCE_LAST_GAME + 1):
    game = games[game_index - 1]
    tags = dict(re.findall(r'^\[([^ ]+) "([^"]*)"\]$', game, re.MULTILINE))
    body_start = game.find("\n\n")
    if body_start < 0:
        raise SystemExit(f"game {game_index}: missing movetext")
    movetext = re.sub(r"\{.*?\}", " ", game[body_start + 2 :], flags=re.DOTALL)
    movetext = re.sub(r";[^\n]*", " ", movetext)
    if "(" in movetext or ")" in movetext:
        raise SystemExit(f"game {game_index}: variations are unsupported")
    moves = [
        token
        for token in movetext.split()
        if re.fullmatch(r"[a-h][1-8][a-h][1-8][qrbn]?", token)
    ]
    ply_count = int(tags["PlyCount"])
    if len(moves) != ply_count or ply_count < TARGET_PLIES[-1]:
        raise SystemExit(
            f"game {game_index}: parsed={len(moves)} declared={ply_count}, "
            f"need at least {TARGET_PLIES[-1]}"
        )
    prefix = moves[: TARGET_PLIES[-1]]
    records.append(
        {
            "game_index": game_index,
            "round": tags["Round"],
            "white": tags["White"],
            "black": tags["Black"],
            "full_game_ply_count": ply_count,
            "moves_through_ply_64": prefix,
            "moves_sha256": hashlib.sha256(
                (" ".join(prefix) + "\n").encode()
            ).hexdigest(),
        }
    )

payload = {
    "schema": "ngn-h0-history-shadow-prefixes-v1",
    "source_pgn": str(source),
    "source_pgn_sha256": hashlib.sha256(encoded).hexdigest(),
    "selection": (
        "source games 4 through 11 inclusive; excludes the three earlier "
        "profile games; sequential selection with no result or position filtering"
    ),
    "results_excluded": True,
    "target_after_plies": TARGET_PLIES,
    "records": records,
}
destination.parent.mkdir(parents=True, exist_ok=True)
destination.write_text(json.dumps(payload, indent=2, sort_keys=True) + "\n")
print(
    json.dumps(
        {
            "records": len(records),
            "targets": len(records) * len(TARGET_PLIES),
            "output": str(destination),
        },
        sort_keys=True,
    )
)

