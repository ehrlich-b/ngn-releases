#!/usr/bin/env python3
import hashlib
import json
from pathlib import Path
import re
import sys


source = Path(sys.argv[1])
destination = Path(sys.argv[2])
encoded = source.read_bytes()
text = encoded.decode("utf-8")
games = re.split(r"(?m)(?=^\[Event )", text)
games = [game for game in games if game.startswith("[Event ")]
if len(games) < 3:
    raise SystemExit("fewer than three PGN games")

records = []
for index, game in enumerate(games[:3], 1):
    tags = dict(re.findall(r'^\[([^ ]+) "([^"]*)"\]$', game, re.MULTILINE))
    body_start = game.find("\n\n")
    if body_start < 0:
        raise SystemExit(f"game {index}: missing movetext")
    movetext = re.sub(r"\{.*?\}", " ", game[body_start + 2:], flags=re.DOTALL)
    movetext = re.sub(r";[^\n]*", " ", movetext)
    if "(" in movetext or ")" in movetext:
        raise SystemExit(f"game {index}: variations are unsupported")
    moves = [token for token in movetext.split()
             if re.fullmatch(r"[a-h][1-8][a-h][1-8][qrbn]?", token)]
    ply_count = int(tags["PlyCount"])
    if len(moves) != ply_count or ply_count < 48:
        raise SystemExit(f"game {index}: parsed={len(moves)} declared={ply_count}")
    prefix = moves[:48]
    records.append({
        "game_index": index,
        "round": tags["Round"],
        "white": tags["White"],
        "black": tags["Black"],
        "result_recorded_not_selected": tags["Result"],
        "full_game_ply_count": ply_count,
        "moves_through_ply_48": prefix,
        "moves_sha256": hashlib.sha256((" ".join(prefix) + "\n").encode()).hexdigest(),
    })

payload = {
    "schema": "ngn-counter-profile-prefixes-v1",
    "source_pgn": str(source),
    "source_pgn_sha256": hashlib.sha256(encoded).hexdigest(),
    "selection": "first three games in immutable PGN; no result/position selection",
    "target_after_plies": [23, 48],
    "records": records,
}
destination.parent.mkdir(parents=True, exist_ok=True)
destination.write_text(json.dumps(payload, indent=2, sort_keys=True) + "\n")
print(json.dumps({"records": len(records), "targets": 6, "output": str(destination)}, sort_keys=True))
