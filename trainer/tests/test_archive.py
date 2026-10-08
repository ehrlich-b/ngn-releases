"""Owned fixtures for archive decoding, input-disjoint splits, and training."""

import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch

import numpy as np
import torch

from trainer.archive import ARCHIVE_DTYPE, Archive, board_hash, canonical_records
from trainer.model import decode_gpu
from trainer.ngnp import RECORD_DTYPE, Position, decode_record, decode_records, encode_record, parse_fen
from trainer.tests.synthetic import random_records


def archive_fixture(record):
    """Encode the published layout from an independently specified NGNP board."""
    position = decode_record(record)
    if position.stm:
        position = Position(position.squares ^ 56,
                            (position.pieces + 6) % 12, 0)
    order = np.argsort(position.squares)
    squares, pieces = position.squares[order], position.pieces[order]
    output = np.zeros(1, dtype=ARCHIVE_DTYPE)
    output["occupancy"] = sum(1 << int(square) for square in squares)
    for index, piece in enumerate(pieces):
        code = int(piece) + (2 if piece >= 6 else 0)
        output["pieces"][0, index // 2] |= code << (4 * (index % 2))
    output["score"] = int(record["score"]) * (-1 if record["stm"] else 1)
    output["result"] = 2 - record["result"] if record["stm"] else record["result"]
    output["us_king"] = squares[pieces == 5][0]
    output["them_king"] = int(squares[pieces == 11][0]) ^ 56
    return output[0]


class ArchiveTests(unittest.TestCase):
    def test_black_normalization_preserves_features_and_label_sign(self):
        original = encode_record(parse_fen("4k3/1p3r2/8/3Q4/8/8/2P5/4K3 b - - 0 1"),
                                 312, result=2)
        archive = np.asarray([archive_fixture(original)], dtype=ARCHIVE_DTYPE)
        canonical = canonical_records(archive)
        self.assertEqual(int(canonical[0]["stm"]), 0)
        self.assertEqual(int(canonical[0]["score"]), -312)
        self.assertEqual(int(canonical[0]["result"]), 0)
        old = decode_gpu(torch.from_numpy(np.asarray([original], dtype=RECORD_DTYPE).view(np.uint8).reshape(1, 32)), 8)
        new = decode_gpu(torch.from_numpy(canonical.view(np.uint8).reshape(1, 32)), 8)
        torch.testing.assert_close(new[0][0], old[0][1])
        torch.testing.assert_close(new[0][1], old[0][0])
        for index in (2, 3, 4):
            torch.testing.assert_close(new[index], old[index])

    def test_real_archive_record_and_opponent_king_square_convention(self):
        # First observed corpus record; this byte fixture is data, never code.
        raw = bytes.fromhex("2000420148406000b58003880d00000000000000000000000bff00050e000000")
        archive = np.frombuffer(raw, dtype=ARCHIVE_DTYPE)
        records = canonical_records(archive)
        board = decode_records(records)["board"][0]
        self.assertEqual(int(board[5]), 5)
        self.assertEqual(int(board[54]), 11)
        self.assertEqual(int(records[0]["score"]), -245)
        self.assertEqual(int(records[0]["result"]), 0)
        self.assertEqual(int(records[0]["stm"]), 0)
        bad = archive.copy()
        bad["them_king"] = 54
        with self.assertRaisesRegex(ValueError, "metadata"):
            canonical_records(bad)

    def test_padding_is_ignored_and_hash_excludes_labels(self):
        original = encode_record(parse_fen("4k3/8/8/8/8/8/3P4/4K3 w - - 0 1"), 91)
        records = np.repeat(np.asarray([archive_fixture(original)], dtype=ARCHIVE_DTYPE), 3)
        records[1]["score"], records[1]["result"] = -1200, 2
        records[2]["pieces"][1] |= 0xF0  # unused high nibble after the third piece
        records[2]["pieces"][2:] = 0xFF
        records[2]["reserved"] = 0xFF
        hashed = board_hash(records, 17)
        self.assertTrue(np.all(hashed == hashed[0]))
        decoded = canonical_records(records)
        np.testing.assert_array_equal(decoded["pieces"][0], decoded["pieces"][2])

    def test_rejects_bad_piece_result_missing_king_and_metadata(self):
        original = encode_record(parse_fen("4k3/8/8/8/8/8/3P4/4K3 w - - 0 1"), 91)
        fixture = np.asarray([archive_fixture(original)], dtype=ARCHIVE_DTYPE)
        for field, value in (("result", 3), ("us_king", 64), ("them_king", 0)):
            bad = fixture.copy()
            bad[field] = value
            with self.assertRaises(ValueError):
                canonical_records(bad)
        for code in (6, 7, 14, 15, 0):
            bad = fixture.copy()
            bad["pieces"][0, 0] = (bad["pieces"][0, 0] & 0xF0) | code
            with self.assertRaises(ValueError):
                canonical_records(bad)

    def test_split_counts_global_duplicate_exclusion_seed_and_tails(self):
        source = random_records(256, np.random.default_rng(731))
        records = np.asarray([archive_fixture(record) for record in source], dtype=ARCHIVE_DTYPE)
        # Every board occurs twice, far apart and with different target labels.
        duplicate = records.copy()
        duplicate["score"] = -duplicate["score"]
        duplicate["result"] = 2 - duplicate["result"]
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "archive.bf"
            np.concatenate((records, duplicate)).tofile(path)
            train = Archive([str(path)], block_records=31, seed=992, holdout_modulus=8)
            holdout = train.holdout()
            self.assertIs(train.maps, holdout.maps)
            self.assertEqual(train.raw_count, 512)
            self.assertEqual(train.count + holdout.count, 512)
            def rows(reader, shuffle=False):
                return np.concatenate(list(reader.batches(19, 144, shuffle)))
            training, validation = rows(train), rows(holdout)
            self.assertEqual(len(training), train.count)
            self.assertEqual(len(validation), holdout.count)
            train_boards = {bytes(row[:24]) for row in training}
            validation_boards = {bytes(row[:24]) for row in validation}
            self.assertFalse(train_boards & validation_boards)
            self.assertEqual(len(train_boards) + len(validation_boards), 256)
            np.testing.assert_array_equal(rows(train, True), rows(train, True))
            self.assertEqual({bytes(row) for row in rows(train, True)}, {bytes(row) for row in training})
            repeated = Archive([str(path)], block_records=31, seed=992, holdout_modulus=8)
            np.testing.assert_array_equal(rows(repeated.holdout()), validation)
            other_seed = Archive([str(path)], block_records=31, seed=993, holdout_modulus=8)
            self.assertNotEqual({bytes(row[:24]) for row in rows(other_seed.holdout())}, validation_boards)
            path.write_bytes(path.read_bytes() + b"x")
            with self.assertRaisesRegex(ValueError, "multiple of 32"):
                Archive([str(path)])

    def test_cpu_training_resume_and_archive_config(self):
        source = random_records(96, np.random.default_rng(732))
        records = np.asarray([archive_fixture(record) for record in source], dtype=ARCHIVE_DTYPE)
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "archive.bf"
            output = Path(directory) / "train"
            records.tofile(path)
            command = [sys.executable, "-m", "trainer.train", "--data", str(path), "--data-format", "archive",
                       "--archive-holdout-modulus", "4", "--archive-split-seed", "992",
                       "--out", str(output), "--hidden", "16", "--buckets", "8", "--epochs", "1",
                       "--batch", "19", "--block-records", "31", "--device", "cpu", "--threads", "2",
                       "--precision", "fp32", "--keep-best-only"]
            trained = subprocess.run(command, capture_output=True, text=True, timeout=60)
            self.assertEqual(trained.returncode, 0, trained.stderr)
            config = json.loads((output / "config.json").read_text())
            split = config["archive_split"]
            self.assertEqual(split["raw_records"], 96)
            self.assertEqual(split["training_records"] + split["holdout_records"], 96)
            history = json.loads((output / "history.json").read_text())
            self.assertEqual(history[0]["positions"], split["training_records"])
            self.assertTrue(np.isfinite(history[0]["validation_loss"]))
            resume = command + ["--resume", str(output / "best.pt")]
            continued = subprocess.run(resume, capture_output=True, text=True, timeout=60)
            self.assertEqual(continued.returncode, 0, continued.stderr)
            changed = resume.copy()
            changed[changed.index("--archive-split-seed") + 1] = "993"
            rejected = subprocess.run(changed, capture_output=True, text=True, timeout=60)
            self.assertNotEqual(rejected.returncode, 0)
            self.assertIn("Resume options differ", rejected.stderr)
            rejected_filter = subprocess.run(command + ["--min-ply", "1"],
                                             capture_output=True, text=True, timeout=60)
            self.assertNotEqual(rejected_filter.returncode, 0)
            self.assertIn("lacks NGNP flags/ply", rejected_filter.stderr)

    def test_holdout_cache_preserves_rows_and_avoids_source_on_later_passes(self):
        source = random_records(256, np.random.default_rng(733))
        records = np.asarray([archive_fixture(record) for record in source], dtype=ARCHIVE_DTYPE)
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "archive.bf"
            records.tofile(path)
            train = Archive([str(path)], block_records=31, seed=992, holdout_modulus=4)
            holdout = train.holdout()
            first = np.concatenate(list(holdout.batches(19, 144, shuffle=False)))
            self.assertEqual(len(first), holdout.count)
            self.assertEqual(sum(block.nbytes for block in holdout._holdout_cache if block is not None),
                             holdout.count * 32)
            cold = Archive([str(path)], block_records=31, seed=992, holdout_modulus=4).holdout()
            cold_shuffled = np.concatenate(list(cold.batches(19, 144, shuffle=True)))
            # A second view shares the same cache. Removing all source maps and
            # making conversion fail proves later validation never rereads it.
            second_view = train.holdout()
            second_view.maps = None
            with patch("trainer.archive.canonical_records", side_effect=AssertionError("source conversion")):
                second = np.concatenate(list(second_view.batches(19, 144, shuffle=False)))
                shuffled = np.concatenate(list(second_view.batches(19, 144, shuffle=True)))
                repeated = np.concatenate(list(second_view.batches(19, 144, shuffle=True)))
                np.testing.assert_array_equal(first, second)
                np.testing.assert_array_equal(cold_shuffled, shuffled)
                np.testing.assert_array_equal(shuffled, repeated)
                self.assertEqual({bytes(row) for row in first}, {bytes(row) for row in shuffled})
                # Training still streams/converts records instead of caching.
                with self.assertRaisesRegex(AssertionError, "source conversion"):
                    next(train.batches(19, 144, shuffle=False))


if __name__ == "__main__":
    unittest.main()
