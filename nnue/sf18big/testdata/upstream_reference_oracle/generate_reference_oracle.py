#!/usr/bin/env python3
"""Drive the diagnostic-only pinned Stockfish 18 BIG reference oracle."""

import argparse
import hashlib
import json
import pathlib
import subprocess


PREFIX = "SF18_BIG_REFERENCE_ORACLE_JSON "
SOURCE_COMMIT = "cb3d4ee9b47d0c5aae855b12379378ea1439675c"
NETWORK_SHA256 = "c288c895ea924429ea9092e3f36b2b3c1f00f2a3a4c759ff7e57e79e3b43e4a7"
NETWORK_SIZE = 108_919_594
SEQUENCES_SHA256 = "fe843291d9dde5b52bedc6b81debeeb99017d552009d811a854603c661906246"
SEQUENCES_SCHEMA = "sf18-small-incremental-sequences/v2"
REPRESENTATIVE_SHA256 = "1f375f586e8da6c7e2528195fdf537af284abba7979165abc9c00c864e5cd23d"
REPRESENTATIVE_SCHEMA = "sf18-big-representative-sequences/v1"
ROW_SCHEMA = "sf18-big-reference-oracle/v1"


def sha256_file(path: pathlib.Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for chunk in iter(lambda: source.read(1 << 20), b""):
            digest.update(chunk)
    return digest.hexdigest()


def fail(message: str) -> None:
    raise SystemExit(message)


def require_index_pair(value, label: str, maximum: int) -> None:
    if not isinstance(value, list) or len(value) != 2:
        fail(f"{label}: expected two perspectives")
    for perspective, indices in enumerate(value):
        if not isinstance(indices, list):
            fail(f"{label}[{perspective}]: expected list")
        if any(not isinstance(index, int) or index < 0 or index >= maximum
               for index in indices):
            fail(f"{label}[{perspective}]: invalid index")


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--stockfish", required=True)
    parser.add_argument("--network", required=True)
    parser.add_argument("--sequences", required=True)
    parser.add_argument("--suite", choices=("core", "growth", "representative"), required=True)
    parser.add_argument("--output", required=True)
    parser.add_argument("--transcript", required=True)
    parser.add_argument("--timeout", type=int, default=900)
    args = parser.parse_args()

    stockfish = pathlib.Path(args.stockfish)
    network = pathlib.Path(args.network)
    sequences = pathlib.Path(args.sequences)
    if not stockfish.is_file():
        fail(f"missing Stockfish binary {stockfish}")
    if network.stat().st_size != NETWORK_SIZE:
        fail(f"network size {network.stat().st_size}, expected {NETWORK_SIZE}")
    if sha256_file(network) != NETWORK_SHA256:
        fail("wrong BIG network SHA-256")
    sequence_digest = sha256_file(sequences)

    manifest = json.loads(sequences.read_text())
    expected_identity = {
        SEQUENCES_SHA256: (SEQUENCES_SCHEMA, {"core", "growth"}),
        REPRESENTATIVE_SHA256: (REPRESENTATIVE_SCHEMA, {"representative"}),
    }.get(sequence_digest)
    if expected_identity is None:
        fail("wrong frozen sequence manifest SHA-256")
    expected_schema, allowed_suites = expected_identity
    if manifest.get("schema") != expected_schema:
        fail("wrong frozen sequence schema")
    if args.suite not in allowed_suites:
        fail(f"suite {args.suite} does not belong to this frozen manifest")
    if manifest.get("source_commit") != SOURCE_COMMIT:
        fail("wrong frozen sequence source commit")
    suite = manifest.get("suites", {}).get(args.suite)
    if not isinstance(suite, dict) or not isinstance(suite.get("cases"), list):
        fail(f"missing suite {args.suite}")
    cases = suite["cases"]
    if suite.get("case_count") != len(cases):
        fail("declared case count mismatch")
    identifiers = [case.get("id") for case in cases]
    if any(not isinstance(case_id, str) or not case_id for case_id in identifiers):
        fail("case ids must be nonempty strings")
    if len(set(identifiers)) != len(identifiers):
        fail("duplicate case id")
    for case in cases:
        if not isinstance(case.get("fen"), str) or not case["fen"]:
            fail(f"{case.get('id')}: missing FEN")
        actions = case.get("actions")
        if not isinstance(actions, list) or any(
                not isinstance(action, str) or not action for action in actions):
            fail(f"{case['id']}: invalid actions")

    output_path = pathlib.Path(args.output)
    transcript_path = pathlib.Path(args.transcript)
    stderr_path = transcript_path.with_suffix(transcript_path.suffix + ".stderr")
    for path in (output_path, transcript_path, stderr_path):
        if path.exists():
            fail(f"refusing existing output {path}")

    commands = ["uci", f"setoption name EvalFile value {network}", "isready"]
    for case in cases:
        commands.append(f"position fen {case['fen']}")
        command = f"sf18bigreference {case['id']}"
        if case["actions"]:
            command += " " + " ".join(case["actions"])
        commands.append(command)
    commands.append("quit")

    try:
        completed = subprocess.run(
            [stockfish],
            input="\n".join(commands) + "\n",
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            timeout=args.timeout,
            check=False,
        )
    except subprocess.TimeoutExpired as error:
        transcript_path.write_text(error.stdout or "")
        stderr_path.write_text(error.stderr or "")
        fail(f"Stockfish exceeded {args.timeout}s; partial output preserved")

    transcript_path.write_text(completed.stdout)
    stderr_path.write_text(completed.stderr)
    if completed.returncode != 0:
        fail(f"Stockfish exited {completed.returncode}")
    if completed.stderr:
        fail(f"unexpected Stockfish stderr; preserved at {stderr_path}")

    lines = completed.stdout.splitlines()
    uciok = [index for index, line in enumerate(lines) if line == "uciok"]
    readyok = [index for index, line in enumerate(lines) if line == "readyok"]
    if len(uciok) != 1 or len(readyok) != 1 or uciok[0] >= readyok[0]:
        fail(f"invalid uciok/readyok sequence: {uciok}/{readyok}")
    payloads = [line[len(PREFIX):] for line in lines if line.startswith(PREFIX)]
    expected_count = sum(1 + len(case["actions"]) for case in cases)
    if len(payloads) != expected_count:
        fail(f"oracle rows {len(payloads)}, expected {expected_count}")

    validated = []
    payload_index = 0
    for case in cases:
        expected_rows = [(0, "root", "")]
        expected_rows.extend(
            (index + 1, "push_null" if action == "null" else "push", action)
            for index, action in enumerate(case["actions"])
        )
        for sequence_index, operation, action in expected_rows:
            payload = payloads[payload_index]
            payload_index += 1
            row = json.loads(payload)
            expected_header = {
                "schema": ROW_SCHEMA,
                "case": case["id"],
                "sequence_index": sequence_index,
                "operation": operation,
                "action": action,
            }
            for key, expected in expected_header.items():
                if row.get(key) != expected:
                    fail(f"{case['id']} row {sequence_index}: {key}={row.get(key)!r}, expected {expected!r}")
            state = row.get("state")
            if not isinstance(state, dict):
                fail(f"{case['id']} row {sequence_index}: missing state")
            if state.get("source_commit") != SOURCE_COMMIT:
                fail(f"{case['id']} row {sequence_index}: wrong source commit")
            if state.get("network_sha256") != NETWORK_SHA256:
                fail(f"{case['id']} row {sequence_index}: wrong network identity")
            if operation == "root" and state.get("fen") != case["fen"]:
                fail(f"{case['id']}: canonical root FEN differs")
            require_index_pair(state.get("active_threats"), "active_threats", 79_856)
            transition = row.get("transition")
            if operation == "root":
                if transition is not None:
                    fail(f"{case['id']} root: transition is not null")
            else:
                if not isinstance(transition, dict):
                    fail(f"{case['id']} row {sequence_index}: missing transition")
                require_index_pair(transition.get("removed"), "removed", 79_856)
                require_index_pair(transition.get("added"), "added", 79_856)
                refresh = transition.get("requires_refresh")
                if not isinstance(refresh, list) or len(refresh) != 2 or any(
                        not isinstance(value, bool) for value in refresh):
                    fail(f"{case['id']} row {sequence_index}: invalid refresh evidence")
                for label in ("removed", "added"):
                    for indices in transition[label]:
                        if indices != sorted(indices) or len(indices) != len(set(indices)):
                            fail(f"{case['id']} row {sequence_index}: {label} is not a sorted set")
            validated.append(payload)

    with output_path.open("x") as output:
        for payload in validated:
            output.write(payload + "\n")


if __name__ == "__main__":
    main()
