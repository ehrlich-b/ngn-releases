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

type TestResult struct {
	TestName    string
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
	fmt.Printf(">>> %s\n", command)
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
				fmt.Printf("<<< %s\n", line)

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

func (e *UCIEngine) searchPosition(moves string, depth int, timeLimit time.Duration) (*TestResult, error) {
	result := &TestResult{}
	startTime := time.Now()

	// Set position
	positionCmd := "position startpos"
	if moves != "" {
		positionCmd += " moves " + moves
	}

	if err := e.sendCommand(positionCmd); err != nil {
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
				fmt.Printf("<<< %s\n", line)

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

func testBasicPositions() error {
	fmt.Println("=== NGN ENGINE TEST HARNESS ===")

	// Build engine first - go back to root directory
	fmt.Println("Building engine...")
	if err := exec.Command("make", "-C", "../..", "build").Run(); err != nil {
		return fmt.Errorf("failed to build engine: %v", err)
	}

	enginePath := "../../build/ngn"

	// Test cases - start with simple positions
	testCases := []struct {
		name  string
		moves string
	}{
		{"Starting Position", ""},
		{"After 1.e4", "e2e4"},
		{"After 1.e4 e5", "e2e4 e7e5"},
		{"After 1.d4", "d2d4"},
		{"After 1.Nf3", "g1f3"},
		{"King's Indian Setup", "d2d4 g8f6 c2c4 g7g6 b1c3 f8g7"},
	}

	// Create results log
	logFile, err := os.Create("simple_test_results.log")
	if err != nil {
		return fmt.Errorf("failed to create log file: %v", err)
	}
	defer logFile.Close()

	logger := log.New(logFile, "", log.LstdFlags|log.Lmicroseconds)

	var successCount, failCount int

	for i, test := range testCases {
		fmt.Printf("\n=== TEST %d: %s ===\n", i+1, test.name)
		logger.Printf("=== TEST %d: %s ===", i+1, test.name)
		logger.Printf("Moves: %s", test.moves)

		engine, err := NewUCIEngine(enginePath)
		if err != nil {
			fmt.Printf("Failed to start engine: %v\n", err)
			logger.Printf("Failed to start engine: %v", err)
			failCount++
			continue
		}

		// Test incrementally up to depth 6 or until hang
		maxDepth := 6
		lastSuccessDepth := 0

		for depth := 1; depth <= maxDepth; depth++ {
			fmt.Printf("--- Depth %d ---\n", depth)
			logger.Printf("--- Testing depth %d ---", depth)

			result, err := engine.searchPosition(test.moves, depth, 15*time.Second)
			if err != nil {
				fmt.Printf("Search error: %v\n", err)
				logger.Printf("Search error: %v", err)
				break
			}

			if result.Success {
				fmt.Printf("✓ SUCCESS: %s (%.2fs, %d nodes, score %d)\n",
					result.BestMove, result.TimeElapsed.Seconds(), result.Nodes, result.Score)
				logger.Printf("SUCCESS: bestmove=%s, time=%v, depth=%d, nodes=%d, score=%d",
					result.BestMove, result.TimeElapsed, result.Depth, result.Nodes, result.Score)
				lastSuccessDepth = depth
			} else {
				fmt.Printf("✗ FAILED: %s (%.2fs)\n", result.Error, result.TimeElapsed.Seconds())
				logger.Printf("FAILED: %s, time=%v", result.Error, result.TimeElapsed)

				if strings.Contains(result.Error, "timeout") {
					fmt.Printf("*** HANG DETECTED at depth %d ***\n", depth)
					logger.Printf("*** HANG DETECTED at depth %d for test: %s ***", depth, test.name)
				}
				break
			}
		}

		if lastSuccessDepth > 0 {
			successCount++
			fmt.Printf("✓ Test passed - Max depth: %d\n", lastSuccessDepth)
		} else {
			failCount++
			fmt.Printf("✗ Test failed - No successful depths\n")
		}

		engine.Close()

		// Brief pause between tests
		time.Sleep(100 * time.Millisecond)
	}

	// Summary
	fmt.Printf(`
=== SUMMARY ===
Total tests: %d
Passed: %d
Failed: %d
`, len(testCases), successCount, failCount)

	logger.Printf(`
=== TEST SUMMARY ===
Total tests: %d
Passed: %d
Failed: %d
`, len(testCases), successCount, failCount)

	return nil
}

func main() {
	if err := testBasicPositions(); err != nil {
		log.Fatalf("Test harness failed: %v", err)
	}
}
