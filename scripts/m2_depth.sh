#!/usr/bin/env bash
# m2_depth.sh — the matched-TIME depth-deficit instrument (the M2 diagnosis).
#
# WHY: h2h.sh fixes DEPTH and compares nodes/nps (the per-node x tree-size
# decomposition). This is the dual: fix TIME and compare DEPTH REACHED. That is
# the strength-relevant number — at a real clock the engine that searches deeper
# (per ply of equal eval) wins. We split middlegame vs endgame because NGN's
# tree blows up 2.5-5x in endgames, so the depth deficit should be WORSE there.
#
# METHOD: `go infinite` + sleep T + `stop`, read the deepest `info depth` line.
# Both NGN and Blunder 7.4.0 honor UCI `stop` (5.0.0/6.1.0 do NOT — do not use
# them here). E-cores via taskpolicy -b. Depth is a coarse integer (~+-1 ply
# noise from the stop landing mid-iteration); re-run for a second opinion. Do
# NOT run under other E-core load (it steals time and shrinks the depths).
#
# Usage: scripts/m2_depth.sh [ms] [ngn_binary] [blunder_binary]
#   defaults: 1000ms  build/ngn  opponents/blunder_740
set -uo pipefail
cd "$(dirname "$0")/.."
MS="${1:-1000}"
NGN="${2:-build/ngn}"
BLU="${3:-opponents/blunder_740}"
[ -x "$NGN" ] || { echo "no NGN binary at $NGN (run: make build)"; exit 1; }
[ -x "$BLU" ] || { echo "no Blunder binary at $BLU (run: make build-anchors)"; exit 1; }
SECS=$(awk "BEGIN{printf \"%.3f\", $MS/1000}")

# name|fen|phase  — same phase-diverse set as h2h.sh (mg vs eg is the split).
POSITIONS=(
  "mid       |r1bq1rk1/pp2bppp/2n2n2/2pp4/3P4/2N1PN2/PPQ1BPPP/R1B2RK1 w - - 0 10|mg"
  "mid2      |r1bqk2r/pp2bppp/2n1pn2/2pp4/3P4/2NBPN2/PPP2PPP/R1BQK2R w KQkq - 0 7|mg"
  "kiwipete  |r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1|mg"
  "pp-endgame|8/2p5/3p4/KP5r/1R3p1k/8/4P1P1/8 w - - 0 1|eg"
  "quiet-end |r7/5pk1/6p1/8/8/6P1/5PK1/R7 w - - 0 1|eg"
)

# drive one fixed-WALL-TIME search on E-cores, print the engine's info lines
send(){ # bin fen
  { printf 'uci\n'; sleep 2; printf 'isready\n'; sleep 1; printf 'position fen %s\n' "$2";
    sleep 0.3; printf 'go infinite\n'; sleep "$SECS"; printf 'stop\n'; sleep 0.6; printf 'quit\n'; } \
  | taskpolicy -b "$1" 2>/dev/null | grep '^info depth'
}
# deepest reached: last info-depth line's depth (the comparable metric). NODES are
# deliberately NOT reported here — Blunder resets node counters per iteration so its
# last line undercounts (h2h.sh sums them); this is a DEPTH instrument, see h2h.sh for nodes.
red(){ echo "$1" | tail -1 | awk '{for(i=1;i<=NF;i++){if($i=="depth")d=$(i+1)}; printf "%d", d}'; }

printf '%s\n' "===== M2 depth-deficit @ ${MS}ms — NGN ($NGN) vs Blunder ($BLU), E-cores ====="
printf '%-10s %-3s | %-9s | %-9s | %s\n' "pos" "ph" "NGN depth" "BLU depth" "ply deficit (BLU-NGN)"
for row in "${POSITIONS[@]}"; do
  IFS='|' read -r nm fen ph <<<"$row"
  nm="${nm// /}"; fen="${fen#"${fen%%[![:space:]]*}"}"
  nd=$(red "$(send "$NGN" "$fen")")
  bd=$(red "$(send "$BLU" "$fen")")
  awk -v nm="$nm" -v ph="$ph" -v nd="$nd" -v bd="$bd" 'BEGIN{
    printf "%-10s %-3s | d%-7d | d%-7d | %+d ply\n", nm, ph, nd, bd, bd-nd }'
done
echo "----"
echo "negative deficit = NGN deeper; positive = Blunder deeper. Expect the eg rows worse for NGN."
