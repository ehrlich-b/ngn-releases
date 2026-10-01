#!/usr/bin/env python3
import os
import subprocess
import sys

variant, engine_a, cwd_a, engine_b, cwd_b = sys.argv[1:]
processes = []
try:
    for index, (engine, cwd) in enumerate([(engine_a, cwd_a), (engine_b, cwd_b)]):
        width = '1' if variant == 'bad-env' and index == 0 else '2'
        mask = {12} if variant == 'bad-mask' and index == 0 else {12, 14}
        processes.append(subprocess.Popen([engine, '2'], cwd=cwd,
            env={'PATH': '/usr/bin:/bin', 'GOMAXPROCS': width}, preexec_fn=lambda: os.sched_setaffinity(0, mask)))
    for process in processes:
        process.wait()
finally:
    for process in processes:
        if process.poll() is None:
            process.terminate(); process.wait()
