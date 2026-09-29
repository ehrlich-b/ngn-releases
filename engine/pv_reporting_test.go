package engine

import (
	"reflect"
	"strings"
	"testing"
)

const pvStartFEN = "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"

type pvPositionSnapshot struct {
	board          Bitboard
	enPassant      Square
	tag            PositionTag
	hash           uint64
	halfMoveClock  uint8
	positions      map[uint64]int
	positionsIsNil bool
}

func snapshotPVPosition(pos *Position) pvPositionSnapshot {
	pos.positionsMutex.RLock()
	positions := make(map[uint64]int, len(pos.Positions))
	for hash, count := range pos.Positions {
		positions[hash] = count
	}
	positionsIsNil := pos.Positions == nil
	pos.positionsMutex.RUnlock()
	return pvPositionSnapshot{
		board: pos.Board, enPassant: pos.EnPassant, tag: pos.Tag, hash: pos.hash,
		halfMoveClock: pos.HalfMoveClock, positions: positions, positionsIsNil: positionsIsNil,
	}
}

func assertPVPositionRestored(t *testing.T, pos *Position, before pvPositionSnapshot) {
	t.Helper()
	after := snapshotPVPosition(pos)
	if after.board != before.board || after.enPassant != before.enPassant || after.tag != before.tag ||
		after.hash != before.hash || after.halfMoveClock != before.halfMoveClock ||
		after.positionsIsNil != before.positionsIsNil || !reflect.DeepEqual(after.positions, before.positions) {
		t.Fatalf("position changed during PV extraction\nbefore: %+v\nafter:  %+v", before, after)
	}
}

func mustPVPosition(t *testing.T, fen string) *Position {
	t.Helper()
	pos, err := ParseFEN(fen)
	if err != nil {
		t.Fatalf("ParseFEN(%q): %v", fen, err)
	}
	return pos
}

func mustPVMove(t *testing.T, pos *Position, text string) Move {
	t.Helper()
	move, err := ParseUCIMove(pos, text)
	if err != nil {
		t.Fatalf("ParseUCIMove(%q): %v", text, err)
	}
	return move
}

func makePVMove(t *testing.T, pos *Position, move Move) (Square, PositionTag, uint8) {
	t.Helper()
	ep, tag, hc, ok := pos.MakeMove(move)
	if !ok {
		t.Fatalf("MakeMove(%s) failed", MoveToAlgebraic(move))
	}
	return ep, tag, hc
}

func seedExactPV(t *testing.T, pos *Position, texts []string) []Move {
	t.Helper()
	type undoEntry struct {
		move Move
		ep   Square
		tag  PositionTag
		hc   uint8
	}
	moves := make([]Move, 0, len(texts))
	undos := make([]undoEntry, 0, len(texts))
	for i, text := range texts {
		move := mustPVMove(t, pos, text)
		moves = append(moves, move)
		if i > 0 {
			defaultSearchEngine.TTStore(pos.Hash(), move, 0, 1, Exact, true)
		}
		ep, tag, hc := makePVMove(t, pos, move)
		undos = append(undos, undoEntry{move: move, ep: ep, tag: tag, hc: hc})
	}
	for i := len(undos) - 1; i >= 0; i-- {
		u := undos[i]
		pos.UnMakeMove(u.move, u.tag, u.ep, u.hc)
	}
	return moves
}

func playPVHistory(t *testing.T, pos *Position, text string) {
	t.Helper()
	for _, moveText := range strings.Fields(text) {
		move := mustPVMove(t, pos, moveText)
		if _, _, _, ok := pos.GameMakeMove(move); !ok {
			t.Fatalf("GameMakeMove(%q) failed", moveText)
		}
	}
}

func movesToPVStrings(moves []Move) []string {
	texts := make([]string, len(moves))
	for i, move := range moves {
		texts[i] = MoveToAlgebraic(move)
	}
	return texts
}

func withFreshPVTT(t *testing.T) {
	t.Helper()
	saved := defaultSearchEngine
	defaultSearchEngine, _ = NewSearchEngineWithHash(1)
	t.Cleanup(func() { defaultSearchEngine = saved })
}

func TestExtractPVFromTTStopsAtRealHistoryRepetition(t *testing.T) {
	withFreshPVTT(t)
	pos := mustPVPosition(t, pvStartFEN)
	playPVHistory(t, pos, "b1c3 g7g6 c3b5 f7f6 d2d3 d7d6 e2e4 e7e5 h2h4 c7c5 b5c3 h7h5 c3d5 f8g7 g1f3 g8e7 f1e2 e8g8 e1g1 b8c6 c2c4 a7a5 c1d2 a5a4 d5c3 c6d4 f3d4 e5d4 c3d5 g8h7 f1e1 a8a7 a1b1 c8e6 b2b4 a4b3 d1b3 e7c6 d2f4 d8d7 d5b6 d7d8 b6d5")
	if count := pos.Positions[pos.Hash()]; count != 2 {
		t.Fatalf("root occurrence count = %d, want 2", count)
	}
	before := snapshotPVPosition(pos)
	moves := seedExactPV(t, pos, []string{"d8d7", "d5b6", "d7d8", "b6d5", "d8d7"})
	got := ExtractPVFromTT(pos, moves[0], 32)
	want := []string{"d8d7", "d5b6", "d7d8", "b6d5"}
	if gotText := movesToPVStrings(got); !reflect.DeepEqual(gotText, want) {
		t.Fatalf("PV = %v, want terminal-safe %v", gotText, want)
	}
	if got[0] != moves[0] {
		t.Fatalf("first legal move changed: got %s, want %s", MoveToAlgebraic(got[0]), MoveToAlgebraic(moves[0]))
	}
	assertPVPositionRestored(t, pos, before)
}

func TestExtractPVFromTTRootCycleWithSparseHistory(t *testing.T) {
	for _, history := range []string{"nil", "root-only"} {
		t.Run(history, func(t *testing.T) {
			withFreshPVTT(t)
			pos := mustPVPosition(t, "4k3/8/8/8/8/8/8/R3K3 w - - 0 1")
			rootHash := pos.Hash()
			if history == "nil" {
				pos.Positions = nil
			} else {
				pos.Positions = map[uint64]int{rootHash: 1}
			}
			before := snapshotPVPosition(pos)
			moves := seedExactPV(t, pos, []string{"a1a2", "e8e7", "a2a1", "e7e8", "a1a2", "e8e7", "a2a1", "e7e8"})
			got := ExtractPVFromTT(pos, moves[0], 32)
			if len(got) != 8 {
				t.Fatalf("PV length = %d (%v), want 8 moves ending at the third root occurrence", len(got), movesToPVStrings(got))
			}
			assertPVPositionRestored(t, pos, before)
		})
	}
}

func TestExtractPVFromTTRejectsTerminalRootWithoutMutation(t *testing.T) {
	tests := []struct {
		name            string
		fen             string
		firstMove       string
		repetitionCount int
		terminal        func(*Position) bool
	}{
		{
			name: "repetition", fen: pvStartFEN, firstMove: "e2e4", repetitionCount: 3,
			terminal: func(pos *Position) bool { return pos.Positions[pos.Hash()] >= 3 },
		},
		{
			name: "fifty-move", fen: "7k/8/8/8/8/8/R7/K7 w - - 100 1", firstMove: "a2b2",
			terminal: func(pos *Position) bool { return pos.HalfMoveClock >= 100 },
		},
		{
			name: "insufficient-material", fen: "7k/8/8/8/8/8/8/K7 w - - 0 1", firstMove: "a1a2",
			terminal: func(pos *Position) bool { return pos.IsDraw() },
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			withFreshPVTT(t)
			pos := mustPVPosition(t, test.fen)
			if test.repetitionCount != 0 {
				pos.Positions[pos.Hash()] = test.repetitionCount
			}
			first := mustPVMove(t, pos, test.firstMove)
			if !test.terminal(pos) || !pvDrawTerminal(pos, pos.Positions[pos.Hash()]) {
				t.Fatalf("root does not satisfy expected terminal predicate")
			}
			before := snapshotPVPosition(pos)
			if got := ExtractPVFromTT(pos, first, 32); got != nil {
				t.Fatalf("PV = %v, want nil for terminal root", movesToPVStrings(got))
			}
			assertPVPositionRestored(t, pos, before)
		})
	}

	for _, test := range []struct {
		name string
		fen  string
	}{
		{name: "checkmate", fen: "7k/6Q1/6K1/8/8/8/8/8 b - - 0 1"},
		{name: "stalemate", fen: "7k/5K2/6Q1/8/8/8/8/8 b - - 0 1"},
	} {
		t.Run(test.name, func(t *testing.T) {
			withFreshPVTT(t)
			pos := mustPVPosition(t, test.fen)
			if pos.HasLegalMove() {
				t.Fatal("root unexpectedly has a legal move")
			}
			before := snapshotPVPosition(pos)
			if got := ExtractPVFromTT(pos, Move(1), 32); got != nil {
				t.Fatalf("PV = %v, want nil for terminal root", movesToPVStrings(got))
			}
			assertPVPositionRestored(t, pos, before)
		})
	}
}

func TestExtractPVFromTTStopsOnFirstMoveDraw(t *testing.T) {
	tests := []struct {
		name, fen string
		moves     []string
		terminal  func(*Position) bool
	}{
		{name: "checkmate", fen: "7k/8/6QK/8/8/8/8/8 w - - 0 1", moves: []string{"g6g7"}, terminal: func(pos *Position) bool { return pos.IsInCheck() && !pos.HasLegalMove() }},
		{name: "stalemate", fen: "7k/8/5K2/6Q1/8/8/8/8 w - - 0 1", moves: []string{"g5g6"}, terminal: func(pos *Position) bool { return !pos.IsInCheck() && !pos.HasLegalMove() }},
		{name: "fifty-move", fen: "7k/8/8/8/8/8/R7/K7 w - - 99 1", moves: []string{"a2b2", "h8g8"}, terminal: func(pos *Position) bool { return pos.HalfMoveClock == 100 }},
		{name: "insufficient-material", fen: "7k/8/8/8/8/8/2r5/KB6 w - - 0 1", moves: []string{"b1c2", "h8g8"}, terminal: func(pos *Position) bool { return pos.IsDraw() }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			withFreshPVTT(t)
			pos := mustPVPosition(t, test.fen)
			before := snapshotPVPosition(pos)
			moves := seedExactPV(t, pos, test.moves)
			ep, tag, hc := makePVMove(t, pos, moves[0])
			if !test.terminal(pos) || !pvPositionTerminal(pos, 1) {
				t.Fatalf("first move %s did not create the expected terminal position", test.moves[0])
			}
			pos.UnMakeMove(moves[0], tag, ep, hc)
			got := ExtractPVFromTT(pos, moves[0], 32)
			if len(got) != 1 || got[0] != moves[0] {
				t.Fatalf("PV = %v, want terminal-causing move %s only", movesToPVStrings(got), test.moves[0])
			}
			assertPVPositionRestored(t, pos, before)
		})
	}
}

func TestExtractPVFromTTStopsOnFirstMoveRepetition(t *testing.T) {
	withFreshPVTT(t)
	pos := mustPVPosition(t, "4k3/8/8/8/8/8/8/R3K3 w - - 0 1")
	first := mustPVMove(t, pos, "a1a2")
	ep, tag, hc := makePVMove(t, pos, first)
	childHash := pos.Hash()
	next := mustPVMove(t, pos, "e8e7")
	pos.UnMakeMove(first, tag, ep, hc)
	pos.Positions[childHash] = 2
	defaultSearchEngine.TTStore(childHash, next, 0, 1, Exact, true)
	before := snapshotPVPosition(pos)
	got := ExtractPVFromTT(pos, first, 32)
	if len(got) != 1 || got[0] != first {
		t.Fatalf("PV = %v, want first move only at third child occurrence", movesToPVStrings(got))
	}
	assertPVPositionRestored(t, pos, before)
}

func TestExtractPVFromTTBoundMissIllegalAndMaxLength(t *testing.T) {
	tests := []struct {
		name              string
		nodeType          NodeType
		setEntry, illegal bool
		max               int
		want              []string
	}{
		{name: "miss", max: 32, want: []string{"e2e4"}},
		{name: "non-exact-bound", setEntry: true, nodeType: LowerBound, max: 32, want: []string{"e2e4"}},
		{name: "illegal-exact-move", setEntry: true, nodeType: Exact, illegal: true, max: 32, want: []string{"e2e4"}},
		{name: "max-length", setEntry: true, nodeType: Exact, max: 2, want: []string{"e2e4", "e7e5"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			withFreshPVTT(t)
			pos := mustPVPosition(t, pvStartFEN)
			before := snapshotPVPosition(pos)
			moves := seedExactPV(t, pos, []string{"e2e4", "e7e5", "g1f3"})
			if !test.setEntry {
				defaultSearchEngine, _ = NewSearchEngineWithHash(1)
			} else {
				ep, tag, hc := makePVMove(t, pos, moves[0])
				ttMove := moves[1]
				if test.illegal {
					ttMove = moves[0]
				}
				defaultSearchEngine.TTStore(pos.Hash(), ttMove, 0, 1, test.nodeType, true)
				pos.UnMakeMove(moves[0], tag, ep, hc)
			}
			got := ExtractPVFromTT(pos, moves[0], test.max)
			if gotText := movesToPVStrings(got); !reflect.DeepEqual(gotText, test.want) {
				t.Fatalf("PV = %v, want %v", gotText, test.want)
			}
			assertPVPositionRestored(t, pos, before)
		})
	}
}

func TestExtractPVFromTTRejectsInvalidRootInputsWithoutMutation(t *testing.T) {
	withFreshPVTT(t)
	pos := mustPVPosition(t, pvStartFEN)
	before := snapshotPVPosition(pos)
	legal := mustPVMove(t, pos, "e2e4")
	for _, test := range []struct {
		name  string
		move  Move
		limit int
	}{
		{name: "empty-move", move: EmptyMove, limit: 32},
		{name: "zero-limit", move: legal, limit: 0},
		{name: "illegal-first-move", move: Move(1), limit: 32},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := ExtractPVFromTT(pos, test.move, test.limit); got != nil {
				t.Fatalf("PV = %v, want nil", movesToPVStrings(got))
			}
			assertPVPositionRestored(t, pos, before)
		})
	}
}
