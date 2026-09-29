//go:build rodentv12oracle

package engine

import (
	"bytes"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/ehrlich-b/ngn/rodentv12eval"
)

func loadRodentV12EngineModel(t *testing.T) (*rodentv12eval.Model, string) {
	t.Helper()
	path := os.Getenv("RODENT_V12_DEFAULT_MODEL")
	if path == "" {
		t.Fatal("RODENT_V12_DEFAULT_MODEL is required")
	}
	model, err := rodentv12eval.LoadV12Default(path)
	if err != nil {
		t.Fatal(err)
	}
	return model, path
}

func requireRodentV12WorkerMatchesFull(t *testing.T, worker *workerEvaluator, pos *Position) int {
	t.Helper()
	if worker == nil || worker.rodentV12Context == nil || worker.model.rodentV12Default == nil {
		t.Fatal("worker has no Rodent V1.2 model/context")
	}
	wantPosition, err := rodentV12PositionFromPosition(pos)
	if err != nil {
		t.Fatal(err)
	}
	if got := worker.rodentV12Context.Position(); got != wantPosition {
		t.Fatalf("Rodent V1.2 context position = %+v, want %+v", got, wantPosition)
	}
	incremental, err := worker.rodentV12Context.EvaluateReleaseStatic()
	if err != nil {
		t.Fatal(err)
	}
	full, err := worker.model.rodentV12Default.EvaluateReleaseStatic(wantPosition)
	if err != nil {
		t.Fatal(err)
	}
	if incremental != full {
		t.Fatalf("Rodent V1.2 incremental static = %d, full refresh = %d", incremental, full)
	}
	if got := worker.SearchSTM(pos); got != rodentSearchScore(full) {
		t.Fatalf("Rodent V1.2 SearchSTM = %d, want %d", got, rodentSearchScore(full))
	}
	return full
}

func TestRodentV12SearchIncrementalMatchesFullRefreshAndUnwinds(t *testing.T) {
	model, _ := loadRodentV12EngineModel(t)
	const fen = "r3k2r/p1ppqpb1/bn2pnp1/2pP4/1p2P3/2N2N2/PPQBBPPP/R3K2R w KQkq - 37 1"

	incremental := NewSearchEngine()
	fullRefresh := NewSearchEngine()
	if err := incremental.SelectRodentV12DefaultEvaluator(model); err != nil {
		t.Fatal(err)
	}
	if err := fullRefresh.SelectRodentV12DefaultEvaluator(model); err != nil {
		t.Fatal(err)
	}
	observed := 0
	incremental.worker.evaluator.transitionObserver = func(pos *Position, evaluator *workerEvaluator) {
		observed++
		requireRodentV12WorkerMatchesFull(t, evaluator, pos)
	}
	fullRefresh.worker.evaluator.fullRefreshOracle = true

	incrementalPos := n3cPosition(t, fen)
	fullRefreshPos := n3cPosition(t, fen)
	incrementalBefore := snapshotPVPosition(incrementalPos)
	fullRefreshBefore := snapshotPVPosition(fullRefreshPos)
	got := incremental.SearchFixed(incrementalPos, 3, nil)
	want := fullRefresh.SearchFixed(fullRefreshPos, 3, nil)
	if observed < 2 {
		t.Fatalf("Rodent V1.2 observer calls = %d, want root and child frames", observed)
	}
	if gotComparable, wantComparable := comparableN3CSearchInfo(got), comparableN3CSearchInfo(want); !reflect.DeepEqual(gotComparable, wantComparable) {
		t.Fatalf("Rodent V1.2 incremental search differs from full refresh:\nincremental=%+v\nfull-refresh=%+v", gotComparable, wantComparable)
	}
	if !reflect.DeepEqual(snapshotPVPosition(incrementalPos), incrementalBefore) ||
		!reflect.DeepEqual(snapshotPVPosition(fullRefreshPos), fullRefreshBefore) {
		t.Fatal("Rodent V1.2 search changed a root position")
	}
	if incremental.worker.evaluator.nnueDepth() != 0 || fullRefresh.worker.evaluator.nnueDepth() != 0 {
		t.Fatal("Rodent V1.2 search did not unwind evaluator contexts")
	}
	requireRodentV12WorkerMatchesFull(t, incremental.worker.evaluator, incrementalPos)
}

func TestRodentV12StoppedSearchUnwindsContextAndPosition(t *testing.T) {
	model, _ := loadRodentV12EngineModel(t)
	searcher := NewSearchEngine()
	if err := searcher.SelectRodentV12DefaultEvaluator(model); err != nil {
		t.Fatal(err)
	}
	pos := n3cPosition(t, "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1")
	before := snapshotPVPosition(pos)
	rootPosition, err := rodentV12PositionFromPosition(pos)
	if err != nil {
		t.Fatal(err)
	}
	rootRaw, err := model.EvaluateReleaseStatic(rootPosition)
	if err != nil {
		t.Fatal(err)
	}
	observed := 0
	changedFromRoot := false
	searcher.worker.evaluator.transitionObserver = func(pos *Position, evaluator *workerEvaluator) {
		observed++
		raw := requireRodentV12WorkerMatchesFull(t, evaluator, pos)
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
		t.Fatal("stopped Rodent V1.2 search did not restore context and position")
	}
	requireRodentV12WorkerMatchesFull(t, searcher.worker.evaluator, pos)
}

func TestRodentV12ExplicitMoveNullPopAndClockIdentity(t *testing.T) {
	model, _ := loadRodentV12EngineModel(t)
	searcher := NewSearchEngine()
	if err := searcher.SelectRodentV12DefaultEvaluator(model); err != nil {
		t.Fatal(err)
	}
	pos := n3cPosition(t, "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1")
	before := snapshotPVPosition(pos)
	if _, _, err := searcher.EvaluateSelected(pos); err != nil {
		t.Fatal(err)
	}
	worker := searcher.worker.evaluator
	rootScore := requireRodentV12WorkerMatchesFull(t, worker, pos)

	clockVariant := pos.Copy()
	clockVariant.HalfMoveClock = 99
	if _, _, err := searcher.EvaluateSelected(clockVariant); err != nil {
		t.Fatal(err)
	}
	if got := requireRodentV12WorkerMatchesFull(t, searcher.worker.evaluator, clockVariant); got != rootScore {
		t.Fatalf("Rodent V1.2 release static changed with halfmove clock: %d -> %d", rootScore, got)
	}
	if _, _, err := searcher.EvaluateSelected(pos); err != nil {
		t.Fatal(err)
	}
	worker = searcher.worker.evaluator

	move := adapterLegalMove(t, pos, "e2e4")
	moveTransition, err := worker.PrepareMove(pos, move)
	if err != nil {
		t.Fatal(err)
	}
	undoEP, undoTag, undoClock, _ := pos.MakeMove(move)
	if err := worker.PushMove(pos, moveTransition); err != nil {
		t.Fatal(err)
	}
	requireRodentV12WorkerMatchesFull(t, worker, pos)

	nullTransition, err := worker.PrepareNull(pos)
	if err != nil {
		t.Fatal(err)
	}
	nullEP := pos.MakeNullMove()
	if err := worker.PushNull(pos, nullTransition); err != nil {
		t.Fatal(err)
	}
	requireRodentV12WorkerMatchesFull(t, worker, pos)
	if err := worker.Pop(); err != nil {
		t.Fatal(err)
	}
	pos.UnMakeNullMove(nullEP)
	requireRodentV12WorkerMatchesFull(t, worker, pos)
	if err := worker.Pop(); err != nil {
		t.Fatal(err)
	}
	pos.UnMakeMove(move, undoTag, undoEP, undoClock)
	if !reflect.DeepEqual(snapshotPVPosition(pos), before) || worker.nnueDepth() != 0 {
		t.Fatal("Rodent V1.2 move/null/pop did not restore root")
	}
	requireRodentV12WorkerMatchesFull(t, worker, pos)
}

func TestRodentV12SMPContextsArePrivateResetAndUnwound(t *testing.T) {
	model, _ := loadRodentV12EngineModel(t)
	searcher := NewSearchEngine()
	if err := searcher.SelectRodentV12DefaultEvaluator(model); err != nil {
		t.Fatal(err)
	}
	if err := searcher.ConfigureThreads(3); err != nil {
		t.Fatal(err)
	}
	workers := searcher.configuredWorkers()
	for i, worker := range workers {
		if worker.evaluator == nil || worker.evaluator.rodentV12Context == nil || worker.evaluator.model.rodentV12Default != model {
			t.Fatalf("Rodent V1.2 worker %d missing model/context", i)
		}
		for prior := 0; prior < i; prior++ {
			if worker.evaluator == workers[prior].evaluator || worker.evaluator.rodentV12Context == workers[prior].evaluator.rodentV12Context {
				t.Fatalf("Rodent V1.2 workers %d and %d share mutable state", i, prior)
			}
		}
	}
	pos := n3cPosition(t, "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1")
	wantRoot, err := rodentV12PositionFromPosition(pos)
	if err != nil {
		t.Fatal(err)
	}
	reset := make([]bool, len(workers))
	searcher.smpHooks = &smpSearchHooks{afterReset: func(index int) {
		reset[index] = true
		if got := workers[index].evaluator.rodentV12Context.Position(); got != wantRoot {
			t.Fatalf("Rodent V1.2 worker %d reset position = %+v, want %+v", index, got, wantRoot)
		}
	}}
	searcher.SearchIterativeDeepening(pos, 2, nil)
	searcher.smpHooks = nil
	for i, worker := range workers {
		if !reset[i] || worker.evaluator.nnueDepth() != 0 {
			t.Fatalf("Rodent V1.2 worker %d reset=%v depth=%d", i, reset[i], worker.evaluator.nnueDepth())
		}
	}
}

func TestRodentV12SelectionAndStateInvalidationAreTransactional(t *testing.T) {
	model, _ := loadRodentV12EngineModel(t)
	searcher, err := NewSearchEngineWithHash(1)
	if err != nil {
		t.Fatal(err)
	}
	terminal := n3cPosition(t, "7k/6Q1/6K1/8/8/8/8/8 b - - 0 1")
	searcher.SearchFixed(terminal.Copy(), 1, nil)
	beforeModel := searcher.evaluatorModel
	beforeWorker := searcher.worker.evaluator
	beforeTTIdentity := searcher.ttIdentity
	if err := searcher.SelectRodentV12DefaultEvaluator(nil); err == nil {
		t.Fatal("nil Rodent V1.2 model selected")
	}
	if searcher.evaluatorModel != beforeModel || searcher.worker.evaluator != beforeWorker || searcher.ttIdentity != beforeTTIdentity {
		t.Fatal("failed Rodent V1.2 selection changed published state")
	}

	poisonMove := NewMove(A1, A2, WhiteKing, NoPiece, NoType, 0)
	poisonN3CWorkerHistory(&searcher.worker.history, poisonMove)
	const hceKey = uint64(0x12a11ad)
	searcher.TTStore(hceKey, poisonMove, 17, 3, Exact, false)
	if err := searcher.SelectRodentV12DefaultEvaluator(model); err != nil {
		t.Fatal(err)
	}
	if _, _, _, _, hit, _ := searcher.TTProbe(hceKey); hit {
		t.Fatal("Rodent V1.2 selection retained HCE TT entry")
	}
	searcher.SearchFixed(terminal.Copy(), 1, nil)
	if searcher.worker.history != (workerHistory{}) || searcher.worker.evaluatorIdentity.backend != evaluatorBackendRodentV12Default {
		t.Fatal("Rodent V1.2 selection retained prior history or wrong identity")
	}

	identity := searcher.worker.evaluatorIdentity
	poisonN3CWorkerHistory(&searcher.worker.history, poisonMove)
	warmHistory := searcher.worker.history
	const rodentKey = uint64(0x12a11ae)
	searcher.TTStore(rodentKey, poisonMove, 18, 3, Exact, false)
	if err := searcher.SelectRodentV12DefaultEvaluator(model); err != nil {
		t.Fatal(err)
	}
	searcher.SearchFixed(terminal.Copy(), 1, nil)
	if searcher.worker.evaluatorIdentity != identity || searcher.worker.history != warmHistory {
		t.Fatal("same exact Rodent V1.2 identity discarded warm history")
	}
	if _, _, _, _, hit, _ := searcher.TTProbe(rodentKey); !hit {
		t.Fatal("same exact Rodent V1.2 identity discarded warm TT")
	}
}

func TestUCIRodentV12HeaderlessSelectionAndFailureTransaction(t *testing.T) {
	model, path := loadRodentV12EngineModel(t)
	staged, err := loadUCIEvaluatorModel(path)
	if err != nil {
		t.Fatal(err)
	}
	if staged.kind != stagedEvaluatorRodentV12Default || staged.rodentV12Default == nil {
		t.Fatalf("exact Rodent V1.2 artifact staged as %+v", staged)
	}

	uci := NewUCIEngine()
	var output bytes.Buffer
	uci.handleCommand("setoption name EvalFile value "+path, &output)
	uci.handleCommand("setoption name EvalBackend value rodent-v1.2-default", &output)
	if strings.Contains(output.String(), "info string error") || uci.searcher.SelectedEvaluatorBackend() != EvaluatorBackendRodentV12DefaultName {
		t.Fatalf("Rodent V1.2 UCI selection failed: %s", output.String())
	}
	config := requireUCIEvaluatorConfig(t, uci, uciEvaluatorRodentV12Default, path)
	identity := uci.searcher.evaluatorModel.identity
	position := n3cPosition(t, "4k3/8/8/8/8/8/8/4K3 w - - 99 1")
	wantPosition, err := rodentV12PositionFromPosition(position)
	if err != nil {
		t.Fatal(err)
	}
	want, err := model.EvaluateReleaseStatic(wantPosition)
	if err != nil {
		t.Fatal(err)
	}
	if got, backend, err := uci.searcher.EvaluateSelected(position); err != nil || backend != EvaluatorBackendRodentV12DefaultName || got != rodentSearchScore(want) {
		t.Fatalf("Rodent V1.2 selected eval = %d/%s/%v, want %d/%s", got, backend, err, rodentSearchScore(want), EvaluatorBackendRodentV12DefaultName)
	}
	output.Reset()
	uci.position = position.Copy()
	uci.handleCommand("eval", &output)
	wantDiagnostic := fmt.Sprintf("info string eval backend %s score_cp %d pov side-to-move policy base-SearchSTM rule50-and-backend-adapter correction-history excluded", EvaluatorBackendRodentV12DefaultName, rodentSearchScore(want))
	if got := strings.TrimSpace(output.String()); got != wantDiagnostic {
		t.Fatalf("Rodent V1.2 diagnostic = %q, want %q", got, wantDiagnostic)
	}
	poisonMove := NewMove(A1, A2, WhiteKing, NoPiece, NoType, 0)
	poisonN3CWorkerHistory(&uci.searcher.worker.history, poisonMove)
	warmHistory := uci.searcher.worker.history
	warmWorker := uci.searcher.worker.evaluator
	const rodentKey = uint64(0x12a11af)
	uci.searcher.TTStore(rodentKey, poisonMove, 19, 3, Exact, false)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data[len(data)/2] ^= 1
	corrupt := t.TempDir() + "/corrupt-v12.nn"
	if err := os.WriteFile(corrupt, data, 0o600); err != nil {
		t.Fatal(err)
	}
	output.Reset()
	uci.handleCommand("setoption name EvalFile value "+corrupt, &output)
	if !strings.Contains(output.String(), "info string error eval option") {
		t.Fatalf("corrupt Rodent V1.2 replacement was accepted: %s", output.String())
	}
	after := requireUCIEvaluatorConfig(t, uci, uciEvaluatorRodentV12Default, path)
	if after.staged.rodentV12Default != config.staged.rodentV12Default || uci.searcher.evaluatorModel.identity != identity {
		t.Fatal("failed Rodent V1.2 replacement changed staged or selected model")
	}
	if uci.searcher.worker.evaluator != warmWorker || uci.searcher.worker.history != warmHistory {
		t.Fatal("failed Rodent V1.2 replacement changed worker or history")
	}
	if _, _, _, _, hit, _ := uci.searcher.TTProbe(rodentKey); !hit {
		t.Fatal("failed Rodent V1.2 replacement discarded TT state")
	}

	startup := NewUCIEngine()
	if err := startup.ConfigureStartupEvaluator(uciEvaluatorRodentV12Default, path); err != nil {
		t.Fatal(err)
	}
	output.Reset()
	startup.handleCommand("uci", &output)
	if !strings.Contains(output.String(), "option name EvalBackend type combo default rodent-v1.2-default var hce var ngn-v1 var ngn-k4-768-v1 var sf18-big var counter-5.5 var rodent-v1.1-anand var rodent-v1.2-default") {
		t.Fatalf("Rodent V1.2 startup defaults not advertised: %s", output.String())
	}
	if got, backend, err := startup.searcher.EvaluateSelected(position); err != nil || backend != EvaluatorBackendRodentV12DefaultName || got != rodentSearchScore(want) {
		t.Fatalf("Rodent V1.2 startup eval = %d/%s/%v, want %d/%s", got, backend, err, rodentSearchScore(want), EvaluatorBackendRodentV12DefaultName)
	}
}
