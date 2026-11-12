"""Board holdout, frozen-prefix and resume controls for owned self-play data."""

import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

import numpy as np

from trainer.archive import board_hash
from trainer.ngnp import RECORD_DTYPE, Shards, encode_record, parse_fen
from trainer.ngnp_split import ContentShards
from trainer.tests.synthetic import random_records


def rows(reader, shuffle=False):
    batches = list(reader.batches(19, 33, shuffle))
    return np.concatenate(batches).reshape(-1).view(RECORD_DTYPE)


class ContentShardsTests(unittest.TestCase):
    def test_duplicates_across_shards_labels_stm_padding_and_filters(self):
        source = random_records(256, np.random.default_rng(841))
        source["flags"], source["ply"] = 0, 50
        duplicate = source.copy()
        duplicate["score"] = -duplicate["score"]
        duplicate["result"] = 2 - duplicate["result"]
        duplicate["stm"] = 1 - duplicate["stm"]
        duplicate["reserved"] = 255
        # Filter one copy while retaining the other; filters remain independent
        # of the partition and do not change where the eligible copy belongs.
        source["flags"][:5] = 1
        source["ply"][5] = 2
        with tempfile.TemporaryDirectory() as directory:
            first, second = Path(directory) / "a.ngnp", Path(directory) / "b.ngnp"
            source.tofile(first)
            duplicate.tofile(second)
            train = ContentShards([str(first), str(second)], min_ply=10,
                                  block_records=31, modulus=8, seed=992)
            holdout = train.holdout()
            self.assertIs(train.maps, holdout.maps)
            self.assertEqual(train.count + holdout.count, 506)
            training, validation = rows(train), rows(holdout)
            self.assertEqual(len(training), train.count)
            self.assertEqual(len(validation), holdout.count)
            keys = lambda records: {r.tobytes()[:24] for r in records}
            self.assertFalse(keys(training) & keys(validation))
            self.assertEqual(len(keys(training) | keys(validation)), 256)
            np.testing.assert_array_equal(rows(train, True), rows(train, True))
            self.assertEqual({r.tobytes() for r in rows(train, True)}, {r.tobytes() for r in training})
            other = ContentShards([str(first), str(second)], min_ply=10,
                                  block_records=31, modulus=8, seed=993)
            self.assertNotEqual(keys(rows(other.holdout())), keys(validation))

    def test_unused_nibbles_and_metadata_do_not_change_hash(self):
        record = encode_record(parse_fen("4k3/8/8/8/8/8/3P4/4K3 w - - 0 1"), 91)
        records = np.repeat(np.asarray([record], dtype=RECORD_DTYPE), 3)
        records[1]["pieces"][1] |= 0xF0
        records[1]["pieces"][2:] = 255
        records[1]["score"], records[1]["result"], records[1]["stm"] = -900, 2, 1
        records[2]["flags"], records[2]["ply"], records[2]["reserved"] = 15, 300, 255
        hashed = board_hash(records, 992)
        self.assertTrue(np.all(hashed == hashed[0]))

    def test_entire_holdout_blocks_and_partial_batches_are_retained(self):
        records = random_records(128, np.random.default_rng(842))
        records["flags"], records["ply"] = 0, 50
        selected = (board_hash(records, 992) & np.uint64(3)) == 0
        records = np.concatenate((records[selected], records[~selected]))
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "a.ngnp"
            records.tofile(path)
            train = ContentShards([str(path)], block_records=1, modulus=4, seed=992)
            self.assertEqual(len(train.blocks), 128)
            self.assertEqual(train.holdout().count, int(selected.sum()))
            for reader in (train, train.holdout()):
                batches = list(reader.batches(19, 33, False))
                self.assertEqual(sum(map(len, batches)), reader.count)
                self.assertEqual(len(batches[-1]), reader.count % 19 or 19)

    def test_prefix_appends_do_not_change_split_or_receipt(self):
        records = random_records(128, np.random.default_rng(843))
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "a.ngnp"
            records.tofile(path)
            cap = {str(path): 96}
            first = ContentShards([str(path)], record_counts=cap, modulus=4, seed=992)
            with path.open("ab") as stream:
                stream.write(records.tobytes() + b"partial-tail")
            second = ContentShards([str(path)], record_counts=cap, modulus=4, seed=992)
            self.assertEqual(first.split_receipt, second.split_receipt)
            self.assertEqual(first.raw_count, 96)
            np.testing.assert_array_equal(rows(first), rows(second))
            np.testing.assert_array_equal(rows(first.holdout()), rows(second.holdout()))
            with self.assertRaisesRegex(ValueError, "multiple of 32"):
                ContentShards([str(path)], modulus=4)

    def test_bad_modulus_empty_partition_and_filtered_validation(self):
        records = random_records(128, np.random.default_rng(844))
        records["flags"] = 0
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "a.ngnp"
            records.tofile(path)
            for modulus in (-1, 0, 1, 3):
                with self.assertRaisesRegex(ValueError, "power of two"):
                    ContentShards([str(path)], modulus=modulus)
            np.repeat(records[:1], 20).tofile(path)
            with self.assertRaisesRegex(ValueError, "empty"):
                ContentShards([str(path)], modulus=4)
            records["flags"][0], records["result"][0] = 15, 3
            records.tofile(path)
            with self.assertRaisesRegex(ValueError, "invalid NGNP1"):
                ContentShards([str(path)], modulus=4)

    def test_training_resume_binds_partition_and_legacy_default(self):
        records = random_records(96, np.random.default_rng(845))
        with tempfile.TemporaryDirectory() as directory:
            path, output = Path(directory) / "a.ngnp", Path(directory) / "train"
            records.tofile(path)
            command = [sys.executable, "-m", "trainer.train", "--data", str(path),
                       "--ngnp-holdout-modulus", "4", "--ngnp-split-seed", "992",
                       "--out", str(output), "--hidden", "16", "--buckets", "8", "--epochs", "1",
                       "--batch", "19", "--block-records", "31", "--device", "cpu", "--threads", "2",
                       "--precision", "fp32", "--keep-best-only"]
            def run(argv):
                return subprocess.run(argv, capture_output=True, text=True, timeout=90)
            trained = run(command)
            self.assertEqual(trained.returncode, 0, trained.stderr)
            config = json.loads((output / "config.json").read_text())
            split = config["ngnp_split"]
            self.assertEqual(split["raw_records"], 96)
            history = json.loads((output / "history.json").read_text())
            self.assertEqual(history[0]["positions"], split["training_records"])
            self.assertTrue(np.isfinite(history[0]["validation_loss"]))
            resumed = command + ["--resume", str(output / "best.pt")]
            same = run(resumed)
            self.assertEqual(same.returncode, 0, same.stderr)
            for option, value in (("--ngnp-split-seed", "993"), ("--ngnp-holdout-modulus", "8")):
                changed = resumed.copy()
                changed[changed.index(option) + 1] = value
                rejected = run(changed)
                self.assertNotEqual(rejected.returncode, 0)
                self.assertIn("Resume options differ", rejected.stderr)
            rejected = run(command + ["--validation-data", str(path)])
            self.assertNotEqual(rejected.returncode, 0)
            self.assertIn("cannot be combined", rejected.stderr)
            legacy = Shards([str(path)])
            self.assertEqual(legacy.count, 96)
