package engine

import (
	"testing"
)

func TestSyzygyMaterialKey(t *testing.T) {
	testCases := []struct {
		name        string
		fen         string
		expectedKey string
	}{
		{
			name:        "KQ vs K",
			fen:         "4k3/8/8/8/8/8/8/4K2Q w - - 0 1",
			expectedKey: "KQvK",
		},
		{
			name:        "KR vs K",
			fen:         "4k3/8/8/8/8/8/8/R3K3 w Q - 0 1", // Note: has castling rights so won't probe
			expectedKey: "KRvK",
		},
		{
			name:        "KQ vs KR",
			fen:         "4k2r/8/8/8/8/8/8/4K2Q w - - 0 1",
			expectedKey: "KRvKQ", // Normalized: equal pieces, so alphabetically R > Q
		},
		{
			name:        "KRR vs KR",
			fen:         "4k2r/8/8/8/8/8/8/R3K2R w - - 0 1",
			expectedKey: "KRRvKR",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pos, err := ParseFEN(tc.fen)
			if err != nil {
				t.Fatalf("Failed to parse FEN: %v", err)
			}

			key := getMaterialKey(pos)
			if key != tc.expectedKey {
				t.Errorf("Expected material key %s, got %s", tc.expectedKey, key)
			}
		})
	}
}

func TestSyzygyPieceCount(t *testing.T) {
	testCases := []struct {
		name     string
		fen      string
		expected int
	}{
		{
			name:     "Starting position",
			fen:      "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
			expected: 32,
		},
		{
			name:     "KQ vs K",
			fen:      "4k3/8/8/8/8/8/8/4K2Q w - - 0 1",
			expected: 3,
		},
		{
			name:     "KRR vs KR",
			fen:      "4k2r/8/8/8/8/8/8/R3K2R w - - 0 1",
			expected: 5,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pos, err := ParseFEN(tc.fen)
			if err != nil {
				t.Fatalf("Failed to parse FEN: %v", err)
			}

			count := countPieces(pos)
			if count != tc.expected {
				t.Errorf("Expected %d pieces, got %d", tc.expected, count)
			}
		})
	}
}

func TestSyzygyProbeWithoutTB(t *testing.T) {
	// Without tablebases loaded, probing should return Found=false
	pos, _ := ParseFEN("4k3/8/8/8/8/8/8/4K2Q w - - 0 1")

	result := ProbeWDL(pos)

	if result.Found {
		t.Error("Expected probe to fail without tablebases loaded")
	}
}

func TestSyzygySwapMaterialKey(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"KQvKR", "KRvKQ"},
		{"KvKQ", "KQvK"},
		{"KRRvKBB", "KBBvKRR"},
	}

	for _, tc := range testCases {
		result := swapMaterialKey(tc.input)
		if result != tc.expected {
			t.Errorf("swapMaterialKey(%s) = %s, expected %s", tc.input, result, tc.expected)
		}
	}
}

func TestSyzygyFilenameCount(t *testing.T) {
	testCases := []struct {
		filename string
		expected int
	}{
		{"KQvK", 3},
		{"KRvKR", 4},
		{"KQRvKBB", 6},
		{"KPPPvKP", 6},
	}

	for _, tc := range testCases {
		result := countPiecesInFilename(tc.filename)
		if result != tc.expected {
			t.Errorf("countPiecesInFilename(%s) = %d, expected %d", tc.filename, result, tc.expected)
		}
	}
}
