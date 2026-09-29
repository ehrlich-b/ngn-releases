package main

import (
	"flag"
	"fmt"
	"math"
	"runtime"
	"strings"
	"time"

	"github.com/ehrlich-b/ngn/internal/rating"
	"github.com/ehrlich-b/ngn/internal/uci"
)

// Assessment levels
const (
	Quick    = 1  // 1 minute - 4 games at 0.5s/move
	Standard = 5  // 5 minutes - 10 games at 1s/move
	Thorough = 10 // 10 minutes - 20 games at 1.5s/move
)

type Config struct {
	NGNPath       string
	StockfishPath string
	Level         int
	Verbose       bool
	TargetELO     int
	MinGames      int
	MaxGames      int
	MovetimeMs    int     // NGN time per move
	SFMovetimeMs  int     // Stockfish time per move (asymmetric); 0 = same as MovetimeMs
	StopWidthELO  float64 // stop early when 95% ELO CI width ≤ this
	LowPower      bool    // route to macOS E-cores via taskpolicy -b
}

type GameResult struct {
	NGNWins    int
	SFWins     int
	Draws      int
	TotalGames int
}

func main() {
	level := flag.Int("level", 0, "Assessment level (1/5/10). 0 = use explicit flags below.")
	verbose := flag.Bool("v", false, "Verbose output")
	targetELO := flag.Int("elo", 1320, "Stockfish ELO target")
	games := flag.Int("games", 0, "Override max games (0 = use level default)")
	movetimeMs := flag.Int("movetime", 0, "NGN ms per move (0 = use level default)")
	sfMovetimeMs := flag.Int("sfmovetime", 0, "Stockfish ms per move (0 = same as -movetime)")
	minGames := flag.Int("min", 6, "Minimum games before early-stop can fire")
	stopWidth := flag.Float64("ci", 200, "Stop when 95% ELO CI width ≤ this (0 = never early-stop)")
	lowPower := flag.Bool("lowpower", true, "Route engines to macOS E-cores (taskpolicy -b)")
	flag.Parse()

	// Level presets kept for backward compat. Lighter defaults than before.
	var maxGames, moveMs int
	switch *level {
	case 1:
		maxGames, moveMs = 8, 250
	case 5:
		maxGames, moveMs = 20, 400
	case 10:
		maxGames, moveMs = 40, 600
	default:
		maxGames, moveMs = 24, 250 // sensible cool default
	}
	if *games > 0 {
		maxGames = *games
	}
	if *movetimeMs > 0 {
		moveMs = *movetimeMs
	}
	if *minGames > maxGames {
		*minGames = maxGames
	}

	sfMs := *sfMovetimeMs
	if sfMs == 0 {
		sfMs = moveMs
	}

	config := &Config{
		NGNPath:       "./build/ngn",
		StockfishPath: "./stockfish/stockfish-ubuntu-x86-64-avx2",
		Level:         *level,
		Verbose:       *verbose,
		TargetELO:     *targetELO,
		MinGames:      *minGames,
		MaxGames:      maxGames,
		MovetimeMs:    moveMs,
		SFMovetimeMs:  sfMs,
		StopWidthELO:  *stopWidth,
		LowPower:      *lowPower,
	}

	fmt.Printf("NGN ELO Assessment (Bayesian early-stop)\n")
	fmt.Printf("========================================\n")
	if config.MovetimeMs == config.SFMovetimeMs {
		fmt.Printf("Stockfish target: %d ELO | movetime: %dms (both)\n", config.TargetELO, config.MovetimeMs)
	} else {
		fmt.Printf("Stockfish target: %d ELO | NGN: %dms, SF: %dms\n",
			config.TargetELO, config.MovetimeMs, config.SFMovetimeMs)
	}
	fmt.Printf("Games: min=%d max=%d | stop at 95%% ELO CI ≤ %.0f\n",
		config.MinGames, config.MaxGames, config.StopWidthELO)
	if config.LowPower && runtime.GOOS == "darwin" {
		fmt.Printf("Power: taskpolicy -b (E-cores, low power)\n")
	} else {
		fmt.Printf("Power: normal\n")
	}
	fmt.Println()

	result := runGamesBayesian(config)
	displayResults(result, config.TargetELO)
}

// runGamesBayesian plays games until CI is tight enough or max games reached.
// Colors alternate each game.
func runGamesBayesian(config *Config) GameResult {
	result := GameResult{}
	ngnTime := float64(config.MovetimeMs) / 1000.0
	sfTime := float64(config.SFMovetimeMs) / 1000.0

	for g := 1; g <= config.MaxGames; g++ {
		fmt.Printf("G%2d ", g)
		ngnWhite := g%2 == 1

		start := time.Now()
		winner, reason := playGame(config, ngnWhite, ngnTime, sfTime)
		elapsed := time.Since(start).Truncate(time.Second)

		color := "B"
		if ngnWhite {
			color = "W"
		}

		switch winner {
		case "ngn":
			result.NGNWins++
		case "stockfish":
			result.SFWins++
		case "draw":
			result.Draws++
		}
		result.TotalGames++

		score := float64(result.NGNWins) + 0.5*float64(result.Draws)
		eloEst, eloLo, eloHi := rating.PerfRating(score, float64(result.TotalGames), float64(config.TargetELO))
		ciWidth := eloHi - eloLo

		fmt.Printf("[%s] %-10s %-12s | W%d D%d L%d  est %.0f  CI [%.0f, %.0f]  (%v)\n",
			color, winner, reason,
			result.NGNWins, result.Draws, result.SFWins,
			eloEst, eloLo, eloHi, elapsed)

		// Early stop: CI tight enough AND past min games.
		if config.StopWidthELO > 0 &&
			result.TotalGames >= config.MinGames &&
			!math.IsInf(ciWidth, 0) &&
			ciWidth <= config.StopWidthELO {
			fmt.Printf("\nEarly stop: ELO CI width %.0f ≤ %.0f after %d games\n",
				ciWidth, config.StopWidthELO, result.TotalGames)
			break
		}
	}
	return result
}

func playGame(config *Config, ngnWhite bool, ngnTime, sfTime float64) (string, string) {
	ngn := uci.Start(config.NGNPath, "NGN", config.LowPower)
	if ngn == nil {
		return "error", "failed to start NGN"
	}
	defer uci.Stop(ngn)

	sf := uci.Start(config.StockfishPath, "Stockfish", config.LowPower)
	if sf == nil {
		return "error", "failed to start Stockfish"
	}
	defer uci.Stop(sf)

	uci.Send(sf, "setoption name UCI_LimitStrength value true")
	uci.Send(sf, fmt.Sprintf("setoption name UCI_Elo value %d", config.TargetELO))

	moves := []string{}
	maxMoves := 200

	for moveNum := 0; moveNum < maxMoves; moveNum++ {
		isWhiteMove := moveNum%2 == 0
		var currentEngine *uci.Engine
		var timePerMove float64

		if (isWhiteMove && ngnWhite) || (!isWhiteMove && !ngnWhite) {
			currentEngine = ngn
			timePerMove = ngnTime
		} else {
			currentEngine = sf
			timePerMove = sfTime
		}

		posCmd := "position startpos"
		if len(moves) > 0 {
			posCmd += " moves " + strings.Join(moves, " ")
		}
		uci.Send(currentEngine, posCmd)

		goCmd := fmt.Sprintf("go movetime %d", int(timePerMove*1000))
		// Force-stop at budget + 1s grace, hard cap budget + 30s (overshoot-safe).
		mt := time.Duration(timePerMove * float64(time.Second))
		move := uci.GetMove(currentEngine, goCmd, mt+30*time.Second, mt+1*time.Second)

		if move == "" || move == "(none)" {
			// Game over — side to move can't move. If it's NGN, NGN lost.
			if currentEngine == ngn {
				return "stockfish", "checkmate"
			}
			return "ngn", "checkmate"
		}

		moves = append(moves, move)
		if config.Verbose {
			fmt.Printf("  %d. %s\n", len(moves), move)
		}
	}

	return "draw", "max_moves"
}

func displayResults(result GameResult, targetELO int) {
	fmt.Println()
	fmt.Println("=== RESULTS ===")
	fmt.Printf("Games played: %d\n", result.TotalGames)
	fmt.Printf("NGN wins: %d\n", result.NGNWins)
	fmt.Printf("Stockfish wins: %d\n", result.SFWins)
	fmt.Printf("Draws: %d\n", result.Draws)

	if result.TotalGames == 0 {
		return
	}
	score := float64(result.NGNWins) + 0.5*float64(result.Draws)
	p := score / float64(result.TotalGames)
	est, lo, hi := rating.PerfRating(score, float64(result.TotalGames), float64(targetELO))

	fmt.Printf("\nScore: %.1f/%d  (%.1f%%)\n", score, result.TotalGames, p*100)
	fmt.Printf("Performance vs SF%d slider: %.0f   95%% CI: [%.0f, %.0f]  (width %.0f)\n",
		targetELO, est, lo, hi, hi-lo)
	fmt.Println("NOTE: relative to Stockfish's UCI_Elo slider (inflated ~+400 vs CCRL), NOT an absolute rating.")
	fmt.Println("      For NGN's absolute CCRL rating use `make gauntlet` (the Blunder anchors).")

	// Relative interpretive hint vs the SF slider setting (not an absolute claim).
	switch {
	case lo > float64(targetELO)+50:
		fmt.Printf("Verdict: outscoring the SF %d slider setting (raise target)\n", targetELO)
	case hi < float64(targetELO)-50:
		fmt.Printf("Verdict: below the SF %d slider setting (lower target)\n", targetELO)
	default:
		fmt.Printf("Verdict: roughly matched with the SF %d slider setting\n", targetELO)
	}
}
