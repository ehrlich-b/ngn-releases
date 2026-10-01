#!/usr/bin/env python3
# ebfprobe.py — the EBF/depth-at-fixed-nodes probe (the conthist-lane filter + the
# 3000-path falsification instrument). Committed 2026-06-10 after a host reboot
# wiped the /tmp original (instruments live in scripts/, not /tmp).
#
# Usage: python3 scripts/ebfprobe.py BASE_BIN CAND_BIN   (run x2 — output is
# deterministic, byte-identical reruns; engines run as-launched, wrap the whole
# call in `taskpolicy -b` for E-cores.)
#
# Reads: depth reached at 400K fixed nodes on 6 out-of-book positions (4 quiet/
# closed/late middlegames + 2 endgames). Deltas are what matter; matched-depth
# node counts disambiguate a coarse 0-ply delta (reduction-where-it-counts = real,
# inflation-in-sharp = reject — see TODO Lane A exec log, qmvv vs A2 precedents).
#
# STANDING BASELINE RE-MEASURED 2026-07-26 on HEAD (post T5-aspiration / corrhist family / T1b+T1e):
#   mg depths QGD 14 / closed 12 / lateMg-ph6-11 13 / najdorf 14
#   rook-eg 17, pawn-eg 32   (nodes 183331/169011/396246/390110/354408/325143)
#   implied EBF: 2.38 / 2.73 / 2.70 / 2.51 / 2.12 / 1.49 -- mg is ~2.4-2.7, NOT the ~3.0 this header
#   and memory:project_ebf_lane both claimed for six weeks. The lane was never exhausted; it was
#   delivering unmeasured. RE-READ THIS AFTER EVERY STRUCTURAL KEEP (~4 min, zero box time).
# PRIOR BASELINE (2026-06-11, post-W1 HEAD 67081e4 = futility d3->8 kept), kept for the delta:
#   mg depths QGD 12 / closed 9 / lateMg-ph6-11 11 / najdorf 9
#   rook-eg 14, pawn-eg 24
# W1 effect at 400K: QGD +2 (open positions prune deeper), closed -1, najdorf -1
# (the closed/quiet tension the killer/counter exemptions accept), eg unchanged.
# NOTE: the prior header's "fa0926d 11/11/.../rook-eg 12" was STALE/mismeasured —
# the actual pre-W1 base measured QGD 10/closed 10/lateMg 11/najdorf 10, rook-eg 14.
# CONTHIST SYSTEM CHECKPOINT (still standing): the depth-17-19 class gap (EBF
# ~2.0-2.2 at this same 400K) is INTACT — W1's QGD 10->12 is nowhere near it;
# attribution owned by the docs/11 §3 diagnostic (memory: project_ebf_lane).
import subprocess, select, sys

# Out-of-book positions: quiet middlegames + endgames (deltas are what matter).
POS = [
    ("quiet-mg-QGD",   "r1bq1rk1/pp2bppp/2n1pn2/3p4/3P4/2NBPN2/PP3PPP/R1BQ1RK1 w - - 0 1"),
    ("quiet-mg-closed","r1bq1rk1/pp2bppp/2n2n2/2pp4/3P4/2NBPN2/PP3PPP/R1BQ1RK1 w - - 0 1"),
    ("lateMg-ph6-11",  "2rq1rk1/pp1bppbp/3p1np1/8/3NP3/1BN1BP2/PPPQ2PP/2KR3R w - - 0 1"),
    ("najdorf-mg",     "r2q1rk1/1b1nbppp/p2ppn2/1p6/3NPP2/2N1B3/PPPQB1PP/R4RK1 w - - 0 1"),
    ("rook-eg-R4P",    "8/5pk1/4p1p1/4P3/5PP1/r6P/5K2/1R6 w - - 0 1"),
    ("pawn-eg",        "8/3k4/2p1p3/2P1P3/3K4/8/8/8 w - - 0 1"),
]
NODES = 400000

def run(engine, fen, nodes, diag=False):
    p = subprocess.Popen([engine], stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                         stderr=subprocess.DEVNULL, text=True, bufsize=1)
    def send(s):
        p.stdin.write(s + "\n"); p.stdin.flush()
    send("uci")
    def wait(tok, tmo=20.0):
        buf = []
        while True:
            r,_,_ = select.select([p.stdout], [], [], tmo)
            if not r:
                return buf
            line = p.stdout.readline()
            if not line:
                return buf
            buf.append(line.rstrip("\n"))
            if line.startswith(tok):
                return buf
    wait("uciok")
    send("isready"); wait("readyok")
    if diag:
        send("debug on")  # gates the sd_* attribution dump (docs/11 §3)
    send("position fen " + fen)
    send(f"go nodes {nodes}")
    depth = seldepth = lastnodes = 0
    sd = []
    # Blocking reads until bestmove: a select-per-line loop races Python's
    # internal line buffer (select sees an empty fd while buffered lines —
    # e.g. the 6-line sd_* burst — sit unread, dropping them on timeout).
    # `go nodes` always terminates with bestmove, so blocking is safe.
    while True:
        line = p.stdout.readline()
        if not line:
            break
        if line.startswith("info string sd_"):
            sd.append(line.rstrip("\n").replace("info string ", "", 1))
        elif line.startswith("info") and not line.startswith("info string") and " depth " in line:
            # progress lines only — the debug-mode `stats` line would otherwise
            # overwrite these with whole-search totals (full budget, max seldepth)
            t = line.split()
            try:
                if "depth" in t:    depth = int(t[t.index("depth")+1])
                if "seldepth" in t: seldepth = int(t[t.index("seldepth")+1])
                if "nodes" in t:    lastnodes = int(t[t.index("nodes")+1])
            except (ValueError, IndexError):
                pass
        if line.startswith("bestmove"):
            break
    send("quit")
    try: p.wait(timeout=5)
    except subprocess.TimeoutExpired: p.kill()
    return depth, seldepth, lastnodes, sd

args = [a for a in sys.argv[1:] if not a.startswith("-")]
DIAG = "-diag" in sys.argv
for i, a in enumerate(sys.argv):
    if a == "-nodes":
        NODES = int(sys.argv[i+1])
        args = [x for x in args if x != sys.argv[i+1]]
base, cand = args[0], args[1]
print(f"{'position':<18}{'base d':>7}{'cand d':>7}{'Δd':>4}{'base sd':>9}{'cand sd':>9}{'base N':>10}{'cand N':>10}")
for name, fen in POS:
    bd, bsd, bn, bsd_lines = run(base, fen, NODES, DIAG)
    cd, csd, cn, csd_lines = run(cand, fen, NODES, DIAG)
    print(f"{name:<18}{bd:>7}{cd:>7}{cd-bd:>4}{bsd:>9}{csd:>9}{bn:>10}{cn:>10}")
    if DIAG:
        for ln in bsd_lines:
            print(f"  base {ln}")
        for ln in csd_lines:
            print(f"  cand {ln}")
