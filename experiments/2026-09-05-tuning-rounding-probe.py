#!/usr/bin/env python3
"""Run an isolated integer/float diagnostic; never fit or alter weights on disk."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import subprocess

root = Path(__file__).resolve().parents[1]
parser = argparse.ArgumentParser()
parser.add_argument("--model", type=Path, required=True)
parser.add_argument("--output-dir", type=Path, required=True)
args = parser.parse_args()
model = args.model.resolve()
out = args.output_dir.resolve()
out.mkdir(parents=True, exist_ok=False)
probe = root / "experiments/2026-09-05-tuning-rounding-probe_test.go.txt"
overlay = out / "overlay.json"
overlay.write_text(json.dumps({"Replace": {str(root / "engine/review_tuning_rounding_test.go"): str(probe)}}, indent=2)+"\n")
command = ["go", "test", "-overlay", str(overlay), "./engine", "-run", "^TestReviewTuningRounding$", "-count=1", "-v"]
manifest = {"command": command, "model": str(model), "model_sha256": hashlib.sha256(model.read_bytes()).hexdigest(), "probe_sha256": hashlib.sha256(probe.read_bytes()).hexdigest(), "source_commit": subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=root, text=True).strip(), "engine_diff": subprocess.check_output(["git", "diff", "HEAD", "--", "engine"], cwd=root, text=True)}
(out / "manifest.json").write_text(json.dumps(manifest, indent=2)+"\n")
run = subprocess.run(command, cwd=root, env={**os.environ, "NGN_REVIEW_MODEL": str(model)}, text=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, timeout=120)
(out / "result.txt").write_text(run.stdout)
print(run.stdout, end="")
raise SystemExit(run.returncode)
