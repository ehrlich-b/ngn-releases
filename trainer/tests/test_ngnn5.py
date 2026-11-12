"""Small CPU proofs of tensor threats, migration, training and quantization."""
import tempfile
import unittest
from pathlib import Path

import numpy as np
import torch

from trainer.export import export_state
from trainer.king_buckets import DEFAULT_MAP
from trainer.model import NGNN, decode_gpu, warm_start_ngnn3
from trainer.ngnp import RECORD_DTYPE, Position, encode_record, parse_fen
from trainer.threats import reference_indices


class NGNN5Tests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        torch.set_num_threads(2)

    def batch(self, positions):
        return torch.from_numpy(np.asarray([encode_record(p, 150) for p in positions],
                                          dtype=RECORD_DTYPE).view(np.uint8).reshape(-1, 32))

    def test_tensor_rows_all_king_squares_and_symmetries(self):
        from trainer.tests.test_ngnn3 import positions
        samples = positions()
        # Include sliders, first blockers, friendly targets and both pawn directions.
        samples += [parse_fen(f) for f in (
            'r3k2r/pp2qppp/2n1bn2/3pP3/3P4/2N1BN2/PPQ2PPP/R3K2R w - - 0 1',
            '4k3/8/8/8/8/8/4R3/4K3 b - - 0 1')]
        for flip in (0, 7, 56, 63):
            swapped = bool(flip & 56)
            panel = [Position(p.squares ^ flip, (p.pieces + 6) % 12 if swapped else p.pieces,
                              p.stm ^ swapped) for p in samples]
            model = NGNN(16, 8, DEFAULT_MAP, threats=True)
            features, *_ = decode_gpu(self.batch(panel), 8, model.king_map, 8, True)
            indices, offsets = features[1]
            for p in (0, 1):
                for i, pos in enumerate(panel):
                    bag = p * len(panel) + i
                    actual = sorted(indices[offsets[bag]:offsets[bag+1]].tolist())
                    self.assertEqual(actual, reference_indices(pos, p, DEFAULT_MAP))

    def test_checkpoint_warm_start_equality_and_guards(self):
        from trainer.tests.test_ngnn3 import positions
        raw = self.batch(positions())
        for factorizer in (False, True):
            base = NGNN(16, 8, DEFAULT_MAP, factorizer)
            target = NGNN(16, 8, DEFAULT_MAP, factorizer, threats=True)
            checkpoint = {'config': {'format': 3, 'hidden': 16, 'buckets': 8}, 'model': base.state_dict()}
            with tempfile.TemporaryDirectory() as directory:
                path = Path(directory) / 'base.pt'
                torch.save(checkpoint, path)
                warm_start_ngnn3(target, torch.load(path, weights_only=True))
            f3, stm, _, _, bucket = decode_gpu(raw, 8, base.king_map, 8)
            f5, *_ = decode_gpu(raw, 8, target.king_map, 8, True)
            self.assertLessEqual((base(f3, stm, bucket) - target(f5, stm, bucket)).abs().max().item(), 1e-6)
            self.assertEqual(torch.count_nonzero(target.threat_weights).item(), 0)
            checkpoint['config']['format'] = 5
            with self.assertRaisesRegex(ValueError, 'NGNN3'):
                warm_start_ngnn3(target, checkpoint)
        bad = NGNN(16, 8, [0]*64, threats=True)
        checkpoint['config']['format'] = 3
        with self.assertRaisesRegex(ValueError, 'architecture'):
            warm_start_ngnn3(bad, checkpoint)

    def test_threat_gradient_clipping_and_export_guard(self):
        model = NGNN(16, 1, [0]*64, threats=True)
        raw = self.batch([parse_fen('4k3/8/8/8/8/8/4R3/4K3 w - - 0 1')])
        features, stm, _, _, bucket = decode_gpu(raw, 1, model.king_map, 1, True)
        model(features, stm, bucket).sum().backward()
        self.assertGreater(model.threat_weights.grad.abs().sum().item(), 0)
        with torch.no_grad():
            model.threat_weights.fill_(1)
        model.clip_quantized()
        self.assertLessEqual(model.threat_weights.max().item(), 127/255 + 1e-7)
        with tempfile.TemporaryDirectory() as directory:
            with self.assertRaisesRegex(ValueError, 'require NGNN5'):
                export_state(model.state_dict(), Path(directory)/'wrong.nnue', 3)

    def test_cpu_training_fixture_reproducible_and_resume(self):
        import json
        import subprocess
        import sys
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            proc = subprocess.run([sys.executable, '-m', 'trainer.make_ngnn5_fixture', '--out', str(root)],
                                  capture_output=True, text=True, timeout=60)
            self.assertEqual(proc.returncode, 0, proc.stdout + proc.stderr)
            self.assertEqual((root/'trained/epoch-0003.nnue').read_bytes(),
                             Path('testdata/ngnn5/trained_h16_k8_b8.nnue').read_bytes())
            self.assertEqual(json.loads((root/'evals.json').read_text()),
                             json.loads(Path('testdata/ngnn5/trained_h16_k8_b8_evals.json').read_text()))
            checkpoint = torch.load(root/'trained/epoch-0003.pt', weights_only=True)
            self.assertEqual(checkpoint['config']['format'], 5)
            self.assertGreater(checkpoint['history'][-1]['positions_per_second'], 0)
            seconds = sum(row['seconds'] for row in checkpoint['history'])
            print(json.dumps({'device': 'cpu', 'python': sys.executable, 'torch': torch.__version__,
                              'records': 60, 'seconds': seconds, 'records_per_second': 60/seconds}))
            config = checkpoint['config']
            command = [sys.executable, '-m', 'trainer.train', '--data', str(root/'records.ngnp'),
                       '--out', str(root/'resumed'), '--format', '5', '--hidden', '16', '--buckets', '8',
                       '--resume', str(root/'trained/epoch-0003.pt'), '--device', 'cpu', '--precision', 'fp32',
                       '--threads', '2', '--batch', '10', '--epochs', '4', '--decay', 'none', '--lr', '0.02',
                       '--seed', str(config['seed']), '--validation-data', str(root/'records.ngnp')]
            # Existing overlap protection applies to NGNN5 too.
            proc = subprocess.run(command, capture_output=True, text=True, timeout=60)
            self.assertNotEqual(proc.returncode, 0)
            self.assertIn('overlap', proc.stderr)
            proc = subprocess.run(command[:-2], capture_output=True, text=True, timeout=60)
            self.assertEqual(proc.returncode, 0, proc.stdout + proc.stderr)
            resumed = torch.load(root/'resumed/epoch-0004.pt', weights_only=True)
            self.assertEqual(resumed['step'], 8)
            self.assertEqual(len(resumed['history']), 4)
