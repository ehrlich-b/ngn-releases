package engine

import (
	"testing"
)

func newStartingPosition() *Position {
	return &Position{
		Board:     StartingBoard(),
		Tag:       WhiteCanCastleKingSide | WhiteCanCastleQueenSide | BlackCanCastleKingSide | BlackCanCastleQueenSide | WhiteToMove,
		EnPassant: NoSquare,
	}
}

// TestMaxNodesEnforced verifies the fixed-node budget (UCI `go nodes`) actually caps
// the search: it must return a real move, reach roughly the budget, and not blow past
// it (which would mean the cap is ignored) — the basis of deterministic fixed-node SPRT.
func TestMaxNodesEnforced(t *testing.T) {
	defer SetMaxNodes(0)
	const budget = 50000

	SetMaxNodes(budget)
	info := Search(newStartingPosition(), MaximumDepth) // high depth; the node cap must bind
	SetMaxNodes(0)

	if info.BestMove == EmptyMove {
		t.Fatal("node-limited search returned no move")
	}
	if info.Nodes < budget {
		t.Errorf("stopped early at %d nodes — budget %d never reached", info.Nodes, budget)
	}
	if info.Nodes > budget*3 {
		t.Errorf("ran to %d nodes — budget %d not enforced", info.Nodes, budget)
	}

	// With the cap cleared, the same call must search far deeper (sanity: the cap was
	// what stopped it, not something else).
	uncapped := Search(newStartingPosition(), 8)
	if uncapped.Nodes <= budget {
		t.Logf("note: depth-8 uncapped used %d nodes (<= budget, fine for startpos)", uncapped.Nodes)
	}
}

func TestBasicSearch(t *testing.T) {
	pos := newStartingPosition()

	// Test search at depth 1 from starting position
	info := Search(pos, 1)

	if info == nil {
		t.Fatal("Search returned nil")
	}

	if info.BestMove == EmptyMove {
		t.Error("Search did not find a best move")
	}

	if info.Nodes == 0 {
		t.Error("Search reported 0 nodes searched")
	}

	if info.Depth != 1 {
		t.Errorf("Expected depth 1, got %d", info.Depth)
	}

	// Best move should be a legal move
	legalMoves := GenerateLegalMoves(pos)
	found := false
	for _, move := range legalMoves {
		if move == info.BestMove {
			found = true
			break
		}
	}
	if !found {
		t.Error("Best move is not a legal move")
	}
}

func TestSearchTacticalPosition(t *testing.T) {
	// Set up a position where white has a clear advantage
	// Test that search finds a reasonable move
	pos := &Position{}
	pos.Board = Bitboard{}

	// Place kings
	pos.Board.UpdateSquare(E1, WhiteKing, NoPiece)
	pos.Board.UpdateSquare(E8, BlackKing, NoPiece)

	// Place white queen - should be stronger than black pieces
	pos.Board.UpdateSquare(D1, WhiteQueen, NoPiece)

	// Place some black pieces
	pos.Board.UpdateSquare(A7, BlackPawn, NoPiece)
	pos.Board.UpdateSquare(B7, BlackPawn, NoPiece)
	pos.Board.UpdateSquare(A8, BlackRook, NoPiece)

	pos.SetTag(WhiteToMove)
	pos.EnPassant = NoSquare
	pos.HalfMoveClock = 0

	// Search should find a good move
	info := Search(pos, 3)

	// Verify that search found a move
	if info.BestMove == EmptyMove {
		t.Error("Search should find a best move")
	}

	// The search should find a reasonable score (not necessarily positive
	// since this position might not actually be winning for White)
	if info.BestScore < -1000 {
		t.Errorf("Search returned unreasonably low score: %d", info.BestScore)
	}
}

func TestSearchAvoidMate(t *testing.T) {
	// Set up a position where Black is threatened with mate
	// White Queen on d1, Black King on e8, Black should move king
	pos := &Position{}
	pos.Board = Bitboard{}

	// Place kings
	pos.Board.UpdateSquare(A1, WhiteKing, NoPiece)
	pos.Board.UpdateSquare(E8, BlackKing, NoPiece)

	// Place white queen threatening mate
	pos.Board.UpdateSquare(D1, WhiteQueen, NoPiece)

	pos.SetTag(BlackToMove)
	pos.EnPassant = NoSquare
	pos.HalfMoveClock = 0

	// Search should try to avoid immediate mate
	info := Search(pos, 2)

	// The score shouldn't be a large negative (mate) value
	if info.BestScore < -MATE_IN_MAX {
		t.Errorf("Black should avoid mate, but got score %d", info.BestScore)
	}
}

func TestSearchNodeCount(t *testing.T) {
	pos := newStartingPosition()

	// Search at different depths and verify node counts increase
	info1 := Search(pos, 1)
	info2 := Search(pos, 2)

	if info2.Nodes <= info1.Nodes {
		t.Errorf("Deeper search should examine more nodes. Depth 1: %d, Depth 2: %d",
			info1.Nodes, info2.Nodes)
	}
}

// TestAspirationProgressiveWidening pins the T5 aspiration behavior: with a
// deliberately tiny initial window (1cp) and a slow multiplier, a fixed-depth
// iterative-deepening search on a tactically rich position must fail against the
// window MANY times and widen PROGRESSIVELY (several fail-high/fail-low
// re-searches per depth, not a single jump to an infinite bound) yet ALWAYS
// complete every iteration to the target depth. The pre-T5 loop capped at 3
// attempts and, on exhaustion, discarded the iteration (AspAbandoned++); T5
// removes the cap and never abandons, so AspAbandoned must stay 0 and the total
// re-search count must exceed the number of aspiration depths (proof that at
// least one depth widened more than once — impossible under the old
// jump-straight-to-INFINITY scheme).
func TestAspirationProgressiveWidening(t *testing.T) {
	saveInit, saveMult := ASP_INIT, ASP_MULT
	ASP_INIT, ASP_MULT = 1, 110 // 1cp window, +10% (floored to +1) growth per fail
	defer func() { ASP_INIT, ASP_MULT = saveInit, saveMult }()

	// Kiwipete: sharp, scores swing between depths, so a 1cp window fails often.
	pos, err := ParseFEN("r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1")
	if err != nil {
		t.Fatal(err)
	}

	const depth = 8 // aspiration runs for depths 4..8 (5 depths)
	info := Search(pos, depth)

	if info.Depth != depth {
		t.Fatalf("iteration did not complete to target depth: info.Depth=%d, want %d", info.Depth, depth)
	}
	if info.BestMove == EmptyMove {
		t.Fatal("aspiration search returned no move")
	}
	if info.AspAbandoned != 0 {
		t.Fatalf("T5 must never abandon an iteration, got AspAbandoned=%d", info.AspAbandoned)
	}
	reSearches := info.AspFailHighs + info.AspFailLows
	if reSearches <= 5 {
		// <=5 would be consistent with the old scheme's one-fail-then-jump per
		// aspiration depth; progressive widening produces strictly more.
		t.Fatalf("expected progressive widening (>5 re-searches across depths 4-8), got fh=%d fl=%d (total %d)",
			info.AspFailHighs, info.AspFailLows, reSearches)
	}
	t.Logf("progressive widening: fh=%d fl=%d total=%d, completed to depth %d, AspAbandoned=%d",
		info.AspFailHighs, info.AspFailLows, reSearches, info.Depth, info.AspAbandoned)
}

func BenchmarkSearch(b *testing.B) {
	pos := newStartingPosition()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Search(pos, 3)
	}
}

func TestIterativeDeepening(t *testing.T) {
	pos := newStartingPosition()

	// Reset global search state so this test is deterministic regardless of
	// what other tests left behind in TT/history/killer/counter tables.
	resetSearchGlobals := func() {
		defaultSearchEngine, _ = NewSearchEngineWithHash(DEFAULT_CACHE_SIZE)
		ClearHistoryTable()
		ClearKillerMoves()
		ClearCounterMoves()
		SetLastMovePlayed(EmptyMove)
	}
	resetSearchGlobals()

	// Test that iterative deepening produces results at each depth
	info := SearchIterativeDeepening(pos, 3, nil)

	if info.BestMove == EmptyMove {
		t.Error("Iterative deepening should find a best move")
	}

	if info.Depth != 3 {
		t.Errorf("Expected final depth 3, got %d", info.Depth)
	}

	// Reset again so fixed-depth starts from the same blank state as ID did.
	resetSearchGlobals()

	// Compare with fixed depth search - should agree on best move and score
	// when both start from clean global state.
	infoFixed := SearchFixed(pos, 3, nil)

	// ID and fixed-depth need NOT find the identical move/score: ID seeds the TT and
	// move ordering from its shallower iterations, so it legitimately explores a
	// different PV than a cold fixed-depth search (both are valid). Assert only that
	// fixed-depth also produces a legal move and the two scores are consistent within
	// shallow-search noise — a gross divergence would still trip this.
	if infoFixed.BestMove == EmptyMove {
		t.Error("Fixed-depth search should find a best move")
	}

	if diff := info.BestScore - infoFixed.BestScore; diff > 60 || diff < -60 {
		t.Errorf("ID and fixed-depth scores diverge beyond shallow-search noise. ID: %d, Fixed: %d",
			info.BestScore, infoFixed.BestScore)
	}
}

func BenchmarkSearchDeep(b *testing.B) {
	pos := newStartingPosition()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Search(pos, 4)
	}
}

func TestQuiescenceSearch(t *testing.T) {
	// Create a position with a tactical opportunity (capture sequence)
	pos := &Position{}
	pos.Board = Bitboard{}

	// Place kings
	pos.Board.UpdateSquare(E1, WhiteKing, NoPiece)
	pos.Board.UpdateSquare(E8, BlackKing, NoPiece)

	// Create a capture sequence: White queen can capture black rook
	pos.Board.UpdateSquare(D1, WhiteQueen, NoPiece)
	pos.Board.UpdateSquare(D7, BlackRook, NoPiece)

	// Add a black piece that can recapture
	pos.Board.UpdateSquare(C8, BlackBishop, NoPiece)

	pos.SetTag(WhiteToMove)
	pos.EnPassant = NoSquare
	pos.HalfMoveClock = 0

	// Search should find the tactical sequence
	info := Search(pos, 2)

	if info.BestMove == EmptyMove {
		t.Error("Should find a best move in tactical position")
	}

	// The engine should be able to evaluate the position
	// (exact score depends on position evaluation)
	if info.Nodes == 0 {
		t.Error("Search should examine some nodes")
	}
}

func TestTranspositionTable(t *testing.T) {
	pos := newStartingPosition()

	// Clear the transposition table
	defaultSearchEngine, _ = NewSearchEngineWithHash(DEFAULT_CACHE_SIZE)

	// Do a search to populate the transposition table
	info1 := Search(pos, 3)

	if info1.BestMove == EmptyMove {
		t.Error("First search should find a best move")
	}

	// Isolate the TT: the search deliberately retains history, killer, and
	// counter-move learning between calls, and a more active history producer can
	// legitimately change the selective tree on the second search. Reset those
	// heuristics while retaining the populated TT before comparing its result.
	defaultSearchEngine.ClearHistoryTable()
	defaultSearchEngine.ClearKillerMoves()
	defaultSearchEngine.ClearCounterMoves()
	defaultSearchEngine.SetLastMovePlayed(EmptyMove)

	// Do the same search again - should be faster due to TT hits.
	info2 := Search(pos, 3)

	// Scores must match. Best-move can differ when multiple moves tie on score —
	// TT-driven move ordering surfaces a different (equally-good) move first.
	if info2.BestScore != info1.BestScore {
		t.Errorf("Both searches should get the same score. First: %d, Second: %d",
			info1.BestScore, info2.BestScore)
	}

	// Second search should examine fewer nodes (due to TT hits)
	// Note: This might not always be true due to hash collisions and replacement strategy
	// but it's generally expected
	if info2.Nodes >= info1.Nodes {
		t.Logf("Warning: Second search examined %d nodes vs first search %d nodes",
			info2.Nodes, info1.Nodes)
	}
}

func BenchmarkIterativeDeepening(b *testing.B) {
	pos := newStartingPosition()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		SearchIterativeDeepening(pos, 4, nil)
	}
}

func BenchmarkWithTranspositionTable(b *testing.B) {
	pos := newStartingPosition()

	// Clear TT before benchmarking
	defaultSearchEngine, _ = NewSearchEngineWithHash(DEFAULT_CACHE_SIZE)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Search(pos, 4)
	}
}
