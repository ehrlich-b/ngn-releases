package main

import (
	"testing"

	"github.com/ehrlich-b/ngn/engine"
)

func TestStaticStrataMatchK4PieceAndKingContract(t *testing.T) {
	position, err := engine.ParseFEN("r3k2r/8/8/8/8/8/8/R3K2R b - - 0 12")
	if err != nil {
		t.Fatal(err)
	}
	if got := pieceCount(position); got != 6 {
		t.Fatalf("piece count = %d, want 6", got)
	}
	if got := outputHead(position); got != 1 {
		t.Fatalf("output head = %d, want 1", got)
	}
	if got := kingBucket(4, false); got != 0 {
		t.Fatalf("white e1 bucket = %d, want 0", got)
	}
	if got := kingBucket(60, true); got != 0 {
		t.Fatalf("black e8 bucket = %d, want 0", got)
	}
	if got, err := sourcePly("r3k2r/8/8/8/8/8/8/R3K2R b - - 17 12"); err != nil || got != 23 {
		t.Fatalf("source ply = %d, %v; want 23", got, err)
	}
}
