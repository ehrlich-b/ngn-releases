package engine

import "testing"

func castleMove(pos *Position, uci string) (Move, bool) {
	var moves [256]Move
	n := GenerateMovesIntoBuffer(pos, moves[:])
	for _, move := range moves[:n] {
		if move.ToString() == uci {
			return move, true
		}
	}
	return EmptyMove, false
}

func TestCastlePathLegality(t *testing.T) {
	cases := []struct {
		name, fen, castle string
	}{
		{"white checked king wing", "4r2k/8/8/8/8/8/8/R3K2R w KQ - 0 1", "e1g1"},
		{"white checked queen wing", "4r2k/8/8/8/8/8/8/R3K2R w KQ - 0 1", "e1c1"},
		{"black checked king wing", "r3k2r/8/8/8/8/8/8/4R2K b kq - 0 1", "e8g8"},
		{"black checked queen wing", "r3k2r/8/8/8/8/8/8/4R2K b kq - 0 1", "e8c8"},
		{"white king transit attacked", "4k3/8/8/8/2b5/8/8/R3K2R w KQ - 0 1", "e1g1"},
		{"white queen transit attacked", "4k3/8/8/8/8/1b6/8/R3K2R w KQ - 0 1", "e1c1"},
		{"black king transit attacked", "r3k2r/8/8/2B5/8/8/8/4K3 b kq - 0 1", "e8g8"},
		{"black queen transit attacked", "r3k2r/8/1B6/8/8/8/8/4K3 b kq - 0 1", "e8c8"},
		{"white king destination attacked", "4k1r1/8/8/8/8/8/8/R3K2R w KQ - 0 1", "e1g1"},
		{"white queen destination attacked", "2r1k3/8/8/8/8/8/8/R3K2R w KQ - 0 1", "e1c1"},
		{"black king destination attacked", "r3k2r/8/8/8/8/8/8/4K1R1 b kq - 0 1", "e8g8"},
		{"black queen destination attacked", "r3k2r/8/8/8/8/8/8/2R1K3 b kq - 0 1", "e8c8"},
	}
	for _, fen := range []string{
		"r3k2r/8/8/8/8/8/8/R3K2R w KQkq - 0 1",
		"r3k2r/8/8/8/8/8/8/R3K2R b KQkq - 0 1",
	} {
		pos, err := ParseFEN(fen)
		if err != nil {
			t.Fatal(err)
		}
		castles := 0
		for _, move := range GenerateLegalMoves(pos) {
			if move.IsCastle() {
				castles++
			}
		}
		if castles != 2 {
			t.Fatalf("rejected safe castling in %s: got %d legal castles", fen, castles)
		}
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pos, err := ParseFEN(tc.fen)
			if err != nil {
				t.Fatal(err)
			}
			move, found := castleMove(pos, tc.castle)
			if !found {
				t.Fatalf("expected pseudo-legal castle %s", tc.castle)
			}
			if isLegalCastle(pos, move) || IsLegalMove(pos, move) {
				t.Fatalf("accepted illegal castle %s in %s", tc.castle, tc.fen)
			}
			for _, legal := range GenerateLegalMoves(pos) {
				if legal == move {
					t.Fatalf("GenerateLegalMoves retained illegal castle %s", tc.castle)
				}
			}
		})
	}
}

func TestCastleFromCheckNeverEntersSearch(t *testing.T) {
	const fen = "r1bqk2r/pppnnpp1/3p3p/4p3/3P1PPb/1P2P2P/P1P1N1B1/RNBQK2R w KQkq - 3 9"
	const illegalCastle = "e1g1"
	history := []string{
		"g2g4", "d7d6", "f2f4", "b8d7", "b2b3", "h7h6", "f1g2", "e7e5",
		"e2e3", "f8e7", "h2h3", "e7f6", "d2d4", "g8e7", "g1e2", "f6h4",
	}
	pos, err := ParseFEN("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	for _, uci := range history {
		move, err := ParseUCIMove(pos, uci)
		if err != nil {
			t.Fatalf("%s: %v", uci, err)
		}
		if !IsLegalMove(pos, move) {
			t.Fatalf("history contains illegal move %s", uci)
		}
		pos.GameMakeMove(move)
	}
	target, err := ParseFEN(fen)
	if err != nil {
		t.Fatal(err)
	}
	if !PositionEqual(target, pos) {
		t.Fatalf("history did not recreate target board: got %s", GenerateFEN(pos))
	}
	if !pos.IsInCheck() || !isInCheck(pos, White) {
		t.Fatal("reproduced position must check White")
	}
	if move, found := castleMove(pos, illegalCastle); !found || IsLegalMove(pos, move) {
		t.Fatal("reproduced position must pseudo-generate but reject e1g1")
	}

	before := pos.Copy()
	beforeHistory := make(map[uint64]int, len(pos.Positions))
	for key, count := range pos.Positions {
		beforeHistory[key] = count
	}
	for run := 0; run < 2; run++ {
		ClearStop()
		ClearHistoryTable()
		defaultSearchEngine, _ = NewSearchEngineWithHash(DEFAULT_CACHE_SIZE)
		info := SearchIterativeDeepening(pos, 3, nil)
		if info.BestMove == EmptyMove || info.BestMove.ToString() == illegalCastle || !IsLegalMove(pos, info.BestMove) {
			t.Fatalf("run %d chose illegal root move %s", run, info.BestMove.ToString())
		}
		if !PositionEqual(before, pos) {
			t.Fatalf("run %d changed board/hash/tag during search", run)
		}
		if len(pos.Positions) != len(beforeHistory) {
			t.Fatalf("run %d changed played-history keys", run)
		}
		for key, want := range beforeHistory {
			if got := pos.Positions[key]; got != want {
				t.Fatalf("run %d changed played-history count for %x: got %d want %d", run, key, got, want)
			}
		}
	}
}

func TestQuiescenceDoesNotInventCastleEvasion(t *testing.T) {
	// Stockfish perft 1 reports no legal moves for each: a castle would finish
	// on a safe square, but starts in double check and crosses an attacked square.
	// This reaches qsearch's in-check pseudo-legal loop directly.
	for _, fen := range []string{
		"k2r1r2/8/8/8/7b/8/3q4/4K2R w K - 0 1",
		"4k2r/3Q4/8/7B/8/8/8/K2R1R2 b k - 0 1",
	} {
		pos, err := ParseFEN(fen)
		if err != nil {
			t.Fatal(err)
		}
		move, found := castleMove(pos, map[Color]string{White: "e1g1", Black: "e8g8"}[pos.Turn()])
		if !found || IsLegalMove(pos, move) {
			t.Fatalf("fixture must pseudo-generate only an illegal castle: %s", fen)
		}
		before := pos.Copy()
		if pos.HasLegalMove() {
			t.Fatalf("HasLegalMove invented a castle evasion in %s", fen)
		}
		if got, resolved := texelQuietSearch(pos, -INFINITY, INFINITY, 0); !resolved || got != -MATE_VALUE {
			t.Fatalf("offline quiet search(%s)=(%d,%v), want resolved mate", fen, got, resolved)
		}
		ClearStop()
		defaultSearchEngine, _ = NewSearchEngineWithHash(DEFAULT_CACHE_SIZE)
		if got := quiescence(pos, -INFINITY, INFINITY, 0, newQSearchInfo()); got != -MATE_VALUE {
			t.Fatalf("qsearch(%s)=%d, want checkmate %d", fen, got, -MATE_VALUE)
		}
		if !PositionEqual(before, pos) {
			t.Fatalf("qsearch changed position for %s", fen)
		}
		for _, search := range []struct {
			name string
			run  func(*Position) *SearchInfo
		}{
			{"iterative", func(p *Position) *SearchInfo { return SearchIterativeDeepening(p, 2, nil) }},
			{"fixed", func(p *Position) *SearchInfo { return SearchFixed(p, 2, nil) }},
		} {
			ClearStop()
			defaultSearchEngine, _ = NewSearchEngineWithHash(DEFAULT_CACHE_SIZE)
			info := search.run(pos)
			if info.BestMove != EmptyMove || info.BestScore != -MATE_VALUE {
				t.Fatalf("%s root(%s): move=%s score=%d, want mate", search.name, fen, info.BestMove.ToString(), info.BestScore)
			}
			if !PositionEqual(before, pos) {
				t.Fatalf("%s root changed position for %s", search.name, fen)
			}
		}
	}
}

func TestRootAllPseudoMovesIllegalIsStalemate(t *testing.T) {
	for _, fen := range []string{
		"7k/5K2/6Q1/8/8/8/8/8 b - - 0 1",
		"8/8/8/8/8/6q1/5k2/7K w - - 0 1",
	} {
		pos, err := ParseFEN(fen)
		if err != nil {
			t.Fatal(err)
		}
		var moves [256]Move
		if GenerateMovesIntoBuffer(pos, moves[:]) == 0 || len(GenerateLegalMoves(pos)) != 0 || pos.IsInCheck() {
			t.Fatalf("fixture must have pseudo moves but no legal moves and no check: %s", fen)
		}
		for _, search := range []func(*Position, int, *TimeManager) *SearchInfo{SearchIterativeDeepening, SearchFixed} {
			defaultSearchEngine, _ = NewSearchEngineWithHash(DEFAULT_CACHE_SIZE)
			info := search(pos, 2, nil)
			if info.BestMove != EmptyMove || info.BestScore != 0 || info.Stopped {
				t.Fatalf("root(%s): move=%s score=%d stopped=%v, want stalemate", fen, info.BestMove.ToString(), info.BestScore, info.Stopped)
			}
		}
	}
}
