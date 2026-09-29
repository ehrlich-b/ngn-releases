package main

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
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

type BenchmarkPosition struct {
	Name string `json:"name"`
	FEN  string `json:"fen"`
}

type BenchmarkResult struct {
	Position    BenchmarkPosition `json:"position"`
	Depth       int               `json:"depth"`
	TimeLimit   time.Duration     `json:"time_limit"`
	TimeElapsed time.Duration     `json:"time_elapsed"`
	Nodes       int               `json:"nodes"`
	NPS         int               `json:"nps"`
	Score       int               `json:"score"`
	BestMove    string            `json:"best_move"`
	Success     bool              `json:"success"`
	Error       string            `json:"error,omitempty"`
}

type BenchmarkSummary struct {
	Timestamp      time.Time         `json:"timestamp"`
	TotalPositions int               `json:"total_positions"`
	Successful     int               `json:"successful"`
	Failed         int               `json:"failed"`
	TotalTime      time.Duration     `json:"total_time"`
	AvgNPS         int               `json:"avg_nps"`
	MinNPS         int               `json:"min_nps"`
	MaxNPS         int               `json:"max_nps"`
	Results        []BenchmarkResult `json:"results"`
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

func (e *UCIEngine) benchmarkPosition(fen string, timeLimit time.Duration) (*BenchmarkResult, error) {
	result := &BenchmarkResult{}
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
	infoPattern := regexp.MustCompile(`info.*depth\s+(\d+).*score\s+(?:cp\s+(-?\d+)|mate\s+(-?\d+)).*nodes\s+(\d+).*time\s+(\d+).*nps\s+(\d+)`)

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

				// Parse info lines for nodes and nps
				if matches := infoPattern.FindStringSubmatch(line); matches != nil {
					if depth, err := strconv.Atoi(matches[1]); err == nil {
						result.Depth = depth
					}
					// Handle both cp and mate scores (groups 2 and 3)
					if matches[2] != "" { // cp score
						if score, err := strconv.Atoi(matches[2]); err == nil {
							result.Score = score
						}
					} else if matches[3] != "" { // mate score - convert to large cp equivalent
						if mate, err := strconv.Atoi(matches[3]); err == nil {
							if mate > 0 {
								result.Score = 29000 + (1000 - mate) // Positive mate
							} else {
								result.Score = -29000 + (1000 + mate) // Negative mate
							}
						}
					}
					if nodes, err := strconv.Atoi(matches[4]); err == nil {
						result.Nodes = nodes
					}
					// Skip time (matches[5])
					if nps, err := strconv.Atoi(matches[6]); err == nil {
						result.NPS = nps
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

func loadBenchmarkPositions(filename string) ([]BenchmarkPosition, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var positions []BenchmarkPosition
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse format: "Name: FEN"
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		pos := BenchmarkPosition{
			Name: strings.TrimSpace(parts[0]),
			FEN:  strings.TrimSpace(parts[1]),
		}
		positions = append(positions, pos)
	}

	return positions, scanner.Err()
}

func saveResultsToCSV(results []BenchmarkResult, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	writer.Write([]string{"Name", "FEN", "TimeElapsed(ms)", "Nodes", "NPS", "Score", "BestMove", "Success", "Error"})

	// Write results
	for _, result := range results {
		writer.Write([]string{
			result.Position.Name,
			result.Position.FEN,
			strconv.FormatInt(result.TimeElapsed.Milliseconds(), 10),
			strconv.Itoa(result.Nodes),
			strconv.Itoa(result.NPS),
			strconv.Itoa(result.Score),
			result.BestMove,
			strconv.FormatBool(result.Success),
			result.Error,
		})
	}

	return nil
}

func saveResultsToJSON(results []BenchmarkResult, filename string) error {
	summary := BenchmarkSummary{
		Timestamp:      time.Now(),
		TotalPositions: len(results),
		Results:        results,
	}

	// Calculate summary stats
	var totalTime time.Duration
	var totalNPS, successful, minNPS, maxNPS int
	minNPS = int(^uint(0) >> 1) // max int

	for _, result := range results {
		totalTime += result.TimeElapsed
		if result.Success {
			successful++
			totalNPS += result.NPS
			if result.NPS < minNPS {
				minNPS = result.NPS
			}
			if result.NPS > maxNPS {
				maxNPS = result.NPS
			}
		}
	}

	summary.Successful = successful
	summary.Failed = len(results) - successful
	summary.TotalTime = totalTime

	if successful > 0 {
		summary.AvgNPS = totalNPS / successful
	} else {
		minNPS = 0
	}

	if minNPS == int(^uint(0)>>1) {
		minNPS = 0
	}
	summary.MinNPS = minNPS
	summary.MaxNPS = maxNPS

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(summary)
}

func checkRegression(currentNPS, baselineNPS int, threshold float64) bool {
	if baselineNPS == 0 {
		return false
	}

	regression := float64(baselineNPS-currentNPS) / float64(baselineNPS) * 100
	return regression > threshold
}

func loadBaselineResults(filename string) (map[string]int, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var summary BenchmarkSummary
	if err := json.NewDecoder(file).Decode(&summary); err != nil {
		return nil, err
	}

	baseline := make(map[string]int)
	for _, result := range summary.Results {
		if result.Success {
			baseline[result.Position.Name] = result.NPS
		}
	}

	return baseline, nil
}

func runBenchmark() error {
	fmt.Println("=== NGN CHESS ENGINE BENCHMARK ===")

	// Configuration
	const (
		timeLimitPerPosition = 1000 * time.Millisecond // 1 second per position
		regressionThreshold  = 5.0                     // 5% regression threshold
		positionsFile        = "benchmark_positions.txt"
		baselineFile         = "benchmark_baseline.json"
		resultsCSV           = "benchmark_results.csv"
		resultsJSON          = "benchmark_results.json"
	)

	// Build engine
	fmt.Println("Building engine...")
	if err := exec.Command("make", "-C", "../..", "build").Run(); err != nil {
		return fmt.Errorf("failed to build engine: %v", err)
	}

	enginePath := "../../build/ngn"

	// Load benchmark positions
	positions, err := loadBenchmarkPositions(positionsFile)
	if err != nil {
		return fmt.Errorf("failed to load benchmark positions: %v", err)
	}

	if len(positions) == 0 {
		return fmt.Errorf("no benchmark positions found in %s", positionsFile)
	}

	fmt.Printf("Loaded %d benchmark positions\n", len(positions))

	// Load baseline for regression detection
	var baseline map[string]int
	if _, err := os.Stat(baselineFile); err == nil {
		baseline, err = loadBaselineResults(baselineFile)
		if err != nil {
			fmt.Printf("Warning: failed to load baseline results: %v\n", err)
		} else {
			fmt.Printf("Loaded baseline results for regression detection\n")
		}
	}

	// Start engine
	engine, err := NewUCIEngine(enginePath)
	if err != nil {
		return fmt.Errorf("failed to start engine: %v", err)
	}
	defer engine.Close()

	var results []BenchmarkResult
	var regressionCount int

	fmt.Println("\nRunning benchmark...")

	for i, pos := range positions {
		fmt.Printf("Position %d/%d: %s\n", i+1, len(positions), pos.Name)

		result, err := engine.benchmarkPosition(pos.FEN, timeLimitPerPosition)
		if err != nil {
			fmt.Printf("  Error: %v\n", err)
			result = &BenchmarkResult{
				Position: pos,
				Success:  false,
				Error:    err.Error(),
			}
		}

		result.Position = pos

		if result.Success {
			fmt.Printf("  ✓ NPS: %d, Nodes: %d, Time: %v, Best: %s\n",
				result.NPS, result.Nodes, result.TimeElapsed.Round(time.Millisecond), result.BestMove)

			// Check for regression
			if baselineNPS, exists := baseline[pos.Name]; exists {
				if checkRegression(result.NPS, baselineNPS, regressionThreshold) {
					fmt.Printf("  ⚠️  REGRESSION: NPS dropped by >%.1f%% (was %d, now %d)\n",
						regressionThreshold, baselineNPS, result.NPS)
					regressionCount++
				}
			}
		} else {
			fmt.Printf("  ✗ Failed: %s\n", result.Error)
		}

		results = append(results, *result)
	}

	// Save results
	if err := saveResultsToCSV(results, resultsCSV); err != nil {
		return fmt.Errorf("failed to save CSV results: %v", err)
	}

	if err := saveResultsToJSON(results, resultsJSON); err != nil {
		return fmt.Errorf("failed to save JSON results: %v", err)
	}

	// Calculate summary
	var successful, totalNPS, minNPS, maxNPS int
	var totalTime time.Duration
	minNPS = int(^uint(0) >> 1)

	for _, result := range results {
		totalTime += result.TimeElapsed
		if result.Success {
			successful++
			totalNPS += result.NPS
			if result.NPS < minNPS {
				minNPS = result.NPS
			}
			if result.NPS > maxNPS {
				maxNPS = result.NPS
			}
		}
	}

	avgNPS := 0
	if successful > 0 {
		avgNPS = totalNPS / successful
	}
	if minNPS == int(^uint(0)>>1) {
		minNPS = 0
	}

	// Print summary
	fmt.Printf("\n=== BENCHMARK SUMMARY ===\n")
	fmt.Printf("Positions tested: %d\n", len(positions))
	fmt.Printf("Successful: %d\n", successful)
	fmt.Printf("Failed: %d\n", len(positions)-successful)
	fmt.Printf("Total time: %v\n", totalTime.Round(time.Millisecond))
	fmt.Printf("Average NPS: %d\n", avgNPS)
	fmt.Printf("Min NPS: %d\n", minNPS)
	fmt.Printf("Max NPS: %d\n", maxNPS)

	if regressionCount > 0 {
		fmt.Printf("⚠️  Regressions detected: %d positions\n", regressionCount)
		fmt.Printf("   (NPS dropped by >%.1f%% compared to baseline)\n", regressionThreshold)
		return fmt.Errorf("performance regression detected")
	}

	fmt.Printf("\nResults saved to:\n")
	fmt.Printf("  CSV: %s\n", resultsCSV)
	fmt.Printf("  JSON: %s\n", resultsJSON)

	return nil
}

func main() {
	if err := runBenchmark(); err != nil {
		log.Fatalf("Benchmark failed: %v", err)
	}
}
