"""Headerless, little-endian NGNP1 records; bounded-memory shard shuffling."""

from dataclasses import dataclass
import glob
from pathlib import Path

import numpy as np


RECORD_DTYPE = np.dtype([
    ("occupancy", "<u8"), ("pieces", "u1", (16,)), ("score", "<i2"),
    ("result", "u1"), ("stm", "u1"), ("ply", "<u2"),
    ("flags", "u1"), ("reserved", "u1"),
])
DROP_FLAGS = 0x0F
PIECES = "PNBRQKpnbrqk"


@dataclass
class Position:
    squares: np.ndarray
    pieces: np.ndarray
    stm: int


def parse_fen(fen: str) -> Position:
    fields = fen.split()
    if len(fields) < 2 or fields[1] not in ("w", "b"):
        raise ValueError("FEN must include board and side to move")
    ranks = fields[0].split("/")
    if len(ranks) != 8:
        raise ValueError("FEN must have eight ranks")
    squares, codes = [], []
    for row, rank in enumerate(ranks):
        file = 0
        for char in rank:
            if char in "12345678":
                file += int(char)
            elif char in PIECES and file < 8:
                squares.append((7 - row) * 8 + file)
                codes.append(PIECES.index(char))
                file += 1
            else:
                raise ValueError("Invalid FEN piece or rank")
        if file != 8:
            raise ValueError("FEN rank must span eight squares")
    order = np.argsort(squares)
    return Position(np.asarray(squares, dtype=np.int64)[order],
                    np.asarray(codes, dtype=np.int64)[order], int(fields[1] == "b"))


def to_fen(position: Position) -> str:
    board = {int(s): PIECES[int(c)] for s, c in zip(position.squares, position.pieces)}
    ranks = []
    for rank in range(7, -1, -1):
        row, empty = "", 0
        for file in range(8):
            piece = board.get(rank * 8 + file)
            if piece is None:
                empty += 1
            else:
                if empty:
                    row += str(empty)
                    empty = 0
                row += piece
        if empty:
            row += str(empty)
        ranks.append(row)
    return "/".join(ranks) + (" b" if position.stm else " w") + " - - 0 1"


def feature_indices(position: Position, perspective: int) -> np.ndarray:
    if perspective not in (0, 1):
        raise ValueError("Perspective must be white=0 or black=1")
    relative = (position.pieces // 6 != perspective).astype(np.int64)
    return relative * 384 + (position.pieces % 6) * 64 + (position.squares ^ (56 * perspective))


def decode_records(records: np.ndarray) -> dict[str, np.ndarray]:
    """Vectorized CPU decoder; absent squares have piece code -1."""
    records = np.asarray(records, dtype=RECORD_DTYPE).reshape(-1)
    occupied = ((records["occupancy"][:, None] >> np.arange(64, dtype=np.uint64)) & 1).astype(bool)
    counts = occupied.sum(axis=1)
    if np.any(counts > 32):
        raise ValueError("NGNP1 occupancy exceeds 32 pieces")
    packed = records["pieces"]
    codes = np.empty((len(records), 32), dtype=np.uint8)
    codes[:, 0::2] = packed & 15
    codes[:, 1::2] = packed >> 4
    order = np.maximum(occupied.cumsum(axis=1) - 1, 0)
    board = np.take_along_axis(codes, order, axis=1).astype(np.int16)
    if np.any((board > 11) & occupied) or np.any(records["stm"] > 1) or np.any(records["result"] > 2):
        raise ValueError("Invalid NGNP1 piece, side, or result")
    board[~occupied] = -1
    stm = records["stm"].astype(np.int64)
    sign = 1 - 2 * stm
    return {"board": board, "stm": stm,
            "score_stm": records["score"].astype(np.float32) * sign,
            "result_stm": np.where(stm == 0, records["result"] / 2, 1 - records["result"] / 2)}


def decode_record(record: np.void) -> Position:
    decoded = decode_records(np.asarray([record], dtype=RECORD_DTYPE))
    squares = np.flatnonzero(decoded["board"][0] >= 0)
    return Position(squares, decoded["board"][0, squares].astype(np.int64), int(decoded["stm"][0]))


def encode_record(position: Position, score: int, result: int = 1, ply: int = 40, flags: int = 0) -> np.void:
    squares = np.asarray(position.squares, dtype=np.int64)
    codes = np.asarray(position.pieces, dtype=np.int64)
    if len(squares) != len(codes) or len(squares) > 32 or len(set(squares.tolist())) != len(squares):
        raise ValueError("NGNP1 requires at most 32 distinct occupied squares")
    if np.any((squares < 0) | (squares > 63)) or np.any((codes < 0) | (codes > 11)):
        raise ValueError("Invalid square or piece")
    if position.stm not in (0, 1) or result not in (0, 1, 2) or not -32768 <= score <= 32767:
        raise ValueError("Invalid side, result, or score")
    if not 0 <= ply <= 65535 or not 0 <= flags <= 255:
        raise ValueError("Invalid ply or flags")
    record = np.zeros(1, dtype=RECORD_DTYPE)
    order = np.argsort(squares)
    occupancy = sum(1 << int(s) for s in squares)
    record["occupancy"][0] = occupancy
    for i, code in enumerate(codes[order]):
        record["pieces"][0, i // 2] |= int(code) << (4 * (i % 2))
    record["score"], record["result"], record["stm"] = score, result, position.stm
    record["ply"], record["flags"] = ply, flags
    return record[0]


class Shards:
    """Memmap many shards, shuffle blocks and records, visit every accepted record.

    Index memory is O(block_records), not O(total records). A shuffle is not a
    global uniform permutation: records within a block stay near one another.
    """

    def __init__(self, patterns: list[str], drop_flags: int = DROP_FLAGS,
                 min_ply: int = 0, block_records: int = 262144, require_kings: bool = False,
                 record_counts: dict[str, int] | None = None):
        if not 0 <= drop_flags <= 255 or not 0 <= min_ply <= 65535 or block_records < 1:
            raise ValueError("Invalid filter or block size")
        paths = sorted({p for pattern in patterns for p in glob.glob(pattern)})
        if not paths:
            raise ValueError("No NGNP1 shards match --data")
        caps = None
        if record_counts is not None:
            if not isinstance(record_counts, dict) or any(
                    not isinstance(p, str) or type(n) is not int or n < 0
                    for p, n in record_counts.items()):
                raise ValueError("Record counts must map shard paths to nonnegative integers")
            caps = {str(Path(p).resolve()): n for p, n in record_counts.items()}
            if len(caps) != len(record_counts) or set(caps) != {str(Path(p).resolve()) for p in paths}:
                raise ValueError("Record counts must cover exactly the selected shards")
        self.maps, self.blocks = [], []
        self.paths, self.drop_flags, self.min_ply = paths, drop_flags, min_ply
        self.raw_count, self.count = 0, 0
        for path in paths:
            size = Path(path).stat().st_size
            count = caps[str(Path(path).resolve())] if caps is not None else size // RECORD_DTYPE.itemsize
            if caps is None and size % RECORD_DTYPE.itemsize:
                raise ValueError(f"{path}: size is not a multiple of 32 (raw NGNP1 expected)")
            if count * RECORD_DTYPE.itemsize > size:
                raise ValueError(f"{path}: shorter than the requested record count")
            if not count:
                continue
            records = np.memmap(path, dtype=RECORD_DTYPE, mode="r", shape=(count,))
            shard = len(self.maps)
            self.maps.append(records)
            self.raw_count += len(records)
            for start in range(0, len(records), block_records):
                end = min(start + block_records, len(records))
                block = records[start:end]
                # Validate metadata and packed pieces, including records filtered out.
                # Work one block at a time; no full-shard decode allocation.
                counts = np.zeros(len(block), dtype=np.uint8)
                occupancy = block["occupancy"].copy()
                for _ in range(8):
                    counts += _BYTE_COUNTS[(occupancy & 255).astype(np.uint8)]
                    occupancy >>= 8
                nibble = np.arange(32)[None, :]
                codes = (block["pieces"][:, nibble[0] // 2] >> ((nibble % 2) * 4)) & 15
                if (np.any(counts > 32) or np.any(block["stm"] > 1) or np.any(block["result"] > 2)
                        or np.any((codes > 11) & (nibble < counts[:, None]))):
                    raise ValueError(f"{path}: invalid NGNP1 occupancy, piece, side, or result")
                if require_kings:
                    present = nibble < counts[:, None]
                    for king in (5, 11):
                        if np.any(((codes == king) & present).sum(axis=1) != 1):
                            raise ValueError(f"{path}: NGNN3 requires one king per side")
                accepted = int(self.mask(block).sum())
                if accepted:
                    self.blocks.append((shard, start, end))
                    self.count += accepted
        if not self.count:
            raise ValueError("No records survive the filters")

    def mask(self, records: np.ndarray) -> np.ndarray:
        return ((records["flags"] & self.drop_flags) == 0) & (records["ply"] >= self.min_ply)

    def batches(self, batch_size: int, seed: int, shuffle: bool = True):
        if batch_size < 1:
            raise ValueError("Batch size must be positive")
        rng = np.random.default_rng(seed)
        blocks = rng.permutation(len(self.blocks)) if shuffle else range(len(self.blocks))
        pending = np.empty(0, dtype=RECORD_DTYPE)
        for block_id in blocks:
            shard, start, end = self.blocks[block_id]
            block = self.maps[shard][start:end]
            indexes = np.flatnonzero(self.mask(block))
            if shuffle:
                rng.shuffle(indexes)
            records = block[indexes]
            if len(pending):
                take = min(batch_size - len(pending), len(records))
                pending = np.concatenate((pending, records[:take]))
                records = records[take:]
                if len(pending) == batch_size:
                    yield pending.view(np.uint8).reshape(-1, 32)
                    pending = np.empty(0, dtype=RECORD_DTYPE)
            full = len(records) // batch_size * batch_size
            for offset in range(0, full, batch_size):
                yield records[offset:offset + batch_size].view(np.uint8).reshape(-1, 32)
            if full < len(records):
                pending = np.concatenate((pending, records[full:]))
        if len(pending):
            yield pending.view(np.uint8).reshape(-1, 32)


_BYTE_COUNTS = np.asarray([i.bit_count() for i in range(256)], dtype=np.uint8)
