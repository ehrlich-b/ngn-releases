#!/usr/bin/env python3
"""Serial NGN Slice B checks. Intended to run under the frozen supervisor."""

import argparse
import json
import os
from pathlib import Path
import re
import subprocess


def run_one(name, argv, cwd, environment, output, expected_tests=()):
    result = subprocess.run(argv, cwd=cwd, env=environment, stdout=subprocess.PIPE,
                            stderr=subprocess.STDOUT, check=False)
    log = output / f"{name}.log"
    log.write_bytes(result.stdout)
    text = result.stdout.decode("utf-8", errors="strict")
    if result.returncode != 0:
        raise RuntimeError(f"{name} failed rc={result.returncode}; see {log}")
    if expected_tests and (re.search(r"(?m)^\s*--- SKIP:", text) or "[no tests to run]" in text):
        raise RuntimeError(f"{name} skipped or selected no required tests")
    for test in expected_tests:
        if text.count(f"=== RUN   {test}") != 1 or text.count(f"--- PASS: {test} ") != 1:
            raise RuntimeError(f"{name} missing/duplicate RUN/PASS for {test}")
    return {"name": name, "argv": argv, "returncode": result.returncode,
            "log": str(log), "bytes": len(result.stdout)}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--config", required=True, type=Path)
    args = parser.parse_args()
    config = json.loads(args.config.read_text())
    if config.get("schema") != "ngn-rodenteval-slice-b-check-config-v1":
        raise RuntimeError("wrong config schema")
    source = Path(config["source_root"]).resolve()
    output = Path(config["output"]).resolve()
    output.mkdir(parents=True, exist_ok=False)
    unit_tests = config["unit_tests"]
    oracle_test = config["oracle_test"]
    if not unit_tests or not oracle_test or len(set(unit_tests)) != len(unit_tests):
        raise RuntimeError("unit/oracle tests are absent or duplicated")
    base_env = {
        "PATH": "/usr/local/go/bin:/usr/bin:/bin", "HOME": str(output / "home"),
        "TMPDIR": str(output / "tmp"), "GOCACHE": str(output / "go-cache"),
        "LANG": "C", "LC_ALL": "C", "TZ": "UTC", "GOMAXPROCS": "2",
        "GOTOOLCHAIN": "local", "GOFLAGS": "-mod=readonly", "GOAMD64": "v3",
        "CGO_ENABLED": "0",
    }
    for directory in (Path(base_env["HOME"]), Path(base_env["TMPDIR"]), Path(base_env["GOCACHE"])):
        directory.mkdir(parents=True, exist_ok=True)
    oracle_env = dict(base_env)
    oracle_env.update(config["oracle_environment"])
    unit_regex = "^(" + "|".join(re.escape(name) for name in unit_tests) + ")$"
    rows = []
    rows.append(run_one("unit", ["/usr/local/go/bin/go", "test", "-v", "-count=1", "-run", unit_regex, "./rodenteval"],
                        source, base_env, output, unit_tests))
    rows.append(run_one("oracle", ["/usr/local/go/bin/go", "test", "-v", "-count=1", "-tags", "rodentoracle", "-run", "^" + re.escape(oracle_test) + "$", "./rodenteval"],
                        source, oracle_env, output, (oracle_test,)))
    race_env = dict(base_env)
    race_env["CGO_ENABLED"] = "1"
    rows.append(run_one("race", ["/usr/local/go/bin/go", "test", "-short", "-race", "-count=1", "-p=2", "./rodenteval"],
                        source, race_env, output))
    full_env = dict(base_env)
    full_env["CGO_ENABLED"] = "0"
    rows.append(run_one("fullshort", ["/usr/local/go/bin/go", "test", "-short", "-count=1", "-p=2", "./..."],
                        source, full_env, output))
    (output / "summary.json").write_text(json.dumps({
        "schema": "ngn-rodenteval-slice-b-check-summary-v1", "state": "COMPLETE",
        "required_tests_no_skips": True, "checks": rows,
    }, indent=2) + "\n")


if __name__ == "__main__":
    main()
