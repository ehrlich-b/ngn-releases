#!/usr/bin/python3
from __future__ import annotations

import os
import subprocess
import sys
from pathlib import Path

variant = sys.argv[1]
engine_a, cwd_a, cpu_a = sys.argv[2], sys.argv[3], int(sys.argv[4])
engine_b, cwd_b, cpu_b = sys.argv[5], sys.argv[6], int(sys.argv[7])
children = []
for index, (engine, cwd, cpu) in enumerate(((engine_a, cwd_a, cpu_a), (engine_b, cwd_b, cpu_b))):
    env = dict(os.environ)
    env["GOMAXPROCS"] = "2" if variant == "bad-env" and index == 0 else "1"
    children.append(subprocess.Popen(
        [engine, "0.6"], cwd=cwd, env=env,
        preexec_fn=lambda chosen=cpu: os.sched_setaffinity(0, {chosen}),
    ))
if variant == "wrong-cwd-extra":
    env = dict(os.environ)
    env["GOMAXPROCS"] = "1"
    children.append(subprocess.Popen(
        [engine_a, "0.6"], cwd=str(Path(cwd_a).parent), env=env,
        preexec_fn=lambda: os.sched_setaffinity(0, {cpu_a}),
    ))
raise SystemExit(max(child.wait() for child in children))
