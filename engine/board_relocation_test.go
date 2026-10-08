package engine

import (
	"math/rand"
	"testing"
)

// Square primitives provide an independent check on combined relocation,
// including supplied pieces that disagree with the mailbox and invalid inputs.
func TestBoardRelocationMatchesSquarePrimitives(t *testing.T) {
	rng := rand.New(rand.NewSource(20261005))
	for i := 0; i < 20000; i++ {
		board := StartingBoard()
		for j := 0; j < 12; j++ {
			sq := Square(rng.Intn(64))
			board.UpdateSquare(sq, Piece(rng.Intn(13)), board.PieceAt(sq))
		}
		from, to := Square(rng.Intn(68)-2), Square(rng.Intn(68)-2)
		moving, captured := Piece(rng.Intn(16)-1), Piece(rng.Intn(16)-1)
		if i%2 == 0 {
			moving, captured = board.PieceAt(from), board.PieceAt(to)
		}
		want := board
		if validSquare(from) && validSquare(to) {
			want.Clear(to, captured)
			want.Clear(from, moving)
			if moving == NoPiece {
				want.mailbox[uint8(to)] = NoPiece
			} else {
				want.addPiece(to, moving)
			}
			// Only the four king displacements have a rook side effect.
			for _, castle := range []struct {
				piece          Piece
				from, to, r, d Square
				rook           Piece
			}{
				{WhiteKing, E1, G1, H1, F1, WhiteRook},
				{WhiteKing, E1, C1, A1, D1, WhiteRook},
				{BlackKing, E8, G8, H8, F8, BlackRook},
				{BlackKing, E8, C8, A8, D8, BlackRook},
			} {
				if moving == castle.piece && from == castle.from && to == castle.to {
					want.Clear(castle.r, castle.rook)
					want.addPiece(castle.d, castle.rook)
				}
			}
		}
		board.Move(from, to, moving, captured)
		if board != want {
			t.Fatalf("trial %d from=%d to=%d moving=%d captured=%d: relocation differs", i, from, to, moving, captured)
		}
	}
}
