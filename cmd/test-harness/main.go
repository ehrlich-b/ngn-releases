package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type TestPosition struct {
	Name  string
	ECO   string
	Moves string
	FEN   string
}

type TestResult struct {
	Position    TestPosition
	Success     bool
	BestMove    string
	TimeElapsed time.Duration
	Error       string
	Depth       int
	Nodes       int
	Score       int
}

type UCIEngine struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser
}

func NewUCIEngine(enginePath string) (*UCIEngine, error) {
	cmd := exec.Command(enginePath)

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

	engine := &UCIEngine{
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
		stderr: stderr,
	}

	// Initialize UCI
	if err := engine.sendCommand("uci"); err != nil {
		return nil, err
	}

	// Wait for uciok
	if err := engine.waitForResponse("uciok", 5*time.Second); err != nil {
		return nil, err
	}

	// Set ready
	if err := engine.sendCommand("isready"); err != nil {
		return nil, err
	}

	if err := engine.waitForResponse("readyok", 5*time.Second); err != nil {
		return nil, err
	}

	return engine, nil
}

func (e *UCIEngine) sendCommand(command string) error {
	log.Printf(">>> %s", command)
	_, err := fmt.Fprintf(e.stdin, "%s\n", command)
	return err
}

func (e *UCIEngine) waitForResponse(expectedResponse string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	scanner := bufio.NewScanner(e.stdout)

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for response: %s", expectedResponse)
		default:
			if scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				log.Printf("<<< %s", line)

				if strings.Contains(line, expectedResponse) {
					return nil
				}
			}
			if err := scanner.Err(); err != nil {
				return err
			}
		}
	}
}

func (e *UCIEngine) searchPosition(position string, depth int, timeLimit time.Duration) (*TestResult, error) {
	result := &TestResult{}
	startTime := time.Now()

	// Set position
	if err := e.sendCommand(fmt.Sprintf("position fen %s", position)); err != nil {
		return nil, err
	}

	// Start search with timeout
	searchCmd := fmt.Sprintf("go depth %d", depth)
	if err := e.sendCommand(searchCmd); err != nil {
		return nil, err
	}

	// Wait for bestmove with timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeLimit)
	defer cancel()

	scanner := bufio.NewScanner(e.stdout)
	bestmovePattern := regexp.MustCompile(`bestmove\s+(\w+)`)
	infoPattern := regexp.MustCompile(`info.*depth\s+(\d+).*nodes\s+(\d+).*score\s+cp\s+(-?\d+)`)

	for {
		select {
		case <-ctx.Done():
			result.Success = false
			result.Error = "timeout waiting for bestmove"
			result.TimeElapsed = time.Since(startTime)
			return result, nil

		default:
			if scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				log.Printf("<<< %s", line)

				// Parse info lines
				if matches := infoPattern.FindStringSubmatch(line); matches != nil {
					if depth, err := strconv.Atoi(matches[1]); err == nil {
						result.Depth = depth
					}
					if nodes, err := strconv.Atoi(matches[2]); err == nil {
						result.Nodes = nodes
					}
					if score, err := strconv.Atoi(matches[3]); err == nil {
						result.Score = score
					}
				}

				// Check for bestmove
				if matches := bestmovePattern.FindStringSubmatch(line); matches != nil {
					result.Success = true
					result.BestMove = matches[1]
					result.TimeElapsed = time.Since(startTime)
					return result, nil
				}
			}
			if err := scanner.Err(); err != nil {
				result.Success = false
				result.Error = fmt.Sprintf("scanner error: %v", err)
				result.TimeElapsed = time.Since(startTime)
				return result, nil
			}
		}
	}
}

func (e *UCIEngine) Close() error {
	if e.stdin != nil {
		e.sendCommand("quit")
		e.stdin.Close()
	}
	if e.stdout != nil {
		e.stdout.Close()
	}
	if e.stderr != nil {
		e.stderr.Close()
	}
	if e.cmd != nil {
		return e.cmd.Wait()
	}
	return nil
}

func parsePGN(filename string) ([]TestPosition, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var positions []TestPosition
	var currentPos TestPosition
	_ = false // inGame placeholder

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines
		if line == "" {
			continue
		}

		// Parse headers
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			if strings.Contains(line, `[Black "`) {
				// Extract opening name
				re := regexp.MustCompile(`\[Black\s+"([^"]+)"\]`)
				if matches := re.FindStringSubmatch(line); matches != nil {
					currentPos.Name = matches[1]
				}
			} else if strings.Contains(line, `[ECO "`) {
				re := regexp.MustCompile(`\[ECO\s+"([^"]+)"\]`)
				if matches := re.FindStringSubmatch(line); matches != nil {
					currentPos.ECO = matches[1]
				}
			}
		} else if !strings.HasPrefix(line, "[") {
			// This is the move line
			currentPos.Moves = strings.Replace(line, " *", "", 1)
			currentPos.FEN = movesToFEN(currentPos.Moves)
			positions = append(positions, currentPos)
			currentPos = TestPosition{}
		}
	}

	return positions, scanner.Err()
}

func movesToFEN(moves string) string {
	// For now, return starting position FEN for all positions
	// In a full implementation, you'd apply the moves to get the final FEN
	// This is simplified for the test harness
	return "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"
}

func runTests() error {
	// Parse positions from PGN
	positions, err := parsePGN("openings.pgn")
	if err != nil {
		return fmt.Errorf("failed to parse PGN: %v", err)
	}

	log.Printf("Found %d test positions", len(positions))

	// Create log file for detailed results
	logFile, err := os.Create("test_results.log")
	if err != nil {
		return fmt.Errorf("failed to create log file: %v", err)
	}
	defer logFile.Close()

	logger := log.New(logFile, "", log.LstdFlags|log.Lmicroseconds)

	var successCount, failCount, timeoutCount int
	var totalTime time.Duration

	// Test each position
	for i, pos := range positions {
		logger.Printf("=== TEST %d: %s (%s) ===", i+1, pos.Name, pos.ECO)
		logger.Printf("Moves: %s", pos.Moves)

		// Start fresh engine for each test to avoid state issues
		enginePath := "./ngn"
		if _, err := os.Stat(enginePath); os.IsNotExist(err) {
			// Try building it first
			if err := exec.Command("make", "build").Run(); err != nil {
				return fmt.Errorf("failed to build engine: %v", err)
			}
		}

		engine, err := NewUCIEngine(enginePath)
		if err != nil {
			logger.Printf("Failed to start engine: %v", err)
			failCount++
			continue
		}

		// Test with increasing depth until we find the hang point
		maxDepth := 6
		lastSuccessDepth := 0

		for depth := 1; depth <= maxDepth; depth++ {
			logger.Printf("--- Testing depth %d ---", depth)

			result, err := engine.searchPosition(pos.FEN, depth, 30*time.Second)
			if err != nil {
				logger.Printf("Search error: %v", err)
				break
			}

			totalTime += result.TimeElapsed

			if result.Success {
				logger.Printf("SUCCESS: bestmove=%s, time=%v, depth=%d, nodes=%d, score=%d",
					result.BestMove, result.TimeElapsed, result.Depth, result.Nodes, result.Score)
				lastSuccessDepth = depth
			} else {
				logger.Printf("FAILED: %s, time=%v", result.Error, result.TimeElapsed)
				if strings.Contains(result.Error, "timeout") {
					logger.Printf("*** HANG DETECTED at depth %d for position: %s ***", depth, pos.Name)
					timeoutCount++
				} else {
					failCount++
				}
				break
			}
		}

		if lastSuccessDepth > 0 {
			successCount++
		}

		engine.Close()

		// Print progress
		fmt.Printf("Test %d/%d: %s - Max depth: %d\n", i+1, len(positions), pos.Name, lastSuccessDepth)
	}

	// Summary
	summary := fmt.Sprintf(`
=== TEST SUMMARY ===
Total positions tested: %d
Successful: %d
Failed: %d  
Timeouts/Hangs: %d
Total test time: %v
Average time per position: %v
`, len(positions), successCount, failCount, timeoutCount, totalTime, totalTime/time.Duration(len(positions)))

	logger.Print(summary)
	fmt.Print(summary)

	return nil
}

func main() {
	// Set up logging
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	if err := runTests(); err != nil {
		log.Fatalf("Test harness failed: %v", err)
	}
}
