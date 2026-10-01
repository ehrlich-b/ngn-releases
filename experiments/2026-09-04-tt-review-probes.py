#!/usr/bin/env python3
"""Reproduce TT capture-budget and age-field defects without editing engine files.

Expected nonzero on 70eb527: actual short-budget qsearch poisons a full-budget
TT cutoff; generation 512 exceeds the stored age width. No synthetic TT score
is used for the qsearch probe. Probe tests are excluded from ordinary go test.
"""
import json
import os
from pathlib import Path
import subprocess

root = Path(__file__).resolve().parents[1]
out = root / "output/review-2026-09-04"
out.mkdir(parents=True, exist_ok=True)
overlay = {"Replace": {
    str(root / "engine/tt_review_probe_test.go"):
        str(root / "experiments/2026-09-04-tt-review-probe_test.go.txt"),
}}
path = out / "tt-overlay.json"
path.write_text(json.dumps(overlay, indent=2) + "\n")
result = subprocess.run(
    ["go", "test", "-overlay", str(path), "./engine", "-run", "^TestReview(QBudgetTT|TTAgeWidth)$", "-count=1", "-v"],
    cwd=root, env=os.environ, text=True, stdout=subprocess.PIPE,
    stderr=subprocess.STDOUT, timeout=120,
)
(out / "tt-review-probes.txt").write_text(result.stdout)
print(result.stdout, end="")
raise SystemExit(result.returncode)
