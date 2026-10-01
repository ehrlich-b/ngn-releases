#!/bin/bash
# Analyze a position with Stockfish at depths 1-20

FEN="$1"
MAX_DEPTH="${2:-20}"

if [ -z "$FEN" ]; then
    echo "Usage: $0 '<FEN>' [max_depth]"
    exit 1
fi

# Run Stockfish once for all depths with a timeout
(
    echo "position fen $FEN"
    echo "go depth $MAX_DEPTH"
    sleep 10  # Allow time for Stockfish to complete
    echo "quit"
) | ./stockfish/stockfish-ubuntu-x86-64-avx2 2>&1 | grep "^info depth" | awk '{
    depth = $3
    # Extract score
    for (i=1; i<=NF; i++) {
        if ($i == "score") {
            score_type = $(i+1)
            score = $(i+2)
            if (score_type == "mate") {
                score_str = "mate " score
            } else {
                score_str = sprintf("%+dcp", score)
            }
        }
        if ($i == "pv") {
            bestmove = $(i+1)
        }
        if ($i == "nodes") {
            nodes = $(i+1)
        }
    }
    printf "Depth %2d: %8s | Best: %-6s | Nodes: %s\n", depth, score_str, bestmove, nodes
}'
