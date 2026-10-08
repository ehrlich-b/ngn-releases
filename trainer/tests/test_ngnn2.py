"""Material-output bucket contracts, run only on WSL."""

import json
from pathlib import Path
import struct
import subprocess
import sys
import tempfile
import unittest
import zlib

import numpy as np
import torch

from trainer.export import export_state
from trainer.make_random_net import generate
from trainer.model import NGNN, blended_loss, decode_gpu
from trainer.ngnp import Position, Shards, encode_record, parse_fen
from trainer.refeval import Network, trunc_div
from trainer.tests.synthetic import random_records
from trainer.train import validation_loss


def material_positions():
    start = parse_fen("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w - - 0 1")
    order = np.concatenate((np.flatnonzero(start.pieces % 6 == 5),
                            np.flatnonzero(start.pieces % 6 != 5)))
    return [Position(start.squares[order[:count]], start.pieces[order[:count]], stm)
            for count in range(2, 33) for stm in (0, 1)]


class BucketTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="ngn-buckets-test-")
        self.path = Path(self.temporary.name)

    def tearDown(self):
        self.temporary.cleanup()

    def check_decode(self, device):
        positions = material_positions()
        records = np.asarray([encode_record(pos, 0, 1) for pos in positions])
        raw = torch.from_numpy(records.view(np.uint8).reshape(-1, 32)).to(device)
        for nb in (1, 2, 4, 8):
            *_, buckets = decode_gpu(raw, nb)
            self.assertEqual(buckets.device.type, device)
            self.assertEqual(buckets.dtype, torch.int64)
            self.assertEqual(buckets.cpu().tolist(), [(len(p.squares) - 2) * nb // 32 for p in positions])
            model = NGNN(16, nb).to(device)
            with torch.no_grad():
                model.o.zero_()
                if nb == 1:
                    model.ob.fill_(0.125)
                else:
                    model.ob.copy_(torch.arange(nb, device=device) * 0.125)
                features, stm, _, _, bucket = decode_gpu(raw, nb)
                expected = torch.full((len(positions),), 50., device=device) if nb == 1 else bucket.float() * 50
                torch.testing.assert_close(model(features, stm, bucket), expected, rtol=0, atol=0)

    def test_cpu_buckets(self):
        self.check_decode("cpu")

    @unittest.skipUnless(torch.cuda.is_available(), "CUDA unavailable")
    def test_gpu_buckets(self):
        self.check_decode("cuda")

    def test_single_bucket_exact_equivalence(self):
        model = NGNN(32)
        export_state(model.state_dict(), self.path / "v1.nnue")
        export_state(model.state_dict(), self.path / "v2.nnue", format=2)
        a, b = Network(self.path / "v1.nnue"), Network(self.path / "v2.nnue")
        self.assertEqual(b.version, 2)
        self.assertEqual(b.buckets, 1)
        # Header and CRC differ; every byte of quantized parameters is identical.
        self.assertEqual((self.path / "v1.nnue").read_bytes()[24:-4],
                         (self.path / "v2.nnue").read_bytes()[28:-4])
        for position in material_positions():
            self.assertEqual(a.evaluate(position), b.evaluate(position))

    def test_reference_float_and_selected_gradients(self):
        torch.manual_seed(123)
        positions = material_positions()
        records = np.asarray([encode_record(pos, 0, 1) for pos in positions])
        raw = torch.from_numpy(records.view(np.uint8).reshape(-1, 32))
        for nb in (2, 4, 8):
            features, stm, _, _, bucket = decode_gpu(raw, nb)
            model = NGNN(16, nb)
            with torch.no_grad():
                model.o.normal_(0, 0.025)
                model.ob.uniform_(-0.1, 0.1)
                floated = model(features, stm, bucket).numpy()
            export_state(model.state_dict(), self.path / "net.nnue")
            network = Network(self.path / "net.nnue")
            for index, position in enumerate(positions):
                b = (len(position.squares) - 2) * nb // 32
                total = 0
                # Independent scalar feature/activation/output arithmetic.
                for half, perspective in enumerate((position.stm, 1 - position.stm)):
                    for neuron in range(16):
                        acc = int(network.b1[neuron])
                        for square, piece in zip(position.squares, position.pieces):
                            feature = (int(piece) // 6 != perspective) * 384 + (int(piece) % 6) * 64
                            feature += int(square) ^ (56 * perspective)
                            acc += int(network.w1[feature, neuron])
                        total += max(0, min(255, acc)) ** 2 * int(network.o[b, half * 16 + neuron])
                expected = trunc_div((trunc_div(total, 255) + int(network.ob[b])) * 400, 255 * 64)
                self.assertEqual(network.evaluate(position), expected)
                self.assertLess(abs(expected - floated[index]), 5)
            # A batch in bucket zero must not update any other output row.
            model.zero_grad()
            model(features[:, :], stm, torch.zeros_like(bucket)).sum().backward()
            self.assertTrue(torch.count_nonzero(model.o.grad[0]).item() > 0)
            self.assertEqual(torch.count_nonzero(model.o.grad[1:]).item(), 0)
            self.assertEqual(torch.count_nonzero(model.ob.grad[1:]).item(), 0)

    def test_layout_validation_and_full_weight_range(self):
        model = NGNN(32, 8)
        export_state(model.state_dict(), self.path / "net.nnue")
        good = (self.path / "net.nnue").read_bytes()
        self.assertEqual(len(good), 28 + 769 * 32 * 2 + 8 * 64 * 2 + 8 * 4 + 4)
        self.assertEqual(struct.unpack_from("<4s6I", good), (b"NGN2", 2, 32, 8, 255, 64, 400))
        for offset, value in ((0, 0), (4, 1), (8, 0), (8, 17), (8, 2064), (8, 0xffffffff),
                              (12, 0), (12, 3), (12, 16), (12, 0xffffffff),
                              (16, 254), (20, 65), (24, 401)):
            bad = bytearray(good)
            struct.pack_into("<I", bad, offset, value)
            struct.pack_into("<I", bad, len(bad) - 4, zlib.crc32(bad[:-4]))
            (self.path / "bad.nnue").write_bytes(bad)
            with self.assertRaises(ValueError):
                Network(self.path / "bad.nnue")
        bad = bytearray(good)
        bad[100] ^= 1
        for malformed in (good[:27], good[:-1], good + b"\0", bad):
            (self.path / "bad.nnue").write_bytes(malformed)
            with self.assertRaises(ValueError):
                Network(self.path / "bad.nnue")
        with self.assertRaises(ValueError):
            NGNN(16, 3)
        with self.assertRaises(ValueError):
            export_state(model.state_dict(), self.path / "bad.nnue", format=1)
        # Loaders accept int16 endpoints; arithmetic must widen before sums.
        bad = bytearray(good)
        output_offset = 28 + 769 * 32 * 2
        np.frombuffer(bad, dtype="<i2", count=769 * 32, offset=28)[:] = 32767
        np.frombuffer(bad, dtype="<i2", count=8 * 64, offset=output_offset)[:] = -32768
        struct.pack_into("<I", bad, len(bad) - 4, zlib.crc32(bad[:-4]))
        (self.path / "wide.nnue").write_bytes(bad)
        network = Network(self.path / "wide.nnue")
        self.assertTrue(np.all(network.o == -32768))
        for position in material_positions():
            bucket = network.bucket(position)
            expected = trunc_div((trunc_div(64 * 255 ** 2 * -32768, 255) + int(network.ob[bucket])) * 400, 255 * 64)
            self.assertEqual(network.evaluate(position), expected)

    def test_bucket_validation_tail_and_fixture(self):
        records = random_records(19, np.random.default_rng(34))
        records.tofile(self.path / "heldout.ngnp")
        shards = Shards([str(self.path / "heldout.ngnp")], drop_flags=0)
        model = NGNN(16, 8)
        features, stm, score, result, bucket = decode_gpu(torch.from_numpy(records.view(np.uint8).reshape(-1, 32)), 8)
        with torch.no_grad():
            expected = blended_loss(model(features, stm, bucket), score, result).item()
        self.assertAlmostEqual(validation_loss(model, shards, 8, torch.device("cpu"), 0.75, 400), expected, places=7)
        payload = generate(self.path / "a.nnue", self.path / "a.json", buckets=8)
        generate(self.path / "b.nnue", self.path / "b.json", buckets=8)
        self.assertEqual((self.path / "a.nnue").read_bytes(), (self.path / "b.nnue").read_bytes())
        self.assertEqual((self.path / "a.json").read_bytes(), (self.path / "b.json").read_bytes())
        self.assertEqual({case["bucket"] for case in payload["cases"]}, set(range(8)))
        committed = Path("testdata/ngnn2/random_h32_b8.nnue")
        if committed.exists():
            self.assertEqual(committed.read_bytes(), (self.path / "a.nnue").read_bytes())
            self.assertEqual(json.loads(committed.with_name("random_h32_b8_evals.json").read_text()), payload)

    def test_bucket_resume_and_export_cli(self):
        random_records(64, np.random.default_rng(59)).tofile(self.path / "data.ngnp")
        common = [sys.executable, "-m", "trainer.train", "--data", str(self.path / "data.ngnp"),
                  "--hidden", "16", "--buckets", "8", "--batch", "32", "--device", "cpu",
                  "--precision", "fp32", "--threads", "2", "--decay", "none"]
        for out, epochs, resume in (("full", "2", None), ("resumed", "1", None),
                                     ("resumed", "2", self.path / "resumed/epoch-0001.pt")):
            command = common + ["--out", str(self.path / out), "--epochs", epochs]
            if resume:
                command += ["--resume", str(resume)]
            subprocess.run(command, check=True, capture_output=True, timeout=60)
        a = torch.load(self.path / "full/epoch-0002.pt", weights_only=True)
        b = torch.load(self.path / "resumed/epoch-0002.pt", weights_only=True)
        for name in a["model"]:
            self.assertTrue(torch.equal(a["model"][name], b["model"][name]), name)
        subprocess.run([sys.executable, "-m", "trainer.export", str(self.path / "resumed/epoch-0002.pt"),
                        str(self.path / "export.nnue")], check=True, timeout=60)
        self.assertEqual((self.path / "export.nnue").read_bytes(),
                         (self.path / "resumed/epoch-0002.nnue").read_bytes())
        changed = common + ["--out", str(self.path / "bad"), "--epochs", "2", "--buckets", "4",
                            "--resume", str(self.path / "resumed/epoch-0001.pt")]
        rejected = subprocess.run(changed, capture_output=True, timeout=60)
        self.assertNotEqual(rejected.returncode, 0)
        self.assertIn(b"Resume options differ: buckets", rejected.stderr)

    def test_single_bucket_format2_cli(self):
        random_records(32, np.random.default_rng(73)).tofile(self.path / "data.ngnp")
        command = [sys.executable, "-m", "trainer.train", "--data", str(self.path / "data.ngnp"),
                   "--out", str(self.path / "trained"), "--hidden", "16", "--buckets", "1", "--format", "2",
                   "--epochs", "1", "--batch", "16", "--threads", "2", "--device", "cpu"]
        subprocess.run(command, check=True, capture_output=True, timeout=60)
        path = self.path / "trained/epoch-0001.nnue"
        network = Network(path)
        self.assertEqual((network.version, network.buckets), (2, 1))
        checkpoint = self.path / "trained/epoch-0001.pt"
        subprocess.run([sys.executable, "-m", "trainer.export", str(checkpoint), str(self.path / "copy.nnue")],
                       check=True, capture_output=True, timeout=60)
        self.assertEqual(path.read_bytes(), (self.path / "copy.nnue").read_bytes())
        subprocess.run([sys.executable, "-m", "trainer.export", str(checkpoint), str(self.path / "legacy.nnue"),
                        "--format", "1"], check=True, capture_output=True, timeout=60)
        legacy = Network(self.path / "legacy.nnue")
        for position in material_positions():
            self.assertEqual(network.evaluate(position), legacy.evaluate(position))


if __name__ == "__main__":
    torch.set_num_threads(4)
    unittest.main()
