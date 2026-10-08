package ngnp

import (
	"bytes"
	"encoding/hex"
	"io"
	"math/rand"
	"strings"
	"testing"

	"github.com/ehrlich-b/ngn/engine"
)

const startFEN = "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"

func position(t *testing.T, fen string) *engine.Position {
	t.Helper()
	pos, err := engine.ParseFEN(fen)
	if err != nil {
		t.Fatal(err)
	}
	return pos
}

func TestByteLayout(t *testing.T) {
	r, err := FromPosition(position(t, startFEN), -1234, Win, 0x1234, 0)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := Encode(r)
	if err != nil {
		t.Fatal(err)
	}
	// Ascending squares, low nibble first, then signed score/result/stm/ply/flags.
	want := "ffff00000000ffff13422531000000006666666679a88b972efb020034120000"
	if got := hex.EncodeToString(raw[:]); got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestRandomLegalRoundTrip(t *testing.T) {
	rng := rand.New(rand.NewSource(15971))
	positions := 0
	for game := 0; game < 40; game++ {
		pos := position(t, startFEN)
		for ply := 0; ply < 160; ply++ {
			r, err := FromPosition(pos, rng.Intn(50001)-25000, Result(rng.Intn(3)), uint16(ply), 0)
			if err != nil {
				t.Fatalf("%s: %v", engine.GenerateFEN(pos), err)
			}
			raw, err := Encode(r)
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := Decode(raw[:])
			if err != nil {
				t.Fatal(err)
			}
			if decoded != r {
				t.Fatalf("record changed: %+v != %+v", decoded, r)
			}
			board, err := decoded.Position()
			if err != nil {
				t.Fatal(err)
			}
			want := strings.Fields(engine.GenerateFEN(pos))[:2]
			got := strings.Fields(engine.GenerateFEN(board))[:2]
			if strings.Join(got, " ") != strings.Join(want, " ") {
				t.Fatalf("board/stm FEN: got %v, want %v", got, want)
			}
			positions++
			moves := engine.GenerateLegalMoves(pos)
			if len(moves) == 0 || pos.IsFIDEDrawRule() {
				break
			}
			pos.GameMakeMove(moves[rng.Intn(len(moves))])
		}
	}
	if positions < 3000 {
		t.Fatalf("only %d positions exercised", positions)
	}
}

func TestValidationRejectsCorruption(t *testing.T) {
	r, err := FromPosition(position(t, startFEN), 100, Draw, 8, 0)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := Encode(r)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name  string
		index int
		value byte
	}{
		{"reserved byte", 31, 1}, {"reserved flags", 30, 16}, {"result", 26, 3}, {"stm", 27, 2},
		{"invalid piece", 8, 15}, {"pawn on first rank", 8, 0x10},
		{"missing king", 10, 0x24}, {"extra pawn", 16, 0x60},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bad := raw
			bad[tc.index] = tc.value
			if _, err := Decode(bad[:]); err == nil {
				t.Fatal("accepted corrupt record")
			}
		})
	}
	for length := 0; length < Size; length++ {
		if _, err := Decode(raw[:length]); err == nil {
			t.Fatalf("accepted %d bytes", length)
		}
	}
	if _, err := FromPosition(position(t, "4k3/4R3/8/8/8/8/8/4K3 w - - 0 1"), 0, Draw, 0, 0); err == nil {
		t.Fatal("accepted side not to move in check")
	}
	if _, err := FromPosition(position(t, "4k3/8/8/8/8/8/8/QQ2K3 w - - 0 1"), 0, Draw, 0, 0); err != nil {
		t.Fatalf("legal promoted material rejected: %v", err)
	}
	if _, err := FromPosition(position(t, "4k3/8/8/8/8/8/PPPPPPPP/QQ2K3 w - - 0 1"), 0, Draw, 0, 0); err == nil {
		t.Fatal("accepted two queens without a missing pawn")
	}
	kings, err := FromPosition(position(t, "4k3/8/8/8/8/8/8/4K3 w - - 0 1"), 0, Draw, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	kings.Pieces[1] = 1
	if err := kings.Validate(); err == nil {
		t.Fatal("accepted nonzero unused nibble")
	}
}

func TestFlagsAndClamping(t *testing.T) {
	pos := position(t, "4k3/8/8/8/8/8/4r3/4K3 w - - 0 1")
	r, err := FromPosition(pos, 50000, Loss, 0, FlagCapture|FlagPromotion)
	if err != nil {
		t.Fatal(err)
	}
	if r.Score != 32767 || r.Flags != 15 {
		t.Fatalf("flags/score: %+v", r)
	}
	r.Flags &^= FlagInCheck
	if err := r.Validate(); err == nil {
		t.Fatal("accepted wrong in-check flag")
	}
	r, err = FromPosition(pos, -50000, Loss, 0, 0)
	if err != nil || r.Score != -32768 {
		t.Fatalf("negative clamp: %+v %v", r, err)
	}
}

func TestIterateRejectsPartialTail(t *testing.T) {
	r, err := FromPosition(position(t, startFEN), 0, Draw, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := Encode(r)
	for tail := 0; tail < Size; tail++ {
		data := append(append([]byte{}, raw[:]...), raw[:tail]...)
		count := 0
		err := Iterate(bytes.NewReader(data), func(Record) error { count++; return nil })
		if count != 1 {
			t.Fatalf("tail %d: count=%d", tail, count)
		}
		if tail == 0 && err != nil {
			t.Fatal(err)
		}
		if tail != 0 && (err == nil || !strings.Contains(err.Error(), io.ErrUnexpectedEOF.Error()) || !strings.Contains(err.Error(), "byte 32")) {
			t.Fatalf("tail %d: %v", tail, err)
		}
	}
}
