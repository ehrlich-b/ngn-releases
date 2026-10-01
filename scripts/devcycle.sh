#!/usr/bin/env bash
# Fast dev cycle: rebuild + smoke-test the engine.
# Usage:
#   scripts/devcycle.sh build                    # build only
#   scripts/devcycle.sh test                     # build + quick unit tests
#   scripts/devcycle.sh test-all                 # build + full unit tests
#   scripts/devcycle.sh depth NN                 # build + search middlegame to depth NN (default 10)
#   scripts/devcycle.sh search LABEL NN          # build + search named position to depth NN
#   scripts/devcycle.sh nodes LABEL NN           # report node counts and NPS for a position
#   scripts/devcycle.sh elo1                     # build + level-1 ELO assessment vs SF 1320 (~1 min)
#   scripts/devcycle.sh elo1-at ELO              # build + level-1 vs arbitrary Stockfish ELO
#   scripts/devcycle.sh elo5                     # build + level-5 ELO assessment vs SF 1320 (~5 min)
#   scripts/devcycle.sh elo5-at ELO              # build + level-5 vs arbitrary Stockfish ELO
#   scripts/devcycle.sh all                      # build + depth 10 sanity on middlegame
#   scripts/devcycle.sh list-positions           # list test positions by label
set -euo pipefail

cd "$(dirname "$0")/.."

POSITIONS_FILE="scripts/test_positions.txt"

build() {
  go build -o build/ngn main.go
  echo "built build/ngn"
}

# Look up a FEN by label from the positions file.
# Usage: fen_for LABEL
fen_for() {
  local label="$1"
  local line
  line=$(grep -E "^${label}\\|" "$POSITIONS_FILE" || true)
  if [ -z "$line" ]; then
    echo "unknown position label: $label" >&2
    echo "available labels:" >&2
    grep -v '^#' "$POSITIONS_FILE" | cut -d'|' -f1 >&2
    exit 1
  fi
  echo "${line#*|}"
}

# Run a UCI search on a FEN to a given depth. Strips noisy info strings.
run_search() {
  local fen="$1"
  local depth="$2"
  local sleep_s="${3:-30}"
  {
    echo "position fen $fen"
    echo "go depth $depth"
    sleep "$sleep_s"
    echo "quit"
  } | ./build/ngn | grep -v '^info string' | grep -v '^$' || true
}

case "${1:-all}" in
  build)
    build
    ;;
  test)
    build
    go test ./engine/ -run "TestSearchNodesPerSecond|TestMemoryStressSearch|TestIterativeDeepening" -timeout 60s
    ;;
  test-all)
    build
    go test ./engine/ -timeout 180s
    ;;
  depth)
    build
    DEPTH="${2:-10}"
    run_search "$(fen_for middlegame)" "$DEPTH" 30
    ;;
  search)
    build
    LABEL="${2:-middlegame}"
    DEPTH="${3:-10}"
    SLEEP="${4:-30}"
    echo "position: $LABEL depth: $DEPTH"
    run_search "$(fen_for "$LABEL")" "$DEPTH" "$SLEEP"
    ;;
  nodes)
    build
    LABEL="${2:-middlegame}"
    DEPTH="${3:-10}"
    echo "nodes for $LABEL to depth $DEPTH"
    run_search "$(fen_for "$LABEL")" "$DEPTH" 30 | awk '/^info depth/ {print $0}; /^bestmove/ {print $0}'
    ;;
  list-positions)
    grep -v '^#' "$POSITIONS_FILE" | cut -d'|' -f1
    ;;
  elo1)
    build
    ./build/elo-assess -level 1 -elo 1320
    ;;
  elo1-at)
    ELO="${2:-1320}"
    build
    ./build/elo-assess -level 1 -elo "$ELO"
    ;;
  elo5)
    build
    ./build/elo-assess -level 5 -elo 1320
    ;;
  elo5-at)
    ELO="${2:-1320}"
    build
    ./build/elo-assess -level 5 -elo "$ELO"
    ;;
  all)
    build
    run_search "$(fen_for middlegame)" 10 20
    ;;
  *)
    echo "unknown subcommand: $1"
    exit 1
    ;;
esac
