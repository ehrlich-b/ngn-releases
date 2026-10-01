#!/usr/bin/env python3
"""Compare exact old and optimized K4 UCI binaries on the frozen 32 FENs."""

import argparse
import hashlib
import json
import os
from pathlib import Path
import sys


PANEL_SHA = "96de37a99f6c7649a1bf297329e49ce20490aa1c0c7c1e935620ce67ad92e50f"
PRIOR_RESULT_SHA = "4b571bed01a0230d66402064c7b162b1aea45a27195a51310546846f8fcc0f67"
BASELINE_ENGINE_SHA = "3c3257bf14b43883ef7c88608fdb06eefe2379928119154ea6d38571c05e61b8"
MODEL_SHA = "034559653a83a7e64d4407badff334f66e3eae6a3a1f33147e3516b3c88c3e69"
UCI_RUNNER_SHA = "e5a32d1b61a39d1fff67b4c994a6876aa0116757505e6cb66efbc69f70164d25"


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for block in iter(lambda: source.read(1 << 20), b""):
            digest.update(block)
    return digest.hexdigest()


def require_sha(path: Path, expected: str) -> None:
    actual = sha256(path)
    if actual != expected:
        raise ValueError(f"SHA-256 mismatch for {path}: {actual} != {expected}")


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--panel", type=Path, required=True)
    parser.add_argument("--prior-result", type=Path, required=True)
    parser.add_argument("--uci-runner", type=Path, required=True)
    parser.add_argument("--baseline-engine", type=Path, required=True)
    parser.add_argument("--optimized-engine", type=Path, required=True)
    parser.add_argument("--optimized-engine-sha256", required=True)
    parser.add_argument("--k4-model", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    if args.output.exists():
        raise FileExistsError(args.output)
    for path, digest in ((args.panel, PANEL_SHA), (args.prior_result, PRIOR_RESULT_SHA),
                         (args.uci_runner, UCI_RUNNER_SHA), (args.baseline_engine, BASELINE_ENGINE_SHA),
                         (args.optimized_engine, args.optimized_engine_sha256), (args.k4_model, MODEL_SHA)):
        require_sha(path, digest)
    sys.path.insert(0, str(args.uci_runner.parent))
    from run_panel import UCI  # pylint: disable=import-outside-toplevel

    panel = json.loads(args.panel.read_text())
    prior = json.loads(args.prior_result.read_text())
    if panel["schema"] != "ngn-k4-search-panel-v1" or len(panel["positions"]) != 32 or \
       prior["schema"] != "ngn-k4-search-panel-result-v1" or prior["panel_sha256"] != PANEL_SHA or \
       prior["binary_sha256"]["engine"] != BASELINE_ENGINE_SHA or \
       prior["binary_sha256"]["k4"] != MODEL_SHA or len(prior["rows"]) != 32:
        raise ValueError("frozen panel/result contract mismatch")
    options = {"Threads": "1", "Hash": "128", "Move Overhead": "100"}
    sessions = {}
    try:
        for name, path in (("baseline", args.baseline_engine), ("optimized", args.optimized_engine)):
            sessions[name] = UCI(
                [str(path), "-eval-backend", "ngn-k4-768-v1", "-eval-file", str(args.k4_model)],
                options, name,
            )
        rows = []
        for index, position in enumerate(panel["positions"]):
            previous = prior["rows"][index]
            if position["id"] != previous["id"] or position["fen"] != previous["fen"]:
                raise ValueError(f"position order mismatch at {index}")
            row = {"id": position["id"], "fen": position["fen"],
                   "teacher_best_move": previous["teacher"]["best_move"]}
            for mode, budget, command in (("equal_nodes", 100_000, "nodes"),
                                          ("equal_time", 250, "movetime")):
                row[mode] = {}
                order = ("baseline", "optimized") if index % 2 == 0 else ("optimized", "baseline")
                for name in order:
                    row[mode][name] = sessions[name].search(position["fen"], command, budget)
            rows.append(row)
        summaries = {}
        for mode in ("equal_nodes", "equal_time"):
            summaries[mode] = {}
            for name in ("baseline", "optimized"):
                searches = [row[mode][name] for row in rows]
                summaries[mode][name] = {
                    "positions": len(rows),
                    "teacher_move_agreement": sum(
                        row[mode][name]["best_move"] == row["teacher_best_move"] for row in rows),
                    "mean_depth": sum(s["depth"] for s in searches) / len(searches),
                    "mean_reported_nodes": sum(s["nodes"] for s in searches) / len(searches),
                    "mean_elapsed_ms": sum(s["elapsed_ms"] for s in searches) / len(searches),
                }
            summaries[mode]["same_best_move"] = sum(
                row[mode]["baseline"]["best_move"] == row[mode]["optimized"]["best_move"] for row in rows)
        result = {
            "schema": "ngn-k4-speed-panel-result-v1",
            "panel_sha256": PANEL_SHA,
            "prior_result_sha256": PRIOR_RESULT_SHA,
            "runner_sha256": sha256(Path(__file__)),
            "engine_sha256": {"baseline": BASELINE_ENGINE_SHA,
                              "optimized": args.optimized_engine_sha256},
            "k4_model_sha256": MODEL_SHA,
            "cpu_affinity": sorted(os.sched_getaffinity(0)),
            "nice": os.nice(0),
            "summaries": summaries,
            "rows": rows,
        }
        with args.output.open("x") as destination:
            json.dump(result, destination, indent=2)
            destination.write("\n")
        print(json.dumps(summaries, indent=2))
    finally:
        for session in sessions.values():
            session.close()


if __name__ == "__main__":
    main()
