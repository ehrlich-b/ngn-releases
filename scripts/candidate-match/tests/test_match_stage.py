from __future__ import annotations

import hashlib
import json
import os
import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

HERE = Path(__file__).resolve().parent
STAGE = HERE.parent / "run_match_stage.py"
PARENT = HERE / "fixture_match_parent.py"


def sha256(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


class MatchStageTests(unittest.TestCase):
    def run_case(self, variant: str) -> tuple[subprocess.CompletedProcess[str], dict]:
        if not {12, 14}.issubset(os.sched_getaffinity(0)):
            self.skipTest("fixture CPUs 12 and 14 unavailable")
        with tempfile.TemporaryDirectory(prefix="candidate-stage-") as raw:
            root = Path(raw)
            role_a = root / "role a"
            role_b = root / "role b"
            role_a.mkdir()
            role_b.mkdir()
            stdout = root / "match.stdout"
            stderr = root / "match.stderr"
            witness = root / "witness.json"
            sleep = Path("/usr/bin/sleep").resolve(strict=True)
            engine_a = root / "engine-a"
            engine_b = root / "engine-b"
            shutil.copy2(sleep, engine_a)
            shutil.copy2(sleep, engine_b)
            engine_a.chmod(0o555)
            engine_b.chmod(0o555)
            launcher_a = role_a / "launch"
            launcher_b = role_b / "launch"
            shutil.copy2(PARENT, launcher_a)
            shutil.copy2(PARENT, launcher_b)
            launcher_a.chmod(0o555)
            launcher_b.chmod(0o555)
            command = ["/usr/bin/python3", str(PARENT), variant, str(engine_a), str(role_a), "12", str(engine_b), str(role_b), "14"]
            config = {
                "schema": "ngn-candidate-match-stage-v1", "command": command,
                "cwd": str(root), "environment": {"PATH": "/usr/bin:/bin", "LANG": "C", "LC_ALL": "C", "TZ": "UTC"},
                "roles": [
                    {"id": "a", "engine": str(engine_a), "engine_sha256": sha256(engine_a), "cwd": str(role_a), "gomaxprocs": "1", "allowed_cpu_masks": ["12", "14"], "launcher": str(launcher_a)},
                    {"id": "b", "engine": str(engine_b), "engine_sha256": sha256(engine_b), "cwd": str(role_b), "gomaxprocs": "1", "allowed_cpu_masks": ["12", "14"], "launcher": str(launcher_b)},
                ],
                "sample_interval_seconds": 0.01, "match_stdout": str(stdout),
                "match_stderr": str(stderr), "witness": str(witness),
            }
            config_path = root / "config.json"
            config_path.write_text(json.dumps(config), encoding="utf-8")
            completed = subprocess.run(
                [sys.executable, str(STAGE), "--config", str(config_path)],
                text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=10,
            )
            return completed, json.loads(witness.read_text(encoding="utf-8"))

    def test_records_exact_child_identity_affinity_and_environment(self) -> None:
        completed, witness = self.run_case("good")
        self.assertEqual(completed.returncode, 0, completed.stderr)
        self.assertEqual(witness["state"], "COMPLETE")
        self.assertFalse(witness["violations"])
        self.assertTrue(witness["observed_instances"]["a"])
        self.assertTrue(witness["observed_instances"]["b"])
        masks = {item["cpus_allowed_list"] for item in witness["observations"]}
        self.assertEqual(masks, {"12", "14"})
        self.assertEqual({item["gomaxprocs"] for item in witness["observations"]}, {"1"})

    def test_exact_engine_in_wrong_cwd_is_rejected_even_with_correct_instances(self) -> None:
        completed, witness = self.run_case("wrong-cwd-extra")
        self.assertNotEqual(completed.returncode, 0)
        self.assertEqual(witness["state"], "FAILED")
        self.assertTrue(witness["observed_instances"]["a"])
        self.assertTrue(witness["observed_instances"]["b"])
        self.assertTrue(any("cwd " in item and " != " in item for item in witness["violations"]))

    def test_wrong_child_gomaxprocs_is_rejected(self) -> None:
        completed, witness = self.run_case("bad-env")
        self.assertNotEqual(completed.returncode, 0)
        self.assertEqual(witness["state"], "FAILED")
        self.assertTrue(any("GOMAXPROCS 2 != 1" in item for item in witness["violations"]))


if __name__ == "__main__":
    unittest.main()
