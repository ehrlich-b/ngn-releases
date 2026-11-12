"""Exact warm start, staged gradients and integer export parity for NGNN6."""
import hashlib
import json
import struct
import zlib
import unittest

import numpy as np
import torch

from trainer.export import export_bytes
from trainer.model import NGNN
from trainer.residual_export import lift_bytes
from trainer.residual_model import ResidualNGNN, _quantized, _ste
from trainer.residual_refeval import Network
from trainer.ngnp import RECORD_DTYPE, parse_fen
from trainer.tests.residual_fixture import ROOT, PACKET, SOURCE, inputs, train_fixture


class ResidualModelTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        torch.set_num_threads(2)
        cls.trained, cls.result = train_fixture()

    def test_initial_h32_and_actual_h768(self):
        for path in (SOURCE, ROOT / "engine/nets/ngn-h768-2dcb6d93.nnue"):
            source = path.read_bytes()
            zero = lift_bytes(source)
            model = ResidualNGNN.from_zero_lift(zero)
            self.assertEqual(export_bytes(model.state_dict(), 6), zero)
            model.clip_quantized()
            self.assertEqual(export_bytes(model.state_dict(), 6), zero)
            self.assertEqual(zero[120:120+len(source)-100], source[96:-4])
            features, stm, _, _, bucket = inputs(model)
            oracle = Network(source)
            records = np.frombuffer((PACKET / 'records.ngnp').read_bytes(), dtype=RECORD_DTYPE)
            expected = [oracle.evaluate_record(rec) for rec in records]
            with torch.no_grad():
                self.assertEqual(model(features, stm, bucket).tolist(), expected)
                # Same float trunk and linear values at initialization.
                base = NGNN(model.hidden, 8, model.king_map.tolist())
                base.ob = torch.nn.Parameter(base.ob.double())
                base.load_state_dict({k: v for k, v in model.state_dict().items()
                                      if k in base.state_dict()})
                for name in ('w1', 'b1', 'o', 'ob'):
                    self.assertTrue(torch.equal(getattr(model, name), getattr(base, name)))
                # The original unquantized model can differ by cp rounding;
                # integer equality above has no tolerance.
                self.assertLess(float((base(features, stm, bucket) - model(features, stm, bucket)).abs().max()), 3.0)
            print(f'warm start H{model.hidden}: payload={len(source)-100}, bytes={len(zero)}, exact 16 scores', flush=True)

    def test_staged_gradients_and_optimizer_coverage(self):
        first, second = self.result['gradients']
        self.assertGreater(first['r'], 0)
        self.assertEqual(first['a'], 0)
        self.assertEqual(first['hb'], 0)
        self.assertGreater(second['a'], 0)
        self.assertGreater(second['hb'], 0)
        self.assertTrue(torch.all(self.trained.r.grad.abs().sum(dim=1) > 0))
        groups = self.trained.optimizer_groups(0.1)
        grouped = [id(p) for group in groups for p in group['params']]
        self.assertEqual(len(grouped), len(set(grouped)))
        self.assertEqual(set(grouped), {id(p) for p in self.trained.parameters()})
        self.assertEqual(groups[1]['weight_decay'], 0)
        print('two-step gradients:', self.result['gradients'], flush=True)

    def test_trained_packet_and_independent_integer_parity(self):
        raw = export_bytes(self.trained.state_dict(), 6)
        self.assertEqual(raw, (PACKET/'trained-h32.ngn6').read_bytes())
        state = np.load(PACKET/'weights.npz', allow_pickle=False)
        for name, tensor in self.trained.state_dict().items():
            np.testing.assert_array_equal(tensor.detach().numpy(), state[name])
        oracle = Network(raw)
        packet = json.loads((PACKET/'expected.json').read_text())
        self.assertEqual(packet['sha256'], hashlib.sha256(raw).hexdigest())
        self.assertEqual(packet['training'], self.result)
        features, stm, _, _, bucket = inputs(self.trained)
        with torch.no_grad():
            scores = self.trained(features, stm, bucket).tolist()
        coverage = set()
        for i, case in enumerate(packet['cases']):
            self.assertEqual(scores[i], case['cp'])
            self.assertEqual(oracle.evaluate_fen(case['fen']), case['cp'])
            self.assertEqual(sum(oracle.components(parse_fen(case['fen']))), case['cp'])
            self.assertNotEqual(case['residual_cp'], 0)
            coverage.add((case['bucket'], case['stm']))
        self.assertEqual(coverage, {(b, s) for b in range(8) for s in (0, 1)})

    def test_signed_arithmetic_and_final_clamp(self):
        model = ResidualNGNN.from_ngnn3(SOURCE.read_bytes())
        features, stm, _, _, bucket = inputs(model)
        with torch.no_grad():
            model.w1.zero_()
            model.b1.zero_()
            model.o.zero_()
            model.ob.fill_(1224000 / (255*64))  # +30000 cp, before clamp
            model.rb.fill_(-408000 / (255*64))  # -10000 cp
            self.assertEqual(model(features, stm, bucket).tolist(), [20000] * 16)
            model.ob.fill_(-1 / (255*64))
            model.rb.zero_()
            self.assertEqual(model(features, stm, bucket).tolist(), [0] * 16)
            model.ob.zero_()
            model.a.zero_()
            model.hb.fill_(65 / (255*64))  # trunc(65/64) = 1
            model.r.fill_(-65 / 64)
            # Sixteen negative contributions, truncation toward zero in cp.
            self.assertEqual(model(features, stm, bucket).tolist(), [-25] * 16)
            oracle = Network(export_bytes(model.state_dict(), 6))
            for rec in np.frombuffer((PACKET/'records.ngnp').read_bytes(), dtype=RECORD_DTYPE):
                self.assertEqual(oracle.evaluate_record(rec), -25)
            model.hb.fill_(-65 / (255*64))
            self.assertEqual(model(features, stm, bucket).tolist(), [0] * 16)

    def test_full_int32_inherited_bias_round_trip(self):
        source = bytearray(SOURCE.read_bytes())
        struct.pack_into('<8i', source, len(source)-36,
                         -2147483648, 2147483647, 16777217, -16777217, 1, -1, 0, 123456789)
        struct.pack_into('<I', source, len(source)-4, zlib.crc32(source[:-4]))
        zero = lift_bytes(bytes(source))
        model = ResidualNGNN.from_zero_lift(zero)
        model.clip_quantized()
        self.assertEqual(export_bytes(model.state_dict(), 6), zero)

    def test_unused_king_buckets_preserve_fixed_allocation(self):
        for path in (SOURCE, ROOT / "engine/nets/ngn-h768-2dcb6d93.nnue"):
            for highest in (0, 3, 6):
                with self.subTest(source=path.name, highest=highest):
                    source = bytearray(path.read_bytes())
                    source[32:96] = bytes(i % (highest + 1) for i in range(64))
                    struct.pack_into('<I', source, len(source)-4, zlib.crc32(source[:-4]))
                    source = bytes(source)
                    zero = lift_bytes(source)
                    model = ResidualNGNN.from_ngnn3(source)
                    self.assertEqual(model.king_buckets, 8)
                    self.assertEqual(tuple(model.w1.shape), (6144, model.hidden))
                    self.assertEqual(model.king_map.tolist(), list(source[32:96]))
                    model.clip_quantized()
                    self.assertEqual(export_bytes(model.state_dict(), 6), zero)
                    features, stm, _, _, bucket = inputs(model)
                    oracle = Network(source)
                    records = np.frombuffer((PACKET/'records.ngnp').read_bytes(), dtype=RECORD_DTYPE)
                    with torch.no_grad():
                        self.assertEqual(model(features, stm, bucket).tolist(),
                                         [oracle.evaluate_record(rec) for rec in records])

    def test_invalid_king_maps(self):
        for mapping in ([], [0] * 63, [0] * 65, [-1] + [0] * 63,
                        [8] + [0] * 63, [0.5] * 64):
            with self.subTest(mapping=mapping), self.assertRaises(ValueError):
                ResidualNGNN(32, mapping)

    def test_rounding_truncation_and_validation(self):
        values = torch.tensor([-2.5, -1.5, -0.5, 0.5, 1.5, 2.5], requires_grad=True)
        self.assertEqual(_quantized(values, 1).tolist(), [-2, -2, 0, 0, 2, 2])
        self.assertEqual(_ste(values, torch.trunc).tolist(), [-2, -1, 0, 0, 1, 2])
        _quantized(values, 1).sum().backward()
        self.assertTrue(torch.equal(values.grad, torch.ones_like(values)))
        state = self.trained.state_dict()
        for name, value in [('residual_scales', torch.zeros(8)), ('a', torch.zeros(15, 64)),
                            ('r', torch.full((8, 16), float('nan'))),
                            ('hb', torch.full((16,), 2**31 / (255*64))),
                            ('a', torch.full((16, 64), 32768/64)),
                            ('rb', torch.full((8,), (2**31-1)/(255*64)))]:
            with self.subTest(name=name), self.assertRaises(ValueError):
                export_bytes(dict(state, **{name: value}), 6)
        with self.assertRaises(ValueError):
            export_bytes(dict(state, a=torch.full((16, 64), 8/64),
                              hb=torch.full((16,), (2**31-32768)/(255*64), dtype=torch.float64)), 6)
        bad = bytearray(lift_bytes(SOURCE.read_bytes()))
        bad[100] = 63
        with self.assertRaises(ValueError):
            ResidualNGNN.from_zero_lift(bytes(bad))
        with self.assertRaises(ValueError):
            self.trained(*inputs(self.trained)[:2])
        model = ResidualNGNN.from_ngnn3(SOURCE.read_bytes())
        with torch.no_grad():
            model.hb.fill_(float(2**31))
            model.rb.fill_(-float(2**31))
        model.clip_quantized()
        export_bytes(model.state_dict(), 6)
        with self.assertRaises(ValueError):
            model.optimizer_groups(float('nan'))


if __name__ == '__main__':
    unittest.main()
