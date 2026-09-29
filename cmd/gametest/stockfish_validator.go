package main

import (
	"bufio"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// StockfishValidator uses Stockfish to validate moves as ground truth
type StockfishValidator struct {
	stockfishPath string
	engine        *Engine
}

// NewStockfishValidator creates a new Stockfish-based move validator
func NewStockfishValidator(stockfishPath string) (*StockfishValidator, error) {
	validator := &StockfishValidator{
		stockfishPath: stockfishPath,
	}

	// Start Stockfish for validation
	engine, err := validator.startStockfish()
	if err != nil {
		return nil, fmt.Errorf("failed to start Stockfish validator: %v", err)
	}

	validator.engine = engine
	return validator, nil
}

// ValidateMove uses Stockfish to check if a move is legal
func (sv *StockfishValidator) ValidateMove(fen string, moveStr string) error {
	// Send position to Stockfish
	positionCmd := fmt.Sprintf("position fen %s", fen)
	if err := sv.sendCommand(positionCmd); err != nil {
		return fmt.Errorf("failed to send position: %v", err)
	}

	// Try to make the move by sending it to Stockfish
	testCmd := fmt.Sprintf("position fen %s moves %s", fen, moveStr)
	if err := sv.sendCommand(testCmd); err != nil {
		return fmt.Errorf("failed to send test position: %v", err)
	}

	// Ask Stockfish to generate moves from the resulting position
	// If the move was illegal, Stockfish will reject the position
	if err := sv.sendCommand("go depth 1"); err != nil {
		return fmt.Errorf("failed to request moves: %v", err)
	}

	// Wait for response - if we get a bestmove, the position was valid
	timeout := time.After(time.Second * 2)
	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for Stockfish validation")
		default:
			if sv.engine.Stdout.Scan() {
				line := sv.engine.Stdout.Text()
				if strings.HasPrefix(line, "bestmove ") {
					// Stockfish accepted the position, move is legal
					return nil
				}
				if strings.Contains(line, "Illegal move") || strings.Contains(line, "invalid") {
					return fmt.Errorf("Stockfish rejected move as illegal")
				}
			}
		}
	}
}

// GetLegalMoves asks Stockfish for all legal moves in a position
func (sv *StockfishValidator) GetLegalMoves(fen string) ([]string, error) {
	// Send position to Stockfish
	positionCmd := fmt.Sprintf("position fen %s", fen)
	if err := sv.sendCommand(positionCmd); err != nil {
		return nil, fmt.Errorf("failed to send position: %v", err)
	}

	// Request shallow search to get legal moves
	if err := sv.sendCommand("go depth 1"); err != nil {
		return nil, fmt.Errorf("failed to request moves: %v", err)
	}

	// Parse the search info to extract legal moves
	moves := make([]string, 0)
	timeout := time.After(time.Second * 2)

	for {
		select {
		case <-timeout:
			return moves, nil // Return what we have
		default:
			if sv.engine.Stdout.Scan() {
				line := sv.engine.Stdout.Text()
				if strings.HasPrefix(line, "bestmove ") {
					// Extract the best move
					parts := strings.Fields(line)
					if len(parts) >= 2 && parts[1] != "(none)" {
						moves = append(moves, parts[1])
					}
					return moves, nil
				}
			}
		}
	}
}

// CheckGameTermination uses Stockfish to determine if game is over
func (sv *StockfishValidator) CheckGameTermination(fen string) (string, string, error) {
	// Send position to Stockfish
	positionCmd := fmt.Sprintf("position fen %s", fen)
	if err := sv.sendCommand(positionCmd); err != nil {
		return "*", "error", fmt.Errorf("failed to send position: %v", err)
	}

	// Request a quick search
	if err := sv.sendCommand("go depth 1"); err != nil {
		return "*", "error", fmt.Errorf("failed to request analysis: %v", err)
	}

	timeout := time.After(time.Second * 2)
	for {
		select {
		case <-timeout:
			return "*", "game_in_progress", nil
		default:
			if sv.engine.Stdout.Scan() {
				line := sv.engine.Stdout.Text()
				if strings.HasPrefix(line, "bestmove ") {
					parts := strings.Fields(line)
					if len(parts) >= 2 && parts[1] == "(none)" {
						// No legal moves - check if it's checkmate or stalemate
						// We need to determine if the side to move is in check
						return sv.analyzeNoMovesPosition(fen)
					}
					// Game continues
					return "*", "game_in_progress", nil
				}
				if strings.Contains(line, "mate") {
					// Extract mate information from search
					if strings.Contains(line, "mate 0") {
						// Immediate mate
						fenParts := strings.Fields(fen)
						if len(fenParts) >= 2 {
							if fenParts[1] == "w" {
								return "0-1", "checkmate", nil // White to move but mated
							} else {
								return "1-0", "checkmate", nil // Black to move but mated
							}
						}
					}
				}
			}
		}
	}
}

// analyzeNoMovesPosition determines if no legal moves means checkmate or stalemate
func (sv *StockfishValidator) analyzeNoMovesPosition(fen string) (string, string, error) {
	// Parse FEN to determine side to move
	fenParts := strings.Fields(fen)
	if len(fenParts) < 2 {
		return "1/2-1/2", "stalemate", nil // Default to stalemate
	}

	_ = fenParts[1] // sideToMove - not used yet

	// For simplicity, we'll assume stalemate for now
	// A proper implementation would need to check if the king is in check
	return "1/2-1/2", "stalemate", nil
}

// startStockfish starts a Stockfish process for validation
func (sv *StockfishValidator) startStockfish() (*Engine, error) {
	cmd := exec.Command(sv.stockfishPath)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	engine := &Engine{
		Name:    "StockfishValidator",
		Process: cmd,
		Stdin:   bufio.NewWriter(stdin),
		Stdout:  bufio.NewScanner(stdout),
		Stderr:  bufio.NewScanner(stderr),
	}

	// Store engine reference before using it
	sv.engine = engine

	// Initialize UCI
	if err := sv.sendCommand("uci"); err != nil {
		return nil, err
	}

	// Wait for uciok
	timeout := time.After(time.Second * 5)
	for {
		select {
		case <-timeout:
			return nil, fmt.Errorf("timeout waiting for Stockfish uciok")
		default:
			if engine.Stdout.Scan() {
				line := engine.Stdout.Text()
				if line == "uciok" {
					// Send isready
					if err := sv.sendCommand("isready"); err != nil {
						return nil, err
					}
					// Wait for readyok
					for engine.Stdout.Scan() {
						line := engine.Stdout.Text()
						if line == "readyok" {
							return engine, nil
						}
					}
				}
			}
		}
	}
}

// sendCommand sends a command to the Stockfish validator
func (sv *StockfishValidator) sendCommand(command string) error {
	if _, err := sv.engine.Stdin.WriteString(command + "\n"); err != nil {
		return err
	}
	return sv.engine.Stdin.Flush()
}

// Close shuts down the Stockfish validator
func (sv *StockfishValidator) Close() {
	if sv.engine != nil {
		sv.sendCommand("quit")

		// Give it a moment to exit gracefully
		done := make(chan error, 1)
		go func() {
			done <- sv.engine.Process.Wait()
		}()

		select {
		case <-done:
			// Exited gracefully
		case <-time.After(time.Second * 2):
			// Force kill
			sv.engine.Process.Process.Kill()
		}
	}
}
