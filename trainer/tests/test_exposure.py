"""Exact per-source presentation counts across batches, tails and cycles."""

from pathlib import Path
import tempfile
import unittest

import numpy as np

from trainer.archive import ARCHIVE_DTYPE, Archive
from trainer.ngnp import RECORD_DTYPE
from trainer.train import epoch_batches, prefetched


class ExposureTest(unittest.TestCase):
    def test_source_counts_match_admitted_rows(self):
        with tempfile.TemporaryDirectory() as directory:
            paths = []
            # Distinct king squares give a nonempty content-hash holdout.
            for source in range(2):
                rows = np.zeros(97 + source * 13, dtype=ARCHIVE_DTYPE)
                for i, row in enumerate(rows):
                    white, black = i % 32, 32 + (i * 7) % 32
                    row["occupancy"] = (1 << white) | (1 << black)
                    row["pieces"][0] = 5 | (13 << 4)
                    row["us_king"], row["them_king"] = white, black ^ 56
                    row["score"], row["result"] = source + 1, i % 3
                path = Path(directory) / f"source-{source}.bf"
                rows.tofile(path)
                paths.append(str(path))
            for batch in (1, 17, 79, 256):
                for positions in (1, 51, 301):
                    with self.subTest(batch=batch, positions=positions):
                        shards = Archive(paths, block_records=23, holdout_modulus=8)
                        expected = [0, 0]
                        admitted = []
                        for raw in prefetched(epoch_batches(shards, batch, positions, 123)):
                            admitted.append(raw.copy())
                            for score in raw.copy().view(RECORD_DTYPE).reshape(-1)["score"]:
                                expected[int(score) - 1] += 1
                        self.assertEqual(sum(expected), positions)
                        self.assertEqual(shards.exposure_counts, expected)
                        # Logging leaves bytes and sampling order unchanged.
                        control = Archive(paths, block_records=23, holdout_modulus=8)
                        control.record_exposure = lambda take: None
                        original = list(epoch_batches(control, batch, positions, 123))
                        self.assertTrue(np.array_equal(np.concatenate(admitted), np.concatenate(original)))
                        list(shards.holdout().batches(batch, 0, shuffle=False))
                        self.assertEqual(shards.exposure_counts, expected)


if __name__ == "__main__":
    unittest.main()
