// Command texel runs Texel tuning of NGN's piece-square tables.
//
//	texel -mode gendata -games 1500 -nodes 8000 -out data.txt
//	texel -mode tune    -data data.txt -out tuned_psts.go
//
// gendata self-plays games in-process through one compatibility SearchEngine
// receiver. Run several process-isolated instances under `taskpolicy -b` and
// concatenate for more data. It records
// quiet, non-check positions labeled with the game's final result from White's
// perspective. tune loads that dataset, calibrates K, runs coordinate descent
// over the PSTs, and dumps the tuned tables as Go source to paste into eval.go.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/ehrlich-b/ngn/engine"
)

const startposFEN = "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"

func main() {
	mode := flag.String("mode", "", "gendata | tune | gradient | audit | dump")
	// gendata flags
	games := flag.Int("games", 1500, "gendata: number of self-play games")
	nodes := flag.Int("nodes", 8000, "gendata: fixed node budget per move")
	prefix := flag.Int("prefix", 8, "gendata: random plies before recording (opening diversity)")
	maxPlies := flag.Int("maxplies", 240, "gendata: max plies before scoring the game a draw")
	seed := flag.Int64("seed", 0, "gendata: RNG seed (0 = time-based)")
	// tune flags
	data := flag.String("data", "", "tune/gradient: dataset ('<FEN> <result>' lines or result-labeled EPD)")
	corpus := flag.String("corpus", "", "gradient/audit: comma-separated game-grouped JSONL files")
	splitSeed := flag.String("split-seed", "ngn-classical-v2", "grouped corpus: deterministic opening partition seed")
	report := flag.String("report", "", "grouped corpus: required JSON audit/fit report path")
	passes := flag.Int("passes", 8, "tune: max coordinate-descent passes per step size")
	target := flag.String("target", "weights", "tune: weights (10 eval-term scalars) | pst (768 PST entries + weights) | full (material + PST + weights)")
	// gradient flags
	epochs := flag.Int("epochs", 300, "gradient: max Adam epochs")
	lr := flag.Float64("lr", 1.0, "gradient: Adam learning rate (~centipawn step ceiling/epoch)")
	patience := flag.Int("patience", 30, "gradient: early-stop after this many epochs without held-out improvement (0 = off)")
	split := flag.Float64("split", 0.1, "legacy flat gradient: validation fraction for early stopping")
	l2 := flag.Float64("l2", 0.0, "gradient: L2 pull toward original PeSTO values (regularizes weakly-supported PST cells; e.g. 0.01)")
	// shared
	out := flag.String("out", "", "output file (default stdout)")
	flag.Parse()

	switch *mode {
	case "gendata":
		genData(*games, *nodes, *prefix, *maxPlies, *seed, *out)
	case "tune":
		tune(*data, *passes, *target, *out)
	case "gradient":
		if *corpus != "" {
			if *data != "" {
				fmt.Fprintln(os.Stderr, "choose one of -corpus or legacy -data")
				os.Exit(2)
			}
			if err := groupedGradient(*corpus, *splitSeed, *out, *report, fitConfig{*epochs, *patience, *lr, *l2}, false); err != nil {
				fmt.Fprintln(os.Stderr, "gradient:", err)
				os.Exit(1)
			}
		} else {
			gradientTune(*data, *epochs, *lr, *patience, *split, *l2, *seed, *out)
		}
	case "audit":
		if err := groupedGradient(*corpus, *splitSeed, "", *report, fitConfig{}, true); err != nil {
			fmt.Fprintln(os.Stderr, "audit:", err)
			os.Exit(1)
		}
	case "dump":
		dumpWeights(*out)
	default:
		fmt.Fprintln(os.Stderr, "specify -mode gendata, -mode tune, -mode gradient, or -mode dump")
		flag.Usage()
		os.Exit(2)
	}
}

// dumpWeights emits the full current model as versioned JSON, also used by
// gradient. Legacy coordinate descent still emits its original Go source subset.
func dumpWeights(out string) {
	w := os.Stdout
	if out != "" {
		f, err := os.Create(out)
		if err != nil {
			fmt.Fprintln(os.Stderr, "create:", err)
			os.Exit(1)
		}
		defer f.Close()
		w = f
	}
	bw := bufio.NewWriter(w)
	defer bw.Flush()
	fmt.Fprint(bw, engine.FormatTexelModelExport())
}

// genData self-plays games and writes labeled quiet positions.
func genData(games, nodes, prefix, maxPlies int, seed int64, out string) {
	if seed == 0 {
		seed = time.Now().UnixNano()
	}
	rng := rand.New(rand.NewSource(seed))

	w := os.Stdout
	if out != "" {
		f, err := os.Create(out)
		if err != nil {
			fmt.Fprintln(os.Stderr, "create:", err)
			os.Exit(1)
		}
		defer f.Close()
		w = f
	}
	bw := bufio.NewWriter(w)
	defer bw.Flush()

	start := time.Now()
	totalPos := 0
	for g := 0; g < games; g++ {
		fens, result := playGame(nodes, prefix, maxPlies, rng)
		for _, fen := range fens {
			fmt.Fprintf(bw, "%s %.1f\n", fen, result)
		}
		totalPos += len(fens)
		if (g+1)%50 == 0 {
			bw.Flush()
			fmt.Fprintf(os.Stderr, "  %d/%d games, %d positions, %s elapsed\n",
				g+1, games, totalPos, time.Since(start).Round(time.Second))
		}
	}
	fmt.Fprintf(os.Stderr, "done: %d games, %d positions in %s\n",
		games, totalPos, time.Since(start).Round(time.Second))
}

// playGame plays one self-play game at fixed nodes/move after a random opening
// prefix, returning the recorded quiet (non-check, post-prefix) position FENs and
// the game result from White's perspective (1.0 / 0.5 / 0.0).
func playGame(nodes, prefix, maxPlies int, rng *rand.Rand) ([]string, float64) {
	// Fresh per-game search state (ucinewgame semantics).
	engine.ClearHash()
	engine.ClearHistoryTable()
	engine.ClearKillerMoves()
	engine.ClearCounterMoves()
	engine.SetLastMovePlayed(engine.EmptyMove)

	pos, _ := engine.ParseFEN(startposFEN)
	var fens []string
	result := 0.5 // draw if the game reaches maxPlies

	for ply := 0; ply < maxPlies; ply++ {
		legal := engine.GenerateLegalMoves(pos)
		if len(legal) == 0 {
			if pos.IsInCheck() {
				// Side to move is checkmated and loses.
				if pos.Turn() == engine.White {
					result = 0.0
				} else {
					result = 1.0
				}
			} // else stalemate -> 0.5
			break
		}
		if pos.IsFIDEDrawRule() {
			break // threefold / 50-move -> 0.5
		}

		if ply < prefix {
			// Random opening move for diversity.
			pos.GameMakeMove(legal[rng.Intn(len(legal))])
			continue
		}

		// Record only quiet positions (no pending tactic): a hanging piece would make
		// the static eval meaningless against the game result. IsQuietPosition runs a
		// quiescence probe — the prior not-in-check-only filter let tactics through.
		if engine.IsQuietPosition(pos) {
			fens = append(fens, engine.GenerateFEN(pos))
		}

		engine.SetMaxNodes(uint64(nodes))
		info := engine.Search(pos, engine.MaximumDepth)
		engine.SetMaxNodes(0)
		if info.BestMove == engine.EmptyMove {
			break
		}
		pos.GameMakeMove(info.BestMove)
	}
	return fens, result
}

// tune loads the dataset, calibrates K, runs a decreasing-step coordinate-descent
// schedule over the chosen target, and writes the tuned values as Go source.
//
// target=weights tunes only the 10 hand-set eval-term scalars; target=pst tunes
// the 768 raw PeSTO piece-square entries jointly with those scalars (material is
// left fixed — a uniform shift of a piece's 64 squares already spans it — and the
// black tables are rebuilt by mirroring after every mutation via RebuildPST).
func tune(data string, passes int, target, out string) {
	if data == "" {
		fmt.Fprintln(os.Stderr, "tune: -data is required")
		os.Exit(2)
	}
	fmt.Fprintf(os.Stderr, "loading %s ...\n", data)
	samples, nDup, err := engine.LoadTexelSamples(data)
	if err != nil {
		fmt.Fprintln(os.Stderr, "load:", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "loaded %d unique positions (dropped %d duplicates)\n", len(samples), nDup)
	if len(samples) == 0 {
		os.Exit(1)
	}

	k := engine.TexelFindK(samples)
	fmt.Fprintf(os.Stderr, "calibrated K = %.4f\n", k)

	weights := engine.TexelWeights()
	weightPtrs := make([]*int, len(weights))
	for i := range weights {
		weightPtrs[i] = weights[i].Ptr
	}

	var ptrs []*int
	var rebuild func()
	var dump func() string
	switch target {
	case "weights":
		ptrs = weightPtrs
		dump = func() string { return engine.FormatTexelWeights(weights) }
	case "pst":
		ptrs = append(engine.TexelPSTParams(), weightPtrs...)
		rebuild = engine.RebuildPST
		dump = func() string {
			return engine.FormatPSTTables() + "// eval.go tunable weights:\n" + engine.FormatTexelWeights(weights)
		}
	case "full":
		// Material (10) + 768 PST entries + 10 weights, tuned jointly. Material
		// lets CD move a piece's value directly instead of via 64 per-square steps.
		ptrs = append(engine.TexelMaterialParams(), engine.TexelPSTParams()...)
		ptrs = append(ptrs, weightPtrs...)
		rebuild = engine.RebuildPST
		dump = func() string {
			return engine.FormatMaterialValues() + "\n" + engine.FormatPSTTables() + "// eval.go tunable weights:\n" + engine.FormatTexelWeights(weights)
		}
	default:
		fmt.Fprintf(os.Stderr, "tune: unknown -target %q (want weights|pst|full)\n", target)
		os.Exit(2)
	}
	fmt.Fprintf(os.Stderr, "tuning %d params (target=%s)\n", len(ptrs), target)
	fmt.Fprintf(os.Stderr, "weights before:\n%s", engine.FormatTexelWeights(weights))

	start := time.Now()
	for _, step := range []int{8, 4, 2, 1} {
		fmt.Fprintf(os.Stderr, "--- coordinate descent step=%d ---\n", step)
		if _, err := engine.TryTexelTune(samples, ptrs, k, step, passes, rebuild, func(pass int, mse float64, moved int) {
			fmt.Fprintf(os.Stderr, "  step=%d pass=%d MSE=%.6f moved=%d  (%s)\n",
				step, pass, mse, moved, time.Since(start).Round(time.Second))
		}); err != nil {
			fmt.Fprintln(os.Stderr, "tune:", err)
			os.Exit(1)
		}
	}
	fmt.Fprintf(os.Stderr, "weights after:\n%s", engine.FormatTexelWeights(weights))

	w := os.Stdout
	if out != "" {
		f, err := os.Create(out)
		if err != nil {
			fmt.Fprintln(os.Stderr, "create:", err)
			os.Exit(1)
		}
		defer f.Close()
		w = f
	}
	fmt.Fprint(w, dump())
	fmt.Fprintf(os.Stderr, "tuning done in %s\n", time.Since(start).Round(time.Second))
}

// gradientTune fits the full eval parameter vector (material + 768 PST cells + the
// scalar weights) by Adam gradient descent over cached eval traces, holding out a
// fraction for early stopping, and writes the tuned values as Go source. This is
// the gradient solver for the same Texel objective the coordinate-descent `tune`
// minimizes — closed-form steps over all parameters jointly, ~100-1000x cheaper.
func gradientTune(data string, epochs int, lr float64, patience int, split, l2 float64, seed int64, out string) {
	if data == "" {
		fmt.Fprintln(os.Stderr, "gradient: -data is required")
		os.Exit(2)
	}
	fmt.Fprintf(os.Stderr, "loading %s ...\n", data)
	samples, nDup, err := engine.LoadTexelSamples(data)
	if err != nil {
		fmt.Fprintln(os.Stderr, "load:", err)
		os.Exit(1)
	}
	if len(samples) == 0 {
		fmt.Fprintln(os.Stderr, "no samples")
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "loaded %d unique positions (dropped %d duplicates)\n", len(samples), nDup)

	if seed == 0 {
		seed = time.Now().UnixNano()
	}
	// Legacy flat rows lack game/opening identity and cannot establish independent
	// validation. The grouped -corpus route is required for the v2 experiment.
	train, test := engine.SplitTrainTest(samples, split, seed)
	fmt.Fprintf(os.Stderr, "split: %d train / %d test (seed %d)\n", len(train), len(test), seed)

	k := engine.TexelFindK(train)
	fmt.Fprintf(os.Stderr, "calibrated K = %.4f on train\n", k)
	weights := engine.TexelWeights()
	fmt.Fprintf(os.Stderr, "weights before:\n%s", engine.FormatTexelWeights(weights))

	start := time.Now()
	trainMSE, testMSE, err := engine.TryTexelGradientTune(train, test, engine.TexelGradientConfig{
		K: k, LR: lr, Epochs: epochs, Patience: patience, L2: l2,
		Progress: func(epoch int, trMSE, teMSE float64) {
			if epoch == 1 || epoch%10 == 0 {
				fmt.Fprintf(os.Stderr, "  epoch %d  trainMSE=%.6f  testMSE=%.6f  (%s)\n",
					epoch, trMSE, teMSE, time.Since(start).Round(time.Second))
			}
		},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "gradient:", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "final integer-eval MSE: train=%.6f test=%.6f\n", trainMSE, testMSE)
	fmt.Fprintf(os.Stderr, "weights after:\n%s", engine.FormatTexelWeights(weights))

	w := os.Stdout
	if out != "" {
		f, err := os.Create(out)
		if err != nil {
			fmt.Fprintln(os.Stderr, "create:", err)
			os.Exit(1)
		}
		defer f.Close()
		w = f
	}
	fmt.Fprint(w, engine.FormatTexelModelExport())
	fmt.Fprintf(os.Stderr, "gradient tuning done in %s\n", time.Since(start).Round(time.Second))
}
