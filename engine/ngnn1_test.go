package engine

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// All random weights and reference arithmetic here are NGN-produced.
func ngnn1TestBytes(h int, seed int64, extreme bool) []byte {
	rng := rand.New(rand.NewSource(seed))
	var b bytes.Buffer
	b.WriteString("NGNN")
	for _, v := range []uint32{1, uint32(h), 255, 64, 400} {
		_ = binary.Write(&b, binary.LittleEndian, v)
	}
	for i := 0; i < (768+3)*h; i++ {
		v := int16(rng.Intn(65) - 32)
		if i >= 768*h && i < 769*h {
			v = int16(rng.Intn(256))
		}
		if extreme {
			v = int16(rng.Intn(65536) - 32768)
		}
		_ = binary.Write(&b, binary.LittleEndian, v)
	}
	ob := int32(-1234)
	if extreme {
		ob = -2147483648
	}
	_ = binary.Write(&b, binary.LittleEndian, ob)
	_ = binary.Write(&b, binary.LittleEndian, crc32.ChecksumIEEE(b.Bytes()))
	return b.Bytes()
}

func ngnn1TestNetwork(t *testing.T, h int, extreme bool) *NGNN1Network {
	t.Helper()
	n, err := ReadNGNN1(bytes.NewReader(ngnn1TestBytes(h, 19, extreme)))
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// Deliberately recomputes each neuron from the mailbox with int64 arithmetic,
// without using production features, refresh, activation, or output kernels.
func ngnn1Oracle(n *NGNN1Network, pos *Position) int {
	var sum int64
	for half := 0; half < 2; half++ {
		color := pos.Turn()
		if half == 1 {
			color = color.Other()
		}
		for i := 0; i < n.hidden; i++ {
			x := int64(n.bias[i])
			for square, piece := range pos.Board.mailbox {
				if piece < WhitePawn || piece > BlackKing {
					continue
				}
				typeIndex, pieceColor := int(piece)-1, White
				if typeIndex >= 6 {
					typeIndex -= 6
					pieceColor = Black
				}
				oriented := square
				if color == Black {
					oriented = (7-square/8)*8 + square%8
				}
				feature := typeIndex*64 + oriented
				if pieceColor != color {
					feature += 384
				}
				x += int64(n.weights[feature*n.hidden+i])
			}
			if x < 0 {
				x = 0
			}
			if x > 255 {
				x = 255
			}
			sum += x * x * int64(n.output[half*n.hidden+i])
		}
	}
	cp := (sum/255 + int64(n.outputBias)) * 400 / (255 * 64)
	if cp > 24999 {
		cp = 24999
	}
	if cp < -24999 {
		cp = -24999
	}
	return int(cp)
}

func TestNGNN1Validation(t *testing.T) {
	good := ngnn1TestBytes(32, 7, false)
	for _, field := range []struct {
		name   string
		offset int
		value  uint32
	}{
		{"magic", 0, 0}, {"version", 4, 2}, {"zero", 8, 0},
		{"unaligned", 8, 17}, {"oversize", 8, 2064}, {"huge", 8, 0xffffffff},
		{"qa", 12, 256}, {"qb", 16, 63}, {"scale", 20, 401},
	} {
		t.Run(field.name, func(t *testing.T) {
			bad := append([]byte(nil), good...)
			binary.LittleEndian.PutUint32(bad[field.offset:], field.value)
			binary.LittleEndian.PutUint32(bad[len(bad)-4:], crc32.ChecksumIEEE(bad[:len(bad)-4]))
			if n, err := ReadNGNN1(bytes.NewReader(bad)); err == nil || n != nil {
				t.Fatal("accepted invalid header")
			}
		})
	}
	for _, cut := range []int{0, 3, 23, 24, 100, len(good) - 1, len(good) - 4} {
		if n, err := ReadNGNN1(bytes.NewReader(good[:cut])); err == nil || n != nil {
			t.Fatalf("accepted length %d", cut)
		}
	}
	badCRC := append([]byte(nil), good...)
	badCRC[99] ^= 1
	for _, bad := range [][]byte{badCRC, append(append([]byte(nil), good...), 0)} {
		if n, err := ReadNGNN1(bytes.NewReader(bad)); err == nil || n != nil {
			t.Fatal("accepted corruption/trailing data")
		}
	}
	if n, err := LoadNGNN1(t.TempDir() + "/missing"); err == nil || n != nil {
		t.Fatal("accepted missing file")
	}
	for _, h := range []int{16, 32, 48, 256, 2048} {
		if _, err := ReadNGNN1(bytes.NewReader(ngnn1TestBytes(h, 3, false))); err != nil {
			t.Fatalf("H=%d: %v", h, err)
		}
	}
}

func TestNGNN1ArithmeticLimits(t *testing.T) {
	n := ngnn1TestNetwork(t, 16, false)
	clear(n.output)
	acc := make([]int32, 32)
	// The first division truncates before OB is added; negatives must use
	// Go's truncation toward zero at both divisions, rather than flooring.
	n.output[0] = -65
	acc[0] = 19
	n.outputBias = 16
	want := ((int64(19*19*-65) / 255) + 16) * 400 / (255 * 64)
	if got := n.evaluate(acc, White); int64(got) != want || got != -1 {
		t.Fatalf("negative truncation: %d, want %d", got, want)
	}
	for _, sign := range []int32{-1, 1} {
		n.outputBias = sign * 2147483647
		if got := n.evaluate(acc, White); got != int(sign)*24999 {
			t.Fatalf("clamp: %d", got)
		}
	}
	n.outputBias = 0
	acc[0] = 2147483647
	if got := ngnn1Dot(acc[:16], n.output[:16]); got != int64(255*255*-65) {
		t.Fatalf("activation overflow: %d", got)
	}
	acc[0] = -2147483648
	if got := ngnn1Dot(acc[:16], n.output[:16]); got != 0 {
		t.Fatalf("negative activation: %d", got)
	}
	s, err := NewSearchEngineWithHash(1)
	if err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []*NGNN1Network{nil, {}} {
		if err := s.SelectNGNN1Evaluator(invalid); err == nil {
			t.Fatal("selected invalid network")
		}
	}
}

func ngnn1AssertRefresh(t *testing.T, e *workerEvaluator, pos *Position) {
	t.Helper()
	n := e.nnue.network
	want := make([]int32, 2*n.hidden)
	n.refresh(pos, want)
	if !reflect.DeepEqual(want, e.nnue.at(e.depth)) {
		t.Fatalf("accumulator mismatch at ply %d", e.depth)
	}
	if e.SearchSTM(pos) != n.evaluate(want, pos.Turn()) {
		t.Fatalf("incremental score mismatch at ply %d", e.depth)
	}
}

func ngnn1NewWorker(t *testing.T, n *NGNN1Network, pos *Position) *workerEvaluator {
	t.Helper()
	m := evaluatorModel{identity: evaluatorModelIdentity{backend: evaluatorBackendNGNN1, adapterRevision: evaluatorAdapterRevision, network: n}}
	e, err := m.newWorker(&hceEvaluator{})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Reset(pos); err != nil {
		t.Fatal(err)
	}
	return e
}

func TestNGNN1IncrementalRandomGames(t *testing.T) {
	n := ngnn1TestNetwork(t, 32, true)
	rng := rand.New(rand.NewSource(811))
	plies := 0
	for game := 0; game < 32; game++ {
		pos := newUCIStartingPosition()
		root := pos.Board.mailbox
		e := ngnn1NewWorker(t, n, pos)
		type undo struct {
			move  Move
			ep    Square
			tag   PositionTag
			clock uint8
		}
		var undos []undo
		for ply := 0; ply < 200; ply++ {
			moves := GenerateLegalMoves(pos)
			if len(moves) == 0 {
				break
			}
			if ply%7 == 0 && !pos.IsInCheck() {
				tr := e.mustPrepareNull(pos)
				ep := pos.MakeNullMove()
				e.mustPushMadeNull(pos, tr, ep)
				ngnn1AssertRefresh(t, e, pos)
				e.mustPop()
				pos.UnMakeNullMove(ep)
				ngnn1AssertRefresh(t, e, pos)
			}
			move := moves[rng.Intn(len(moves))]
			tr := e.mustPrepareMove(pos, move)
			ep, tag, clock, _ := pos.MakeMove(move)
			e.mustPushMadeMove(pos, tr, move, tag, ep, clock)
			undos = append(undos, undo{move, ep, tag, clock})
			plies++
			ngnn1AssertRefresh(t, e, pos)
		}
		for i := len(undos) - 1; i >= 0; i-- {
			u := undos[i]
			e.mustPop()
			pos.UnMakeMove(u.move, u.tag, u.ep, u.clock)
			ngnn1AssertRefresh(t, e, pos)
		}
		if e.depth != 0 || pos.Board.mailbox != root {
			t.Fatal("did not restore root")
		}
	}
	if plies < 4000 {
		t.Fatalf("only %d legal plies", plies)
	}
	t.Logf("%d random legal plies and inverse transitions, with null moves", plies)
}

func TestNGNN1SpecialMoves(t *testing.T) {
	for _, h := range []int{16, 256, 2048} {
		n := ngnn1TestNetwork(t, h, true)
		for _, fen := range []string{
			"r3k2r/8/8/8/8/8/8/R3K2R w KQkq - 0 1",
			"r3k2r/8/8/8/8/8/8/R3K2R b KQkq - 0 1",
			"4k3/8/8/3pP3/8/8/8/4K3 w - d6 0 1",
			"4k3/8/8/8/3Pp3/8/8/4K3 b - d3 0 1",
			"1r2k3/P7/8/8/8/8/8/4K3 w - - 0 1",
			"4k3/8/8/8/8/8/p7/1R2K3 b - - 0 1",
		} {
			pos, err := ParseFEN(fen)
			if err != nil {
				t.Fatal(err)
			}
			e := ngnn1NewWorker(t, n, pos)
			castles, epMoves, promotions := 0, 0, 0
			for _, move := range GenerateLegalMoves(pos) {
				tr := e.mustPrepareMove(pos, move)
				ep, tag, clock, _ := pos.MakeMove(move)
				e.mustPushMadeMove(pos, tr, move, tag, ep, clock)
				ngnn1AssertRefresh(t, e, pos)
				if e.SearchSTM(pos) != ngnn1Oracle(n, pos) {
					t.Fatal("special move oracle mismatch")
				}
				e.mustPop()
				pos.UnMakeMove(move, tag, ep, clock)
				ngnn1AssertRefresh(t, e, pos)
				if move.IsCastle() {
					castles++
				}
				if move.IsEnPassant() {
					epMoves++
				}
				if move.PromoType() != NoType {
					promotions++
				}
			}
			if strings.Contains(fen, "KQkq") && castles != 2 || strings.Contains(fen, "d6") && epMoves != 1 || strings.Contains(fen, "d3") && epMoves != 1 || strings.Contains(fen, "P7") && promotions != 8 || strings.Contains(fen, "p7") && promotions != 8 {
				t.Fatalf("missing special coverage in %s: castle=%d ep=%d promo=%d", fen, castles, epMoves, promotions)
			}
		}
	}
}

func TestNGNN1OracleAndMirror(t *testing.T) {
	for _, h := range []int{32, 256, 2048} {
		for _, extreme := range []bool{false, true} {
			n := ngnn1TestNetwork(t, h, extreme)
			pos := newUCIStartingPosition()
			rng := rand.New(rand.NewSource(404))
			e := ngnn1NewWorker(t, n, pos)
			other := ngnn1NewWorker(t, n, pos)
			for sample := 0; sample < 64; sample++ {
				if err := e.Reset(pos); err != nil {
					t.Fatal(err)
				}
				if got, want := e.SearchSTM(pos), ngnn1Oracle(n, pos); got != want {
					t.Fatalf("H=%d sample=%d got=%d want=%d", h, sample, got, want)
				}
				mirrored := &Position{EnPassant: NoSquare}
				if pos.Turn() == White {
					mirrored.Tag = BlackToMove
				} else {
					mirrored.Tag = WhiteToMove
				}
				for square, piece := range pos.Board.mailbox {
					if validPiece(piece) {
						mirrored.Board.UpdateSquare(Square(square^56), GetPiece(piece.Type(), piece.Color().Other()), NoPiece)
					}
				}
				if err := other.Reset(mirrored); err != nil {
					t.Fatal(err)
				}
				if e.SearchSTM(pos) != other.SearchSTM(mirrored) {
					t.Fatal("color-mirror STM score mismatch")
				}
				a, b := e.nnue.at(0), other.nnue.at(0)
				if !reflect.DeepEqual(a[:h], b[h:]) || !reflect.DeepEqual(a[h:], b[:h]) {
					t.Fatal("mirror did not exchange perspectives")
				}
				moves := GenerateLegalMoves(pos)
				if len(moves) == 0 {
					pos = newUCIStartingPosition()
				} else {
					pos.MakeMove(moves[rng.Intn(len(moves))])
				}
			}
		}
	}
}

func TestNGNN1TrainerParity(t *testing.T) {
	root := filepath.Join("..", "testdata", "ngnn1")
	data, err := os.ReadFile(filepath.Join(root, "random_h32_evals.json"))
	if err != nil {
		t.Fatal(err)
	}
	n, err := LoadNGNN1(filepath.Join(root, "random_h32.nnue"))
	if err != nil {
		t.Fatal(err)
	}
	if n.hidden != 32 {
		t.Fatal("trainer fixture must have H=32")
	}
	type sample struct {
		FEN    string `json:"fen"`
		EvalCP *int   `json:"eval_cp"`
	}
	var cases []sample
	if err := json.Unmarshal(data, &cases); err != nil {
		var document struct {
			Cases     []sample `json:"cases"`
			Positions []sample `json:"positions"`
		}
		if err := json.Unmarshal(data, &document); err != nil {
			t.Fatal(err)
		}
		cases = document.Cases
		if len(cases) == 0 {
			cases = document.Positions
		}
	}
	if len(cases) != 64 {
		t.Fatalf("trainer fixture has %d cases, want 64", len(cases))
	}
	e := ngnn1NewWorker(t, n, newUCIStartingPosition())
	for i, c := range cases {
		if c.EvalCP == nil {
			t.Fatalf("fixture case %d has no eval_cp", i)
		}
		pos, err := ParseFEN(c.FEN)
		if err != nil {
			t.Fatal(err)
		}
		if err := e.Reset(pos); err != nil {
			t.Fatal(err)
		}
		if got := e.SearchSTM(pos); got != *c.EvalCP {
			t.Fatalf("trainer case %d: got %d want %d (%s)", i, got, *c.EvalCP, c.FEN)
		}
		if got := ngnn1Oracle(n, pos); got != *c.EvalCP {
			t.Fatalf("oracle case %d: got %d want %d", i, got, *c.EvalCP)
		}
	}
}

func TestNGNN1SearchSmoke(t *testing.T) {
	path := filepath.Join(t.TempDir(), "random.nnue")
	if err := os.WriteFile(path, ngnn1TestBytes(256, 19, false), 0600); err != nil {
		t.Fatal(err)
	}
	depth := 10
	if testing.Short() {
		depth = 3
	}
	for _, threads := range []int{1, 4} {
		u := NewUCIEngine()
		if err := u.searcher.ResizeHash(1); err != nil {
			t.Fatal(err)
		}
		var out bytes.Buffer
		for _, command := range []string{
			"setoption name OwnBook value false",
			fmt.Sprintf("setoption name Threads value %d", threads),
			"setoption name EvalFile value " + path,
			"setoption name UseNNUE value true",
			fmt.Sprintf("go depth %d", depth),
		} {
			u.handleCommand(command, &out)
		}
		u.lifecycleMu.Lock()
		session := u.activeSearch
		u.lifecycleMu.Unlock()
		if session != nil {
			<-session.done
		}
		s, n := u.searcher, u.evaluatorConfigSnapshot().network
		if s.SelectedEvaluatorBackend() != "ngnn1" || s.ThreadCount() != threads ||
			!strings.Contains(out.String(), fmt.Sprintf("info depth %d ", depth)) ||
			!strings.Contains(out.String(), "bestmove ") || strings.Contains(out.String(), "bestmove 0000") ||
			strings.Contains(out.String(), "info string error") {
			t.Fatalf("threads=%d depth=%d: %s", threads, depth, out.String())
		}
		for i, w := range s.configuredWorkers() {
			if w.evaluator.depth != 0 || w.evaluator.nnue.network != n {
				t.Fatal("worker did not restore root or share weights")
			}
			for j := 0; j < i; j++ {
				if &w.evaluator.nnue.stack[0] == &s.configuredWorkers()[j].evaluator.nnue.stack[0] {
					t.Fatal("workers alias accumulators")
				}
			}
		}
	}
}

func TestNGNN1WriteProbeNetwork(t *testing.T) {
	path := os.Getenv("NGN_NETINFER_NET_PATH")
	if path == "" {
		t.Skip("set NGN_NETINFER_NET_PATH to export the NGN random H=256 probe network")
	}
	if err := os.WriteFile(path, ngnn1TestBytes(256, 19, false), 0600); err != nil {
		t.Fatal(err)
	}
}
