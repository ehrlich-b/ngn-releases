"""Optional board-disjoint holdout for NGN-owned self-play shards."""

import copy
from pathlib import Path

import numpy as np

from trainer.archive import HASH_VERSION, board_hash
from trainer.ngnp import Shards


class ContentShards(Shards):
    """Share read-only shard maps; partition only records surviving NGNP filters.

    Use NGN's existing occupied-board hash. Labels, STM, ply, flags and inactive
    packed padding do not choose the partition. In particular, the same board
    with either STM stays together. This is a board holdout, not a game holdout
    or a grouping of all symmetry-equivalent network inputs.
    """

    def __init__(self, patterns, drop_flags=0x0F, min_ply=0, block_records=262144,
                 require_kings=False, record_counts=None, modulus=1024, seed=20261008):
        if modulus < 2 or modulus & (modulus - 1):
            raise ValueError("NGNP holdout modulus must be a power of two >= 2")
        # The ordinary reader validates all records and indexes every block
        # containing eligible rows, including blocks entirely assigned holdout.
        super().__init__(patterns, drop_flags, min_ply, block_records, require_kings, record_counts)
        eligible = self.count
        heldout = 0
        for shard, start, end in self.blocks:
            block = self.maps[shard][start:end]
            heldout += int((super().mask(block)
                            & ((board_hash(block, seed) & np.uint64(modulus - 1)) == 0)).sum())
        if heldout == 0 or heldout == eligible:
            raise ValueError("NGNP split has an empty training or holdout partition")
        self.modulus, self.split_seed, self.split = modulus, seed, "train"
        self.count = eligible - heldout
        # Record mapped prefixes rather than current file sizes: a snapshot's
        # later appends cannot alter either the split or resume identity.
        self.split_receipt = {
            "hash": HASH_VERSION, "seed": seed, "modulus": modulus,
            "holdout_remainder": 0, "block_records": block_records,
            "sources": [{"path": str(Path(m.filename).resolve()), "records": len(m)} for m in self.maps],
            "raw_records": self.raw_count, "eligible_records": eligible,
            "training_records": self.count, "holdout_records": heldout,
            "contract": "identical occupied boards share partition; labels and STM excluded",
        }

    def holdout(self):
        view = copy.copy(self)
        view.split = "holdout"
        view.count = self.split_receipt["holdout_records"]
        return view

    def mask(self, records):
        eligible = super().mask(records)
        if not hasattr(self, "split"):
            return eligible
        selected = (board_hash(records, self.split_seed) & np.uint64(self.modulus - 1)) == 0
        return eligible & (selected if self.split == "holdout" else ~selected)
