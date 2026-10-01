#!/usr/bin/env bash
# scripts/acpl.sh — the standing EXTERNAL eval ruler: average centipawn loss vs
# Stockfish over a committed, SF-labeled corpus (acpl_corpus.labels).
#
# WHY THIS EXISTS: the self-play SPRT over-credits eval changes (transfer-mirages —
# gains that vanish on the gauntlet), and the gauntlet is too slow to run per change.
# ACPL is an EXTERNAL ruler: continuous, low-variance, and it sees quiet positional
# error the best-move/SPRT signals miss. Run it BEFORE spending an SPRT on an eval
# change — a change that RAISES ACPL (gives up more eval vs SF) is suspect no matter
# what self-play says. It FILTERS; the SPRT still confirms.
#
# The SF labels are NGN-independent ground truth, so the committed corpus stays valid
# across engine changes. Regrow/expand it with:
#   ./build/oracle -mode gencorpus -new build/ngn -out output/c.fens -opens 300
#   ./build/oracle -mode label -fens output/c.fens -out acpl_corpus.labels -sfdepth 16
#
# Usage:
#   scripts/acpl.sh                                # ACPL of build/ngn
#   scripts/acpl.sh build/ngn_new build/ngn_base   # A/B: Δ vs base (positive Δ = worse)
set -euo pipefail
cd "$(dirname "$0")/.."
NEW="${1:-build/ngn}"
BASE="${2:-}"
go build -o build/oracle ./cmd/oracle
ARGS=(-mode acpl -labels acpl_corpus.labels -new "$NEW" -nodes 200000)
[ -n "$BASE" ] && ARGS+=(-base "$BASE")
exec ./build/oracle "${ARGS[@]}"
