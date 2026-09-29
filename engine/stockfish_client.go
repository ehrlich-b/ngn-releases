package engine

import (
	"bufio"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// StockfishClient manages communication with Stockfish engine
type StockfishClient struct {
	cmd    *exec.Cmd
	stdin  *bufio.Writer
	stdout *bufio.Scanner
}

// NewStockfishClient creates a new Stockfish client
func NewStockfishClient(stockfishPath string) (*StockfishClient, error) {
	return NewStockfishClientArgs(stockfishPath)
}

// NewStockfishClientArgs starts Stockfish via an arbitrary command + args, e.g.
// ("taskpolicy", "-b", path) to pin it to macOS E-cores for battery-friendly runs.
func NewStockfishClientArgs(name string, args ...string) (*StockfishClient, error) {
	cmd := exec.Command(name, args...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start stockfish: %w", err)
	}

	client := &StockfishClient{
		cmd:    cmd,
		stdin:  bufio.NewWriter(stdin),
		stdout: bufio.NewScanner(stdout),
	}

	// Initialize UCI
	if err := client.sendCommand("uci"); err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to initialize UCI: %w", err)
	}

	if err := client.waitForResponse("uciok", 5*time.Second); err != nil {
		client.Close()
		return nil, fmt.Errorf("UCI initialization failed: %w", err)
	}

	return client, nil
}

// Close closes the Stockfish client
func (sf *StockfishClient) Close() error {
	if sf.stdin != nil {
		sf.sendCommand("quit")
		sf.stdin.Flush()
	}

	if sf.cmd != nil && sf.cmd.Process != nil {
		// Give it a moment to quit gracefully
		done := make(chan error, 1)
		go func() {
			done <- sf.cmd.Wait()
		}()

		select {
		case <-done:
			// Process ended gracefully
		case <-time.After(2 * time.Second):
			// Force kill if it doesn't quit
			sf.cmd.Process.Kill()
		}
	}

	return nil
}

// sendCommand sends a command to Stockfish
func (sf *StockfishClient) sendCommand(command string) error {
	_, err := sf.stdin.WriteString(command + "\n")
	if err != nil {
		return err
	}
	return sf.stdin.Flush()
}

// waitForResponse waits for a specific response from Stockfish
func (sf *StockfishClient) waitForResponse(expected string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		if sf.stdout.Scan() {
			line := strings.TrimSpace(sf.stdout.Text())
			if strings.Contains(line, expected) {
				return nil
			}
		}

		if err := sf.stdout.Err(); err != nil {
			return fmt.Errorf("error reading from stockfish: %w", err)
		}
	}

	return fmt.Errorf("timeout waiting for response: %s", expected)
}

// GetLegalMoves gets all legal moves for a position using Stockfish
func (sf *StockfishClient) GetLegalMoves(fen string) ([]string, error) {
	// Set the position
	if err := sf.sendCommand(fmt.Sprintf("position fen %s", fen)); err != nil {
		return nil, fmt.Errorf("failed to set position: %w", err)
	}

	// Request move generation with very short search to get all legal moves
	// Use depth 1 and very short time to just get move list
	if err := sf.sendCommand("go depth 1"); err != nil {
		return nil, fmt.Errorf("failed to request moves: %w", err)
	}

	// Parse the response to get the best move (which proves the position is legal)
	// But we actually want all legal moves, so let's use a different approach

	// Wait for bestmove response
	deadline := time.Now().Add(5 * time.Second)
	var bestMove string

	for time.Now().Before(deadline) {
		if sf.stdout.Scan() {
			line := strings.TrimSpace(sf.stdout.Text())
			if strings.HasPrefix(line, "bestmove") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					bestMove = parts[1]
				}
				break
			}
		}
	}

	if bestMove == "" {
		return nil, fmt.Errorf("no bestmove received from stockfish")
	}

	// For now, let's use a more sophisticated approach: use the UCI 'go perft 1' command
	// which will give us all legal moves with their counts
	if err := sf.sendCommand(fmt.Sprintf("position fen %s", fen)); err != nil {
		return nil, fmt.Errorf("failed to set position for perft: %w", err)
	}

	if err := sf.sendCommand("go perft 1"); err != nil {
		return nil, fmt.Errorf("failed to run perft: %w", err)
	}

	// Parse perft output to extract moves
	moves := make([]string, 0)
	deadline = time.Now().Add(5 * time.Second)

	// Regex to match perft output like "e2e4: 1"
	moveRegex := regexp.MustCompile(`^([a-h][1-8][a-h][1-8][qrbn]?): \d+$`)

	for time.Now().Before(deadline) {
		if sf.stdout.Scan() {
			line := strings.TrimSpace(sf.stdout.Text())

			// Check if this line contains a move
			if matches := moveRegex.FindStringSubmatch(line); matches != nil {
				moves = append(moves, matches[1])
			}

			// Stop when we see "Nodes searched:"
			if strings.HasPrefix(line, "Nodes searched:") {
				break
			}
		}
	}

	// Sort moves for consistent comparison
	sort.Strings(moves)
	return moves, nil
}

// ValidatePosition checks if a FEN position is legal according to Stockfish
func (sf *StockfishClient) ValidatePosition(fen string) (bool, error) {
	if err := sf.sendCommand(fmt.Sprintf("position fen %s", fen)); err != nil {
		return false, fmt.Errorf("failed to set position: %w", err)
	}

	// Try to get a move - if position is illegal, this will fail
	if err := sf.sendCommand("go depth 1"); err != nil {
		return false, fmt.Errorf("failed to validate position: %w", err)
	}

	deadline := time.Now().Add(3 * time.Second)

	for time.Now().Before(deadline) {
		if sf.stdout.Scan() {
			line := strings.TrimSpace(sf.stdout.Text())
			if strings.HasPrefix(line, "bestmove") {
				// If we get a bestmove, position is legal
				return true, nil
			}
			// Stockfish might output an error for illegal positions
			if strings.Contains(strings.ToLower(line), "illegal") ||
				strings.Contains(strings.ToLower(line), "invalid") {
				return false, nil
			}
		}
	}

	// If we timeout, assume position is legal (conservative approach)
	return true, nil
}

// GetLegalMovesWithTime gets all legal moves for a position with custom time limit
func (sf *StockfishClient) GetLegalMovesWithTime(fen string, timeMs int) ([]string, error) {
	// Set the position
	if err := sf.sendCommand(fmt.Sprintf("position fen %s", fen)); err != nil {
		return nil, fmt.Errorf("failed to set position: %w", err)
	}

	// Use perft 1 for getting all legal moves (most reliable)
	if err := sf.sendCommand("go perft 1"); err != nil {
		return nil, fmt.Errorf("failed to run perft: %w", err)
	}

	// Parse perft output to extract moves
	moves := make([]string, 0)
	deadline := time.Now().Add(time.Duration(timeMs)*time.Millisecond + 2*time.Second) // Add buffer

	// Regex to match perft output like "e2e4: 1"
	moveRegex := regexp.MustCompile(`^([a-h][1-8][a-h][1-8][qrbn]?): \d+$`)

	for time.Now().Before(deadline) {
		if sf.stdout.Scan() {
			line := strings.TrimSpace(sf.stdout.Text())

			// Check if this line contains a move
			if matches := moveRegex.FindStringSubmatch(line); matches != nil {
				moves = append(moves, matches[1])
			}

			// Stop when we see "Nodes searched:"
			if strings.HasPrefix(line, "Nodes searched:") {
				break
			}
		}
	}

	// Sort moves for consistent comparison
	sort.Strings(moves)
	return moves, nil
}

// GetBestMove gets Stockfish's best move for a position with custom time limit
func (sf *StockfishClient) GetBestMove(fen string, timeMs int) (string, error) {
	// Set the position
	if err := sf.sendCommand(fmt.Sprintf("position fen %s", fen)); err != nil {
		return "", fmt.Errorf("failed to set position: %w", err)
	}

	// Search with time limit
	if err := sf.sendCommand(fmt.Sprintf("go movetime %d", timeMs)); err != nil {
		return "", fmt.Errorf("failed to start search: %w", err)
	}

	// Wait for bestmove response
	deadline := time.Now().Add(time.Duration(timeMs)*time.Millisecond + 3*time.Second) // Add buffer

	for time.Now().Before(deadline) {
		if sf.stdout.Scan() {
			line := strings.TrimSpace(sf.stdout.Text())
			if strings.HasPrefix(line, "bestmove") {
				parts := strings.Fields(line)
				if len(parts) >= 2 && parts[1] != "(none)" {
					return parts[1], nil
				}
			}
		}

		if err := sf.stdout.Err(); err != nil {
			return "", fmt.Errorf("error reading from stockfish: %w", err)
		}
	}

	return "", fmt.Errorf("no bestmove received within time limit")
}

// GetEval searches a position for timeMs and returns Stockfish's final score in
// centipawns (from the side-to-move's perspective; mate scores mapped to large
// magnitudes) along with its best move. Intended as an objective oracle.
func (sf *StockfishClient) GetEval(fen string, timeMs int) (score int, bestMove string, err error) {
	if err = sf.sendCommand(fmt.Sprintf("position fen %s", fen)); err != nil {
		return 0, "", err
	}
	if err = sf.sendCommand(fmt.Sprintf("go movetime %d", timeMs)); err != nil {
		return 0, "", err
	}

	scoreRe := regexp.MustCompile(`score (cp|mate) (-?\d+)`)
	haveScore := false
	deadline := time.Now().Add(time.Duration(timeMs)*time.Millisecond + 5*time.Second)

	for time.Now().Before(deadline) {
		if !sf.stdout.Scan() {
			if e := sf.stdout.Err(); e != nil {
				return 0, "", e
			}
			break
		}
		line := strings.TrimSpace(sf.stdout.Text())

		if m := scoreRe.FindStringSubmatch(line); m != nil {
			v, _ := strconv.Atoi(m[2])
			if m[1] == "mate" {
				if v >= 0 {
					score = 30000 - v
				} else {
					score = -30000 - v
				}
			} else {
				score = v
			}
			haveScore = true
		}

		if strings.HasPrefix(line, "bestmove") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				bestMove = parts[1]
			}
			if !haveScore {
				return 0, bestMove, fmt.Errorf("no score reported (terminal position?)")
			}
			return score, bestMove, nil
		}
	}
	return 0, "", fmt.Errorf("timeout or pipe closed before bestmove")
}
