package engine

import (
	"math"
	"math/bits"
)

const (
	INFINITY    = int(math.MaxInt32)
	MATE_VALUE  = 30000
	MATE_IN_MAX = MATE_VALUE - 1000

	RAZORING_MARGIN    = 400 // DEAD (razoring removed): kept as documentation, not tuned
	RAZORING_MAX_DEPTH = 3   // DEAD
)

// Tunable search parameters. These are var, not const, so the SPSA driver and
// UCI `setoption` can override them at runtime (rebuild-free A/B); the literals
// below are the current hand-set defaults and MUST stay node-identical
// (scripts/nodecheck.sh). TunableSearchParams below is the single source of
// truth tying each var to its UCI name and spin range.
var (
	NULL_MOVE_R             = 3   // Null move reduction depth
	NULL_MOVE_MIN_DEPTH     = 2   // Minimum depth for null move pruning
	LMR_FULL_DEPTH          = 3   // Minimum depth for LMR
	LMR_MOVE_THRESHOLD      = 2   // Start reducing after this many moves
	FUTILITY_MARGIN         = 103 // Futility per-depth coefficient (margin = FUTILITY_MARGIN*(depth+improving), CG-grounded)
	FUTILITY_MAX_DEPTH      = 8   // Extended futility pruning depth (W1: was 3, CounterGo prunes quiets to depth 8)
	REVERSE_FUTILITY_MARGIN = 88  // Reverse futility margin
	DELTA_MARGIN            = 96  // Delta pruning margin in quiescence
	SINGULAR_DEPTH          = 6   // Minimum depth for singular extensions
	SINGULAR_MARGIN         = 48  // Margin for singular move detection
	EXTENSION_BUDGET        = 26  // Max net extensions on a single root-to-leaf path (anti-explosion)
	ASP_INIT                = 13  // Aspiration initial half-window (cp); delta = ASP_INIT + |prevScore|/ASP_SCORE_DIV
	ASP_MULT                = 154 // Aspiration widening multiplier in percent (150 => x1.5 delta growth per fail)
)

// ASP_SCORE_DIV scales the aspiration init window by the previous score's
// magnitude. NGN chose 80 as a neutral round placeholder for later NGN tuning.
// Not a tunable — the two window
// knobs exposed to SPSA are ASP_INIT and ASP_MULT.
const ASP_SCORE_DIV = 80

// stoppedSearchScore is a throwaway value returned only while unwinding a stopped
// search. Every caller must check info.Stopped before using the returned score.
const stoppedSearchScore = 0

// TunableParam ties a runtime search parameter to its UCI spin-option metadata.
type TunableParam struct {
	Name          string
	Ptr           *int
	Def, Min, Max int
}

// TunableSearchParams is the registry of search params exposed as UCI spin
// options (advertised + applied in uci.go, and importable by the SPSA driver).
// Min/Max bound both the spin advertisement and SPSA exploration; Def is the
// shipped default (== the literal above).
var TunableSearchParams = []TunableParam{
	{"NullMoveR", &NULL_MOVE_R, 3, 2, 5},
	{"NullMoveMinDepth", &NULL_MOVE_MIN_DEPTH, 2, 1, 6},
	{"LMRFullDepth", &LMR_FULL_DEPTH, 3, 1, 6},
	{"LMRMoveThreshold", &LMR_MOVE_THRESHOLD, 2, 1, 8},
	{"FutilityMargin", &FUTILITY_MARGIN, 103, 50, 160},
	{"FutilityMaxDepth", &FUTILITY_MAX_DEPTH, 8, 1, 8},
	{"ReverseFutilityMargin", &REVERSE_FUTILITY_MARGIN, 88, 40, 300},
	{"DeltaMargin", &DELTA_MARGIN, 96, 0, 400},
	{"SingularDepth", &SINGULAR_DEPTH, 6, 4, 12},
	{"SingularMargin", &SINGULAR_MARGIN, 48, 16, 256},
	{"ExtensionBudget", &EXTENSION_BUDGET, 26, 4, 64},
	{"AspInit", &ASP_INIT, 13, 4, 40},
	{"AspMult", &ASP_MULT, 154, 110, 250},
	{"LMPBase", &LMP_BASE, 4, 2, 16},
}

// Late Move Pruning thresholds, indexed [improving][depth] (S4, W11). Follows
// CounterGo's convergent class curve: improving = 5+depth², not-improving = that
// halved (prune ~2x sooner when our static eval isn't improving vs two plies ago).
// Re-bracketed from the prior hand-tuned schedule, which over-pruned at d5-7
// (was 38 vs CG's 54 at improving d7), then extended to d8 (guard is depth<9) to
// match CounterGo's depth-8 leaf-pruning range. Depth 0 is unused (the LMP guard
// requires depth > 0).
// T8b: the table above was already EXACTLY (LMP_BASE+depth²)/2 and LMP_BASE+depth²
// with base 5 — its own comment said so, and all 16 entries reproduce exactly. It is
// therefore computed inline from the tunable base rather than kept as a frozen
// literal, which exposes it to SPSA without changing a single threshold at the
// default. Kept as a var (not const) so the UCI/SPSA path can write through it.
var LMP_BASE = 4

// lmpThresholdFor returns the late-move-pruning move count for a depth, matching
// the historical table exactly at LMP_BASE == 5 (integer division intended).
func lmpThresholdFor(depth int, improving bool) int {
	t := LMP_BASE + depth*depth
	if !improving {
		t /= 2
	}
	return t
}

// SearchToggles enables/disables individual pruning & reduction techniques for
// the correctness-ablation harness (TestPruningAblation). All default ON, so
// normal play is unchanged; the test flips one OFF at a time — a technique whose
// removal makes the engine solve MORE tactics is over-pruning real refutations
// (a bug), not just trading accuracy for speed.
var SearchToggles = struct {
	NullMove  bool
	Futility  bool
	RFP       bool
	Probcut   bool
	LMP       bool
	SEEPrune  bool
	LMR       bool
	HistPrune bool
	IID       bool
	Singular  bool
}{true, true, true, true, true, true, true, true, true, true}

// lmrTable[depth][legalTried] precomputes the Stockfish-style log-based LMR
// base reduction: floor(log(depth) * log(legalTried) / 2). Smoother gradient
// than the prior piecewise tiers (legalTried thresholds at 6/12 plus a
// depth>6 bump), and reduces less at typical mid-depth/mid-moveNum nodes
// where discrete +1 jumps tended to over-bite. PV/non-PV split, history,
// killer adjustments, and the depth cap remain on top.
var lmrTable [64][64]int

func init() {
	for d := 1; d < 64; d++ {
		for m := 1; m < 64; m++ {
			lmrTable[d][m] = int(math.Log(float64(d)) * math.Log(float64(m)) / 2.0)
		}
	}
}

// scoreToTT converts a search score to a value safe to store in the TT, with
// mate scores adjusted to be node-relative ("mate-distance-from-this-node")
// rather than root-relative. Without this, a TT hit retrieved at a different
// ply via the same hash would report the wrong distance to mate.
func scoreToTT(score, ply int) int16 {
	if score >= MATE_IN_MAX {
		return int16(score + ply)
	}
	if score <= -MATE_IN_MAX {
		return int16(score - ply)
	}
	return int16(score)
}

// scoreFromTT inverts scoreToTT: takes a node-relative TT score and returns a
// root-relative score for the current node's ply.
func scoreFromTT(score int16, ply int) int {
	s := int(score)
	if s >= MATE_IN_MAX {
		return s - ply
	}
	if s <= -MATE_IN_MAX {
		return s + ply
	}
	return s
}

// seeNonLosingByMVV reports that a capture is provably non-losing (SEE >= 0)
// from material values alone: capturing a piece worth at least as much as the
// mover can never lose material, since the worst case is the immediate recapture
// (net = captured - mover >= 0) and SEE never continues an exchange that worsens
// the result. Used to short-circuit the full SEE swap at sites that only test the
// sign of SEE against a non-positive threshold (probcut, bad-capture pruning,
// qsearch losing-capture pruning) — node-identical, since for these moves the full
// SEE would return >= 0 and never trip those tests. Promotions and en passant are
// excluded (the mover changes value / the captured square differs), so they fall
// through to the exact computation.
func seeNonLosingByMVV(move Move) bool {
	return !move.IsEnPassant() && move.PromoType() == NoType &&
		move.CapturedPiece().seeWeight() >= move.MovingPiece().seeWeight()
}

// Static Exchange Evaluation (SEE) implementation
// Returns the material balance after a series of exchanges on the target square
func staticExchangeEvaluation(pos *Position, move Move) int {
	// Castling is a king move with no exchange — SEE is meaningless and the king's
	// infinite weight would poison the swap. Quiet (non-capture) moves DO flow
	// through: gain[0] = 0 (nothing captured) and the swap then evaluates whether
	// the moving piece survives on its destination square. This is what quiet-move
	// SEE pruning reads; capture/EP/promotion paths are unchanged.
	if move.IsCastle() {
		return 0
	}

	from := move.Source()
	to := move.Destination()
	targetSquare := to

	// Handle en passant specially
	if move.IsEnPassant() {
		// For en passant, the captured pawn is on a different square
		if move.MovingPiece().Color() == White {
			targetSquare = Square(int(to) - 8) // Black pawn is one rank down
		} else {
			targetSquare = Square(int(to) + 8) // White pawn is one rank up
		}
	}

	// Get the initial captured piece value
	capturedValue := move.CapturedPiece().seeWeight()

	// Get all attackers of the target square
	attackers := getAttackers(pos, targetSquare)

	// Pawn/knight/king attackers are occupancy-independent (their attack patterns
	// can't be blocked or revealed), so derive them once and re-mask by occupancy
	// each swap step instead of rebuilding them in getAttackersAfterMove.
	nonSlider := seeNonSliderAttackers(pos, targetSquare)

	// Remove the moving piece from attackers (it just moved)
	movingPiece := move.MovingPiece()
	attackers &^= SquareMask[int(from)]

	// Initialize the exchange - use stack array to avoid allocation
	var gain [32]int // Exchange gains on stack
	depth := 0
	color := movingPiece.Color().Other() // Side to move after initial capture
	occupied := pos.Board.GetWhitePieces() | pos.Board.GetBlackPieces()
	occupied &^= SquareMask[int(from)] // mover left its square -> reveals x-ray attackers behind it

	// gain[0] is the value of the piece initially captured. pieceOnSquare tracks
	// the value of whatever piece now stands on the target square (initially the
	// moving piece) -- that is what the NEXT recapture wins. For a promotion-
	// capture the pawn becomes the promoted piece, both for the material gain and
	// as the piece now standing on the square.
	gain[depth] = int(capturedValue)
	pieceOnSquare := int(movingPiece.seeWeight())
	if move.PromoType() != NoType {
		promoPiece := GetPiece(move.PromoType(), movingPiece.Color())
		gain[depth] += int(promoPiece.seeWeight()) - 100 // 100 = pawn seeWeight
		pieceOnSquare = int(promoPiece.seeWeight())
	}

	for {
		depth++
		if depth >= 32 {
			break // Prevent overflow of the gain array
		}

		// Find the least valuable attacker for the current side
		attackerSquare, attackerPiece := findLeastValuableAttacker(pos, attackers, color)
		if attackerSquare == NoSquare {
			break // No more attackers; last valid swap entry is gain[depth-1]
		}

		// Remove this attacker from the occupied bitboard
		occupied &^= SquareMask[int(attackerSquare)]

		// Update attackers (reveal sliders that were behind the moved attacker)
		attackers = getAttackersAfterMove(pos, nonSlider, occupied, targetSquare)

		// This recapture wins the piece currently standing on the square; the
		// recapturing piece then becomes the piece on the square for the next ply.
		gain[depth] = pieceOnSquare - gain[depth-1]
		pieceOnSquare = int(attackerPiece.seeWeight())

		// Switch sides
		color = color.Other()
	}

	// Negamax the swap list backward over the valid entries [0 .. depth-1]. Each
	// side, working back, declines a recapture that would leave it worse off.
	for depth--; depth > 0; depth-- {
		if gain[depth-1] > -gain[depth] {
			gain[depth-1] = -gain[depth]
		}
	}

	return gain[0]
}

// getAttackers returns a bitboard of all pieces attacking the given square
func getAttackers(pos *Position, square Square) uint64 {
	attackers := uint64(0)

	// Pawn attacks
	whitePawns := pos.Board.GetBitboardOf(WhitePawn)
	blackPawns := pos.Board.GetBitboardOf(BlackPawn)

	// White pawn attacks. A white pawn attacking `square` sits one rank below it:
	// at square-9 (its file one LEFT of the target, valid when target file > A) or
	// square-7 (one file RIGHT, valid when target file < H). The file guards must
	// pair with the matching offset or SEE misses edge-file recapturers and invents
	// wrap-around attackers (off-by-file bug fixed 2026-05-30).
	if square.Rank() > Rank1 {
		if square.File() > FileA {
			attackSq := int(square) - 9
			if attackSq >= 0 && attackSq < 64 {
				attackers |= (whitePawns & SquareMask[attackSq])
			}
		}
		if square.File() < FileH {
			attackSq := int(square) - 7
			if attackSq >= 0 && attackSq < 64 {
				attackers |= (whitePawns & SquareMask[attackSq])
			}
		}
	}

	// Black pawn attacks. A black pawn attacking `square` sits one rank above it:
	// at square+7 (file one LEFT, valid when target file > A) or square+9 (one file
	// RIGHT, valid when target file < H).
	if square.Rank() < Rank8 {
		if square.File() > FileA {
			attackSq := int(square) + 7
			if attackSq >= 0 && attackSq < 64 {
				attackers |= (blackPawns & SquareMask[attackSq])
			}
		}
		if square.File() < FileH {
			attackSq := int(square) + 9
			if attackSq >= 0 && attackSq < 64 {
				attackers |= (blackPawns & SquareMask[attackSq])
			}
		}
	}

	// Knight attacks
	whiteKnights := pos.Board.GetBitboardOf(WhiteKnight)
	blackKnights := pos.Board.GetBitboardOf(BlackKnight)
	knightAttacks := KnightAttacks[square]
	attackers |= (whiteKnights | blackKnights) & knightAttacks

	// King attacks
	whiteKing := pos.Board.GetBitboardOf(WhiteKing)
	blackKing := pos.Board.GetBitboardOf(BlackKing)
	kingAttacks := KingAttacks[square]
	attackers |= (whiteKing | blackKing) & kingAttacks

	// Bishop/Queen diagonal attacks
	whiteBishops := pos.Board.GetBitboardOf(WhiteBishop)
	blackBishops := pos.Board.GetBitboardOf(BlackBishop)
	whiteQueens := pos.Board.GetBitboardOf(WhiteQueen)
	blackQueens := pos.Board.GetBitboardOf(BlackQueen)

	diagonalAttackers := whiteBishops | blackBishops | whiteQueens | blackQueens
	attackers |= diagonalAttackers & seeGetBishopAttacks(square, pos.Board.GetWhitePieces()|pos.Board.GetBlackPieces())

	// Rook/Queen straight attacks
	whiteRooks := pos.Board.GetBitboardOf(WhiteRook)
	blackRooks := pos.Board.GetBitboardOf(BlackRook)

	straightAttackers := whiteRooks | blackRooks | whiteQueens | blackQueens
	attackers |= straightAttackers & seeGetRookAttacks(square, pos.Board.GetWhitePieces()|pos.Board.GetBlackPieces())

	return attackers
}

// seeGetBishopAttacks returns squares attacked by bishops/queens on diagonals
// Uses magic bitboards for O(1) lookup
func seeGetBishopAttacks(square Square, occupied uint64) uint64 {
	return GetBishopAttacks(int(square), occupied)
}

// seeGetRookAttacks returns squares attacked by rooks/queens on ranks/files
// Uses magic bitboards for O(1) lookup
func seeGetRookAttacks(square Square, occupied uint64) uint64 {
	return GetRookAttacks(int(square), occupied)
}

// findLeastValuableAttacker finds the least valuable piece attacking the square.
// The `attackers` bitboard is already the EXACT set of pieces attacking `square`
// under the current `occupied` (getAttackers / getAttackersAfterMove filter each
// piece through its attack pattern against this same occupancy), so any member of
// a piece-type's slice is a genuine attacker — no per-candidate re-verification is
// needed. (Removing a used attacker only UNBLOCKS slider lines, so a set member can
// never stop attacking; the recompute keeps the set exact.) Dropping the old
// canPieceAttackSquare call removes 1-2 redundant magic lookups per swap step.
func findLeastValuableAttacker(pos *Position, attackers uint64, color Color) (Square, Piece) {
	// Try attackers cheapest-first: Pawn, Knight, Bishop, Rook, Queen, King — exactly the
	// PieceType order (Pawn=1..King=6). Enemy pieces are the White constants plus a color
	// offset, so hoist the offset instead of a per-call slice literal + per-iteration GetPiece.
	var base Piece
	if color == Black {
		base = 6
	}
	for pt := Pawn; pt <= King; pt++ {
		piece := Piece(pt) + base
		pieceBB := pos.Board.GetBitboardOf(piece) & attackers
		if pieceBB != 0 {
			return Square(bits.TrailingZeros64(pieceBB)), piece
		}
	}

	return NoSquare, NoPiece
}

// seeNonSliderAttackers returns the raw (occupancy-independent) set of pawn,
// knight and king attackers of targetSquare. Pawn/knight/king attack PATTERNS
// don't depend on occupancy — removing pieces during the SEE swap can never block
// or reveal one — so this set is computed ONCE per SEE call and re-masked by the
// current occupancy each swap step (getAttackersAfterMove), instead of re-deriving
// the file-guard branches and pattern ANDs every iteration.
func seeNonSliderAttackers(pos *Position, targetSquare Square) uint64 {
	nonSlider := uint64(0)

	// Pawn attacks (file guards paired with the matching offset — see getAttackers).
	whitePawns := pos.Board.GetBitboardOf(WhitePawn)
	blackPawns := pos.Board.GetBitboardOf(BlackPawn)
	if targetSquare.Rank() > Rank1 {
		if targetSquare.File() > FileA {
			nonSlider |= whitePawns & SquareMask[int(targetSquare)-9]
		}
		if targetSquare.File() < FileH {
			nonSlider |= whitePawns & SquareMask[int(targetSquare)-7]
		}
	}
	if targetSquare.Rank() < Rank8 {
		if targetSquare.File() > FileA {
			nonSlider |= blackPawns & SquareMask[int(targetSquare)+7]
		}
		if targetSquare.File() < FileH {
			nonSlider |= blackPawns & SquareMask[int(targetSquare)+9]
		}
	}

	// Knight + king attacks (occupancy-independent patterns).
	nonSlider |= (pos.Board.GetBitboardOf(WhiteKnight) | pos.Board.GetBitboardOf(BlackKnight)) & KnightAttacks[targetSquare]
	nonSlider |= (pos.Board.GetBitboardOf(WhiteKing) | pos.Board.GetBitboardOf(BlackKing)) & KingAttacks[targetSquare]
	return nonSlider
}

// getAttackersAfterMove returns the attacker set of targetSquare under the updated
// occupancy. The non-slider attackers (nonSlider, precomputed once) only need
// re-masking by occupied; only the sliders are recomputed, since removing the used
// attacker can reveal x-ray bishops/rooks/queens behind it. Bit-identical to a full
// rebuild but skips re-deriving the occupancy-independent pawn/knight/king masks.
func getAttackersAfterMove(pos *Position, nonSlider uint64, occupied uint64, targetSquare Square) uint64 {
	tempAttackers := nonSlider & occupied

	// Bishop/Queen diagonal attacks
	whiteBishops := pos.Board.GetBitboardOf(WhiteBishop) & occupied
	blackBishops := pos.Board.GetBitboardOf(BlackBishop) & occupied
	whiteQueens := pos.Board.GetBitboardOf(WhiteQueen) & occupied
	blackQueens := pos.Board.GetBitboardOf(BlackQueen) & occupied

	diagonalAttackers := whiteBishops | blackBishops | whiteQueens | blackQueens
	tempAttackers |= diagonalAttackers & seeGetBishopAttacks(targetSquare, occupied)

	// Rook/Queen straight attacks
	whiteRooks := pos.Board.GetBitboardOf(WhiteRook) & occupied
	blackRooks := pos.Board.GetBitboardOf(BlackRook) & occupied

	straightAttackers := whiteRooks | blackRooks | whiteQueens | blackQueens
	tempAttackers |= straightAttackers & seeGetRookAttacks(targetSquare, occupied)

	return tempAttackers
}

// Search debug logging removed for production

// searchFrame holds one alphaBetaPV invocation's move buffers, reused across the search
// via a recursion-frame pool (SearchInfo.Frames) instead of being stack-allocated and
// heap-escaping per node — cuts search allocations ~75% (docs/10 T1). Indexed by a
// recursion-FRAME counter, NOT ply: IID and singular verification re-enter at the same
// ply, so a ply-indexed buffer would clobber a live parent (docs/09 J3).
type searchFrame struct {
	moveBuffer             [256]Move              // raw pseudo-legal generation target (transient: gen -> order)
	ordered                [256]Move              // ordered moves, iterated across the recursive move loop
	scores                 [256]int               // ordering scores, parallel to ordered, for incremental selection
	quiets                 [64]Move               // quiet moves tried (history penalties on a cutoff)
	captures               [64]Move               // captures tried (capture-history penalties)
	completedHistoryQuiets h1CompletedQuietBuffer // completed quiet candidates for policy-11 learning
}

// searchFramePoolSize bounds the reusable frame pool; a recursion depth beyond it falls
// back to a stack-allocated frame (still correct, just not allocation-free). Sized well
// above MaximumDepth(100) + extensions + same-ply re-entrant (IID/singular) restacking.
const searchFramePoolSize = 256

type SearchInfo struct {
	Nodes            uint64
	EffectiveThreads int
	Depth            int
	SelDepth         int // Maximum ply reached including quiescence
	RootDepth        int // Depth of the current ID iteration; the reference for the per-path extension budget
	BestMove         Move
	BestScore        int
	Stopped          bool
	TimeManager      *TimeManager
	PV               [MaximumDepth]Move // Principal variation array
	PVLength         int                // Length of current PV
	Hashfull         int                // Permille occupancy sampled with the completed PV

	// Static eval per ply (white-to-move perspective of the node) for the
	// improving heuristic; indexed by ply like PV.
	StaticEvalStack [MaximumDepth]int

	// Move made at each ply on the current search path (EmptyMove for a null
	// move); indexed like StaticEvalStack. A node at ply p reads [p-2] as its
	// side's previous move for the follow-up history; only live-ancestor slots
	// are ever read, so no restore on unwind is needed.
	MoveStack [MaximumDepth]Move

	// Zero-allocation buffers passed through recursion
	MoveBuffer    *[256]Move // For move generation
	CaptureBuffer *[64]Move  // For capture generation
	OrderedBuffer *[256]Move // For move ordering
	SEEGains      *[32]int   // For SEE evaluation

	// Pruning and search efficiency statistics
	FutilityPrunes       uint64 // Moves pruned by futility pruning
	NullMoveCutoffs      uint64 // Beta cutoffs from null move pruning
	LMRReductions        uint64 // Moves reduced by Late Move Reduction
	NullWindowScouts     uint64 // Non-first moves searched with a null window (re-search denominator)
	LMRReSearches        uint64 // Reduced fail-highs re-searched at full depth (LMR verification churn)
	PVReSearches         uint64 // PV nodes re-searched with the full window after a null-window fail-high
	LMPPrunes            uint64 // Moves pruned by Late Move Pruning
	ProbcutPrunes        uint64 // Positions pruned by Probcut
	SEEQuietPrunes       uint64 // Quiet moves pruned by SEE (soft hangs)
	TTProbes             uint64 // Transposition table probes (denominator for hit/cutoff rate)
	TTHits               uint64 // Transposition table hits
	TTCutoffs            uint64 // Beta cutoffs from TT
	BetaCutoffs          uint64 // Total beta cutoffs
	FirstMoveCutoffs     uint64 // Beta cutoffs on the first legal move tried (move-ordering quality)
	QNodes               uint64 // Quiescence search nodes
	QSearchTTStores      uint64 // Quiescence results written to the TT (S3)
	CheckExtensions      uint64 // Check extensions applied
	SingularExtensions   uint64 // Singular extensions applied
	RecaptureExtensions  uint64 // Recapture extensions applied
	PassedPawnExtensions uint64 // Passed pawn extensions applied

	// EBF-attribution diagnostics (docs/11 §3). Plain per-search counters,
	// incremented unconditionally, NEVER read by search decisions — node counts
	// are identical with or without them (nodecheck is the implementation gate).
	// Reported once after bestmove via the `debug on` sd_* dump in uci.go.
	MoveLoopNodes  uint64    // Nodes that reach the move loop (ordering denominator)
	TTMoveListed   uint64    // Move-loop nodes where a hash move was available to ordering
	CutIdxHist     [5]uint64 // Beta cutoffs bucketed by legalTried: 1, 2, 3, 4-7, 8+
	CutTriedSum    uint64    // Sum of legalTried at beta cutoffs (mean cut index; b_cut numerator)
	BcutBand       [4]uint64 // Beta cutoffs by remaining-depth band: 1-2, 3-5, 6-9, 10+
	FmcBand        [4]uint64 // First-move cutoffs by the same depth bands
	CutByTT        uint64    // Cutoff move classes (scoring-time data only; killer/counter
	CutByCapture   uint64    // labels are approximate — tables mutate during child searches)
	CutByPromo     uint64
	CutByKiller    uint64
	CutByCounter   uint64
	CutByQuietHist uint64
	CutMissByClass [6][3]uint64 // Non-first beta cutoffs (ordering misses) by [class][band];
	//                                  class 0..5 = TT/capture/promo/killer/counter/quiet-hist,
	//                                  band 0..2 = the cut landed on move 2 / move 3 / move 4+.
	RFPPrunes           uint64 // Reverse-futility returns (was uncounted)
	NullMoveTries       uint64 // Null-move attempts (failed nulls are pure overhead)
	SEECapPrunes        uint64 // Captures pruned by SEE in the move loop (was uncounted)
	HistPrunes          uint64 // Quiets pruned by history score (was uncounted)
	AllNodes            uint64 // Completed move loops storing UpperBound (all-nodes)
	PVNodesExact        uint64 // Completed move loops storing Exact
	AllTriedSum         uint64 // Sum of legalTried at all-nodes (b_all = AllTriedSum/AllNodes)
	LMRPliesSum         uint64 // Total reduction plies (mean = LMRPliesSum/LMRReductions)
	QDepthCapHits       uint64 // qsearch qDepth>=6 truncations to raw static eval (A4 soundness)
	QTTProbes           uint64 // qsearch TT probes
	QTTHits             uint64 // qsearch TT hits (any bound)
	QTTCutoffs          uint64 // qsearch TT returns
	QStandPatCuts       uint64 // qsearch stand-pat beta cutoffs
	QDeltaPrunes        uint64 // qsearch delta prunes
	QSEEPrunes          uint64 // qsearch SEE prunes
	QBetaCutoffs        uint64 // qsearch beta cutoffs in the capture/evasion loops
	AspFailLows         uint64 // Aspiration fail-lows (each costs a full-width re-iteration)
	AspFailHighs        uint64 // Aspiration fail-highs
	AspAbandoned        uint64 // Iterations discarded after exhausting widening attempts
	RootBestMoveChanges uint64 // Root best-move changes across completed iterations
	IIDSearches         uint64 // Internal iterative deepening searches run
	SingularTries       uint64 // Singular verification searches run (success ctr exists)
	ExtBudgetClamps     uint64 // Paths clamped by the extension budget (explosion lines)

	ExcludedMove Move // Move to exclude during singular search
	ExcludedPly  int  // ply at which ExcludedMove applies (the singular verification root)

	// Repetition detection during search
	RepStack    [512]uint64 // Stack of Zobrist hashes on current search path
	RepStackLen int         // Current depth in repetition stack

	// maxNodes snapshots the `go nodes` budget at search start — it is constant for
	// the whole search, so the per-node check reads a plain field instead of paying
	// an atomic load at every main and qsearch node.
	control  *SearchControl
	maxNodes uint64
	// history is the persistent heuristic owner supplied by the SearchEngine.
	// It is explicit on every production entry path and never falls back to a global.
	history   *workerHistory
	evaluator *workerEvaluator
	tt        *Cache

	nodePublisher      *searchNodePublisher
	lastPublishedNodes uint64

	// Reusable per-invocation move buffers (docs/10 T1). Frames is allocated once per
	// search; FrameDepth is the recursion-frame counter, pushed/popped in lockstep with
	// RepStackLen so each live alphaBetaPV invocation owns a distinct frame.
	Frames     []searchFrame
	FrameDepth int
}

func (info *SearchInfo) observeTermination(reason searchTerminationReason) {
	if info == nil || info.TimeManager == nil {
		return
	}
	info.TimeManager.observeControlTermination(reason, info.RootDepth)
}

// shouldStopForControl preserves the existing external/node cancellation
// behavior while attributing the first decisive cause when the opt-in C0
// observer is attached. A pre-existing request wins over a simultaneously
// reached node cap; a node-cap request is latched before its shared stop bit can
// later look like an external request.
func (info *SearchInfo) shouldStopForControl() bool {
	if info.control.StopRequested() {
		info.observeTermination(searchTerminationExternalStop)
		return true
	}
	if info.maxNodes != 0 && info.Nodes >= info.maxNodes {
		info.observeTermination(searchTerminationNodeCap)
		info.control.RequestStop()
		return true
	}
	return false
}

// countNode records one completed unit of search work and latches a fixed-node
// stop exactly at the configured cap. A child can consume the final node and
// return normally (for example through a qsearch stand-pat cutoff); checking
// only on the next recursive entry lets its parent increment once more.
func (info *SearchInfo) countNode() bool {
	if info.maxNodes != 0 && info.Nodes >= info.maxNodes {
		info.observeTermination(searchTerminationNodeCap)
		info.control.RequestStop()
		info.Stopped = true
		return false
	}
	info.Nodes++
	info.publishNodesIfDue(false)
	if info.maxNodes != 0 && info.Nodes >= info.maxNodes {
		info.observeTermination(searchTerminationNodeCap)
		info.control.RequestStop()
		info.Stopped = true
		return false
	}
	return true
}

// DepthCallback is called after each depth completes during iterative deepening
type DepthCallback func(info *SearchInfo)

type iterativeSearchOptions struct {
	advanceTTAge     bool
	effectiveThreads int
	nodePublisher    *searchNodePublisher
}

func searchIterativeDeepeningUnsafe(pos *Position, maxDepth int, timeManager *TimeManager, callback DepthCallback, control *SearchControl, history *workerHistory, evaluator *workerEvaluator, tt *Cache) *SearchInfo {
	return searchIterativeDeepeningWorkerUnsafe(pos, maxDepth, timeManager, callback, control, history, evaluator, tt, iterativeSearchOptions{advanceTTAge: true, effectiveThreads: 1})
}

func searchIterativeDeepeningWorkerUnsafe(pos *Position, maxDepth int, timeManager *TimeManager, callback DepthCallback, control *SearchControl, history *workerHistory, evaluator *workerEvaluator, tt *Cache, options iterativeSearchOptions) *SearchInfo {
	if options.advanceTTAge {
		tt.AdvanceAge()
	}

	// Allocate buffers once for the entire search tree
	var moveBuffer [256]Move
	var captureBuffer [64]Move
	var orderedBuffer [256]Move
	var seeGains [32]int

	info := &SearchInfo{
		Nodes:            0,
		EffectiveThreads: options.effectiveThreads,
		Depth:            maxDepth,
		BestMove:         EmptyMove,
		BestScore:        -INFINITY,
		Stopped:          false,
		TimeManager:      timeManager,
		MoveBuffer:       &moveBuffer,
		CaptureBuffer:    &captureBuffer,
		OrderedBuffer:    &orderedBuffer,
		SEEGains:         &seeGains,
		Frames:           make([]searchFrame, searchFramePoolSize),
		control:          control,
		maxNodes:         control.MaxNodes(),
		history:          history,
		evaluator:        evaluator,
		tt:               tt,
		nodePublisher:    options.nodePublisher,
	}
	defer info.publishNodesIfDue(true)
	if callback != nil {
		defer refreshSearchReport(pos, info)
	}

	seedRootStaticEval(pos, info)

	// Generate root moves without heap allocations (pseudo-legal; legality handled in search loop)
	var rootGen [256]Move
	numGen := GenerateMovesIntoBuffer(pos, rootGen[:])
	moves := rootGen[:numGen]
	if len(moves) == 0 {
		if isInCheck(pos, pos.Turn()) {
			info.BestScore = -MATE_VALUE
		} else {
			info.BestScore = 0
		}
		info.observeTermination(searchTerminationTerminal)
		return info
	}

	// Iterative deepening - start from depth 1 and go up to maxDepth
	previousScore := 0 // Track previous iteration score for aspiration windows

	terminalCompletion := false
	for currentDepth := 1; currentDepth <= maxDepth && !info.Stopped; currentDepth++ {
		// Honor an external UCI stop request even when no time manager is set
		// (e.g. `go infinite`).
		if info.control.StopRequested() {
			info.observeTermination(searchTerminationExternalStop)
			info.Stopped = true
			break
		}

		// Mark the start of this iteration so the soft-stop heuristic in
		// ShouldStopSearch can measure iteration time.
		if info.TimeManager != nil {
			info.TimeManager.NewIteration()
		}

		// Reset seldepth at the top of each iteration so UCI info reports the
		// deepest reach for THIS iteration, not the max across all prior ones.
		info.SelDepth = currentDepth

		// Record this iteration's root depth as the reference for the per-path
		// extension budget (search.go ~1470): a node's net extensions along the
		// current path == depth + ply - RootDepth, which we cap to keep a
		// non-decaying checking line from recursing toward MaximumDepth.
		info.RootDepth = currentDepth

		// Check time before starting each new depth iteration
		if info.TimeManager != nil {
			if info.TimeManager.shouldStopSearchAt(currentDepth, timeCheckIterationAdmission) {
				info.Stopped = true
				break
			}
		}

		// T5: Modern aspiration windows. Shallow depths (<=3) search full width — a
		// stable previousScore isn't established yet and re-searches there are pure
		// waste. From depth 4 up, open a small window scaled by the previous score's
		// magnitude (delta = ASP_INIT + |prevScore|/ASP_SCORE_DIV) and
		// widen it PROGRESSIVELY on each fail below (delta grows by ASP_MULT), never
		// jumping straight to an infinite bound and never abandoning the iteration.
		// Widening caps naturally at +/-INFINITY (a full-width search always lands
		// in-window), so the iteration always completes.
		var alpha, beta, delta int
		if currentDepth <= 3 {
			alpha = -INFINITY
			beta = INFINITY
		} else {
			scoreAbs := previousScore
			if scoreAbs < 0 {
				scoreAbs = -scoreAbs
			}
			delta = ASP_INIT + scoreAbs/ASP_SCORE_DIV
			alpha = previousScore - delta
			beta = previousScore + delta
		}

		// Aspiration re-search loop. T5: runs until the score lands inside the
		// window or the clock stops — the window widens progressively on each fail
		// (below), so there is no attempt cap and no discarded iteration. attempts
		// still drives the window-held-on-first-attempt time-management signal.
		iterationComplete := false
		attempts := 0

		for !iterationComplete {
			attempts++

			// Check time before attempting search
			if info.TimeManager != nil && info.TimeManager.ShouldStopSearch(currentDepth) {
				info.Stopped = true
				break
			}

			// Snapshot the window for this attempt before the move loop mutates alpha.
			// Classification at the end must compare against the original window, not
			// the alpha that was raised by successful moves during the loop.
			windowAlpha := alpha
			windowBeta := beta

			// Root search with aspiration window
			var orderedRoot [256]Move
			orderedCount := info.history.orderMovesIntoBufferWithDepth(moves, 0, orderedRoot[:], pos) // killers are ply-keyed; root is ply 0
			// S6: search the previous iteration's best move first — a good alpha early
			// tightens pruning and gives time management a best-move stability signal.
			// info.BestMove holds the last COMPLETED depth's best move.
			if info.BestMove != EmptyMove {
				for i := 1; i < orderedCount; i++ {
					if orderedRoot[i] == info.BestMove {
						orderedRoot[0], orderedRoot[i] = orderedRoot[i], orderedRoot[0]
						break
					}
				}
			}
			var completedRootQuiets h1CompletedQuietBuffer
			completedRootQuietCount := 0
			iterationBestScore := -INFINITY
			iterationBestMove := EmptyMove
			bestMoveNodes := uint64(0)      // T1e: nodes spent inside the current best move's subtree
			attemptStartNodes := info.Nodes // T1e: node baseline for this attempt's per-move attribution
			rootInCheck := pos.IsInCheck()  // root position: don't root-LMR while in check
			legalRoot := 0                  // count of LEGAL root moves (PVS first-move + LMR index)

			for oi := 0; oi < orderedCount; oi++ {
				move := orderedRoot[oi]
				// Check time before each move
				if info.TimeManager != nil && info.TimeManager.ShouldStopSearch(currentDepth) {
					info.Stopped = true
					break
				}
				if move.IsCastle() && !isLegalCastle(pos, move) {
					continue
				}

				// Explore root moves with MakeMove (the search make), NOT GameMakeMove:
				// the game-history map (pos.Positions) must count only positions actually
				// PLAYED, so root exploration must not increment it. GameMakeMove here
				// inflated Positions[child] for the very position the ply-1 child then
				// reads, so a 2nd occurrence (true count 1) looked like count 2 and the
				// child's "count >= 2 == threefold" check declared a bogus draw.
				// Thread lastMovePlayed so the counter-move heuristic sees the
				// actual predecessor inside search, not a stale global.
				savedLast := info.history.GetLastMovePlayed()
				evalMove := info.evaluator.mustPrepareMove(pos, move)
				ep, tag, hc, _ := pos.MakeMove(move)
				if isInCheck(pos, move.MovingPiece().Color()) {
					pos.UnMakeMove(move, tag, ep, hc)
					continue
				}
				info.evaluator.mustPushMadeMove(pos, evalMove, move, tag, ep, hc)
				info.history.SetLastMovePlayed(move)
				info.MoveStack[0] = move
				legalRoot++
				if h1NewHistoryProducer && h1QuietMove(move) {
					h1AppendCompletedQuiet(&completedRootQuiets, &completedRootQuietCount, move)
				}
				nodesBefore := info.Nodes // T1e: node count before searching this root move's subtree

				var score int
				if legalRoot == 1 {
					// First legal root move: full window + PV (establishes alpha / the PV line).
					score = -alphaBetaPV(pos, currentDepth-1, 1, -beta, -alpha, true, true, false, info)
				} else {
					// Root PVS + root LMR. NGN previously full-windowed EVERY root move at full
					// depth (waste on moves 2..N); CounterGo scouts later root moves with a null
					// window and reduces late quiet ones. Mirrors the internal PVS+LMR
					// (search.go ~1746-1837), gentler — root is always PV, so no non-PV/history bumps.
					reduction := 0
					givesCheck := pos.IsInCheck()                                              // we made `move`; side-now-to-move in check == move gave check
					if SearchToggles.LMR && currentDepth >= LMR_FULL_DEPTH && legalRoot > 4 && // root: only reduce move 5+ (keep best+captures+killers+top-history full-depth)
						!move.IsCapture() && move.PromoType() == NoType && !rootInCheck && !givesCheck {
						d := currentDepth
						if d > 63 {
							d = 63
						}
						m := legalRoot
						if m > 63 {
							m = 63
						}
						reduction = lmrTable[d][m]
						if reduction >= currentDepth-1 {
							reduction = currentDepth - 2
						}
						if reduction < 0 {
							reduction = 0
						}
					}
					// Null-window scout at (possibly) reduced depth.
					score = -alphaBetaPV(pos, currentDepth-1-reduction, 1, -alpha-1, -alpha, false, true, true, info)
					if info.Stopped {
						info.evaluator.mustPop()
						pos.UnMakeMove(move, tag, ep, hc)
						info.history.SetLastMovePlayed(savedLast)
						break
					}
					// A reduced scout that beat alpha may have under-counted it: re-verify at
					// full depth (null window) before trusting it.
					if reduction > 0 && score > alpha {
						score = -alphaBetaPV(pos, currentDepth-1, 1, -alpha-1, -alpha, false, true, true, info)
						if info.Stopped {
							info.evaluator.mustPop()
							pos.UnMakeMove(move, tag, ep, hc)
							info.history.SetLastMovePlayed(savedLast)
							break
						}
					}
					// Scout failed high inside the window: full-window re-search (root is PV).
					if score > alpha && score < beta {
						score = -alphaBetaPV(pos, currentDepth-1, 1, -beta, -alpha, true, true, false, info)
					}
				}
				info.evaluator.mustPop()
				pos.UnMakeMove(move, tag, ep, hc)
				info.history.SetLastMovePlayed(savedLast)
				if info.Stopped {
					break
				}

				if !info.countNode() {
					break
				}
				moveNodes := info.Nodes - nodesBefore // T1e: this root move's subtree node cost

				if score > iterationBestScore {
					iterationBestScore = score
					iterationBestMove = move
					bestMoveNodes = moveNodes // the new best move so far owns this many nodes
				}

				if score > alpha {
					alpha = score
				}

				// Beta cutoff
				if alpha >= beta {
					break
				}
			}

			// If the clock interrupted this iteration mid-flight, its results are
			// partial: child searches were cut off and returned unreliable scores, so
			// iterationBestMove may be a half-searched (often blundering) move. Discard
			// the interrupted iteration entirely and keep the last fully-completed
			// iteration's best move. Without this guard a partial iteration can still
			// land in the in-window branch below and overwrite the good move — the root
			// cause of catastrophic time-pressure blunders.
			if info.Stopped {
				break
			}
			// Pseudo-legal generation can be non-empty even when every move is
			// illegal (for example a castle while in double check). Do not leave
			// the root at -INFINITY after filtering those moves.
			if legalRoot == 0 {
				if isInCheck(pos, pos.Turn()) {
					info.BestScore = -MATE_VALUE
				} else {
					info.BestScore = 0
				}
				info.BestMove = EmptyMove
				info.observeTermination(searchTerminationTerminal)
				return info
			}
			if h1NewHistoryProducer {
				info.history.h1UpdateCompletedNode(
					h1CompletedQuiets(&completedRootQuiets, completedRootQuietCount),
					iterationBestMove,
					info.history.GetLastMovePlayed(),
					EmptyMove,
					currentDepth,
					windowAlpha,
					iterationBestScore,
				)
			}

			// Classify against the original window (windowAlpha/windowBeta), not the
			// alpha that was mutated during the move loop. Otherwise every successful
			// iteration looks like a fail-low because alpha was just raised to
			// iterationBestScore.
			if iterationBestScore <= windowAlpha && currentDepth > 3 {
				// Fail low: grow delta and widen alpha downward (anchored on the
				// score center), keeping the upper bound tight. The +1 floor on the
				// growth guarantees delta strictly increases even when the
				// percentage term truncates to zero, so alpha reaches -INFINITY in
				// a bounded number of re-searches.
				info.AspFailLows++
				grow := delta * (ASP_MULT - 100) / 100
				if grow < 1 {
					grow = 1
				}
				delta += grow
				alpha = previousScore - delta
				if alpha < -INFINITY {
					alpha = -INFINITY
				}
				beta = windowBeta
			} else if iterationBestScore >= windowBeta && currentDepth > 3 {
				// Fail high: grow delta and widen beta upward, keeping the lower
				// bound tight. Symmetric to the fail-low branch.
				info.AspFailHighs++
				grow := delta * (ASP_MULT - 100) / 100
				if grow < 1 {
					grow = 1
				}
				delta += grow
				beta = previousScore + delta
				if beta > INFINITY {
					beta = INFINITY
				}
				alpha = windowAlpha
			} else {
				// Search completed successfully within the window. Feed the time
				// manager the decision-stability signal: did the best move change vs
				// the last completed iteration, and did the window hold on the first
				// attempt (attempts==1 ⇒ no fail-high/fail-low re-search). info.BestMove
				// still holds the PREVIOUS completed iteration's move at this point.
				bestMoveChanged := iterationBestMove != info.BestMove
				if bestMoveChanged && currentDepth > 1 {
					info.RootBestMoveChanges++
				}
				info.BestScore = iterationBestScore
				info.BestMove = iterationBestMove
				previousScore = iterationBestScore
				iterationComplete = true
				if info.TimeManager != nil {
					// T1e: fraction of this attempt's nodes spent inside the best move's
					// subtree. totalAttemptNodes is the exact sum of the per-move deltas
					// (info.Nodes only advances inside the move loop), so the fraction is
					// bestMoveNodes/totalAttemptNodes in (0,1]. Feeds nodeEffortFactor.
					totalAttemptNodes := info.Nodes - attemptStartNodes
					bestMoveNodeFraction := 0.0
					if totalAttemptNodes > 0 {
						bestMoveNodeFraction = float64(bestMoveNodes) / float64(totalAttemptNodes)
					}
					info.TimeManager.ReportCompletedIteration(bestMoveChanged, attempts == 1, bestMoveNodeFraction)
					info.TimeManager.observeCompletedIteration(currentDepth, attempts)
				}
			}
		}

		// Only commit the new depth and emit UCI info when the iteration actually
		// completed. T5 never abandons an iteration to widening exhaustion, so the
		// only way to reach here incomplete is a clock stop, in which case we keep
		// the previous completed depth/best move (info.AspAbandoned stays 0).
		if iterationComplete {
			info.Depth = currentDepth
			info.publishNodesIfDue(true)
			if callback != nil && info.BestMove != EmptyMove {
				refreshSearchReport(pos, info)
				callback(info)
			}

			// A cached mate must survive enough completed root depth to cover its
			// distance plus a small verification margin. Stopping at depth one can
			// preserve a qsearch/TT mate that ignores an available repetition.
			const mateStopMargin = 5
			if currentDepth > mateStopMargin &&
				(info.BestScore >= MATE_VALUE-(currentDepth-mateStopMargin) ||
					info.BestScore <= -MATE_VALUE+(currentDepth-mateStopMargin)) {
				terminalCompletion = true
				break
			}
		}
	}
	if !info.Stopped {
		reason := searchTerminationDepthComplete
		if terminalCompletion {
			reason = searchTerminationTerminal
		}
		info.observeTermination(reason)
	}

	return info
}

// refreshSearchReport copies reporting-only TT state while the receiver session
// and model lease are already held. Search decisions never read these fields.
func refreshSearchReport(pos *Position, info *SearchInfo) {
	info.PVLength = 0
	clear(info.PV[:])
	if info.BestMove != EmptyMove && info.Depth > 0 {
		maxLength := info.Depth
		if maxLength < 32 {
			maxLength = 32
		}
		if maxLength > len(info.PV) {
			maxLength = len(info.PV)
		}
		pv := extractPVFromTT(info.tt, pos, info.BestMove, maxLength)
		info.PVLength = copy(info.PV[:], pv)
	}
	info.Hashfull = info.tt.Consumed()
}

// ExtractPVFromTT reconstructs the principal variation by walking the
// transposition table from the current position. firstMove is applied at the
// root (since the iterative-deepening root loop doesn't write a TT entry there);
// subsequent moves are read from Exact (PV) TT entries until a miss, a
// non-Exact bound, an illegal move, a terminal position, or maxLength is reached.
// A move that creates the terminal position remains in the returned PV.
//
// The position is mutated during the walk and restored before return. Caller
// must run this only after a search iteration has completed (no concurrent
// search writes).
func extractPVFromTT(tt *Cache, pos *Position, firstMove Move, maxLength int) []Move {
	if maxLength <= 0 || firstMove == EmptyMove {
		return nil
	}
	type undoEntry struct {
		move Move
		ep   Square
		tag  PositionTag
		hc   uint8
	}
	pv := make([]Move, 0, maxLength)
	undoStack := make([]undoEntry, 0, maxLength)
	defer func() {
		for i := len(undoStack) - 1; i >= 0; i-- {
			e := undoStack[i]
			pos.UnMakeMove(e.move, e.tag, e.ep, e.hc)
		}
	}()

	// Search MakeMove deliberately does not touch the game-history map. Take a
	// private count snapshot instead: the PV walk must account for occurrences
	// before the root without mutating Positions or copying its live mutex.
	pos.positionsMutex.RLock()
	repetitions := make(map[uint64]int, len(pos.Positions)+maxLength)
	for hash, count := range pos.Positions {
		repetitions[hash] = count
	}
	pos.positionsMutex.RUnlock()
	rootHash := pos.Hash()
	if repetitions[rootHash] == 0 {
		repetitions[rootHash] = 1
	}
	// A supplied best move cannot cross a game boundary that already exists at
	// the root. Check only draw predicates here: successful first-move
	// validation below already proves that the root is not mate or stalemate.
	if pvDrawTerminal(pos, repetitions[rootHash]) {
		return nil
	}

	apply := func(m Move) bool {
		legal := false
		for _, lm := range GenerateLegalMoves(pos) {
			if lm == m {
				legal = true
				break
			}
		}
		if !legal {
			return false
		}
		ep, tag, hc, ok := pos.MakeMove(m)
		if !ok {
			return false
		}
		undoStack = append(undoStack, undoEntry{m, ep, tag, hc})
		pv = append(pv, m)
		return true
	}

	if !apply(firstMove) {
		return nil
	}

	hash := pos.Hash()
	repetitions[hash]++
	for !pvPositionTerminal(pos, repetitions[hash]) && len(pv) < maxLength {
		ttMove, _, _, ttNodeType, ttHit, _ := tt.Get(hash)
		if !ttHit || ttMove == EmptyMove || ttNodeType != Exact {
			break
		}
		if !apply(ttMove) {
			break
		}
		hash = pos.Hash()
		repetitions[hash]++
	}

	return pv
}

// pvPositionTerminal applies game boundaries while reconstructing reported PVs.
// Repetition uses the walker's private history snapshot; IsDraw supplies the
// engine's existing insufficient-material rule, while the explicit >= 100 check
// matches the FIDE fifty-move boundary (IsDraw historically uses > 100).
func pvPositionTerminal(pos *Position, repetitionCount int) bool {
	if !pos.HasLegalMove() {
		return true
	}
	return pvDrawTerminal(pos, repetitionCount)
}

func pvDrawTerminal(pos *Position, repetitionCount int) bool {
	return repetitionCount >= 3 || pos.HalfMoveClock >= 100 || pos.IsDraw()
}

// SearchFixed runs a single fixed-depth search and is a DIAGNOSTIC instrument
// only (profiling, fixed-depth node counts, cmd/trace_move). Production play uses
// SearchIterativeDeepening via the UCI loop. It shares alphaBetaPV with the
// production search, so repetition detection, the per-path extension budget, and
// stop handling are IDENTICAL to production. It diverges only in the root move
// loop: it full-windows every root move with no root PVS and no root LMR, so its
// node counts are LARGER than production's and are NOT comparable as a strength or
// EBF measure. SearchFixed returns the exact FULL-WIDTH minimax value at the given
// depth; production's PVS/LMR/aspiration only approximate it (typically within a
// few cp), so the two agree on the best move and track within search noise but are
// NOT centipawn-identical (guarded by TestSearchFixedMatchesProductionScore). Do
// not draw strength conclusions from it.
func searchFixedUnsafe(pos *Position, depth int, timeManager *TimeManager, control *SearchControl, history *workerHistory, evaluator *workerEvaluator, tt *Cache) *SearchInfo {
	// Allocate buffers once for the entire search tree
	var moveBuffer [256]Move
	var captureBuffer [64]Move
	var orderedBuffer [256]Move
	var seeGains [32]int

	info := &SearchInfo{
		Nodes:            0, // Fresh node count for each depth
		EffectiveThreads: 1,
		Depth:            depth,
		RootDepth:        depth,
		BestMove:         EmptyMove,
		BestScore:        -INFINITY,
		Stopped:          false,
		TimeManager:      timeManager,
		MoveBuffer:       &moveBuffer,
		CaptureBuffer:    &captureBuffer,
		OrderedBuffer:    &orderedBuffer,
		SEEGains:         &seeGains,
		Frames:           make([]searchFrame, searchFramePoolSize),
		control:          control,
		maxNodes:         control.MaxNodes(),
		history:          history,
		evaluator:        evaluator,
		tt:               tt,
	}

	alpha := -INFINITY
	beta := INFINITY

	var rootGen [256]Move
	numGen := GenerateMovesIntoBuffer(pos, rootGen[:])
	moves := rootGen[:numGen]
	if len(moves) == 0 {
		if isInCheck(pos, pos.Turn()) {
			info.BestScore = -MATE_VALUE
		} else {
			info.BestScore = 0
		}
		return info
	}

	seedRootStaticEval(pos, info)

	var orderedRoot [256]Move
	orderedCount := info.history.orderMovesIntoBufferWithDepth(moves, 0, orderedRoot[:], pos) // killers are ply-keyed; root is ply 0
	legalRoot := 0

	for oi := 0; oi < orderedCount; oi++ {
		move := orderedRoot[oi]
		// Check if search was stopped
		if info.Stopped {
			break
		}

		// Check time before each move in fixed depth search
		if info.TimeManager != nil && info.TimeManager.ShouldStopSearch(depth) {
			info.Stopped = true
			break
		}
		if move.IsCastle() && !isLegalCastle(pos, move) {
			continue
		}

		savedLast := info.history.GetLastMovePlayed()
		evalMove := info.evaluator.mustPrepareMove(pos, move)
		ep, tag, hc, _ := pos.MakeMove(move)
		movingColor := move.MovingPiece().Color()
		if isInCheck(pos, movingColor) {
			pos.UnMakeMove(move, tag, ep, hc)
			continue
		}
		info.evaluator.mustPushMadeMove(pos, evalMove, move, tag, ep, hc)
		info.history.SetLastMovePlayed(move)
		info.MoveStack[0] = move
		legalRoot++

		score := -alphaBetaPV(pos, depth-1, 1, -beta, -alpha, true, true, false, info)
		if info.Stopped {
			info.evaluator.mustPop()
			pos.UnMakeMove(move, tag, ep, hc)
			info.history.SetLastMovePlayed(savedLast)
			break
		}

		// Mate threat extension: if we found a very high score, search deeper to find mate
		if score >= MATE_IN_MAX && depth <= 2 {
			score = -alphaBetaPV(pos, depth+2, 1, -beta, -alpha, true, true, false, info)
			if info.Stopped {
				info.evaluator.mustPop()
				pos.UnMakeMove(move, tag, ep, hc)
				info.history.SetLastMovePlayed(savedLast)
				break
			}
			if !info.countNode() {
				info.evaluator.mustPop()
				pos.UnMakeMove(move, tag, ep, hc)
				info.history.SetLastMovePlayed(savedLast)
				break
			}
		}

		info.evaluator.mustPop()

		pos.UnMakeMove(move, tag, ep, hc)
		info.history.SetLastMovePlayed(savedLast)
		if !info.countNode() {
			break
		}

		if score > info.BestScore {
			info.BestScore = score
			info.BestMove = move
		}

		if score > alpha {
			alpha = score
		}

		if alpha >= beta {
			break
		}
	}
	if !info.Stopped && legalRoot == 0 {
		if isInCheck(pos, pos.Turn()) {
			info.BestScore = -MATE_VALUE
		} else {
			info.BestScore = 0
		}
		info.BestMove = EmptyMove
	}

	return info
}

func alphaBeta(pos *Position, depth int, alpha int, beta int, info *SearchInfo) int {
	return alphaBetaPV(pos, depth, 0, alpha, beta, true, true, false, info) // Root is always PV
}

// unknownStaticEval marks a StaticEvalStack slot that holds no usable eval: an
// in-check node has no static eval, so it stores this instead of a number.
const unknownStaticEval = -INFINITY

// improvingVsPrevTurn reports whether the side to move is trending up — its
// static eval beats the one from its previous turn, two plies back.
//
// T17: the reference slot can be unknown (an in-check ancestor stores
// unknownStaticEval), so an unknown reference falls back four plies, and
// improving stays false when nothing usable is on the stack. Comparing against
// the sentinel as if it were a value made every such node unconditionally
// "improving", which mis-set four pruning knobs at once (RFP margin, futility
// margin, LMP threshold, LMR reduction).
func improvingVsPrevTurn(info *SearchInfo, ply int, staticEval int) bool {
	for back := 2; back <= 4; back += 2 {
		if ply < back {
			return false
		}
		if prev := info.StaticEvalStack[ply-back]; prev != unknownStaticEval {
			return staticEval > prev
		}
	}
	return false
}

// seedRootStaticEval writes the root's corrected static eval into slot 0.
//
// T17: nothing else ever writes slot 0 — alphaBetaPV is only entered at ply >= 1
// — so without this a node at ply 2 compared its eval against a zeroed slot and
// computed "is my eval positive" rather than "is my eval trending up". The value
// mirrors the non-check branch of alphaBetaPV (raw eval plus all three
// correction tables); a root in check has no static eval and seeds the sentinel.
func seedRootStaticEval(pos *Position, info *SearchInfo) {
	if isInCheck(pos, pos.Turn()) {
		info.StaticEvalStack[0] = unknownStaticEval
		return
	}
	stm := pos.Turn()
	corrIdx := pawnCorrectionIndex(pos.Board.pieces[WhitePawn], pos.Board.pieces[BlackPawn])
	nonPawnCorrIdx := nonPawnCorrectionIndex(pos.Board.GetWhitePieces()&^pos.Board.pieces[WhitePawn], pos.Board.GetBlackPieces()&^pos.Board.pieces[BlackPawn])
	minorCorrIdx := minorCorrectionIndex(pos.Board.pieces[WhiteKnight]|pos.Board.pieces[WhiteBishop], pos.Board.pieces[BlackKnight]|pos.Board.pieces[BlackBishop])
	info.StaticEvalStack[0] = info.evaluator.SearchSTM(pos) + info.history.correctionValue(stm, corrIdx) + info.history.nonPawnCorrectionValue(stm, nonPawnCorrIdx) + info.history.minorCorrectionValue(stm, minorCorrIdx)
}

// searchDrawScore applies the same terminal rules in the main and quiescence
// searches. Game history contains played positions only; RepStack contains
// ancestors of this invocation, with same-position verification entries hidden.
func searchDrawScore(pos *Position, ply int, hash uint64, info *SearchInfo) (int, bool) {
	// Checkmate ends the game before a fifty-move draw can be claimed. Check
	// this before IsDraw too, since that helper includes the halfmove clock.
	if pos.HalfMoveClock >= 100 {
		if pos.IsInCheck() && !pos.HasLegalMove() {
			return -MATE_VALUE + ply, true
		}
		return 0, true
	}
	if pos.IsDraw() {
		return 0, true
	}
	// An irreversible move makes all pre-root positions unreachable.
	if pos.Positions != nil && int(pos.HalfMoveClock) >= ply {
		pos.positionsMutex.RLock()
		count := pos.Positions[hash]
		pos.positionsMutex.RUnlock()
		if count >= 2 {
			return 0, true
		}
	}
	lo := info.RepStackLen - int(pos.HalfMoveClock)
	if lo < 0 {
		lo = 0
	}
	for i := info.RepStackLen - 1; i >= lo; i-- {
		if info.RepStack[i] == hash {
			return 0, true
		}
	}
	return 0, false
}

func alphaBetaPV(pos *Position, depth int, ply int, alpha int, beta int, isPV bool, canNull bool, cutNode bool, info *SearchInfo) int {
	if ply > info.SelDepth {
		info.SelDepth = ply
	}
	// Fixed-node and external cancellation share one stop bit. The helper keeps
	// their first decisive causes distinct for the opt-in observer.
	if info.shouldStopForControl() {
		info.Stopped = true
		return stoppedSearchScore
	}

	// Check time limit if we have a time manager (now cheap to call frequently)
	if info.TimeManager != nil {
		if info.TimeManager.ShouldStopSearch(depth) {
			info.Stopped = true
			return stoppedSearchScore
		}
	}

	// Safety check for excessive depth (use MaximumDepth constant)
	if depth > MaximumDepth {
		return info.evaluator.LegacyUndampedSTM(pos)
	}

	// Safety check for negative depth (bug prevention)
	if depth < 0 {
		return info.evaluator.LegacyUndampedSTM(pos)
	}

	// Safety check for excessive ply: no legal search should recurse past
	// MaximumDepth plies. A check extension restores depth (nextDepth == depth
	// when givesCheck), so a non-decaying checking line — perpetual check or a
	// king hunt — never bottoms out and recurses until a ply-indexed array
	// (e.g. PV[MaximumDepth]) overflows. The clock unwinds it in real games;
	// fixed-depth analysis has no clock, so it crashes outright.
	if ply >= MaximumDepth {
		return info.evaluator.LegacyUndampedSTM(pos)
	}

	// Quiescence owns the horizon frame and checks terminal positions itself.
	if depth == 0 {
		return quiescence(pos, alpha, beta, ply, info)
	}

	// Compute hash once and reuse for repetition detection and TT probe.
	hash := pos.Hash()

	if score, drawn := searchDrawScore(pos, ply, hash, info); drawn {
		return score
	}

	// Push current hash onto repetition stack
	if info.RepStackLen < len(info.RepStack) {
		info.RepStack[info.RepStackLen] = hash
		info.RepStackLen++
	}
	// Reserve this invocation's reusable move-buffer frame (docs/10 T1). frameIdx is the
	// recursion-FRAME index (not ply): IID/singular re-enter at the same ply, so each gets
	// a distinct frame here, never clobbering a live parent's buffers (docs/09 J3). Balanced
	// by the same defer as the repetition stack — no extra per-node defer.
	frameIdx := info.FrameDepth
	info.FrameDepth++
	// Pop repetition stack and release the buffer frame on function exit.
	defer func() { info.RepStackLen--; info.FrameDepth-- }()

	// Probe transposition table
	origAlpha := alpha

	ttMove, ttEvalRaw, ttDepth, ttNodeType, ttHit, ttPvEntry := info.tt.Get(hash)

	// T19: ttPv marks a node that has ever been searched as PV — either it is PV
	// now, or a previous search stored it as such. It is STICKY (once true it is
	// re-stored true) so the mark survives the node dropping out of the PV, which
	// is the whole point: those nodes still deserve less reduction later.
	nodeTTPv := isPV || (ttHit && ttPvEntry)
	info.TTProbes++
	ttEval := scoreFromTT(ttEvalRaw, ply)

	// Singular verification root: this node is being re-searched with one move
	// excluded, so the cached entry (which reflects the full search INCLUDING the
	// excluded move) must not short-circuit it — otherwise the verification just
	// returns the original score and never proves singularity. Probe still runs
	// for move ordering; only the cutoff and the (corrupting) store are skipped.
	inSingular := info.ExcludedMove != EmptyMove && ply == info.ExcludedPly

	if ttHit && !isPV && ttDepth >= int8(depth) && !inSingular {
		info.TTHits++
		switch ttNodeType {
		case Exact:
			return ttEval
		case LowerBound:
			if ttEval >= beta {
				info.TTCutoffs++
				return ttEval
			}
		case UpperBound:
			if ttEval <= alpha {
				return ttEval
			}
		}
	}

	// Internal Iterative Deepening (IID)
	// When we have no TT move at high depth, do a reduced search to find one
	if SearchToggles.IID && ttMove == EmptyMove && depth >= 4 && isPV {
		// Search at reduced depth to get a good move for ordering
		iidDepth := depth - 2
		if iidDepth < 1 {
			iidDepth = 1
		}
		// This re-enters alphaBetaPV on the SAME position (no move made). The
		// current node already pushed its hash onto info.RepStack above, so the
		// re-entry would otherwise hit its own hash in the search-path repetition
		// scan and return a bogus draw (0). Hide our entry for the duration.
		info.IIDSearches++
		info.RepStackLen--
		alphaBetaPV(pos, iidDepth, ply, alpha, beta, true, true, cutNode, info)
		info.RepStackLen++
		if info.Stopped {
			return stoppedSearchScore
		}
		// Re-probe TT to get the move we just found
		ttMove, _, _, _, _, _ = info.tt.Get(hash)
	}

	// Internal Iterative Reduction (IIR): a cut-node that reaches this depth with no
	// hash move is poorly explored — searching it at full depth pays for ordering we
	// don't have. Trim one ply instead (the modern, search-free complement to the IID
	// block above, which only helps PV nodes). It shrinks the EBF at the ~3/4 of
	// middlegame nodes that carry no TT move (2026-06-29 sd_order data). Gated to
	// cutNode (not all-nodes) and skipped in low-material positions (accPhase < 8):
	// endgames are low-branching and need depth, where reducing no-TT-move nodes cost
	// ~2-4 ply in the probe and turned the unguarded version net-negative.
	if ttMove == EmptyMove && depth >= 4 && cutNode && pos.Board.accPhase >= 8 {
		depth--
	}

	// Mate distance pruning - don't look for mates that are too far away.
	// Use ply (distance from root) directly. Earlier code computed
	// `info.Depth - depth`, but info.Depth is only set after iteration
	// completion (line 618), so during the tree walk it was either maxDepth
	// (first iteration) or the prior completed depth — i.e. wrong.
	mateAlpha := -MATE_VALUE + ply
	mateBeta := MATE_VALUE - ply - 1

	if alpha < mateAlpha {
		alpha = mateAlpha
	}
	if beta > mateBeta {
		beta = mateBeta
	}
	if alpha >= beta {
		return alpha
	}

	// Check if we're in check before attempting null move. The InCheck tag is
	// maintained for the side to move on every entry path (ParseFEN at the root,
	// makeMoveHelper after a move, preserved across IID/singular re-entry, and
	// clear after a null move since NMP only fires when !inCheck), so read it
	// instead of rescanning (N2, node-identical).
	inCheck := pos.IsInCheck()

	// Static eval for this node, computed once and reused by RFP/futility and the
	// improving heuristic. Undefined in check, so inherit the value from two plies
	// ago to keep the improving chain sane across checking plies.
	stm := pos.Turn()
	var staticEval int
	var corrIdx int        // pawn-correction slot; valid only when !inCheck
	var nonPawnCorrIdx int // non-pawn-correction slot; valid only when !inCheck
	var minorCorrIdx int   // minor-piece-correction slot; valid only when !inCheck
	// T4: corrStaticEval is the CORRECTED static (raw + correction) captured BEFORE the S7 TT-refinement
	// overwrites staticEval — the corrhist learning target. NGN's corrhist uses the gravity update
	// (updatePawnCorrection, same form as gravityUpdate), which converges to the full correction only when
	// it learns from the corrected value; learning from raw would saturate the entry to the clamp limit.
	var corrStaticEval int
	if inCheck {
		if ply >= 2 {
			staticEval = info.StaticEvalStack[ply-2]
		} else {
			staticEval = unknownStaticEval
		}
	} else {
		corrIdx = pawnCorrectionIndex(pos.Board.pieces[WhitePawn], pos.Board.pieces[BlackPawn])
		nonPawnCorrIdx = nonPawnCorrectionIndex(pos.Board.GetWhitePieces()&^pos.Board.pieces[WhitePawn], pos.Board.GetBlackPieces()&^pos.Board.pieces[BlackPawn])
		minorCorrIdx = minorCorrectionIndex(pos.Board.pieces[WhiteKnight]|pos.Board.pieces[WhiteBishop], pos.Board.pieces[BlackKnight]|pos.Board.pieces[BlackBishop])
		staticEval = info.evaluator.SearchSTM(pos) + info.history.correctionValue(stm, corrIdx) + info.history.nonPawnCorrectionValue(stm, nonPawnCorrIdx) + info.history.minorCorrectionValue(stm, minorCorrIdx)
		corrStaticEval = staticEval // T4: capture the corrected static (raw + pawn + non-pawn + minor) BEFORE S7 overwrites it (all tables' learn target)
		// S7: a search-refined TT score is a sharper static estimate than a fresh
		// eval for the margin heuristics below (RFP/futility/NMP/improving). Use it
		// when the stored bound brackets it. Skip mate scores (would distort margins)
		// and the singular-verification root (keep its pruning behaviour unchanged).
		if ttHit && !inSingular && abs(ttEval) < MATE_IN_MAX {
			if ttNodeType == Exact ||
				(ttNodeType == LowerBound && ttEval > staticEval) ||
				(ttNodeType == UpperBound && ttEval < staticEval) {
				staticEval = ttEval
			}
		}
	}
	info.StaticEvalStack[ply] = staticEval

	// improving: static eval better than two plies ago (our previous turn). When
	// the position is trending up we reduce one less ply in LMR below.
	improving := !inCheck && improvingVsPrevTurn(info, ply, staticEval)

	// Extended futility pruning (depths 1-3) and reverse futility pruning
	var futilityPrune bool

	// Reverse futility pruning - if position is very good, return early.
	// A2a: extended from depth<=3 to depth<=7 with the margin gated on
	// improving (SF11 217*(d-improving), Ethereal/Weiss equivalents) — every
	// reference engine prunes harder when the eval is NOT trending up; the
	// ungated depth<=8 extension inflated sharp-position trees (probe-rejected).
	if SearchToggles.RFP && depth <= 7 && !inCheck && beta-alpha == 1 {
		imp := 0
		if improving {
			imp = 1
		}
		margin := REVERSE_FUTILITY_MARGIN * (depth - imp)
		if staticEval-margin >= beta {
			info.RFPPrunes++
			return staticEval - margin
		}
	}

	// Extended futility pruning. W1: reach extended from depth<=3 to depth<=8
	// (CounterGo prunes quiets this deep) with the CG-grounded margin
	// FUTILITY_MARGIN*(depth+improving). The improving term widens the margin
	// when the eval is trending up (prune less), mirroring the A2a RFP gate that
	// made the depth extension sound where the ungated A2 version inflated trees.
	if SearchToggles.Futility && depth <= FUTILITY_MAX_DEPTH && !inCheck && !isPV { // Don't prune in PV nodes
		imp := 0
		if improving {
			imp = 1
		}
		margin := FUTILITY_MARGIN * (depth + imp)
		if staticEval+margin <= alpha {
			futilityPrune = true
		}
	}

	// Note: Razoring pruning removed - futility pruning already provides effective pruning
	// with better performance characteristics for this engine architecture

	// Null-move pruning at cut nodes only. C3 fix: the prior gate required a full
	// window (beta-alpha>1), but PVS scouts every non-PV node with a null window, so
	// NMP fired only on the PV spine — almost never. Gate on !isPV (every cut node) +
	// staticEval>=beta (standard soundness/efficiency guard) + canNull (no two null
	// moves in a row, replacing the implicit beta-alpha>1 brake). Zugzwang guard:
	// in pawn-only endgames passing can be best, so require non-pawn material.
	// W8: skip NMP when the TT says this node fails low (UpperBound with ttEval < beta).
	// NMP's "node is already >= beta" premise is contradicted there, so the null search
	// is wasted (CG + Ethereal converge on this guard; NullMoveTries should drop).
	// T12: restrict to true cut nodes. `!isPV` admits BOTH cut and all nodes -- in PVS an
	// all-node is expected to fail low, so NMP's "this node is already >= beta" premise is
	// wrong there and the null search is speculative work against the node's own expectation.
	// This makes the code match the claim the comment above already makes.
	if SearchToggles.NullMove && !inCheck && depth >= NULL_MOVE_MIN_DEPTH && canNull && !isPV && cutNode && staticEval >= beta &&
		hasNonPawnMaterial(&pos.Board, pos.Turn()) &&
		!(ttHit && ttNodeType == UpperBound && ttEval < beta) {
		// Set lastMovePlayed to EmptyMove so children don't pick spurious
		// counter-move replies keyed on our own prior move.
		savedLast := info.history.GetLastMovePlayed()
		info.NullMoveTries++
		evalNull := info.evaluator.mustPrepareNull(pos)
		oldEP := pos.MakeNullMove()
		info.evaluator.mustPushMadeNull(pos, evalNull, oldEP)
		info.history.SetLastMovePlayed(EmptyMove)
		info.MoveStack[ply] = EmptyMove

		// Floor the null child depth at 0. At depth==NULL_MOVE_MIN_DEPTH (3) the
		// reduction depth-NULL_MOVE_R-1 = -1 trips the negative-depth guard and
		// verifies the null move with a raw static eval (tactically blind) instead
		// of quiescence; flooring routes it through qsearch like the depth>=4 path.
		nullDepth := depth - (NULL_MOVE_R + 1 + depth/6) // NMP reduction: tunable base NULL_MOVE_R (def 3 => R+1=4) + depth-scaling; CG/Ethereal/Weiss converge
		if nullDepth < 0 {
			nullDepth = 0
		}
		score := -alphaBetaPV(pos, nullDepth, ply+1, -beta, -beta+1, false, false, !cutNode, info)

		info.evaluator.mustPop()
		pos.UnMakeNullMove(oldEP)
		info.history.SetLastMovePlayed(savedLast)
		if info.Stopped {
			return stoppedSearchScore
		}

		if score >= beta {
			info.NullMoveCutoffs++
			return beta
		}
	}

	// Acquire this invocation's reusable buffers from the frame pool (used by probcut
	// below AND the main move loop); fall back to a fresh stack frame only when the pool
	// is exhausted by deep re-entrant nesting (correct, just not allocation-free).
	// Kept ABOVE probcut so probcut's generation is allocation-free too. docs/10 T1.
	var frame *searchFrame
	if info.Frames != nil && frameIdx < len(info.Frames) {
		frame = &info.Frames[frameIdx]
	} else {
		frame = new(searchFrame)
	}

	// Probcut pruning: if a reduced-depth search at a higher beta fails high,
	// we can prune because the full search would likely also fail high
	if SearchToggles.Probcut && !inSingular && !isPV && !inCheck && depth >= 5 && abs(beta) < MATE_VALUE-100 {
		probcutBeta := beta + 200 // Higher beta threshold

		// Generate into the reused frame buffer (NOT a per-node [256]Move that escapes
		// to the heap via the non-inlined GenerateMovesIntoBuffer — that was 34MB/run).
		var capBuffer [64]Move
		capCount := 0
		tempCount := GenerateMovesIntoBuffer(pos, frame.moveBuffer[:])
		for i := 0; i < tempCount && capCount < 64; i++ {
			if frame.moveBuffer[i].IsCapture() {
				capBuffer[capCount] = frame.moveBuffer[i]
				capCount++
			}
		}

		for i := 0; i < capCount; i++ {
			move := capBuffer[i]

			// Only consider captures with good SEE (skip the full swap when the
			// capture is provably non-losing — see seeNonLosingByMVV).
			if !seeNonLosingByMVV(move) && staticExchangeEvaluation(pos, move) < 0 {
				continue
			}

			savedLast := info.history.GetLastMovePlayed()
			evalMove := info.evaluator.mustPrepareMove(pos, move)
			ep, tag, hc, _ := pos.MakeMove(move)
			if isInCheck(pos, move.MovingPiece().Color()) {
				pos.UnMakeMove(move, tag, ep, hc)
				continue
			}
			info.evaluator.mustPushMadeMove(pos, evalMove, move, tag, ep, hc)
			info.history.SetLastMovePlayed(move)
			info.MoveStack[ply] = move

			score := -quiescence(pos, -probcutBeta, -probcutBeta+1, ply+1, info)
			if info.Stopped {
				info.evaluator.mustPop()
				pos.UnMakeMove(move, tag, ep, hc)
				info.history.SetLastMovePlayed(savedLast)
				return stoppedSearchScore
			}

			if score >= probcutBeta {
				score = -alphaBetaPV(pos, depth-4, ply+1, -probcutBeta, -probcutBeta+1, false, true, !cutNode, info)
				if info.Stopped {
					info.evaluator.mustPop()
					pos.UnMakeMove(move, tag, ep, hc)
					info.history.SetLastMovePlayed(savedLast)
					return stoppedSearchScore
				}
			}

			info.evaluator.mustPop()

			pos.UnMakeMove(move, tag, ep, hc)
			info.history.SetLastMovePlayed(savedLast)

			if score >= probcutBeta {
				info.ProbcutPrunes++
				return beta
			}
		}
	}

	// Singular extension detection
	// If we have a TT move that's clearly best, extend its search
	singularMove := EmptyMove
	if SearchToggles.Singular && ttHit && ttMove != EmptyMove && depth >= SINGULAR_DEPTH &&
		info.ExcludedMove == EmptyMove && // Not already in singular search
		(ttNodeType == Exact || ttNodeType == LowerBound) &&
		ttDepth >= int8(depth-3) {
		singularMove = ttMove
	}

	// Use pseudo-legal moves for performance, check legality only when trying moves
	// (frame acquired above, before probcut, so all generation here is allocation-free).
	numMoves := GenerateMovesIntoBuffer(pos, frame.moveBuffer[:])
	moves := frame.moveBuffer[:numMoves]
	if numMoves == 0 {
		if inCheck {
			// Checkmate - distance from root is `ply` directly.
			mateDistance := ply
			if mateDistance > 1000 {
				mateDistance = 1000
			}
			return -MATE_VALUE + mateDistance
		}
		// Stalemate is a draw regardless of material balance. Scoring it -500
		// when "winning" breaks the zero-sum invariant — the opponent would read
		// the same drawn position as +500 and steer into it.
		return 0
	}

	// Order moves, prioritizing TT move if available. Scores are written eagerly; the sort
	// itself is driven incrementally inside the loop (selectNextMove) so the selection tail
	// past a beta cutoff is never paid. Node-identical to a full sort (docs/10 T2a).
	// prev2 (our side's previous move, for follow-up history) is constant for the
	// whole node: [ply-2] belongs to a live ancestor suspended mid-loop.
	prev2 := EmptyMove
	if ply >= 2 {
		prev2 = info.MoveStack[ply-2]
	}
	ordered := frame.ordered[:]
	scores := frame.scores[:]
	orderedCount, ttIndex := info.history.scoreMovesIntoBuffer(moves, ply, info.history.GetLastMovePlayed(), prev2, ttMove, ttHit, ordered, scores, pos, true)
	info.MoveLoopNodes++
	if ttIndex >= 0 {
		info.TTMoveListed++
	}
	bestScore := -INFINITY
	bestMove := EmptyMove
	moveCount := 0
	legalTried := 0

	// Track quiet/capture moves searched for history penalization (reused frame buffers)
	searchedQuiets := frame.quiets[:]
	searchedQuietCount := 0
	searchedCaptures := frame.captures[:]
	searchedCaptureCount := 0
	completedHistoryQuietCount := 0

	capturesScored := false // lazySEE: real capture scores not yet materialized
	for oi := 0; oi < orderedCount; oi++ {
		// Deferred selection sort: pull the next-best move into ordered[oi]. Doing this
		// per-iteration (rather than fully up front) skips the sort tail after a cutoff.
		if oi == 0 && ttIndex >= 0 {
			// The TT move carries the unique top score (100000), so selectNextMove(oi=0)
			// would scan the whole list only to find it. Swap it to the front in O(1) — the
			// identical result — skipping that O(n) scan on the ~88% of nodes that cut on the
			// TT move (node-identical: verified by scripts/nodecheck.sh).
			if ttIndex != 0 {
				ordered[0], ordered[ttIndex] = ordered[ttIndex], ordered[0]
				scores[0], scores[ttIndex] = scores[ttIndex], scores[0]
			}
		} else {
			// Capture SEE scores were deferred (lazySEE): materialize them once,
			// immediately before the first selection compare. All scores are final
			// before any comparison, so the emitted order is identical to eager
			// scoring — but the ~88% of cutting nodes that cut on the hoisted TT
			// move above never reach this line and skip ordering SEE entirely.
			if !capturesScored {
				materializeCaptureScores(ordered, scores, orderedCount, pos)
				capturesScored = true
			}
			selectNextMove(ordered, scores, orderedCount, oi)
		}
		move := ordered[oi]
		moveCount++

		// Skip excluded move at the singular verification root only (ply-scoped so
		// a coincidentally-identical move deeper in the verification subtree isn't
		// also skipped).
		if inSingular && move == info.ExcludedMove {
			continue
		}

		// Check time before evaluating each move (now cheap to call). A stopped node
		// returns like every other stop site: its partial result must not reach the
		// TT or correction history.
		if info.TimeManager != nil && info.TimeManager.ShouldStopSearch(depth) {
			info.Stopped = true
			return stoppedSearchScore
		}

		// Futility pruning - skip quiet moves if position is hopeless (but never prune
		// the TT move or ANY promotion — a promotion is forcing and material-changing, beyond any
		// futility margin, so the "this quiet move can't reach alpha" premise is false).
		// legalTried > 0 guard (F1): never prune before a single legal move has been
		// searched, else a node whose every move is futile prunes them all, hits
		// legalTried==0, and returns 0 (false stalemate) instead of the true fail-low.
		// W1 exemptions: never futility-prune killers or the counter-move. At the
		// d3 reach these rarely mattered, but extending to d8 prunes quiet
		// maneuvering moves that hold alpha on closed positions — pruning a killer/
		// counter there fails the node low and triggers a re-search (the closed-
		// position tree inflation the EBF probe flagged). Keep searching them.
		if futilityPrune && !move.IsCapture() && move != ttMove && move.PromoType() == NoType && legalTried > 0 &&
			!info.history.IsKillerMove(move, ply) && move != info.history.GetCounterMove(info.history.GetLastMovePlayed()) {
			info.FutilityPrunes++
			continue
		}

		// Late Move Pruning (LMP) - skip very late quiet moves at low depths
		// Only apply in non-PV nodes, not in check, and for quiet moves
		// Use legalTried (not moveCount) since moveCount includes illegal moves
		if SearchToggles.LMP && !isPV && !inCheck && depth > 0 && depth < 9 &&
			!move.IsCapture() && move.PromoType() == NoType && move != ttMove && legalTried > 0 {
			threshold := lmpThresholdFor(depth, improving)
			if legalTried > threshold {
				info.LMPPrunes++
				continue
			}
		}

		// SEE pruning for bad captures in main search
		// Skip obviously losing captures at low depths (not in PV, not TT move).
		// legalTried > 0 guard (F1): never prune before one legal move is searched.
		if SearchToggles.SEEPrune && !isPV && !inCheck && depth <= 4 && move.IsCapture() && move != ttMove && legalTried > 0 {
			// Skip the full swap when the capture is provably non-losing (SEE >= 0 > -100).
			if !seeNonLosingByMVV(move) && staticExchangeEvaluation(pos, move) < -100 { // losing more than a pawn
				info.SEECapPrunes++
				continue
			}
		}

		// SEE pruning of QUIET moves: at shallow depth, skip a quiet move whose
		// piece can be captured on its destination for a net material loss (a
		// "soft hang"). Threshold scales with depth so deeper nodes prune only
		// larger losses. Exclude the TT move, promotions (tactical), and king
		// moves — the king's infinite SEE weight poisons the swap, and a pinned
		// enemy "attacker" of a king square may not be a legal capture.
		if SearchToggles.SEEPrune && !isPV && !inCheck && depth <= 4 &&
			!move.IsCapture() && move != ttMove && move.PromoType() == NoType && legalTried > 0 {
			if mp := move.MovingPiece(); mp != WhiteKing && mp != BlackKing {
				if staticExchangeEvaluation(pos, move) < -80*depth {
					info.SEEQuietPrunes++
					continue
				}
			}
		}

		// History pruning: skip quiet moves with very bad history at low depths
		if SearchToggles.HistPrune && !isPV && !inCheck && depth <= 3 && !move.IsCapture() && move.PromoType() == NoType && move != ttMove {
			histScore := info.history.GetHistoryScore(move, info.history.GetLastMovePlayed(), prev2)
			if histScore < -1000 && legalTried > 3 { // Very bad history and not early move
				info.HistPrunes++
				continue
			}
		}

		// MakeMove alone cannot validate castling: its rook relocation can shield an
		// attack on the king's original square. Validate the complete path first.
		if move.IsCastle() && !isLegalCastle(pos, move) {
			continue
		}

		// Capture the predecessor move (the one that brought us into this node)
		// so children see the correct lastMovePlayed for counter-move lookups,
		// and extension logic uses the true parent move.
		savedLast := info.history.GetLastMovePlayed()
		evalMove := info.evaluator.mustPrepareMove(pos, move)
		ep, tag, hc, _ := pos.MakeMove(move)
		movingColor := move.MovingPiece().Color()
		if isInCheck(pos, movingColor) {
			pos.UnMakeMove(move, tag, ep, hc)
			continue
		}
		info.evaluator.mustPushMadeMove(pos, evalMove, move, tag, ep, hc)
		info.history.SetLastMovePlayed(move)
		info.MoveStack[ply] = move
		legalTried++

		// Track quiet moves for history penalization
		if !move.IsCapture() && move.PromoType() == NoType && searchedQuietCount < 64 {
			searchedQuiets[searchedQuietCount] = move
			searchedQuietCount++
		}
		if h1NewHistoryProducer && h1QuietMove(move) {
			h1AppendCompletedQuiet(&frame.completedHistoryQuiets, &completedHistoryQuietCount, move)
		}
		// Track captures for capture-history penalization
		if move.IsCapture() && searchedCaptureCount < 64 {
			searchedCaptures[searchedCaptureCount] = move
			searchedCaptureCount++
		}

		// Refined Check Extension logic - balance tactical strength with performance.
		// givesCheck == "is the side now to move in check" == exactly what makeMoveHelper
		// already computed and cached in the InCheck tag (position.go:263), so read the
		// cached tag instead of rescanning the board (N2, node-identical).
		givesCheck := pos.IsInCheck()

		// Compute next depth with check extension
		nextDepth := depth - 1
		if givesCheck && depth > 1 {
			nextDepth++
			info.CheckExtensions++
		}

		// A9: recapture extension REMOVED (probe). It was NGN's #1 extension source
		// (~14-19K/400K nodes); Counter 3.8 (2994 CCRL) ships none. Trimming it is a
		// simplification + tree-size cut; gated at [-5,0] non-regression on the cloud.
		// (savedLast is still read by the singular extension below.)

		// Passed pawn extension: extend an actually-passed pawn pushed to the rank ONE
		// step from promotion (7th for White, 2nd for Black). Was 6th/7th rank, but the
		// 6th-rank half still extended quiet endgame pawn shuffles that storm the eg tree
		// (autopsy) without being immediate promotion threats; the real tactical urgency
		// is the about-to-queen push. pos is post-MakeMove, so the pushed pawn sits on its
		// destination; it is passed iff its front span is clear of enemy pawns.
		if move.MovingPiece().Type() == Pawn && !move.IsCapture() {
			dest := int(move.Destination())
			destRank := dest / 8
			color := move.MovingPiece().Color()
			if (color == White && destRank >= 6) || (color == Black && destRank <= 1) {
				var oppPawns uint64
				if color == White {
					oppPawns = pos.Board.GetBitboardOf(BlackPawn)
				} else {
					oppPawns = pos.Board.GetBitboardOf(WhitePawn)
				}
				if passedPawnFrontSpan[color][dest]&oppPawns == 0 && nextDepth+1 <= depth {
					nextDepth++
					info.PassedPawnExtensions++
				}
			}
		}

		// Singular Extension: extend if this move is clearly the best
		if move == singularMove && singularMove != EmptyMove {
			// Unmake to do singular search at the parent position.
			// Restore lastMove to savedLast so the singular search (which is
			// evaluating other replies from the parent node) sees the correct
			// predecessor.
			info.evaluator.mustPop()
			pos.UnMakeMove(move, tag, ep, hc)
			info.history.SetLastMovePlayed(savedLast)

			singularBeta := int(ttEval) - (SINGULAR_MARGIN*depth)/32 // singular margin: tunable base SINGULAR_MARGIN (def 64 => 2*depth) depth-scaled; CG/Ethereal/Weiss converge here
			info.SingularTries++
			info.ExcludedMove = singularMove
			info.ExcludedPly = ply           // the verification re-searches THIS node's position
			singularDepth := (depth - 1) / 2 // singular-margin: half-depth verification (was depth-4), convergent
			if singularDepth < 1 {
				singularDepth = 1
			}
			// The verification search re-enters alphaBetaPV on the SAME (parent)
			// position with the singular move excluded. Hide this node's own
			// repetition-stack entry so the re-entry doesn't match its own hash
			// and bail out with a bogus draw (0) before doing any work.
			info.RepStackLen--
			singularScore := alphaBetaPV(pos, singularDepth, ply, singularBeta-1, singularBeta, false, true, cutNode, info)
			info.RepStackLen++
			info.ExcludedMove = EmptyMove
			if info.Stopped {
				return stoppedSearchScore
			}

			// Re-make our move and restore the global to it for children.
			// The verification re-entry ran at this same ply and overwrote
			// MoveStack[ply] with its own moves; restore ours.
			ep, tag, hc, _ = pos.MakeMove(move)
			info.evaluator.mustPushMadeMove(pos, evalMove, move, tag, ep, hc)
			info.history.SetLastMovePlayed(move)
			info.MoveStack[ply] = move

			if singularScore < singularBeta {
				nextDepth++
				info.SingularExtensions++
			} else if !isPV && ttEval >= beta {
				// Singular-neg-ext (W-SE2): the verification failed high — another move
				// also reached singularBeta — and the TT eval already exceeds beta, so
				// this is very likely a multi-good-move cut-node. Shave a ply instead of
				// searching full depth (multicut-LITE; the return-beta multicut stays
				// rejected, the A3 lesson). Non-PV only — never reduce a PV node.
				nextDepth--
			}
		}

		if nextDepth > depth+1 {
			// Guardrail: limit extensions to at most +1 beyond current depth
			nextDepth = depth + 1
		}

		// Per-path extension budget (anti-explosion). The per-node guardrail above
		// caps a single node at +1, but a check extension only RESTORES the decrement
		// (nextDepth == depth), so a non-decaying checking line keeps depth flat and
		// can recurse toward MaximumDepth (the seldepth-111, ~20x-node explosion).
		// Bound the CUMULATIVE net extension on this root-to-leaf path: the child's
		// extensions == nextDepth + (ply+1) - RootDepth, so cap nextDepth to keep that
		// <= EXTENSION_BUDGET. Once the budget is spent, checks decrement normally and
		// the line terminates. Generous enough to leave normal tactical search
		// untouched (verified node-identical on Kiwipete d10-12), finite enough that
		// the C4/C5 LMR-rescan vectors can no longer recurse without bound.
		if maxNextDepth := info.RootDepth - (ply + 1) + EXTENSION_BUDGET; nextDepth > maxNextDepth {
			nextDepth = maxNextDepth
			info.ExtBudgetClamps++
		}

		var score int

		// Enhanced Principal Variation Search (PVS) with PV-node optimization
		if legalTried == 1 {
			// First move: search with full window and PV flag
			score = -alphaBetaPV(pos, nextDepth, ply+1, -beta, -alpha, isPV, true, !isPV && !cutNode, info)
		} else {
			// Non-first moves: enhanced PVS with PV-aware reductions

			// PV-aware Late Move Reductions (use legalTried, not moveCount which includes illegal moves)
			reduction := 0
			if SearchToggles.LMR && depth >= LMR_FULL_DEPTH && legalTried > LMR_MOVE_THRESHOLD &&
				!move.IsCapture() && move.PromoType() == NoType && !inCheck && !givesCheck {

				// Stockfish-style log-based base reduction; non-PV gets +1
				d := depth
				if d > 63 {
					d = 63
				}
				m := legalTried
				if m > 63 {
					m = 63
				}
				reduction = lmrTable[d][m]
				if !isPV {
					reduction++
				}

				// Reduce one less ply when our position is improving.
				if !improving {
					reduction++
				}
				// History-based LMR adjustment:
				// Reduce more for moves with bad history, less for moves with good history
				histScore := info.history.GetHistoryScore(move, savedLast, prev2)
				if h1NormalizedLMRConsumer {
					reduction += h1NormalizedLMRTerm(histScore)
				} else if histScore < -500 {
					reduction++ // Bad history = reduce more
				} else if histScore > 1000 {
					reduction-- // Good history = reduce less
				}
				if reduction < 0 {
					reduction = 0
				}

				// T19: reduce less at nodes that have ever been PV. NGN's FMC is ~84%
				// vs elite ~90% and T3a proved ADDING reduction hurts here, so this
				// moves with the measured weakness rather than against it.
				if nodeTTPv {
					reduction--
					if reduction < 0 {
						reduction = 0
					}
				}

				// Killer move reduction adjustment - don't reduce killer moves as much
				if info.history.IsKillerMove(move, ply) {
					reduction-- // Reduce less for killer moves
					if reduction < 0 {
						reduction = 0
					}
				}

				// Cap maximum reduction
				if reduction >= depth-1 {
					reduction = depth - 2
				}
				if reduction < 0 {
					reduction = 0
				}
				// Track LMR usage
				if reduction > 0 {
					info.LMRReductions++
					info.LMRPliesSum += uint64(reduction)
				}
			}

			// Compute reduced depth (check extension already applied via nextDepth)
			reducedDepth := depth - 1 - reduction
			if reducedDepth >= depth {
				reducedDepth = depth - 1 - reduction
			}

			// Search with null window first (not PV)
			info.NullWindowScouts++
			score = -alphaBetaPV(pos, reducedDepth, ply+1, -alpha-1, -alpha, false, true, true, info)
			if info.Stopped {
				info.evaluator.mustPop()
				pos.UnMakeMove(move, tag, ep, hc)
				info.history.SetLastMovePlayed(savedLast)
				return stoppedSearchScore
			}

			// LMR re-search (C4): a reduced search that beats alpha may have
			// under-counted the move, so re-verify it at full depth with the null
			// window. This must NOT be gated by score<beta — in a non-PV node
			// beta==alpha+1, so that guard was never satisfiable and reduced fail-highs
			// went UNVERIFIED in ~99% of the tree (reduced cutoffs taken on faith, and
			// the check extension in nextDepth never actually applied off the PV). The
			// per-path extension budget bounds the check recursion this re-search opens.
			if reduction > 0 && score > alpha {
				info.LMRReSearches++
				score = -alphaBetaPV(pos, nextDepth, ply+1, -alpha-1, -alpha, false, true, !cutNode, info)
				if info.Stopped {
					info.evaluator.mustPop()
					pos.UnMakeMove(move, tag, ep, hc)
					info.history.SetLastMovePlayed(savedLast)
					return stoppedSearchScore
				}
			}
			// PVS full-window re-search: only a PV node has a window wider than the null
			// window, so only there does an in-window score need its exact value.
			if isPV && score > alpha && score < beta {
				info.PVReSearches++
				score = -alphaBetaPV(pos, nextDepth, ply+1, -beta, -alpha, isPV, true, false, info)
				if info.Stopped {
					info.evaluator.mustPop()
					pos.UnMakeMove(move, tag, ep, hc)
					info.history.SetLastMovePlayed(savedLast)
					return stoppedSearchScore
				}
			}
		}

		info.evaluator.mustPop()

		pos.UnMakeMove(move, tag, ep, hc)
		// Restore predecessor for the next sibling (and for the caller if we
		// return via a beta cutoff or loop exit below).
		info.history.SetLastMovePlayed(savedLast)
		if info.Stopped {
			return stoppedSearchScore
		}
		if !info.countNode() {
			return stoppedSearchScore
		}

		if score > bestScore {
			bestScore = score
			bestMove = move
		}

		if score >= beta {
			if h1NewHistoryProducer {
				info.history.h1UpdateCompletedNode(
					h1CompletedQuiets(&frame.completedHistoryQuiets, completedHistoryQuietCount),
					bestMove,
					savedLast,
					prev2,
					depth,
					origAlpha,
					bestScore,
				)
			}
			info.BetaCutoffs++
			if legalTried == 1 {
				info.FirstMoveCutoffs++
			}
			// EBF-attribution: cut-index histogram, depth bands, and cut-move class.
			// Classified BEFORE the killer/counter updates below so the labels read
			// the tables as ordering saw them (still approximate — child searches
			// mutate them — exact for TT/capture/promo).
			switch {
			case legalTried <= 3:
				info.CutIdxHist[legalTried-1]++
			case legalTried <= 7:
				info.CutIdxHist[3]++
			default:
				info.CutIdxHist[4]++
			}
			info.CutTriedSum += uint64(legalTried)
			db := depthBand(depth)
			info.BcutBand[db]++
			if legalTried == 1 {
				info.FmcBand[db]++
			}
			var cutClass int
			switch {
			case ttHit && move == ttMove:
				cutClass = 0
				info.CutByTT++
			case move.IsCapture():
				cutClass = 1
				info.CutByCapture++
			case move.PromoType() != NoType:
				cutClass = 2
				info.CutByPromo++
			case info.history.IsKillerMove(move, ply):
				cutClass = 3
				info.CutByKiller++
			case move == info.history.GetCounterMove(savedLast):
				cutClass = 4
				info.CutByCounter++
			default:
				cutClass = 5
				info.CutByQuietHist++
			}
			// Among NON-first cutoffs (the ordering misses), record what class
			// of move actually deserved to be first, by how late it landed.
			// Diagnostic only — never read by search (node-identity preserved).
			if legalTried >= 2 {
				band := 2
				if legalTried == 2 {
					band = 0
				} else if legalTried == 3 {
					band = 1
				}
				info.CutMissByClass[cutClass][band]++
			}
			// Policy 00/01 retain cutoff-only gravity history. Policy 10/11 has
			// already applied the completed-winner response exactly once above.
			if !h1NewHistoryProducer {
				info.history.UpdateHistoryTable(move, savedLast, prev2, depth)
			}
			// Update killer moves for quiet moves that cause beta cutoffs
			info.history.UpdateKillerMoves(move, ply)
			// Counter move: the cutoff move is a good reply to savedLast,
			// the move that actually brought us into this node.
			info.history.UpdateCounterMove(move, savedLast)

			// Penalize all quiet moves that were tried but didn't cause the cutoff
			if !h1NewHistoryProducer {
				for i := 0; i < searchedQuietCount; i++ {
					quietMove := searchedQuiets[i]
					if quietMove != move {
						info.history.PenalizeHistoryTable(quietMove, savedLast, prev2, depth)
					}
				}
			}

			// Capture history: reward a cutting capture, penalize tried captures that
			// didn't cut (whether the cutoff move was a capture or a quiet).
			if move.IsCapture() {
				info.history.UpdateCaptureHistory(move, depth)
			}
			for i := 0; i < searchedCaptureCount; i++ {
				if searchedCaptures[i] != move {
					info.history.PenalizeCaptureHistory(searchedCaptures[i], depth)
				}
			}

			// Learn the pawn-structure + non-pawn + minor corrections from this fail-high before returning.
			info.history.maybeUpdatePawnCorrection(stm, corrIdx, corrStaticEval, bestScore, beta, depth, bestMove, inCheck)
			info.history.maybeUpdateNonPawnCorrection(stm, nonPawnCorrIdx, corrStaticEval, bestScore, beta, depth, bestMove, inCheck)
			info.history.maybeUpdateMinorCorrection(stm, minorCorrIdx, corrStaticEval, bestScore, beta, depth, bestMove, inCheck)

			// Store in transposition table as Lower Bound (Cut-Node). Skip at the
			// singular verification root: its result is a move-excluded search and
			// would corrupt the real entry for this position.
			if !inSingular {
				info.tt.Set(hash, move, scoreToTT(score, ply), int8(depth), LowerBound, nodeTTPv)
			}
			return score
		}

		if score > alpha {
			alpha = score
		}
	}

	// Check for checkmate/stalemate: no legal moves were found
	if legalTried == 0 {
		if inCheck {
			// Checkmate - distance from root is `ply` directly.
			mateDistance := ply
			if mateDistance > 1000 {
				mateDistance = 1000
			}
			return -MATE_VALUE + mateDistance
		}
		// Stalemate is a draw
		return 0
	}

	if h1NewHistoryProducer && !info.Stopped {
		info.history.h1UpdateCompletedNode(
			h1CompletedQuiets(&frame.completedHistoryQuiets, completedHistoryQuietCount),
			bestMove,
			info.history.GetLastMovePlayed(),
			prev2,
			depth,
			origAlpha,
			bestScore,
		)
	}

	// Learn the pawn-structure + non-pawn + minor corrections from the completed search of this node.
	info.history.maybeUpdatePawnCorrection(stm, corrIdx, corrStaticEval, bestScore, beta, depth, bestMove, inCheck)
	info.history.maybeUpdateNonPawnCorrection(stm, nonPawnCorrIdx, corrStaticEval, bestScore, beta, depth, bestMove, inCheck)
	info.history.maybeUpdateMinorCorrection(stm, minorCorrIdx, corrStaticEval, bestScore, beta, depth, bestMove, inCheck)

	// Store in transposition table
	var nodeType NodeType
	if bestScore <= origAlpha {
		// No move improved alpha - Upper Bound (All-Node)
		nodeType = UpperBound
		info.AllNodes++
		info.AllTriedSum += uint64(legalTried)
	} else {
		// Best move improved alpha - Exact (PV-Node)
		nodeType = Exact
		info.PVNodesExact++
	}

	// Skip the store at the singular verification root (move-excluded result must
	// not overwrite the real entry for this position).
	if !inSingular {
		info.tt.Set(hash, bestMove, scoreToTT(bestScore, ply), int8(depth), nodeType, nodeTTPv)
	}
	return bestScore
}

// depthBand buckets remaining depth for the EBF-attribution counters:
// 0: d1-2, 1: d3-5, 2: d6-9, 3: d10+.
func depthBand(depth int) int {
	switch {
	case depth <= 2:
		return 0
	case depth <= 5:
		return 1
	case depth <= 9:
		return 2
	}
	return 3
}

func quiescence(pos *Position, alpha int, beta int, ply int, info *SearchInfo) int {
	return quiescenceWithDepth(pos, alpha, beta, ply, info, 0)
}

func quiescenceWithDepth(pos *Position, alpha int, beta int, ply int, info *SearchInfo, qDepth int) int {
	if ply > info.SelDepth {
		info.SelDepth = ply
	}
	if ply >= MaximumDepth {
		return info.evaluator.SearchSTM(pos)
	}

	// Fixed-node and external cancellation share one stop bit. Attribute before
	// unwinding so a node limit cannot be relabeled as an external stop.
	if info.shouldStopForControl() {
		info.Stopped = true
		return stoppedSearchScore
	}
	// Capture/evasion trees can run for many nodes without returning to the
	// main search. They must observe the clock too; depth 0 checks the hard
	// tournament deadline without applying an iteration's soft stop.
	if info.TimeManager != nil && info.TimeManager.ShouldStopSearch(0) {
		info.Stopped = true
		return stoppedSearchScore
	}

	hash := pos.Hash()
	if score, drawn := searchDrawScore(pos, ply, hash, info); drawn {
		return score
	}
	inCheck := pos.IsInCheck()

	// qDepth remains a diagnostic/call-site argument, not a capture horizon.
	// Qsearch lower bounds share depth-0 TT entries, so a capture-only cap here
	// would let a shorter search certify a bound for a later, longer one.
	// TT cutoff probe. Qsearch is depth 0; any cached entry at depth >= 0
	// covers us (main search stores at depth >= 1 only). Mate scores need
	// converting from node-relative TT encoding back to root-relative.
	// Also keep ttMove for move ordering even when no cutoff is taken.
	ttMove, ttEvalRaw, _, ttNodeType, ttHit, ttPvEntry := info.tt.Get(hash)
	info.QTTProbes++
	if ttHit {
		info.QTTHits++
		ttEval := scoreFromTT(ttEvalRaw, ply)
		switch ttNodeType {
		case Exact:
			info.QTTCutoffs++
			return ttEval
		case LowerBound:
			if ttEval >= beta {
				info.QTTCutoffs++
				return ttEval
			}
		case UpperBound:
			if ttEval <= alpha {
				info.QTTCutoffs++
				return ttEval
			}
		}
	}

	info.QNodes++
	if !info.countNode() {
		return stoppedSearchScore
	}

	// Reusable move buffer from the recursion-frame pool (docs/10 T1b): replaces the
	// per-qnode stack `[256]Move` that escaped via the non-inlined GenerateMovesIntoBuffer
	// (~7GB allocated/search here, 96.9% of all search allocs). Indexed by the FrameDepth
	// counter so the in-check buffer survives the recursive qsearch call; stack fallback on
	// pool exhaustion. Only `moveBuffer` is needed here — the in-check and stalemate-probe
	// branches are mutually exclusive per call, and `captures [64]Move` below stays on the
	// stack (inlined copy, never escaped).
	qframeIdx := info.FrameDepth
	info.FrameDepth++
	// Checks and evasions can repeat even though ordinary captures cannot.
	// Preserve the caller's stack length on every return, including cutoffs.
	savedRepLen := info.RepStackLen
	if info.RepStackLen < len(info.RepStack) {
		info.RepStack[info.RepStackLen] = hash
		info.RepStackLen++
	}
	defer func() { info.FrameDepth--; info.RepStackLen = savedRepLen }()
	var qframe *searchFrame
	if info.Frames != nil && qframeIdx < len(info.Frames) {
		qframe = &info.Frames[qframeIdx]
	} else {
		qframe = new(searchFrame)
	}

	// In-check branch: must search all evasions, stand-pat is illegal here.
	if inCheck {
		moveBuffer := qframe.moveBuffer[:]
		moveCount := GenerateMovesIntoBuffer(pos, moveBuffer)

		// Move ttMove to the front if it's among the legal moves. Cheap O(n)
		// pass that pays off when the TT suggestion produces an early cutoff.
		if ttMove != EmptyMove {
			for i := 0; i < moveCount; i++ {
				if moveBuffer[i] == ttMove {
					moveBuffer[0], moveBuffer[i] = moveBuffer[i], moveBuffer[0]
					break
				}
			}
		}

		bestScore := -INFINITY
		legalMoves := 0

		for i := 0; i < moveCount; i++ {
			move := moveBuffer[i]
			if move.IsCastle() && !isLegalCastle(pos, move) {
				continue
			}
			evalMove := info.evaluator.mustPrepareMove(pos, move)
			ep, tag, hc, _ := pos.MakeMove(move)
			if isInCheck(pos, move.MovingPiece().Color()) {
				pos.UnMakeMove(move, tag, ep, hc)
				continue
			}
			info.evaluator.mustPushMadeMove(pos, evalMove, move, tag, ep, hc)
			legalMoves++

			score := -quiescenceWithDepth(pos, -beta, -alpha, ply+1, info, qDepth+1)
			info.evaluator.mustPop()
			pos.UnMakeMove(move, tag, ep, hc)
			if info.Stopped {
				return stoppedSearchScore
			}

			if score > bestScore {
				bestScore = score
			}
			if score >= beta {
				info.QBetaCutoffs++
				return beta
			}
			if score > alpha {
				alpha = score
			}
		}

		if legalMoves == 0 {
			// Checkmate at this node — distance from root is `ply` directly.
			// Earlier code used info.Depth + qDepth, which was wrong because
			// info.Depth is the last completed iteration depth, not the current
			// search distance.
			return -MATE_VALUE + ply
		}

		return bestScore
	}

	// Not in check. A legal pawn push proves this is not stalemate without
	// generating every piece's moves. Fall back to the full probe when needed.
	if !hasLegalPawnPush(pos) {
		moveBuffer := qframe.moveBuffer[:]
		moveCount := GenerateMovesIntoBuffer(pos, moveBuffer)
		hasLegalMove := false
		for i := 0; i < moveCount && !hasLegalMove; i++ {
			move := moveBuffer[i]
			if move.IsCastle() {
				hasLegalMove = isLegalCastle(pos, move)
			} else {
				// Board-only make: this legality probe reads nothing but the board
				// (isInCheck), so the hash/tag/clock/InCheck-tag bookkeeping of the
				// full MakeMove — paid at EVERY horizon entry — is skipped.
				pos.partialMakeMove(move)
				if !isInCheck(pos, move.MovingPiece().Color()) {
					hasLegalMove = true
				}
				pos.partialUnMakeMove(move)
			}
		}
		if !hasLegalMove {
			// Stalemate is a draw regardless of material balance.
			return 0
		}
	}

	// Stand-pat via the exact eval cache (qsearch re-evaluates at ~every non-check
	// qnode and is ~42% of middlegame nodes). The lazy-margin fast-out this used to
	// route through (evaluateLazyStandPat) is pinned dead — qsearchLazyMargin=100000
	// means lazyFailHigh never fires — so this is value-identical; re-route through a
	// cache-aware lazy path if that margin is ever unpinned (SHELVED 2026-06-02).
	standPat := info.history.correctedStandPat(pos, info.evaluator)
	if standPat >= beta {
		info.QStandPatCuts++
		return beta
	}

	bestScore := standPat
	bestMove := EmptyMove
	if standPat > alpha {
		alpha = standPat
	}

	// Captures only. Quiet checks are dropped because moveGivesCheck misses
	// discovered, en-passant, and promotion checks; an incomplete quiet-check
	// path is worse than captures-only at the horizon.
	captureCount := GenerateCapturesIntoBuffer(pos, info.CaptureBuffer[:])
	var captures [64]Move
	copy(captures[:], (*info.CaptureBuffer)[:captureCount])

	// MVV/LVA scores for an INCREMENTAL selection-sort (select-max per iter, stop at
	// cutoff — avoids the upfront full-sort cost the void "-129" comment cited).
	// Encoding-only (victim/attacker seeWeight, no board access); ttMove forced top.
	var capScore [64]int
	for i := 0; i < captureCount; i++ {
		capScore[i] = 8*int(captures[i].CapturedPiece().seeWeight()) - int(captures[i].MovingPiece().seeWeight())
		if captures[i] == ttMove {
			capScore[i] = 1 << 30
		}
	}

	for i := 0; i < captureCount; i++ {
		// incremental selection: pull the best-scored remaining capture to i
		best := i
		for j := i + 1; j < captureCount; j++ {
			if capScore[j] > capScore[best] {
				best = j
			}
		}
		captures[i], captures[best] = captures[best], captures[i]
		capScore[i], capScore[best] = capScore[best], capScore[i]
		move := captures[i]

		// Delta pruning: skip captures that cannot possibly raise alpha. For
		// promotion-captures, include the promotion gain so we don't prune a
		// winning Pxf8=Q that would actually deliver a queen.
		gain := int(move.CapturedPiece().seeWeight())
		if move.PromoType() != NoType {
			promoPiece := GetPiece(move.PromoType(), move.MovingPiece().Color())
			gain += int(promoPiece.seeWeight()) - 100
		}
		if standPat+gain+DELTA_MARGIN <= alpha {
			info.QDeltaPrunes++
			continue
		}

		// SEE pruning: skip losing captures (skip the full swap when provably non-losing).
		if !seeNonLosingByMVV(move) && staticExchangeEvaluation(pos, move) < 0 {
			info.QSEEPrunes++
			continue
		}

		evalMove := info.evaluator.mustPrepareMove(pos, move)
		ep, tag, hc, _ := pos.MakeMove(move)
		if isInCheck(pos, move.MovingPiece().Color()) {
			pos.UnMakeMove(move, tag, ep, hc)
			continue
		}
		info.evaluator.mustPushMadeMove(pos, evalMove, move, tag, ep, hc)

		score := -quiescenceWithDepth(pos, -beta, -alpha, ply+1, info, qDepth+1)
		info.evaluator.mustPop()
		pos.UnMakeMove(move, tag, ep, hc)
		if info.Stopped {
			return stoppedSearchScore
		}

		if score > bestScore {
			bestScore = score
			bestMove = move
		}
		if score >= beta {
			info.QBetaCutoffs++
			// Fail-high: store a lower bound (the cutting capture) at depth 0 (S3).
			// Only fail-highs are stored — the common non-failing-high case would
			// flood the TT with depth-0 entries and evict deeper main-search ones.
			// T19: carry the ttPv mark forward. Qsearch already probed this slot, so
			// preserving it is free — and dropping it would let a depth-0 store erase a
			// mark the main search had earned, silently weakening the LMR adjustment on
			// exactly the nodes it is meant to protect. (SF preserves ttPv likewise.)
			info.tt.Set(pos.Hash(), bestMove, scoreToTT(beta, ply), 0, LowerBound, ttHit && ttPvEntry)
			info.QSearchTTStores++
			return beta
		}
		if score > alpha {
			alpha = score
		}
	}

	return bestScore
}
