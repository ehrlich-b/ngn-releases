#!/usr/bin/env python3
"""Reproduce review findings without changing production Go files.

Expected exit: 1 on the reviewed release, with explicit trace/quiet-filter
failures; 0 after the tuning-contract repair. The scout and node-cap tests report observations and pass. The
ordinary repository test suite does not include these review-only probes.
"""
import json
import os
from pathlib import Path
import subprocess


root = Path(__file__).resolve().parents[1]
out = root / "output/review-2026-09-04"
out.mkdir(parents=True, exist_ok=True)
source = (root / "engine/search.go").read_text()
needle = "\t\t\t// Search with null window first (not PV)\n"
if source.count(needle) != 1:
    raise SystemExit("Search changed: review instrumentation needs revalidation")
instrumented = source.replace(needle, """\t\t\tif reduction == 0 && nextDepth > reducedDepth {
\t\t\t\treviewScoutSkips++
\t\t\t\tif reviewScoutFirst == "" { reviewScoutFirst = GenerateFEN(pos) }
\t\t\t}
""" + needle)
(out / "search_instrumented.go.txt").write_text(instrumented)
probe = root / "experiments/2026-09-04-review-probe_test.go.txt"
overlay = {"Replace": {
    str(root / "engine/search.go"): str(out / "search_instrumented.go.txt"),
    str(root / "engine/review_probe_test.go"): str(probe),
}}
(out / "overlay.json").write_text(json.dumps(overlay, indent=2) + "\n")
command = ["go", "test", "-overlay", str(out / "overlay.json"), "./engine",
           "-run", "^TestReview", "-count=1", "-v"]
result = subprocess.run(command, cwd=root, env=os.environ, text=True,
                        stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
                        timeout=120)
(out / "review-probes.txt").write_text(result.stdout)
print(result.stdout, end="")
raise SystemExit(result.returncode)
