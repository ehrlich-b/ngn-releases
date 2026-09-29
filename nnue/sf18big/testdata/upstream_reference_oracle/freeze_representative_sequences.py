#!/usr/bin/env python3
"""Freeze representative legal prefixes from a bound legacy game artifact."""

import argparse
import hashlib
import json
import pathlib


SOURCE_SHA256 = "4c69caf87e01f31bb82890d75f65290a6093e90490e3c8ee83dcfe1c66c85023"
SOURCE_COMMIT = "cb3d4ee9b47d0c5aae855b12379378ea1439675c"
ROOT_FEN = "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"
CASE_COUNT = 12
PREFIX_PLIES = 64


def sha256_file(path: pathlib.Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for chunk in iter(lambda: source.read(1 << 20), b""):
            digest.update(chunk)
    return digest.hexdigest()


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--source", required=True)
    parser.add_argument("--output", required=True)
    args = parser.parse_args()
    source = pathlib.Path(args.source)
    output = pathlib.Path(args.output)
    if output.exists():
        raise SystemExit(f"refusing existing output {output}")
    if sha256_file(source) != SOURCE_SHA256:
        raise SystemExit("representative game source SHA-256 mismatch")
    games = []
    for line in source.read_text().splitlines():
        if not line.startswith("GAME "):
            continue
        parts = line.split(" | ", 1)
        if len(parts) != 2:
            raise SystemExit("malformed GAME line")
        actions = parts[1].split()
        if len(actions) < PREFIX_PLIES:
            continue
        games.append(actions[:PREFIX_PLIES])
        if len(games) == CASE_COUNT:
            break
    if len(games) != CASE_COUNT:
        raise SystemExit(f"found {len(games)} games, expected {CASE_COUNT}")
    cases = [
        {"id": f"representative-{index + 1:02d}", "fen": ROOT_FEN, "actions": actions}
        for index, actions in enumerate(games)
    ]
    manifest = {
        "schema": "sf18-big-representative-sequences/v1",
        "source_commit": SOURCE_COMMIT,
        "source_artifact_sha256": SOURCE_SHA256,
        "prefix_plies": PREFIX_PLIES,
        "suites": {
            "representative": {
                "case_count": len(cases),
                "row_count": sum(1 + len(case["actions"]) for case in cases),
                "cases": cases,
            }
        },
    }
    with output.open("x") as target:
        json.dump(manifest, target, indent=2)
        target.write("\n")


if __name__ == "__main__":
    main()
