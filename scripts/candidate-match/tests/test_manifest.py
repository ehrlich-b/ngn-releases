from __future__ import annotations

import copy
import sys
import unittest
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE.parent))

from manifest import HELD_APPROVAL, ManifestError, review_subject_sha256, validate_manifest
from run_candidate_match import budget_options

SHA = "a" * 64


def file_spec(path: str = "/tmp/input") -> dict:
    return {"path": path, "sha256": SHA}


def binary_spec() -> dict:
    return {
        **file_spec("/tmp/same engine"),
        "provenance": {
            "source_commit": "4bd54add",
            "source_tree": "clean",
            "build_receipt": file_spec("/tmp/build receipt.json"),
            "build_command": ["go", "build"],
        },
    }


def role(role_id: str, backend: str) -> dict:
    network = None if backend == "hce" else {**file_spec("/tmp/pilot network.ngn"), "format": backend}
    eval_file = "<empty>" if backend == "hce" else "$FROZEN_NETWORK"
    return {
        "id": role_id,
        "display_name": role_id.upper(),
        "binary": binary_spec(),
        "network": network,
        "capability_profile": "ngn-pre-m4c-pinned-one-worker-v1",
        "backend": backend,
        "uci_options": [
            {"name": "Threads", "value": 1},
            {"name": "OwnBook", "value": False},
            {"name": "EvalFile", "value": eval_file},
            {"name": "EvalBackend", "value": backend},
            {"name": "Hash", "value": 64},
            {"name": "Move Overhead", "value": 50},
        ],
        "requested": {"Threads": 1, "OwnBook": False, "Hash": 64, "Move Overhead": 50},
        "effective": {
            "Threads": 1, "GOMAXPROCS": 1, "OwnBook": False, "Hash": 64,
            "Move Overhead": 50,
            "threads_basis": "pinned-source-single-worker-plus-one-cpu-mask",
        },
        "environment": {"GOMAXPROCS": "1"},
    }


def manifest() -> dict:
    names = (
        "runner", "schema", "common", "manifest_module", "supervisor",
        "uci_preflight", "match_stage", "trace_auditor", "role_exec",
        "chess_auditor", "fastchess", "stockfish", "opening_pgn", "opening_prefixes",
    )
    return {
        "schema": "ngn-candidate-match-v1", "status": "HELD",
        "purpose": "bounded same-code evaluator diagnostic", "classification": "diagnostic",
        "approval": copy.deepcopy(HELD_APPROVAL),
        "inputs": {**{name: file_spec(f"/tmp/{name} source") for name in names},
                   "roles": [role("hce", "hce"), role("nnue", "ngn-v1")]},
        "match": {
            "games": 2, "pairs": 1, "time_control": "1+0.01", "concurrency": 2,
            "seed": 20260906, "opening_plies": 6, "use_affinity": True,
            "affinity_cpus": [12, 14], "runner_strict": True, "recover": False,
            "ponder": False, "book": False, "tablebase": False,
            "score_adjudication": False, "resign_adjudication": False,
            "draw_adjudication": False, "maxmoves_adjudication": False,
        },
        "resources": {
            "runner_cpus": [12, 14], "required_cpus_allowed_list": "12,14",
            "memory_limit_kib": 1048576, "affinity_mode": "fastchess-per-game-one-worker",
        },
        "limits": {
            "preflight_seconds": 10, "match_seconds": 60, "trace_audit_seconds": 10,
            "chess_audit_seconds": 30, "term_grace_seconds": 2,
            "sample_interval_seconds": 0.02,
        },
        "operational_profile": {
            "name": "ngn-pre-m4c-known-benign-v1", "allow_fastchess_normal_exit": True,
            "ngn_crash_handler": "required-two-line-init-per-observed-process",
        },
        "acceptance": {
            "expected_games": 2, "expected_pairs": 1, "require_independent_audit": True,
            "require_zero_operational_errors": True, "require_zero_survivors": True,
            "strength_claim": "none",
        },
    }


class ManifestTests(unittest.TestCase):
    def test_borrowed_comparison_requires_exact_k4_rodent_pair(self) -> None:
        value = manifest()
        value["inputs"]["roles"] = [role("k4", "ngn-k4-768-v1"), role("rodent", "rodent-v1.1-anand")]
        with self.assertRaisesRegex(ManifestError, "v1 requires one HCE"):
            validate_manifest(value)
        value["schema"] = "ngn-candidate-match-borrowed-v1"
        self.assertIs(validate_manifest(value), value)
        value["inputs"]["roles"][0] = role("hce", "hce")
        with self.assertRaisesRegex(ManifestError, "requires owned K4 and Rodent"):
            validate_manifest(value)

    def test_fixed_node_borrowed_contract_forbids_clock_and_mixed_budget(self) -> None:
        value = manifest()
        value["inputs"]["roles"] = [role("k4", "ngn-k4-768-v1"), role("rodent", "rodent-v1.1-anand")]
        value["schema"] = "ngn-candidate-match-borrowed-fixed-nodes-v1"
        value["match"].pop("time_control")
        value["match"]["node_limit"] = 160_000
        self.assertIs(validate_manifest(value), value)
        self.assertEqual(budget_options(value["match"]), ["nodes=160000"])
        value["match"]["time_control"] = "10+0.1"
        with self.assertRaisesRegex(ManifestError, "unknown"):
            validate_manifest(value)
        value["match"].pop("time_control")
        value["match"]["node_limit"] = 0
        with self.assertRaisesRegex(ManifestError, "node_limit"):
            validate_manifest(value)

    def test_clock_budget_preserved(self) -> None:
        self.assertEqual(budget_options(manifest()["match"]), ["tc=1+0.01", "timemargin=0"])

    def test_held_and_bound_approved_manifest(self) -> None:
        value = manifest()
        self.assertIs(validate_manifest(value), value)
        digest = review_subject_sha256(value)
        approved = copy.deepcopy(value)
        approved["status"] = "APPROVED_TO_RUN"
        approved["approval"] = {
            "authority": "root", "token": "fixture-approval",
            "exclusive_match_window": True, "reviewed_manifest_sha256": digest,
        }
        self.assertIs(validate_manifest(approved), approved)

    def test_experiment_mutation_invalidates_approval(self) -> None:
        value = manifest()
        digest = review_subject_sha256(value)
        value["status"] = "APPROVED_TO_RUN"
        value["approval"] = {
            "authority": "root", "token": "fixture-approval",
            "exclusive_match_window": True, "reviewed_manifest_sha256": digest,
        }
        value["match"]["time_control"] = "30+0.3"
        with self.assertRaisesRegex(ManifestError, "does not bind"):
            validate_manifest(value)

    def test_ownbook_must_be_false_and_ordered(self) -> None:
        value = manifest()
        value["inputs"]["roles"][0]["uci_options"][1]["value"] = True
        with self.assertRaises(ManifestError):
            validate_manifest(value)
        value = manifest()
        value["inputs"]["roles"][0]["uci_options"][1:3] = reversed(value["inputs"]["roles"][0]["uci_options"][1:3])
        with self.assertRaisesRegex(ManifestError, "canonical order"):
            validate_manifest(value)

    def test_boolean_numeric_aliases_are_rejected(self) -> None:
        mutations = (
            lambda value: value["inputs"]["roles"][0]["uci_options"][0].__setitem__("value", True),
            lambda value: value["inputs"]["roles"][0]["requested"].__setitem__("Threads", True),
            lambda value: value["inputs"]["roles"][0]["effective"].__setitem__("Threads", True),
            lambda value: value["inputs"]["roles"][0]["effective"].__setitem__("GOMAXPROCS", True),
        )
        for mutate in mutations:
            with self.subTest(mutation=mutate):
                value = manifest()
                mutate(value)
                with self.assertRaises(ManifestError):
                    validate_manifest(value)

    def test_nonfinite_limits_are_rejected(self) -> None:
        for limit in (float("nan"), float("inf"), float("-inf")):
            with self.subTest(limit=limit):
                value = manifest()
                value["limits"]["match_seconds"] = limit
                with self.assertRaisesRegex(ManifestError, "finite positive"):
                    validate_manifest(value)

    def test_evalfile_must_match_role(self) -> None:
        value = manifest()
        value["inputs"]["roles"][1]["uci_options"][2]["value"] = "<empty>"
        with self.assertRaisesRegex(ManifestError, "backend/thread/file mismatch"):
            validate_manifest(value)

    def test_k4_exact_network_role_is_supported(self) -> None:
        value = manifest()
        value["inputs"]["roles"][1] = role("k4", "ngn-k4-768-v1")
        self.assertIs(validate_manifest(value), value)

    def test_network_format_must_match_backend(self) -> None:
        value = manifest()
        value["inputs"]["roles"][1]["network"]["format"] = "ngn-k4-768-v1"
        with self.assertRaisesRegex(ManifestError, "format must equal backend"):
            validate_manifest(value)


if __name__ == "__main__":
    unittest.main()
