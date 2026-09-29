package engine

import "testing"

func TestRodentSearchScoreReservesMateBandWithoutRule50Input(t *testing.T) {
	tests := []struct {
		input int
		want  int
	}{
		{-29000, -25000},
		{-25001, -25000},
		{-25000, -25000},
		{-24999, -24999},
		{0, 0},
		{24999, 24999},
		{25000, 25000},
		{25001, 25000},
		{29000, 25000},
	}
	for _, test := range tests {
		if got := rodentSearchScore(test.input); got != test.want {
			t.Fatalf("rodentSearchScore(%d) = %d, want %d", test.input, got, test.want)
		}
	}
}
