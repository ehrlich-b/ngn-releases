"""NGN reader for the published STM-normalized ChessBoard 32-byte layout.

This archive's gather step already mapped its i16 scores to NGN's sigmoid
scale. Preserve those scores and results. Original STM, castling and EP are
absent; canonical NGNP records describe us as White, with White to move.
No implementation or weights from another trainer are used here.
"""

import glob
from pathlib import Path

import numpy as np

from trainer.ngnp import RECORD_DTYPE, _BYTE_COUNTS


ARCHIVE_DTYPE = np.dtype([
    ("occupancy", "<u8"), ("pieces", "u1", (16,)), ("score", "<i2"),
    ("result", "u1"), ("us_king", "u1"), ("them_king", "u1"),
    ("reserved", "u1", (3,)),
])
HASH_VERSION = "ngn-board-mix64-v1"


def piece_counts(occupancy):
    work = occupancy.copy()
    counts = np.zeros(len(work), dtype=np.uint8)
    for _ in range(8):
        counts += _BYTE_COUNTS[(work & 255).astype(np.uint8)]
        work >>= 8
    return counts


def _mix(value):
    """Independent unsigned avalanche; overflow is intentional arithmetic mod 2^64."""
    value = value ^ (value >> np.uint64(29))
    value *= np.uint64(0xD6E8FEB86659FD93)
    value ^= value >> np.uint64(32)
    value *= np.uint64(0xA24BAED4963EE407)
    return value ^ (value >> np.uint64(31))


def board_hash(records, seed):
    """Hash exactly the occupied board, excluding labels and unused padding.

    Identical model inputs share a partition even with differing score/result.
    Hash collisions only reserve additional training positions for holdout.
    """
    counts = piece_counts(records["occupancy"])
    if np.any(counts > 32):
        raise ValueError("Archive occupancy exceeds 32 pieces")
    packed = np.ascontiguousarray(records["pieces"]).view("<u8").reshape(-1, 2)
    words = []
    for word in range(2):
        bits = np.clip(counts.astype(np.int16) * 4 - word * 64, 0, 64)
        # Shift by 64 yields zero; selecting all bits separately avoids relying
        # on the platform's behavior for an oversized integer shift.
        mask = (np.uint64(1) << np.minimum(bits, 63).astype(np.uint64)) - np.uint64(1)
        mask[bits == 64] = np.iinfo(np.uint64).max
        words.append(packed[:, word] & mask)
    hashed = _mix(records["occupancy"] ^ np.uint64(seed & ((1 << 64) - 1)))
    hashed ^= _mix(words[0] ^ np.uint64(0x9E3779B185EBCA87))
    hashed ^= _mix(words[1] ^ np.uint64(0xC2B2AE3D27D4EB4F))
    return _mix(hashed)


def canonical_records(records):
    """Validate occupied pieces/kings and transcode one bounded memory block."""
    records = np.asarray(records, dtype=ARCHIVE_DTYPE).reshape(-1)
    counts = piece_counts(records["occupancy"])
    nibble = np.arange(32)[None, :]
    codes = (records["pieces"][:, nibble[0] // 2] >> ((nibble % 2) * 4)) & 15
    present = nibble < counts[:, None]
    valid_code = (codes <= 5) | ((codes >= 8) & (codes <= 13))
    if (np.any(counts > 32) or np.any(records["result"] > 2)
            or np.any(~valid_code & present)):
        raise ValueError("Invalid archive occupancy, piece, or result")
    for field, king_code, flip in (("us_king", 5, 0), ("them_king", 13, 56)):
        stored_square = records[field].astype(np.uint64)
        if np.any(stored_square >= 64) or np.any(((codes == king_code) & present).sum(axis=1) != 1):
            raise ValueError("Archive requires exactly one king per side")
        square = stored_square ^ np.uint64(flip)
        king_bit = np.uint64(1) << square
        ordinal = piece_counts(records["occupancy"] & (king_bit - np.uint64(1)))
        occupied_king = (records["occupancy"] & king_bit) != 0
        index = np.minimum(ordinal, 31)
        if np.any(~occupied_king) or np.any(codes[np.arange(len(records)), index] != king_code):
            raise ValueError("Archive king metadata disagrees with occupied board")
    normalized = np.where(codes >= 8, codes - 2, codes).astype(np.uint8)
    normalized[~present] = 0
    output = np.zeros(len(records), dtype=RECORD_DTYPE)
    output["occupancy"] = records["occupancy"]
    output["pieces"] = normalized[:, 0::2] | (normalized[:, 1::2] << 4)
    output["score"] = records["score"]
    output["result"] = records["result"]
    return output


class Archive:
    """Read in place with a seeded, position-disjoint finite holdout.

    Initialization scans only board bytes, retains sparse holdout indexes, and
    counts both partitions exactly. Training and validation share this index.
    Each read validates labels and king metadata before yielding NGNP bytes.
    No file proportional to corpus size is written. Memory is O(block size +
    holdout records), about 9 MiB of sparse indexes for this 2.37B-row corpus.
    Validation lazily retains its canonical rows in RAM (32 bytes per held-out
    record), avoiding sparse corpus reads on subsequent validation passes.
    """

    def __init__(self, patterns, block_records=262144, seed=20261006, holdout_modulus=1024):
        if block_records < 1 or block_records > np.iinfo(np.uint32).max:
            raise ValueError("Invalid archive block size")
        if holdout_modulus < 2 or holdout_modulus & (holdout_modulus - 1):
            raise ValueError("Archive holdout modulus must be a power of two >= 2")
        paths = sorted({str(Path(p).resolve()) for pattern in patterns for p in glob.glob(pattern)})
        if not paths:
            raise ValueError("No archives match --data")
        self.paths, self.maps, self.blocks, self.heldout = paths, [], [], []
        self.raw_count, holdout_count = 0, 0
        sources = []
        for path in paths:
            size = Path(path).stat().st_size
            if size % 32:
                raise ValueError(f"{path}: archive size is not a multiple of 32")
            count = size // 32
            sources.append({"path": path, "bytes": size, "records": count})
            if not count:
                continue
            shard = len(self.maps)
            records = np.memmap(path, dtype=ARCHIVE_DTYPE, mode="r", shape=(count,))
            self.maps.append(records)
            self.raw_count += count
            for start in range(0, count, block_records):
                end = min(start + block_records, count)
                selected = np.flatnonzero((board_hash(records[start:end], seed)
                                          & np.uint64(holdout_modulus - 1)) == 0).astype(np.uint32)
                self.blocks.append((shard, start, end))
                self.heldout.append(selected)
                holdout_count += len(selected)
        self.count = self.raw_count - holdout_count
        self.split = "train"
        self.split_receipt = {"hash": HASH_VERSION, "seed": seed, "modulus": holdout_modulus,
                              "holdout_remainder": 0, "block_records": block_records,
                              "sources": sources, "raw_records": self.raw_count,
                              "training_records": self.count, "holdout_records": holdout_count,
                              "contract": "identical occupied boards share partition; labels excluded"}
        if not self.count or not holdout_count:
            raise ValueError("Archive split has an empty training or holdout partition")
        # Only the sparse finite holdout is cached. Views share this RAM-only
        # list; the training path never populates it or caches corpus blocks.
        self._holdout_cache = [None] * len(self.blocks)

    def holdout(self):
        view = object.__new__(Archive)
        view.__dict__ = self.__dict__.copy()
        view.split = "holdout"
        view.count = self.split_receipt["holdout_records"]
        return view

    def batches(self, batch_size, seed, shuffle=True):
        if batch_size < 1:
            raise ValueError("Batch size must be positive")
        rng = np.random.default_rng(seed)
        order = rng.permutation(len(self.blocks)) if shuffle else range(len(self.blocks))
        pending = np.empty(0, dtype=RECORD_DTYPE)
        for block_id in order:
            shard, start, end = self.blocks[block_id]
            selected = self.heldout[block_id]
            if self.split == "train":
                keep = np.ones(end - start, dtype=bool)
                keep[selected] = False
                indexes = np.flatnonzero(keep)
                if not len(indexes):
                    continue
                if shuffle:
                    rng.shuffle(indexes)
                records = canonical_records(self.maps[shard][start + indexes])
            else:
                if not len(selected):
                    continue
                # First materialize each block in source order. This makes
                # later order/shuffle choices independent of cache warmup.
                records = self._holdout_cache[block_id]
                if records is None:
                    records = canonical_records(self.maps[shard][start + selected])
                    self._holdout_cache[block_id] = records
                if shuffle:
                    indexes = np.arange(len(records))
                    rng.shuffle(indexes)
                    records = records[indexes]
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
