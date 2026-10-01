#!/usr/bin/env bash
# nodecheck.sh — the canonical node-identity gate for [NEUTRAL] Lane-B changes.
#
# Prints the cumulative search node count for a binary on three phase-diverse
# canonical positions. A node-identical change MUST reproduce the baselines
# exactly; any difference means the change is secretly [BEHAVIOUR] (→ SPRT lane).
#
# WHY THIS SCRIPT (lessons from 2026-06-01, when ad-hoc /tmp checks kept flaking):
#   - `debug on` is REQUIRED or the `info string stats` line (with the cumulative
#     info.Nodes) is never printed — easy to forget.
#   - node counts are DETERMINISTIC (core/contention-independent), so a long
#     fixed window is contention-proof: under E-core load a search just takes
#     longer to REACH the target depth; give it enough time and the count is exact.
#   - perft (`go test -short -run Perft ./engine/`) is the stronger correctness
#     gate for movegen/board changes; this is the search-tree identity gate.
#
# Usage:
#   scripts/nodecheck.sh [binary]      # default build/ngn; prints the 3 counts
# Compare candidate vs the baselines below (which are main-relative — RECOMPUTE
# and update them here + docs/10 whenever main's SEARCH behavior changes, e.g.
# after a kept [BEHAVIOUR] change like de-pad/A2/A4).
set -uo pipefail
cd "$(dirname "$0")/.."
BIN="${1:-build/ngn}"
[ -x "$BIN" ] || { echo "no binary at $BIN"; exit 1; }

# name|fen|depth|baseline   (baselines RE-LOCKED 2026-06-02 night with the wait-for-bestmove
# nodes() below. The OLD baselines (1372558/1003482/772119) were FIXED-SLEEP-WINDOW ARTIFACTS:
# the old nodes() slept depth+8s then grepped tail -1, so under E-core load it caught a MID-search
# stats line, not the depth-N-complete one — the "unstable ruler" (end d16 read 441388/772119/
# 1268761 depending on timing). go-depth searches ARE deterministic; waiting for the bestmove line
# makes the count EXACT and reproducible (verified 3x identical). EXACT-MATCH node-IDENTITY gate.
# Baselines RE-LOCKED 2026-06-03 after the night's kept [BEHAVIOUR] fixes (F1 legalTried-guard,
# C4 phase-6 king, C1 insufficient-material draw-in-search). These shift node counts legitimately;
# C1 especially (kiwipete -37% as deep lines trade into recognized draws, end +45% from corrected
# draw scores reordering the search). RE-LOCKED AGAIN 2026-06-03 LATE after the gradient tune
# (5915619) + #19 passed-pawn rank-scaling (0cf5c5b) shifted eval→cutoffs→tree.
# RE-LOCKED 2026-06-05 night to HEAD = ...+tune+#19+C1(king-dist 1695e20)+#20(passer-ext cda2033)
# +C2(rook-behind 964755e). #20 trims the 6th-rank extension (mid -37%); C2 (eval) COMPOUNDS the
# quiet rook-EG tree WITH DEPTH: end 571500(@C1+#20)->1023904 (+79% @d16, only +9% @d14 — a
# depth-compounding cost, see [[project_endgame_tree_diagnostic]] / TODO C2 caveat). Deterministic
# (wait-for-bestmove). NOTE for the speed lane: gate node-IDENTITY against THESE numbers.
# RE-LOCKED 2026-06-05 to HEAD = ...+threats (f60f9dd): the static threats eval term
# reorders the search a lot (kiwipete -46%, end -50%, mid +35% — eval butterfly, NOT a
# bug; tactical 30/30, SPRT +20.5/204g, faithfulness/symmetry green). Deterministic
# (verified reproduced exactly). Gate node-IDENTITY against THESE for the speed lane.
# RE-LOCKED 2026-06-07 to HEAD = ...+king-safety attacker-material scaling (7587eb6; SPRT
# +43.7 elo [+9,+78] over 400g, KEEP): the eval change reorders cutoffs (mid -20%, kiwipete
# +9%, end ~flat). Deterministic (wait-for-bestmove). Gate node-IDENTITY against THESE.
# RE-LOCKED 2026-06-10 to HEAD = ...+qmvv qsearch MVV/LVA selection-sort (3d86936; SPRT
# +22.6 elo over 400g at 10+0.1, KEEP): qsearch ordering shifts the tree both ways by
# position (najdorf -21%, kiwipete +55%, end -6%) — SPRT is the strength authority, these
# counts are the identity gate. The same commit's micro-opt stack (eval cache, lazy SEE,
# piece tables, stop checks) is byte-identical by construction (verified on this trio).
# RE-LOCKED 2026-06-10 AGAIN to HEAD = ...+A2a improving-gated RFP depth 3->7 (95379a8;
# SPRT +6.1 elo over 400g at 10+0.1, positive at cap, KEEP): deep RFP prunes hard at
# matched depth (kiwipete -60%, mid -44%, end -14%) — the intended width reduction.
# RE-LOCKED 2026-06-11 to HEAD = ...+O1 badcaps-below-quiets (e6b3f4b; SPRT +14.8 at cap,
# KEEP): kiwipete +7%, mid -6%, end -35% matched-depth.
# Prior: RE-LOCKED 2026-06-10 (4th) to HEAD = ...+CH-fmh follow-up history (fa0926d; SPRT +0.9
# at cap, KEEP as conthist stage 2): the 2-ply table reorders again — kiwipete -14%, mid -43%,
# end +5% vs grav baselines. Matched-depth counts are the identity gate; SPRT owns strength.
# Prior: RE-LOCKED 2026-06-10 (3rd) to HEAD = ...+CH-grav gravity-form history (b659c4e; SPRT
# +13.0 over 400g / pooled +9.4 over 556g at 10+0.1, KEEP): history values now evolve
# asymptotically with no halving cliffs, reordering the tree both ways by position
# (kiwipete +7%, mid +55%, end +45% here; the 4 EBF-probe middlegames went -2..-10%) —
# matched-depth counts are the identity gate, the real-clock SPRT is the strength ruler.
# RE-LOCKED 2026-06-11 to HEAD = ...+W1 futility d3->8 (67081e4; SPRT +6.7 elo [-13,+26]
# over 1200g at 10+0.1 conc-3, positive sign at cap, KEEP): deeper futility trims the
# middlegames (kiwipete 999540->875107 -12%, mid 579466->544730 -6%) but inflates the quiet
# rook-EG +40% (end 418446->584878) — futility is less efficient on quiet eg maneuvers
# (single-position node shape, NOT a strength signal; the 1200g SPRT nets it +6.7).
# RE-LOCKED 2026-06-13 to HEAD = ...+C1 mobility-shape un-freeze (per-count concave MG/EG
# tables; SPRT +22.5 elo [+0.6,+44.5] over 588g at 10+0.1 conc-7 cloud, runid 260612-223801,
# KEEP): the more-discriminating mobility eval prunes harder (kiwipete 875107->633299 -28%,
# mid 544730->404640 -26%) with a small quiet-eg rise (end 584878->639761 +9%).
# RE-LOCKED 2026-06-13 (2nd) to HEAD = ...+O4 killers-by-ply (key killers by PLY not remaining
# depth — a tree-wide aliasing fix, 4-engine convergent; SPRT +26.0 elo [-4.9,+57.3] over 295g
# at 10+0.1 cloud, runid 260613-081026, 2-shard partial, KEEP on positive sign): the reorder
# co-tunes with the LMR killer-reduction + futility killer-exemption, RAISING fixed-depth node
# counts — single-position node shape, NOT strength. Re-locked again for W11 (LMP re-bracket to
# CounterGo's 5+depth² curve, SPRT +1.2 elo [-19.7,+22.1] over 579g 10+0.1 cloud, runid
# 260613-201457, KEEP on positive sign): raising the LMP thresholds gave better cutoff bounds
# that propagate, LOWERING the counts (672812->634572, 521332->517737, 889431->520424, end -41%).
# Re-locked AGAIN for singular-neg-ext (W-SE2, nextDepth-- on non-PV cut-nodes where singular
# verification fails high with ttEval>=beta; SPRT +6.0 elo [-15.5,+27.5] over 582g 10+0.1 cloud,
# runid 260613-211923, KEEP): the negative extension shrinks the middlegame tree (mid -34%,
# kiwipete -1.7%) and buys endgame depth (+9%) — matches the ebfprobe mixed profile.
# Re-locked AGAIN for W11-d8-row (extend LMP to depth 8, CG range; SPRT +10.1 elo [-11.2,+31.6]
# over 583g 10+0.1 cloud, runid 260613-232400, KEEP): d8 LMP fires in high-quiet-count positions
# (kiwipete -8.5%), mid +1.5%, endgame identical (no d8 high-fanout nodes).
# RE-LOCKED 2026-06-29 to HEAD 0b008d1: the 2026-06-05 baselines were stale by ~3 weeks of kept
# behaviour changes (chiefly the 2026-06-28 P0 hard reset + mobility tables) — every binary showed a
# spurious mismatch, making the gate unreadable. New counts verified deterministic (3 identical runs)
# and node-identical across HEAD vs the sd_miss instrumentation binary.
# RE-LOCKED 2026-07-03 to HEAD 13560dc: the e16f512 baselines (270446/81006/840033) went stale over
# three engine commits, attributed by build-and-measure at each. iir3 (7394668, phase-guarded cut-node
# IIR: depth-- at cutNodes with no TT move when accPhase>=8) reshaped the FIXED-depth tree (mid d12
# 81006->168053, a ~2x tree-shape change NOT a strength regression — at fixed NODES iir3 is +1 mg ply /
# +2.0 penta real-clock); then the two T4 corrhist keeps (ffb24af pawn, 13560dc non-pawn) moved all
# three again (mid ->124266; end d16 806334->294316 as corrhist resolves the R+P endgame faster). All
# verified deterministic (reproduced exact). Gate node-IDENTITY against THESE.
# RE-LOCKED 2026-07-09 to HEAD a195717 (T4c minor-piece corrhist KEEP, +3.7 penta H1 / pLLR +2.98 / 7842g
# real-clock 10+0.1 c8): the third corrector reshapes the fixed-depth tree — kiwipete d12 371112->230036
# (-38%), mid d12 124266->181982 (+46%), end d16 294316->587557 (+100%). Non-node-identical eval-value change;
# single-position node shape is NOT a strength signal (the real-clock SPRT owns strength). Deterministic
# (reproduced exact on 2 runs). Gate node-IDENTITY against THESE.
# RE-LOCKED 2026-07-19 to HEAD 5b93966 (T5 aspiration-window modernization KEEP, +20.2 penta [+7,+33] H1 /
# pLLR peak +2.98 / 1309g real-clock 10+0.1 c8): replacing the fixed-50cp / jump-to-INFINITY / discard-after-3
# aspiration scheme with a scaled initial window (12+|score|/81), progressive ×1.5 widening and never-discard
# iterations reshapes the root re-search and the fixed-depth tree — kiwipete d12 230036->346662 (+51%),
# mid d12 181982->149587 (-18%), end d16 587557->765656 (+30%). Search-behavior change (both-directions
# RE-LOCKED 2026-08-02 by the T12 provisional keep (NMP restricted to true cut nodes, penta +2.2 [-3,+8]
# capped at 8000g): kiwipete 346662->451141 (+30.1%), mid 149587->136100 (-9.0%), end 765656->796906 (+4.1%).
# RE-LOCKED AGAIN 2026-08-02 by the T19 ttPv FULL H1 ACCEPT (penta +7.3 [-0,+15], pLLR +3.05 crossed at
# G3908 with 16 samples at/above, 3929g, measured ON TOP of T12): kiwipete 451141->295507 (-34.5%),
# mid 136100->112109 (-17.6%), end 796906->667703 (-16.2%). Note the composition SHRINKS all three while
# T12 and T19 each GREW kiwipete alone -- tree sizes do not compose additively (see
# experiments/2026-08-01-queue-composition-check.md).
# butterfly), NOT node-identical; single-position node shape is NOT a strength signal (the real-clock SPRT owns
# strength). Deterministic (reproduced exact on 2 runs). Gate node-IDENTITY against THESE.
# RE-LOCKED 2026-09-04 after the accepted correctness repair + exact pawn-push
# witness: kiwipete/mid unchanged; end 667703 -> 858052 from corrected terminal
# scores. Identity verified on both Mac and Windows; strength gate +23.5 [+1,+46].
# See experiments/2026-09-04-fast-correctness.md. These are identity, not Elo, counts.
# baselines locked @ 2026-09-04 correctness release
POSITIONS=(
  "kiwipete|r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1|12|295507"
  "mid|r1bq1rk1/pp2bppp/2n2n2/2pp4/3P4/2N1PN2/PPQ1BPPP/R1B2RK1 w - - 0 10|12|112109"
  "end|8/2p5/3p4/KP5r/1R3p1k/8/4P1P1/8 w - - 0 1|16|858052"
)

nodes() { # bin fen depth — waits for the bestmove line so the count is depth-N-complete (exact)
  local out fifo pid i
  out=$(mktemp); fifo=$(mktemp -u); mkfifo "$fifo"
  taskpolicy -b "$1" < "$fifo" > "$out" 2>/dev/null &
  pid=$!
  exec 3>"$fifo"
  printf 'uci\n' >&3; sleep 0.4; printf 'isready\n' >&3; sleep 0.2; printf 'debug on\n' >&3; sleep 0.2
  printf 'position fen %s\n' "$2" >&3
  printf 'go depth %d\n' "$3" >&3
  for i in $(seq 1 240); do grep -q '^bestmove' "$out" && break; sleep 0.5; done
  printf 'quit\n' >&3; exec 3>&-
  wait "$pid" 2>/dev/null
  grep 'info string stats' "$out" | tail -1 | sed -E 's/.* nodes ([0-9]+).*/\1/'
  rm -f "$out" "$fifo"
}

echo "node-identity check: $BIN"
for row in "${POSITIONS[@]}"; do
  IFS='|' read -r nm fen d base <<<"$row"
  n=$(nodes "$BIN" "$fen" "$d")
  printf "  %-9s d%-2s = %-10s baseline %s\n" "$nm" "$d" "${n:-FAIL}" "$base"
done
