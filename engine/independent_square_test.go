package engine

import "testing"

func TestIndependentPrimitiveSquareConstantsAndRoundTrips(t *testing.T) {
	for rank := Rank1; rank <= Rank8; rank++ {
		for file := FileA; file <= FileH; file++ {
			square := SquareOf(file, rank)
			want := Square(int8(rank)*8 + int8(file))
			if square != want || square.File() != file || square.Rank() != rank {
				t.Errorf("SquareOf(%d, %d) = %d (%d, %d), want %d (%d, %d)", file, rank, square, square.File(), square.Rank(), want, file, rank)
			}
		}
	}
	if A1 != 0 || B1 != 1 || H1 != 7 || A8 != 56 || H8 != 63 || NoSquare != -1 {
		t.Fatal("square constants do not have their boundary values")
	}
	for i := 0; i < 8; i++ {
		if Ranks[i] != Rank(i) || Files[i] != File(i) {
			t.Errorf("ordered coordinate slices differ at %d", i)
		}
	}
}

func TestIndependentPrimitiveSquareCompatibilityForArbitraryInt8Values(t *testing.T) {
	tests := []struct {
		square Square
		file   File
		rank   Rank
	}{
		{Square(-128), FileA, Rank(-16)},
		{Square(-9), FileH, Rank(-1)},
		{Square(-1), FileH, Rank(0)},
		{Square(0), FileA, Rank(0)},
		{Square(63), FileH, Rank(7)},
		{Square(64), FileA, Rank(8)},
		{Square(127), FileH, Rank(15)},
	}
	for _, test := range tests {
		if got := test.square.File(); got != test.file {
			t.Errorf("Square(%d).File() = %d, want %d", test.square, got, test.file)
		}
		if got := test.square.Rank(); got != test.rank {
			t.Errorf("Square(%d).Rank() = %d, want %d", test.square, got, test.rank)
		}
	}
}

func TestIndependentPrimitiveCoordinateNames(t *testing.T) {
	if FileA.Name() != "a" || FileH.Name() != "h" || File(-1).Name() != "`" {
		t.Fatal("file names do not use byte-wrapped coordinates")
	}
	if Rank1.Name() != 1 || Rank8.Name() != 8 || Rank(-1).Name() != 0 {
		t.Fatal("rank names do not return int(rank)+1")
	}
	if A1.Name() != "a1" || H8.Name() != "h8" || A1.String() != "a1" || Square(-1).String() != "h1" {
		t.Fatal("square coordinate names are incorrect")
	}
}

func TestIndependentPrimitiveSquareColorsAndMap(t *testing.T) {
	if DarkSquares != 0xAA55AA55AA55AA55 {
		t.Fatalf("DarkSquares = %#x", DarkSquares)
	}
	for _, test := range []struct {
		square Square
		color  Color
	}{
		{A1, Black},
		{B1, White},
		{A2, White},
		{B2, Black},
		{H8, Black},
		{Square(-1), White},
		{Square(64), White},
	} {
		if got := test.square.GetColor(); got != test.color {
			t.Errorf("Square(%d).GetColor() = %d, want %d", test.square, got, test.color)
		}
	}

	if len(NameToSquareMap) != 64 {
		t.Fatalf("len(NameToSquareMap) = %d, want 64", len(NameToSquareMap))
	}
	for rank := Rank1; rank <= Rank8; rank++ {
		for file := FileA; file <= FileH; file++ {
			square := SquareOf(file, rank)
			if got, ok := NameToSquareMap[square.Name()]; !ok || got != square {
				t.Errorf("NameToSquareMap[%q] = %d, %t; want %d, true", square.Name(), got, ok, square)
			}
		}
	}
}
