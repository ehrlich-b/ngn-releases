#!/usr/bin/env python3
"""Drive the diagnostic-only Stockfish 18 SMALL incremental oracle."""

import argparse
import hashlib
import json
import pathlib
import subprocess

PREFIX = "SF18_SMALL_INCREMENTAL_ORACLE_JSON "
SOURCE_COMMIT = "cb3d4ee9b47d0c5aae855b12379378ea1439675c"
NETWORK_SHA256 = "37f18f62d772f3107e1d6aaca3898c130c3c86f2ab63e6555fbbca20635a899d"
NETWORK_SIZE = 3_519_630
SEQUENCES_SCHEMA = "sf18-small-incremental-sequences/v2"
ROW_SCHEMA = "sf18-small-incremental-oracle/v1"


def sha256_file(path: pathlib.Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for chunk in iter(lambda: source.read(1 << 20), b""):
            digest.update(chunk)
    return digest.hexdigest()


def captured_text(value) -> str:
    if value is None:
        return ""
    if isinstance(value, bytes):
        return value.decode("utf-8", errors="replace")
    return value


def require_declared_root_fen(case_id, declared_fen, operation, state):
    if operation != "root":
        return
    actual_fen = state.get("fen")
    if actual_fen != declared_fen:
        raise SystemExit(
            f"{case_id}: root FEN {actual_fen!r}, expected declared {declared_fen!r}")


def expected_rows(case):
    actions = case["actions"]
    rows = [(0, "root", "", 0, 0)]
    logical_depth = 0
    accumulator_depth = 0
    applied = []
    for action in actions:
        logical_depth += 1
        is_null = action == "null"
        if not is_null:
            accumulator_depth += 1
        applied.append((action, is_null))
        rows.append((len(rows), "push_null" if is_null else "push", action,
                     logical_depth, accumulator_depth))
    while applied:
        action, is_null = applied.pop()
        logical_depth -= 1
        if not is_null:
            accumulator_depth -= 1
        rows.append((len(rows), "pop_null" if is_null else "pop", action,
                     logical_depth, accumulator_depth))
    return rows


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--stockfish", required=True)
    parser.add_argument("--network", required=True)
    parser.add_argument("--sequences", required=True)
    parser.add_argument("--suite", required=True, choices=("core", "growth"))
    parser.add_argument("--output", required=True)
    parser.add_argument("--transcript", required=True)
    parser.add_argument("--timeout", type=int, default=300)
    args = parser.parse_args()

    network = pathlib.Path(args.network)
    if network.stat().st_size != NETWORK_SIZE:
        raise SystemExit(f"network size {network.stat().st_size}, expected {NETWORK_SIZE}")
    actual_digest = sha256_file(network)
    if actual_digest != NETWORK_SHA256:
        raise SystemExit(f"network SHA-256 {actual_digest}, expected {NETWORK_SHA256}")

    manifest = json.loads(pathlib.Path(args.sequences).read_text())
    if manifest.get("schema") != SEQUENCES_SCHEMA:
        raise SystemExit("wrong sequences schema")
    if manifest.get("source_commit") != SOURCE_COMMIT:
        raise SystemExit("wrong sequences source commit")
    if manifest.get("network_sha256") != NETWORK_SHA256:
        raise SystemExit("wrong sequences network identity")
    suite = manifest.get("suites", {}).get(args.suite)
    if not isinstance(suite, dict) or not isinstance(suite.get("cases"), list):
        raise SystemExit(f"missing suite {args.suite}")
    cases = suite["cases"]
    identifiers = [case.get("id") for case in cases]
    if any(not isinstance(case_id, str) or not case_id for case_id in identifiers):
        raise SystemExit("case ids must be nonempty strings")
    if len(set(identifiers)) != len(identifiers):
        raise SystemExit("duplicate case id")
    for case in cases:
        if not isinstance(case.get("fen"), str) or not case["fen"]:
            raise SystemExit(f"{case.get('id')}: missing FEN")
        if not isinstance(case.get("actions"), list) or any(
                not isinstance(action, str) or not action for action in case["actions"]):
            raise SystemExit(f"{case['id']}: invalid actions")
    expected_count = sum(len(expected_rows(case)) for case in cases)
    if suite.get("case_count") != len(cases) or suite.get("row_count") != expected_count:
        raise SystemExit("declared suite counts do not match cases")

    output_path = pathlib.Path(args.output)
    transcript_path = pathlib.Path(args.transcript)
    stderr_path = transcript_path.with_suffix(transcript_path.suffix + ".stderr")
    for path in (output_path, transcript_path, stderr_path):
        if path.exists():
            raise SystemExit(f"refusing existing output {path}")

    commands = ["uci", f"setoption name EvalFileSmall value {network}", "isready"]
    for case in cases:
        commands.append(f"position fen {case['fen']}")
        command = f"sf18smallincremental {case['id']}"
        if case["actions"]:
            command += " " + " ".join(case["actions"])
        commands.append(command)
    commands.append("quit")

    try:
        completed = subprocess.run(
            [args.stockfish], input="\n".join(commands) + "\n", text=True,
            stdout=subprocess.PIPE, stderr=subprocess.PIPE,
            timeout=args.timeout, check=False,
        )
    except subprocess.TimeoutExpired as failure:
        with transcript_path.open("x") as target:
            target.write(captured_text(failure.stdout))
        with stderr_path.open("x") as target:
            target.write(captured_text(failure.stderr))
        raise SystemExit(f"Stockfish exceeded {args.timeout}s; partial output preserved") from failure

    with transcript_path.open("x") as target:
        target.write(completed.stdout)
    with stderr_path.open("x") as target:
        target.write(completed.stderr)
    if completed.returncode != 0:
        raise SystemExit(f"Stockfish exited {completed.returncode}")
    if completed.stderr:
        raise SystemExit(f"unexpected Stockfish stderr; preserved at {stderr_path}")

    protocol_lines = completed.stdout.splitlines()
    uciok = [index for index, line in enumerate(protocol_lines) if line == "uciok"]
    readyok = [index for index, line in enumerate(protocol_lines) if line == "readyok"]
    if len(uciok) != 1 or len(readyok) != 1 or uciok[0] >= readyok[0]:
        raise SystemExit(f"invalid uciok/readyok sequence: {uciok}/{readyok}")

    payloads = [line[len(PREFIX):] for line in protocol_lines if line.startswith(PREFIX)]
    if len(payloads) != expected_count:
        raise SystemExit(f"oracle rows {len(payloads)}, expected {expected_count}")

    payload_index = 0
    validated = []
    saved_states = {}
    for case in cases:
        case_id = case["id"]
        for sequence_index, operation, action, logical_depth, accumulator_depth in expected_rows(case):
            payload = payloads[payload_index]
            payload_index += 1
            decoded = json.loads(payload)
            expected_header = {
                "schema": ROW_SCHEMA, "case": case_id,
                "sequence_index": sequence_index, "operation": operation,
                "action": action, "logical_depth": logical_depth,
                "accumulator_depth": accumulator_depth,
            }
            for key, value in expected_header.items():
                if decoded.get(key) != value:
                    raise SystemExit(
                        f"{case_id} row {sequence_index}: {key}={decoded.get(key)!r}, expected {value!r}")
            state = decoded.get("state")
            if not isinstance(state, dict):
                raise SystemExit(f"{case_id} row {sequence_index}: missing state")
            if state.get("source_commit") != SOURCE_COMMIT:
                raise SystemExit(f"{case_id} row {sequence_index}: wrong source commit")
            if state.get("network_sha256") != actual_digest:
                raise SystemExit(f"{case_id} row {sequence_index}: wrong network identity")
            require_declared_root_fen(case_id, case["fen"], operation, state)
            update_kind = decoded.get("update_kind")
            if not (isinstance(update_kind, list) and len(update_kind) == 2
                    and all(kind in ("reuse", "incremental", "refresh") for kind in update_kind)):
                raise SystemExit(f"{case_id} row {sequence_index}: invalid update-kind evidence")
            if operation in ("root", "push") and update_kind == ["reuse", "reuse"]:
                raise SystemExit(f"{case_id} row {sequence_index}: no real update branch observed")
            if operation in ("push_null", "pop_null", "pop") and update_kind != ["reuse", "reuse"]:
                raise SystemExit(f"{case_id} row {sequence_index}: expected accumulator reuse")
            if operation in ("root", "push", "push_null"):
                saved_states[logical_depth] = state
            elif state != saved_states.get(logical_depth):
                raise SystemExit(
                    f"{case_id} row {sequence_index}: unwind differs from saved depth {logical_depth}")
            validated.append(payload)
        saved_states.clear()

    with output_path.open("x") as output:
        for payload in validated:
            output.write(payload + "\n")


if __name__ == "__main__":
    main()
