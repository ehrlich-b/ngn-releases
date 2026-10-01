package engine

import (
	"bytes"
	"encoding/binary"
	"math/bits"
	"math/rand"
	"os"
	"reflect"
	"testing"
)

func TestPrivateBoardCompatibilityDump(t *testing.T) {
	path := os.Getenv("NGN_PRIVATE_BOARD_DUMP")
	if path == "" {
		t.Skip("private board proof only")
	}
	var output bytes.Buffer
	put := func(value interface{}) {
		if err := binary.Write(&output, binary.LittleEndian, value); err != nil {
			t.Fatal(err)
		}
	}
	record := func(board *Bitboard) {
		var masks [13]uint64
		var white, black uint64
		var mg, eg, phase int
		all := board.AllPieces()
		for sq := Square(0); sq <= H8; sq++ {
			piece := board.PieceAt(sq)
			put(int8(piece))
			if piece != NoPiece {
				masks[piece] |= uint64(1) << uint(sq)
				if piece.Color() == White {
					white |= uint64(1) << uint(sq)
				} else {
					black |= uint64(1) << uint(sq)
				}
				mg += mgPST[piece][sq]
				eg += egPST[piece][sq]
				phase += piecePhaseInc[piece]
				if all[sq] != piece {
					t.Fatalf("map/mailbox disagree at %d", sq)
				}
			} else if _, exists := all[sq]; exists {
				t.Fatal("map includes empty square")
			}
		}
		if board.pieces != masks || board.whitePieces != white || board.blackPieces != black {
			t.Fatal("occupancy invariant")
		}
		if board.accMG != mg || board.accEG != eg || board.accPhase != phase {
			t.Fatal("accumulator invariant")
		}
		if len(all) != bits.OnesCount64(white|black) {
			t.Fatal("map population invariant")
		}
		put(board.pieces)
		put(board.GetWhitePieces())
		put(board.GetBlackPieces())
		put(int64(board.accMG))
		put(int64(board.accEG))
		put(int64(board.accPhase))
		put(board.Pawns())
		put(board.Knights())
		put(board.Bishops())
		put(board.Rooks())
		put(board.Queens())
		put(board.Kings())
		put(board.IsEndGame(White))
		put(board.IsEndGame(Black))
		put(board.IsEndGame(NoColor))
		drawn := board.Draw()
		put(uint16(len(drawn)))
		output.WriteString(drawn)
		copy := board.copy()
		if !reflect.DeepEqual(copy, *board) {
			t.Fatal("copy differs")
		}
		copy.recomputeAccumulator()
		if !reflect.DeepEqual(copy, *board) {
			t.Fatal("recompute differs")
		}
	}
	zero := Bitboard{}
	record(&zero)
	start := StartingBoard()
	record(&start)
	for i := 0; i < 64; i++ {
		put(SquareMask[i])
		put(bitScanForward(uint64(1) << uint(i)))
		put(bitScanReverse(uint64(1) << uint(i)))
	}
	put(bitScanForward(0))
	put(bitScanReverse(0))
	put(int64(PopCount(^uint64(0))))
	random := rand.New(rand.NewSource(0x428311))
	board := StartingBoard()
	for step := 0; step < 5000; step++ {
		if step%3 == 0 {
			sq := Square(random.Intn(64))
			old := board.PieceAt(sq)
			newPiece := Piece(random.Intn(13))
			board.UpdateSquare(sq, newPiece, old)
		} else {
			src := Square(random.Intn(64))
			dest := Square(random.Intn(64))
			if src == dest {
				continue
			}
			moving := board.PieceAt(src)
			if (moving == WhiteKing && src == E1 && (dest == C1 || dest == G1)) || (moving == BlackKing && src == E8 && (dest == C8 || dest == G8)) {
				continue
			}
			board.Move(src, dest, moving, board.PieceAt(dest))
		}
		record(&board)
	}
	for _, black := range []bool{false, true} {
		for _, kingSide := range []bool{false, true} {
			b := Bitboard{}
			src := E1
			dest := C1
			rook := A1
			kingPiece := WhiteKing
			rookPiece := WhiteRook
			if black {
				src = E8
				dest = C8
				rook = A8
				kingPiece = BlackKing
				rookPiece = BlackRook
			}
			if kingSide {
				if black {
					dest = G8
					rook = H8
				} else {
					dest = G1
					rook = H1
				}
			}
			b.UpdateSquare(src, kingPiece, NoPiece)
			b.UpdateSquare(rook, rookPiece, NoPiece)
			record(&b)
			b.Move(src, dest, kingPiece, NoPiece)
			record(&b)
		}
	}
	for mover := WhitePawn; mover <= BlackKing; mover++ {
		for captured := WhitePawn; captured <= BlackKing; captured++ {
			if mover.Color() == captured.Color() {
				continue
			}
			b := Bitboard{}
			b.UpdateSquare(B3, mover, NoPiece)
			b.UpdateSquare(C4, captured, NoPiece)
			b.Move(B3, C4, mover, NoPiece)
			if b.PieceAt(C4) != mover {
				t.Fatal("overlap mailbox lost mover")
			}
			b.Clear(C4, captured)
			record(&b)
		}
	}
	before := board
	board.Move(NoSquare, A1, WhitePawn, NoPiece)
	board.Move(A1, NoSquare, WhitePawn, NoPiece)
	if !reflect.DeepEqual(before, board) {
		t.Fatal("NoSquare move changed board")
	}
	if err := os.WriteFile(path, output.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestIndependentBoardMutationNoHeap(t *testing.T) {
	board := StartingBoard()
	if got := testing.AllocsPerRun(1000, func() {
		board.Move(E2, E4, WhitePawn, NoPiece)
		board.Move(E4, E2, WhitePawn, NoPiece)
		board.UpdateSquare(A2, WhiteQueen, WhitePawn)
		board.UpdateSquare(A2, WhitePawn, WhiteQueen)
		board.recomputeAccumulator()
	}); got != 0 {
		t.Fatalf("board mutation heap allocations=%v", got)
	}
	if !reflect.DeepEqual(board, StartingBoard()) {
		t.Fatal("allocation probe not restored")
	}
}
