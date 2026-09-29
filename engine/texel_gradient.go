package engine

import (
	"encoding/json"
	"fmt"
	"math"
	"math/bits"
	"runtime"
	"slices"
	"sync"
)

// Gradient Texel tuning. The static eval is LINEAR in its tunable parameters for
// a fixed position: the PeSTO core is Σ_pieces sign·(material[pt] + rawPST[pt][sq])
// tapered by phase, and every other term is feature·weight (optionally /100). So
// for each position we can emit a sparse feature-coefficient vector once — the
// trace — and thereafter evaluate the eval (and its exact gradient w.r.t. every
// weight) as cheap linear algebra over the cached trace, with no re-evaluation.
// That caching is what makes gradient descent 100-1000x cheaper than coordinate
// descent's per-parameter line search.
//
// Versioned parameter layout (TexelFullParams and JSON Names use this order):
//
//	[0..4]    pestoMGMaterial[Pawn..Queen]
//	[5..9]    pestoEGMaterial[Pawn..Queen]
//	[10..777] 768 raw PST cells: 12 tables (mgPawn..mgKing, egPawn..egKing) × 64
//	[778..797] 20 legacy scalar slots (two mobility reservations are inactive)
//	[798..863] 66 MG mobility cells: knight, bishop, rook, queen
//	[864..929] 66 EG mobility cells, same order
//	[930..935] six threat coefficients
//
// King has no material parameter (its value is fixed at 0, conventional since both
// sides always have exactly one), but it does have a PST.

const (
	texelNumMaterial    = 10  // mg Pawn..Queen (0..4), eg Pawn..Queen (5..9)
	texelNumPST         = 768 // 12 tables × 64
	texelNumScalars     = 20  // legacy scalar prefix: 10 MG + 10 EG twins
	texelPSTBase        = texelNumMaterial
	texelScalarBase     = texelNumMaterial + texelNumPST
	texelLegacyParams   = texelNumMaterial + texelNumPST + texelNumScalars // 798
	texelMobilityMGBase = texelLegacyParams
	texelMobilityEGBase = texelMobilityMGBase + mobilityCellCount
	texelThreatBase     = texelMobilityEGBase + mobilityCellCount
	texelNumThreats     = 6
	texelNumParams      = texelThreatBase + texelNumThreats // 936, model v2
)

const TexelModelVersion = "ngn-texel-v2-936"

type TexelModelExport struct {
	Version              string
	Names                []string
	Values               []int
	StoredCount          int
	ActiveCount          int
	InactiveReservations []string
}

func TexelParameterNames() []string {
	names := make([]string, 0, texelNumParams)
	for _, phase := range []string{"mg", "eg"} {
		for _, p := range []string{"pawn", "knight", "bishop", "rook", "queen"} {
			names = append(names, phase+"_material_"+p)
		}
	}
	for _, phase := range []string{"mg", "eg"} {
		for _, p := range []string{"pawn", "knight", "bishop", "rook", "queen", "king"} {
			for sq := 0; sq < 64; sq++ {
				names = append(names, fmt.Sprintf("%s_pst_%s_%d", phase, p, sq))
			}
		}
	}
	for _, w := range TexelWeights() {
		names = append(names, w.Name)
	}
	for _, phase := range []string{"mg", "eg"} {
		for _, p := range []string{"knight", "bishop", "rook", "queen"} {
			n := map[string]int{"knight": 9, "bishop": 14, "rook": 15, "queen": 28}[p]
			for cell := 0; cell < n; cell++ {
				names = append(names, fmt.Sprintf("%s_mobility_%s_%d", phase, p, cell))
			}
		}
	}
	for _, n := range []string{"threat_pawn", "threat_minor_major", "threat_rook_queen", "threat_hanging_minor", "threat_hanging_rook", "threat_hanging_queen"} {
		names = append(names, n)
	}
	return names
}
func ExportTexelModel() TexelModelExport {
	_ = mustAcquireHCEModelUse()
	defer releaseHCEModelUse()
	return exportTexelModelLeased()
}

func exportTexelModelLeased() TexelModelExport {
	ptrs := TexelFullParams()
	values := make([]int, len(ptrs))
	for i, p := range ptrs {
		values[i] = *p
	}
	return TexelModelExport{TexelModelVersion, TexelParameterNames(), values, texelNumParams, texelNumParams - 2, []string{"mobilityWeight", "mobilityWeightEG"}}
}

type preparedTexelModel struct {
	values []int
	mgPST  [13][64]int
	egPST  [13][64]int
}

func prepareTexelModel(x TexelModelExport) (preparedTexelModel, error) {
	if x.Version != TexelModelVersion || len(x.Values) != texelNumParams || len(x.Names) != texelNumParams || x.StoredCount != texelNumParams || x.ActiveCount != texelNumParams-2 {
		return preparedTexelModel{}, fmt.Errorf("texel model incompatible: version=%q params=%d names=%d", x.Version, len(x.Values), len(x.Names))
	}
	if !slices.Equal(x.Names, TexelParameterNames()) || !slices.Equal(x.InactiveReservations, []string{"mobilityWeight", "mobilityWeightEG"}) {
		return preparedTexelModel{}, fmt.Errorf("texel model parameter names mismatch")
	}
	prepared := preparedTexelModel{values: append([]int(nil), x.Values...)}
	for pt := Pawn; pt <= King; pt++ {
		mgMaterial, egMaterial := 0, 0
		if pt != King {
			mgMaterial = prepared.values[texelMatMGIndex(pt)]
			egMaterial = prepared.values[texelMatEGIndex(pt)]
		}
		whitePiece := GetPiece(pt, White)
		blackPiece := GetPiece(pt, Black)
		mgBase := texelPSTBase + (int(pt)-1)*64
		egBase := texelPSTBase + (6+int(pt)-1)*64
		for sq := 0; sq < 64; sq++ {
			prepared.mgPST[whitePiece][sq] = mgMaterial + prepared.values[mgBase+(sq^56)]
			prepared.egPST[whitePiece][sq] = egMaterial + prepared.values[egBase+(sq^56)]
			prepared.mgPST[blackPiece][sq] = -(mgMaterial + prepared.values[mgBase+sq])
			prepared.egPST[blackPiece][sq] = -(egMaterial + prepared.values[egBase+sq])
		}
	}
	return prepared, nil
}

// ApplyTexelModel is the supported atomic process-wide HCE import. It validates,
// clones, and prebuilds derived PST tables before reserving the model. If any
// search, direct evaluation, Apply, or tuner is active it returns
// ErrHCEModelBusy immediately and changes nothing.
func installPreparedTexelModel(prepared preparedTexelModel) {
	for i, p := range TexelFullParams() {
		*p = prepared.values[i]
	}
	mgPST = prepared.mgPST
	egPST = prepared.egPST
}

func beginHCETuning() (preparedTexelModel, error) {
	if err := tryBeginHCEModelMutation(); err != nil {
		return preparedTexelModel{}, err
	}
	initial, err := prepareTexelModel(exportTexelModelLeased())
	if err != nil {
		abortHCEModelMutation()
		return preparedTexelModel{}, err
	}
	return initial, nil
}

// finishHCETuning is deferred immediately after beginHCETuning succeeds. A
// callback panic may occur after a raw parameter write but before its rebuild;
// restore the complete captured model and its prebuilt PSTs before publishing
// one invalidation, then preserve the caller's panic value.
func finishHCETuning(initial preparedTexelModel) {
	failure := recover()
	if failure != nil {
		installPreparedTexelModel(initial)
	} else {
		// Coordinate callers may omit or provide a partial rebuild callback. Make
		// the engine's derived PST state coherent before any reader is admitted.
		RebuildPST()
	}
	publishHCEModelMutation()
	if failure != nil {
		panic(failure)
	}
}

func ApplyTexelModel(x TexelModelExport) error {
	prepared, err := prepareTexelModel(x)
	if err != nil {
		return err
	}
	if err := tryBeginHCEModelMutation(); err != nil {
		return err
	}
	installPreparedTexelModel(prepared)
	publishHCEModelMutation()
	return nil
}
func FormatTexelModelExport() string {
	b, _ := json.MarshalIndent(ExportTexelModel(), "", "  ")
	return string(b) + "\n"
}

// Global-index helpers. King (pt==6) is out of material range — callers must skip
// material for the king; its PST indices are valid.
func texelMatMGIndex(pt PieceType) int          { return int(pt) - 1 }     // Pawn..Queen -> 0..4
func texelMatEGIndex(pt PieceType) int          { return 5 + int(pt) - 1 } // -> 5..9
func texelPSTMGIndex(pt PieceType, c uint8) int { return texelPSTBase + (int(pt)-1)*64 + int(c) }
func texelPSTEGIndex(pt PieceType, c uint8) int { return texelPSTBase + (6+int(pt)-1)*64 + int(c) }

// pieceTerm is one piece's contribution to the PeSTO core: its type, the raw-table
// cell it indexes (sq^56 for white, sq for black — matching RebuildPST), and its
// sign (+1 white, −1 black, so summing yields the white-perspective score).
type pieceTerm struct {
	Pt   uint8
	Cell uint8
	Sign int8
}

// evalTrace is the cached linear decomposition of one position's eval: the PeSTO
// occupancy (per-piece terms) plus the integer feature values of the non-PeSTO
// terms (exactly as evaluateUnsafe computes them, with the phase gates folded in
// so an inactive term's feature is 0). Phase and the game result ride along.
type evalTrace struct {
	Phase  int
	Result float64
	Pieces []pieceTerm
	// Non-PeSTO term features (white-perspective deltas, integers).
	Passed       int
	Doubled      int
	Isolated     int
	Backward     int
	Chain        int
	Mobility     int
	RookOpen     int
	Outpost      int
	KingSafety   int // 0 unless phase >= 6 (matches the live eval's C4 gate)
	KingActivity int // 0 unless phase < 6
	// Per-cell white-minus-black mobility counts. These are appended v2
	// parameters; the old Mobility scalar remains an intentionally inactive
	// reservation in the 798-prefix.
	MobilityCounts [mobilityCellCount]int
	ThreatCounts   [texelNumThreats]int
	// Fixed is the integer sum of the non-tunable structural terms (blockaded-passer
	// discount + king-vs-passer discount), already tapered exactly as evalCoreWhite
	// computes them. They carry no texel weight, so they ride as one constant lump
	// rather than the per-term feature list — keeping reconstruct bit-exact while
	// contributing zero gradient.
	Fixed int
}

// texelRawMG/EG cache the (fixed) addresses of the raw white-POV PST tables so the
// reconstruction need not rebuild the slice each call. The CONTENTS change during
// tuning; the pointers do not.
var texelRawMG, texelRawEG [7]*[64]int

func texelMobilityMGAddress(i int) *int {
	if i < 9 {
		return &knightMobMG[i]
	}
	i -= 9
	if i < 14 {
		return &bishopMobMG[i]
	}
	i -= 14
	if i < 15 {
		return &rookMobMG[i]
	}
	i -= 15
	return &queenMobMG[i]
}
func texelMobilityEGAddress(i int) *int {
	if i < 9 {
		return &knightMobEG[i]
	}
	i -= 9
	if i < 14 {
		return &bishopMobEG[i]
	}
	i -= 14
	if i < 15 {
		return &rookMobEG[i]
	}
	i -= 15
	return &queenMobEG[i]
}
func texelMobilityMGPtr(i int) int { return *texelMobilityMGAddress(i) }
func texelMobilityEGPtr(i int) int { return *texelMobilityEGAddress(i) }
func texelThreatPtrs() []*int {
	return []*int{&threatByPawn, &threatMinorOnMajor, &threatRookOnQueen, &threatHangingMinor, &threatHangingRook, &threatHangingQueen}
}

func texelRawTables() {
	if texelRawMG[Pawn] == nil {
		texelRawMG = [7]*[64]int{nil, &mgPawnTable, &mgKnightTable, &mgBishopTable, &mgRookTable, &mgQueenTable, &mgKingTable}
		texelRawEG = [7]*[64]int{nil, &egPawnTable, &egKnightTable, &egBishopTable, &egRookTable, &egQueenTable, &egKingTable}
	}
}

// buildEvalTrace decomposes a position's eval into a cached trace. It mirrors
// evaluateUnsafe term-for-term: the per-piece occupancy reproduces the PeSTO
// accumulator (accMG/accEG), and the feature integers are captured from the same
// helper calls evaluateUnsafe makes — so reconstructEvalInt(trace) is bit-exact
// with evaluateUnsafe(board), which the gate test asserts.
func buildEvalTrace(board *Bitboard) evalTrace {
	var t evalTrace
	phase := board.accPhase
	if phase > totalPhase {
		phase = totalPhase
	}
	t.Phase = phase

	// PeSTO core occupancy. White pieces index the raw table at sq^56 (display
	// order) with +1; black at sq with −1 — exactly RebuildPST's convention.
	pieces := make([]pieceTerm, 0, 32)
	for p := WhitePawn; p <= WhiteKing; p++ {
		pt := PieceType(p) // WhitePawn=1=Pawn .. WhiteKing=6=King
		for bb := board.GetBitboardOf(p); bb != 0; bb &= bb - 1 {
			sq := bitScanForward(bb)
			pieces = append(pieces, pieceTerm{uint8(pt), sq ^ 56, 1})
		}
	}
	for p := BlackPawn; p <= BlackKing; p++ {
		pt := PieceType(p - 6) // BlackPawn=7 -> Pawn ..
		for bb := board.GetBitboardOf(p); bb != 0; bb &= bb - 1 {
			sq := bitScanForward(bb)
			pieces = append(pieces, pieceTerm{uint8(pt), sq, -1})
		}
	}
	t.Pieces = pieces

	// Non-PeSTO terms — identical calls/order to evaluateUnsafe.
	whitePawns := board.GetBitboardOf(WhitePawn)
	blackPawns := board.GetBitboardOf(BlackPawn)
	_, _, wPassedW, bPassedW, wDoubled, bDoubled, wIso, bIso, chainW, chainB := pawnStructureCounts(nil, board, whitePawns, blackPawns)
	t.Passed = wPassedW - bPassedW // rank-weighted passed-pawn sum (see passedRankMul)
	t.Doubled = wDoubled - bDoubled
	t.Isolated = wIso - bIso
	wBackward, bBackward := backwardPawnCounts(whitePawns, blackPawns)
	t.Backward = wBackward - bBackward
	t.Chain = chainW - chainB
	var sliderAtt [64]uint64
	fillSliderAttacks(board, &sliderAtt)
	// V2 traces the live per-count mobility curves. The legacy scalar is zero
	// to preserve the old prefix without pretending that scalar is active.
	whitePieces, blackPieces, _, whitePawnAttacks, blackPawnAttacks := mobilitySetup(board)
	wMob := mobilityCountsForSide(&sliderAtt, whitePieces|blackPawnAttacks,
		board.GetBitboardOf(WhiteKnight), board.GetBitboardOf(WhiteBishop), board.GetBitboardOf(WhiteRook), board.GetBitboardOf(WhiteQueen))
	bMob := mobilityCountsForSide(&sliderAtt, blackPieces|whitePawnAttacks,
		board.GetBitboardOf(BlackKnight), board.GetBitboardOf(BlackBishop), board.GetBitboardOf(BlackRook), board.GetBitboardOf(BlackQueen))
	for i := range t.MobilityCounts {
		t.MobilityCounts[i] = wMob[i] - bMob[i]
	}
	t.Mobility = 0
	t.RookOpen = evaluateRookOnOpenFile(board, White) - evaluateRookOnOpenFile(board, Black)

	whiteKnights := board.GetBitboardOf(WhiteKnight)
	whiteBishops := board.GetBitboardOf(WhiteBishop)
	blackKnights := board.GetBitboardOf(BlackKnight)
	blackBishops := board.GetBitboardOf(BlackBishop)
	t.Outpost = evaluateOutpostSquares(whiteKnights, whiteBishops, whitePawns, blackPawns, White) -
		evaluateOutpostSquares(blackKnights, blackBishops, blackPawns, whitePawns, Black)

	if phase >= 6 {
		t.KingSafety = evaluateKingSafety(board, White, &sliderAtt) - evaluateKingSafety(board, Black, &sliderAtt)
	} else if phase < 6 {
		t.KingActivity = evaluateKingActivity(board)
	}

	// Fixed (non-tunable) structural lump — blockaded-passer + king-vs-passer discounts,
	// captured with the SAME arithmetic/order as evalCoreWhite so reconstructEvalInt
	// stays bit-exact. Not in the texel param vector, hence constant in the gradient.
	blackNonPawns := board.GetBlackPieces() &^ blackPawns
	whiteNonPawns := board.GetWhitePieces() &^ whitePawns
	whitePassers := passersOf(whitePawns, blackPawns, White)
	blackPassers := passersOf(blackPawns, whitePawns, Black)
	wBlocked := passedBlockedCount(whitePassers, blackNonPawns, White)
	bBlocked := passedBlockedCount(blackPassers, whiteNonPawns, Black)
	t.Fixed = -(wBlocked - bBlocked) * taperW(blockedPasserPenalty, blockedPasserPenaltyEG, phase)
	wKingSq := trailingZeros(board.GetBitboardOf(WhiteKing))
	bKingSq := trailingZeros(board.GetBitboardOf(BlackKing))
	wKingDisc := passedKingDiscount(whitePassers, bKingSq, White)
	bKingDisc := passedKingDiscount(blackPassers, wKingSq, Black)
	t.Fixed -= (wKingDisc - bKingDisc) * taperW(0, kingPasserBlockEG, phase)
	wRookBehind := passedRookBehindCount(whitePassers, board.GetBitboardOf(BlackRook), White)
	bRookBehind := passedRookBehindCount(blackPassers, board.GetBitboardOf(WhiteRook), Black)
	t.Fixed -= (wRookBehind - bRookBehind) * taperW(0, rookBehindPasserEG, phase)
	// Trace the six live threat counts, whose weighted subtotal is MG-tapered.
	wAtt := computeSideAtt(&sliderAtt, ((whitePawns&^FileMasks[FileA])<<7)|((whitePawns&^FileMasks[FileH])<<9),
		board.GetBitboardOf(WhiteKnight), board.GetBitboardOf(WhiteBishop), board.GetBitboardOf(WhiteRook), board.GetBitboardOf(WhiteQueen), board.GetBitboardOf(WhiteKing))
	bAtt := computeSideAtt(&sliderAtt, ((blackPawns&^FileMasks[FileH])>>7)|((blackPawns&^FileMasks[FileA])>>9),
		board.GetBitboardOf(BlackKnight), board.GetBitboardOf(BlackBishop), board.GetBitboardOf(BlackRook), board.GetBitboardOf(BlackQueen), board.GetBitboardOf(BlackKing))
	wt := threatFeatureCounts(wAtt, board.GetBitboardOf(BlackKnight), board.GetBitboardOf(BlackBishop), board.GetBitboardOf(BlackRook), board.GetBitboardOf(BlackQueen), bAtt.all)
	bt := threatFeatureCounts(bAtt, board.GetBitboardOf(WhiteKnight), board.GetBitboardOf(WhiteBishop), board.GetBitboardOf(WhiteRook), board.GetBitboardOf(WhiteQueen), wAtt.all)
	for i := range t.ThreatCounts {
		t.ThreatCounts[i] = wt[i] - bt[i]
	}
	return t
}

// reconstructEvalInt rebuilds the integer eval from a trace using the SAME integer
// arithmetic as evaluateUnsafe (one taper division, per-term /100 truncation). It
// reads the current parameter values from the package globals, so at trace time it
// is bit-exact with evaluateUnsafe — the gate that proves the trace is faithful.
func reconstructEvalInt(t *evalTrace) int {
	texelRawTables()
	mg, eg := 0, 0
	for _, pc := range t.Pieces {
		pt := PieceType(pc.Pt)
		s := int(pc.Sign)
		mg += s * (pestoMGMaterial[pt] + texelRawMG[pt][pc.Cell])
		eg += s * (pestoEGMaterial[pt] + texelRawEG[pt][pc.Cell])
	}
	eval := (mg*t.Phase + eg*(totalPhase-t.Phase)) / totalPhase
	ph := t.Phase
	eval += t.Passed * taperW(passedPawnBonus, passedPawnBonusEG, ph)
	eval -= t.Doubled * taperW(doubledPawnPenalty, doubledPawnPenaltyEG, ph)
	eval -= t.Isolated * taperW(isolatedPawnPenalty, isolatedPawnPenaltyEG, ph)
	eval -= t.Backward * taperW(backwardPawnPenalty, backwardPawnPenaltyEG, ph)
	eval += t.Chain * taperW(pawnChainWeight, pawnChainWeightEG, ph) / 100
	eval += t.Mobility * taperW(mobilityWeight, mobilityWeightEG, ph) / 100
	eval += t.RookOpen * taperW(rookOpenWeight, rookOpenWeightEG, ph) / 100
	eval += t.Outpost * taperW(outpostWeight, outpostWeightEG, ph) / 100
	eval += t.KingSafety * taperW(kingSafetyWeight, kingSafetyWeightEG, ph) / 100
	eval += t.KingActivity * taperW(kingActivityWeight, kingActivityWeightEG, ph) / 100
	mgMob, egMob := 0, 0
	for i, n := range t.MobilityCounts {
		mgMob += n * texelMobilityMGPtr(i)
		egMob += n * texelMobilityEGPtr(i)
	}
	eval += taperW(mgMob, egMob, ph)
	threat := 0
	for i, n := range t.ThreatCounts {
		threat += n * *texelThreatPtrs()[i]
	}
	eval += threat * ph / totalPhase
	eval += t.Fixed // non-tunable structural lump (already tapered in buildEvalTrace)
	return eval
}

// evalTraceFloat is the linear eval model used for gradients: s = Σ wⱼ·cⱼ over the
// float weight vector w (length texelNumParams). It is the float relaxation of
// reconstructEvalInt — the only difference is that the /24 taper and the six /100
// term scalings are exact here rather than integer-truncated, a sub-centipawn
// residual the gradient ignores. PST and material share each piece's occupancy
// coefficient (the eval sums material+table into one PST lookup).
func evalTraceFloat(t *evalTrace, w []float64) float64 {
	mgF := float64(t.Phase) / float64(totalPhase)
	egF := float64(totalPhase-t.Phase) / float64(totalPhase)
	var s float64
	for _, pc := range t.Pieces {
		pt := PieceType(pc.Pt)
		sg := float64(pc.Sign)
		s += sg * mgF * w[texelPSTMGIndex(pt, pc.Cell)]
		s += sg * egF * w[texelPSTEGIndex(pt, pc.Cell)]
		if pt != King {
			s += sg * mgF * w[texelMatMGIndex(pt)]
			s += sg * egF * w[texelMatEGIndex(pt)]
		}
	}
	// Each scalar term is tapered: weight = mgWeight·mgF + egWeight·egF, the float
	// relaxation of taperW (MG twin at texelScalarBase+i, EG twin at +10+i).
	s += float64(t.Passed) * (w[texelScalarBase+0]*mgF + w[texelScalarBase+10]*egF)
	s -= float64(t.Doubled) * (w[texelScalarBase+1]*mgF + w[texelScalarBase+11]*egF)
	s -= float64(t.Isolated) * (w[texelScalarBase+2]*mgF + w[texelScalarBase+12]*egF)
	s -= float64(t.Backward) * (w[texelScalarBase+3]*mgF + w[texelScalarBase+13]*egF)
	s += float64(t.Chain) / 100.0 * (w[texelScalarBase+4]*mgF + w[texelScalarBase+14]*egF)
	s += float64(t.Mobility) / 100.0 * (w[texelScalarBase+5]*mgF + w[texelScalarBase+15]*egF)
	s += float64(t.RookOpen) / 100.0 * (w[texelScalarBase+6]*mgF + w[texelScalarBase+16]*egF)
	s += float64(t.Outpost) / 100.0 * (w[texelScalarBase+7]*mgF + w[texelScalarBase+17]*egF)
	s += float64(t.KingSafety) / 100.0 * (w[texelScalarBase+8]*mgF + w[texelScalarBase+18]*egF)
	s += float64(t.KingActivity) / 100.0 * (w[texelScalarBase+9]*mgF + w[texelScalarBase+19]*egF)
	for i, n := range t.MobilityCounts {
		s += float64(n) * (w[texelMobilityMGBase+i]*mgF + w[texelMobilityEGBase+i]*egF)
	}
	for i, n := range t.ThreatCounts {
		s += float64(n) * w[texelThreatBase+i] * mgF
	}
	s += float64(t.Fixed) // non-tunable structural lump (constant: zero gradient)
	return s
}

// TexelFullParams returns offline-only pointers to all 798 tunable parameters
// in global index order (material, PST, scalar weights). The pointers carry no
// lifecycle protection and must not escape into concurrent search/evaluation;
// runtime replacement uses ApplyTexelModel or a checked Try tuner entry point.
func TexelFullParams() []*int {
	ptrs := make([]*int, 0, texelNumParams)
	ptrs = append(ptrs, TexelMaterialParams()...)
	ptrs = append(ptrs, TexelPSTParams()...)
	for _, w := range TexelWeights() {
		ptrs = append(ptrs, w.Ptr)
	}
	for i := 0; i < mobilityCellCount; i++ {
		ptrs = append(ptrs, texelMobilityMGAddress(i))
	}
	for i := 0; i < mobilityCellCount; i++ {
		ptrs = append(ptrs, texelMobilityEGAddress(i))
	}
	ptrs = append(ptrs, texelThreatPtrs()...)
	return ptrs
}

// traceDataset builds the cached eval trace for every sample in parallel (nil
// pawn cache, matching texelMSE's regime). Each board's accumulator is
// recomputed first so accMG/accEG/accPhase are current, and the sample result is
// carried onto the trace.
func traceDataset(samples []TexelSample) []evalTrace {
	traces := make([]evalTrace, len(samples))
	n := len(samples)
	if n == 0 {
		return traces
	}
	workers := runtime.GOMAXPROCS(0)
	if workers > n {
		workers = n
	}
	chunk := (n + workers - 1) / workers
	var wg sync.WaitGroup
	for wk := 0; wk < workers; wk++ {
		lo := wk * chunk
		if lo >= n {
			break
		}
		hi := lo + chunk
		if hi > n {
			hi = n
		}
		wg.Add(1)
		go func(lo, hi int) {
			defer wg.Done()
			for i := lo; i < hi; i++ {
				samples[i].Board.recomputeAccumulator()
				tr := buildEvalTrace(&samples[i].Board)
				tr.Result = samples[i].Result
				traces[i] = tr
			}
		}(lo, hi)
	}
	wg.Wait()
	return traces
}

// texelTraceAccumulate computes one trace's float eval, returns its squared error,
// and (when g != nil) adds its gradient contribution factor·coeff into g over every
// parameter the position touches — a single pass mirroring evalTraceFloat's term
// structure (and signs) exactly. a = K·ln10/400 is the sigmoid-derivative constant;
// σ'(s) = a·σ(1−σ), and d/dwⱼ of (result−σ)² carries 2(σ−result)·σ'·coeffⱼ.
func texelTraceAccumulate(t *evalTrace, w, g []float64, k, a float64) float64 {
	s := evalTraceFloat(t, w)
	p := texelSigmoid(s, k)
	diff := p - t.Result
	if g != nil {
		factor := 2 * diff * a * p * (1 - p)
		mgF := float64(t.Phase) / float64(totalPhase)
		egF := float64(totalPhase-t.Phase) / float64(totalPhase)
		for _, pc := range t.Pieces {
			pt := PieceType(pc.Pt)
			sg := float64(pc.Sign)
			g[texelPSTMGIndex(pt, pc.Cell)] += factor * sg * mgF
			g[texelPSTEGIndex(pt, pc.Cell)] += factor * sg * egF
			if pt != King {
				g[texelMatMGIndex(pt)] += factor * sg * mgF
				g[texelMatEGIndex(pt)] += factor * sg * egF
			}
		}
		// Each scalar term feeds its MG slot (·mgF) and EG twin (·egF) — the partials
		// of coeff·(wMG·mgF + wEG·egF). Signs match evalTraceFloat.
		g[texelScalarBase+0] += factor * float64(t.Passed) * mgF
		g[texelScalarBase+10] += factor * float64(t.Passed) * egF
		g[texelScalarBase+1] += factor * float64(-t.Doubled) * mgF
		g[texelScalarBase+11] += factor * float64(-t.Doubled) * egF
		g[texelScalarBase+2] += factor * float64(-t.Isolated) * mgF
		g[texelScalarBase+12] += factor * float64(-t.Isolated) * egF
		g[texelScalarBase+3] += factor * float64(-t.Backward) * mgF
		g[texelScalarBase+13] += factor * float64(-t.Backward) * egF
		g[texelScalarBase+4] += factor * float64(t.Chain) / 100.0 * mgF
		g[texelScalarBase+14] += factor * float64(t.Chain) / 100.0 * egF
		g[texelScalarBase+5] += factor * float64(t.Mobility) / 100.0 * mgF
		g[texelScalarBase+15] += factor * float64(t.Mobility) / 100.0 * egF
		g[texelScalarBase+6] += factor * float64(t.RookOpen) / 100.0 * mgF
		g[texelScalarBase+16] += factor * float64(t.RookOpen) / 100.0 * egF
		g[texelScalarBase+7] += factor * float64(t.Outpost) / 100.0 * mgF
		g[texelScalarBase+17] += factor * float64(t.Outpost) / 100.0 * egF
		g[texelScalarBase+8] += factor * float64(t.KingSafety) / 100.0 * mgF
		g[texelScalarBase+18] += factor * float64(t.KingSafety) / 100.0 * egF
		g[texelScalarBase+9] += factor * float64(t.KingActivity) / 100.0 * mgF
		g[texelScalarBase+19] += factor * float64(t.KingActivity) / 100.0 * egF
		for i, n := range t.MobilityCounts {
			g[texelMobilityMGBase+i] += factor * float64(n) * mgF
			g[texelMobilityEGBase+i] += factor * float64(n) * egF
		}
		for i, n := range t.ThreatCounts {
			g[texelThreatBase+i] += factor * float64(n) * mgF
		}
	}
	return diff * diff
}

// texelTraceGradient returns the mean squared error of the float model over the
// traces and (when wantGrad) the mean gradient w.r.t. every parameter, computed in
// parallel with per-worker partials reduced at the end.
func texelTraceGradient(traces []evalTrace, w []float64, k float64, wantGrad bool) (mse float64, grad []float64) {
	n := len(traces)
	if n == 0 {
		return 0, make([]float64, texelNumParams)
	}
	a := k * math.Ln10 / 400.0
	workers := runtime.GOMAXPROCS(0)
	if workers > n {
		workers = n
	}
	chunk := (n + workers - 1) / workers
	partialMSE := make([]float64, workers)
	partialG := make([][]float64, workers)
	var wg sync.WaitGroup
	for wk := 0; wk < workers; wk++ {
		lo := wk * chunk
		if lo >= n {
			break
		}
		hi := lo + chunk
		if hi > n {
			hi = n
		}
		wg.Add(1)
		go func(idx, lo, hi int) {
			defer wg.Done()
			var g []float64
			if wantGrad {
				g = make([]float64, texelNumParams)
			}
			var sum float64
			for i := lo; i < hi; i++ {
				sum += texelTraceAccumulate(&traces[i], w, g, k, a)
			}
			partialMSE[idx] = sum
			partialG[idx] = g
		}(wk, lo, hi)
	}
	wg.Wait()
	var total float64
	for _, s := range partialMSE {
		total += s
	}
	mse = total / float64(n)
	if wantGrad {
		grad = make([]float64, texelNumParams)
		for _, pg := range partialG {
			for j, v := range pg {
				grad[j] += v
			}
		}
		inv := 1.0 / float64(n)
		for j := range grad {
			grad[j] *= inv
		}
	}
	return mse, grad
}

// TexelGradientConfig carries the gradient-tuning hyperparameters.
type TexelGradientConfig struct {
	K        float64 // sigmoid scaling (calibrate on the training set via TexelFindK)
	LR       float64 // Adam learning rate (≈ centipawn step ceiling per epoch)
	Epochs   int     // maximum epochs
	Patience int     // stop after this many epochs without test-MSE improvement (0 = off)
	L2       float64 // L2 pull toward the starting params (0 = off); reins in weakly-
	// supported PST cells and pins the material/PST gauge so values don't wander the
	// null space. Penalty λΣ(wⱼ−w0ⱼ)², gradient 2λ(wⱼ−w0ⱼ); well-supported params are
	// unaffected (their data gradient dominates).
	Progress func(epoch int, trainMSE, testMSE float64)
}

// TexelGradientTune fits the full parameter vector (material + 768 PST cells + the
// scalar, mobility and threat weights) by Adam on training traces, monitoring
// held-out test MSE for early stopping, then writes the rounded best parameters
// into the live eval (RebuildPST after). Returns the final train/test MSE measured
// from the INTEGER eval (the real engine), proving the tuned weights transfer off
// the float model. Mutates the eval's global parameters as a side effect.
func TryTexelGradientTune(train, test []TexelSample, cfg TexelGradientConfig) (trainMSE, testMSE float64, err error) {
	initial, err := beginHCETuning()
	if err != nil {
		return 0, 0, err
	}
	defer finishHCETuning(initial)
	trainMSE, testMSE = texelGradientTuneLeased(train, test, cfg)
	return trainMSE, testMSE, nil
}

// TexelGradientTune preserves the legacy signature and fails fast rather than
// waiting if model state is busy. New callers should use TryTexelGradientTune.
func TexelGradientTune(train, test []TexelSample, cfg TexelGradientConfig) (trainMSE, testMSE float64) {
	trainMSE, testMSE, err := TryTexelGradientTune(train, test, cfg)
	if err != nil {
		panic(err)
	}
	return trainMSE, testMSE
}

func texelGradientTuneLeased(train, test []TexelSample, cfg TexelGradientConfig) (trainMSE, testMSE float64) {
	trainTr := traceDataset(train)
	testTr := traceDataset(test)

	ptrs := TexelFullParams()
	w := make([]float64, len(ptrs))
	for i, p := range ptrs {
		w[i] = float64(*p)
	}
	w0 := append([]float64(nil), w...) // L2 reference: the original (PeSTO) values

	m := make([]float64, len(w)) // Adam first moment
	v := make([]float64, len(w)) // Adam second moment
	const b1, b2, eps = 0.9, 0.999, 1e-8

	best := append([]float64(nil), w...)
	sinceImprove := 0

	// monitor returns the loss to minimize/early-stop on: held-out test MSE when a
	// test set is present, else the training MSE.
	monitor := func() float64 {
		if len(testTr) > 0 {
			mse, _ := texelTraceGradient(testTr, w, cfg.K, false)
			return mse
		}
		mse, _ := texelTraceGradient(trainTr, w, cfg.K, false)
		return mse
	}
	// Include the unmodified vector in checkpoint selection. Otherwise the first
	// update wins unconditionally, even if every update worsens validation loss.
	bestMonitor := monitor()

	for epoch := 1; epoch <= cfg.Epochs; epoch++ {
		trMSE, g := texelTraceGradient(trainTr, w, cfg.K, true)
		if cfg.L2 > 0 {
			for j := range g {
				g[j] += 2 * cfg.L2 * (w[j] - w0[j]) // L2 pull toward the original values
			}
		}
		b1t := 1 - math.Pow(b1, float64(epoch))
		b2t := 1 - math.Pow(b2, float64(epoch))
		for j := range w {
			m[j] = b1*m[j] + (1-b1)*g[j]
			v[j] = b2*v[j] + (1-b2)*g[j]*g[j]
			mhat := m[j] / b1t
			vhat := v[j] / b2t
			w[j] -= cfg.LR * mhat / (math.Sqrt(vhat) + eps)
		}
		mon := monitor()
		if cfg.Progress != nil {
			cfg.Progress(epoch, trMSE, mon)
		}
		if mon+1e-12 < bestMonitor {
			bestMonitor = mon
			copy(best, w)
			sinceImprove = 0
		} else {
			sinceImprove++
			if cfg.Patience > 0 && sinceImprove >= cfg.Patience {
				break
			}
		}
	}

	for i, p := range ptrs {
		*p = int(math.Round(best[i]))
	}
	RebuildPST()

	trainMSE = texelMSE(train, cfg.K)
	if len(test) > 0 {
		testMSE = texelMSE(test, cfg.K)
	}
	return trainMSE, testMSE
}

// Offline feature extraction mirrors the direct live evaluator. Keeping it here
// avoids adding array construction to the playing evaluation path. Parity tests
// exercise every new coefficient against the live evaluator under perturbation.
const mobilityCellCount = 9 + 14 + 15 + 28

func mobilityCountsForSide(sliderAtt *[64]uint64, excluded, knightBB, bishopBB, rookBB, queenBB uint64) (counts [mobilityCellCount]int) {
	offset := 0
	for bb := knightBB; bb != 0; bb &= bb - 1 {
		sq := bits.TrailingZeros64(bb)
		mc := PopCount(KnightAttacks[sq] &^ excluded)
		counts[offset+mc]++
	}
	offset += len(knightMobMG)
	for bb := bishopBB; bb != 0; bb &= bb - 1 {
		sq := bits.TrailingZeros64(bb)
		mc := PopCount(sliderAtt[sq] &^ excluded)
		counts[offset+mc]++
	}
	offset += len(bishopMobMG)
	for bb := rookBB; bb != 0; bb &= bb - 1 {
		sq := bits.TrailingZeros64(bb)
		mc := PopCount(sliderAtt[sq] &^ excluded)
		counts[offset+mc]++
	}
	offset += len(rookMobMG)
	for bb := queenBB; bb != 0; bb &= bb - 1 {
		sq := bits.TrailingZeros64(bb)
		mc := PopCount(sliderAtt[sq] &^ excluded)
		counts[offset+mc]++
	}
	return counts
}

func threatFeatureCounts(att sideAtt, enemyKnight, enemyBishop, enemyRook, enemyQueen, enemyAll uint64) (counts [6]int) {
	enemyMinors := enemyKnight | enemyBishop
	enemyPieces := enemyMinors | enemyRook | enemyQueen
	counts[0] = PopCount(enemyPieces & att.pawn)
	counts[1] = PopCount((enemyRook | enemyQueen) & att.minor)
	counts[2] = PopCount(enemyQueen & att.rook)
	weak := att.all &^ enemyAll // attacked by us, not defended by them
	counts[3] = PopCount(enemyMinors & weak)
	counts[4] = PopCount(enemyRook & weak)
	counts[5] = PopCount(enemyQueen & weak)
	return counts
}
