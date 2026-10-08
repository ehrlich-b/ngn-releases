"""NGNN3 layout, GPU features, export and resume checks using only owned data."""

import hashlib
import json
import os
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
from trainer.king_buckets import DEFAULT_MAP, feature_indices, validate_map
from trainer.make_random_net import generate
from trainer.model import NGNN, decode_gpu
from trainer.ngnp import Position, Shards, encode_record, parse_fen, RECORD_DTYPE, to_fen
from trainer.refeval import Network


def positions():
    # All king squares, asymmetric material, both turns, both color perspectives.
    result = []
    for king in range(64):
        occupied = [king, 63-king]
        extras = [s for s in (1, 8, 17, 30, 45, 54, 62) if s not in occupied]
        result.append(Position(np.asarray(occupied + extras),
                               np.asarray([5, 11] + [i % 5 + (6 if i % 2 else 0)
                                                     for i in range(len(extras))]), king % 2))
    return result


class NGNN3Tests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        torch.set_num_threads(2)

    def test_default_layout(self):
        expected = [[0, 0, 1, 1], [2, 2, 3, 3], [4]*4, [5]*4,
                    [6]*4, [6]*4, [7]*4, [7]*4]
        for rank in range(8):
            self.assertEqual(list(DEFAULT_MAP[rank*8:rank*8+4]), expected[rank])
            self.assertEqual(list(DEFAULT_MAP[rank*8+4:rank*8+8]), expected[rank][::-1])
        for bad in ([0]*63, [0.0]*64, [-1]*64, [8]*64):
            with self.assertRaises(ValueError):
                validate_map(bad)

    def check_decode(self, device, mapping=DEFAULT_MAP):
        samples = positions()
        records = np.asarray([encode_record(pos, i-30) for i, pos in enumerate(samples)], dtype=RECORD_DTYPE)
        raw = torch.from_numpy(records.view(np.uint8).reshape(-1, 32)).to(device)
        table = torch.tensor(mapping, dtype=torch.int64, device=device)
        features, stm, score, result, bucket = decode_gpu(raw, 8, table, max(mapping)+1)
        expected = np.zeros(features.shape, dtype=np.float32)
        for p in (0, 1):
            for i, pos in enumerate(samples):
                expected[p*len(samples)+i, feature_indices(pos, p, mapping)] = 1
        np.testing.assert_array_equal(features.cpu().numpy(), expected)
        np.testing.assert_array_equal(stm.cpu().numpy(), records["stm"])
        np.testing.assert_array_equal(score.cpu().numpy(), records["score"]*(1-2*records["stm"].astype(int)))
        np.testing.assert_array_equal(result.cpu().numpy(), np.full(len(samples), 0.5))
        np.testing.assert_array_equal(bucket.cpu().numpy(), [(len(pos.squares)-2)*8//32 for pos in samples])

    def test_cpu_decode(self):
        self.check_decode("cpu")
        self.check_decode("cpu", tuple((i//8)%4 for i in range(64)))

    @unittest.skipUnless(torch.cuda.is_available(), "CUDA unavailable")
    def test_cuda_decode(self):
        self.check_decode("cuda")
        self.check_decode("cuda", tuple((i//8)%4 for i in range(64)))

    def test_features_symmetry(self):
        for pos in positions():
            for flip in (7, 56, 63):
                swapped = bool(flip & 56)
                codes = (pos.pieces+6)%12 if swapped else pos.pieces
                other = Position(pos.squares ^ flip, codes, pos.stm ^ swapped)
                for p in (0, 1):
                    np.testing.assert_array_equal(feature_indices(pos, p, DEFAULT_MAP),
                                                  feature_indices(other, p ^ swapped, DEFAULT_MAP))

    def test_export_shapes_header_and_reference(self):
        torch.manual_seed(876)
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory)/"net.nnue"
            model = NGNN(32, 8, DEFAULT_MAP)
            export_state(model.state_dict(), path, 3)
            raw = path.read_bytes()
            self.assertEqual(struct.unpack_from("<4s7I", raw), (b"NGN3", 3, 32, 8, 8, 255, 64, 400))
            self.assertEqual(raw[32:96], bytes(DEFAULT_MAP))
            self.assertEqual(len(raw), 96+2*(6144*32+32+8*64)+8*4+4)
            self.assertEqual(zlib.crc32(raw[:-4]), struct.unpack_from("<I", raw, len(raw)-4)[0])
            network = Network(path)
            records = np.asarray([encode_record(pos, 0) for pos in positions()], dtype=RECORD_DTYPE)
            features, stm, _, _, bucket = decode_gpu(torch.from_numpy(records.view(np.uint8).reshape(-1, 32)),
                                                      8, model.king_map, 8)
            cp = model(features, stm, bucket).detach().numpy()
            integer_cp = np.asarray([network.evaluate(pos) for pos in positions()])
            self.assertLess(np.max(np.abs(cp-integer_cp)), 10)
            for format in (1, 2):
                with self.assertRaises(ValueError):
                    export_state(model.state_dict(), path, format)
            with self.assertRaises(ValueError):
                export_state(NGNN(32, 8).state_dict(), path, 3)
            bad = model.state_dict().copy()
            bad["w1"] = torch.zeros(768, 32)
            with self.assertRaises(ValueError):
                export_state(bad, path, 3)

    def test_custom_map_export(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory)/"net.nnue"
            mapping = tuple((i//8+1)%4 for i in range(64))
            model = NGNN(16, 1, mapping)
            self.assertEqual(model.w1.shape, (3072, 16))
            export_state(model.state_dict(), path, 3)
            net = Network(path)
            self.assertEqual(net.king_map, mapping)
            self.assertEqual(net.king_buckets, 4)
            for pos in positions():
                net.evaluate(pos)

    def test_reference_rejects_invalid_files(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory)/"net.nnue"
            export_state(NGNN(16, 8, DEFAULT_MAP).state_dict(), path, 3)
            good = path.read_bytes()
            for offset, value in ((12, 9), (16, 3), (20, 254), (32, 8), (95, 255)):
                data = bytearray(good)
                if offset < 32:
                    struct.pack_into("<I", data, offset, value)
                else:
                    data[offset] = value
                struct.pack_into("<I", data, len(data)-4, zlib.crc32(data[:-4]))
                path.write_bytes(data)
                with self.assertRaises(ValueError):
                    Network(path)
            for bad in (good[:95], good[:-1], good+b"x", good[:100]+b"x"+good[101:]):
                path.write_bytes(bad)
                with self.assertRaises(ValueError):
                    Network(path)

    def test_selected_input_and_output_gradients(self):
        model = NGNN(16, 8, DEFAULT_MAP)
        # One perspective in bucket 0 and the other in bucket 1.
        pos = parse_fen("3k4/8/8/8/8/8/8/K7 w - - 0 1")
        raw = np.asarray([encode_record(pos, 0)], dtype=RECORD_DTYPE).view(np.uint8).reshape(-1, 32)
        features, stm, _, _, bucket = decode_gpu(torch.from_numpy(raw), 8, model.king_map, 8)
        model(features, stm, bucket).sum().backward()
        grad = model.w1.grad.reshape(8, 768, 16).abs().sum(dim=(1, 2))
        self.assertGreater(grad[0].item(), 0)
        self.assertGreater(grad[1].item(), 0)
        self.assertEqual(grad[2:].sum().item(), 0)
        self.assertGreater(model.o.grad[0].abs().sum().item(), 0)
        self.assertEqual(model.o.grad[1:].abs().sum().item(), 0)

    def test_missing_kings_rejected(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory)/"bad.ngnp"
            np.asarray([encode_record(parse_fen("8/8/8/8/8/8/8/K7 w - - 0 1"), 0)], dtype=RECORD_DTYPE).tofile(path)
            with self.assertRaisesRegex(ValueError, "one king"):
                Shards([str(path)], require_kings=True)
            self.assertEqual(Shards([str(path)]).count, 1)

    def test_fixture_reproducible(self):
        with tempfile.TemporaryDirectory() as directory:
            path, evals = Path(directory)/"random.nnue", Path(directory)/"evals.json"
            payload = generate(path, evals, 32, 20261006, 8, 3)
            committed = Path("testdata/ngnn3/random_h32_k8_b8.nnue")
            self.assertEqual(path.read_bytes(), committed.read_bytes())
            self.assertEqual(payload, json.loads(Path("testdata/ngnn3/random_h32_k8_b8_evals.json").read_text()))
            self.assertEqual(payload["network_sha256"], hashlib.sha256(path.read_bytes()).hexdigest())

    def test_cli_resume_and_map_guard(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            shard = root/"data.ngnp"
            records = np.asarray([encode_record(pos, i*5) for i, pos in enumerate(positions())], dtype=RECORD_DTYPE)
            records.tofile(shard)
            common = [sys.executable, "-m", "trainer.train", "--data", str(shard), "--hidden", "16",
                      "--buckets", "8", "--format", "3", "--kb-map", "default", "--batch", "32",
                      "--decay", "none", "--device", "cpu", "--precision", "fp32", "--threads", "2"]
            env = dict(os.environ, OMP_NUM_THREADS="2", OPENBLAS_NUM_THREADS="2", MKL_NUM_THREADS="2")
            def run(extra, succeeds=True):
                proc = subprocess.run(common+extra, env=env, capture_output=True, text=True, timeout=60)
                self.assertEqual(proc.returncode == 0, succeeds, proc.stdout+proc.stderr)
                return proc
            run(["--out", str(root/"full"), "--epochs", "2"])
            run(["--out", str(root/"resumed"), "--epochs", "1"])
            resume = root/"resumed/epoch-0001.pt"
            run(["--out", str(root/"resumed"), "--epochs", "2", "--resume", str(resume)])
            self.assertEqual((root/"full/epoch-0002.nnue").read_bytes(), (root/"resumed/epoch-0002.nnue").read_bytes())
            # Even a same-sized changed mapping must fail before loading optimizer state.
            mapping = list(DEFAULT_MAP)
            mapping[0] = 1
            map_path = root/"changed.json"
            map_path.write_text(json.dumps(mapping))
            proc = run(["--out", str(root/"changed"), "--epochs", "2", "--resume", str(resume),
                        "--kb-map", str(map_path)], False)
            self.assertIn("king_map", proc.stderr)
            proc = subprocess.run([sys.executable, "-m", "trainer.export", str(root/"resumed/epoch-0002.pt"),
                                   str(root/"reexport.nnue")], env=env, capture_output=True, text=True, timeout=60)
            self.assertEqual(proc.returncode, 0, proc.stderr)
            self.assertEqual((root/"reexport.nnue").read_bytes(), (root/"full/epoch-0002.nnue").read_bytes())


if __name__ == "__main__":
    unittest.main()
