package engine

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestBlackEnPassantAcrossMoveGenerators(t *testing.T) {
	pos, err := ParseFEN("4k3/8/8/8/2pP4/8/8/4K3 b - d3 0 1")
	if err != nil {
		t.Fatal(err)
	}
	want := "c4d3"
	var buf [256]Move
	n := GenerateMovesIntoBuffer(pos, buf[:])
	for _, tc := range []struct {
		name  string
		moves []Move
	}{
		{"buffer", buf[:n]},
		{"slice", GenerateMoves(pos)},
		{"legal", GenerateLegalMoves(pos)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, move := range tc.moves {
				if move.ToString() == want && move.IsEnPassant() && IsLegalMove(pos, move) {
					return
				}
			}
			t.Fatalf("missing legal black en-passant capture %s", want)
		})
	}
	if move, err := ParseUCIMove(pos, want); err != nil || !move.IsEnPassant() {
		t.Fatalf("UCI rejected black en-passant: move=%v err=%v", move, err)
	}
}

func TestUCIBlackEnPassantOnlyEvasion(t *testing.T) {
	// d2-d4 checked the king on e5. All its escape squares are covered;
	// e4xd3 en passant is the only reply. The old output validator discarded
	// that searched move and emitted (none) because its move list omitted it.
	const fen = "2B5/8/8/2K1k3/3Pp3/8/8/5R2 b - d3 0 1"
	uci := NewUCIEngine()
	var output bytes.Buffer
	uci.handleCommand("position fen "+fen, &output)
	uci.handleCommand("go depth 1", &output)
	defer uci.handleStop(&output)
	deadline := time.Now().Add(5 * time.Second)
	for uci.isSearching() {
		if time.Now().After(deadline) {
			t.Fatal("UCI search did not finish")
		}
		time.Sleep(time.Millisecond)
	}
	if !strings.Contains(output.String(), "bestmove e4d3\n") {
		t.Fatalf("UCI discarded the only legal check evasion:\n%s", output.String())
	}
}

func TestUCIReceivesBlackEnPassant(t *testing.T) {
	uci := NewUCIEngine()
	var output bytes.Buffer
	uci.handleCommand("position fen 4k3/8/8/8/2pP4/8/8/4K3 b - d3 0 1 moves c4d3", &output)
	if uci.position.Turn() != White || uci.position.Board.PieceAt(D3) != BlackPawn || uci.position.Board.PieceAt(D4) != NoPiece {
		t.Fatalf("UCI failed to apply the opponent's black en-passant capture: %s", GenerateFEN(uci.position))
	}
}
