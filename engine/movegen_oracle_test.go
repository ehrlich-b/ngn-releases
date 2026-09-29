package engine

import (
	"bufio"
	"context"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Opt-in independent move-generation audit:
// NGN_MOVEGEN_ORACLE=/path/to/stockfish go test ./engine -run TestMovegenOracle -v
// Each root move's depth-2 divide is compared with Stockfish. Board, hash and
// check-tag restoration are also checked while walking reproducible legal games.
func TestMovegenOracle(t *testing.T) {
	oracle := os.Getenv("NGN_MOVEGEN_ORACLE")
	if oracle == "" {
		t.Skip("set NGN_MOVEGEN_ORACLE to a Stockfish binary for the independent audit")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, oracle)
	in, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		in.Close()
		cancel()
		_ = cmd.Wait()
	}()
	scanner := bufio.NewScanner(out)
	query := func(fen string) map[string]int {
		t.Helper()
		if _, err := fmt.Fprintf(in, "position fen %s\ngo perft 2\n", fen); err != nil {
			t.Fatal(err)
		}
		divide := make(map[string]int)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "Nodes searched:") {
				return divide
			}
			fields := strings.Fields(line)
			if len(fields) == 2 && strings.HasSuffix(fields[0], ":") {
				move := strings.TrimSuffix(fields[0], ":")
				if n, err := strconv.Atoi(fields[1]); err == nil && (len(move) == 4 || len(move) == 5) {
					divide[move] = n
				}
			}
		}
		t.Fatalf("oracle stopped before perft result: %v; deadline: %v", scanner.Err(), ctx.Err())
		return nil
	}
	starts := []string{
		"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
		"r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1",
		"r3k2r/8/8/8/8/8/8/R3K2R w KQkq - 0 1",
		"4k3/8/8/3pP3/8/8/8/4K3 w - d6 0 1",
		"4k3/8/8/4KPpr/8/8/8/8 w - g6 0 1", // EP would expose the king to the rook
		"n1n5/PPPk4/8/8/8/8/4Kppp/5N1N w - - 0 1",
	}
	rng := rand.New(rand.NewSource(2600))
	positions, edges, searchRoots := 0, 0, 0
	for game := 0; game < 36; game++ {
		pos, err := ParseFEN(starts[game%len(starts)])
		if err != nil {
			t.Fatal(err)
		}
		for ply := 0; ply < 64; ply++ {
			fen := GenerateFEN(pos)
			want := query(fen)
			board, hash, tag, ep, hc := pos.Board, pos.Hash(), pos.Tag, pos.EnPassant, pos.HalfMoveClock
			// Perft validates the legal move generator, but the search uses raw
			// pseudo-legal generation for speed. Audit its unguarded BestMove too:
			// this caught castling-out-of-check being searched even though UCI's
			// final legality fallback declined to play it.
			if pos.HasCastling() && searchRoots < 128 {
				ClearStop()
				ClearHistoryTable()
				defaultSearchEngine, _ = NewSearchEngineWithHash(DEFAULT_CACHE_SIZE)
				info := SearchIterativeDeepening(pos, 2, nil)
				_, legal := want[info.BestMove.ToString()]
				if (len(want) == 0 && info.BestMove != EmptyMove) || (len(want) != 0 && !legal) {
					t.Fatalf("search chose illegal root %s in %s", info.BestMove.ToString(), fen)
				}
				if pos.Board != board || pos.Hash() != hash || pos.Tag != tag || pos.EnPassant != ep || pos.HalfMoveClock != hc {
					t.Fatalf("search changed state in %s", fen)
				}
				searchRoots++
			}
			wantPawnPush := false
			for uciMove := range want {
				from := Square(int(uciMove[1]-'1')*8 + int(uciMove[0]-'a'))
				if pos.Board.PieceAt(from).Type() == Pawn && uciMove[0] == uciMove[2] &&
					(int(uciMove[3])-int(uciMove[1]) == 1 || int(uciMove[1])-int(uciMove[3]) == 1) {
					wantPawnPush = true
				}
			}
			if got := hasLegalPawnPush(pos); got != wantPawnPush {
				t.Fatalf("pawn-push witness %v, oracle %v in %s", got, wantPawnPush, fen)
			}
			if pos.Board != board || pos.Hash() != hash || pos.Tag != tag || pos.EnPassant != ep || pos.HalfMoveClock != hc {
				t.Fatalf("pawn-push probe changed state in %s", fen)
			}
			moves := GenerateLegalMoves(pos)
			if len(moves) != len(want) {
				t.Fatalf("game %d ply %d: NGN has %d legal moves, oracle %d; FEN %s", game, ply, len(moves), len(want), fen)
			}
			// Search and UCI validation use separate generation paths. Compare
			// encoded moves too, so capture/promotion/EP flags cannot disagree.
			remaining := make(map[Move]bool, len(moves))
			for _, move := range moves {
				remaining[move] = true
			}
			var buffer [256]Move
			count := GenerateMovesIntoBuffer(pos, buffer[:])
			for _, move := range buffer[:count] {
				if !IsLegalMove(pos, move) {
					continue
				}
				if !remaining[move] {
					t.Fatalf("buffer generated extra/differently encoded move %s in %s", move.ToString(), fen)
				}
				delete(remaining, move)
			}
			if len(remaining) != 0 {
				t.Fatalf("buffer omitted legal moves %v in %s", remaining, fen)
			}
			for _, move := range moves {
				oldEP, oldTag, oldHC, _ := pos.MakeMove(move)
				rebuilt, err := ParseFEN(GenerateFEN(pos))
				if err != nil || rebuilt.Hash() != pos.Hash() || rebuilt.Board != pos.Board || rebuilt.Tag != pos.Tag {
					t.Fatalf("incremental state differs from FEN rebuild after %s in %s: %v", move.ToString(), fen, err)
				}
				n := len(GenerateLegalMoves(pos))
				if expected, exists := want[move.ToString()]; !exists || n != expected {
					t.Fatalf("%s in %s: NGN divide %d, oracle %d (present=%v)", move.ToString(), fen, n, expected, exists)
				}
				pos.UnMakeMove(move, oldTag, oldEP, oldHC)
				if pos.Board != board || pos.Hash() != hash || pos.Tag != tag || pos.EnPassant != ep || pos.HalfMoveClock != hc {
					t.Fatalf("make/unmake changed the position: %s in %s", move.ToString(), fen)
				}
				edges++
			}
			positions++
			if len(moves) == 0 {
				break
			}
			pos.GameMakeMove(moves[rng.Intn(len(moves))])
		}
	}
	t.Logf("seed 2600: %d positions, %d root moves, %d raw search roots; every depth-2 divide and state restoration matched", positions, edges, searchRoots)
}
