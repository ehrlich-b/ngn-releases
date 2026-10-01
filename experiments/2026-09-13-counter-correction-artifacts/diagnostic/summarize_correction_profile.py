#!/usr/bin/env python3
import hashlib
import json
import sys
from pathlib import Path

observed_path = Path(sys.argv[1])
baseline_path = Path(sys.argv[2])
output_path = Path(sys.argv[3])
observed = json.loads(observed_path.read_text())
baseline = json.loads(baseline_path.read_text())
if len(observed["roots"]) != 6 or len(baseline["persistent_roots"]) != 6:
    raise SystemExit("expected six observed and baseline roots")

identity = []
for got, want in zip(observed["roots"], baseline["persistent_roots"]):
    if (got["game_index"], got["after_ply"]) != (want["game_index"], want["after_ply"]):
        raise SystemExit("root order mismatch")
    same = got["completed_iteration_decisions"] == want["trajectory"]
    if not same:
        raise SystemExit(f"frozen baseline mismatch game={got['game_index']} ply={got['after_ply']}")
    identity.append({
        "game_index": got["game_index"],
        "after_ply": got["after_ply"],
        "off_on_identity": got["off_on_identity"],
        "frozen_baseline_identity": same,
        "trajectory_sha256": got["control_trajectory_sha256"],
        "completed_iterations": len(got["completed_iteration_decisions"]["callbacks"]),
        "final_best_move": got["completed_iteration_decisions"]["final"]["pv"][0],
        "final_best_score": got["completed_iteration_decisions"]["final"]["scalars"]["BestScore"],
        "final_nodes": got["completed_iteration_decisions"]["final"]["scalars"]["Nodes"],
    })

value_names = ["raw_static", "pawn_term", "nonpawn_term", "minor_term", "aggregate"]
predicate_names = ["rfp", "futility", "nmp", "q_stand_pat_beta", "q_stand_pat_alpha"]
summary = {
    "main_nodes": sum(r["trace"]["main_nodes"] for r in observed["roots"]),
    "q_nodes": sum(r["trace"]["q_nodes"] for r in observed["roots"]),
    "values": {}, "tt": {}, "predicates": {},
}
for name in value_names:
    records = [r["trace"][name] for r in observed["roots"]]
    summary["values"][name] = {
        key: sum(x[key] for x in records)
        for key in ("count", "nonzero", "positive", "negative", "sum", "abs_sum")
    }
    summary["values"][name]["min"] = min(x["min"] for x in records)
    summary["values"][name]["max"] = max(x["max"] for x in records)
summary["terms_all_same_nonzero_sign"] = sum(r["trace"]["terms_all_same_nonzero_sign"] for r in observed["roots"])
summary["terms_mixed_sign"] = sum(r["trace"]["terms_mixed_sign"] for r in observed["roots"])
summary["entry_at_limit"] = {
    name: sum(r["trace"][name + "_entry_at_limit"] for r in observed["roots"])
    for name in ("pawn", "nonpawn", "minor")
}
for key in ("hits", "compatible", "live_applied", "minor_off_applied", "eligibility_changed"):
    summary["tt"][key] = sum(r["trace"]["tt"][key] for r in observed["roots"])
for name in predicate_names:
    summary["predicates"][name] = {
        key: sum(r["trace"][name][key] for r in observed["roots"])
        for key in ("eligible", "live_true", "minor_off_true", "turned_on_by_minor", "turned_off_by_minor")
    }

result = {
    "schema": "ngn-counter-correction-profile-summary-v1",
    "observed_sha256": hashlib.sha256(observed_path.read_bytes()).hexdigest(),
    "frozen_baseline_sha256": hashlib.sha256(baseline_path.read_bytes()).hexdigest(),
    "identity": identity,
    "aggregate": summary,
    "interpretation_limit": observed["counterfactual_scope"],
}
output_path.write_text(json.dumps(result, indent=2, sort_keys=True) + "\n")
print(json.dumps(result, indent=2, sort_keys=True))
