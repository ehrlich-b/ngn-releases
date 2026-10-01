#!/usr/bin/env bash
# h2h.sh — the canonical NGN-vs-Blunder per-node decomposition instrument.
#
# WHY THIS EXISTS: a scalar "NPS" is meaningless (NGN's own NPS ranges ~307K
# middlegame to ~860K endgame on E-cores, ~1M on P-cores). This tool measures
# the gap the only way that doesn't oscillate: fixed-depth on identical
# positions, both engines on E-cores, decomposed into the two independent
# factors that multiply to the whole gap:
#
#     time-to-depth  =  per-node-speed  x  tree-size
#
#   per-node speed (nps ratio): raw "how fast is each node". Engineering.
#   tree size (nodes ratio):    "how good is pruning/ordering". Search quality.
#
# NODE-COUNTING: both engines count qsearch nodes (NGN search.go:1779-80;
# Blunder search.go:307), so nps is comparable. CAVEAT: NGN reports CUMULATIVE
# nodes/time; Blunder RESETS its counters per iteration (search.go:91), so we
# SUM Blunder's per-depth info lines to recover cumulative. Blunder also needs a
# paced UCI handshake (it ignores fire-everything-at-once), hence the sleeps.
#
# Usage: scripts/h2h.sh [ngn_binary] [blunder_binary]
#   defaults: build/ngn  opponents/blunder_740   (run `make build-anchors` first)
set -uo pipefail
cd "$(dirname "$0")/.."
NGN="${1:-build/ngn}"
BLU="${2:-opponents/blunder_740}"
[ -x "$NGN" ] || { echo "no NGN binary at $NGN (run: make build)"; exit 1; }
[ -x "$BLU" ] || { echo "no Blunder binary at $BLU (run: make build-anchors)"; exit 1; }

# name|fen|depth  — phase-diverse; depths chosen so each finishes in seconds.
POSITIONS=(
  "mid       |r1bq1rk1/pp2bppp/2n2n2/2pp4/3P4/2N1PN2/PPQ1BPPP/R1B2RK1 w - - 0 10|12"
  "mid2      |r1bqk2r/pp2bppp/2n1pn2/2pp4/3P4/2NBPN2/PPP2PPP/R1BQK2R w KQkq - 0 7|12"
  "kiwipete  |r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1|12"
  "pp-endgame|8/2p5/3p4/KP5r/1R3p1k/8/4P1P1/8 w - - 0 1|16"
  "quiet-end |r7/5pk1/6p1/8/8/6P1/5PK1/R7 w - - 0 1|18"
)

# drive one fixed-depth search on E-cores, print the engine's raw info lines
send(){ # bin fen depth wait
  { printf 'uci\n'; sleep 2; printf 'isready\n'; sleep 1; printf 'position fen %s\n' "$2";
    sleep 0.3; printf 'go depth %s\n' "$3"; sleep "$4"; printf 'quit\n'; } \
  | taskpolicy -b "$1" 2>/dev/null | grep '^info depth'
}
ngn_red(){ echo "$1" | tail -1 | awk '{for(i=1;i<=NF;i++){if($i=="nodes")n=$(i+1);if($i=="time")t=$(i+1);if($i=="nps")p=$(i+1)}; printf "%d %d %d",n,t,p}'; }
blu_red(){ echo "$1"  | awk '{for(i=1;i<=NF;i++){if($i=="nodes")n=$(i+1);if($i=="time")t=$(i+1)}; sn+=n; st+=t} END{printf "%d %d %d",sn,st,(st>0?sn*1000/st:0)}'; }

printf '%s\n' "===== NGN ($NGN) vs Blunder ($BLU) — E-cores, fixed depth ====="
printf '%-10s %-3s | %-26s | %-26s | %s\n' "pos" "d" "NGN (cumulative)" "BLU (summed)" "per-node x tree = to-depth"
for row in "${POSITIONS[@]}"; do
  IFS='|' read -r nm fen d <<<"$row"
  nm="${nm// /}"; fen="${fen#"${fen%%[![:space:]]*}"}"
  no=$(ngn_red "$(send "$NGN" "$fen" "$d" 12)")
  bo=$(blu_red "$(send "$BLU" "$fen" "$d" 12)")
  read -r nn nt np <<<"$no"; read -r bn bt bp <<<"$bo"
  awk -v nm="$nm" -v d="$d" -v nn="$nn" -v nt="$nt" -v np="$np" -v bn="$bn" -v bt="$bt" -v bp="$bp" 'BEGIN{
    pn=(np>0&&bp>0)?bp/np:0; tr=(bn>0)?nn/bn:0; td=(bt>0)?nt/bt:0;
    printf "%-10s d%-2s | %8dn %5dms %7dnps | %8dn %5dms %7dnps | %.2fx x %.2fx = %.2fx\n", nm, d, nn,nt,np, bn,bt,bp, pn, tr, td }'
done
echo "----"
echo "per-node ~2x flat = engineering tax; tree ~1x middlegame but 2.5-5x endgame = pruning gap."
