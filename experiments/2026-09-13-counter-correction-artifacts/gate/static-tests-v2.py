#!/usr/bin/env python3
import importlib.util
from pathlib import Path

DRIVER = Path("/home/ehrli/repos/ngn-counter-policy-gate-20260913/driver-held-v2.py")
spec = importlib.util.spec_from_file_location("counter_policy_gate", DRIVER)
gate = importlib.util.module_from_spec(spec)
spec.loader.exec_module(gate)

assert gate.BASE_COMMIT == "270c73563139996d03f3c34b2840015b8e130df9"
assert gate.EXPECTED_FROZEN_SOURCES == {
    "role_exec.py", "run_match_stage.py", "process_supervisor.py", "trace_audit.py",
    "audit_fastchess_match.py", "uci_preflight.py", "common.py", "fastchess", "stockfish",
    "openings", "prefixes", "model", "candidate_production_patch",
    "candidate_regression_patch", "build_test_receipt",
}
assert gate.EXPECTED_PROTOCOL == {
    "aa_games": 100, "candidate_games": 400, "tc": "10+0.1", "concurrency": 4,
    "physical_cpu_mask": ["0", "2", "4", "6"], "seed": 20260912,
    "bootstrap_seed": 2026091201, "bootstrap_replicates": 100000,
}

model = Path("/frozen/n-30-5268.nn")
assert gate.options(model) == [
    {"name": "Threads", "value": "1"},
    {"name": "OwnBook", "value": "false"},
    {"name": "EvalFile", "value": str(model)},
    {"name": "EvalBackend", "value": "counter-5.5"},
    {"name": "Hash", "value": "128"},
    {"name": "Move Overhead", "value": "100"},
]

roles = [
    {"launcher": Path("/frozen/a"), "name": "CANDIDATE-A", "cwd": Path("/role/a")},
    {"launcher": Path("/frozen/b"), "name": "BASE-B", "cwd": Path("/role/b")},
]
argv = gate.phase_command(Path("/tools"), Path("/phase"), roles, model,
                          Path("/inputs/openings.pgn"), 200, 20260912, ["0", "2", "4", "6"])
joined = "\n".join(argv)
for exact in ("tc=10+0.1", "-concurrency", "4", "-games", "2", "-repeat",
              "-strict", "-use-affinity", "0,2,4,6", "order=sequential", "plies=6"):
    assert exact in argv or exact in joined
assert "-resign" not in argv and "-draw" not in argv

pairs = [{"half_points": value} for value in (0, 1, 2, 3, 4)]
first = gate.paired_bootstrap(pairs, 2026091201, 100)
second = gate.paired_bootstrap(pairs, 2026091201, 100)
assert first == second and first["pairs"] == 5 and first["replicates"] == 100

print("PASS static driver checks")
