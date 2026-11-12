"""Bounded archive indexing checks without allocating a multi-billion-row corpus."""
import unittest

import numpy as np

from trainer.archive import ARCHIVE_DTYPE, Archive, canonical_records


class SparseSource:
    def __init__(self, expected):
        self.expected = expected
        self.calls = 0
        self.rows = np.zeros(len(expected), dtype=ARCHIVE_DTYPE)
        self.rows["occupancy"] = np.uint64(1) | (np.uint64(1) << np.uint64(63))
        self.rows["pieces"][:, 0] = 0xD5  # White king a1, Black king h8.
        self.rows["them_king"] = 7  # Stored in Black's vertically flipped view.
        self.rows["result"] = 1
        self.rows["score"] = np.arange(len(expected)) * 17

    def __getitem__(self, indexes):
        np.testing.assert_array_equal(indexes, self.expected)
        self.calls += 1
        return self.rows


class ArchiveIndexTests(unittest.TestCase):
    def test_holdout_absolute_rows_and_cache(self):
        for start in (0, 2**32 - 2, 2**32, 2**32 + 262144):
            with self.subTest(start=start):
                offsets = np.array([0, 1, 2, 3], dtype=np.uint32)
                source = SparseSource([start + int(i) for i in offsets])
                archive = object.__new__(Archive)
                archive.maps = [source]
                archive.blocks = [(0, start, start + 4)]
                archive.heldout = [offsets]
                archive._holdout_cache = [None]
                archive.split = "holdout"
                expected = canonical_records(source.rows).view(np.uint8).reshape(-1, 32)
                for _ in range(2):
                    actual = np.concatenate(list(archive.batches(3, 0, shuffle=False)))
                    np.testing.assert_array_equal(actual, expected)
                self.assertEqual(source.calls, 1)
                np.testing.assert_array_equal(offsets, [0, 1, 2, 3])
                self.assertEqual(offsets.dtype, np.uint32)


if __name__ == "__main__":
    unittest.main()
