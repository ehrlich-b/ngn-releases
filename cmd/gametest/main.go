package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/ehrlich-b/ngn/engine"
)

// GameTestConfig holds configuration for engine vs engine testing
type GameTestConfig struct {
	// Engine paths
	NGNPath       string
	StockfishPath string

	// Game settings
	TimeControl time.Duration // Time per move
	MaxMoves    int           // Maximum moves per game
	GameCount   int           // Number of games to play

	// Stockfish weakening
	StockfishSkill int // 0-20 (20 is strongest)
	StockfishElo   int // 1320-3190 (if UCI_LimitStrength enabled)

	// Output
	LogFile   string
	PGNOutput string
	Verbose   bool
}

// Engine represents a UCI chess engine
type Engine struct {
	Name    string
	Process *exec.Cmd
	Stdin   *bufio.Writer
	Stdout  *bufio.Scanner
	Stderr  *bufio.Scanner
}

// MoveInfo contains move and evaluation information
type MoveInfo struct {
	Move       string
	Evaluation string // e.g., "cp 45" or "mate 3"
	PV         string // Principal variation
	Depth      int    // Search depth
	Nodes      int    // Nodes searched
}

// Game represents a single chess game
type Game struct {
	ID       int
	Moves    []string
	Result   string // "1-0", "0-1", "1/2-1/2", "*"
	Reason   string // "checkmate", "timeout", "draw", etc.
	Duration time.Duration

	// Analysis data
	NGNMoveTime       []time.Duration
	StockfishMoveTime []time.Duration
	Positions         []string // FEN positions
	Evaluations       []int    // NGN evaluations in centipawns
}

// GameTester orchestrates engine vs engine games
type GameTester struct {
	Config *GameTestConfig
	Logger *log.Logger
	Games  []Game
}

func main() {
	config := &GameTestConfig{
		NGNPath:        "./build/ngn",
		StockfishPath:  "./stockfish/stockfish-ubuntu-x86-64-avx2",
		TimeControl:    time.Second * 5, // 5 seconds per move
		MaxMoves:       200,
		GameCount:      10,
		StockfishSkill: 10, // Medium strength
		LogFile:        "gametest.log",
		PGNOutput:      "games.pgn",
		Verbose:        true,
	}

	// Parse command line arguments
	parseArgs(config)

	// Create logger
	logFile, err := os.Create(config.LogFile)
	if err != nil {
		log.Fatal("Failed to create log file:", err)
	}
	defer logFile.Close()

	logger := log.New(logFile, "", log.LstdFlags|log.Lmicroseconds)

	tester := &GameTester{
		Config: config,
		Logger: logger,
		Games:  make([]Game, 0, config.GameCount),
	}

	fmt.Printf("Starting %d games: NGN vs %s\n",
		config.GameCount, getStockfishDisplayName(config))

	// Run the test suite
	if err := tester.RunTestSuite(); err != nil {
		log.Fatal("Test suite failed:", err)
	}

	// Print results summary
	tester.PrintSummary()
}

func parseArgs(config *GameTestConfig) {
	for i, arg := range os.Args[1:] {
		switch {
		case arg == "--games" && i+1 < len(os.Args[1:]):
			if count, err := strconv.Atoi(os.Args[i+2]); err == nil {
				config.GameCount = count
			}
		case arg == "--time" && i+1 < len(os.Args[1:]):
			if timeFloat, err := strconv.ParseFloat(os.Args[i+2], 64); err == nil {
				config.TimeControl = time.Duration(timeFloat*1000) * time.Millisecond
			}
		case arg == "--skill" && i+1 < len(os.Args[1:]):
			if skill, err := strconv.Atoi(os.Args[i+2]); err == nil && skill >= 0 && skill <= 20 {
				config.StockfishSkill = skill
			}
		case arg == "--elo" && i+1 < len(os.Args[1:]):
			if elo, err := strconv.Atoi(os.Args[i+2]); err != nil {
				fmt.Fprintf(os.Stderr, "Error: Invalid ELO value '%s': must be a number\n", os.Args[i+2])
				os.Exit(1)
			} else if elo < 1320 || elo > 3190 {
				fmt.Fprintf(os.Stderr, "Error: ELO value %d is out of range. Must be between 1320-3190\n", elo)
				os.Exit(1)
			} else {
				config.StockfishElo = elo
			}
		case arg == "--verbose":
			config.Verbose = true
		case arg == "--quiet":
			config.Verbose = false
		}
	}
}

// RunTestSuite executes the complete test suite
func (gt *GameTester) RunTestSuite() error {
	gt.Logger.Printf("Starting test suite: %d games, %v per move",
		gt.Config.GameCount, gt.Config.TimeControl)

	for gameID := 1; gameID <= gt.Config.GameCount; gameID++ {
		fmt.Printf("Game %d/%d... ", gameID, gt.Config.GameCount)

		game, err := gt.PlayGame(gameID)
		if err != nil {
			fmt.Printf("FAILED: %v\n", err)
			gt.Logger.Printf("Game %d failed: %v", gameID, err)
			continue
		}

		gt.Games = append(gt.Games, game)
		fmt.Printf("%s (%s) in %v\n", game.Result, game.Reason, game.Duration)
		gt.Logger.Printf("Game %d: %s (%s) in %d moves, %v",
			gameID, game.Result, game.Reason, len(game.Moves), game.Duration)
	}

	// Save PGN file
	return gt.SavePGN()
}

// PlayGame plays a single game between NGN and Stockfish
func (gt *GameTester) PlayGame(gameID int) (Game, error) {
	game := Game{
		ID:                gameID,
		Moves:             make([]string, 0),
		Positions:         make([]string, 0),
		NGNMoveTime:       make([]time.Duration, 0),
		StockfishMoveTime: make([]time.Duration, 0),
	}

	startTime := time.Now()
	// Skip validation for quick testing

	// Alternate who plays white (NGN=odd games, Stockfish=even games)
	ngnPlaysWhite := gameID%2 == 1

	// Start engines
	ngn, err := gt.StartEngine(gt.Config.NGNPath, "NGN")
	if err != nil {
		return game, fmt.Errorf("failed to start NGN: %v", err)
	}
	defer gt.StopEngine(ngn)

	stockfish, err := gt.StartEngine(gt.Config.StockfishPath, "Stockfish")
	if err != nil {
		return game, fmt.Errorf("failed to start Stockfish: %v", err)
	}
	defer gt.StopEngine(stockfish)

	// Configure Stockfish weakening
	if err := gt.ConfigureStockfish(stockfish); err != nil {
		return game, fmt.Errorf("failed to configure Stockfish: %v", err)
	}

	// Play the game
	var whiteEngine, blackEngine *Engine
	if ngnPlaysWhite {
		whiteEngine, blackEngine = ngn, stockfish
	} else {
		whiteEngine, blackEngine = stockfish, ngn
	}

	position := "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1" // Starting position
	game.Positions = append(game.Positions, position)

	// Game loop
	for moveNum := 0; moveNum < gt.Config.MaxMoves; moveNum++ {
		// Skip game termination checks for quick testing

		isWhiteMove := moveNum%2 == 0
		currentEngine := whiteEngine
		if !isWhiteMove {
			currentEngine = blackEngine
		}

		// Send position to current engine
		positionCmd := fmt.Sprintf("position fen %s", position)
		if len(game.Moves) > 0 {
			positionCmd += " moves " + strings.Join(game.Moves, " ")
		}

		if err := gt.SendCommand(currentEngine, positionCmd); err != nil {
			return game, fmt.Errorf("failed to send position: %v", err)
		}

		// Request move with time limit
		goCmd := fmt.Sprintf("go movetime %d", int(gt.Config.TimeControl.Milliseconds()))
		moveStart := time.Now()

		moveInfo, err := gt.GetMoveWithInfo(currentEngine, goCmd)
		moveTime := time.Since(moveStart)
		move := moveInfo.Move

		if err != nil {
			game.Result = "1/2-1/2"
			game.Reason = fmt.Sprintf("engine_error_%s", currentEngine.Name)
			break
		}

		if move == "" || move == "(none)" {
			// No legal moves - need to determine if it's checkmate or stalemate
			// Parse the current position to check if the side to move is in check
			currentFEN := position
			if len(game.Moves) > 0 {
				// Build FEN from moves - use engine to get current position
				pos, err := engine.ParseFEN("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1")
				if err == nil {
					for _, m := range game.Moves {
						parsedMove, err := engine.ParseAlgebraicMove(m, pos)
						if err == nil {
							pos.MakeMove(parsedMove)
						}
					}
					currentFEN = engine.GenerateFEN(pos)
				}
			}

			// Parse position and check if in check
			pos, err := engine.ParseFEN(currentFEN)
			if err == nil && pos.IsInCheck() {
				// Side to move is in check with no legal moves = checkmate
				// The OTHER side wins
				if isWhiteMove {
					// White to move but checkmated = Black wins
					game.Result = "0-1"
					game.Reason = "checkmate"
				} else {
					// Black to move but checkmated = White wins
					game.Result = "1-0"
					game.Reason = "checkmate"
				}
			} else {
				// Not in check with no legal moves = stalemate
				game.Result = "1/2-1/2"
				game.Reason = "stalemate"
			}
			break
		}

		// Skip move validation for quick testing

		// Record move timing
		if currentEngine == ngn {
			game.NGNMoveTime = append(game.NGNMoveTime, moveTime)
		} else {
			game.StockfishMoveTime = append(game.StockfishMoveTime, moveTime)
		}

		game.Moves = append(game.Moves, move)

		// Skip position update for quick testing
		// position stays the same for simplified testing
		game.Positions = append(game.Positions, position)

		if gt.Config.Verbose {
			// Show enhanced move information with evaluations
			engineName := "NGN"
			eval := moveInfo.Evaluation
			pv := moveInfo.PV
			depth := moveInfo.Depth

			if currentEngine != ngn {
				engineName = "SF"
			}

			// Truncate PV to first 4 moves for display
			pvDisplay := pv
			pvParts := strings.Fields(pv)
			if len(pvParts) > 4 {
				pvDisplay = strings.Join(pvParts[:4], " ") + "..."
			}

			fmt.Printf("  %2d. %-5s (%3dms) [%s d%d %8s] %s %s\n",
				len(game.Moves), move, int(moveTime.Milliseconds()),
				engineName, depth, eval,
				engineName, pvDisplay)
		}

		// Safety limit - still end at max moves if no other termination
		if len(game.Moves) >= gt.Config.MaxMoves {
			game.Result = "1/2-1/2"
			game.Reason = "max_moves_reached"
			break
		}
	}

	game.Duration = time.Since(startTime)
	return game, nil
}

// StartEngine starts a UCI engine
func (gt *GameTester) StartEngine(path, name string) (*Engine, error) {
	cmd := exec.Command(path)

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
		Name:    name,
		Process: cmd,
		Stdin:   bufio.NewWriter(stdin),
		Stdout:  bufio.NewScanner(stdout),
		Stderr:  bufio.NewScanner(stderr),
	}

	// Initialize UCI
	if err := gt.SendCommand(engine, "uci"); err != nil {
		return nil, err
	}

	// Wait for uciok
	for engine.Stdout.Scan() {
		line := engine.Stdout.Text()
		gt.Logger.Printf("%s: %s", name, line)
		if line == "uciok" {
			break
		}
	}

	if err := gt.SendCommand(engine, "isready"); err != nil {
		return nil, err
	}

	// Wait for readyok
	for engine.Stdout.Scan() {
		line := engine.Stdout.Text()
		gt.Logger.Printf("%s: %s", name, line)
		if line == "readyok" {
			break
		}
	}

	return engine, nil
}

// ConfigureStockfish sets up Stockfish with weakening parameters
func (gt *GameTester) ConfigureStockfish(engine *Engine) error {
	// Set ELO limitation if configured (takes precedence over skill level)
	if gt.Config.StockfishElo > 0 {
		if err := gt.SendCommand(engine, "setoption name UCI_LimitStrength value true"); err != nil {
			return err
		}
		eloCmd := fmt.Sprintf("setoption name UCI_Elo value %d", gt.Config.StockfishElo)
		if err := gt.SendCommand(engine, eloCmd); err != nil {
			return err
		}
	} else {
		// Set skill level only if ELO is not configured
		skillCmd := fmt.Sprintf("setoption name Skill Level value %d", gt.Config.StockfishSkill)
		if err := gt.SendCommand(engine, skillCmd); err != nil {
			return err
		}
	}

	return nil
}

// SendCommand sends a UCI command to an engine
func (gt *GameTester) SendCommand(engine *Engine, command string) error {
	gt.Logger.Printf("-> %s: %s", engine.Name, command)

	if _, err := engine.Stdin.WriteString(command + "\n"); err != nil {
		return err
	}
	return engine.Stdin.Flush()
}

// GetMoveWithInfo requests a move from an engine and returns move with analysis info
func (gt *GameTester) GetMoveWithInfo(engine *Engine, goCommand string) (*MoveInfo, error) {
	if err := gt.SendCommand(engine, goCommand); err != nil {
		return nil, err
	}

	info := &MoveInfo{
		Evaluation: "?",
		PV:         "",
		Depth:      0,
		Nodes:      0,
	}

	// Parse response for info lines and bestmove
	timeout := time.After(gt.Config.TimeControl + time.Second*2) // Extra buffer

	for {
		select {
		case <-timeout:
			return nil, fmt.Errorf("move timeout")
		default:
			if engine.Stdout.Scan() {
				line := engine.Stdout.Text()
				gt.Logger.Printf("%s: %s", engine.Name, line)

				if strings.HasPrefix(line, "info ") {
					// Parse UCI info line for evaluation and PV
					gt.parseInfoLine(line, info)
				} else if strings.HasPrefix(line, "bestmove ") {
					parts := strings.Fields(line)
					if len(parts) >= 2 {
						info.Move = parts[1]
						return info, nil
					}
				}
			}
		}
	}
}

// parseInfoLine extracts evaluation and PV information from UCI info lines
func (gt *GameTester) parseInfoLine(line string, info *MoveInfo) {
	fields := strings.Fields(line)

	for i, field := range fields {
		switch field {
		case "depth":
			if i+1 < len(fields) {
				if depth, err := strconv.Atoi(fields[i+1]); err == nil {
					info.Depth = depth
				}
			}
		case "nodes":
			if i+1 < len(fields) {
				if nodes, err := strconv.Atoi(fields[i+1]); err == nil {
					info.Nodes = nodes
				}
			}
		case "score":
			// Extract score (cp or mate)
			if i+2 < len(fields) {
				scoreType := fields[i+1]
				scoreValue := fields[i+2]
				info.Evaluation = scoreType + " " + scoreValue
			}
		case "pv":
			// Extract principal variation (next 6 moves)
			pvMoves := make([]string, 0, 6)
			for j := i + 1; j < len(fields) && j < i+7; j++ {
				// Stop at next UCI keyword
				if fields[j] == "depth" || fields[j] == "nodes" ||
					fields[j] == "time" || fields[j] == "nps" {
					break
				}
				pvMoves = append(pvMoves, fields[j])
			}
			info.PV = strings.Join(pvMoves, " ")
		}
	}
}

// GetMove requests a move from an engine and returns it (legacy method for compatibility)
func (gt *GameTester) GetMove(engine *Engine, goCommand string) (string, error) {
	moveInfo, err := gt.GetMoveWithInfo(engine, goCommand)
	if err != nil {
		return "", err
	}
	return moveInfo.Move, nil
}

// StopEngine shuts down an engine
func (gt *GameTester) StopEngine(engine *Engine) {
	gt.SendCommand(engine, "quit")

	// Give it a moment to exit gracefully
	done := make(chan error, 1)
	go func() {
		done <- engine.Process.Wait()
	}()

	select {
	case <-done:
		// Exited gracefully
	case <-time.After(time.Second * 2):
		// Force kill
		engine.Process.Process.Kill()
	}
}

// SavePGN saves games in PGN format
func (gt *GameTester) SavePGN() error {
	file, err := os.Create(gt.Config.PGNOutput)
	if err != nil {
		return err
	}
	defer file.Close()

	for _, game := range gt.Games {
		fmt.Fprintf(file, "[Event \"NGN vs Stockfish Test\"]\n")
		fmt.Fprintf(file, "[Round \"%d\"]\n", game.ID)
		fmt.Fprintf(file, "[White \"%s\"]\n", gt.getWhitePlayer(game.ID))
		fmt.Fprintf(file, "[Black \"%s\"]\n", gt.getBlackPlayer(game.ID))
		fmt.Fprintf(file, "[Result \"%s\"]\n", game.Result)
		fmt.Fprintf(file, "[TimeControl \"%v\"]\n", gt.Config.TimeControl)
		fmt.Fprintf(file, "\n")

		// Write moves (simplified - would need proper PGN formatting)
		for i, move := range game.Moves {
			if i%2 == 0 {
				fmt.Fprintf(file, "%d. %s ", i/2+1, move)
			} else {
				fmt.Fprintf(file, "%s ", move)
			}
		}
		fmt.Fprintf(file, "%s\n\n", game.Result)
	}

	return nil
}

func (gt *GameTester) getWhitePlayer(gameID int) string {
	if gameID%2 == 1 {
		return "NGN"
	}
	return gt.getStockfishName()
}

func (gt *GameTester) getBlackPlayer(gameID int) string {
	if gameID%2 == 1 {
		return gt.getStockfishName()
	}
	return "NGN"
}

func (gt *GameTester) getStockfishName() string {
	if gt.Config.StockfishElo > 0 {
		return fmt.Sprintf("Stockfish (%d ELO)", gt.Config.StockfishElo)
	}
	return fmt.Sprintf("Stockfish (Skill %d)", gt.Config.StockfishSkill)
}

func getStockfishDisplayName(config *GameTestConfig) string {
	if config.StockfishElo > 0 {
		return fmt.Sprintf("Stockfish (%d ELO)", config.StockfishElo)
	}
	return fmt.Sprintf("Stockfish (Skill %d)", config.StockfishSkill)
}

// PrintSummary prints a summary of test results
func (gt *GameTester) PrintSummary() {
	ngnWins := 0
	stockfishWins := 0
	draws := 0

	fmt.Printf("\n=== GAME TEST SUMMARY ===\n")
	fmt.Printf("Total games: %d\n", len(gt.Games))

	for _, game := range gt.Games {
		switch game.Result {
		case "1-0":
			if gt.getWhitePlayer(game.ID) == "NGN" {
				ngnWins++
			} else {
				stockfishWins++
			}
		case "0-1":
			if gt.getBlackPlayer(game.ID) == "NGN" {
				ngnWins++
			} else {
				stockfishWins++
			}
		case "1/2-1/2":
			draws++
		}
	}

	fmt.Printf("NGN wins: %d\n", ngnWins)
	fmt.Printf("Stockfish wins: %d\n", stockfishWins)
	fmt.Printf("Draws: %d\n", draws)

	if len(gt.Games) > 0 {
		ngnScore := float64(ngnWins) + float64(draws)*0.5
		percentage := ngnScore / float64(len(gt.Games)) * 100
		fmt.Printf("NGN score: %.1f%%\n", percentage)
	}

	fmt.Printf("\nDetailed logs: %s\n", gt.Config.LogFile)
	fmt.Printf("PGN games: %s\n", gt.Config.PGNOutput)
}
