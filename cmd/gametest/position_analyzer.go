package main

import (
	"fmt"

	"github.com/ehrlich-b/ngn/engine"
)

// PositionAnalyzer provides chess position analysis capabilities
type PositionAnalyzer struct{}

// NewPositionAnalyzer creates a new position analyzer
func NewPositionAnalyzer() *PositionAnalyzer {
	return &PositionAnalyzer{}
}

// AnalyzePosition analyzes a chess position and returns game state info
func (pa *PositionAnalyzer) AnalyzePosition(fen string) (*PositionInfo, error) {
	pos, err := engine.ParseFEN(fen)
	if err != nil {
		return nil, fmt.Errorf("failed to parse FEN: %v", err)
	}

	info := &PositionInfo{
		FEN:           fen,
		IsWhiteToMove: pos.Turn() == engine.White,
	}

	// Generate all legal moves
	moves := engine.GenerateLegalMoves(pos)
	info.LegalMoveCount = len(moves)

	// Check if position is terminal
	if len(moves) == 0 {
		// No legal moves - checkmate or stalemate
		if pos.IsInCheck() {
			info.IsCheckmate = true
			if info.IsWhiteToMove {
				info.GameResult = "0-1" // Black wins
				info.ResultReason = "checkmate"
			} else {
				info.GameResult = "1-0" // White wins
				info.ResultReason = "checkmate"
			}
		} else {
			info.IsStalemate = true
			info.GameResult = "1/2-1/2"
			info.ResultReason = "stalemate"
		}
		return info, nil
	}

	// Check for insufficient material draw
	if pa.isInsufficientMaterial(pos) {
		info.IsDraw = true
		info.GameResult = "1/2-1/2"
		info.ResultReason = "insufficient_material"
		return info, nil
	}

	// Check for 50-move rule
	if pos.HalfMoveClock >= 100 { // 50 moves = 100 half-moves
		info.IsDraw = true
		info.GameResult = "1/2-1/2"
		info.ResultReason = "fifty_move_rule"
		return info, nil
	}

	// Check for threefold repetition
	if pa.isThreefoldRepetition(pos) {
		info.IsDraw = true
		info.GameResult = "1/2-1/2"
		info.ResultReason = "threefold_repetition"
		return info, nil
	}

	// Position is ongoing
	info.GameResult = "*"
	info.ResultReason = "game_in_progress"

	return info, nil
}

// PositionInfo contains analysis results for a chess position
type PositionInfo struct {
	FEN            string
	IsWhiteToMove  bool
	LegalMoveCount int
	IsCheckmate    bool
	IsStalemate    bool
	IsDraw         bool
	GameResult     string // "1-0", "0-1", "1/2-1/2", "*"
	ResultReason   string
}

// isInsufficientMaterial checks if the position has insufficient material for checkmate
func (pa *PositionAnalyzer) isInsufficientMaterial(pos *engine.Position) bool {
	// Get piece counts
	whitePawns := engine.PopCount(pos.Board.GetBitboardOf(engine.WhitePawn))
	blackPawns := engine.PopCount(pos.Board.GetBitboardOf(engine.BlackPawn))
	whiteQueens := engine.PopCount(pos.Board.GetBitboardOf(engine.WhiteQueen))
	blackQueens := engine.PopCount(pos.Board.GetBitboardOf(engine.BlackQueen))
	whiteRooks := engine.PopCount(pos.Board.GetBitboardOf(engine.WhiteRook))
	blackRooks := engine.PopCount(pos.Board.GetBitboardOf(engine.BlackRook))
	whiteBishops := engine.PopCount(pos.Board.GetBitboardOf(engine.WhiteBishop))
	blackBishops := engine.PopCount(pos.Board.GetBitboardOf(engine.BlackBishop))
	whiteKnights := engine.PopCount(pos.Board.GetBitboardOf(engine.WhiteKnight))
	blackKnights := engine.PopCount(pos.Board.GetBitboardOf(engine.BlackKnight))

	// Any pawns, queens, or rooks = sufficient material
	if whitePawns > 0 || blackPawns > 0 || whiteQueens > 0 || blackQueens > 0 || whiteRooks > 0 || blackRooks > 0 {
		return false
	}

	// Count total minor pieces
	whiteMinor := whiteBishops + whiteKnights
	blackMinor := blackBishops + blackKnights

	// King vs King
	if whiteMinor == 0 && blackMinor == 0 {
		return true
	}

	// King + minor piece vs King
	if (whiteMinor == 1 && blackMinor == 0) || (whiteMinor == 0 && blackMinor == 1) {
		return true
	}

	// King + Knight vs King + Knight
	if whiteMinor == 1 && blackMinor == 1 && whiteKnights == 1 && blackKnights == 1 {
		return true
	}

	// Other combinations have sufficient material
	return false
}

// isThreefoldRepetition checks for threefold repetition
func (pa *PositionAnalyzer) isThreefoldRepetition(pos *engine.Position) bool {
	// For now, just return false - threefold repetition tracking
	// would require game history which we don't have here
	return false
}

// MoveToAlgebraic converts a move to algebraic notation (simplified)
func (pa *PositionAnalyzer) MoveToAlgebraic(move engine.Move, pos *engine.Position) string {
	// This is a simplified version - full algebraic notation is complex
	return move.ToString()
}

// UpdatePositionWithMove applies a move to a position and returns the new FEN
func (pa *PositionAnalyzer) UpdatePositionWithMove(fen string, moveStr string) (string, error) {
	pos, err := engine.ParseFEN(fen)
	if err != nil {
		return "", fmt.Errorf("failed to parse FEN: %v", err)
	}

	move, err := engine.ParseAlgebraicMove(moveStr, pos)
	if err != nil {
		return "", fmt.Errorf("failed to parse move %s: %v", moveStr, err)
	}

	// Make the move
	_, _, _, _ = pos.MakeMove(move)

	// Return new FEN
	return engine.GenerateFEN(pos), nil
}

// ValidateMove checks if a move is legal in the given position
func (pa *PositionAnalyzer) ValidateMove(fen string, moveStr string) error {
	pos, err := engine.ParseFEN(fen)
	if err != nil {
		return fmt.Errorf("failed to parse FEN: %v", err)
	}

	move, err := engine.ParseAlgebraicMove(moveStr, pos)
	if err != nil {
		return fmt.Errorf("invalid move syntax: %v", err)
	}

	// Check if move is in legal moves list
	legalMoves := engine.GenerateLegalMoves(pos)
	for _, legalMove := range legalMoves {
		if move == legalMove {
			return nil // Move is legal
		}
	}

	return fmt.Errorf("move %s is not legal in position %s", moveStr, fen)
}

// GetPositionEvaluation gets a quick evaluation of the position
func (pa *PositionAnalyzer) GetPositionEvaluation(fen string) (int, error) {
	pos, err := engine.ParseFEN(fen)
	if err != nil {
		return 0, fmt.Errorf("failed to parse FEN: %v", err)
	}

	// Use the engine's evaluation function
	eval := engine.Evaluate(&pos.Board)
	return eval, nil
}
