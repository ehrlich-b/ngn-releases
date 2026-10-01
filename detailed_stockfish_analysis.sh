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
go depth 18
quit
EOF
    
    echo "Stockfish analysis:"
    timeout 30 $STOCKFISH < temp_analysis.txt | grep -A5 -B5 "bestmove" | tail -10
    echo ""
    rm -f temp_analysis.txt
}

echo "Stockfish Deep Analysis of Critical Game Positions"
echo "================================================="

# Position 1: After 13...d5 - Critical central break
analyze_position "r1bqr1k1/1p2bppp/2n1pn2/p2p4/3P4/2N1BN2/PPP2PPP/R2QR1K1 w - d6 0 14" \
                "After 13...d5 - Central break evaluation" \
                "+0.72 (NGN thinks White better)"

# Position 3: After 18...Bc5 - Active pieces vs material
analyze_position "r2qr1k1/1p3ppp/4pn2/p1b5/3p1b2/2N5/PPP2PPP/R2QR1K1 w - - 2 19" \
                "After 18...Bc5 - Activity vs material" \
                "+0.12 (NGN sees roughly equal)"

# Position 5: Critical mate position
analyze_position "8/1p3p1p/5p2/p1b5/3P4/2PB1b1P/P1P1r3/3R2K1 w - - 1 29" \
                "After 28...Bh3 - MATE POSITION" \
                "-2.97 (NGN missing mate in 7)"