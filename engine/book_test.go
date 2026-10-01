package engine

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

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

func TestPolyglotHash(t *testing.T) {
	pos, err := ParseFEN("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	if got := PolyglotHash(pos); got != 0x463B96181691FC9C {
		t.Fatalf("start-position Polyglot key = %016x, want 463b96181691fc9c", got)
	}
}

func TestExternalPolyglotBookInterop(t *testing.T) {
	// One independently specified standard Polyglot entry, in its native
	// big-endian 16-byte format: startpos key, e2e4 (0x031c), weight 100,
	// learn 0. The key is intentionally literal, not produced by NGN.
	entry := []byte{
		0x46, 0x3b, 0x96, 0x18, 0x16, 0x91, 0xfc, 0x9c,
		0x03, 0x1c, 0x00, 0x64, 0x00, 0x00, 0x00, 0x00,
	}
	path := filepath.Join(t.TempDir(), "standard-start.bin")
	if err := os.WriteFile(path, entry, 0o600); err != nil {
		t.Fatal(err)
	}
	book, err := LoadPolyglotBook(path)
	if err != nil {
		t.Fatal(err)
	}
	if book.Size() != 1 {
		t.Fatalf("book size = %d, want 1", book.Size())
	}
	start, err := ParseFEN("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	beforeHash := start.Hash()
	beforeCounts := cloneBookPositionCounts(start.Positions)
	move, found := book.ProbeBook(start)
	if !found || move.ToString() != "e2e4" || !IsLegalMove(start, move) {
		t.Fatalf("standard external book probe found=%v move=%s legal=%v, want legal e2e4",
			found, move.ToString(), found && IsLegalMove(start, move))
	}
	if start.Hash() != beforeHash || !reflect.DeepEqual(start.Positions, beforeCounts) {
		t.Fatal("external book probe mutated ordinary hash or occurrence counts")
	}
	afterE4, err := ParseFEN("rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq e3 0 1")
	if err != nil {
		t.Fatal(err)
	}
	if move, found := book.ProbeBook(afterE4); found || move != EmptyMove {
		t.Fatalf("different-position probe found=%v move=%s, want miss", found, move.ToString())
	}
}

func cloneBookPositionCounts(counts map[uint64]int) map[uint64]int {
	copyCounts := make(map[uint64]int, len(counts))
	for key, count := range counts {
		copyCounts[key] = count
	}
	return copyCounts
}
