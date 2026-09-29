package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type TacticalPosition struct {
	ID       string
	FEN      string
	BestMove string
	Comment  string
}

type TacticalResult struct {
	Position     TacticalPosition
	FoundMove    string
	Correct      bool
	TimeToSolve  time.Duration
	DepthReached int
	Nodes        int
	NPS          int
	Score        int
}

type StockfishValidation struct {
	Position        TacticalPosition
	StockfishMove   string
	MatchesExpected bool
	StockfishTime   time.Duration
}

type TacticalSummary struct {
	Timestamp      time.Time        `json:"timestamp"`
	TotalTests     int              `json:"total_tests"`
	Correct        int              `json:"correct"`
	Accuracy       float64          `json:"accuracy"`
	AvgSolveTime   time.Duration    `json:"avg_solve_time"`
	MaxSolveTime   time.Duration    `json:"max_solve_time"`
	MinSolveTime   time.Duration    `json:"min_solve_time"`
	RandomSeed     int64            `json:"random_seed"`
	TotalPositions int              `json:"total_positions_available"`
	Results        []TacticalResult `json:"results"`
}

func validateWithStockfish(positions []TacticalPosition, stockfishPath string) ([]StockfishValidation, error) {
	fmt.Println("=== VALIDATING POSITIONS WITH STOCKFISH ===")

	// Start Stockfish
	stockfish, err := NewUCIEngine(stockfishPath)
	if err != nil {
		return nil, fmt.Errorf("failed to start Stockfish: %v", err)
	}
	defer stockfish.Close()

	var validations []StockfishValidation

	for i, pos := range positions {
		fmt.Printf("Validating position %d/%d: %s\n", i+1, len(positions), pos.Comment)

		// Run Stockfish with 500ms
		result, err := stockfish.solveTacticalPosition(pos.FEN, pos.BestMove, 500*time.Millisecond)
		if err != nil {
			fmt.Printf("  Error running Stockfish: %v\n", err)
			continue
		}

		validation := StockfishValidation{
			Position:        pos,
			StockfishMove:   result.FoundMove,
			MatchesExpected: result.Correct,
			StockfishTime:   result.TimeToSolve,
		}

		if validation.MatchesExpected {
			fmt.Printf("  ✓ Stockfish confirms: %s (%.2fs)\n", result.FoundMove, result.TimeToSolve.Seconds())
		} else {
			fmt.Printf("  ⚠ Stockfish disagrees: expected %s, got %s (%.2fs)\n",
				pos.BestMove, result.FoundMove, result.TimeToSolve.Seconds())
		}

		validations = append(validations, validation)
	}

	return validations, nil
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

func (e *UCIEngine) solveTacticalPosition(fen string, expectedMove string, timeLimit time.Duration) (*TacticalResult, error) {
	result := &TacticalResult{}
	startTime := time.Now()

	// Set position
	if err := e.sendCommand(fmt.Sprintf("position fen %s", fen)); err != nil {
		return nil, err
	}

	// Start search with time limit
	searchCmd := fmt.Sprintf("go movetime %d", int(timeLimit.Milliseconds()))
	if err := e.sendCommand(searchCmd); err != nil {
		return nil, err
	}

	// Wait for bestmove with timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeLimit*2) // 2x timeout for safety
	defer cancel()

	scanner := bufio.NewScanner(e.stdout)
	bestmovePattern := regexp.MustCompile(`bestmove\s+(\w+)`)
	infoPattern := regexp.MustCompile(`info.*depth\s+(\d+).*score\s+cp\s+(-?\d+).*nodes\s+(\d+).*time\s+(\d+).*nps\s+(\d+)`)

	foundExpectedMove := false

	for {
		select {
		case <-ctx.Done():
			result.TimeToSolve = time.Since(startTime)
			result.Correct = foundExpectedMove
			if foundExpectedMove {
				result.FoundMove = expectedMove
			}
			return result, nil

		default:
			if scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())

				// Parse info lines for performance metrics
				if matches := infoPattern.FindStringSubmatch(line); matches != nil {
					if depth, err := strconv.Atoi(matches[1]); err == nil {
						result.DepthReached = depth
					}
					if score, err := strconv.Atoi(matches[2]); err == nil {
						result.Score = score
					}
					if nodes, err := strconv.Atoi(matches[3]); err == nil {
						result.Nodes = nodes
					}
					// Skip time (matches[4])
					if nps, err := strconv.Atoi(matches[5]); err == nil {
						result.NPS = nps
					}
				}

				// Check for bestmove
				if matches := bestmovePattern.FindStringSubmatch(line); matches != nil {
					result.FoundMove = matches[1]
					result.TimeToSolve = time.Since(startTime)
					result.Correct = (result.FoundMove == expectedMove)
					return result, nil
				}
			}
			if err := scanner.Err(); err != nil {
				result.TimeToSolve = time.Since(startTime)
				result.Correct = foundExpectedMove
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

func parseEPD(filename string) ([]TacticalPosition, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var positions []TacticalPosition
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// EPD format: FEN bm MOVE; comment
		// Example: rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1 bm b1c3; Starting position
		parts := strings.Split(line, "bm ")
		if len(parts) != 2 {
			continue
		}

		fen := strings.TrimSpace(parts[0])
		rest := parts[1]

		// Extract best move and comment
		var bestMove, comment string
		if strings.Contains(rest, ";") {
			moveComment := strings.SplitN(rest, ";", 2)
			bestMove = strings.TrimSpace(moveComment[0])
			if len(moveComment) > 1 {
				comment = strings.TrimSpace(moveComment[1])
			}
		} else {
			bestMove = strings.TrimSpace(rest)
		}

		pos := TacticalPosition{
			ID:       fmt.Sprintf("pos_%d", len(positions)+1),
			FEN:      fen,
			BestMove: bestMove,
			Comment:  comment,
		}
		positions = append(positions, pos)
	}

	return positions, scanner.Err()
}

func runTacticalTests() error {
	fmt.Println("=== NGN TACTICAL TEST SUITE WITH STOCKFISH VALIDATION ===")

	// Configuration
	const (
		ngnTimeLimit       = 2000 * time.Millisecond // 2 seconds per position for NGN
		stockfishTimeLimit = 500 * time.Millisecond  // 500ms for Stockfish validation
		epdFile            = "../../validated_tactical_positions.epd"
		stockfishPath      = "../../stockfish/stockfish-ubuntu-x86-64-avx2"
		numTestPositions   = 30 // Test 30 random positions
		randomSeed         = 42 // Static seed for consistent results
	)

	// Load tactical positions
	allPositions, err := parseEPD(epdFile)
	if err != nil {
		return fmt.Errorf("failed to load tactical positions: %v", err)
	}

	if len(allPositions) == 0 {
		return fmt.Errorf("no tactical positions found in %s", epdFile)
	}

	fmt.Printf("Loaded %d tactical positions from validated set\n", len(allPositions))

	// Select random subset with static seed for consistency
	rand.Seed(randomSeed)
	selectedIndices := rand.Perm(len(allPositions))[:numTestPositions]
	sort.Ints(selectedIndices) // Sort for consistent ordering

	var positions []TacticalPosition
	for _, idx := range selectedIndices {
		positions = append(positions, allPositions[idx])
	}

	fmt.Printf("Selected %d random positions for testing (seed: %d)\n", len(positions), randomSeed)

	// Step 1: Validate positions with Stockfish
	fmt.Println("\n=== STEP 1: STOCKFISH VALIDATION ===")
	validations, err := validateWithStockfish(positions, stockfishPath)
	if err != nil {
		return fmt.Errorf("Stockfish validation failed: %v", err)
	}

	// Check if all positions are valid
	validPositions := 0
	for _, validation := range validations {
		if validation.MatchesExpected {
			validPositions++
		}
	}

	fmt.Printf("\nStockfish validation complete: %d/%d positions confirmed\n", validPositions, len(positions))

	if validPositions < len(positions) {
		fmt.Println("⚠ Warning: Some positions may have incorrect expected moves!")
		for i, validation := range validations {
			if !validation.MatchesExpected {
				fmt.Printf("  Position %d: Expected %s, Stockfish found %s\n",
					i+1, validation.Position.BestMove, validation.StockfishMove)
			}
		}
	}

	// Step 2: Build and test NGN
	fmt.Println("\n=== STEP 2: BUILDING NGN ENGINE ===")
	if err := exec.Command("go", "build", "-o", "../../build/ngn", "../../main.go").Run(); err != nil {
		return fmt.Errorf("failed to build engine: %v", err)
	}

	enginePath := "../../build/ngn"

	// Start NGN engine
	ngn, err := NewUCIEngine(enginePath)
	if err != nil {
		return fmt.Errorf("failed to start NGN: %v", err)
	}
	defer ngn.Close()

	var results []TacticalResult
	correctCount := 0

	fmt.Println("\n=== STEP 3: RUNNING NGN TACTICAL TESTS ===")

	for i, pos := range positions {
		fmt.Printf("Position %d/%d: %s\n", i+1, len(positions), pos.Comment)

		result, err := ngn.solveTacticalPosition(pos.FEN, pos.BestMove, ngnTimeLimit)
		if err != nil {
			fmt.Printf("  Error: %v\n", err)
			result = &TacticalResult{
				Position: pos,
				Correct:  false,
			}
		}

		result.Position = pos

		if result.Correct {
			correctCount++
			fmt.Printf("  ✓ CORRECT: %s (%.2fs, depth %d, %d nodes, %d NPS)\n",
				result.FoundMove, result.TimeToSolve.Seconds(), result.DepthReached,
				result.Nodes, result.NPS)
		} else {
			fmt.Printf("  ✗ WRONG: expected %s, got %s (%.2fs, depth %d)\n",
				pos.BestMove, result.FoundMove, result.TimeToSolve.Seconds(), result.DepthReached)
		}

		results = append(results, *result)
	}

	// Calculate summary statistics
	var totalSolveTime time.Duration
	var minSolveTime, maxSolveTime time.Duration
	minSolveTime = time.Hour // Initialize to large value

	for _, result := range results {
		totalSolveTime += result.TimeToSolve
		if result.TimeToSolve < minSolveTime {
			minSolveTime = result.TimeToSolve
		}
		if result.TimeToSolve > maxSolveTime {
			maxSolveTime = result.TimeToSolve
		}
	}

	avgSolveTime := totalSolveTime / time.Duration(len(results))
	accuracy := float64(correctCount) / float64(len(results)) * 100

	// Print comprehensive summary
	fmt.Printf("\n=== COMPREHENSIVE TACTICAL TEST SUMMARY ===\n")
	fmt.Printf("Test Set: %d random positions from %d validated positions (seed: %d)\n",
		len(positions), len(allPositions), randomSeed)
	fmt.Printf("Stockfish Validation: %d/%d positions confirmed (%.1f%%)\n",
		validPositions, len(positions), float64(validPositions)/float64(len(positions))*100)
	fmt.Printf("NGN Results: %d/%d correct (%.1f%%)\n",
		correctCount, len(results), accuracy)
	fmt.Printf("NGN Average solve time: %v\n", avgSolveTime.Round(time.Millisecond))
	fmt.Printf("NGN Min solve time: %v\n", minSolveTime.Round(time.Millisecond))
	fmt.Printf("NGN Max solve time: %v\n", maxSolveTime.Round(time.Millisecond))

	// Performance comparison
	if validPositions == len(positions) {
		fmt.Printf("✅ All test positions validated by Stockfish\n")
	} else {
		fmt.Printf("⚠ Some test positions may need review\n")
	}

	if accuracy >= 80 {
		fmt.Printf("🎉 Excellent tactical performance!\n")
	} else if accuracy >= 60 {
		fmt.Printf("👍 Good tactical performance\n")
	} else if accuracy >= 40 {
		fmt.Printf("🤔 Moderate tactical performance - room for improvement\n")
	} else {
		fmt.Printf("❌ Poor tactical performance - significant issues detected\n")
	}

	// Save detailed results
	summary := TacticalSummary{
		Timestamp:      time.Now(),
		TotalTests:     len(positions),
		Correct:        correctCount,
		Accuracy:       accuracy,
		AvgSolveTime:   avgSolveTime,
		MinSolveTime:   minSolveTime,
		MaxSolveTime:   maxSolveTime,
		RandomSeed:     randomSeed,
		TotalPositions: len(allPositions),
		Results:        results,
	}

	// Save to JSON file
	resultsFile, err := os.Create("../../cmd/tactical-test/tactical_results.json")
	if err != nil {
		return fmt.Errorf("failed to create results file: %v", err)
	}
	defer resultsFile.Close()

	encoder := json.NewEncoder(resultsFile)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(summary); err != nil {
		return fmt.Errorf("failed to save results: %v", err)
	}

	fmt.Printf("\nResults saved to: ../../cmd/tactical-test/tactical_results.json\n")

	// Detailed results with Stockfish comparison
	fmt.Printf("\n=== DETAILED RESULTS (NGN vs Expected) ===\n")
	for i, result := range results {
		status := "FAIL"
		if result.Correct {
			status = "PASS"
		}

		// Show Stockfish validation status
		stockfishStatus := ""
		if i < len(validations) {
			if validations[i].MatchesExpected {
				stockfishStatus = " [SF: ✓]"
			} else {
				stockfishStatus = fmt.Sprintf(" [SF: %s]", validations[i].StockfishMove)
			}
		}

		fmt.Printf("%d. %s: %s (%.2fs)%s\n",
			i+1, status, result.Position.Comment, result.TimeToSolve.Seconds(), stockfishStatus)
	}

	return nil
}

func main() {
	if err := runTacticalTests(); err != nil {
		log.Fatalf("Tactical test failed: %v", err)
	}
}
