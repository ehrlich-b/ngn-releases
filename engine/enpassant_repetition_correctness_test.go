package engine

import (
	"reflect"
	"sort"
	"testing"
)

// These six fixtures and their legal-move/perft expectations are pinned by
// experiments/2026-10-01-ep-repetition-correctness/stockfish-probe.json
// (SHA-256 897f7ebd0c3adc602839d7782673ddbecdd7c1bc72c58dc73fb424f8884b283e,
// Stockfish 18 executable SHA-256
// 6b087694916228c905a5e14db74cca8c7e5643602226af1fa5d42353c455b9f9).
type enPassantRepetitionFixture struct {
	name             string
	startFEN         string
	push             string
	epFEN            string
	noEPFEN          string
	epMove           string
	cycle            []string
	wantEPMoves      []string
	wantNoEPMoves    []string
	wantEPPerft2     uint64
	wantNoEPPerft2   uint64
	illegalEnPassant bool
}

var enPassantRepetitionFixtures = []enPassantRepetitionFixture{
	{
		name: "pinned_white_capturer", illegalEnPassant: true,
		startFEN: "6k1/3p4/8/K3P2r/8/8/8/8 b - - 0 1", push: "d7d5",
		epFEN: "6k1/8/8/K2pP2r/8/8/8/8 w - d6 0 2", noEPFEN: "6k1/8/8/K2pP2r/8/8/8/8 w - - 0 2",
		epMove: "e5d6", cycle: []string{"a5a4", "h5h6", "a4a5", "h6h5"},
		wantEPMoves:   []string{"a5a4", "a5a6", "a5b4", "a5b5", "a5b6", "e5e6"},
		wantNoEPMoves: []string{"a5a4", "a5a6", "a5b4", "a5b5", "a5b6", "e5e6"},
		wantEPPerft2:  95, wantNoEPPerft2: 95,
	},
	{
		name: "pinned_black_capturer", illegalEnPassant: true,
		startFEN: "8/8/8/8/R2p3k/8/4P3/1K6 w - - 0 1", push: "e2e4",
		epFEN: "8/8/8/8/R2pP2k/8/8/1K6 b - e3 0 1", noEPFEN: "8/8/8/8/R2pP2k/8/8/1K6 b - - 0 1",
		epMove: "d4e3", cycle: []string{"h4h5", "a4a3", "h5h4", "a3a4"},
		wantEPMoves:   []string{"d4d3", "h4g3", "h4g4", "h4g5", "h4h3", "h4h5"},
		wantNoEPMoves: []string{"d4d3", "h4g3", "h4g4", "h4g5", "h4h3", "h4h5"},
		wantEPPerft2:  95, wantNoEPPerft2: 95,
	},
	{
		name: "unrelated_check_white_capturer", illegalEnPassant: true,
		startFEN: "4b1k1/3p4/8/4P3/K7/8/8/8 b - - 0 1", push: "d7d5",
		epFEN: "4b1k1/8/8/3pP3/K7/8/8/8 w - d6 0 2", noEPFEN: "4b1k1/8/8/3pP3/K7/8/8/8 w - - 0 2",
		epMove: "e5d6", cycle: []string{"a4a3", "e8f7", "a3a4", "f7e8"},
		wantEPMoves:   []string{"a4a3", "a4a5", "a4b3", "a4b4"},
		wantNoEPMoves: []string{"a4a3", "a4a5", "a4b3", "a4b4"},
		wantEPPerft2:  52, wantNoEPPerft2: 52,
	},
	{
		name: "unrelated_check_black_capturer", illegalEnPassant: true,
		startFEN: "8/8/8/7k/3p4/8/4P3/1K1B4 w - - 0 1", push: "e2e4",
		epFEN: "8/8/8/7k/3pP3/8/8/1K1B4 b - e3 0 1", noEPFEN: "8/8/8/7k/3pP3/8/8/1K1B4 b - - 0 1",
		epMove: "d4e3", cycle: []string{"h5h6", "d1c2", "h6h5", "c2d1"},
		wantEPMoves:   []string{"h5g5", "h5g6", "h5h4", "h5h6"},
		wantNoEPMoves: []string{"h5g5", "h5g6", "h5h4", "h5h6"},
		wantEPPerft2:  52, wantNoEPPerft2: 52,
	},
	{
		name:     "legal_white_capturer",
		startFEN: "6k1/3p4/7r/K3P3/8/8/8/8 b - - 0 1", push: "d7d5",
		epFEN: "6k1/8/7r/K2pP3/8/8/8/8 w - d6 0 2", noEPFEN: "6k1/8/7r/K2pP3/8/8/8/8 w - - 0 2",
		epMove:        "e5d6",
		wantEPMoves:   []string{"a5a4", "a5b4", "a5b5", "e5d6", "e5e6"},
		wantNoEPMoves: []string{"a5a4", "a5b4", "a5b5", "e5e6"},
		wantEPPerft2:  91, wantNoEPPerft2: 75,
	},
	{
		name:     "legal_black_capturer",
		startFEN: "8/8/8/8/3p3k/R7/4P3/1K6 w - - 0 1", push: "e2e4",
		epFEN: "8/8/8/8/3pP2k/R7/8/1K6 b - e3 0 1", noEPFEN: "8/8/8/8/3pP2k/R7/8/1K6 b - - 0 1",
		epMove:        "d4e3",
		wantEPMoves:   []string{"d4d3", "d4e3", "h4g4", "h4g5", "h4h5"},
		wantNoEPMoves: []string{"d4d3", "h4g4", "h4g5", "h4h5"},
		wantEPPerft2:  91, wantNoEPPerft2: 75,
	},
}

func mustParseEnPassantPosition(t *testing.T, fen string) *Position {
	t.Helper()
	pos, err := ParseFEN(fen)
	if err != nil {
		t.Fatalf("ParseFEN(%q): %v", fen, err)
	}
	return pos
}

func mustPlayEnPassantMove(t *testing.T, pos *Position, text string, game bool) (Move, Square, PositionTag, uint8) {
	t.Helper()
	for _, move := range GenerateLegalMoves(pos) {
		if move.ToString() != text {
			continue
		}
		var ep Square
		var tag PositionTag
		var clock uint8
		var ok bool
		if game {
			ep, tag, clock, ok = pos.GameMakeMove(move)
		} else {
			ep, tag, clock, ok = pos.MakeMove(move)
		}
		if !ok {
			t.Fatalf("%s rejected in %s", text, GenerateFEN(pos))
		}
		return move, ep, tag, clock
	}
	t.Fatalf("%s is not legal in %s (legal %v)", text, GenerateFEN(pos), legalMoveStrings(pos))
	return EmptyMove, NoSquare, 0, 0
}

func legalMoveStrings(pos *Position) []string {
	moves := GenerateLegalMoves(pos)
	result := make([]string, 0, len(moves))
	for _, move := range moves {
		result = append(result, move.ToString())
	}
	sort.Strings(result)
	return result
}

func containsMoveString(moves []string, wanted string) bool {
	for _, move := range moves {
		if move == wanted {
			return true
		}
	}
	return false
}

func clonePositionCounts(counts map[uint64]int) map[uint64]int {
	result := make(map[uint64]int, len(counts))
	for key, count := range counts {
		result[key] = count
	}
	return result
}

func perft2Nodes(pos *Position) uint64 {
	return RunPerftTest(pos, 2).Nodes
}

type enPassantPositionSnapshot struct {
	board     Bitboard
	tag       PositionTag
	ep        Square
	hash      uint64
	clock     uint8
	positions map[uint64]int
}

func snapshotEnPassantPosition(pos *Position) enPassantPositionSnapshot {
	return enPassantPositionSnapshot{
		board: pos.Board, tag: pos.Tag, ep: pos.EnPassant, hash: pos.Hash(), clock: pos.HalfMoveClock,
		positions: clonePositionCounts(pos.Positions),
	}
}

func assertEnPassantPositionRestored(t *testing.T, pos *Position, want enPassantPositionSnapshot) {
	t.Helper()
	if pos.Board != want.board || pos.Tag != want.tag || pos.EnPassant != want.ep ||
		pos.Hash() != want.hash || pos.HalfMoveClock != want.clock || !reflect.DeepEqual(pos.Positions, want.positions) {
		t.Fatalf("position not restored:\n got fen=%s tag=%08b hash=%016X map=%v\nwant ep=%s tag=%08b hash=%016X map=%v",
			GenerateFEN(pos), pos.Tag, pos.Hash(), pos.Positions, want.ep, want.tag, want.hash, want.positions)
	}
}

func assertPinnedOracleStage(t *testing.T, label string, pos *Position, moves []string, perft2 uint64) {
	t.Helper()
	if got := legalMoveStrings(pos); !reflect.DeepEqual(got, moves) {
		t.Fatalf("%s legal moves = %v, want pinned Stockfish set %v", label, got, moves)
	}
	if got := perft2Nodes(pos); got != perft2 {
		t.Fatalf("%s perft 2 = %d, want pinned Stockfish %d", label, got, perft2)
	}
}

func assertIncrementalEnPassantKeyMatchesFEN(t *testing.T, label string, pos *Position) {
	t.Helper()
	rebuilt := mustParseEnPassantPosition(t, GenerateFEN(pos))
	if pos.Hash() != rebuilt.Hash() {
		t.Fatalf("%s incremental key %016X != FEN full key %016X (fen %s)",
			label, pos.Hash(), rebuilt.Hash(), GenerateFEN(pos))
	}
}

func TestLegalEnPassantOrdinaryHashAndPinnedOracle(t *testing.T) {
	for _, fixture := range enPassantRepetitionFixtures {
		t.Run(fixture.name, func(t *testing.T) {
			incremental := mustParseEnPassantPosition(t, fixture.startFEN)
			mustPlayEnPassantMove(t, incremental, fixture.push, true)
			fenEP := mustParseEnPassantPosition(t, fixture.epFEN)
			fenNoEP := mustParseEnPassantPosition(t, fixture.noEPFEN)

			assertPinnedOracleStage(t, "real push", incremental, fixture.wantEPMoves, fixture.wantEPPerft2)
			assertPinnedOracleStage(t, "EP FEN", fenEP, fixture.wantEPMoves, fixture.wantEPPerft2)
			assertPinnedOracleStage(t, "no-EP FEN", fenNoEP, fixture.wantNoEPMoves, fixture.wantNoEPPerft2)

			if incremental.EnPassant == NoSquare || fenEP.EnPassant == NoSquare {
				t.Fatal("raw adjacent EP state was cleared; move-generation state must be retained")
			}
			if incremental.Hash() != fenEP.Hash() {
				t.Fatalf("incremental key %016X != EP-FEN full key %016X", incremental.Hash(), fenEP.Hash())
			}
			if fixture.illegalEnPassant {
				if incremental.Hash() != fenNoEP.Hash() {
					t.Fatalf("illegal EP changed ordinary key: real/EP=%016X no-EP=%016X", incremental.Hash(), fenNoEP.Hash())
				}
				if containsMoveString(fixture.wantEPMoves, fixture.epMove) {
					t.Fatalf("fixture error: illegal EP %s appears in pinned legal set", fixture.epMove)
				}
			} else {
				if incremental.Hash() == fenNoEP.Hash() {
					t.Fatalf("legal EP failed to change ordinary key: %016X", incremental.Hash())
				}
				if !containsMoveString(fixture.wantEPMoves, fixture.epMove) || containsMoveString(fixture.wantNoEPMoves, fixture.epMove) {
					t.Fatalf("fixture error: legal EP distinction missing for %s", fixture.epMove)
				}
			}
		})
	}
}

func TestIllegalEnPassantPlayedThreefold(t *testing.T) {
	for _, fixture := range enPassantRepetitionFixtures {
		if !fixture.illegalEnPassant {
			continue
		}
		t.Run(fixture.name, func(t *testing.T) {
			pos := mustParseEnPassantPosition(t, fixture.startFEN)
			sequence := append([]string{fixture.push}, fixture.cycle...)
			sequence = append(sequence, fixture.cycle...)
			var firstHash uint64
			for ply, text := range sequence {
				mustPlayEnPassantMove(t, pos, text, true)
				assertIncrementalEnPassantKeyMatchesFEN(t, "played ply", pos)
				if ply == 0 {
					firstHash = pos.Hash()
				}
			}
			if pos.Hash() != firstHash {
				t.Fatalf("true repeated position split: first=%016X third=%016X", firstHash, pos.Hash())
			}
			if got := pos.Positions[pos.Hash()]; got != 3 {
				t.Fatalf("third-occurrence count = %d, want 3 (history=%v)", got, pos.Positions)
			}
			if !pos.IsFIDEDrawRule() {
				t.Fatal("true third occurrence was not recognized as a FIDE draw")
			}
		})
	}
}

func TestEnPassantKeyTransitionRestoration(t *testing.T) {
	for _, fixture := range enPassantRepetitionFixtures {
		t.Run(fixture.name, func(t *testing.T) {
			base := mustParseEnPassantPosition(t, fixture.startFEN)
			mustPlayEnPassantMove(t, base, fixture.push, true)

			transitionMoves := []string{fixture.epMove}
			if fixture.illegalEnPassant {
				transitionMoves = []string{fixture.cycle[0]}
			} else {
				// In addition to the EP capture, cover clearing a legal raw target
				// through an ordinary quiet move.
				for _, text := range fixture.wantEPMoves {
					if text != fixture.epMove {
						transitionMoves = append(transitionMoves, text)
						break
					}
				}
			}

			for _, text := range transitionMoves {
				t.Run(text, func(t *testing.T) {
					// Search make/unmake restores the ordinary key and all raw
					// state; the child incremental key equals a full rebuild.
					searchBefore := snapshotEnPassantPosition(base)
					move, ep, tag, clock := mustPlayEnPassantMove(t, base, text, false)
					assertIncrementalEnPassantKeyMatchesFEN(t, "search child", base)
					base.UnMakeMove(move, tag, ep, clock)
					assertEnPassantPositionRestored(t, base, searchBefore)

					// Game make/unmake additionally restores the occurrence map.
					gameBefore := snapshotEnPassantPosition(base)
					move, ep, tag, clock = mustPlayEnPassantMove(t, base, text, true)
					assertIncrementalEnPassantKeyMatchesFEN(t, "game child", base)
					base.GameUnMakeMove(move, tag, ep, clock)
					assertEnPassantPositionRestored(t, base, gameBefore)
				})
			}

			// Warm null make/unmake must preserve raw EP and its key eligibility.
			warmBefore := snapshotEnPassantPosition(base)
			nullEP := base.MakeNullMove()
			base.UnMakeNullMove(nullEP)
			assertEnPassantPositionRestored(t, base, warmBefore)

			// Force the null transition down the cold-hash path too. Hashing the
			// transient null state must not lose the reversible pre-null EP metadata.
			cold := mustParseEnPassantPosition(t, GenerateFEN(base))
			coldWant := snapshotEnPassantPosition(cold)
			cold.hash = 0
			nullEP = cold.MakeNullMove()
			nullRebuilt := mustParseEnPassantPosition(t, GenerateFEN(cold))
			if cold.Hash() != nullRebuilt.Hash() {
				t.Fatalf("cold null incremental key %016X != full key %016X", cold.Hash(), nullRebuilt.Hash())
			}
			cold.UnMakeNullMove(nullEP)
			assertEnPassantPositionRestored(t, cold, coldWant)
		})
	}
}

func TestLegalEnPassantPredicateEdgesAndPurity(t *testing.T) {
	tests := []struct {
		name      string
		startFEN  string
		push      string
		wantMoves []string
	}{
		{
			name: "two adjacent only unpinned capturer legal",
			// e5 is pinned to Ke4 by Re8; c5 can capture while e5 remains a blocker.
			startFEN: "k3r3/3p4/8/2P1P3/4K3/8/8/8 b - - 0 1", push: "d7d5",
			wantMoves: []string{"c5d6"},
		},
		{
			name: "EP captures checking pawn",
			// d7-d5 checks Ke4; e5xd6 e.p. removes that checking pawn.
			startFEN: "6k1/3p4/8/4P3/4K3/8/8/8 b - - 0 1", push: "d7d5",
			wantMoves: []string{"e5d6"},
		},
		{
			name: "diagonal exposure is illegal",
			// e5 currently blocks Bh8-Kd4; e5xd6 would expose the diagonal.
			startFEN: "k6b/3p4/8/4P3/3K4/8/8/8 b - - 0 1", push: "d7d5",
			wantMoves: nil,
		},
		{
			name: "blocked diagonal remains legal",
			// Nf6 remains between Bh8 and Kd4 after e5xd6.
			startFEN: "k6b/3p4/5N2/4P3/3K4/8/8/8 b - - 0 1", push: "d7d5",
			wantMoves: []string{"e5d6"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pos := mustParseEnPassantPosition(t, test.startFEN)
			mustPlayEnPassantMove(t, pos, test.push, false)
			before := snapshotEnPassantPosition(pos)

			gotLegal := hasLegalEnPassant(pos)
			assertEnPassantPositionRestored(t, pos, before)

			var gotMoves []string
			for _, move := range GenerateLegalMoves(pos) {
				if move.IsEnPassant() {
					gotMoves = append(gotMoves, move.ToString())
				}
			}
			sort.Strings(gotMoves)
			if !reflect.DeepEqual(gotMoves, test.wantMoves) {
				t.Fatalf("legal EP moves = %v, want %v", gotMoves, test.wantMoves)
			}
			if gotLegal != (len(test.wantMoves) != 0) {
				t.Fatalf("hasLegalEnPassant = %v, legal generator moves = %v", gotLegal, gotMoves)
			}
			assertEnPassantPositionRestored(t, pos, before)
		})
	}
}
