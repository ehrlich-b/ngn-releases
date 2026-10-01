package engine

import "testing"

func oracleSlider(sq int, occupied uint64, directions [][2]int) uint64 {
	if sq < 0 || sq >= 64 {
		return 0
	}
	file, rank := sq&7, sq>>3
	var attacks uint64
	for _, direction := range directions {
		for nextFile, nextRank := file+direction[0], rank+direction[1]; nextFile >= 0 && nextFile < 8 && nextRank >= 0 && nextRank < 8; nextFile, nextRank = nextFile+direction[0], nextRank+direction[1] {
			next := nextRank*8 + nextFile
			mask := uint64(1) << uint(next)
			attacks |= mask
			if occupied&mask != 0 {
				break
			}
		}
	}
	return attacks
}

var oracleRookDirections = [][2]int{{0, 1}, {0, -1}, {1, 0}, {-1, 0}}
var oracleBishopDirections = [][2]int{{1, 1}, {-1, 1}, {1, -1}, {-1, -1}}
var oracleQueenDirections = [][2]int{{0, 1}, {0, -1}, {1, 0}, {-1, 0}, {1, 1}, {-1, 1}, {1, -1}, {-1, -1}}

func TestSliderAttacksExhaustiveBlockers(t *testing.T) {
	for sq := 0; sq < 64; sq++ {
		for blocker := 0; blocker < 64; blocker++ {
			occupied := uint64(1) << uint(blocker)
			wantRook := oracleSlider(sq, occupied, oracleRookDirections)
			wantBishop := oracleSlider(sq, occupied, oracleBishopDirections)
			wantQueen := oracleSlider(sq, occupied, oracleQueenDirections)
			if got := GetRookAttacks(sq, occupied); got != wantRook {
				t.Fatalf("rook sq=%d blocker=%d: got %016x want %016x", sq, blocker, got, wantRook)
			}
			if got := getRookAttacksBB(sq, occupied); got != wantRook {
				t.Fatalf("rook BB sq=%d blocker=%d: got %016x want %016x", sq, blocker, got, wantRook)
			}
			if got := GetBishopAttacks(sq, occupied); got != wantBishop {
				t.Fatalf("bishop sq=%d blocker=%d: got %016x want %016x", sq, blocker, got, wantBishop)
			}
			if got := getBishopAttacksBB(sq, occupied); got != wantBishop {
				t.Fatalf("bishop BB sq=%d blocker=%d: got %016x want %016x", sq, blocker, got, wantBishop)
			}
			if got := GetQueenAttacks(sq, occupied); got != wantQueen {
				t.Fatalf("queen sq=%d blocker=%d: got %016x want %016x", sq, blocker, got, wantQueen)
			}
		}

		// Occupancy on the origin is explicitly ignored.
		emptyRook := GetRookAttacks(sq, 0)
		if got := GetRookAttacks(sq, uint64(1)<<uint(sq)); got != emptyRook {
			t.Fatalf("origin occupancy changed rook attacks at %d", sq)
		}
		emptyBishop := GetBishopAttacks(sq, 0)
		if got := GetBishopAttacks(sq, uint64(1)<<uint(sq)); got != emptyBishop {
			t.Fatalf("origin occupancy changed bishop attacks at %d", sq)
		}
	}

	if GetRookAttacks(-1, ^uint64(0)) != 0 || GetRookAttacks(64, ^uint64(0)) != 0 {
		t.Fatal("invalid rook origin was not empty")
	}
	if GetBishopAttacks(-1, ^uint64(0)) != 0 || GetBishopAttacks(64, ^uint64(0)) != 0 {
		t.Fatal("invalid bishop origin was not empty")
	}
	if GetQueenAttacks(-1, ^uint64(0)) != 0 || GetQueenAttacks(64, ^uint64(0)) != 0 {
		t.Fatal("invalid queen origin was not empty")
	}
}

func TestSliderAttacksDeterministicOccupancies(t *testing.T) {
	state := uint64(0x6d2b79f5a4c3e291)
	for iteration := 0; iteration < 2000; iteration++ {
		state ^= state << 7
		state ^= state >> 9
		state ^= state << 8
		sq := int((state >> 17) & 63)
		occupied := state
		if got, want := GetRookAttacks(sq, occupied), oracleSlider(sq, occupied, oracleRookDirections); got != want {
			t.Fatalf("random rook iteration=%d: got %016x want %016x", iteration, got, want)
		}
		if got, want := GetBishopAttacks(sq, occupied), oracleSlider(sq, occupied, oracleBishopDirections); got != want {
			t.Fatalf("random bishop iteration=%d: got %016x want %016x", iteration, got, want)
		}
		if got, want := GetQueenAttacks(sq, occupied), oracleSlider(sq, occupied, oracleQueenDirections); got != want {
			t.Fatalf("random queen iteration=%d: got %016x want %016x", iteration, got, want)
		}
	}
}

func TestSliderAttacksNoAllocationAndInputPreservation(t *testing.T) {
	occupied := uint64(0x9182736455463728)
	before := occupied
	if got := GetQueenAttacks(27, occupied); got != oracleSlider(27, occupied, oracleQueenDirections) {
		t.Fatal("queen attack mismatch before allocation check")
	}
	if occupied != before {
		t.Fatal("attack generation changed occupancy")
	}
	if allocations := testing.AllocsPerRun(1000, func() {
		_ = GetRookAttacks(27, occupied)
		_ = GetBishopAttacks(27, occupied)
		_ = GetQueenAttacks(27, occupied)
		_ = trailingZeros(occupied)
	}); allocations != 0 {
		t.Fatalf("sliding attacks allocated %v times per run", allocations)
	}
	if trailingZeros(0) != 64 {
		t.Fatal("zero trailing-zeros sentinel is not 64")
	}
}
