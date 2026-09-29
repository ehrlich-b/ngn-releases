#!/usr/bin/env python3
# Drive a separately built, instrumented pinned Stockfish oracle.

import argparse
import hashlib
import json
import pathlib
import subprocess

PREFIX = "SF18_SMALL_ORACLE_JSON "
SOURCE_COMMIT = "cb3d4ee9b47d0c5aae855b12379378ea1439675c"
NETWORK_SHA256 = "37f18f62d772f3107e1d6aaca3898c130c3c86f2ab63e6555fbbca20635a899d"
NETWORK_SIZE = 3_519_630


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


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--stockfish", required=True)
    parser.add_argument("--network", required=True)
    parser.add_argument("--fens", required=True)
    parser.add_argument("--output", required=True)
    parser.add_argument("--transcript", required=True)
    parser.add_argument("--timeout", type=int, default=120)
    args = parser.parse_args()

    network = pathlib.Path(args.network)
    if network.stat().st_size != NETWORK_SIZE:
        raise SystemExit(f"network size {network.stat().st_size}, expected {NETWORK_SIZE}")
    actual_digest = sha256_file(network)
    if actual_digest != NETWORK_SHA256:
        raise SystemExit(f"network SHA-256 {actual_digest}, expected {NETWORK_SHA256}")

    output_path = pathlib.Path(args.output)
    transcript = pathlib.Path(args.transcript)
    stderr_path = transcript.with_suffix(transcript.suffix + ".stderr")
    for path in (output_path, transcript, stderr_path):
        if path.exists():
            raise SystemExit(f"refusing existing output {path}")

    rows = []
    commands = ["uci", f"setoption name EvalFileSmall value {network}", "isready"]
    for raw in pathlib.Path(args.fens).read_text().splitlines():
        if not raw or raw.startswith("#"):
            continue
        case_id, fen = raw.split("|", 1)
        rows.append((case_id, fen))
        commands.extend((f"position fen {fen}", "eval"))
    commands.append("quit")

    try:
        completed = subprocess.run(
            [args.stockfish],
            input="\n".join(commands) + "\n",
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            timeout=args.timeout,
            check=False,
        )
    except subprocess.TimeoutExpired as failure:
        with transcript.open("x") as target:
            target.write(captured_text(failure.stdout))
        with stderr_path.open("x") as target:
            target.write(captured_text(failure.stderr))
        raise SystemExit(f"Stockfish exceeded {args.timeout}s; partial output preserved") from failure
    with transcript.open("x") as target:
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
    if len(payloads) != len(rows):
        raise SystemExit(f"oracle rows {len(payloads)}, expected {len(rows)}")
    decoded_rows = []
    for (case_id, requested_fen), payload in zip(rows, payloads):
        decoded = json.loads(payload)
        if decoded.get("source_commit") != SOURCE_COMMIT:
            raise SystemExit(f"{case_id}: wrong source commit")
        if decoded.get("network_sha256") != actual_digest:
            raise SystemExit(f"{case_id}: wrong network identity")
        if decoded.get("fen") != requested_fen:
            raise SystemExit(f"{case_id}: FEN changed")
        decoded_rows.append(payload)
    with output_path.open("x") as output:
        for payload in decoded_rows:
            output.write(payload + "\n")


if __name__ == "__main__":
    main()
