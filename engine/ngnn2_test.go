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

func ngnn2TestBytes(h, nb int, extreme bool) []byte {
	base := ngnn1TestBytes(h, 19, extreme)
	var out bytes.Buffer
	out.WriteString("NGN2")
	for _, field := range []uint32{2, uint32(h), uint32(nb), 255, 64, 400} {
		_ = binary.Write(&out, binary.LittleEndian, field)
	}
	out.Write(base[24 : 24+769*h*2])
	out.Write(base[24+769*h*2 : len(base)-8])
	rng := rand.New(rand.NewSource(98231))
	for b := 1; b < nb; b++ {
		for i := 0; i < 2*h; i++ {
			w := int16(rng.Intn(257) - 128)
			if extreme {
				w = int16(rng.Uint32())
			}
			_ = binary.Write(&out, binary.LittleEndian, w)
		}
	}
	out.Write(base[len(base)-8 : len(base)-4])
	for b := 1; b < nb; b++ {
		_ = binary.Write(&out, binary.LittleEndian, int32(b*173-2000))
	}
	_ = binary.Write(&out, binary.LittleEndian, crc32.ChecksumIEEE(out.Bytes()))
	return out.Bytes()
}

// Count from the mailbox, independently of the production bitboard formula,
// and run the original scalar oracle with only the selected output row.
func ngnn2Oracle(n *NGNN1Network, pos *Position) int {
	count := 0
	for _, piece := range pos.Board.mailbox {
		if piece >= WhitePawn && piece <= BlackKing {
			count++
		}
	}
	b := 0
	for next := 1; next < n.bucketCount(); next++ {
		if (count-2)*n.bucketCount() >= next*32 {
			b = next
		}
	}
	view := *n
	view.output = n.output[b*2*n.hidden : (b+1)*2*n.hidden]
	if b > 0 {
		view.outputBias = n.otherBiases[b-1]
	}
	return ngnn1Oracle(&view, pos)
}

func TestNGNN2Validation(t *testing.T) {
	good := ngnn2TestBytes(32, 8, false)
	for _, field := range []struct {
		offset int
		value  uint32
	}{
		{0, 0}, {4, 1}, {8, 0}, {8, 17}, {8, 2064}, {8, 0xffffffff},
		{12, 0}, {12, 3}, {12, 16}, {12, 0xffffffff}, {16, 254}, {20, 65}, {24, 401},
	} {
		bad := append([]byte(nil), good...)
		binary.LittleEndian.PutUint32(bad[field.offset:], field.value)
		binary.LittleEndian.PutUint32(bad[len(bad)-4:], crc32.ChecksumIEEE(bad[:len(bad)-4]))
		if n, err := ReadNGNN1(bytes.NewReader(bad)); err == nil || n != nil {
			t.Fatalf("accepted field %d=%d", field.offset, field.value)
		}
	}
	for _, cut := range []int{0, 3, 23, 24, 27, 28, 100, len(good) - 1, len(good) - 4} {
		if n, err := ReadNGNN1(bytes.NewReader(good[:cut])); err == nil || n != nil {
			t.Fatalf("accepted truncated file of length %d", cut)
		}
	}
	bad := append([]byte(nil), good...)
	bad[99] ^= 1
	for _, data := range [][]byte{bad, append(append([]byte(nil), good...), 0)} {
		if n, err := ReadNGNN1(bytes.NewReader(data)); err == nil || n != nil {
			t.Fatal("accepted CRC corruption or trailing byte")
		}
	}
	for _, nb := range []int{1, 2, 4, 8} {
		for _, h := range []int{16, 48, 256, 2048} {
			n, err := ReadNGNN1(bytes.NewReader(ngnn2TestBytes(h, nb, false)))
			if err != nil || !n.valid() || n.version != 2 || n.buckets != nb {
				t.Fatalf("H=%d NB=%d: %v", h, nb, err)
			}
		}
	}
}

func TestNGNN2BucketsAndSingleBucketEquivalence(t *testing.T) {
	legacy := ngnn1TestNetwork(t, 32, false)
	for _, nb := range []int{1, 2, 4, 8} {
		n, err := ReadNGNN1(bytes.NewReader(ngnn2TestBytes(32, nb, false)))
		if err != nil {
			t.Fatal(err)
		}
		for count := 0; count <= 64; count++ {
			pos := &Position{EnPassant: NoSquare}
			for square := 0; square < count; square++ {
				pos.Board.UpdateSquare(Square(square), GetPiece(Pawn, Color(square%2)), NoPiece)
			}
			want := min(nb-1, max(0, (count-2)*nb/32))
			if got := n.bucket(pos); got != want {
				t.Fatalf("NB=%d pieces=%d got=%d want=%d", nb, count, got, want)
			}
			e := ngnn1NewWorker(t, n, pos)
			for _, turn := range []Color{White, Black} {
				pos.Tag = BlackToMove
				if turn == White {
					pos.Tag = WhiteToMove
				}
				if got := e.SearchSTM(pos); got != ngnn2Oracle(n, pos) {
					t.Fatalf("NB=%d pieces=%d oracle mismatch", nb, count)
				}
				if nb == 1 && e.SearchSTM(pos) != ngnn1Oracle(legacy, pos) {
					t.Fatal("NB=1 changed NGNN1 evaluation")
				}
			}
		}
	}
}

func TestNGNN2BucketKernelRows(t *testing.T) {
	for _, h := range []int{16, 256, 2048} {
		for _, extreme := range []bool{false, true} {
			n, err := ReadNGNN1(bytes.NewReader(ngnn2TestBytes(h, 8, extreme)))
			if err != nil {
				t.Fatal(err)
			}
			if n.smallOutput == extreme {
				t.Fatal("incorrect output bound")
			}
			acc := make([]int32, 2*h)
			for i := range acc {
				acc[i] = []int32{-2147483648, -1, 0, 19, 127, 254, 255, 2147483647}[i%8]
			}
			for b := 0; b < 8; b++ {
				row := n.output[b*2*h : (b+1)*2*h]
				for _, turn := range []Color{White, Black} {
					us, them := acc[:h], acc[h:]
					if turn == Black {
						us, them = them, us
					}
					wantSum := ngnn1DotGo(us, row[:h]) + ngnn1DotGo(them, row[h:])
					gotSum := ngnn1Dot(us, row[:h]) + ngnn1Dot(them, row[h:])
					if n.smallOutput {
						gotSum = ngnn1OutputSmall(us, them, row)
					}
					if gotSum != wantSum {
						t.Fatalf("H=%d bucket=%d turn=%d raw output mismatch", h, b, turn)
					}
					bias := n.outputBias
					if b > 0 {
						bias = n.otherBiases[b-1]
					}
					want := (wantSum/255 + int64(bias)) * 400 / (255 * 64)
					want = min(max(want, -ngnn1ScoreLimit), ngnn1ScoreLimit)
					if int64(n.evaluateBucket(acc, turn, b)) != want {
						t.Fatal("bucket output arithmetic mismatch")
					}
				}
			}
		}
	}
}

func ngnn2AssertRefresh(t *testing.T, e *workerEvaluator, pos *Position) {
	t.Helper()
	n := e.nnue.network
	want := make([]int32, 2*n.hidden)
	n.refresh(pos, want)
	if !reflect.DeepEqual(want, e.nnue.at(e.depth)) || e.SearchSTM(pos) != ngnn2Oracle(n, pos) {
		t.Fatalf("incremental/refresh/oracle mismatch at ply %d", e.depth)
	}
}

func TestNGNN2CaptureBoundaryAndSpecialMoves(t *testing.T) {
	for _, extreme := range []bool{false, true} {
		n, err := ReadNGNN1(bytes.NewReader(ngnn2TestBytes(32, 8, extreme)))
		if err != nil {
			t.Fatal(err)
		}
		for _, fen := range []string{
			"r3k2r/8/8/8/8/8/8/R3K2R w KQkq - 0 1", // six pieces: capture crosses 1 -> 0
			"r3k2r/8/8/8/8/8/8/R3K2R b KQkq - 0 1",
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
			ngnn2AssertRefresh(t, e, pos)
			crossings := 0
			for _, move := range GenerateLegalMoves(pos) {
				before := n.bucket(pos)
				tr := e.mustPrepareMove(pos, move)
				ep, tag, clock, _ := pos.MakeMove(move)
				e.mustPushMadeMove(pos, tr, move, tag, ep, clock)
				ngnn2AssertRefresh(t, e, pos)
				if n.bucket(pos) != before {
					crossings++
				}
				e.mustPop()
				pos.UnMakeMove(move, tag, ep, clock)
				ngnn2AssertRefresh(t, e, pos)
			}
			if crossings == 0 {
				t.Fatal("did not exercise a capture bucket crossing: " + fen)
			}
			if !pos.IsInCheck() {
				tr := e.mustPrepareNull(pos)
				ep := pos.MakeNullMove()
				e.mustPushMadeNull(pos, tr, ep)
				ngnn2AssertRefresh(t, e, pos)
				e.mustPop()
				pos.UnMakeNullMove(ep)
				ngnn2AssertRefresh(t, e, pos)
			}
		}
	}
}

func TestNGNN2TrainerParity(t *testing.T) {
	netPath := os.Getenv("NGN_NGNN2_PARITY_NET")
	evalsPath := os.Getenv("NGN_NGNN2_PARITY_EVALS")
	if netPath == "" && evalsPath == "" {
		root := filepath.Join("..", "testdata", "ngnn2")
		netPath, evalsPath = filepath.Join(root, "random_h32_b8.nnue"), filepath.Join(root, "random_h32_b8_evals.json")
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
		Format  string `json:"format"`
		Hidden  int    `json:"hidden"`
		Buckets int    `json:"buckets"`
		SHA256  string `json:"network_sha256"`
		Cases   []struct {
			FEN    string `json:"fen"`
			EvalCP *int   `json:"eval_cp"`
			Bucket int    `json:"bucket"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Format != "NGNN2" || fixture.Hidden != n.hidden || fixture.Buckets != n.buckets ||
		fixture.SHA256 != fmt.Sprintf("%x", sha256.Sum256(raw)) || len(fixture.Cases) < 20 {
		t.Fatal("fixture metadata disagrees with loaded network")
	}
	e := ngnn1NewWorker(t, n, newUCIStartingPosition())
	for i, sample := range fixture.Cases {
		pos, err := ParseFEN(sample.FEN)
		if err != nil {
			t.Fatal(err)
		}
		if err := e.Reset(pos); err != nil {
			t.Fatal(err)
		}
		if sample.EvalCP == nil || n.bucket(pos) != sample.Bucket || e.SearchSTM(pos) != *sample.EvalCP ||
			ngnn2Oracle(n, pos) != *sample.EvalCP {
			t.Fatalf("trainer case %d mismatch: %s", i, sample.FEN)
		}
	}
	t.Logf("%d exact trainer cases, H=%d NB=%d kernel=%s", len(fixture.Cases), n.hidden, n.buckets, ngnn1KernelBackend)
}

func TestNGNN2UCIAndFailClosed(t *testing.T) {
	u := NewUCIEngine()
	if err := u.searcher.ResizeHash(1); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "buckets.nnue")
	good := ngnn2TestBytes(32, 8, false)
	if err := os.WriteFile(path, good, 0600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	u.handleCommand("setoption name UseNNUE value true", &out)
	u.handleCommand("setoption name EvalFile value "+path, &out)
	u.handleCommand("eval", &out)
	if u.searcher.SelectedEvaluatorBackend() != "ngnn2" || !strings.Contains(out.String(), "eval backend ngnn2") {
		t.Fatal(out.String())
	}
	good[99] ^= 1
	if err := os.WriteFile(path, good, 0600); err != nil {
		t.Fatal(err)
	}
	u.handleCommand("setoption name EvalFile value "+path, &out)
	if u.searcher.SelectedEvaluatorBackend() != "hce" || u.evaluatorConfigSnapshot().network != nil {
		t.Fatal("corrupt reload retained stale NGNN2 weights")
	}
}

func TestNGNN2SearchWorkers(t *testing.T) {
	u := NewUCIEngine()
	if err := u.searcher.ResizeHash(1); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "buckets.nnue")
	if err := os.WriteFile(path, ngnn2TestBytes(256, 8, false), 0600); err != nil {
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
	if !strings.Contains(out.String(), "info depth 3 ") || !strings.Contains(out.String(), "bestmove ") ||
		strings.Contains(out.String(), "info string error") || u.searcher.SelectedEvaluatorBackend() != "ngnn2" {
		t.Fatal(out.String())
	}
	n := u.evaluatorConfigSnapshot().network
	for i, worker := range u.searcher.configuredWorkers() {
		if worker.evaluator.depth != 0 || worker.evaluator.nnue.network != n {
			t.Fatal("worker did not restore root/share network")
		}
		ngnn2AssertRefresh(t, worker.evaluator, u.position)
		for j := 0; j < i; j++ {
			if &worker.evaluator.nnue.stack[0] == &u.searcher.configuredWorkers()[j].evaluator.nnue.stack[0] {
				t.Fatal("workers share mutable accumulators")
			}
		}
	}
}
