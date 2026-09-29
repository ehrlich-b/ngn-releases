package engine

import "testing"

func TestHistoryLearnsCompletedRootWinner(t *testing.T) {
	searcher, err := NewSearchEngineWithHash(1)
	if err != nil {
		t.Fatal(err)
	}
	pos, err := ParseFEN("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	searcher.NewGame()
	info := searcher.Search(pos, 1)
	if info.Stopped || !h1QuietMove(info.BestMove) {
		t.Fatalf("depth-one root did not complete with quiet winner: stopped=%v best=%s", info.Stopped, info.BestMove.ToString())
	}
	if got := searcher.worker.history.GetHistoryScore(info.BestMove, EmptyMove, EmptyMove); got <= 0 {
		t.Fatalf("completed root winner was not rewarded: %d", got)
	}
}
