package engine

import "testing"

func TestEmbeddedBookStartingPosition(t *testing.T) {
	pos, _ := ParseFEN("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1")

	move, found := ProbeEmbeddedBook(pos)

	if !found {
		t.Fatal("Expected to find a book move for starting position")
	}

	// Should be e2e4 (highest weighted)
	expected := "e2e4"
	if move.ToString() != expected {
		t.Errorf("Expected %s, got %s", expected, move.ToString())
	}

	t.Logf("Starting position book move: %s", move.ToString())
}

func TestEmbeddedBookAfterE4(t *testing.T) {
	pos, _ := ParseFEN("rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq e3 0 1")

	move, found := ProbeEmbeddedBook(pos)

	if !found {
		t.Fatal("Expected to find a book move after 1.e4")
	}

	// Should be e7e5 (highest weighted)
	expected := "e7e5"
	if move.ToString() != expected {
		t.Errorf("Expected %s, got %s", expected, move.ToString())
	}

	t.Logf("After 1.e4 book move: %s", move.ToString())
}

func TestEmbeddedBookAfterD4(t *testing.T) {
	pos, _ := ParseFEN("rnbqkbnr/pppppppp/8/8/3P4/8/PPP1PPPP/RNBQKBNR b KQkq d3 0 1")

	move, found := ProbeEmbeddedBook(pos)

	if !found {
		t.Fatal("Expected to find a book move after 1.d4")
	}

	// Should be d7d5 (highest weighted)
	expected := "d7d5"
	if move.ToString() != expected {
		t.Errorf("Expected %s, got %s", expected, move.ToString())
	}

	t.Logf("After 1.d4 book move: %s", move.ToString())
}

func TestEmbeddedBookNoHit(t *testing.T) {
	// Random middlegame position - should not be in book
	pos, _ := ParseFEN("r1bqkb1r/pppp1ppp/2n2n2/4p3/2B1P3/5N2/PPPP1PPP/RNBQK2R w KQkq - 4 4")

	_, found := ProbeEmbeddedBook(pos)

	if found {
		t.Error("Did not expect to find a book move for random middlegame")
	}

	t.Log("Correctly returned no book move for middlegame position")
}
