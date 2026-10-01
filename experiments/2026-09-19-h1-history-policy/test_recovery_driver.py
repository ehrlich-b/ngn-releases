#!/usr/bin/env python3
"""Focused tests for the H1 telemetry-race recovery driver."""

from __future__ import annotations

import importlib.util
from pathlib import Path
from unittest import mock
import unittest


DRIVER_PATH = Path(__file__).with_name("recovery_driver.py")
SPEC = importlib.util.spec_from_file_location("h1_recovery_driver", DRIVER_PATH)
assert SPEC is not None and SPEC.loader is not None
DRIVER = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(DRIVER)


class RecoveryDriverTests(unittest.TestCase):
    def test_procfs_process_disappearance_is_nonfatal(self) -> None:
        with mock.patch.object(Path, "read_text", side_effect=ProcessLookupError):
            self.assertIsNone(DRIVER.read_proc_entry(Path("/proc/999999")))

    def test_decision_rule_is_unchanged(self) -> None:
        self.assertEqual(
            DRIVER.decide({"upper95_elo": -0.01}),
            "SHELVE_CLEARLY_BAD_PILOT",
        )
        self.assertEqual(DRIVER.decide({"upper95_elo": 0.0}), "NEED_CONFIRMATION")
        self.assertEqual(
            DRIVER.decide({"upper95_elo": 20.0}, {"lower95_elo": 0.01}),
            "ACCEPT",
        )
        self.assertEqual(
            DRIVER.decide({"upper95_elo": 20.0}, {"lower95_elo": 0.0}),
            "SHELVE",
        )


if __name__ == "__main__":
    unittest.main()
