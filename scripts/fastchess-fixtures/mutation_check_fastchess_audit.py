#!/usr/bin/env python3
"""Prove the fixture auditor rejects representative PGN corruptions."""

from __future__ import annotations

import argparse
import json
import re
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path


def replace_once(path: Path, old: str, new: str, regex: bool = False) -> None:
    text = path.read_text()
    if regex:
        updated, count = re.subn(old, new, text, count=1)
    else:
        count = text.count(old)
        updated = text.replace(old, new, 1)
    if count != 1:
        raise RuntimeError(f"mutation target count for {old!r} is {count}, wanted 1")
    path.write_text(updated)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--auditor", type=Path, required=True)
    parser.add_argument("--cases", type=Path, required=True)
    parser.add_argument("--results", type=Path, required=True)
    parser.add_argument("--stockfish", type=Path, required=True)
    parser.add_argument("--stockfish-sha256", required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    mutations = [
        {
            "name": "wrong_result_and_winner",
            "case": "final_ply_mate_beats_maxmoves",
            "old": '[Result "1-0"]', "new": '[Result "0-1"]',
            "required_failure": "Result='0-1'",
        },
        {
            "name": "changed_played_move",
            "case": "ordinary_maxmoves_control",
            "old": "1. e2e4 {", "new": "1. d2d4 {",
            "required_failure": "played moves",
        },
        {
            "name": "omitted_played_move",
            "case": "ordinary_maxmoves_control",
            "old": r'1\. e2e4 \{[^{}]*\}\s*',
            "new": "", "regex": True,
            "required_failure": "played moves",
        },
        {
            "name": "false_mate_label",
            "case": "ordinary_maxmoves_control",
            "old": "Draw by adjudication", "new": "White mates",
            "required_failure": "PGN terminal reasons",
        },
    ]
    results = []
    with tempfile.TemporaryDirectory(prefix="ngn-fixture-mutations-") as temporary:
        root = Path(temporary)
        for mutation in mutations:
            work = root / mutation["name"]
            shutil.copytree(args.results, work)
            pgn = work / mutation["case"] / "games.pgn"
            replace_once(pgn, mutation["old"], mutation["new"], mutation.get("regex", False))
            report = root / f"{mutation['name']}.json"
            command = [
                sys.executable, str(args.auditor), "--cases", str(args.cases),
                "--results", str(work), "--stockfish", str(args.stockfish),
                "--stockfish-sha256", args.stockfish_sha256, "--output", str(report),
            ]
            completed = subprocess.run(command, text=True, capture_output=True, timeout=180, check=False)
            parsed = json.loads(report.read_text()) if report.exists() else {}
            serialized = json.dumps(parsed)
            passed = (
                completed.returncode == 1
                and parsed.get("status") == "FAIL"
                and mutation["required_failure"] in serialized
            )
            results.append({
                "name": mutation["name"], "case": mutation["case"],
                "auditor_returncode": completed.returncode,
                "auditor_status": parsed.get("status"),
                "required_failure": mutation["required_failure"],
                "required_failure_observed": mutation["required_failure"] in serialized,
                "passed": passed,
            })
    output = {"status": "PASS" if all(row["passed"] for row in results) else "FAIL", "mutations": results}
    args.output.write_text(json.dumps(output, indent=2) + "\n")
    print(json.dumps(output, indent=2))
    return 0 if output["status"] == "PASS" else 1


if __name__ == "__main__":
    raise SystemExit(main())
