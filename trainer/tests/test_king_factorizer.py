"""Shared-row gradients, merged quantization, and checkpoint custody controls."""

import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

import numpy as np
import torch

from trainer.export import export_state, quantize
from trainer.king_buckets import DEFAULT_MAP
from trainer.model import NGNN, QA, decode_gpu
from trainer.ngnp import RECORD_DTYPE, encode_record
from trainer.refeval import Network
from trainer.tests.test_ngnn3 import positions


class KingFactorizerTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        torch.set_num_threads(2)

    def test_zero_initialization_preserves_default_rng_and_export(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            for buckets in (1, 8):
                torch.manual_seed(887)
                ordinary = NGNN(16, buckets, DEFAULT_MAP)
                after_ordinary = torch.rand(8)
                torch.manual_seed(887)
                factorized = NGNN(16, buckets, DEFAULT_MAP, True)
                torch.testing.assert_close(torch.rand(8), after_ordinary, rtol=0, atol=0)
                self.assertNotIn("w1_shared", ordinary.state_dict())
                self.assertEqual(factorized.w1_shared.count_nonzero().item(), 0)
                torch.testing.assert_close(factorized.effective_w1(), ordinary.w1, rtol=0, atol=0)
                export_state(ordinary.state_dict(), root / "ordinary.nnue")
                export_state(factorized.state_dict(), root / "factorized.nnue")
                self.assertEqual((root / "ordinary.nnue").read_bytes(),
                                 (root / "factorized.nnue").read_bytes())

    def check_forward_gradients(self, device, amp=False):
        for mapping in (DEFAULT_MAP, tuple((i // 8) % 4 for i in range(64))):
            for buckets in (1, 8):
                torch.manual_seed(888)
                factorized = NGNN(16, buckets, mapping, True).to(device)
                with torch.no_grad():
                    factorized.w1_shared.normal_(0, 0.005)
                ordinary = NGNN(16, buckets, mapping).to(device)
                state = {k: v for k, v in factorized.state_dict().items() if k != "w1_shared"}
                state["w1"] = factorized.effective_w1().detach().clone()
                ordinary.load_state_dict(state)
                records = np.asarray([encode_record(pos, i - 30) for i, pos in enumerate(positions())],
                                     dtype=RECORD_DTYPE)
                raw = torch.from_numpy(records.view(np.uint8).reshape(-1, 32)).to(device)
                features, stm, _, _, bucket = decode_gpu(raw, buckets, factorized.king_map,
                                                        factorized.king_buckets)
                with torch.autocast(device_type=device, dtype=torch.bfloat16, enabled=amp):
                    first = factorized(features, stm, bucket)
                    second = ordinary(features, stm, bucket)
                torch.testing.assert_close(first, second, rtol=0, atol=0)
                first.sum().backward()
                second.sum().backward()
                torch.testing.assert_close(factorized.w1.grad, ordinary.w1.grad, rtol=0, atol=0)
                expected_shared = ordinary.w1.grad.reshape(factorized.king_buckets, 768, 16).sum(0)
                torch.testing.assert_close(factorized.w1_shared.grad, expected_shared, rtol=0, atol=0)
                # AdamW must actually update both components, with representable
                # effective rows after clipping, including the BF16 CUDA path.
                before_shared = factorized.w1_shared.detach().clone()
                optimizer = torch.optim.AdamW(factorized.parameters(), lr=0.001,
                                              fused=device == "cuda")
                optimizer.step()
                factorized.clip_quantized()
                self.assertFalse(torch.equal(before_shared, factorized.w1_shared))
                with tempfile.TemporaryDirectory() as directory:
                    path = Path(directory) / "net.nnue"
                    export_state(factorized.state_dict(), path)
                    network = Network(path)
                    np.testing.assert_array_equal(network.w1, quantize(factorized.effective_w1(), QA, "<i2"))

    def test_cpu_forward_and_shared_gradient(self):
        self.check_forward_gradients("cpu")

    @unittest.skipUnless(torch.cuda.is_available(), "CUDA unavailable")
    def test_cuda_forward_gradients_and_bf16_step(self):
        self.check_forward_gradients("cuda")
        self.check_forward_gradients("cuda", True)

    def test_merge_before_rounding_and_integer_reference(self):
        torch.manual_seed(889)
        model = NGNN(16, 8, DEFAULT_MAP, True)
        with torch.no_grad():
            # Each component rounds to zero; their sum rounds to one.
            model.w1.fill_(0.3 / QA)
            model.w1_shared.fill_(0.3 / QA)
        folded = {k: v for k, v in model.state_dict().items() if k != "w1_shared"}
        folded["w1"] = model.effective_w1().detach()
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            export_state(model.state_dict(), root / "shared.nnue")
            export_state(folded, root / "folded.nnue")
            self.assertEqual((root / "shared.nnue").read_bytes(), (root / "folded.nnue").read_bytes())
            shared, ordinary = Network(root / "shared.nnue"), Network(root / "folded.nnue")
            self.assertTrue(np.all(shared.w1 == 1))
            for pos in positions():
                self.assertEqual(shared.evaluate(pos), ordinary.evaluate(pos))

    def test_clipping_bounds_effective_rows_at_both_limits(self):
        model = NGNN(16, 8, DEFAULT_MAP, True)
        with torch.no_grad():
            model.w1_shared[:, ::2] = 1000
            model.w1_shared[:, 1::2] = -1000
            for i, rows in enumerate(model.w1.reshape(8, 768, 16)):
                rows.fill_(1000 if i % 2 else -1000)
            model.clip_quantized()
        effective = model.effective_w1()
        self.assertLessEqual(effective.max().item(), 32767 / QA + 0.0001)
        self.assertGreaterEqual(effective.min().item(), -32768 / QA - 0.0001)
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "clipped.nnue"
            export_state(model.state_dict(), path)
            network = Network(path)
            self.assertEqual(network.w1.max(), 32767)
            self.assertEqual(network.w1.min(), -32768)

    def test_invalid_configuration_shapes_and_nonfinite_sum(self):
        for mapping in (None, (0,) * 64):
            with self.assertRaisesRegex(ValueError, "at least two"):
                NGNN(16, 8, mapping, True)
        model = NGNN(16, 8, DEFAULT_MAP, True)
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "bad.nnue"
            for replacement in (torch.zeros(767, 16), torch.zeros(768, 32),
                                torch.zeros(768, 16, dtype=torch.float64),
                                torch.full((768, 16), float("nan")),
                                torch.full((768, 16), float("inf"))):
                state = model.state_dict().copy()
                state["w1_shared"] = replacement
                with self.assertRaises(ValueError):
                    export_state(state, path)
            for version in (1, 2):
                with self.assertRaises(ValueError):
                    export_state(model.state_dict(), path, version)
            state = NGNN(16, 8).state_dict()
            state["w1_shared"] = torch.zeros(768, 16)
            with self.assertRaises(ValueError):
                export_state(state, path)

    def test_cli_resume_exactness_reexport_and_changed_flag(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            shard = root / "data.ngnp"
            np.asarray([encode_record(pos, i * 5) for i, pos in enumerate(positions())],
                       dtype=RECORD_DTYPE).tofile(shard)
            common = [sys.executable, "-m", "trainer.train", "--data", str(shard), "--hidden", "16",
                      "--buckets", "8", "--format", "3", "--batch", "32", "--decay", "none",
                      "--device", "cpu", "--precision", "fp32", "--threads", "2"]
            env = dict(os.environ, OMP_NUM_THREADS="2", OPENBLAS_NUM_THREADS="2", MKL_NUM_THREADS="2")

            def run(extra, succeeds=True):
                proc = subprocess.run(common + extra, env=env, capture_output=True, text=True, timeout=90)
                self.assertEqual(proc.returncode == 0, succeeds, proc.stdout + proc.stderr)
                return proc

            run(["--out", str(root / "full"), "--epochs", "2", "--king-factorizer"])
            run(["--out", str(root / "resumed"), "--epochs", "1", "--king-factorizer"])
            checkpoint = root / "resumed/epoch-0001.pt"
            resume_args = ["--out", str(root / "resumed"), "--epochs", "2", "--resume", str(checkpoint)]
            rejected = run(resume_args, False)
            self.assertIn("king_factorizer", rejected.stderr)
            run(resume_args + ["--king-factorizer"])
            self.assertEqual((root / "full/epoch-0002.nnue").read_bytes(),
                             (root / "resumed/epoch-0002.nnue").read_bytes())
            full = torch.load(root / "full/epoch-0002.pt", weights_only=True)
            resumed = torch.load(root / "resumed/epoch-0002.pt", weights_only=True)
            self.assertTrue(full["config"]["king_factorizer"])
            for name in full["model"]:
                torch.testing.assert_close(full["model"][name], resumed["model"][name], rtol=0, atol=0)
            self.assertGreater(full["model"]["w1_shared"].abs().sum().item(), 0)
            proc = subprocess.run([sys.executable, "-m", "trainer.export", str(root / "full/epoch-0002.pt"),
                                   str(root / "reexport.nnue")], env=env, capture_output=True, text=True, timeout=90)
            self.assertEqual(proc.returncode, 0, proc.stderr)
            self.assertEqual((root / "reexport.nnue").read_bytes(), (root / "full/epoch-0002.nnue").read_bytes())
            run(["--out", str(root / "legacy"), "--epochs", "1"])
            legacy = torch.load(root / "legacy/epoch-0001.pt", weights_only=True)
            del legacy["config"]["king_factorizer"]
            torch.save(legacy, root / "old.pt")
            run(["--out", str(root / "legacy"), "--epochs", "2", "--resume", str(root / "old.pt")])
            rejected = run(["--out", str(root / "wrong"), "--epochs", "2", "--resume", str(root / "old.pt"),
                            "--king-factorizer"], False)
            self.assertIn("king_factorizer", rejected.stderr)
            for version in (1, 2):
                rejected = run(["--out", str(root / "wrong"), "--epochs", "1", "--king-factorizer",
                                "--format", str(version), "--buckets", "1"], False)
                self.assertIn("at least two", rejected.stderr)


if __name__ == "__main__":
    unittest.main()
