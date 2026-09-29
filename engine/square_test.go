package engine

import (
	"testing"
)

func TestSquareCreation(t *testing.T) {
	tests := []struct {
		file     File
		rank     Rank
		expected Square
		name     string
	}{
		{FileA, Rank1, A1, "A1 Square"},
		{FileB, Rank2, B2, "B2 Square"},
		{FileE, Rank1, E1, "E1 Square"},
		{FileE, Rank8, E8, "E8 Square"},
		{FileH, Rank8, H8, "H8 Square"},
		{FileA, Rank8, A8, "A8 Square"},
		{FileH, Rank1, H1, "H1 Square"},
		{FileD, Rank4, D4, "D4 Square"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			square := SquareOf(tt.file, tt.rank)
			if square != tt.expected {
				t.Errorf("SquareOf(%v, %v) = %v; want %v", tt.file, tt.rank, square, tt.expected)
			}
		})
	}
}

func TestSquareProperties(t *testing.T) {
	tests := []struct {
		square       Square
		expectedFile File
		expectedRank Rank
		expectedName string
		name         string
	}{
		{A1, FileA, Rank1, "a1", "A1 Properties"},
		{E4, FileE, Rank4, "e4", "E4 Properties"},
		{H8, FileH, Rank8, "h8", "H8 Properties"},
		{B7, FileB, Rank7, "b7", "B7 Properties"},
		{F3, FileF, Rank3, "f3", "F3 Properties"},
		{C6, FileC, Rank6, "c6", "C6 Properties"},
		{D5, FileD, Rank5, "d5", "D5 Properties"},
		{G2, FileG, Rank2, "g2", "G2 Properties"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.square.File() != tt.expectedFile {
				t.Errorf("%s File() = %v; want %v", tt.name, tt.square.File(), tt.expectedFile)
			}
			if tt.square.Rank() != tt.expectedRank {
				t.Errorf("%s Rank() = %v; want %v", tt.name, tt.square.Rank(), tt.expectedRank)
			}
			if tt.square.Name() != tt.expectedName {
				t.Errorf("%s Name() = %q; want %q", tt.name, tt.square.Name(), tt.expectedName)
			}
		})
	}
}

func TestFileNames(t *testing.T) {
	tests := []struct {
		file     File
		expected string
		name     string
	}{
		{FileA, "a", "File A Name"},
		{FileB, "b", "File B Name"},
		{FileC, "c", "File C Name"},
		{FileD, "d", "File D Name"},
		{FileE, "e", "File E Name"},
		{FileF, "f", "File F Name"},
		{FileG, "g", "File G Name"},
		{FileH, "h", "File H Name"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.file.Name()
			if result != tt.expected {
				t.Errorf("%s Name() = %q; want %q", tt.name, result, tt.expected)
			}
		})
	}
}

func TestRankNames(t *testing.T) {
	tests := []struct {
		rank     Rank
		expected int
		name     string
	}{
		{Rank1, 1, "Rank 1 Name"},
		{Rank2, 2, "Rank 2 Name"},
		{Rank3, 3, "Rank 3 Name"},
		{Rank4, 4, "Rank 4 Name"},
		{Rank5, 5, "Rank 5 Name"},
		{Rank6, 6, "Rank 6 Name"},
		{Rank7, 7, "Rank 7 Name"},
		{Rank8, 8, "Rank 8 Name"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.rank.Name()
			if result != tt.expected {
				t.Errorf("%s Name() = %v; want %v", tt.name, result, tt.expected)
			}
		})
	}
}

func TestSquareColor(t *testing.T) {
	tests := []struct {
		square        Square
		expectedColor Color
		name          string
	}{
		{A1, Black, "A1 Color (Black)"},
		{A2, White, "A2 Color (White)"},
		{B1, White, "B1 Color (White)"},
		{B2, Black, "B2 Color (Black)"},
		{E4, White, "E4 Color (White)"},
		{E5, Black, "E5 Color (Black)"},
		{H1, White, "H1 Color (White)"},
		{H8, Black, "H8 Color (Black)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.square.GetColor()
			if result != tt.expectedColor {
				t.Errorf("%s GetColor() = %v; want %v", tt.name, result, tt.expectedColor)
			}
		})
	}
}

func TestSquareStringConversion(t *testing.T) {
	tests := []struct {
		square   Square
		expected string
		name     string
	}{
		{A1, "a1", "A1 String"},
		{E4, "e4", "E4 String"},
		{H8, "h8", "H8 String"},
		{D5, "d5", "D5 String"},
		{B7, "b7", "B7 String"},
		{F2, "f2", "F2 String"},
		{C6, "c6", "C6 String"},
		{G3, "g3", "G3 String"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.square.String()
			if result != tt.expected {
				t.Errorf("%s String() = %q; want %q", tt.name, result, tt.expected)
			}
		})
	}
}

func TestSquareRoundTrip(t *testing.T) {
	// Test that creating a square from file/rank and then extracting file/rank works
	for file := FileA; file <= FileH; file++ {
		for rank := Rank1; rank <= Rank8; rank++ {
			square := SquareOf(file, rank)
			actualFile := square.File()
			actualRank := square.Rank()

			if actualFile != file {
				t.Errorf("SquareOf(%v, %v).File() = %v; want %v", file, rank, actualFile, file)
			}
			if actualRank != rank {
				t.Errorf("SquareOf(%v, %v).Rank() = %v; want %v", file, rank, actualRank, rank)
			}
		}
	}
}

func BenchmarkSquareOf(b *testing.B) {
	BenchmarkFunction(b, func() {
		SquareOf(FileE, Rank4)
	})
}

func BenchmarkSquareFile(b *testing.B) {
	square := E4
	BenchmarkFunction(b, func() {
		_ = square.File()
	})
}

func BenchmarkSquareRank(b *testing.B) {
	square := E4
	BenchmarkFunction(b, func() {
		_ = square.Rank()
	})
}

func BenchmarkSquareName(b *testing.B) {
	square := E4
	BenchmarkFunction(b, func() {
		_ = square.Name()
	})
}

func BenchmarkSquareGetColor(b *testing.B) {
	square := E4
	BenchmarkFunction(b, func() {
		_ = square.GetColor()
	})
}
