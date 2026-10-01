#!/usr/bin/env python3
"""Focused static tests for the frozen H1 driver."""

from __future__ import annotations

import copy
import importlib.util
from pathlib import Path
import tempfile
import unittest


DRIVER_PATH = Path(__file__).with_name("driver.py")
SPEC = importlib.util.spec_from_file_location("h1_driver", DRIVER_PATH)
assert SPEC is not None and SPEC.loader is not None
DRIVER = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(DRIVER)


def audit(half_points: list[int], prefix: str = "opening") -> dict:
    return {
        "pair_audit": [
            {
                "pair": index,
                "opening_identity": f"{prefix}-{index}",
                "half_points": value,
            }
            for index, value in enumerate(half_points, start=1)
        ]
    }


class DriverTests(unittest.TestCase):
    def manifest(self) -> dict:
        sources = {
            name: {"path": f"/frozen/{name}", "sha256": "0" * 64}
            for name in DRIVER.EXPECTED_SOURCES
        }
        engines = {
            policy: {
                "path": f"/frozen/ngn-h1-{policy}",
                "sha256": policy * 32,
                "build_tags": tags,
            }
            for policy, tags in DRIVER.EXPECTED_TAGS.items()
        }
        return {
            "schema": DRIVER.SCHEMA,
            "driver_sha256": "d" * 64,
            "release_state": DRIVER.HELD,
            "run_root": DRIVER.RUN_ROOT,
            "protocol": copy.deepcopy(DRIVER.PROTOCOL),
            "decision_rule": DRIVER.DECISION_RULE,
            "claim_boundary": DRIVER.CLAIM_BOUNDARY,
            "shared_host_contract": DRIVER.SHARED_HOST_CONTRACT,
            "source": {
                "commit": "a" * 40,
                "tree": "b" * 40,
                "go_version": "go1.25.5 linux/amd64",
                "goamd64": "v3",
                "cgo_enabled": "0",
                "build_flags": ["-trimpath"],
            },
            "engines": engines,
            "frozen_sources": sources,
        }

    def test_manifest_rejects_host_build_and_run_root_drift(self) -> None:
        original_run_root = DRIVER.RUN_ROOT
        self.addCleanup(setattr, DRIVER, "RUN_ROOT", original_run_root)
        with tempfile.TemporaryDirectory() as directory:
            DRIVER.RUN_ROOT = str(Path(directory) / "not-created")
            manifest = self.manifest()
            DRIVER.validate_manifest(manifest, "d" * 64)
            for key, value in (
                ("shared_host_contract", "changed"),
                ("run_root", str(Path(directory) / "other-run")),
            ):
                changed = copy.deepcopy(manifest)
                changed[key] = value
                with self.assertRaises(SystemExit):
                    DRIVER.validate_manifest(changed, "d" * 64)
            changed = copy.deepcopy(manifest)
            changed["source"]["goamd64"] = "v1"
            with self.assertRaises(SystemExit):
                DRIVER.validate_manifest(changed, "d" * 64)

    def test_engine_identity_accepts_validated_build_tags(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            engine = Path(directory) / "ngn"
            engine.write_bytes(b"frozen engine")
            spec = {
                "path": str(engine),
                "sha256": DRIVER.sha256(engine),
                "build_tags": ["h1producer", "h1consumer"],
            }
            self.assertEqual(DRIVER.checked_engine(spec, "engine 11"), engine.resolve())
            changed = copy.deepcopy(spec)
            changed["unexpected"] = True
            with self.assertRaises(SystemExit):
                DRIVER.checked_engine(changed, "engine 11")

    def test_joint_interaction_resamples_matched_openings(self) -> None:
        left = audit([1, 2, 1, 2])
        right = audit([3, 2, 3, 2])
        result = DRIVER.paired_interaction(left, right, 17, 1000, lambda score: score)
        self.assertEqual(result["pairs"], 4)
        self.assertAlmostEqual(result["elo_difference"], 0.25)
        self.assertEqual(result["seed"], 17)

    def test_joint_interaction_rejects_opening_misalignment(self) -> None:
        left = audit([1, 2, 3])
        right = audit([1, 2, 3], prefix="different")
        with self.assertRaises(SystemExit):
            DRIVER.paired_interaction(left, right, 17, 100, lambda score: score)


if __name__ == "__main__":
    unittest.main()
