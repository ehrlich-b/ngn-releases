package engine

import "fmt"

// EmbeddedBook provides a simple built-in opening book
// This is used when no external book file is found

// Common opening moves with weights (higher = better)
var embeddedBookMoves = map[string][]struct {
	Move   string
	Weight int
}{
	// Starting position
	"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq -": {
		{"e2e4", 100}, // King's pawn
		{"d2d4", 95},  // Queen's pawn
		{"c2c4", 50},  // English
		{"g1f3", 45},  // Reti
	},

	// After 1.e4
	"rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq -": {
		{"e7e5", 100}, // Open game
		{"c7c5", 90},  // Sicilian
		{"e7e6", 70},  // French
		{"c7c6", 60},  // Caro-Kann
		{"d7d5", 40},  // Scandinavian
	},

	// After 1.d4
	"rnbqkbnr/pppppppp/8/8/3P4/8/PPP1PPPP/RNBQKBNR b KQkq -": {
		{"d7d5", 100}, // Closed game
		{"g8f6", 90},  // Indian defenses
		{"e7e6", 50},  // Various
		{"f7f5", 30},  // Dutch
	},

	// After 1.e4 e5
	"rnbqkbnr/pppp1ppp/8/4p3/4P3/8/PPPP1PPP/RNBQKBNR w KQkq -": {
		{"g1f3", 100}, // King's Knight
		{"f1c4", 60},  // Bishop's Opening
		{"b1c3", 40},  // Vienna
	},

	// After 1.e4 e5 2.Nf3
	"rnbqkbnr/pppp1ppp/8/4p3/4P3/5N2/PPPP1PPP/RNBQKB1R b KQkq -": {
		{"b8c6", 100}, // Normal
		{"g8f6", 60},  // Petrov
		{"d7d6", 30},  // Philidor
	},

	// After 1.e4 e5 2.Nf3 Nc6
	"r1bqkbnr/pppp1ppp/2n5/4p3/4P3/5N2/PPPP1PPP/RNBQKB1R w KQkq -": {
		{"f1b5", 100}, // Ruy Lopez
		{"f1c4", 80},  // Italian
		{"d2d4", 50},  // Scotch
		{"b1c3", 40},  // Four Knights
	},

	// After 1.e4 c5 (Sicilian)
	"rnbqkbnr/pp1ppppp/8/2p5/4P3/8/PPPP1PPP/RNBQKBNR w KQkq -": {
		{"g1f3", 100}, // Open Sicilian
		{"b1c3", 60},  // Closed Sicilian
		{"c2c3", 40},  // Alapin
	},

	// After 1.d4 d5
	"rnbqkbnr/ppp1pppp/8/3p4/3P4/8/PPP1PPPP/RNBQKBNR w KQkq -": {
		{"c2c4", 100}, // Queen's Gambit
		{"g1f3", 60},  // Various
		{"b1c3", 40},  // Various
	},

	// After 1.d4 Nf6
	"rnbqkb1r/pppppppp/5n2/8/3P4/8/PPP1PPPP/RNBQKBNR w KQkq -": {
		{"c2c4", 100}, // Main line
		{"g1f3", 70},  // Various
		{"c1g5", 30},  // Trompowsky
	},

	// After 1.d4 Nf6 2.c4
	"rnbqkb1r/pppppppp/5n2/8/2PP4/8/PP2PPPP/RNBQKBNR b KQkq -": {
		{"e7e6", 100}, // QID/Nimzo
		{"g7g6", 80},  // King's Indian
		{"c7c5", 60},  // Benoni
		{"e7e5", 40},  // Budapest
	},

	// After 1.d4 Nf6 2.c4 e6
	"rnbqkb1r/pppp1ppp/4pn2/8/2PP4/8/PP2PPPP/RNBQKBNR w KQkq -": {
		{"b1c3", 100}, // Main
		{"g1f3", 80},  // Various
		{"g2g3", 40},  // Catalan
	},

	// After 1.d4 Nf6 2.c4 g6
	"rnbqkb1r/pppppp1p/5np1/8/2PP4/8/PP2PPPP/RNBQKBNR w KQkq -": {
		{"b1c3", 100}, // King's Indian main
		{"g1f3", 70},  // Various
	},

	// Ruy Lopez main line
	"r1bqkbnr/pppp1ppp/2n5/1B2p3/4P3/5N2/PPPP1PPP/RNBQK2R b KQkq -": {
		{"a7a6", 100}, // Morphy Defense
		{"g8f6", 60},  // Berlin
		{"f7f5", 20},  // Schliemann
	},

	// Italian Game
	"r1bqkbnr/pppp1ppp/2n5/4p3/2B1P3/5N2/PPPP1PPP/RNBQK2R b KQkq -": {
		{"f8c5", 100}, // Giuoco Piano
		{"g8f6", 80},  // Two Knights
	},
}

// EmbeddedOpeningBook provides opening book functionality without external files
type EmbeddedOpeningBook struct{}

// ProbeEmbeddedBook looks up a position in the embedded book
func ProbeEmbeddedBook(pos *Position) (Move, bool) {
	// Generate FEN without move clocks for lookup
	fen := GenerateFEN(pos)
	// Strip the move clocks (last two parts)
	parts := splitFEN(fen)
	if len(parts) < 4 {
		return EmptyMove, false
	}
	// Normalize en passant: use "-" if there's no actual capturable en passant
	epPart := parts[3]
	if epPart != "-" && !canCaptureEnPassant(pos) {
		epPart = "-"
	}
	key := parts[0] + " " + parts[1] + " " + parts[2] + " " + epPart

	moves, exists := embeddedBookMoves[key]
	if !exists || len(moves) == 0 {
		return EmptyMove, false
	}

	// Pick highest weighted move
	bestMove := moves[0]
	for _, m := range moves[1:] {
		if m.Weight > bestMove.Weight {
			bestMove = m
		}
	}

	// Parse the move string
	move, err := ParseUCIMove(pos, bestMove.Move)
	if err != nil {
		return EmptyMove, false
	}

	return move, true
}

func splitFEN(fen string) []string {
	var parts []string
	current := ""
	for _, c := range fen {
		if c == ' ' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

// ParseUCIMove resolves a UCI move against the legal move list. Callers include
// the external-engine match harness, so decoding coordinates alone is not
// sufficient: an illegal move must not be applied to the authoritative board.
func ParseUCIMove(pos *Position, uciMove string) (Move, error) {
	if len(uciMove) != 4 && len(uciMove) != 5 {
		return EmptyMove, fmt.Errorf("invalid UCI move: %s", uciMove)
	}
	for _, move := range GenerateLegalMoves(pos) {
		if move.ToString() == uciMove {
			return move, nil
		}
	}
	return EmptyMove, fmt.Errorf("illegal UCI move: %s", uciMove)
}
