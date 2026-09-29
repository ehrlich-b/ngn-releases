from __future__ import annotations

import json
import os
import signal
import subprocess
import sys
import tempfile
import time
import unittest
from pathlib import Path

HERE = Path(__file__).resolve().parent
FIXTURE = HERE / "fixture_signal_runner.py"


class SignalCleanupTests(unittest.TestCase):
    def test_sigterm_enters_controlled_cleanup_and_reaps_descendant(self) -> None:
        if not {12, 14}.issubset(os.sched_getaffinity(0)):
            self.skipTest("fixture CPUs 12 and 14 unavailable")
        with tempfile.TemporaryDirectory(prefix="candidate-signal-") as raw:
            output = Path(raw) / "output"
            process = subprocess.Popen([sys.executable, str(FIXTURE), str(output)], start_new_session=True)
            marker = output / "ready.json"
            deadline = time.monotonic() + 5
            while not marker.is_file() and process.poll() is None and time.monotonic() < deadline:
                time.sleep(0.02)
            self.assertTrue(marker.is_file(), f"fixture failed before ready rc={process.poll()}")
            pids = json.loads(marker.read_text(encoding="utf-8"))
            os.kill(process.pid, signal.SIGTERM)
            self.assertNotEqual(process.wait(timeout=8), 0)
            terminal = json.loads((output / "terminal.json").read_text(encoding="utf-8"))
            self.assertEqual(terminal["state"], "FAILED_SIGNAL")
            self.assertIn("SIGTERM", terminal["error"])
            self.assertTrue(terminal["cleanup"]["attempted"])
            self.assertFalse(terminal["cleanup"]["surviving_pids"])
            deadline = time.monotonic() + 2
            while any(Path(f"/proc/{pid}").exists() for pid in pids.values()) and time.monotonic() < deadline:
                time.sleep(0.02)
            self.assertFalse([pid for pid in pids.values() if Path(f"/proc/{pid}").exists()])


if __name__ == "__main__":
    unittest.main()
