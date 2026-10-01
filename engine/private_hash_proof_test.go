package engine

import (
	"bytes"
	"encoding/binary"
	"os"
	"reflect"
	"testing"
)

func TestPrivateHashCompatibilityDump(t *testing.T) {
	path := os.Getenv("NGN_PRIVATE_HASH_DUMP")
	if path == "" {
		t.Skip("private hash proof only")
	}
	var output bytes.Buffer
	put := func(value interface{}) {
		if err := binary.Write(&output, binary.LittleEndian, value); err != nil {
			t.Fatal(err)
		}
	}
	put(whiteTurnZC)
	for _, row := range piecesZC {
		for _, key := range row {
			put(key)
		}
	}
	for _, key := range castleRightsZC {
		put(key)
	}
	for _, key := range enPassantZC {
		put(key)
	}
	fens := []string{
		"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
		"r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1",
		"4k3/P7/8/8/8/8/7p/4K3 w - - 0 1",
		"4k3/8/8/3pP3/8/8/8/4K3 w - d6 0 1",
		"4r1k1/8/8/3pP3/8/8/8/4K3 w - d6 0 1",
		"8/8/8/r4pPK/8/8/8/4k3 w - f6 0 1",
		"r3k2r/8/8/8/8/8/8/R3K2R w KQkq - 0 1",
		"4k3/8/8/8/3Pp3/8/8/4K3 b - d3 0 1",
	}
	record := func(position *Position) {
		put(position.Hash())
		put(position.hash)
		put(generateZobristHash(position))
		put(PolyglotHash(position))
		put(uint8(position.Tag))
		put(int8(position.EnPassant))
		put(position.HalfMoveClock)
		put(hasLegalEnPassant(position))
		fen := GenerateFEN(position)
		put(uint16(len(fen)))
		output.WriteString(fen)
	}
	for _, fen := range fens {
		position, err := ParseFEN(fen)
		if err != nil {
			t.Fatal(err)
		}
		record(position)
		for _, move := range GenerateLegalMoves(position) {
			original := position.Copy()
			before := position.Hash()
			board := position.Board
			ep, tag, clock, legal := position.MakeMove(move)
			if !legal {
				t.Fatalf("generated illegal move %s", move.ToString())
			}
			put(uint32(move))
			record(position)
			if position.Hash() != generateZobristHash(position) {
				t.Fatalf("incremental key mismatch after %s", move.ToString())
			}
			position.UnMakeMove(move, tag, ep, clock)
			record(position)
			if position.Hash() != before || GenerateFEN(position) != GenerateFEN(original) || !reflect.DeepEqual(position.Board, board) {
				t.Fatal("ordinary move not restored")
			}
		}
		original := position.Copy()
		before := position.Hash()
		ep := position.MakeNullMove()
		record(position)
		if position.Hash() != generateZobristHash(position) {
			t.Fatal("null incremental key mismatch")
		}
		position.UnMakeNullMove(ep)
		record(position)
		if position.Hash() != before || GenerateFEN(position) != GenerateFEN(original) || !reflect.DeepEqual(position.Board, original.Board) {
			t.Fatal("null not restored")
		}
		var moves []Move
		var targets []Square
		var tags []PositionTag
		var clocks []uint8
		random := uint32(0x389a17e4)
		for ply := 0; ply < 160; ply++ {
			legalMoves := GenerateLegalMoves(position)
			if len(legalMoves) == 0 {
				break
			}
			random ^= random << 13
			random ^= random >> 17
			random ^= random << 5
			move := legalMoves[int(random)%len(legalMoves)]
			ep, tag, clock, legal := position.MakeMove(move)
			if !legal {
				t.Fatal("walk generated illegal move")
			}
			moves = append(moves, move)
			targets = append(targets, ep)
			tags = append(tags, tag)
			clocks = append(clocks, clock)
			put(uint32(move))
			record(position)
			if position.Hash() != generateZobristHash(position) {
				t.Fatalf("walk key mismatch at %d", ply)
			}
		}
		for i := len(moves) - 1; i >= 0; i-- {
			position.UnMakeMove(moves[i], tags[i], targets[i], clocks[i])
			record(position)
		}
		if position.Hash() != before || GenerateFEN(position) != GenerateFEN(original) || !reflect.DeepEqual(position.Board, original.Board) {
			t.Fatal("walk not restored")
		}
	}
	if err := os.WriteFile(path, output.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestIndependentHashLegalEPNoHeap(t *testing.T) {
	for _, fen := range []string{
		"4k3/8/8/3pP3/8/8/8/4K3 w - d6 0 1",
		"4k3/8/8/2PpP3/8/8/8/4K3 w - d6 0 1",
		"4r1k1/8/8/3pP3/8/8/8/4K3 w - d6 0 1",
		"8/8/8/r4pPK/8/8/8/4k3 w - f6 0 1",
		"4k3/8/8/8/3Pp3/8/8/4K3 b - d3 0 1",
	} {
		position, err := ParseFEN(fen)
		if err != nil {
			t.Fatal(err)
		}
		board := position.Board
		tag := position.Tag
		target := position.EnPassant
		hash := position.hash
		if got := testing.AllocsPerRun(1000, func() { hasLegalEnPassant(position) }); got != 0 {
			t.Fatalf("EP probe allocation = %v for %s", got, fen)
		}
		if position.hash != hash || position.Tag != tag || position.EnPassant != target || !reflect.DeepEqual(position.Board, board) {
			t.Fatal("EP probe mutated source")
		}
	}
}
