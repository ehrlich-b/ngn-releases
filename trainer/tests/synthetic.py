"""Generate owned legal-ish material positions, in bounded NumPy chunks.

Each side has one king and a subset of its initial non-king inventory. Kings
are on e1/e8; pawns use ranks 2..7, other pieces use any free square. We do
not enforce checks or reachability: this is a trainer plumbing test, not a
search-quality dataset. Flags are zero intentionally, not engine annotations.
"""

import argparse
from pathlib import Path

import numpy as np

from trainer.ngnp import Position, RECORD_DTYPE, to_fen

MATERIAL = np.asarray([100, 320, 330, 500, 900, 0], dtype=np.int32)
INVENTORY = np.asarray([0] * 8 + [1] * 2 + [2] * 2 + [3] * 2 + [4], dtype=np.uint8)


def random_records(count: int, rng: np.random.Generator) -> np.ndarray:
    squares = np.empty((count, 32), dtype=np.int64)
    # Reserve valid pawn squares, then sample other pieces from remaining squares.
    # This covers back-rank features while retaining distinct occupied squares.
    pawn_slots = np.concatenate((np.arange(8), np.arange(15, 23)))
    other_slots = np.concatenate((np.arange(8, 15), np.arange(23, 30)))
    squares[:, pawn_slots] = rng.random((count, 48)).argsort(axis=1)[:, :16] + 8
    keys = rng.random((count, 64))
    np.put_along_axis(keys, squares[:, pawn_slots], 2.0, axis=1)
    keys[:, [4, 60]] = 2.0
    squares[:, other_slots] = keys.argsort(axis=1)[:, :14]
    squares[:, 30:] = [4, 60]
    codes = np.broadcast_to(np.concatenate((INVENTORY, INVENTORY + 6, [5, 11])), (count, 32))
    keep = rng.random((count, 32)) < rng.uniform(0.25, 0.85, (count, 1))
    keep[:, 30:] = True
    order = np.where(keep, squares, 64).argsort(axis=1, kind="stable")
    ordered_codes = np.take_along_axis(codes, order, axis=1).astype(np.uint8)
    ordered_keep = np.take_along_axis(keep, order, axis=1)
    ordered_codes[~ordered_keep] = 0
    records = np.zeros(count, dtype=RECORD_DTYPE)
    records["occupancy"] = ((np.uint64(1) << squares.astype(np.uint64)) * keep).sum(axis=1, dtype=np.uint64)
    records["pieces"] = ordered_codes[:, 0::2] | (ordered_codes[:, 1::2] << 4)
    value = MATERIAL[codes % 6] * np.where(codes < 6, 1, -1)
    score = (value * keep).sum(axis=1)
    records["score"] = score
    # A seeded sampled outcome has expectation sigmoid(material/K).
    probability = 1 / (1 + np.exp(-score.astype(np.float64) / 400))
    records["result"] = 2 * (rng.random(count) < probability).astype(np.uint8)
    records["stm"] = rng.integers(0, 2, count, dtype=np.uint8)
    records["ply"] = rng.integers(20, 121, count, dtype=np.uint16)
    return records


def generate(path: Path, count: int, seed: int, chunk: int = 32768) -> None:
    if count < 1 or chunk < 1:
        raise ValueError("Count and chunk must be positive")
    path.parent.mkdir(parents=True, exist_ok=True)
    rng = np.random.default_rng(seed)
    with path.open("wb") as output:
        for start in range(0, count, chunk):
            random_records(min(chunk, count - start), rng).tofile(output)


def random_fens(count: int, seed: int) -> list[str]:
    from trainer.ngnp import decode_record
    records = random_records(count, np.random.default_rng(seed))
    return [to_fen(decode_record(record)) for record in records]


def main() -> None:
    cli = argparse.ArgumentParser(description=__doc__)
    cli.add_argument("--out", type=Path, required=True)
    cli.add_argument("--records", type=int, default=131072)
    cli.add_argument("--seed", type=int, default=20261005)
    args = cli.parse_args()
    generate(args.out, args.records, args.seed)
    print(f"wrote {args.records} records, {args.records * 32} bytes: {args.out}")


if __name__ == "__main__":
    main()
