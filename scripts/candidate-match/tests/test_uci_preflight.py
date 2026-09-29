from __future__ import annotations

import json
import shlex
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

HERE = Path(__file__).resolve().parent
ROOT = HERE.parent
PREFLIGHT = ROOT / "uci_preflight.py"
FIXTURE = HERE / "fixture_uci_engine.py"


def options(backend: str = "hce") -> list[dict]:
    return [
        {"name": "Threads", "value": 1},
        {"name": "OwnBook", "value": False},
        {"name": "EvalFile", "value": "<empty>" if backend == "hce" else "/tmp/frozen.ngn"},
        {"name": "EvalBackend", "value": backend},
        {"name": "Hash", "value": 64},
        {"name": "Move Overhead", "value": 50},
    ]


class PreflightTests(unittest.TestCase):
    def run_case(self, variant: str, backend: str = "hce") -> tuple[subprocess.CompletedProcess[str], dict]:
        with tempfile.TemporaryDirectory(prefix="candidate-preflight-") as raw:
            root = Path(raw)
            cwd = root / "engine cwd"
            cwd.mkdir()
            fixture = root / "fixture_uci_engine.py"
            fixture_source = FIXTURE.read_text(encoding="utf-8")
            old_combo = 'print("option name EvalBackend type combo default hce var hce var ngn-v1")'
            new_combo = 'print("option name EvalBackend type combo default hce var hce var ngn-v1 var ngn-k4-768-v1 var counter-5.5 var rodent-v1.1-anand var rodent-v1.2-default")'
            old_diagnostic = 'print(f"info string eval backend {backend} score_cp 0 pov side-to-move policy base-SearchSTM rule50-and-backend-adapter correction-history excluded", flush=True)'
            new_diagnostic = '\n'.join([
                'reported = "counter-5.5" if variant == "diagnostic-mismatch" else backend',
                '        print(f"info string eval backend {reported} score_cp 0 pov side-to-move policy base-SearchSTM rule50-and-backend-adapter correction-history excluded", flush=True)',
            ])
            if old_combo not in fixture_source or old_diagnostic not in fixture_source:
                self.fail("fixture source no longer matches the explicit extended-backend test overlay")
            fixture.write_text(fixture_source.replace(old_combo, new_combo).replace(old_diagnostic, new_diagnostic), encoding="utf-8")
            launcher = root / "fixture launcher"
            launcher.write_text(
                "#!/bin/sh\nexec /usr/bin/python3 "
                + shlex.quote(str(fixture))
                + " "
                + shlex.quote(variant)
                + "\n",
                encoding="utf-8",
            )
            launcher.chmod(0o555)
            config = {
                "schema": "ngn-candidate-uci-preflight-v1",
                "role_id": backend.replace("-", "_").replace(".", "_"),
                "launcher": str(launcher),
                "cwd": str(cwd),
                "backend": backend,
                "capability_profile": "ngn-pre-m4c-pinned-one-worker-v1",
                "options": options(backend),
                "barrier_timeout_seconds": 2,
            }
            config_path = root / "config.json"
            config_path.write_text(json.dumps(config), encoding="utf-8")
            output = root / "output"
            completed = subprocess.run(
                [sys.executable, str(PREFLIGHT), "--config", str(config_path), "--output", str(output)],
                text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=10,
            )
            receipt = json.loads((output / "receipt.json").read_text(encoding="utf-8"))
            transcript = [json.loads(line) for line in (output / "transcript.jsonl").read_text(encoding="utf-8").splitlines()]
            receipt["_test_transcript"] = transcript
            return completed, receipt

    def test_good_proves_order_backend_and_persistent_no_book_search(self) -> None:
        completed, receipt = self.run_case("good")
        self.assertEqual(completed.returncode, 0, completed.stderr)
        self.assertEqual(receipt["state"], "COMPLETE")
        self.assertEqual(receipt["backend"], "hce")
        self.assertEqual(receipt["advertised_options"]["Hash"]["max"], 1024)
        self.assertEqual(
            receipt["advertised_options"]["EvalBackend"]["vars"],
            ["hce", "ngn-v1", "ngn-k4-768-v1", "counter-5.5", "rodent-v1.1-anand", "rodent-v1.2-default"],
        )
        self.assertTrue(receipt["_test_transcript"])
        self.assertEqual(receipt["ownbook_false_search"]["nodes"], 81)
        self.assertEqual(
            [item["option"] for item in receipt["ordered_option_barriers"]],
            ["Threads", "OwnBook", "EvalFile", "EvalBackend", "Hash", "Move Overhead"],
        )

    def test_extended_backends_are_proved_not_merely_advertised(self) -> None:
        for backend in ("ngn-k4-768-v1", "counter-5.5", "rodent-v1.1-anand", "rodent-v1.2-default"):
            with self.subTest(backend=backend):
                completed, receipt = self.run_case("good", backend)
                self.assertEqual(completed.returncode, 0, completed.stderr)
                self.assertEqual(receipt["backend"], backend)
                self.assertIn(f"eval backend {backend} ", receipt["eval_diagnostic"])

    def test_selected_backend_diagnostic_mismatch_is_rejected(self) -> None:
        completed, receipt = self.run_case("diagnostic-mismatch", "rodent-v1.2-default")
        self.assertNotEqual(completed.returncode, 0)
        self.assertIn("selected evaluator diagnostic mismatch", receipt["error"])

    def test_advertised_domain_and_backend_variants_are_enforced(self) -> None:
        for variant, phrase in (("narrow-hash", "outside advertised"), ("missing-backend-var", "absent from advertised vars")):
            with self.subTest(variant=variant):
                completed, receipt = self.run_case(variant)
                self.assertNotEqual(completed.returncode, 0)
                self.assertIn(phrase, receipt["error"])

    def test_error_after_bestmove_is_drained_and_rejected(self) -> None:
        completed, receipt = self.run_case("late-error")
        self.assertNotEqual(completed.returncode, 0)
        self.assertIn("operational error after bestmove", receipt["error"])

    def test_error_plus_legal_fallback_is_rejected(self) -> None:
        completed, receipt = self.run_case("error-fallback")
        self.assertNotEqual(completed.returncode, 0)
        self.assertEqual(receipt["state"], "FAILED")
        self.assertIn("operational error", receipt["error"])

    def test_ignored_or_reset_ownbook_is_rejected(self) -> None:
        for variant in ("ignore-ownbook", "reset-ownbook"):
            with self.subTest(variant=variant):
                completed, receipt = self.run_case(variant)
                self.assertNotEqual(completed.returncode, 0)
                self.assertEqual(receipt["state"], "FAILED")
                self.assertIn("opening-book signature", receipt["error"])


if __name__ == "__main__":
    unittest.main()
