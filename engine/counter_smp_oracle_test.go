//go:build counteroracle

package engine

import (
	"bytes"
	"encoding/binary"
	"os"
	"testing"

	"github.com/ehrlich-b/ngn/countereval"
)

func TestLazySMPCounterAcceptedModelThreads8IdentityAndReset(t *testing.T) {
	modelPath := requireCounterOracleEnv(t, "COUNTER_MODEL")
	encoded, err := os.ReadFile(modelPath)
	if err != nil {
		t.Fatal(err)
	}
	modelA, metadataA, err := countereval.LoadCounter55Legacy(bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	if metadataA.SHA256 != acceptedCounter55SHA256 || metadataA.Bytes != countereval.LegacyFileSize {
		t.Fatalf("accepted Counter identity=%+v", metadataA)
	}

	searcher, workers, evaluatorsA, contextsA := requireLazySMPCounterContexts(t, modelA, 8)
	if len(workers) != 8 {
		t.Fatalf("Counter configured workers=%d want=8", len(workers))
	}

	// Change only a copied output-bias bit. The strict loader creates a distinct,
	// immutable model identity without mutating the accepted input or its workers.
	replacementBytes := append([]byte(nil), encoded...)
	biasOffset := len(replacementBytes) - 4
	binary.LittleEndian.PutUint32(replacementBytes[biasOffset:], binary.LittleEndian.Uint32(replacementBytes[biasOffset:])^1)
	modelB, metadataB, err := countereval.LoadCounter55Legacy(bytes.NewReader(replacementBytes))
	if err != nil {
		t.Fatal(err)
	}
	if metadataB.SHA256 == metadataA.SHA256 {
		t.Fatal("Counter replacement retained accepted model identity")
	}

	poisonMove := NewMove(A1, A2, WhiteKing, NoPiece, NoType, 0)
	for _, worker := range workers {
		poisonN3CWorkerHistory(&worker.history, poisonMove)
	}
	const staleKey = uint64(0xc0558008)
	searcher.TTStore(staleKey, poisonMove, 19, 4, Exact, false)
	if err := searcher.SelectCounter55Evaluator(modelB); err != nil {
		t.Fatal(err)
	}
	terminal := mustParseCounterPosition(t, "7k/6Q1/6K1/8/8/8/8/8 b - - 0 1")
	if result := searcher.Search(terminal, 1); result.EffectiveThreads != 8 {
		t.Fatalf("Counter replacement effective=%d want=8", result.EffectiveThreads)
	}
	for index, worker := range workers {
		if worker.evaluator == evaluatorsA[index] || worker.evaluator.counterContext == nil ||
			worker.evaluator.counterContext == contextsA[index] || worker.evaluator.Identity().counterMetadata.SHA256 != metadataB.SHA256 {
			t.Fatalf("Counter model replacement left stale worker %d", index)
		}
		if worker.history != (workerHistory{}) || worker.evaluator.nnueDepth() != 0 {
			t.Fatalf("Counter model replacement retained history/frame for worker %d", index)
		}
		requireCounterRawMatchesFull(t, worker.evaluator, terminal)
	}
	if _, _, _, _, hit, _ := searcher.TTProbe(staleKey); hit {
		t.Fatal("Counter model replacement retained stale TT entry")
	}

	contextsB := make([]*countereval.SearchContext, len(workers))
	for index, worker := range workers {
		contextsB[index] = worker.evaluator.counterContext
	}
	searcher.TTStore(staleKey, poisonMove, 23, 5, Exact, false)
	if err := searcher.SelectHCEEvaluator(); err != nil {
		t.Fatal(err)
	}
	if result := searcher.Search(terminal.Copy(), 1); result.EffectiveThreads != 8 {
		t.Fatalf("HCE replacement effective=%d want=8", result.EffectiveThreads)
	}
	for index, worker := range workers {
		if worker.evaluator.Identity().backend != evaluatorBackendHCE || worker.evaluator.counterContext != nil {
			t.Fatalf("backend replacement left Counter state in worker %d", index)
		}
	}
	if _, _, _, _, hit, _ := searcher.TTProbe(staleKey); hit {
		t.Fatal("backend replacement retained stale TT entry")
	}

	if err := searcher.SelectCounter55Evaluator(modelA); err != nil {
		t.Fatal(err)
	}
	if result := searcher.Search(terminal.Copy(), 1); result.EffectiveThreads != 8 {
		t.Fatalf("accepted Counter reselection effective=%d want=8", result.EffectiveThreads)
	}
	for index, worker := range workers {
		if worker.evaluator.Identity().counterMetadata.SHA256 != acceptedCounter55SHA256 ||
			worker.evaluator.counterContext == nil || worker.evaluator.counterContext == contextsA[index] ||
			worker.evaluator.counterContext == contextsB[index] || worker.evaluator.nnueDepth() != 0 {
			t.Fatalf("accepted Counter reselection left stale worker %d", index)
		}
		requireCounterRawMatchesFull(t, worker.evaluator, terminal)
	}
}
