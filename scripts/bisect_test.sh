#!/usr/bin/env bash
# Run one ~500ms search on each test position and print bestmove + score.
# Used for bisecting regressions without running full ELO assessments.
#
# Usage: scripts/bisect_test.sh [DEPTH]
#   DEPTH defaults to 8 (~500ms-1s on middlegame positions)
set -euo pipefail

cd "$(dirname "$0")/.."

DEPTH="${1:-8}"

go build -o build/ngn main.go

POSITIONS_FILE="scripts/test_positions.txt"

echo "running depth $DEPTH on all positions..."
echo "=========================================="

while IFS='|' read -r label fen; do
  # skip comments/blanks
  case "$label" in
    \#*|"") continue ;;
  esac

  result=$({
    echo "position fen $fen"
    echo "go depth $DEPTH"
    sleep 2
    echo "quit"
  } | ./build/ngn 2>/dev/null | grep -E '^(info depth '"$DEPTH"' |bestmove )' | tail -2)

  printf '%-14s %s\n' "$label:" "$result"
done < "$POSITIONS_FILE"
