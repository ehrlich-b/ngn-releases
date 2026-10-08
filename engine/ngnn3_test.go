package engine

import (
	"bytes"
	"crypto/sha256"
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

func ngnn3DefaultMap() [64]uint8 {
	var result [64]uint8
	for square := range result {
		rank, file := square/8, min(square%8, 7-square%8)
		bucket := 7
		switch {
		case rank < 2:
			bucket = rank*2 + file/2
		case rank == 2:
			bucket = 4
		case rank == 3:
			bucket = 5
		case rank < 6:
			bucket = 6
		}
		result[square] = uint8(bucket)
	}
	return result
}

func ngnn3TestBytes(h, kb, nb int, extreme bool) []byte {
	rng := rand.New(rand.NewSource(20261006))
	var out bytes.Buffer
	out.WriteString("NGN3")
	for _, v := range []uint32{3, uint32(h), uint32(kb), uint32(nb), 255, 64, 400} {
		_ = binary.Write(&out, binary.LittleEndian, v)
	}
	mapping := ngnn3DefaultMap()
	for _, b := range mapping {
		out.WriteByte(b % uint8(kb))
	}
	for i := 0; i < (kb*768+1+nb*2)*h; i++ {
		w := int16(rng.Intn(65) - 32)
		if i >= kb*768*h && i < (kb*768+1)*h {
			w = int16(rng.Intn(256))
		}
		if extreme {
			w = int16(rng.Uint32())
		}
		_ = binary.Write(&out, binary.LittleEndian, w)
	}
	for b := 0; b < nb; b++ {
		_ = binary.Write(&out, binary.LittleEndian, int32(137*b-1300))
	}
	_ = binary.Write(&out, binary.LittleEndian, crc32.ChecksumIEEE(out.Bytes()))
	return out.Bytes()
}

func ngnn3TestNetwork(t *testing.T, h int, extreme bool) *NGNN1Network {
	t.Helper()
	n, err := ReadNGNN1(bytes.NewReader(ngnn3TestBytes(h, 8, 8, extreme)))
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// Compute the transformation from rank/file arithmetic and mailbox kings,
// then accumulate in int64 without calling engine features or row kernels.
func ngnn3Oracle(n *NGNN1Network, pos *Position) ([]int32, int) {
	acc := make([]int32, 2*n.hidden)
	for p, color := range []Color{White, Black} {
		king := 0
		for square, piece := range pos.Board.mailbox {
			if piece == GetPiece(King, color) {
				king = square
				break
			}
		}
		rank, file := king/8, king%8
		if color == Black {
			rank = 7 - rank
		}
		mirror := file >= 4
		if mirror {
			file = 7 - file
		}
		bucket := int(n.kingMap[rank*8+file])
		for i, bias := range n.bias {
			x := int64(bias)
			for square, piece := range pos.Board.mailbox {
				if !validPiece(piece) {
					continue
				}
				r, f := square/8, square%8
				if color == Black {
					r = 7 - r
				}
				if mirror {
					f = 7 - f
				}
				feature := bucket*768 + ((int(piece)-1)%6)*64 + r*8 + f
				if piece.Color() != color {
					feature += 384
				}
				x += int64(n.weights[feature*n.hidden+i])
			}
			acc[p*n.hidden+i] = int32(x)
		}
	}
	count := 0
	for _, piece := range pos.Board.mailbox {
		if validPiece(piece) {
			count++
		}
	}
	bucket := min(n.bucketCount()-1, max(0, (count-2)*n.bucketCount()/32))
	bias := n.outputBias
	if bucket > 0 {
		bias = n.otherBiases[bucket-1]
	}
	var sum int64
	for half := 0; half < 2; half++ {
		p := half
		if pos.Turn() == Black {
			p = 1 - half
		}
		for i := 0; i < n.hidden; i++ {
			x := int64(min(255, max(0, acc[p*n.hidden+i])))
			sum += x * x * int64(n.output[bucket*2*n.hidden+half*n.hidden+i])
		}
	}
	cp := (sum/255 + int64(bias)) * 400 / (255 * 64)
	return acc, int(min(24999, max(-24999, cp)))
}

func ngnn3Assert(t *testing.T, e *workerEvaluator, pos *Position) {
	t.Helper()
	n := e.nnue.network
	want, cp := ngnn3Oracle(n, pos)
	full := make([]int32, 2*n.hidden)
	n.refresh(pos, full)
	if !reflect.DeepEqual(want, full) || !reflect.DeepEqual(want, e.nnue.at(e.depth)) || e.SearchSTM(pos) != cp {
		t.Fatalf("NGNN3 scalar/refresh/incremental mismatch at ply %d", e.depth)
	}
}

func TestNGNN3Validation(t *testing.T) {
	good := ngnn3TestBytes(32, 8, 8, false)
	for _, field := range [][2]uint32{
		{0, 0}, {4, 2}, {8, 0}, {8, 17}, {8, 2064}, {8, 0xffffffff},
		{12, 0}, {12, 9}, {12, 0xffffffff}, {16, 0}, {16, 3}, {16, 16},
		{20, 254}, {24, 65}, {28, 401},
	} {
		bad := bytes.Clone(good)
		binary.LittleEndian.PutUint32(bad[field[0]:], field[1])
		binary.LittleEndian.PutUint32(bad[len(bad)-4:], crc32.ChecksumIEEE(bad[:len(bad)-4]))
		if n, err := ReadNGNN1(bytes.NewReader(bad)); err == nil || n != nil {
			t.Fatalf("accepted bad header %v", field)
		}
	}
	for _, offset := range []int{32, 35, 63, 95} {
		bad := bytes.Clone(good)
		bad[offset] = 8
		binary.LittleEndian.PutUint32(bad[len(bad)-4:], crc32.ChecksumIEEE(bad[:len(bad)-4]))
		if n, err := ReadNGNN1(bytes.NewReader(bad)); err == nil || n != nil {
			t.Fatal("accepted out-of-range king map")
		}
	}
	for _, cut := range []int{0, 23, 24, 31, 32, 95, 96, len(good) - 1} {
		if n, err := ReadNGNN1(bytes.NewReader(good[:cut])); err == nil || n != nil {
			t.Fatalf("accepted truncated length %d", cut)
		}
	}
	bad := bytes.Clone(good)
	bad[100] ^= 1
	for _, data := range [][]byte{bad, append(bytes.Clone(good), 0)} {
		if n, err := ReadNGNN1(bytes.NewReader(data)); err == nil || n != nil {
			t.Fatal("accepted corruption or trailing data")
		}
	}
	for _, kb := range []int{1, 4, 5, 8} {
		for _, nb := range []int{1, 2, 4, 8} {
			n, err := ReadNGNN1(bytes.NewReader(ngnn3TestBytes(16, kb, nb, false)))
			if err != nil || !n.valid() || n.kingBuckets != kb || n.buckets != nb {
				t.Fatalf("KB=%d NB=%d: %v", kb, nb, err)
			}
			// A valid file-defined layout need not resemble the default map.
			for i := range n.kingMap {
				n.kingMap[i] = uint8((i/8 + 3) % kb)
			}
			n.weights[0] = -32768
			pos := newUCIStartingPosition()
			ngnn3Assert(t, ngnn1NewWorker(t, n, pos), pos)
		}
	}
	for _, h := range []int{48, 256, 2048} {
		n, err := ReadNGNN1(bytes.NewReader(ngnn3TestBytes(h, 8, 8, true)))
		if err != nil || !n.valid() {
			t.Fatalf("H=%d: %v", h, err)
		}
		pos := newUCIStartingPosition()
		ngnn3Assert(t, ngnn1NewWorker(t, n, pos), pos)
	}
}

func TestNGNN3KingTransitionsAndSpecialMoves(t *testing.T) {
	for _, extreme := range []bool{false, true} {
		n := ngnn3TestNetwork(t, 32, extreme)
		for _, fen := range []string{
			"r3k2r/8/8/8/8/8/8/R3K2R w KQkq - 0 1",
			"r3k2r/8/8/8/8/8/8/R3K2R b KQkq - 0 1",
			"7k/8/8/8/8/8/8/1K6 w - - 0 1",
			"7k/8/8/8/8/8/8/3K4 w - - 0 1",
			"7k/8/8/8/8/3K4/8/8 w - - 0 1",
			"4k3/8/8/3pP3/8/8/PP6/4K3 w - d6 0 1",
			"4k3/pp6/8/8/3Pp3/8/8/4K3 b - d3 0 1",
			"1r2k3/P7/8/8/8/8/PP6/4K3 w - - 0 1",
			"4k3/pp6/8/8/8/8/p7/1R2K3 b - - 0 1",
		} {
			pos, err := ParseFEN(fen)
			if err != nil {
				t.Fatal(err)
			}
			e := ngnn1NewWorker(t, n, pos)
			ngnn3Assert(t, e, pos)
			for _, move := range GenerateLegalMoves(pos) {
				tr := e.mustPrepareMove(pos, move)
				// The opponent's own king never moves; only the mover can refresh.
				if move.MovingPiece().Type() != King && tr.delta.refreshMask != 0 {
					t.Fatal("non-king move refreshed")
				}
				p := 0
				if pos.Turn() == Black {
					p = 1
				}
				if tr.delta.refreshMask&(1<<(1-p)) != 0 {
					t.Fatal("refreshed opponent perspective")
				}
				for visit := 0; visit < 2; visit++ {
					ep, tag, clock, _ := pos.MakeMove(move)
					e.mustPushMadeMove(pos, tr, move, tag, ep, clock)
					ngnn3Assert(t, e, pos)
					e.mustPop()
					pos.UnMakeMove(move, tag, ep, clock)
					ngnn3Assert(t, e, pos)
				}
			}
			if !pos.IsInCheck() {
				tr := e.mustPrepareNull(pos)
				ep := pos.MakeNullMove()
				e.mustPushMadeNull(pos, tr, ep)
				ngnn3Assert(t, e, pos)
				e.mustPop()
				pos.UnMakeNullMove(ep)
				ngnn3Assert(t, e, pos)
			}
		}
		for _, pair := range []struct {
			from, to Square
			mask     uint8
		}{
			{E1, F1, 0}, {D1, E1, 1}, {B1, C1, 1}, {D3, E3, 1}, {E4, F4, 0},
		} {
			pos := &Position{Tag: WhiteToMove, EnPassant: NoSquare}
			pos.Board.UpdateSquare(pair.from, WhiteKing, NoPiece)
			pos.Board.UpdateSquare(H8, BlackKing, NoPiece)
			move := NewMove(pair.from, pair.to, WhiteKing, NoPiece, NoType, 0)
			d := n.kingMoveDelta(pos, move, ngnn1MoveDelta(move))
			if d.refreshMask != pair.mask {
				t.Fatalf("%v -> %v refresh mask %d, want %d", pair.from, pair.to, d.refreshMask, pair.mask)
			}
		}
	}
}

func TestNGNN3RandomGamesAndSymmetry(t *testing.T) {
	rng := rand.New(rand.NewSource(4306))
	kingMoves, crossings := 0, 0
	for _, h := range []int{32, 256} {
		for _, extreme := range []bool{false, true} {
			n := ngnn3TestNetwork(t, h, extreme)
			for game := 0; game < 8; game++ {
				pos := newUCIStartingPosition()
				if game%2 == 1 {
					pos, _ = ParseFEN("r3k2r/pp6/8/8/8/8/PP6/R3K2R w KQkq - 0 1")
				}
				e := ngnn1NewWorker(t, n, pos)
				other := ngnn1NewWorker(t, n, pos)
				type undo struct {
					move  Move
					ep    Square
					tag   PositionTag
					clock uint8
				}
				var undos []undo
				for ply := 0; ply < 120; ply++ {
					ngnn3Assert(t, e, pos)
					for _, flip := range []int{7, 56, 63} {
						transformed := &Position{Tag: pos.Tag, EnPassant: NoSquare}
						if flip&56 != 0 {
							transformed.ToggleTurn()
						}
						for square, piece := range pos.Board.mailbox {
							if validPiece(piece) {
								if flip&56 != 0 {
									piece = GetPiece(piece.Type(), piece.Color().Other())
								}
								transformed.Board.UpdateSquare(Square(square^flip), piece, NoPiece)
							}
						}
						if err := other.Reset(transformed); err != nil {
							t.Fatal(err)
						}
						if e.SearchSTM(pos) != other.SearchSTM(transformed) {
							t.Fatalf("H=%d symmetry flip=%d", h, flip)
						}
						a, b := e.nnue.at(e.depth), other.nnue.at(0)
						if flip&56 != 0 {
							b = append(append([]int32{}, b[h:]...), b[:h]...)
						}
						if !reflect.DeepEqual(a, b) {
							t.Fatal("symmetry accumulator mismatch")
						}
					}
					moves := GenerateLegalMoves(pos)
					if len(moves) == 0 {
						break
					}
					move := moves[rng.Intn(len(moves))]
					if ply%4 != 0 {
						var kings []Move
						for _, candidate := range moves {
							if candidate.MovingPiece().Type() == King {
								kings = append(kings, candidate)
							}
						}
						if len(kings) != 0 {
							move = kings[rng.Intn(len(kings))]
						}
					}
					tr := e.mustPrepareMove(pos, move)
					if move.MovingPiece().Type() == King {
						kingMoves++
					}
					if tr.delta.refreshMask != 0 {
						crossings++
					}
					ep, tag, clock, _ := pos.MakeMove(move)
					e.mustPushMadeMove(pos, tr, move, tag, ep, clock)
					undos = append(undos, undo{move, ep, tag, clock})
				}
				for i := len(undos) - 1; i >= 0; i-- {
					ngnn3Assert(t, e, pos)
					u := undos[i]
					e.mustPop()
					pos.UnMakeMove(u.move, u.tag, u.ep, u.clock)
				}
				ngnn3Assert(t, e, pos)
				if &e.nnue.refreshCache[0] == &other.nnue.refreshCache[0] {
					t.Fatal("workers share refresh cache")
				}
				if err := e.Reset(pos); err != nil {
					t.Fatal(err)
				}
				ngnn3Assert(t, e, pos)
			}
		}
	}
	if kingMoves < 100 || crossings < 50 {
		t.Fatalf("insufficient king/cache coverage: %d king moves, %d crossings", kingMoves, crossings)
	}
	t.Logf("%d king moves, %d bucket/mirror transitions", kingMoves, crossings)
}

func TestNGNN3TrainerParity(t *testing.T) {
	netPath, evalsPath := os.Getenv("NGN_NGNN3_PARITY_NET"), os.Getenv("NGN_NGNN3_PARITY_EVALS")
	if netPath == "" && evalsPath == "" {
		netPath = filepath.Join("..", "testdata", "ngnn3", "random_h32_k8_b8.nnue")
		evalsPath = filepath.Join("..", "testdata", "ngnn3", "random_h32_k8_b8_evals.json")
	}
	raw, err := os.ReadFile(netPath)
	if err != nil {
		t.Fatal(err)
	}
	n, err := ReadNGNN1(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(evalsPath)
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Format      string `json:"format"`
		Hidden      int    `json:"hidden"`
		Buckets     int    `json:"buckets"`
		KingBuckets int    `json:"king_buckets"`
		SHA256      string `json:"network_sha256"`
		Cases       []struct {
			FEN    string `json:"fen"`
			CP     *int   `json:"eval_cp"`
			Bucket int    `json:"bucket"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Format != "NGNN3" || fixture.Hidden != n.hidden || fixture.KingBuckets != n.kingBuckets || fixture.Buckets != n.buckets || fixture.SHA256 != fmt.Sprintf("%x", sha256.Sum256(raw)) || len(fixture.Cases) < 20 {
		t.Fatal("NGNN3 fixture metadata mismatch")
	}
	e := ngnn1NewWorker(t, n, newUCIStartingPosition())
	for i, c := range fixture.Cases {
		pos, err := ParseFEN(c.FEN)
		if err != nil {
			t.Fatal(err)
		}
		if err := e.Reset(pos); err != nil {
			t.Fatal(err)
		}
		ngnn3Assert(t, e, pos)
		if c.CP == nil || e.SearchSTM(pos) != *c.CP || n.bucket(pos) != c.Bucket {
			t.Fatalf("trainer case %d: %s", i, c.FEN)
		}
	}
	t.Logf("%d exact trainer cases, H=%d KB=%d NB=%d kernel=%s", len(fixture.Cases), n.hidden, n.kingBuckets, n.buckets, ngnn1KernelBackend)
}

func TestNGNN3UCIWorkersAndReload(t *testing.T) {
	u := NewUCIEngine()
	if err := u.searcher.ResizeHash(1); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "king-buckets.nnue")
	data := ngnn3TestBytes(256, 8, 8, false)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	for _, command := range []string{
		"setoption name OwnBook value false", "setoption name Threads value 4",
		"setoption name EvalFile value " + path, "setoption name UseNNUE value true",
		"position fen r3k2r/8/8/8/8/8/8/R3K2R w KQkq - 0 1", "go depth 3",
	} {
		u.handleCommand(command, &out)
	}
	u.lifecycleMu.Lock()
	session := u.activeSearch
	u.lifecycleMu.Unlock()
	if session != nil {
		<-session.done
	}
	if !strings.Contains(out.String(), "info depth 3 ") || !strings.Contains(out.String(), "bestmove ") || u.searcher.SelectedEvaluatorBackend() != "ngnn3" {
		t.Fatal(out.String())
	}
	for i, worker := range u.searcher.configuredWorkers() {
		if worker.evaluator.depth != 0 {
			t.Fatal("worker did not restore root")
		}
		ngnn3Assert(t, worker.evaluator, u.position)
		for j := 0; j < i; j++ {
			if &worker.evaluator.nnue.refreshCache[0] == &u.searcher.configuredWorkers()[j].evaluator.nnue.refreshCache[0] {
				t.Fatal("workers share mutable refresh caches")
			}
		}
	}
	old := u.evaluatorConfigSnapshot().network
	if err := os.WriteFile(path, ngnn3TestBytes(32, 4, 2, false), 0600); err != nil {
		t.Fatal(err)
	}
	u.handleCommand("setoption name EvalFile value "+path, &out)
	u.handleCommand("eval", &out)
	if u.evaluatorConfigSnapshot().network == old || u.searcher.SelectedEvaluatorBackend() != "ngnn3" {
		t.Fatal("reload retained old model")
	}
	ngnn3Assert(t, u.searcher.worker.evaluator, u.position)
	data[100] ^= 1
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	u.handleCommand("setoption name EvalFile value "+path, &out)
	if u.searcher.SelectedEvaluatorBackend() != "hce" || u.evaluatorConfigSnapshot().network != nil {
		t.Fatal("invalid NGNN3 reload retained model")
	}
}
