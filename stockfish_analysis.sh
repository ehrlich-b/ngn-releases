#!/bin/bash

STOCKFISH="./stockfish/stockfish-ubuntu-x86-64-avx2"

analyze_position() {
    local fen="$1"
    local desc="$2"
    local ngn_eval="$3"
    
    echo "=== $desc ==="
    echo "FEN: $fen"
    echo "NGN (in-game): $ngn_eval"
    
    # Create temp file with commands
    cat > temp_analysis.txt << EOF
position fen $fen
go depth 15
quit
EOF
    
    echo "Stockfish analysis:"
    $STOCKFISH < temp_analysis.txt | grep -E "(bestmove|score|depth)"
    echo ""
    rm temp_analysis.txt
}

echo "Stockfish Analysis of Critical Game Positions"
echo "============================================="

# Position 1: After 13...d5
analyze_position "r1bqr1k1/1p2bppp/2n1pn2/p2p4/3P4/2N1BN2/PPP2PPP/R2QR1K1 w - d6 0 14" \
                "After 13...d5 - Black's central break" \
                "+0.72"

# Position 2: After 16...cxd4  
analyze_position "r2qr1k1/1p2bppp/4pn2/p7/3p4/2N5/PPP2PPP/R2QR1K1 w - - 0 17" \
                "After 16...cxd4 - Black has compensation" \
                "+0.33"

# Position 3: After 18...Bc5
analyze_position "r2qr1k1/1p3ppp/4pn2/p1b5/3p1b2/2N5/PPP2PPP/R2QR1K1 w - - 2 19" \
                "After 18...Bc5 - Black's active pieces" \
                "+0.12"

# Position 4: After 23...Nd5
analyze_position "2r5/1p3ppp/4p3/p1bn4/1b6/2P5/P1P2PPP/1NR3K1 w - - 4 24" \
                "After 23...Nd5 - Black dominates center" \
                "-1.10"

# Position 5: After 28...Bh3 (Mate in 7)
analyze_position "8/1p3p1p/5p2/p1b5/3P4/2PB1b1P/P1P1r3/3R2K1 w - - 1 29" \
                "After 28...Bh3 - Should be mate in 7" \
                "-2.97"