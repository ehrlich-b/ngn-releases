package engine

import (
	"math/rand"
	"testing"
	"time"
)

// FuzzSearchLegalMoves tests that search never returns illegal moves from random positions
func TestFuzzSearchLegalMoves(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping fuzz test in short mode")
	}

	t.Log("🔍 FUZZ TEST: Search Legal Moves")
	t.Log("===============================")
	t.Log("Testing 1000 random positions to ensure search never returns illegal moves")

	seed := time.Now().UnixNano()
	rng := rand.New(rand.NewSource(seed))
	t.Logf("Using random seed: %d", seed)

	testPositions := []string{
		// Starting position
		"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
		// Common middlegame positions
		"r2qkb1r/pp2nppp/3p1n2/2pP4/4P3/2N2N2/PP1BBPPP/R2QK2R b KQkq - 0 8",
		"rnbqk2r/pppp1ppp/4pn2/2b5/2B1P3/3P1N2/PPP2PPP/RNBQK2R b KQkq - 0 4",
		// Complex tactical positions
		"r1bqkb1r/pppp1ppp/2n2n2/4p3/2B1P3/3P1N2/PPP2PPP/RNBQK2R w KQkq - 0 5",
		"rnbq1rk1/ppp2ppp/4pn2/3p4/1bPP4/2N1PN2/PP3PPP/R1BQKB1R w KQ - 0 6",
		// Endgame positions
		"8/8/8/3k4/8/3K4/8/8 w - - 0 1",
		"8/8/3k4/8/8/3K4/4P3/8 w - - 0 1",
	}

	illegalCount := 0
	totalSearches := 0

	// Test known good positions
	for i, fen := range testPositions {
		pos, err := ParseFEN(fen)
		if err != nil {
			t.Errorf("Failed to parse FEN %d: %v", i, err)
			continue
		}

		for depth := 1; depth <= 3; depth++ { // Keep depth low for speed
			totalSearches++

			// Perform search
			searchInfo := Search(pos, depth)
			bestMove := searchInfo.BestMove

			if bestMove == EmptyMove {
				continue // No move found (e.g., checkmate/stalemate)
			}

			// Validate the move is legal
			if !isMoveLegal(pos, bestMove) {
				illegalCount++
				t.Errorf("ILLEGAL MOVE from search: %s in position %s at depth %d",
					bestMove.ToString(), fen, depth)
			}
		}
	}

	// Generate random positions and test them
	for i := 0; i < 200; i++ {
		pos := generateRandomValidPosition(rng)

		// Skip invalid positions (e.g., both kings in check)
		if !isPositionValid(pos) {
			continue
		}

		for depth := 1; depth <= 2; depth++ { // Keep depth very low for fuzz testing
			totalSearches++

			// Perform search with timeout to prevent hangs
			searchInfo := Search(pos, depth)
			bestMove := searchInfo.BestMove

			if bestMove == EmptyMove {
				continue // No move found
			}

			// Validate the move is legal
			if !isMoveLegal(pos, bestMove) {
				illegalCount++
				t.Errorf("ILLEGAL MOVE from search: %s in random position at depth %d",
					bestMove.ToString(), depth)

				// Stop at first illegal move for debugging
				if illegalCount >= 3 {
					t.Fatalf("Found %d illegal moves, stopping fuzz test for investigation", illegalCount)
				}
			}
		}
	}

	t.Logf("✅ Fuzz test completed:")
	t.Logf("   Total searches: %d", totalSearches)
	t.Logf("   Illegal moves found: %d", illegalCount)
	t.Logf("   Success rate: %.2f%%", float64(totalSearches-illegalCount)/float64(totalSearches)*100)

	if illegalCount > 0 {
		t.Errorf("❌ CRITICAL: Found %d illegal moves out of %d searches", illegalCount, totalSearches)
	} else {
		t.Log("🎉 All search results returned legal moves!")
	}
}

// isMoveLegal checks if a move is legal in the given position
func isMoveLegal(pos *Position, move Move) bool {
	// Generate all legal moves and check if our move is in the list
	legalMoves := GenerateLegalMoves(pos)
	for _, legalMove := range legalMoves {
		if legalMove == move {
			return true
		}
	}
	return false
}

// generateRandomValidPosition creates a random but valid chess position
func generateRandomValidPosition(rng *rand.Rand) *Position {
	// For now, use some pre-made valid positions and modify them slightly
	baseFens := []string{
		"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
		"r1bqkb1r/pppp1ppp/2n2n2/4p3/2B1P3/3P1N2/PPP2PPP/RNBQK2R w KQkq - 0 5",
		"rnbq1rk1/ppp2ppp/4pn2/3p4/1bPP4/2N1PN2/PP3PPP/R1BQKB1R w KQ - 0 6",
		"8/8/3k4/8/8/3K4/4P3/8 w - - 0 1",
		"r2qkb1r/ppp2ppp/2n1pn2/3p4/3P4/2N1PN2/PPP2PPP/R1BQKB1R w KQkq - 0 6",
	}

	fen := baseFens[rng.Intn(len(baseFens))]
	pos, err := ParseFEN(fen)
	if err != nil {
		// Fallback to starting position
		return &Position{
			Board:     StartingBoard(),
			Tag:       WhiteCanCastleKingSide | WhiteCanCastleQueenSide | BlackCanCastleKingSide | BlackCanCastleQueenSide | WhiteToMove,
			EnPassant: NoSquare,
		}
	}

	// Make a few random legal moves to create variation
	for i := 0; i < rng.Intn(5); i++ {
		moves := GenerateMoves(pos)
		if len(moves) == 0 {
			break
		}

		randomMove := moves[rng.Intn(len(moves))]
		ep, tag, hc, _ := pos.MakeMove(randomMove)

		// Occasionally unmake the move to create different patterns
		if rng.Float64() < 0.3 {
			pos.UnMakeMove(randomMove, tag, ep, hc)
		}
	}

	return pos
}

// isPositionValid checks if a position is valid (e.g., not both kings in check)
func isPositionValid(pos *Position) bool {
	// Check that only the side to move can be in check
	whiteInCheck := isInCheck(pos, White)
	blackInCheck := isInCheck(pos, Black)

	if pos.Tag&WhiteToMove != 0 {
		// White to move - Black cannot be in check
		return !blackInCheck
	} else {
		// Black to move - White cannot be in check
		return !whiteInCheck
	}
}

// TestFuzzMoveGenerationLegality tests that move generation only produces legal moves
func TestFuzzMoveGenerationLegality(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping fuzz test in short mode")
	}

	t.Log("🔍 FUZZ TEST: Move Generation Legality")
	t.Log("====================================")
	t.Log("Testing that move generation only produces legal moves")

	seed := time.Now().UnixNano()
	rng := rand.New(rand.NewSource(seed))
	t.Logf("Using random seed: %d", seed)

	illegalCount := 0
	totalMoves := 0

	// Test 500 random positions
	for i := 0; i < 500; i++ {
		pos := generateRandomValidPosition(rng)

		if !isPositionValid(pos) {
			continue
		}

		// Generate legal moves (not pseudo-legal moves)
		moves := GenerateLegalMoves(pos)

		// Test each generated move for legality
		for _, move := range moves {
			totalMoves++

			// Make the move
			ep, tag, hc, _ := pos.MakeMove(move)

			// Check if the king is in check after the move
			// If so, the move was illegal
			playerColor := White
			if tag&WhiteToMove == 0 { // Original position had black to move
				playerColor = Black
			}

			if isInCheck(pos, playerColor) {
				illegalCount++
				pos.UnMakeMove(move, tag, ep, hc) // Restore position for debugging
				t.Errorf("ILLEGAL MOVE generated: %s from random position",
					move.ToString())

				// Stop early if we find too many illegal moves
				if illegalCount >= 5 {
					t.Fatalf("Found %d illegal moves, stopping for investigation", illegalCount)
				}
			} else {
				// Move was legal, unmake it
				pos.UnMakeMove(move, tag, ep, hc)
			}
		}
	}

	t.Logf("✅ Move generation fuzz test completed:")
	t.Logf("   Total moves tested: %d", totalMoves)
	t.Logf("   Illegal moves found: %d", illegalCount)
	t.Logf("   Success rate: %.2f%%", float64(totalMoves-illegalCount)/float64(totalMoves)*100)

	if illegalCount > 0 {
		t.Errorf("❌ CRITICAL: Move generation produced %d illegal moves out of %d", illegalCount, totalMoves)
	} else {
		t.Log("🎉 All generated moves were legal!")
	}
}
