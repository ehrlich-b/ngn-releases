package engine

import "testing"

func TestPrimaryWorkerOwnsAndPersistsOneThreadSearchState(t *testing.T) {
	searcher, err := NewSearchEngineWithHash(1)
	if err != nil {
		t.Fatal(err)
	}
	pos, err := ParseFEN("7k/8/8/8/3Q4/8/8/K7 w - - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	seeded := NewMove(A1, A2, WhiteKing, NoPiece, NoType, 0)
	searcher.worker.history.lastMovePlayed = seeded

	callbacks := 0
	first := searcher.SearchIterativeDeepeningWithCallback(pos, 1, nil, func(info *SearchInfo) {
		callbacks++
		if info.control != &searcher.worker.control {
			t.Fatal("one-thread search did not use its persistent worker control")
		}
		if info.history != &searcher.worker.history {
			t.Fatal("one-thread search did not use its persistent worker history")
		}
		if info.evaluator != searcher.worker.evaluator {
			t.Fatal("one-thread search did not use its persistent worker evaluator")
		}
	})
	if callbacks == 0 {
		t.Fatal("one-thread search completed no callback")
	}
	if first.control != &searcher.worker.control || first.history != &searcher.worker.history || first.evaluator != searcher.worker.evaluator {
		t.Fatal("final SearchInfo did not retain the primary worker state owners")
	}
	if searcher.worker.history.lastMovePlayed != seeded {
		t.Fatal("first default-HCE admission discarded legacy seeded history")
	}

	firstEvaluator := searcher.worker.evaluator
	firstHCE := searcher.worker.hce
	firstTT := searcher.tt
	searcher.SearchIterativeDeepening(pos.Copy(), 1, nil)
	if searcher.worker.evaluator != firstEvaluator || searcher.worker.hce != firstHCE {
		t.Fatal("same-identity search replaced persistent worker evaluator state")
	}
	if searcher.tt != firstTT {
		t.Fatal("same-identity search replaced SearchEngine-owned TT")
	}

	other, err := NewSearchEngineWithHash(1)
	if err != nil {
		t.Fatal(err)
	}
	if &other.worker.control == &searcher.worker.control || &other.worker.history == &searcher.worker.history {
		t.Fatal("independent SearchEngines aliased mutable worker state")
	}
	if other.worker.evaluator != nil || other.worker.hce != nil || other.worker.history != (workerHistory{}) {
		t.Fatal("fresh SearchEngine inherited another worker's mutable state")
	}
}
