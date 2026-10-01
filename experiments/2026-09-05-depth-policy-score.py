#!/usr/bin/env python3
"""Annotate all frozen depth-policy choices, preserving history and controls."""
import argparse
import hashlib
import importlib.util
import json
from pathlib import Path
import sys
import time

sys.dont_write_bytecode = True
ROOT = Path(__file__).resolve().parents[1]
STUDY = ROOT / "output/recovery-2026-09-04/safe-study-result.json"
SF = Path("/opt/homebrew/bin/stockfish")


def sha(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def module(name, filename):
    spec = importlib.util.spec_from_file_location(name, ROOT / filename)
    loaded = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(loaded)
    return loaded


def save(path, value):
    path.write_text(json.dumps(value, indent=2) + "\n")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--input", type=Path, required=True)
    parser.add_argument("--output-dir", type=Path, required=True)
    args = parser.parse_args()
    source = json.loads(args.input.read_text())
    if source["status"] != "complete" or len(source["roots"]) != 23:
        raise ValueError("expected the complete 23-root diagnostic")
    if len(source["identity"]) != 26 or not all(all(r["checks"].values()) for r in source["identity"]):
        raise ValueError("instrumentation identity is incomplete")
    if sha(STUDY) != "95926b6cc5dd4aa66615b071ead937f413d9c9f7ed4b3957835d1a6afcf82737":
        raise ValueError("frozen study changed")
    study = {r["game"]["line_number"]: r for r in json.loads(STUDY.read_text())["games"] if r.get("status") == "selected"}
    if {r["line_number"] for r in source["roots"]} != set(study):
        raise ValueError("root selection differs from the frozen study")
    ctrl = module("controls", "experiments/2026-09-05-futility-controls.py")
    oracle_module = module("oracle", "experiments/2026-09-05-corpus-smoke-oracle.py")
    for row in source["roots"]:
        original = study[row["line_number"]]
        if row["prefix"] != original["prefix"] or row["fen"] != original["fen"]:
            raise ValueError("root history/FEN mismatch")
        if set(row["runs"]) != {"A", "B", "C", "D"}:
            raise ValueError("missing or unexpected arm")
        if ctrl.final(row["runs"]["A"]["raw"]) != ctrl.final(original["ngn"]["raw"]):
            raise ValueError(f"frozen baseline identity mismatch: {row['line_number']}")
    out = args.output_dir.resolve()
    out.mkdir(parents=True, exist_ok=False)
    save(out / "input.json", source)
    report = {"status": "running", "input": str(args.input.resolve()), "input_sha256": sha(args.input),
              "study_sha256": sha(STUDY), "script_sha256": sha(__file__),
              "stockfish": {"path": str(SF), "sha256": sha(SF), "command": "go depth 16", "hash_mb": 64, "threads": 1},
              "cases": []}
    save(out / "manifest.json", report)
    start = time.monotonic()
    ctrl.DEADLINE = start + 900
    sf = oracle = None
    try:
        sf = ctrl.Engine(SF, "stockfish")
        oracle = oracle_module.Stockfish()
        for row in source["roots"]:
            choices = {}
            for arm, run in row["runs"].items():
                choices.setdefault(run["bestmove"], []).append(arm)
            _, _, legal = oracle.position(row["prefix"])
            if not set(choices).issubset(legal):
                raise ValueError(f"illegal chosen move: {row['line_number']}")
            case = {"line_number": row["line_number"], "game": study[row["line_number"]]["game"],
                    "prefix": row["prefix"], "fen": row["fen"], "choices": []}
            for move, arms in sorted(choices.items()):
                reply = sf.run(row["prefix"] + [move], "go depth 16")
                if not reply["usable"] or reply["depth"] < 16:
                    raise ValueError("incomplete or bounded reference score")
                case["choices"].append({"move": move, "arms": arms, "reply": reply, "root_pov_cp": -reply["cp"]})
            baseline = next(c["root_pov_cp"] for c in case["choices"] if "A" in c["arms"])
            for choice in case["choices"]:
                choice["delta_vs_A_cp"] = choice["root_pov_cp"] - baseline
            report["cases"].append(case)
            save(out / "partial.json", report)
        summary = {}
        for arm in ("B", "C", "D"):
            differences = [next(c["delta_vs_A_cp"] for c in case["choices"] if arm in c["arms"]) for case in report["cases"]]
            summary[arm] = {"cases": len(differences), "deltas_cp": differences,
                            "at_least_50cp_better": sum(d >= 50 for d in differences),
                            "at_least_50cp_worse": sum(d <= -50 for d in differences),
                            "negative": sum(d < 0 for d in differences)}
        report.update(status="complete", summary=summary, elapsed_seconds=time.monotonic()-start)
        save(out / "result.json", report)
        print(json.dumps({"status": report["status"], "summary": summary}, indent=2))
    finally:
        for engine in (sf, oracle):
            if engine is not None:
                engine.close()


if __name__ == "__main__":
    main()
