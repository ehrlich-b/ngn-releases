package engine

import (
	"bytes"
	"encoding/binary"
	"os"
	"reflect"
	"sort"
	"testing"
)

func TestPrivatePositionCompatibilityDump(t *testing.T) {
	path := os.Getenv("NGN_PRIVATE_POSITION_DUMP")
	if path == "" {
		t.Skip("private position proof only")
	}
	var out bytes.Buffer
	put := func(value interface{}) {
		if err := binary.Write(&out, binary.LittleEndian, value); err != nil {
			t.Fatal(err)
		}
	}
	record := func(p *Position) {
		put(p.Hash())
		put(p.hash)
		put(generateZobristHash(p))
		put(uint8(p.Tag))
		put(int8(p.EnPassant))
		put(p.HalfMoveClock)
		put(p.IsDraw())
		put(p.IsFIDEDrawRule())
		put(p.IsEndGame())
		put(p.IsInCheck())
		put(p.Positions != nil)
		keys := make([]uint64, 0, len(p.Positions))
		for key := range p.Positions {
			keys = append(keys, key)
		}
		sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
		put(uint32(len(keys)))
		for _, key := range keys {
			put(key)
			put(int64(p.Positions[key]))
		}
		fen := GenerateFEN(p)
		put(uint16(len(fen)))
		out.WriteString(fen)
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
	for _, fen := range fens {
		p, err := ParseFEN(fen)
		if err != nil {
			t.Fatal(err)
		}
		record(p)
		for _, move := range GenerateLegalMoves(p) {
			original := p.Copy()
			before := p.Board
			tag := p.Tag
			ep := p.EnPassant
			clock := p.HalfMoveClock
			hash := p.hash
			full := p.Copy()
			_, _, _, legal := full.MakeMove(move)
			if !legal {
				t.Fatal("legal full move rejected")
			}
			p.partialMakeMove(move)
			if !reflect.DeepEqual(p.Board, full.Board) || p.Tag != tag^(BlackToMove|WhiteToMove) || p.EnPassant != ep || p.HalfMoveClock != clock || p.hash != hash {
				t.Fatal("partial move contract")
			}
			p.partialUnMakeMove(move)
			if !reflect.DeepEqual(p.Board, before) || p.Tag != tag || p.EnPassant != ep || p.HalfMoveClock != clock || p.hash != hash {
				t.Fatal("partial move restoration")
			}
			oldEP, oldTag, oldClock, legal := p.GameMakeMove(move)
			if !legal {
				t.Fatal("game move rejected")
			}
			put(uint32(move))
			record(p)
			p.GameUnMakeMove(move, oldTag, oldEP, oldClock)
			record(p)
			if GenerateFEN(p) != GenerateFEN(original) || p.Hash() != original.Hash() || !reflect.DeepEqual(p.Board, original.Board) || !reflect.DeepEqual(p.Positions, original.Positions) {
				t.Fatal("game move restoration")
			}
		}
		original := p.Copy()
		var moves []Move
		var targets []Square
		var tags []PositionTag
		var clocks []uint8
		random := uint32(0x3aef2271)
		for ply := 0; ply < 100; ply++ {
			legalMoves := GenerateLegalMoves(p)
			if len(legalMoves) == 0 {
				break
			}
			random ^= random << 13
			random ^= random >> 17
			random ^= random << 5
			move := legalMoves[int(random)%len(legalMoves)]
			ep, tag, clock, legal := p.GameMakeMove(move)
			if !legal {
				t.Fatal("game walk rejected")
			}
			moves = append(moves, move)
			targets = append(targets, ep)
			tags = append(tags, tag)
			clocks = append(clocks, clock)
			record(p)
		}
		for i := len(moves) - 1; i >= 0; i-- {
			p.GameUnMakeMove(moves[i], tags[i], targets[i], clocks[i])
			record(p)
		}
		if GenerateFEN(p) != GenerateFEN(original) || p.Hash() != original.Hash() || !reflect.DeepEqual(p.Board, original.Board) || !reflect.DeepEqual(p.Positions, original.Positions) {
			t.Fatal("game walk restoration")
		}
	}
	placements := [][]Piece{{}, {WhiteKnight}, {BlackKnight}, {WhiteBishop}, {BlackBishop}, {WhiteKnight, WhiteKnight}, {BlackBishop, BlackBishop}, {WhiteBishop, BlackBishop}, {WhiteBishop, BlackKnight}, {WhitePawn}, {BlackRook}, {WhiteQueen}}
	for _, pieces := range placements {
		for _, clock := range []uint8{0, 99, 100, 101, 255} {
			for count := 0; count <= 3; count++ {
				p := &Position{EnPassant: NoSquare, Tag: WhiteToMove, HalfMoveClock: clock}
				p.Board.UpdateSquare(E1, WhiteKing, NoPiece)
				p.Board.UpdateSquare(E8, BlackKing, NoPiece)
				for i, piece := range pieces {
					p.Board.UpdateSquare(Square(int(A2)+i*2), piece, NoPiece)
				}
				p.Positions = map[uint64]int{p.Hash(): count}
				record(p)
				copy := p.Copy()
				if !reflect.DeepEqual(copy.Board, p.Board) || !reflect.DeepEqual(copy.Positions, p.Positions) || copy.hash != p.hash {
					t.Fatal("copy contents")
				}
				copy.Positions[12345] = 77
				if _, exists := p.Positions[12345]; exists {
					t.Fatal("copy map aliased")
				}
			}
		}
	}
	for tag := 0; tag < 256; tag++ {
		p := &Position{Tag: PositionTag(tag)}
		put(int8(p.Turn()))
		put(p.HasCastling())
		put(p.IsInCheck())
		for _, mask := range []PositionTag{0, 1, 8, 15, 16, 32, 64, 128, 255} {
			put(p.HasTag(mask))
			original := p.Tag
			p.SetTag(mask)
			put(uint8(p.Tag))
			p.Tag = original
			p.ClearTag(mask)
			put(uint8(p.Tag))
			p.Tag = original
			p.ToggleTag(mask)
			put(uint8(p.Tag))
			p.Tag = original
		}
		p.ToggleTurn()
		put(uint8(p.Tag))
	}
	nilMap := (&Position{EnPassant: NoSquare}).Copy()
	put(nilMap.Positions != nil)
	if nilMap.Positions == nil {
		t.Fatal("copy nil-map contract")
	}
	if err := os.WriteFile(path, out.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestIndependentPositionMutationNoHeap(t *testing.T) {
	p, err := ParseFEN("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	p.Hash()
	before := GenerateFEN(p)
	board := p.Board
	var chosen Move
	for _, move := range GenerateLegalMoves(p) {
		if move.ToString() == "e2e4" {
			chosen = move
			break
		}
	}
	if chosen == EmptyMove {
		t.Fatal("probe move absent")
	}
	if got := testing.AllocsPerRun(1000, func() {
		ep, tag, clock, _ := p.MakeMove(chosen)
		p.UnMakeMove(chosen, tag, ep, clock)
		ep = p.MakeNullMove()
		p.UnMakeNullMove(ep)
	}); got != 0 {
		t.Fatalf("position mutation allocations=%v", got)
	}
	if GenerateFEN(p) != before || !reflect.DeepEqual(p.Board, board) {
		t.Fatal("allocation probe restoration")
	}
}
