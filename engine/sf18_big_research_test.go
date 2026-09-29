package engine

import (
	"errors"
	"os"
	"reflect"
	"sync"
	"testing"

	"github.com/ehrlich-b/ngn/nnue/sf18big"
)

const sf18BigResearchModelEnvironment = "NGN_SF18_BIG_OFFICIAL_FILE"

var (
	sf18BigResearchOnce  sync.Once
	sf18BigResearchModel *sf18big.Model
	sf18BigResearchError error
)

func loadSF18BigResearchModel(tb testing.TB) *sf18big.Model {
	tb.Helper()
	path := os.Getenv(sf18BigResearchModelEnvironment)
	if path == "" {
		tb.Skip("official SF18 BIG model is required for the research search gate")
	}
	sf18BigResearchOnce.Do(func() {
		file, err := os.Open(path)
		if err != nil {
			sf18BigResearchError = err
			return
		}
		defer file.Close()
		sf18BigResearchModel, sf18BigResearchError = sf18big.Load(file)
	})
	if sf18BigResearchError != nil {
		tb.Fatal(sf18BigResearchError)
	}
	return sf18BigResearchModel
}

func TestSF18BigResearchScoreAdapter(t *testing.T) {
	bareKings := n3cPosition(t, "4k3/8/8/8/8/8/8/4K3 w - - 0 1")
	withPawn := n3cPosition(t, "4k3/8/8/8/8/8/P7/4K3 w - - 0 1")
	tests := []struct {
		name       string
		components sf18big.Components
		halfmove   uint8
		position   *Position
		want       int
	}{
		{"zero", sf18big.Components{}, 0, bareKings, 0},
		{"stockfish-pawn-scale", sf18big.Components{PSQT: 208, Positional: 208}, 0, bareKings, 200},
		{"material-scale", sf18big.Components{PSQT: 2080, Positional: 2080}, 0, withPawn, 2013},
		{"rule50-half", sf18big.Components{PSQT: 208, Positional: 208}, 128, bareKings, 100},
		{"negative-complexity", sf18big.Components{PSQT: -208, Positional: 0}, 0, bareKings, -96},
		{"positive-clamp", sf18big.Components{PSQT: 1 << 30, Positional: 1 << 30}, 0, bareKings, int(nnueStaticEvalLimit)},
		{"negative-clamp", sf18big.Components{PSQT: -(1 << 30), Positional: -(1 << 30)}, 0, bareKings, -int(nnueStaticEvalLimit)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.position.HalfMoveClock = test.halfmove
			if got := sf18BigSearchScore(test.components, test.position); got != test.want {
				t.Fatalf("score = %d, want %d", got, test.want)
			}
		})
	}
}

func TestSF18BigResearchSelectionAndSearchMatchesFullRefresh(t *testing.T) {
	model := loadSF18BigResearchModel(t)
	const fen = "r3k2r/p1ppqpb1/bn2pnp1/2pP4/1p2P3/2N2N2/PPQBBPPP/R3K2R w KQkq - 37 1"

	incremental := NewSearchEngine()
	beforeModel := incremental.evaluatorModel
	beforeWorker := incremental.worker.evaluator
	if err := incremental.SelectSF18BIGResearchEvaluator(nil); !errors.Is(err, errWorkerEvaluator) {
		t.Fatalf("nil selection error = %v", err)
	}
	if incremental.evaluatorModel != beforeModel || incremental.worker.evaluator != beforeWorker {
		t.Fatal("failed SF18 BIG selection changed receiver state")
	}
	if err := incremental.SelectSF18BIGResearchEvaluator(model); err != nil {
		t.Fatal(err)
	}
	if got := incremental.SelectedEvaluatorBackend(); got != EvaluatorBackendSF18BIGResearchName {
		t.Fatalf("backend = %q", got)
	}

	root := n3cPosition(t, fen)
	gotScore, backend, err := incremental.EvaluateSelected(root.Copy())
	if err != nil {
		t.Fatal(err)
	}
	position, err := incremental.worker.evaluator.rootBuffer.position(root)
	if err != nil {
		t.Fatal(err)
	}
	selected, err := model.EvaluateSelected(position)
	if err != nil {
		t.Fatal(err)
	}
	wantScore := sf18BigSearchScore(selected.Components, root)
	if gotScore != wantScore || backend != EvaluatorBackendSF18BIGResearchName {
		t.Fatalf("selected score/backend = %d/%q, want %d/%q", gotScore, backend, wantScore, EvaluatorBackendSF18BIGResearchName)
	}

	fullRefresh := NewSearchEngine()
	if err := fullRefresh.SelectSF18BIGResearchEvaluator(model); err != nil {
		t.Fatal(err)
	}
	fullRefresh.worker.evaluator.fullRefreshOracle = true
	incrementalPos := root.Copy()
	fullRefreshPos := root.Copy()
	incrementalBefore := snapshotPVPosition(incrementalPos)
	fullRefreshBefore := snapshotPVPosition(fullRefreshPos)
	got := incremental.SearchFixed(incrementalPos, 3, nil)
	want := fullRefresh.SearchFixed(fullRefreshPos, 3, nil)
	if gotComparable, wantComparable := comparableN3CSearchInfo(got), comparableN3CSearchInfo(want); !reflect.DeepEqual(gotComparable, wantComparable) {
		t.Fatalf("SF18 BIG incremental search differs from full refresh:\nincremental=%+v\nfull-refresh=%+v", gotComparable, wantComparable)
	}
	if incremental.worker.evaluator.nnueDepth() != 0 || fullRefresh.worker.evaluator.nnueDepth() != 0 {
		t.Fatalf("context depths incremental=%d full=%d", incremental.worker.evaluator.nnueDepth(), fullRefresh.worker.evaluator.nnueDepth())
	}
	if after := snapshotPVPosition(incrementalPos); !reflect.DeepEqual(after, incrementalBefore) {
		t.Fatal("incremental search changed root position")
	}
	if after := snapshotPVPosition(fullRefreshPos); !reflect.DeepEqual(after, fullRefreshBefore) {
		t.Fatal("full-refresh search changed root position")
	}
}

func BenchmarkSF18BigResearchFixedNodes(b *testing.B) {
	model := loadSF18BigResearchModel(b)
	fens := []string{
		"r1bq1rk1/pp2bppp/2n2n2/2pp4/3P4/2N1PN2/PPQ1BPPP/R1B2RK1 w - - 0 10",
		"r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1",
		"8/2p5/3p4/KP5r/1R3p1k/8/4P1P1/8 w - - 0 1",
	}
	const nodesPerPosition = uint64(5000)
	for _, backend := range []string{"hce", "sf18-big"} {
		b.Run(backend, func(b *testing.B) {
			b.ReportAllocs()
			var nodes uint64
			for iteration := 0; iteration < b.N; iteration++ {
				for _, fen := range fens {
					position, err := ParseFEN(fen)
					if err != nil {
						b.Fatal(err)
					}
					searcher, err := NewSearchEngineWithHash(16)
					if err != nil {
						b.Fatal(err)
					}
					if backend == "sf18-big" {
						if err := searcher.SelectSF18BIGResearchEvaluator(model); err != nil {
							b.Fatal(err)
						}
					}
					searcher.SetMaxNodes(nodesPerPosition)
					info := searcher.SearchFixed(position, 64, nil)
					if !info.Stopped || info.Nodes < nodesPerPosition {
						b.Fatalf("fixed-node search stopped=%v nodes=%d", info.Stopped, info.Nodes)
					}
					nodes += info.Nodes
				}
			}
			b.ReportMetric(float64(nodes)/float64(b.N), "nodes/op")
			b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(nodes), "ns/node")
		})
	}
}
