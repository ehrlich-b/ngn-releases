"""Seeded small NGNN1/2/3 fixture and owned integer FEN evaluations."""

import argparse
import hashlib
import json
from pathlib import Path

import numpy as np

from trainer.export import export_state
from trainer.refeval import Network
from trainer.ngnp import Position, parse_fen, to_fen
from trainer.tests.synthetic import random_fens
from trainer.king_buckets import load_map


def generate(path: Path, evals: Path, hidden: int = 32, seed: int = 20261005,
             buckets: int = 1, format: int | None = None, kb_map: str = "default") -> dict:
    rng = np.random.default_rng(seed)
    # The random fixture exercises signs, saturation, and both STM orderings.
    state = {"w1": rng.normal(0, 0.07, (768, hidden)),
             "b1": rng.uniform(-0.1, 0.6, hidden),
             "o": rng.uniform(-0.3, 0.3, 2 * hidden), "ob": np.asarray(-0.031)}
    if buckets != 1:
        state["o"] = rng.uniform(-0.3, 0.3, (buckets, 2 * hidden))
        state["ob"] = rng.uniform(-0.1, 0.1, buckets)
    if format == 3:
        mapping = load_map(kb_map)
        state["king_map"] = np.asarray(mapping, dtype=np.int64)
        state["w1"] = rng.normal(0, 0.07, ((max(mapping) + 1) * 768, hidden))
    export_state(state, path, format)
    network = Network(path)
    fens = random_fens(30, seed + 1)
    if network.version >= 2:
        # Straddle every NB=8 boundary, including bare kings and full material.
        start = parse_fen("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w - - 0 1")
        kings = np.flatnonzero(start.pieces % 6 == 5)
        others = np.flatnonzero(start.pieces % 6 != 5)
        for i, count in enumerate((2, 5, 6, 9, 10, 13, 14, 17, 18, 21, 22, 25, 26, 29, 30, 32)):
            indices = np.concatenate((kings, others[:count - 2]))
            fens[i] = to_fen(Position(start.squares[indices], start.pieces[indices], i % 2))
    if network.version == 3:
        # Every own-king square in both perspectives; includes both mirror states.
        fens = []
        for square in range(64):
            opponent = 63 - square
            for stm in (0, 1):
                fens.append(to_fen(Position(np.asarray([square, opponent]), np.asarray([5, 11]), stm)))
        fens += random_fens(30, seed + 1)
    fens += ["8/8/8/8/8/8/4Q3/4K2k w - - 0 1", "q3k3/8/8/8/8/8/8/4K3 b - - 0 1"]
    # Side-swapped cases explicitly exercise the second half of output weights.
    fens += [fen.replace(" w ", " b ") if " w " in fen else fen.replace(" b ", " w ") for fen in fens]
    payload = {"format": f"NGNN{network.version}", "hidden": hidden, "seed": seed,
               "network_sha256": hashlib.sha256(path.read_bytes()).hexdigest(),
               "cases": [{"fen": fen, "eval_cp": network.evaluate_fen(fen)} for fen in fens]}
    if network.version >= 2:
        payload["buckets"] = buckets
        for case in payload["cases"]:
            case["bucket"] = network.bucket(parse_fen(case["fen"]))
    if network.version == 3:
        payload["king_buckets"] = network.king_buckets
    evals.parent.mkdir(parents=True, exist_ok=True)
    evals.write_text(json.dumps(payload, indent=2) + "\n")
    return payload


def main() -> None:
    cli = argparse.ArgumentParser(description=__doc__)
    cli.add_argument("--out", type=Path, default=Path("testdata/ngnn1/random_h32.nnue"))
    cli.add_argument("--evals", type=Path, default=Path("testdata/ngnn1/random_h32_evals.json"))
    cli.add_argument("--hidden", type=int, default=32)
    cli.add_argument("--seed", type=int, default=20261005)
    cli.add_argument("--buckets", type=int, choices=(1, 2, 4, 8), default=1)
    cli.add_argument("--format", type=int, choices=(1, 2, 3))
    cli.add_argument("--kb-map", default="default")
    args = cli.parse_args()
    payload = generate(args.out, args.evals, args.hidden, args.seed, args.buckets, args.format, args.kb_map)
    print(json.dumps({"network": str(args.out), "sha256": payload["network_sha256"], "cases": len(payload["cases"])}))


if __name__ == "__main__":
    main()
