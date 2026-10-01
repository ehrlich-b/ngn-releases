#!/usr/bin/env python3
"""Focused static tests for the frozen E0 driver."""

from __future__ import annotations

import copy
import importlib.util
from pathlib import Path
import tempfile
import unittest


DRIVER_PATH = Path(__file__).with_name("driver.py")
SPEC = importlib.util.spec_from_file_location("e0_driver", DRIVER_PATH)
assert SPEC is not None and SPEC.loader is not None
DRIVER = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(DRIVER)


class DriverTests(unittest.TestCase):
    def test_role_options_are_exact_and_transactionally_ordered(self) -> None:
        model = Path("/tmp/model.bin")
        for backend in ("rodent-v1.1-anand", "rodent-v1.2-default"):
            values = DRIVER.options(model, backend)
            self.assertEqual(
                [item["name"] for item in values],
                ["Threads", "OwnBook", "EvalFile", "EvalBackend", "Hash", "Move Overhead"],
            )
            self.assertEqual(values[0]["value"], "1")
            self.assertEqual(values[1]["value"], "false")
            self.assertEqual(values[2]["value"], str(model))
            self.assertEqual(values[3]["value"], backend)
            self.assertEqual(values[4]["value"], "128")
            self.assertEqual(values[5]["value"], "100")

    def test_balanced_paired_statistics_include_zero(self) -> None:
        normal = DRIVER.paired_ci([0, 0, 50, 0, 0])
        self.assertEqual(normal["elo"], 0.0)
        self.assertLessEqual(normal["lower95_elo"], 0.0)
        self.assertGreaterEqual(normal["upper95_elo"], 0.0)
        audit = [{"half_points": 2} for _ in range(50)]
        bootstrap = DRIVER.paired_bootstrap(audit, 2026091201, 1000)
        self.assertEqual(bootstrap["elo"], 0.0)
        self.assertEqual(bootstrap["lower95_elo"], 0.0)
        self.assertEqual(bootstrap["upper95_elo"], 0.0)

    def test_bootstrap_orientation_is_engine_a(self) -> None:
        winning = DRIVER.paired_bootstrap([{"half_points": 3}] * 20, 7, 1000)
        losing = DRIVER.paired_bootstrap([{"half_points": 1}] * 20, 7, 1000)
        self.assertGreater(winning["lower95_elo"], 0.0)
        self.assertLess(losing["upper95_elo"], 0.0)

    def manifest(self, run_root: Path) -> dict:
        sources = {
            name: {"path": f"/frozen/{name}", "sha256": "0" * 64}
            for name in DRIVER.EXPECTED_FROZEN_SOURCES
        }
        return {
            "schema": DRIVER.SCHEMA,
            "driver_sha256": "d" * 64,
            "release_state": DRIVER.HELD_STATE,
            "run_root": str(run_root),
            "protocol": copy.deepcopy(DRIVER.EXPECTED_PROTOCOL),
            "decision_rule": DRIVER.DECISION_RULE,
            "claim_boundary": DRIVER.CLAIM_BOUNDARY,
            "startup_contract": DRIVER.STARTUP_CONTRACT,
            "shared_host_contract": DRIVER.SHARED_HOST_CONTRACT,
            "engine": {
                "path": "/frozen/ngn",
                "sha256": "0" * 64,
                "source_commit": "commit",
                "source_tree": "tree",
                "go_version": "go1.25.5",
                "goamd64": "v3",
                "cgo_enabled": "0",
                "build_flags": ["-trimpath"],
            },
            "frozen_sources": sources,
        }

    def test_manifest_contract_rejects_protocol_and_build_drift(self) -> None:
        with tempfile.TemporaryDirectory() as temporary:
            absent = Path(temporary) / "run-001"
            manifest = self.manifest(absent)
            DRIVER.validate_manifest(manifest, "d" * 64)
            wrong_protocol = copy.deepcopy(manifest)
            wrong_protocol["protocol"]["candidate_games"] = 402
            with self.assertRaises(SystemExit):
                DRIVER.validate_manifest(wrong_protocol, "d" * 64)
            wrong_build = copy.deepcopy(manifest)
            wrong_build["engine"]["goamd64"] = "v1"
            with self.assertRaises(SystemExit):
                DRIVER.validate_manifest(wrong_build, "d" * 64)


if __name__ == "__main__":
    unittest.main()
