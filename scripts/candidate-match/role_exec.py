#!/usr/bin/env python3
"""Hash-check a frozen role binary, set its declared Go width, and exec it."""
from __future__ import annotations

import hashlib
import json
import os
import sys
from pathlib import Path


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for block in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def fail(message: str) -> None:
    print(f"candidate role launcher error: {message}", file=sys.stderr, flush=True)
    raise SystemExit(111)


def main() -> None:
    config_path = Path(__file__).resolve().with_name("role-config.json")
    try:
        config = json.loads(config_path.read_text(encoding="utf-8"))
        if set(config) != {"schema", "engine", "engine_sha256", "gomaxprocs", "expected_cwd"}:
            fail("unexpected role config keys")
        if config["schema"] != "ngn-candidate-role-exec-v1":
            fail("unsupported role config schema")
        engine = Path(config["engine"]).resolve(strict=True)
        expected_cwd = Path(config["expected_cwd"]).resolve(strict=True)
        if Path.cwd().resolve() != expected_cwd:
            fail(f"cwd mismatch {Path.cwd().resolve()} != {expected_cwd}")
        if not engine.is_file() or not os.access(engine, os.X_OK):
            fail(f"engine is not executable: {engine}")
        if sha256(engine) != config["engine_sha256"]:
            fail("engine SHA-256 mismatch")
        gomaxprocs = config["gomaxprocs"]
        if gomaxprocs != "1":
            fail("v1 role launcher permits GOMAXPROCS=1 only")
    except SystemExit:
        raise
    except Exception as error:
        fail(str(error))
    environment = dict(os.environ)
    environment["GOMAXPROCS"] = gomaxprocs
    os.execve(engine, [str(engine)], environment)


if __name__ == "__main__":
    main()
