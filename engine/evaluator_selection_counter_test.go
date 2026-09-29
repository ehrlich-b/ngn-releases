package engine

import (
	"reflect"
	"testing"
)

func TestSearchCounterIncrementalMatchesFullRefreshEveryCommittedFrame(t *testing.T) {
	model := loadVaryingCounterModel(t)
	const fen = "r3k2r/p1ppqpb1/bn2pnp1/2pP4/1p2P3/2N2N2/PPQBBPPP/R3K2R w KQkq - 37 1"
	incremental := NewSearchEngine()
	fullRefresh := NewSearchEngine()
	if err := incremental.SelectCounter55Evaluator(model); err != nil {
		t.Fatal(err)
	}
	if err := fullRefresh.SelectCounter55Evaluator(model); err != nil {
		t.Fatal(err)
	}
	incrementalPos := n3cPosition(t, fen)
	rootBoard, err := counterBoardFromPosition(incrementalPos)
	if err != nil {
		t.Fatal(err)
	}
	rootRaw, err := model.EvaluateFullRefresh(rootBoard)
	if err != nil {
		t.Fatal(err)
	}
	observed := 0
	changedFromRoot := false
	incremental.worker.evaluator.transitionObserver = func(pos *Position, evaluator *workerEvaluator) {
		observed++
		raw := requireCounterRawMatchesFull(t, evaluator, pos)
		if raw != rootRaw {
			changedFromRoot = true
		}
	}
	fullRefresh.worker.evaluator.fullRefreshOracle = true

	fullRefreshPos := n3cPosition(t, fen)
	incrementalBefore := snapshotPVPosition(incrementalPos)
	fullRefreshBefore := snapshotPVPosition(fullRefreshPos)
	got := incremental.SearchFixed(incrementalPos, 3, nil)
	want := fullRefresh.SearchFixed(fullRefreshPos, 3, nil)
	if observed < 2 || !changedFromRoot {
		t.Fatalf("Counter observer calls=%d changed-from-root=%v", observed, changedFromRoot)
	}
	if gotComparable, wantComparable := comparableN3CSearchInfo(got), comparableN3CSearchInfo(want); !reflect.DeepEqual(gotComparable, wantComparable) {
		t.Fatalf("Counter incremental search differs from full refresh:\nincremental=%+v\nfull-refresh=%+v", gotComparable, wantComparable)
	}
	if !reflect.DeepEqual(snapshotPVPosition(incrementalPos), incrementalBefore) ||
		!reflect.DeepEqual(snapshotPVPosition(fullRefreshPos), fullRefreshBefore) {
		t.Fatal("Counter search changed a root position")
	}
	if incremental.worker.evaluator.nnueDepth() != 0 || fullRefresh.worker.evaluator.nnueDepth() != 0 {
		t.Fatalf("Counter context depths incremental=%d full=%d", incremental.worker.evaluator.nnueDepth(), fullRefresh.worker.evaluator.nnueDepth())
	}
	if incremental.worker.hce != nil || fullRefresh.worker.hce != nil {
		t.Fatal("Counter-only search allocated an HCE cache")
	}
}

func TestCounterStoppedSearchUnwindsContextAndPosition(t *testing.T) {
	model := loadVaryingCounterModel(t)
	searcher := NewSearchEngine()
	if err := searcher.SelectCounter55Evaluator(model); err != nil {
		t.Fatal(err)
	}
	pos := n3cPosition(t, "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1")
	before := snapshotPVPosition(pos)
	rootBoard, err := counterBoardFromPosition(pos)
	if err != nil {
		t.Fatal(err)
	}
	rootRaw, err := model.EvaluateFullRefresh(rootBoard)
	if err != nil {
		t.Fatal(err)
	}
	observed := 0
	changedFromRoot := false
	searcher.worker.evaluator.transitionObserver = func(pos *Position, evaluator *workerEvaluator) {
		observed++
		raw := requireCounterRawMatchesFull(t, evaluator, pos)
		if raw != rootRaw {
			changedFromRoot = true
		}
	}
	searcher.SetMaxNodes(1)
	t.Cleanup(func() { searcher.SetMaxNodes(0) })
	info := searcher.SearchFixed(pos, 5, nil)
	if !info.Stopped || observed < 2 || !changedFromRoot {
		t.Fatalf("stopped=%v observer calls=%d changed-from-root=%v", info.Stopped, observed, changedFromRoot)
	}
	if searcher.worker.evaluator.nnueDepth() != 0 || !reflect.DeepEqual(snapshotPVPosition(pos), before) {
		t.Fatal("stopped Counter search did not restore context and position")
	}
	requireCounterRawMatchesFull(t, searcher.worker.evaluator, pos)
}

func TestCounterSingularVerificationRemakesAndRepushesNormally(t *testing.T) {
	model := loadVaryingCounterModel(t)
	searcher, err := NewSearchEngineWithHash(1)
	if err != nil {
		t.Fatal(err)
	}
	if err := searcher.SelectCounter55Evaluator(model); err != nil {
		t.Fatal(err)
	}
	pos := n3cPosition(t, "4k3/8/8/8/8/8/8/R3K3 w - - 0 1")
	before := snapshotPVPosition(pos)

	searcher.sessionMu.Lock()
	defer searcher.sessionMu.Unlock()
	generation := mustAcquireHCEModelUse()
	defer releaseHCEModelUse()
	tt := searcher.prepareTTGeneration(generation)
	evaluator := searcher.mustPreparePrimaryEvaluator(pos, generation)
	depth := SINGULAR_DEPTH
	info := newN3CDirectSearchInfo(searcher, evaluator, tt, depth)
	ttMove := adapterLegalMove(t, pos, "a1a8")
	child := pos.Copy()
	childEP, childTag, childClock, _ := child.MakeMove(ttMove)
	childHash := child.Hash()
	child.UnMakeMove(ttMove, childTag, childEP, childClock)
	rootScore := counterSearchScore(evaluator.counterContext.EvaluateRaw(), pos, true)
	tt.Set(pos.Hash(), ttMove, scoreToTT(rootScore+256, 0), int8(depth), Exact, false)

	sawVerification := false
	sawRepush := false
	evaluator.transitionObserver = func(observedPos *Position, evaluator *workerEvaluator) {
		requireCounterRawMatchesFull(t, evaluator, observedPos)
		if info.ExcludedMove != EmptyMove {
			sawVerification = true
		}
		if info.SingularTries > 0 && info.ExcludedMove == EmptyMove && observedPos.Hash() == childHash {
			sawRepush = true
		}
	}
	score := alphaBetaPV(pos, depth, 0, -INFINITY, INFINITY, true, true, false, info)
	if info.Stopped || score == stoppedSearchScore || info.SingularTries == 0 || !sawVerification || !sawRepush {
		t.Fatalf("Counter singular score=%d stopped=%v tries=%d verification=%v repush=%v", score, info.Stopped, info.SingularTries, sawVerification, sawRepush)
	}
	if evaluator.nnueDepth() != 0 || !reflect.DeepEqual(snapshotPVPosition(pos), before) {
		t.Fatal("Counter singular verification did not restore parent state")
	}
	requireCounterRawMatchesFull(t, evaluator, pos)
}

func TestCounterSelectionFailureAndModelReplacementAreTransactional(t *testing.T) {
	searcher, err := NewSearchEngineWithHash(1)
	if err != nil {
		t.Fatal(err)
	}
	beforeModel := searcher.evaluatorModel
	beforeWorker := searcher.worker.evaluator
	beforeIdentity := searcher.ttIdentity
	if err := searcher.SelectCounter55Evaluator(nil); err == nil {
		t.Fatal("nil Counter model selected")
	}
	if searcher.evaluatorModel != beforeModel || searcher.worker.evaluator != beforeWorker || searcher.ttIdentity != beforeIdentity {
		t.Fatal("failed Counter selection changed published state")
	}

	modelA := loadSparseCounterModel(t, nil)
	modelB := loadSparseCounterModel(t, map[int]float32{counterOutputWeightIndex(0): 1})
	if err := searcher.SelectCounter55Evaluator(modelA); err != nil {
		t.Fatal(err)
	}
	terminal := n3cPosition(t, "7k/6Q1/6K1/8/8/8/8/8 b - - 0 1")
	searcher.SearchFixed(terminal.Copy(), 1, nil)
	identityA := searcher.worker.evaluatorIdentity
	poisonMove := NewMove(A1, A2, WhiteKing, NoPiece, NoType, 0)
	poisonN3CWorkerHistory(&searcher.worker.history, poisonMove)
	const keyA = uint64(0xc055a)
	searcher.TTStore(keyA, poisonMove, 17, 3, Exact, false)

	if err := searcher.SelectCounter55Evaluator(modelB); err != nil {
		t.Fatal(err)
	}
	searcher.SearchFixed(terminal.Copy(), 1, nil)
	if identityB := searcher.worker.evaluatorIdentity; identityB == identityA || identityB.backend != evaluatorBackendCounter55 {
		t.Fatalf("Counter replacement identity old=%+v new=%+v", identityA, identityB)
	}
	if _, _, _, _, hit, _ := searcher.TTProbe(keyA); hit {
		t.Fatal("Counter model replacement retained prior TT entry")
	}
	if searcher.worker.history != (workerHistory{}) {
		t.Fatal("Counter model replacement retained prior history")
	}
}
