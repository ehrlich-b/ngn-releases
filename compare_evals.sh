#!/bin/bash
# Compare NGN and Stockfish evaluations on test positions

POSITIONS=(
    # Starting position
    "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1|Starting position"

    # Move 37 blunder position (known problematic)
    "r3kn1Q/1p2b3/p1n1b3/2ppq3/8/3B4/PPPP1PPP/R1B3K1 w q - 0 1|Move 37 blunder"

    # After e4
    "rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq e3 0 1|After e4"

    # Simple endgame: K+Q vs K
    "8/8/8/4k3/8/8/4K3/4Q3 w - - 0 1|K+Q vs K"

    # Passed pawn endgame
    "8/4k3/8/3P4/8/8/4K3/8 w - - 0 1|Passed pawn endgame"

    # Complex middlegame
    "r1bqkb1r/pppp1ppp/2n2n2/1B2p3/4P3/5N2/PPPP1PPP/RNBQK2R w KQkq - 4 4|Italian Game middlegame"
)

echo "Comparing NGN vs Stockfish evaluations"
echo "======================================="
echo ""

for entry in "${POSITIONS[@]}"; do
    IFS='|' read -r fen description <<< "$entry"

    echo "Position: $description"
    echo "FEN: $fen"

    # Get NGN evaluation (static)
    ngn_eval=$(go run cmd/eval_breakdown/main.go -fen "$fen" 2>/dev/null | grep "Static Evaluation:" | grep -oP '\K[+-]?\d+' || echo "ERROR")

    # Get Stockfish evaluation (depth 15 for stability)
    sf_output=$(
        echo "position fen $fen"
        echo "go depth 15"
        sleep 5
        echo "quit"
    ) | ./stockfish/stockfish-ubuntu-x86-64-avx2 2>&1

    sf_line=$(echo "$sf_output" | grep "^info depth 15" | tail -1)

    if echo "$sf_line" | grep -q "score mate"; then
        sf_mate=$(echo "$sf_line" | grep -oP 'score mate \K[+-]?\d+')
        sf_eval="mate $sf_mate"
    elif echo "$sf_line" | grep -q "score cp"; then
        sf_eval=$(echo "$sf_line" | grep -oP 'score cp \K[+-]?\d+')
    else
        sf_eval="N/A"
    fi

    echo "  NGN:       ${ngn_eval}cp"
    if [[ "$sf_eval" == mate* ]]; then
        echo "  Stockfish: $sf_eval"
    else
        echo "  Stockfish: ${sf_eval}cp"
    fi

    if [[ "$sf_eval" != mate* ]] && [[ "$sf_eval" != "N/A" ]] && [ "$ngn_eval" != "ERROR" ]; then
        diff=$((ngn_eval - sf_eval))
        echo "  Difference: ${diff}cp (NGN - SF)"
    fi

    echo ""
done
