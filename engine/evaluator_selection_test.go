package engine

import (
	"errors"
	"reflect"
	"testing"

	"github.com/ehrlich-b/ngn/nnue"
)

func n3cPosition(t *testing.T, fen string) *Position {
	t.Helper()
	pos, err := ParseFEN(fen)
	if err != nil {
		t.Fatal(err)
	}
	return pos
}

func comparableN3CSearchInfo(info *SearchInfo) SearchInfo {
	copy := *info
	copy.TimeManager = nil
	copy.MoveBuffer = nil
	copy.CaptureBuffer = nil
	copy.OrderedBuffer = nil
	copy.SEEGains = nil
	copy.control = nil
	copy.history = nil
	copy.evaluator = nil
	copy.tt = nil
	copy.Frames = nil
	return copy
}

func TestSearchNGNV1IncrementalMatchesFullRefreshEveryCommittedFrame(t *testing.T) {
	model, tensors := adapterContextModel(t)
	const fen = "r3k2r/p1ppqpb1/bn2pnp1/2pP4/1p2P3/2N2N2/PPQBBPPP/R3K2R w KQkq - 37 1"

	incremental := NewSearchEngine()
	fullRefresh := NewSearchEngine()
	if err := incremental.SelectNGNV1Evaluator(model); err != nil {
		t.Fatal(err)
	}
	if err := fullRefresh.SelectNGNV1Evaluator(model); err != nil {
		t.Fatal(err)
	}

	observed := 0
	incremental.worker.evaluator.transitionObserver = func(pos *Position, evaluator *workerEvaluator) {
		observed++
		requireWorkerContextLanes(t, evaluator, tensors, fullPositionForWorkerTest(t, pos))
	}
	fullRefresh.worker.evaluator.fullRefreshOracle = true

	incrementalPos := n3cPosition(t, fen)
	fullRefreshPos := n3cPosition(t, fen)
	incrementalBefore := snapshotPVPosition(incrementalPos)
	fullRefreshBefore := snapshotPVPosition(fullRefreshPos)
	got := incremental.SearchFixed(incrementalPos, 3, nil)
	want := fullRefresh.SearchFixed(fullRefreshPos, 3, nil)

	if observed < 2 {
		t.Fatalf("incremental lane observer ran %d times, want root and child frames", observed)
	}
	if gotComparable, wantComparable := comparableN3CSearchInfo(got), comparableN3CSearchInfo(want); !reflect.DeepEqual(gotComparable, wantComparable) {
		t.Fatalf("incremental search differs from full-refresh oracle:\nincremental=%+v\nfull-refresh=%+v", gotComparable, wantComparable)
	}
	if after := snapshotPVPosition(incrementalPos); !reflect.DeepEqual(after, incrementalBefore) {
		t.Fatal("incremental search changed root position")
	}
	if after := snapshotPVPosition(fullRefreshPos); !reflect.DeepEqual(after, fullRefreshBefore) {
		t.Fatal("full-refresh search changed root position")
	}
	if incremental.worker.evaluator.nnueDepth() != 0 || fullRefresh.worker.evaluator.nnueDepth() != 0 {
		t.Fatalf("context depths incremental=%d full=%d", incremental.worker.evaluator.nnueDepth(), fullRefresh.worker.evaluator.nnueDepth())
	}
	if incremental.worker.hce != nil || fullRefresh.worker.hce != nil {
		t.Fatal("NNUE-only search allocated an unused HCE cache")
	}
	requireWorkerRawMatchesFull(t, incremental.worker.evaluator, incrementalPos)
}

func TestEvaluatorSelectionIsTransactionalAndCompositeIdentityClearsState(t *testing.T) {
	searcher, err := NewSearchEngineWithHash(1)
	if err != nil {
		t.Fatal(err)
	}
	terminal := n3cPosition(t, "7k/6Q1/6K1/8/8/8/8/8 b - - 0 1")
	searcher.SearchFixed(terminal, 1, nil)

	poisonMove := NewMove(A1, A2, WhiteKing, NoPiece, NoType, 0)
	searcher.worker.history.historyTable[WhiteKing][A2] = 1
	searcher.worker.history.continuationHistory[WhiteKing][A1][WhiteKing][A2] = 2
	searcher.worker.history.followupHistory[WhiteKing][A1][WhiteKing][A2] = 3
	searcher.worker.history.pawnCorrectionHistory[White][1] = 4
	searcher.worker.history.nonPawnCorrectionHistory[Black][2] = 5
	searcher.worker.history.minorCorrectionHistory[White][3] = 6
	searcher.worker.history.captureHistory[WhiteKing][A2][BlackPawn] = 7
	searcher.worker.history.killerMoves[0][0] = poisonMove
	searcher.worker.history.counterMoves[A1][A2] = poisonMove
	searcher.worker.history.lastMovePlayed = poisonMove
	const hceKey = uint64(0x4e3348ce)
	searcher.TTStore(hceKey, poisonMove, 17, 3, Exact, false)
	hceIdentity := searcher.ttIdentity

	beforeModel := searcher.evaluatorModel
	beforeWorker := searcher.worker.evaluator
	if err := searcher.SelectNGNV1Evaluator(nil); !errors.Is(err, errWorkerEvaluator) {
		t.Fatalf("invalid selection error = %v", err)
	}
	if searcher.evaluatorModel != beforeModel || searcher.worker.evaluator != beforeWorker || searcher.ttIdentity != hceIdentity {
		t.Fatal("failed NNUE selection changed published receiver state")
	}

	model, _ := adapterContextModel(t)
	if err := searcher.SelectNGNV1Evaluator(model); err != nil {
		t.Fatal(err)
	}
	if searcher.worker.evaluator == nil || searcher.worker.evaluator.context != nil ||
		searcher.worker.evaluator.portableContext == nil {
		t.Fatal("NNUE selection did not prebuild a private portable int32 Context")
	}
	if _, _, _, _, hit, _ := searcher.TTProbe(hceKey); hit {
		t.Fatal("backend swap retained HCE TT entry")
	}
	if searcher.ttIdentity == hceIdentity || searcher.ttIdentity.backend != evaluatorBackendNGNV1 {
		t.Fatalf("TT identity after NNUE selection = %+v, old = %+v", searcher.ttIdentity, hceIdentity)
	}

	searcher.SearchFixed(terminal.Copy(), 1, nil)
	if searcher.worker.history != (workerHistory{}) {
		t.Fatal("backend swap did not clear the complete worker-history family")
	}
	nnueIdentity := searcher.worker.evaluatorIdentity
	searcher.worker.history.lastMovePlayed = poisonMove
	if err := searcher.SelectNGNV1Evaluator(model); err != nil {
		t.Fatal(err)
	}
	searcher.SearchFixed(terminal.Copy(), 1, nil)
	if searcher.worker.evaluatorIdentity != nnueIdentity || searcher.worker.history.lastMovePlayed != poisonMove {
		t.Fatal("semantically identical model selection discarded same-identity warm history")
	}

	const nnueKey = uint64(0x4e3348cf)
	searcher.TTStore(nnueKey, poisonMove, 18, 3, Exact, false)
	if err := searcher.SelectHCEEvaluator(); err != nil {
		t.Fatal(err)
	}
	if _, _, _, _, hit, _ := searcher.TTProbe(nnueKey); hit {
		t.Fatal("NNUE-to-HCE swap retained NNUE TT entry")
	}
	searcher.SearchFixed(terminal.Copy(), 1, nil)
	if searcher.worker.evaluatorIdentity.backend != evaluatorBackendHCE || searcher.worker.history != (workerHistory{}) {
		t.Fatalf("HCE re-selection state identity=%+v history-cleared=%v", searcher.worker.evaluatorIdentity, searcher.worker.history == (workerHistory{}))
	}

	if err := tryBeginHCEModelMutation(); err != nil {
		t.Fatal(err)
	}
	if err := searcher.SelectNGNV1Evaluator(model); !errors.Is(err, ErrHCEModelBusy) {
		abortHCEModelMutation()
		t.Fatalf("selection during mutation error = %v", err)
	}
	abortHCEModelMutation()
}

func TestNGNV1StoppedSearchUnwindsContextAndPosition(t *testing.T) {
	model, tensors := adapterContextModel(t)
	searcher := NewSearchEngine()
	if err := searcher.SelectNGNV1Evaluator(model); err != nil {
		t.Fatal(err)
	}
	pos := n3cPosition(t, "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1")
	before := snapshotPVPosition(pos)
	observed := 0
	searcher.worker.evaluator.transitionObserver = func(pos *Position, evaluator *workerEvaluator) {
		observed++
		requireWorkerContextLanes(t, evaluator, tensors, fullPositionForWorkerTest(t, pos))
	}
	searcher.SetMaxNodes(1)
	t.Cleanup(func() { searcher.SetMaxNodes(0) })
	info := searcher.SearchFixed(pos, 5, nil)
	if !info.Stopped {
		t.Fatal("fixed node limit did not stop NNUE search")
	}
	if observed < 2 {
		t.Fatalf("observer calls = %d, want root and searched child", observed)
	}
	if searcher.worker.evaluator.nnueDepth() != 0 {
		t.Fatalf("stopped search left Context depth %d", searcher.worker.evaluator.nnueDepth())
	}
	if after := snapshotPVPosition(pos); !reflect.DeepEqual(after, before) {
		t.Fatal("stopped NNUE search did not restore root position")
	}
	requireWorkerRawMatchesFull(t, searcher.worker.evaluator, pos)
}

func newN3CDirectSearchInfo(searcher *SearchEngine, evaluator *workerEvaluator, tt *Cache, depth int) *SearchInfo {
	var moveBuffer [256]Move
	var captureBuffer [64]Move
	var orderedBuffer [256]Move
	var seeGains [32]int
	return &SearchInfo{
		Depth:         depth,
		RootDepth:     depth,
		BestMove:      EmptyMove,
		BestScore:     -INFINITY,
		MoveBuffer:    &moveBuffer,
		CaptureBuffer: &captureBuffer,
		OrderedBuffer: &orderedBuffer,
		SEEGains:      &seeGains,
		Frames:        make([]searchFrame, searchFramePoolSize),
		control:       &searcher.worker.control,
		maxNodes:      searcher.worker.control.MaxNodes(),
		history:       &searcher.worker.history,
		evaluator:     evaluator,
		tt:            tt,
	}
}

func TestNGNV1StopInsideSingularVerificationLeavesParentContext(t *testing.T) {
	model := loadEvaluatorTensors(t, new(nnue.Tensors))
	searcher, err := NewSearchEngineWithHash(1)
	if err != nil {
		t.Fatal(err)
	}
	if err := searcher.SelectNGNV1Evaluator(model); err != nil {
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
	tt.Set(pos.Hash(), ttMove, scoreToTT(100, 0), int8(depth), Exact, false)

	injected := false
	evaluator.transitionObserver = func(_ *Position, _ *workerEvaluator) {
		if info.ExcludedMove != EmptyMove && !injected {
			injected = true
			info.control.RequestStop()
		}
	}
	t.Cleanup(info.control.ClearStop)
	score := alphaBetaPV(pos, depth, 0, -INFINITY, INFINITY, true, true, false, info)
	if !injected || info.SingularTries == 0 {
		t.Fatalf("singular stop witness injected=%v tries=%d", injected, info.SingularTries)
	}
	if !info.Stopped || score != stoppedSearchScore {
		t.Fatalf("singular stopped result score=%d stopped=%v", score, info.Stopped)
	}
	if evaluator.nnueDepth() != 0 {
		t.Fatalf("stopped singular verification left Context depth %d", evaluator.nnueDepth())
	}
	if after := snapshotPVPosition(pos); !reflect.DeepEqual(after, before) {
		t.Fatal("stopped singular verification did not leave parent position restored")
	}
	requireWorkerRawMatchesFull(t, evaluator, pos)
}

func runN3CIterativeSnapshot(t *testing.T, searcher *SearchEngine, fen string) (SearchInfo, []SearchInfo) {
	t.Helper()
	pos := n3cPosition(t, fen)
	before := snapshotPVPosition(pos)
	var callbacks []SearchInfo
	final := searcher.SearchIterativeDeepeningWithCallback(pos, 3, nil, func(info *SearchInfo) {
		callbacks = append(callbacks, comparableN3CSearchInfo(info))
	})
	if after := snapshotPVPosition(pos); !reflect.DeepEqual(after, before) {
		t.Fatal("iterative NNUE search changed root position")
	}
	if searcher.worker.evaluator.nnueDepth() != 0 {
		t.Fatalf("iterative NNUE search left Context depth %d", searcher.worker.evaluator.nnueDepth())
	}
	requireWorkerRawMatchesFull(t, searcher.worker.evaluator, pos)
	return comparableN3CSearchInfo(final), callbacks
}

func TestSearchNGNV1IterativeColdWarmMatchesFullRefreshCallbacks(t *testing.T) {
	model, tensors := adapterContextModel(t)
	const fen = "r3k2r/p1ppqpb1/bn2pnp1/2pP4/1p2P3/2N2N2/PPQBBPPP/R3K2R w KQkq - 37 1"

	incremental := NewSearchEngine()
	fullRefresh := NewSearchEngine()
	if err := incremental.SelectNGNV1Evaluator(model); err != nil {
		t.Fatal(err)
	}
	if err := fullRefresh.SelectNGNV1Evaluator(model); err != nil {
		t.Fatal(err)
	}
	observed := 0
	incremental.worker.evaluator.transitionObserver = func(pos *Position, evaluator *workerEvaluator) {
		observed++
		requireWorkerContextLanes(t, evaluator, tensors, fullPositionForWorkerTest(t, pos))
	}
	fullRefresh.worker.evaluator.fullRefreshOracle = true

	for _, lifetime := range []string{"cold", "warm"} {
		t.Run(lifetime, func(t *testing.T) {
			gotFinal, gotCallbacks := runN3CIterativeSnapshot(t, incremental, fen)
			wantFinal, wantCallbacks := runN3CIterativeSnapshot(t, fullRefresh, fen)
			if len(gotCallbacks) != 3 || len(wantCallbacks) != 3 {
				t.Fatalf("callback counts incremental=%d full-refresh=%d, want 3 completed depths", len(gotCallbacks), len(wantCallbacks))
			}
			if !reflect.DeepEqual(gotCallbacks, wantCallbacks) {
				t.Fatalf("iterative callback traces differ:\nincremental=%+v\nfull-refresh=%+v", gotCallbacks, wantCallbacks)
			}
			if !reflect.DeepEqual(gotFinal, wantFinal) {
				t.Fatalf("iterative final results differ:\nincremental=%+v\nfull-refresh=%+v", gotFinal, wantFinal)
			}
			if gotCallbacks[len(gotCallbacks)-1].PVLength != gotFinal.PVLength ||
				gotCallbacks[len(gotCallbacks)-1].PV != gotFinal.PV ||
				gotCallbacks[len(gotCallbacks)-1].BestMove != gotFinal.BestMove ||
				gotCallbacks[len(gotCallbacks)-1].BestScore != gotFinal.BestScore {
				t.Fatal("final iterative result does not preserve the completed callback PV/result")
			}
		})
	}
	if observed < 4 {
		t.Fatalf("incremental lane observer ran %d times, want roots and committed children across cold/warm searches", observed)
	}
}

func poisonN3CWorkerHistory(history *workerHistory, move Move) {
	history.historyTable[WhiteKing][A2] = 1
	history.continuationHistory[WhiteKing][A1][WhiteKing][A2] = 2
	history.followupHistory[WhiteKing][A1][WhiteKing][A2] = 3
	history.pawnCorrectionHistory[White][1] = 4
	history.nonPawnCorrectionHistory[Black][2] = 5
	history.minorCorrectionHistory[White][3] = 6
	history.captureHistory[WhiteKing][A2][BlackPawn] = 7
	history.killerMoves[0][0] = move
	history.counterMoves[A1][A2] = move
	history.lastMovePlayed = move
}

func TestDistinctNGNV1ModelAtoBtoAAdmissionsInvalidateTTAndCompleteHistory(t *testing.T) {
	modelA, tensorsA := adapterContextModel(t)
	tensorsB := *tensorsA
	tensorsB.OutputBias++
	modelB := loadEvaluatorTensors(t, &tensorsB)
	if modelA.Metadata() == modelB.Metadata() {
		t.Fatal("distinct fixture models have identical metadata")
	}

	searcher, err := NewSearchEngineWithHash(1)
	if err != nil {
		t.Fatal(err)
	}
	terminal := n3cPosition(t, "7k/6Q1/6K1/8/8/8/8/8 b - - 0 1")
	poisonMove := NewMove(A1, A2, WhiteKing, NoPiece, NoType, 0)

	if err := searcher.SelectNGNV1Evaluator(modelA); err != nil {
		t.Fatal(err)
	}
	searcher.SearchFixed(terminal.Copy(), 1, nil)
	identityA := searcher.worker.evaluatorIdentity

	poisonN3CWorkerHistory(&searcher.worker.history, poisonMove)
	const keyA = uint64(0x4e334141)
	searcher.TTStore(keyA, poisonMove, 17, 3, Exact, false)
	if err := searcher.SelectNGNV1Evaluator(modelB); err != nil {
		t.Fatal(err)
	}
	if searcher.worker.history == (workerHistory{}) {
		t.Fatal("model selection eagerly cleared history before admission")
	}
	searcher.SearchFixed(terminal.Copy(), 1, nil)
	if _, _, _, _, hit, _ := searcher.TTProbe(keyA); hit {
		t.Fatal("A-to-B admission retained model A TT entry")
	}
	if searcher.worker.history != (workerHistory{}) {
		t.Fatal("A-to-B admission did not clear the complete worker-history family")
	}
	identityB := searcher.worker.evaluatorIdentity
	if identityB == identityA {
		t.Fatal("A-to-B admission retained model A composite identity")
	}

	poisonN3CWorkerHistory(&searcher.worker.history, poisonMove)
	const keyB = uint64(0x4e334242)
	searcher.TTStore(keyB, poisonMove, 18, 3, Exact, false)
	if err := searcher.SelectNGNV1Evaluator(modelA); err != nil {
		t.Fatal(err)
	}
	searcher.SearchFixed(terminal.Copy(), 1, nil)
	if _, _, _, _, hit, _ := searcher.TTProbe(keyB); hit {
		t.Fatal("B-to-A admission retained model B TT entry")
	}
	if searcher.worker.history != (workerHistory{}) {
		t.Fatal("B-to-A admission did not clear the complete worker-history family")
	}
	if searcher.worker.evaluatorIdentity != identityA {
		t.Fatalf("B-to-A identity = %+v, original A = %+v", searcher.worker.evaluatorIdentity, identityA)
	}
}

func TestNGNV1SingularVerificationRemakesAndRepushesNormally(t *testing.T) {
	model := loadEvaluatorTensors(t, new(nnue.Tensors))
	searcher, err := NewSearchEngineWithHash(1)
	if err != nil {
		t.Fatal(err)
	}
	if err := searcher.SelectNGNV1Evaluator(model); err != nil {
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
	tt.Set(pos.Hash(), ttMove, scoreToTT(100, 0), int8(depth), Exact, false)

	sawVerification := false
	sawRepush := false
	evaluator.transitionObserver = func(observedPos *Position, _ *workerEvaluator) {
		if info.ExcludedMove != EmptyMove {
			sawVerification = true
		}
		if info.SingularTries > 0 && info.ExcludedMove == EmptyMove && observedPos.Hash() == childHash {
			sawRepush = true
		}
	}
	score := alphaBetaPV(pos, depth, 0, -INFINITY, INFINITY, true, true, false, info)
	if info.Stopped || score == stoppedSearchScore {
		t.Fatalf("normal singular witness stopped score=%d stopped=%v", score, info.Stopped)
	}
	if info.SingularTries == 0 || !sawVerification || !sawRepush {
		t.Fatalf("singular completion tries=%d verification=%v repush=%v", info.SingularTries, sawVerification, sawRepush)
	}
	if evaluator.nnueDepth() != 0 {
		t.Fatalf("completed singular verification left Context depth %d", evaluator.nnueDepth())
	}
	if after := snapshotPVPosition(pos); !reflect.DeepEqual(after, before) {
		t.Fatal("completed singular verification did not restore root position")
	}
	requireWorkerRawMatchesFull(t, evaluator, pos)
}
