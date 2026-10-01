#!/usr/bin/env python3
import argparse
import datetime as dt
import hashlib
import json
import os
import pathlib
import re
import signal
import subprocess
import time

BINARY = pathlib.Path("/home/ehrli/rodent-v1.1-public-preflight-20260906/extracted/release/release-1-1/rodent_v_1.1_testers_linux_amd6")
RELEASE_DIR = BINARY.parent
NET = RELEASE_DIR / "nets/rodent_anand_512hl.bin"
PERSONALITY = RELEASE_DIR / "personalities/anand.txt"
TAG_DIR = pathlib.Path("/home/ehrli/rodent-v1.1-public-preflight-20260906/extracted/source")
MONITOR = pathlib.Path("/home/ehrli/nnue-public-toolchain-20260906/src/ngn-data-pilot/process_tree_monitor.py")

EXPECTED = {
    BINARY: "9c68d7b39dc933eff5fd2da88bd4be1b0d009f6d2add7485c121cfc0d1308531",
    NET: "5f7480b56538e9e64ee02fb5b4842bf0394ee44b707dae3a63ec7f902abd0afb",
    PERSONALITY: "b0e336bc8892043063e6b1fcc42193bbbc6cacf347e9d561d089ddcb602bf3aa",
    TAG_DIR / "uci.go": "f452492b01ac32fc0cf987383bf85b4d934bdefcc6da7f1800b3686121ce082e",
    TAG_DIR / "options.go": "d71f036cc77bb2788fe6351ebdf6d48911363a8ad7d65d20047c4f60999bc13c",
    TAG_DIR / "nnue.go": "bb6689c3465996fe982bcc77272c1033744fa53030958a4afb737523d55995bc",
    TAG_DIR / "eval.go": "2a8b8b955ad2f2d692f721eb5b39b03ed27b576970d80de1d393fee8fb7a9891",
    MONITOR: "4771585ee567615467cbf5eb3255a78dd234efe6f8fc9fe02885bb9815a7209f",
}

# Layout selection is independent of Rodent scores. It reuses the complete
# accepted Counter profile roots and frozen special fixtures, then adds four
# horizontal-mirror boundary layouts and four isolated material witnesses.
LAYOUTS = [
    ("start", "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1", "counter-fixture:start"),
    ("kiwipete", "r3k2r/p1ppqpb1/bn2pnp1/2Pp4/1p2P3/2N2N2/PPQBBPPP/R3K2R w KQkq - 37 1", "counter-fixture:kiwipete"),
    ("ep_white", "7k/8/8/3pP3/8/8/8/K7 w - d6 0 1", "counter-fixture:ep_white"),
    ("ep_black", "7k/8/8/8/3Pp3/8/8/K7 b - d3 0 1", "counter-fixture:ep_black"),
    ("promoted_mix", "7k/Q7/1Q6/8/8/8/6q1/K7 w - - 99 1", "counter-fixture:promoted"),
    ("asymmetric", "r3k2r/ppp2ppp/2n1bn2/3qp3/3P4/2N1PN2/PPP2PPP/R2Q1RK1 w kq - 0 15", "counter-fixture:asymmetric"),
    ("prefix_g1_g2_p23", "r2Qkb1r/p4ppp/2p1p3/8/8/2P3P1/P1P2P1P/R1B1K2R b KQkq - 0 1", "accepted-counter-games1-and2-ply23-identical"),
    ("prefix_g1_p48", "5R2/p1kr3p/2p3p1/P1b1pp2/8/2P3PP/2P1KP2/2B5 w - - 1 1", "accepted-counter-game1-ply48"),
    ("prefix_g2_p48", "5rk1/5pp1/R1p1p3/4b2p/P1K5/1RP1B1P1/r1P2P1P/8 w - - 4 1", "accepted-counter-game2-ply48"),
    ("prefix_g3_p23", "r2q1rk1/pbp2ppp/1p1p1n2/2nPp3/2P1P3/5N2/PPBN1PPP/R2QR1K1 b - - 4 1", "accepted-counter-game3-ply23"),
    ("prefix_g3_p48", "6k1/4qp1p/2rpnnpQ/pb2p3/4P3/1P3NNP/P4PPK/1B2R3 w - - 0 1", "accepted-counter-game3-ply48"),
    ("castles", "r3k2r/8/8/8/8/8/8/R3K2R w KQkq - 0 1", "counter-transition:castles_both_colors"),
    ("white_promotion", "k6r/6P1/8/8/8/8/8/1K6 w - - 0 1", "counter-transition:white_promotions"),
    ("black_promotion", "7k/8/8/8/8/8/1p6/R6K b - - 0 1", "counter-transition:black_promotions"),
    ("kings_only", "7k/8/8/8/8/8/8/K7 w - - 0 1", "counter-fixture:bounds"),
    ("mirror_a_d", "3k4/6p1/2n5/8/1P6/5N2/P7/3K4 w - - 0 1", "crafted-horizontal-boundary-pair-a"),
    ("mirror_a_e", "4k3/1p6/5n2/8/6P1/2N5/7P/4K3 w - - 0 1", "crafted-horizontal-boundary-pair-a"),
    ("mirror_b_d", "4k3/p7/7b/3P4/8/2N5/6P1/3K4 w - - 0 1", "crafted-horizontal-boundary-pair-b"),
    ("mirror_b_e", "3k4/7p/b7/4P3/8/5N2/1P6/4K3 w - - 0 1", "crafted-horizontal-boundary-pair-b"),
    ("white_bishop_only", "7k/8/8/8/8/8/2B5/K7 w - - 0 1", "crafted-material-quirk"),
    ("black_bishop_only", "2b4k/8/8/8/8/8/8/K7 w - - 0 1", "crafted-material-quirk"),
    ("white_rook_only", "7k/8/8/8/8/8/R7/K7 w - - 0 1", "crafted-material-quirk"),
    ("black_rook_only", "7k/8/8/8/8/8/7r/K7 w - - 0 1", "crafted-material-quirk"),
    ("material_quirk_combo", "2b4k/8/8/8/8/8/R1B4r/K7 w - - 0 1", "crafted-material-quirk"),
]
CLOCK_GRID = {"start", "kiwipete", "promoted_mix", "prefix_g3_p48", "kings_only", "material_quirk_combo"}

def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()

def trunc_div(n, d):
    return (1 if n >= 0 else -1) * (abs(n) // d)

def fields(fen):
    out = fen.split()
    if len(out) != 6:
        raise ValueError(f"bad FEN field count: {fen}")
    return out

def variant(fen, stm, clock=None):
    out = fields(fen)
    out[1] = stm
    if clock is not None:
        out[4] = str(clock)
    return " ".join(out)

def validate_fen(fen):
    placement = fields(fen)[0]
    ranks = placement.split("/")
    if len(ranks) != 8:
        raise ValueError(f"bad ranks: {fen}")
    counts = {p: 0 for p in "PNBRQKpnbrqk"}
    for rank in ranks:
        width = 0
        for c in rank:
            if c.isdigit():
                width += int(c)
            elif c in counts:
                width += 1
                counts[c] += 1
            else:
                raise ValueError(f"bad placement char {c}: {fen}")
        if width != 8:
            raise ValueError(f"bad rank width: {fen}")
    if counts["K"] != 1 or counts["k"] != 1:
        raise ValueError(f"king count: {fen}")
    return counts

def mirror_placement(placement):
    rows = []
    for rank in placement.split("/"):
        cells = []
        for c in rank:
            cells.extend([None] * int(c) if c.isdigit() else [c])
        cells.reverse()
        out = ""
        empties = 0
        for c in cells:
            if c is None:
                empties += 1
            else:
                if empties:
                    out += str(empties)
                    empties = 0
                out += c
        if empties:
            out += str(empties)
        rows.append(out)
    return "/".join(rows)

def release_static(raw, counts):
    # Exact released main.evaluateScaledNNUE material expression. It omits
    # Black bishops and counts White rooks at both 300 and 500.
    material = (100 * (counts["P"] + counts["p"])
                + 300 * (counts["N"] + counts["n"])
                + 300 * counts["B"] + 300 * counts["R"]
                + 500 * (counts["R"] + counts["r"])
                + 900 * (counts["Q"] + counts["q"]))
    return trunc_div(raw * (25000 + material), 32768)

def process_group_survivors(pgid):
    survivors = []
    proc = pathlib.Path("/proc")
    for item in proc.iterdir():
        if not item.name.isdigit():
            continue
        try:
            data = (item / "stat").read_text()
            close = data.rfind(")")
            tail = data[close + 2:].split()
            if int(tail[2]) == pgid: # pgrp is field 5; tail starts at field 3
                survivors.append(int(item.name))
        except (FileNotFoundError, ProcessLookupError, PermissionError, ValueError, IndexError):
            pass
    return sorted(survivors)

def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--output", required=True)
    args = ap.parse_args()
    out = pathlib.Path(args.output)
    out.mkdir(parents=True, exist_ok=True)
    started = dt.datetime.now(dt.timezone.utc)

    observed = {str(p): sha(p) for p in EXPECTED}
    for p, expected in EXPECTED.items():
        if observed[str(p)] != expected:
            raise RuntimeError(f"identity mismatch {p}: {observed[str(p)]}")

    by_name = {name: fen for name, fen, _ in LAYOUTS}
    assert mirror_placement(fields(by_name["mirror_a_d"])[0]) == fields(by_name["mirror_a_e"])[0]
    assert mirror_placement(fields(by_name["mirror_b_d"])[0]) == fields(by_name["mirror_b_e"])[0]

    specs = []
    layout_provenance = []
    for name, fen, origin in LAYOUTS:
        validate_fen(fen)
        layout_provenance.append({"name": name, "source_fen": fen, "origin": origin})
        clocks = [0, 50, 99] if name in CLOCK_GRID else [None]
        for clock in clocks:
            for stm in ("w", "b"):
                suffix = f"_{stm}" if clock is None else f"_{stm}_clock{clock}"
                specs.append({"name": name + suffix, "fen": variant(fen, stm, clock)})
    if len(specs) != 72 or len({x["name"] for x in specs}) != len(specs):
        raise RuntimeError(f"unexpected corpus size/IDs: {len(specs)}")
    if len({x["fen"] for x in specs}) != len(specs):
        raise RuntimeError("duplicate full FEN in corpus")
    if not all(sum(1 for x in specs if fields(x["fen"])[1] == stm) == 36 for stm in ("w", "b")):
        raise RuntimeError("STM coverage mismatch")
    for clock in (0, 50, 99):
        if sum(1 for x in specs if int(fields(x["fen"])[4]) == clock) < 12:
            raise RuntimeError(f"clock coverage too small: {clock}")

    commands = ["uci"]
    for pass_index in range(2):
        for spec in specs:
            commands += ["position fen " + spec["fen"], "nnue", "isready"]
    commands.append("quit")
    input_bytes = ("\n".join(commands) + "\n").encode()
    (out / "engine.input").write_bytes(input_bytes)
    command = [str(BINARY)]
    (out / "command.json").write_text(json.dumps({"argv": command, "cwd": str(RELEASE_DIR), "environment": {"LANG": "C", "LC_ALL": "C", "TZ": "UTC", "GOMAXPROCS": "1"}}, indent=2) + "\n")

    env = {"PATH": "/usr/bin:/bin", "LANG": "C", "LC_ALL": "C", "TZ": "UTC", "GOMAXPROCS": "1"}
    t0 = time.monotonic()
    proc = subprocess.Popen(command, cwd=RELEASE_DIR, env=env, stdin=subprocess.PIPE,
                            stdout=subprocess.PIPE, stderr=subprocess.PIPE, start_new_session=True)
    pgid = proc.pid
    timed_out = False
    try:
        stdout, stderr = proc.communicate(input=input_bytes, timeout=30)
    except subprocess.TimeoutExpired:
        timed_out = True
        os.killpg(pgid, signal.SIGTERM)
        try:
            stdout, stderr = proc.communicate(timeout=3)
        except subprocess.TimeoutExpired:
            os.killpg(pgid, signal.SIGKILL)
            stdout, stderr = proc.communicate()
    elapsed = time.monotonic() - t0
    (out / "engine.stdout").write_bytes(stdout)
    (out / "engine.stderr").write_bytes(stderr)
    survivors = process_group_survivors(pgid)
    if timed_out or proc.returncode != 0 or stderr or survivors:
        raise RuntimeError(f"bad termination timeout={timed_out} rc={proc.returncode} stderr={stderr!r} survivors={survivors}")

    text = stdout.decode("utf-8", errors="strict")
    required = [
        "id name Rodent V 1.1 AVX2",
        "option name Hash type spin default 16 min 1 max 4096",
        "option name Clear Hash type button",
        "option name UCI_LimitStrength type check default false",
        "option name UCI_Elo type spin default 3000 min 800 max 3000",
        "info string Loaded NNUE network: nets/rodent_anand_512hl.bin",
        "uciok",
    ]
    for line in required:
        if text.count(line) != 1:
            raise RuntimeError(f"missing/duplicate identity line: {line}")
    if any(bad in text.lower() for bad in ("failure:", "error", "bestmove", "info depth", "nodes:")):
        raise RuntimeError("unexpected error/search output")
    advertised = re.findall(r"(?m)^option name .+$", text)
    if advertised != required[1:5]:
        raise RuntimeError(f"unexpected advertised options: {advertised}")
    values = [int(x) for x in re.findall(r"(?m)^(-?[0-9]+)readyok$", text)]
    if len(values) != 2 * len(specs):
        raise RuntimeError(f"raw result count={len(values)} want={2*len(specs)}")
    if values[:len(specs)] != values[len(specs):]:
        raise RuntimeError("release raw outputs differed between deterministic passes")

    records = []
    for spec, raw in zip(specs, values[:len(specs)]):
        counts = validate_fen(spec["fen"])
        records.append({"name": spec["name"], "fen": spec["fen"], "raw": raw,
                        "release_static": release_static(raw, counts)})

    go = "/usr/local/go/bin/go"
    disassemblies = {
        "release-init-disassembly.txt": "main.init.3",
        "release-evaluate-scaled-disassembly.txt": "main.evaluateScaledNNUE",
        "release-get-eval-disassembly.txt": "getEval",
        "release-add-piece-disassembly.txt": "addPiece",
    }
    for filename, symbol in disassemblies.items():
        tool_env = dict(env)
        tool_env["HOME"] = str(out / "tmp")
        tool_env["GOCACHE"] = str(out / "tmp" / "go-cache")
        data = subprocess.check_output([go, "tool", "objdump", "-s", symbol, str(BINARY)], stderr=subprocess.STDOUT, env=tool_env)
        (out / filename).write_bytes(data)

    init_disasm = (out / "release-init-disassembly.txt").read_text()
    if "MOVQ $0xc0, main.singleOptionValue+24" not in init_disasm or "MOVQ $0x1, main.singleOptionValue+56" not in init_disasm:
        raise RuntimeError("release scale/mirror init witnesses missing")
    tagged_options = (TAG_DIR / "options.go").read_text()
    if 'registerSingleOption(NnueScale, "nnueScale", 400, 10, 2000' not in tagged_options:
        raise RuntimeError("tagged scale-400 witness missing")

    oracle = {
        "schema": "rodent-v1.1-anand-raw-oracle-v1",
        "classification": "exact-release full-refresh raw runtime oracle; final static independently derived",
        "release_binary_sha256": observed[str(BINARY)],
        "network_sha256": observed[str(NET)],
        "personality_sha256": observed[str(PERSONALITY)],
        "tagged_source_commit": "5689d0babebe95d87592eaaaee73ea555ef9345c",
        "identities": {
            "release_binary": {"path": str(BINARY), "sha256": observed[str(BINARY)], "bytes": BINARY.stat().st_size},
            "anand_network": {"path": str(NET), "sha256": observed[str(NET)], "bytes": NET.stat().st_size},
            "anand_personality": {"path": str(PERSONALITY), "sha256": observed[str(PERSONALITY)]},
            "tag": {"name": "Rodent_v_1_1", "commit": "5689d0babebe95d87592eaaaee73ea555ef9345c", "tree": "be0effaf52855fcbdf4a288fc1c203e1febcb5ce", "source_archive_sha256": "929f560996c3b0a8609e594923e587ab0eef61c9c013afbbd4497045113523fd"},
        },
        "raw_oracle_command": "position fen <FEN>; nnue; isready; parse ^(-?[0-9]+)readyok$",
        "raw_oracle_semantics": "side-to-move full-refresh getEval integer before material factor",
        "determinism": {"passes": 2, "matched": True, "records_per_pass": len(specs)},
        "release_static_derivation": {
            "authority": "separately preserved exact-release main.evaluateScaledNNUE disassembly",
            "arithmetic": "signed machine-int multiplication; division truncates toward zero",
            "formula": "raw*(25000 + 100*(WP+BP) + 300*(WN+BN) + 300*WB + 300*WR + 500*(WR+BR) + 900*(WQ+BQ))/32768",
            "quirk": "Black bishops omitted; White rooks counted at both 300 and 500",
        },
        "tag_release_gap": {"tagged_default_scale": 400, "release_locked_scale": 192, "release_locked_horizontal_mirroring": 1, "release_hce_weight": 0, "release_nnue_weight": 100},
        "corpus_design": {"selection": "source-unselected accepted roots/frozen fixtures plus predeclared boundary/material witnesses", "layout_count": len(LAYOUTS), "record_count": len(records), "both_stm": True, "clock_grid": [0, 50, 99], "layout_provenance": layout_provenance},
        "records": records,
    }
    (out / "oracle.json").write_text(json.dumps(oracle, indent=2) + "\n")

    receipt = {
        "schema": "rodent-v1.1-anand-oracle-run-receipt-v1",
        "state": "COMPLETE",
        "started_utc": started.isoformat(),
        "ended_utc": dt.datetime.now(dt.timezone.utc).isoformat(),
        "elapsed_seconds": elapsed,
        "binary_pid": proc.pid,
        "binary_pgid": pgid,
        "returncode": proc.returncode,
        "timed_out": timed_out,
        "surviving_processes": survivors,
        "search_or_benchmark_executed": False,
        "record_count": len(records),
        "deterministic_passes": 2,
        "identities": observed,
        "stdout_sha256": hashlib.sha256(stdout).hexdigest(),
        "stderr_sha256": hashlib.sha256(stderr).hexdigest(),
        "input_sha256": hashlib.sha256(input_bytes).hexdigest(),
    }
    (out / "driver-receipt.json").write_text(json.dumps(receipt, indent=2) + "\n")

    inventory = {}
    for path in sorted(out.iterdir()):
        if path.is_file() and path.name != "inventory.json":
            inventory[path.name] = {"sha256": sha(path), "bytes": path.stat().st_size}
    (out / "inventory.json").write_text(json.dumps({"schema": "rodent-v1.1-anand-oracle-inventory-v1", "files": inventory}, indent=2) + "\n")
    print(json.dumps({"state": "COMPLETE", "records": len(records), "passes": 2}))

if __name__ == "__main__":
    main()
