package engine

import (
	"math"
	"math/rand"
	"sort"
	"testing"
)

// texelGateFENs span every eval term and the full phase range (0..24): startpos,
// a castled middlegame (king safety), rook/pawn and K+P endgames (king activity),
// doubled/isolated/passed pawns, an open-file rook, a knight outpost, and a
// reduced-material position at phase 9 to exercise the lowered king-safety gate.
var texelGateFENs = []string{
	"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
	"r1bq1rk1/pp2bppp/2n1pn2/2pp4/3P4/2N1PN2/PPP1BPPP/R1BQ1RK1 w - - 0 8",
	"8/5pk1/6p1/8/8/1R6/5PK1/8 w - - 0 1",
	"8/8/8/4k3/8/4P3/4K3/8 w - - 0 1",
	"4k3/8/8/8/8/2P1P3/2P5/4K3 w - - 0 1",
	"4k3/8/8/3P4/8/8/8/4K3 w - - 0 1",
	"4k3/8/8/8/8/8/4P3/R3K3 w - - 0 1",
	"4k3/1p6/8/2Np4/8/8/8/4K3 w - - 0 1",
	"1q2k3/8/8/8/8/8/8/1Q2K2N w - - 0 1",
	"r2q1rk1/1b1nbppp/p2ppn2/1p6/3NPP2/2N1B3/PPPQB1PP/R4RK1 w - - 0 12",
	// Phase exactly 6 (Q=4 + N+B=2), asymmetric king shields (White f2/g2/h2 intact,
	// Black f5/g5/h5 advanced) so king-safety is non-zero — exercises the king-safety
	// gate boundary the live eval opens at phase>=6 (C4).
	"2b3k1/pp6/5n2/5ppp/8/8/PP3PPP/3Q2K1 w - - 0 1",
}

// TestEvalTraceReproducesEval is the Phase (a) gate: the cached trace must
// reproduce evaluateUnsafe bit-for-bit (and its PeSTO occupancy must reproduce the
// accumulator), which makes texelMSE-from-traces identical to texelMSE. The tuner
// always runs with the pawn cache disabled, so we match that regime here.
func TestEvalTraceReproducesEval(t *testing.T) {

	for _, fen := range texelGateFENs {
		pos, err := ParseFEN(fen)
		if err != nil {
			t.Fatalf("ParseFEN(%q): %v", fen, err)
		}
		b := pos.Board
		b.recomputeAccumulator()

		want := evaluateUnsafe(&b)
		tr := buildEvalTrace(&b)
		got := reconstructEvalInt(&tr)
		if got != want {
			t.Errorf("reconstructEvalInt=%d, evaluateUnsafe=%d (phase %d) for %q", got, want, tr.Phase, fen)
		}

		// The per-piece occupancy must rebuild the accumulator exactly (guards the
		// white sq^56 / black sq flip convention independently of term cancellation).
		texelRawTables()
		mg, eg := 0, 0
		for _, pc := range tr.Pieces {
			pt := PieceType(pc.Pt)
			s := int(pc.Sign)
			mg += s * (pestoMGMaterial[pt] + texelRawMG[pt][pc.Cell])
			eg += s * (pestoEGMaterial[pt] + texelRawEG[pt][pc.Cell])
		}
		if mg != b.accMG || eg != b.accEG {
			t.Errorf("decomposed (mg=%d,eg=%d) != accumulator (mg=%d,eg=%d) for %q", mg, eg, b.accMG, b.accEG, fen)
		}
	}
}

// TestEvalTraceParityDiverseAndPerturbedWeights guards against a trace that happens
// to match only the default weights or a hand-picked corpus. The deterministic walks
// cover positions outside texelGateFENs, and changing every live coordinate proves
// that the trace remains the evaluator's model rather than a default-value snapshot.
func TestEvalTraceParityDiverseAndPerturbedWeights(t *testing.T) {

	positions := make([]Position, 0, 1600)
	rng := rand.New(rand.NewSource(2600))
	for game := 0; game < 16; game++ {
		pos, err := ParseFEN("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1")
		if err != nil {
			t.Fatal(err)
		}
		for ply := 0; ply < 100; ply++ {
			positions = append(positions, *pos.Copy())
			moves := GenerateLegalMoves(pos)
			if len(moves) == 0 {
				break
			}
			pos.GameMakeMove(moves[rng.Intn(len(moves))])
		}
	}

	traces := make([]evalTrace, len(positions))
	check := func(label string) {
		t.Helper()
		backwardFeatures := 0
		for i := range positions {
			b := positions[i].Board
			b.recomputeAccumulator()
			if label == "default" {
				traces[i] = buildEvalTrace(&b)
			}
			tr := traces[i]
			if tr.Backward != 0 {
				backwardFeatures++
			}
			if got, want := reconstructEvalInt(&tr), evaluateUnsafe(&b); got != want {
				t.Fatalf("%s position %d: trace=%d live=%d FEN=%s", label, i, got, want, GenerateFEN(&positions[i]))
			}
		}
		if backwardFeatures == 0 {
			t.Fatal("diverse corpus did not exercise the backward-pawn feature")
		}
	}

	check("default")
	ptrs := TexelFullParams()
	saved := make([]int, len(ptrs))
	for i, p := range ptrs {
		saved[i] = *p
		delta := (i%11 + 1) * 7
		if i%2 != 0 {
			delta = -delta
		}
		*p += delta
	}
	RebuildPST()
	defer func() {
		for i, p := range ptrs {
			*p = saved[i]
		}
		RebuildPST()
	}()
	check("perturbed")
}

// TestEvalTraceFloatFidelity confirms the float linear model (used for gradients)
// matches the integer eval to within the expected truncation residual: one /24
// taper floor plus six /100 term floors, < ~8cp. A large gap would mean the float
// model and the engine eval have structurally diverged.
func TestEvalTraceFloatFidelity(t *testing.T) {

	ptrs := TexelFullParams()
	if len(ptrs) != texelNumParams {
		t.Fatalf("TexelFullParams len=%d, want %d", len(ptrs), texelNumParams)
	}
	w := make([]float64, len(ptrs))
	for i, p := range ptrs {
		w[i] = float64(*p)
	}

	maxResidual := 0.0
	for _, fen := range texelGateFENs {
		pos, err := ParseFEN(fen)
		if err != nil {
			t.Fatalf("ParseFEN(%q): %v", fen, err)
		}
		b := pos.Board
		b.recomputeAccumulator()
		tr := buildEvalTrace(&b)
		residual := math.Abs(evalTraceFloat(&tr, w) - float64(reconstructEvalInt(&tr)))
		if residual > maxResidual {
			maxResidual = residual
		}
	}
	if maxResidual > 8.0 {
		t.Errorf("float/int eval residual %.3f exceeds expected truncation bound (~8cp)", maxResidual)
	}
	t.Logf("max float/int residual: %.3fcp", maxResidual)
}

func TestTexelV2ExportReloadAndCoverage(t *testing.T) {
	x := ExportTexelModel()
	if x.Version != TexelModelVersion || len(x.Values) != texelNumParams || len(x.Names) != texelNumParams {
		t.Fatalf("bad v2 export version=%q values=%d names=%d", x.Version, len(x.Values), len(x.Names))
	}
	if x.Names[texelScalarBase+5] != "mobilityWeight" || x.Names[texelMobilityMGBase+mobilityCellCount-1] != "mg_mobility_queen_27" {
		t.Fatal("legacy reservations or high mobility cell missing from named export")
	}
	saved := append([]int(nil), x.Values...)
	x.Values[texelMobilityMGBase+mobilityCellCount-1]++
	x.Values[texelThreatBase+5]++
	if err := ApplyTexelModel(x); err != nil {
		t.Fatal(err)
	}
	got := ExportTexelModel()
	if got.Values[texelMobilityMGBase+mobilityCellCount-1] != x.Values[texelMobilityMGBase+mobilityCellCount-1] || got.Values[texelThreatBase+5] != x.Values[texelThreatBase+5] {
		t.Fatal("exported v2 parameters were silently dropped")
	}
	x.Values = saved
	if err := ApplyTexelModel(x); err != nil {
		t.Fatal(err)
	}
}

func TestTexelV2NewParameterGradients(t *testing.T) {
	ptrs := TexelFullParams()
	w := make([]float64, len(ptrs))
	for i, p := range ptrs {
		w[i] = float64(*p)
	}
	for j := texelMobilityMGBase; j < texelNumParams; j++ {
		tr := evalTrace{Phase: 12, Result: 0.37}
		if j < texelMobilityEGBase {
			tr.MobilityCounts[j-texelMobilityMGBase] = 1
		} else if j < texelThreatBase {
			tr.MobilityCounts[j-texelMobilityEGBase] = 1
		} else {
			tr.ThreatCounts[j-texelThreatBase] = 1
		}
		_, g := texelTraceGradient([]evalTrace{tr}, w, 1, true)
		wp, wm := append([]float64(nil), w...), append([]float64(nil), w...)
		wp[j]++
		wm[j]--
		mp, _ := texelTraceGradient([]evalTrace{tr}, wp, 1, false)
		mm, _ := texelTraceGradient([]evalTrace{tr}, wm, 1, false)
		if math.Abs(g[j]-(mp-mm)/2) > 1e-8 {
			t.Fatalf("new gradient %d (%s) mismatch", j, TexelParameterNames()[j])
		}
	}
}

// gateSamples builds a small labeled dataset from the gate FENs with varied
// results, so the non-PeSTO terms and asymmetric occupancy carry real gradient.
func gateSamples(t *testing.T) []TexelSample {
	t.Helper()
	results := []float64{0.5, 1.0, 0.0, 0.0, 1.0, 1.0, 0.5, 0.0, 1.0, 0.5}
	samples := make([]TexelSample, 0, len(texelGateFENs))
	for i, fen := range texelGateFENs {
		pos, err := ParseFEN(fen)
		if err != nil {
			t.Fatalf("ParseFEN(%q): %v", fen, err)
		}
		samples = append(samples, TexelSample{Board: pos.Board, Result: results[i%len(results)]})
	}
	return samples
}

// TestTexelGradientNumericCheck validates the analytic gradient against a central
// finite difference of the float MSE, on the most-active parameters (largest |g|,
// which naturally span material / PST / scalar weights). A wrong sign or index in
// texelTraceAccumulate would diverge here. The model is linear in w so the MSE is
// smooth and central differences match to high precision.
func TestTexelGradientNumericCheck(t *testing.T) {

	traces := traceDataset(gateSamples(t))
	ptrs := TexelFullParams()
	w := make([]float64, len(ptrs))
	for i, p := range ptrs {
		w[i] = float64(*p)
	}
	const k = 1.0

	_, g := texelTraceGradient(traces, w, k, true)
	mseAt := func(wv []float64) float64 { mse, _ := texelTraceGradient(traces, wv, k, false); return mse }

	order := make([]int, len(g))
	for j := range order {
		order[j] = j
	}
	sort.Slice(order, func(a, b int) bool { return math.Abs(g[order[a]]) > math.Abs(g[order[b]]) })

	// Check the 8 most-active params (material/PST) plus every scalar weight — the
	// scalar signs (notably the negated doubled/isolated penalties) are the most
	// error-prone and would not otherwise rank into the top by magnitude.
	check := append([]int(nil), order[:8]...)
	for s := 0; s < texelNumScalars; s++ {
		check = append(check, texelScalarBase+s)
	}

	const h = 1.0
	checked := 0
	for _, j := range check {
		if g[j] == 0 {
			continue // inactive in this tiny dataset — skip
		}
		wp := append([]float64(nil), w...)
		wp[j] += h
		wm := append([]float64(nil), w...)
		wm[j] -= h
		num := (mseAt(wp) - mseAt(wm)) / (2 * h)
		if math.Abs(g[j]-num) > 1e-7+1e-3*math.Abs(num) {
			t.Errorf("grad[%d]=%.6e disagrees with numeric %.6e (diff %.2e)", j, g[j], num, g[j]-num)
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("no active gradients to check")
	}
	t.Logf("gradient check passed on %d params (8 most-active + active scalars)", checked)
}

// TestTexelGradientBackwardPawnCoordinates checks both phase-tapered backward-pawn
// coordinates directly. The general numeric test can miss them when its small corpus
// happens not to contain a backward pawn.
func TestTexelGradientBackwardPawnCoordinates(t *testing.T) {
	pos, err := ParseFEN("1nb1kb1r/ppp2p2/3pqnpp/4p1P1/4BP2/2N1P2N/PPPP3P/R1BQ1K1R w kq - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	b := pos.Board
	b.recomputeAccumulator()
	tr := buildEvalTrace(&b)
	if tr.Backward == 0 || tr.Phase == 0 || tr.Phase == totalPhase {
		t.Fatalf("fixture must have a backward pawn and mixed phase: backward=%d phase=%d", tr.Backward, tr.Phase)
	}
	tr.Result = 0.17
	ptrs := TexelFullParams()
	w := make([]float64, len(ptrs))
	for i, p := range ptrs {
		w[i] = float64(*p)
	}
	_, gradient := texelTraceGradient([]evalTrace{tr}, w, 1.0, true)
	mseAt := func(weights []float64) float64 {
		mse, _ := texelTraceGradient([]evalTrace{tr}, weights, 1.0, false)
		return mse
	}
	for _, index := range []int{texelScalarBase + 3, texelScalarBase + 13} {
		if gradient[index] == 0 {
			t.Fatalf("backward-pawn gradient at %d is zero", index)
		}
		const h = 1.0
		plus, minus := append([]float64(nil), w...), append([]float64(nil), w...)
		plus[index] += h
		minus[index] -= h
		numeric := (mseAt(plus) - mseAt(minus)) / (2 * h)
		if math.Abs(gradient[index]-numeric) > 1e-8+1e-3*math.Abs(numeric) {
			t.Errorf("backward gradient[%d]=%.6e, numeric=%.6e", index, gradient[index], numeric)
		}
	}
}

// A training update can worsen validation loss. The original vector is a valid
// checkpoint and must win if every attempted update is worse than that baseline.
func TestTexelGradientTunePreservesBetterInitialVector(t *testing.T) {
	ptrs := TexelFullParams()
	saved := make([]int, len(ptrs))
	w := make([]float64, len(ptrs))
	for i, p := range ptrs {
		saved[i], w[i] = *p, float64(*p)
	}
	defer func() {
		for i, p := range ptrs {
			*p = saved[i]
		}
		RebuildPST()
	}()
	validation := gateSamples(t)
	traces := traceDataset(validation)
	train := make([]TexelSample, len(validation))
	for i := range validation {
		prediction := texelSigmoid(evalTraceFloat(&traces[i], w), 1)
		validation[i].Result = prediction // exact minimum at the initial vector
		train[i] = validation[i]
		if prediction < 0.5 {
			train[i].Result = 1
		} else {
			train[i].Result = 0
		}
	}
	var attemptedLoss float64
	TexelGradientTune(train, validation, TexelGradientConfig{
		K: 1, LR: 20, Epochs: 1,
		Progress: func(_ int, _, loss float64) { attemptedLoss = loss },
	})
	if attemptedLoss <= 1e-10 {
		t.Fatalf("fixture did not produce a worse validation checkpoint: %g", attemptedLoss)
	}
	for i, p := range ptrs {
		if *p != saved[i] {
			t.Fatalf("worse validation checkpoint replaced initial parameter %d: %d -> %d (loss %g)", i, saved[i], *p, attemptedLoss)
		}
	}
}

// TestTexelGradientTuneReducesMSE is the machinery end-to-end: with a REACHABLE
// target (labels are the float model's own output at the current parameters, so a
// zero-MSE fit exists), perturbing the parameters raises the loss and Adam descent
// must drive the integer-eval MSE back down substantially, writing finite values
// back. This guarantees real, reducible gradient signal (avoiding the degenerate
// K≈0 that arbitrary uncorrelated labels produce). Parameters are saved/restored.
func TestTexelGradientTuneReducesMSE(t *testing.T) {

	ptrs := TexelFullParams()
	savedParams := make([]int, len(ptrs))
	for i, p := range ptrs {
		savedParams[i] = *p
	}
	defer func() {
		for i, p := range ptrs {
			*p = savedParams[i]
		}
		RebuildPST()
	}()

	const k = 1.0
	samples := gateSamples(t)
	traces := traceDataset(samples) // coefficients are param-independent
	w0 := make([]float64, len(ptrs))
	for i, p := range ptrs {
		w0[i] = float64(*p)
	}
	// Labels = logistic of the current float eval → a perfectly reachable target.
	for i := range samples {
		samples[i].Result = texelSigmoid(evalTraceFloat(&traces[i], w0), k)
	}
	// Perturb the live parameters so the starting fit is poor.
	for _, p := range ptrs {
		*p = *p * 4 / 5
	}
	RebuildPST()
	before := texelMSE(samples, k)

	trainMSE, _ := TexelGradientTune(samples, nil, TexelGradientConfig{K: k, LR: 2.0, Epochs: 200})

	if math.IsNaN(trainMSE) || math.IsInf(trainMSE, 0) {
		t.Fatalf("tuned MSE not finite: %v", trainMSE)
	}
	if trainMSE >= 0.7*before {
		t.Errorf("gradient descent failed to substantially reduce MSE: before=%.6f after=%.6f", before, trainMSE)
	}
	t.Logf("integer-eval MSE %.6f -> %.6f over <=200 epochs", before, trainMSE)
}
