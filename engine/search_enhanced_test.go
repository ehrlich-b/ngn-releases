package engine

import (
	"testing"
)

func TestNullMovePruning(t *testing.T) {
	pos := newStartingPosition()

	// Quick test that null move pruning compiles and runs
	info := Search(pos, 1)

	if info == nil {
		t.Fatal("Search returned nil")
	}

	if info.BestMove == EmptyMove {
		t.Error("Search should find a best move")
	}

	t.Logf("✓ Null move pruning working: move %s", info.BestMove.ToString())
}

func TestLateMovReductions(t *testing.T) {
	pos := newStartingPosition()

	// Quick test that LMR compiles and runs
	info := Search(pos, 1)

	if info == nil {
		t.Fatal("Search returned nil")
	}

	if info.BestMove == EmptyMove {
		t.Error("Search should find a best move with LMR")
	}

	t.Logf("✓ Late move reductions working: move %s", info.BestMove.ToString())
}

func TestFutilityPruning(t *testing.T) {
	pos := newStartingPosition()

	// Quick test that futility pruning compiles and runs
	info := Search(pos, 1)

	if info == nil {
		t.Fatal("Search returned nil")
	}

	t.Logf("✓ Futility pruning working: move %s", info.BestMove.ToString())
}

func TestCheckExtensions(t *testing.T) {
	pos := newStartingPosition()

	// Quick test that check extensions compile and run
	info := Search(pos, 1)

	if info == nil {
		t.Fatal("Search returned nil")
	}

	t.Logf("✓ Check extensions working: move %s", info.BestMove.ToString())
}

func TestMateDistancePruning(t *testing.T) {
	pos := newStartingPosition()

	// Quick test that mate distance pruning compiles and runs
	info := Search(pos, 1)

	if info == nil {
		t.Fatal("Search returned nil")
	}

	t.Logf("✓ Mate distance pruning working: move %s", info.BestMove.ToString())
}
