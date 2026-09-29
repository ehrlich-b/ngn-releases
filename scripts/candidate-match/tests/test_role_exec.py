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
ROLE_EXEC = HERE.parent / "role_exec.py"


def sha256(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


class RoleExecTests(unittest.TestCase):
    def test_exact_engine_cwd_hash_and_gomaxprocs(self) -> None:
        with tempfile.TemporaryDirectory(prefix="candidate-role-") as raw:
            root = Path(raw)
            cwd = root / "engine cwd"
            cwd.mkdir()
            launcher_dir = root / "launcher"
            launcher_dir.mkdir()
            launcher = launcher_dir / "role_exec.py"
            shutil.copy2(ROLE_EXEC, launcher)
            engine = Path("/usr/bin/env").resolve(strict=True)
            config = {
                "schema": "ngn-candidate-role-exec-v1", "engine": str(engine),
                "engine_sha256": sha256(engine), "gomaxprocs": "1", "expected_cwd": str(cwd),
            }
            (launcher_dir / "role-config.json").write_text(json.dumps(config), encoding="utf-8")
            completed = subprocess.run(
                [sys.executable, str(launcher)], cwd=cwd,
                env={"PATH": "/usr/bin:/bin", "LANG": "C", "GOMAXPROCS": "99"},
                text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=5,
            )
            self.assertEqual(completed.returncode, 0, completed.stderr)
            self.assertIn("GOMAXPROCS=1\n", completed.stdout)

    def test_hash_or_cwd_drift_is_rejected(self) -> None:
        with tempfile.TemporaryDirectory(prefix="candidate-role-") as raw:
            root = Path(raw)
            cwd = root / "engine"
            wrong = root / "wrong"
            cwd.mkdir()
            wrong.mkdir()
            launcher = root / "role_exec.py"
            shutil.copy2(ROLE_EXEC, launcher)
            engine = Path("/usr/bin/env").resolve(strict=True)
            config = {
                "schema": "ngn-candidate-role-exec-v1", "engine": str(engine),
                "engine_sha256": "0" * 64, "gomaxprocs": "1", "expected_cwd": str(cwd),
            }
            (root / "role-config.json").write_text(json.dumps(config), encoding="utf-8")
            completed = subprocess.run(
                [sys.executable, str(launcher)], cwd=cwd,
                text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=5,
            )
            self.assertEqual(completed.returncode, 111)
            self.assertIn("SHA-256 mismatch", completed.stderr)
            config["engine_sha256"] = sha256(engine)
            (root / "role-config.json").write_text(json.dumps(config), encoding="utf-8")
            completed = subprocess.run(
                [sys.executable, str(launcher)], cwd=wrong,
                text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=5,
            )
            self.assertEqual(completed.returncode, 111)
            self.assertIn("cwd mismatch", completed.stderr)


if __name__ == "__main__":
    unittest.main()
