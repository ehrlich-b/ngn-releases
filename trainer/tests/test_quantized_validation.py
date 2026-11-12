"""Tiny exported-network controls; no torch, corpus, training, or claim games."""

from fractions import Fraction
import json
import math
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch

import numpy as np

from trainer.ngnp import RECORD_DTYPE, decode_record, to_fen
from trainer.quantized_validation import evaluate_stream, prediction_stream, record_stream
from trainer.refeval import Network


FIXTURE = Path(__file__).resolve().parents[2] / "experiments/wave2-1104/integer-validation"


class QuantizedValidationTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.network = Network(FIXTURE / "h32.nnue")
        cls.cases = json.loads((FIXTURE / "cases.json").read_text())["cases"]

    def evaluate(self, **kwargs):
        records = record_stream(FIXTURE / "records.ngnp")
        predictions = prediction_stream(FIXTURE / "float-cp.jsonl")
        try:
            return evaluate_stream(self.network, records, max_records=kwargs.pop("max_records", 32),
                                   float_predictions=kwargs.pop("float_predictions", predictions), **kwargs)
        finally:
            records.close()
            predictions.close()

    def test_hand_calculated_export_scores_and_coverage(self):
        self.assertEqual(self.network.hidden, 32)
        self.assertEqual((self.network.qa, self.network.qb, self.network.scale), (255, 64, 400))
        self.assertEqual(len(self.cases), 32)
        self.assertEqual({c["bucket"] for c in self.cases}, set(range(8)))
        self.assertEqual({c["stm"] for c in self.cases}, {0, 1})
        self.assertTrue(any(c["cp_before_clamp"] > 24999 for c in self.cases))
        self.assertTrue(any(c["cp_before_clamp"] < -24999 for c in self.cases))
        with (FIXTURE / "records.ngnp").open("rb") as source:
            records = np.frombuffer(source.read(), dtype=RECORD_DTYPE)
        self.assertEqual(len(records), 32)
        for case, record in zip(self.cases, records):
            with self.subTest(bucket=case["bucket"], variant=case["variant"]):
                position = decode_record(record)
                self.assertEqual(to_fen(position), case["fen"])
                # Independent explicit integer arithmetic, not the reference's
                # feature builder, clamp, bucket function or trunc_div helper.
                us, them = case["king_q_white"], case["king_q_black"]
                if case["stm"]:
                    us, them = them, us
                pawn = ((-3 if case["stm"] else 7) * 255 ** 2) if case["pieces"] > 2 else 0
                dot = -255 ** 2 - 64 * us ** 2 + 32 * them ** 2 + pawn
                first = int(Fraction(dot, 255))
                cp = int(Fraction((first + case["output_bias"]) * 400, 255 * 64))
                self.assertEqual((dot, first, cp),
                                 (case["dot_total"], case["first_truncated"], case["cp_before_clamp"]))
                self.assertEqual(max(-24999, min(24999, cp)), case["integer_cp"])
                self.assertEqual(self.network.evaluate_record(record), case["integer_cp"])
                self.assertEqual(self.network.bucket(position), case["bucket"])
                if case["float_cp"] is not None:
                    expected_float = float(Fraction(dot * 400, 255 ** 2 * 64)
                                           + Fraction(case["output_bias"] * 400, 255 * 64))
                    self.assertEqual(expected_float, case["float_cp"])
        # The very first dot is negative with a remainder. Its bias places the
        # second division at a boundary that exposes changing only the first.
        self.assertEqual((self.cases[0]["dot_total"], self.cases[0]["integer_cp"]), (-47329, -4))

    def assert_hand_totals(self, actual, cases, lam=0.75, k=400):
        def probability(cp):
            return 1 / (1 + math.exp(-cp / k))

        integer_losses, float_losses, paired_integer, errors = [], [], [], []
        for c in cases:
            score = c["score_white"] * (1 - 2 * c["stm"])
            result = c["result_white"] / 2
            if c["stm"]:
                result = 1 - result
            target = lam * probability(score) + (1 - lam) * result
            loss = (probability(c["integer_cp"]) - target) ** 2
            integer_losses.append(loss)
            if c["float_cp"] is not None:
                paired_integer.append(loss)
                float_losses.append((probability(c["float_cp"]) - target) ** 2)
                errors.append(c["integer_cp"] - c["float_cp"])
        self.assertEqual(actual["count"], len(cases))
        self.assertEqual(actual["float_count"], len(errors))
        self.assertEqual(actual["missing_float_count"], len(cases) - len(errors))
        expected = {
            "integer_loss_sum": sum(integer_losses),
            "integer_loss": sum(integer_losses) / len(cases),
            "paired_integer_loss_sum": sum(paired_integer),
            "paired_integer_loss": sum(paired_integer) / len(errors),
            "float_loss_sum": sum(float_losses),
            "float_loss": sum(float_losses) / len(errors),
            "loss_difference_sum": sum(i - f for i, f in zip(paired_integer, float_losses)),
            "loss_difference": sum(i - f for i, f in zip(paired_integer, float_losses)) / len(errors),
            "cp_error_sum": sum(errors), "cp_error": sum(errors) / len(errors),
            "cp_absolute_error_sum": sum(abs(e) for e in errors),
            "cp_mean_absolute_error": sum(abs(e) for e in errors) / len(errors),
            "cp_squared_error_sum": sum(e * e for e in errors),
            "cp_root_mean_squared_error": math.sqrt(sum(e * e for e in errors) / len(errors)),
            "cp_max_absolute_error": max(abs(e) for e in errors),
        }
        for name, value in expected.items():
            with self.subTest(metric=name, bucket=actual.get("bucket")):
                self.assertAlmostEqual(actual[name], value, delta=max(1e-14, abs(value) * 1e-12))

    def test_overall_and_every_bucket_sums_counts_and_objectives(self):
        for lam, k in [(0.75, 400), (0, 400), (1, 200)]:
            report = self.evaluate(lam=lam, k=k)
            self.assert_hand_totals(report["overall"], self.cases, lam, k)
            for bucket in report["buckets"]:
                self.assert_hand_totals(bucket, [c for c in self.cases if c["bucket"] == bucket["bucket"]], lam, k)
            self.assertEqual(report["overall"]["missing_float_count"], 8)

    def test_missing_predictions_are_unknown(self):
        for predictions in (None, [], [None] * 32):
            report = self.evaluate(float_predictions=predictions)
            for totals in [report["overall"], *report["buckets"]]:
                self.assertEqual(totals["float_count"], 0)
                self.assertEqual(totals["missing_float_count"], totals["count"])
                for name in ("cp_error_sum", "cp_error", "cp_mean_absolute_error", "float_loss_sum",
                             "float_loss", "paired_integer_loss", "loss_difference_sum", "loss_difference"):
                    self.assertIsNone(totals[name])
                self.assertIsNotNone(totals["integer_loss"])
        short = self.evaluate(float_predictions=[self.cases[0]["float_cp"]])
        self.assertEqual(short["overall"]["float_count"], 1)
        self.assertEqual(short["overall"]["missing_float_count"], 31)
        self.assertIsNone(short["buckets"][1]["float_loss"])

    def test_cap_never_consumes_lookahead_and_empty_buckets_are_unknown(self):
        with (FIXTURE / "records.ngnp").open("rb") as source:
            first = np.frombuffer(source.read(32), dtype=RECORD_DTYPE)[0]

        def records():
            yield first
            raise AssertionError("Read beyond max_records")

        def predictions():
            yield 0
            raise AssertionError("Read float prediction beyond max_records")

        report = evaluate_stream(self.network, records(), max_records=1, float_predictions=predictions())
        self.assertEqual(report["overall"]["count"], 1)
        self.assertEqual(report["buckets"][0]["count"], 1)
        for bucket in report["buckets"][1:]:
            self.assertEqual(bucket["count"], 0)
            self.assertIsNone(bucket["integer_loss"])
            self.assertIsNone(bucket["loss_difference"])
        empty = evaluate_stream(self.network, iter(()), max_records=1)
        self.assertIsNone(empty["overall"]["integer_loss"])

    def test_division_mutations_change_the_fixture_result(self):
        # Mutate each division separately as well as both: truncation towards
        # zero is required at both stages, not floor or nearest rounding.
        import trainer.refeval as reference
        original = reference.trunc_div
        baseline = self.evaluate()
        for denominator in (255, 255 * 64, None):
            def floor_div(n, d):
                return n // d if denominator is None or d == denominator else original(n, d)
            with patch.object(reference, "trunc_div", floor_div):
                mutated = self.evaluate()
            self.assertNotEqual(mutated, baseline)
        with patch.object(reference, "trunc_div", lambda n, d: round(n / d)):
            self.assertNotEqual(self.evaluate(), baseline)

    def test_invalid_options_predictions_and_records(self):
        for limit in (0, -1, 1.5, True):
            with self.assertRaises(ValueError):
                self.evaluate(max_records=limit)
        for options in ({"lam": -0.1}, {"lam": float("nan")}, {"k": 0}, {"k": float("inf")}):
            with self.assertRaises(ValueError):
                self.evaluate(**options)
        for value in (float("nan"), float("inf")):
            with self.assertRaises(ValueError):
                self.evaluate(float_predictions=[value])
        with patch.object(self.network, "version", 2):
            with self.assertRaisesRegex(ValueError, "NGNN3"):
                self.evaluate()
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "truncated.ngnp"
            path.write_bytes(b"x")
            with self.assertRaisesRegex(ValueError, "Truncated"):
                list(record_stream(path))
            path.write_text("\n")
            with self.assertRaises(ValueError):
                list(prediction_stream(path))
            path.write_text("true\n")
            with self.assertRaises(ValueError):
                list(prediction_stream(path))
        record = next(record_stream(FIXTURE / "records.ngnp")).copy()
        record["stm"] = 2
        with self.assertRaises(ValueError):
            evaluate_stream(self.network, [record], max_records=1)
        record["stm"] = 0
        record["pieces"][0] = 0  # Remove White king from the two-king record.
        with self.assertRaisesRegex(ValueError, "king"):
            evaluate_stream(self.network, [record], max_records=1)

    def test_fixture_reproducible_and_cli_needs_no_torch(self):
        with tempfile.TemporaryDirectory() as directory:
            subprocess.run([sys.executable, "-m", "experiments.wave2-1104.integer-validation.build_fixture",
                            "--out", directory],
                           check=True)
            for name in ("h32.nnue", "records.ngnp", "float-cp.jsonl", "cases.json"):
                self.assertEqual((Path(directory) / name).read_bytes(), (FIXTURE / name).read_bytes())
            # A hard import blocker proves the executable diagnostic has no
            # transitive torch dependency even where torch happens to exist.
            code = """import sys, runpy
class BlockTorch:
    def find_spec(self, fullname, *args):
        if fullname.split('.')[0] == 'torch':
            raise AssertionError('torch imported by diagnostic')
sys.meta_path.insert(0, BlockTorch())
runpy.run_module('trainer.quantized_validation', run_name='__main__')
"""
            command = [sys.executable, "-c", code,
                       "--network", str(FIXTURE / "h32.nnue"),
                       "--records", str(FIXTURE / "records.ngnp"),
                       "--float-predictions", str(FIXTURE / "float-cp.jsonl"), "--max-records", "32"]
            result = subprocess.run(command, check=True, text=True, capture_output=True)
            actual = json.loads(result.stdout)
            self.assertEqual(actual, self.evaluate())
            self.assertEqual(actual, json.loads((FIXTURE / "report.json").read_text()))
            # The CLI also respects a short cap and omitted float predictions.
            short = command[:-4] + ["--max-records", "1"]
            actual = json.loads(subprocess.run(short, check=True, text=True, capture_output=True).stdout)
            self.assertEqual(actual["overall"]["count"], 1)
            self.assertIsNone(actual["overall"]["float_loss"])


if __name__ == "__main__":
    unittest.main()
