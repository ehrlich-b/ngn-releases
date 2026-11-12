"""Bounded CPU controls for opt-in NGNN3 export-exact forward and STEs."""

import copy
import importlib
import json
import os
from pathlib import Path
import platform
import tempfile
import time
import unittest

import numpy as np
import torch

from trainer.export import export_state, quantize
from trainer.king_buckets import DEFAULT_MAP, feature_indices
from trainer.model import NGNN, QA, QB, SCALE, decode_gpu
from trainer.ngnp import RECORD_DTYPE, encode_record, parse_fen
from trainer.quantization import fake_quantize, trunc_div
from trainer.refeval import Network
from trainer.train import parser

fixture = importlib.import_module('experiments.wave2-1104.qat-model.fixture')


def original_float(model, features, stm, bucket):
    """Pre-QAT B8 expression, independent of forward/forward_float dispatch."""
    acc = features @ model.effective_w1() + model.b1
    white, black = acc.clamp(0, 1).square().chunk(2, dim=0)
    weights = model.o[bucket]
    us = torch.where((stm == 0)[:, None], white, black)
    them = torch.where((stm == 0)[:, None], black, white)
    output = (us * weights[:, :model.hidden] + them * weights[:, model.hidden:]).sum(dim=1)
    return (output.float() + model.ob[bucket]) * SCALE


class QuantizedForwardTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        torch.set_num_threads(2)

    def test_parameter_rounding_bounds_and_straight_through(self):
        for multiplier, dtype, numpy_dtype in ((QA, torch.int16, '<i2'),
                                                (QB, torch.int16, '<i2'),
                                                (QA * QB, torch.int32, '<i4')):
            # Include neighbors where multiplying in float32 spuriously makes
            # an exact half. Export widens first, and must decide that tie.
            half = torch.tensor([0.5, 1.5, -0.5, -1.5, 127.5, 382.5, -127.5, -382.5]) / multiplier
            lower = torch.nextafter(half, torch.full_like(half, -float('inf')))
            upper = torch.nextafter(half, torch.full_like(half, float('inf')))
            ends = (torch.tensor([-32768, 32767]).float() / multiplier if dtype == torch.int16 else
                    torch.tensor([-2147483520, 2147483520], dtype=torch.float64).float() / multiplier)
            value = torch.cat((half, lower, upper, ends)).requires_grad_()
            actual = fake_quantize(value, multiplier, dtype)
            expected = quantize(value, multiplier, numpy_dtype)
            np.testing.assert_array_equal(actual.detach().numpy(), expected)
            actual.sum().backward()
            torch.testing.assert_close(value.grad, torch.full_like(value, multiplier), rtol=0, atol=0)
            for invalid in (float('nan'), float('inf'), -float('inf'),
                            (torch.iinfo(dtype).max + 4096) / multiplier,
                            (torch.iinfo(dtype).min - 4096) / multiplier):
                value = torch.tensor([invalid], dtype=torch.float32)
                with self.assertRaises(ValueError):
                    quantize(value, multiplier, numpy_dtype)
                with self.assertRaises(ValueError):
                    fake_quantize(value, multiplier, dtype)
        # Truly exact half steps round to the even neighbor on both signs.
        for multiplier, values in ((QA, [0.5, 1.5, -0.5, -1.5]),
                                    (QB, [0.5 / QB, 1.5 / QB, -0.5 / QB, -1.5 / QB]),
                                    (QA * QB, [1 / 128, 3 / 128, -1 / 128, -3 / 128])):
            expected = [0, 2, 0, -2] if multiplier == QB else [128, 382, -128, -382]
            self.assertEqual(fake_quantize(torch.tensor(values), multiplier, torch.int16).tolist(), expected)
        with self.assertRaisesRegex(ValueError, 'float32'):
            fake_quantize(torch.ones(1, dtype=torch.float64), QA, torch.int16)

    def test_signed_truncations_and_straight_through(self):
        value = torch.tensor([-511., -256., -255., -254., -1., 0., 1., 254., 255., 256., 511.],
                             dtype=torch.float64, requires_grad=True)
        actual = trunc_div(value, QA)
        self.assertEqual(actual.tolist(), [-2, -1, -1, 0, 0, 0, 0, 0, 1, 1, 2])
        self.assertFalse(torch.equal(actual, torch.floor(value / QA)))
        actual.sum().backward()
        torch.testing.assert_close(value.grad, torch.full_like(value, 1 / QA), rtol=0, atol=0)
        with self.assertRaises(ValueError):
            trunc_div(value, 0)

    def test_one_h32_fixture_forward_backward_export_and_float_off(self):
        data = fixture.load_cases()
        cases = data['cases']
        positions = [parse_fen(case['fen']) for case in cases]
        records = np.asarray([encode_record(pos, 0) for pos in positions], dtype=RECORD_DTYPE)
        model = fixture.make_model(fake_quantize=True)
        raw = torch.from_numpy(records.view(np.uint8).reshape(-1, 32))
        features, stm, _, _, bucket = decode_gpu(raw, 8, model.king_map, 8)
        for perspective in (0, 1):
            for i, position in enumerate(positions):
                expected = torch.zeros(8 * 768)
                expected[feature_indices(position, perspective, DEFAULT_MAP)] = 1
                torch.testing.assert_close(features[perspective * len(cases) + i], expected, rtol=0, atol=0)
        self.assertEqual(bucket.tolist(), [case['bucket'] for case in cases])
        self.assertEqual(stm.tolist(), [case['stm'] for case in cases])
        self.assertEqual(set(bucket.tolist()), set(range(8)))
        self.assertEqual(set(stm.tolist()), {0, 1})
        before = {name: tensor.clone() for name, tensor in model.state_dict().items()}
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / 'fixture.nnue'
            export_state(model.state_dict(), path)
            network = Network(path)
            export_bytes = path.stat().st_size
            for value, integer, multiplier, dtype in (
                    (model.effective_w1(), network.w1, QA, torch.int16),
                    (model.b1, network.b1, QA, torch.int16),
                    (model.o, network.o, QB, torch.int16),
                    (model.ob, network.ob, QA * QB, torch.int32)):
                np.testing.assert_array_equal(fake_quantize(value, multiplier, dtype).detach().numpy(), integer)
            self.assertEqual(set(network.w1[:, 4]), {1})
            self.assertEqual(quantize(model.w1[:, 4], QA, '<i2').max(), 0)
            self.assertEqual(quantize(model.w1_shared[:, 4], QA, '<i2').max(), 0)
            self.assertEqual((network.w1.min(), network.w1.max()), (-32768, 32767))
            self.assertEqual((network.b1.min(), network.b1.max()), (-32768, 32767))
            self.assertEqual((network.o.min(), network.o.max()), (-127, 127))
            reference = [network.evaluate(position) for position in positions]
            self.assertEqual(reference, [case['integer_cp'] for case in cases])
            # A single QAT forward/backward on the entire small fixture. CPU
            # autocast must not reduce the precision of export arithmetic.
            started = time.perf_counter()
            with torch.autocast(device_type='cpu', dtype=torch.bfloat16):
                output = model(features, stm, bucket)
            self.assertEqual(output.dtype, torch.float32)
            self.assertEqual(output.tolist(), reference)
            output.sum().backward()
            seconds = time.perf_counter() - started
            gradients = {}
            for name, parameter in model.named_parameters():
                self.assertIsNotNone(parameter.grad, name)
                self.assertTrue(torch.isfinite(parameter.grad).all().item(), name)
                nonzero = parameter.grad.count_nonzero().item()
                self.assertGreater(nonzero, 0, name)
                gradients[name] = {'finite': True, 'nonzero': nonzero}
            self.assertNotEqual(model.b1.grad[2].item(), 0)
            self.assertNotEqual(model.b1.grad[4].item(), 0)
            self.assertEqual(model.b1.grad[[0, 1, 30, 31]].count_nonzero().item(), 0)
            self.assertNotEqual(model.b1.grad[17].item(), 0)  # Exactly QA.
            self.assertEqual(model.b1.grad[18].item(), 0)  # Just above QA.
            self.assertEqual(model.o.grad[6:].count_nonzero().item(), 0)
            torch.testing.assert_close(model.w1_shared.grad, model.w1.grad.reshape(8, 768, 32).sum(0))
            first_sensitive = sum(case['integer_cp'] != case['floor_first_cp'] for case in cases)
            second_sensitive = sum(case['integer_cp'] != case['floor_second_cp'] for case in cases)
            self.assertGreater(first_sensitive, 0)
            self.assertGreater(second_sensitive, 0)
            for name, tensor in model.state_dict().items():
                torch.testing.assert_close(tensor, before[name], rtol=0, atol=0)

        # Off retains the pre-QAT float32 values AND every gradient bit. Compare
        # to the old expression rather than to a second call through dispatch.
        off = copy.deepcopy(model)
        off.fake_quantize = False
        off.zero_grad(set_to_none=True)
        old = copy.deepcopy(off)
        float_output = off(features, stm, bucket)
        old_output = original_float(old, features, stm, bucket)
        torch.testing.assert_close(float_output, old_output, rtol=0, atol=0)
        torch.testing.assert_close(model.forward_float(features, stm, bucket), old_output, rtol=0, atol=0)
        float_output.sum().backward()
        old_output.sum().backward()
        for (name, parameter), (_, original) in zip(off.named_parameters(), old.named_parameters()):
            torch.testing.assert_close(parameter.grad, original.grad, rtol=0, atol=0, msg=name)
        self.assertFalse(torch.equal(float_output, output))
        report = {
            'synthetic_only': True, 'python': platform.python_version(),
            'torch': torch.__version__, 'numpy': np.__version__, 'tensor_threads': torch.get_num_threads(),
            'hidden': 32, 'king_buckets': 8, 'buckets': 8, 'cases': len(cases),
            'export_bytes': export_bytes, 'integer_cp': reference,
            'floor_first_sensitive_cases': first_sensitive, 'floor_second_sensitive_cases': second_sensitive,
            'gradients': gradients, 'float_off_values_and_gradients_bitwise_equal': True,
            'qat_forward_backward_seconds_diagnostic_only': seconds,
        }
        print(json.dumps(report, sort_keys=True))
        if destination := os.environ.get('NGN_QAT_REPORT'):
            destination = Path(destination).resolve()
            repo = Path(__file__).resolve().parents[2]
            if destination == repo or repo in destination.parents:
                raise ValueError('Write execution evidence outside the checkout')
            destination.write_text(json.dumps(report, indent=2) + '\n')

    def test_opt_in_configuration_and_legacy_defaults(self):
        args = parser().parse_args(['--data', 'unused', '--out', 'unused'])
        self.assertFalse(args.fake_quantize)
        self.assertTrue(parser().parse_args(['--data', 'unused', '--out', 'unused', '--fake-quantize']).fake_quantize)
        self.assertFalse(NGNN(32, 8, DEFAULT_MAP).fake_quantize)
        with self.assertRaisesRegex(ValueError, 'NGNN3'):
            NGNN(32, 8, fake_quantize=True)


if __name__ == '__main__':
    unittest.main()
