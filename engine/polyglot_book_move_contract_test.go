package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

const (
	polyglotBookMoveFixture   = "testdata/polyglot_book_move_oracle.json"
	polyglotBookMoveFixtureID = "8289efed4fd50a58a6bff4a816c9b0ecdc7cea7d7d7acc7dcf0a17d26de993ba"
)

type polyglotBookMoveOracle struct {
	Schema                   string                 `json:"schema"`
	ChessCoreSHA256          string                 `json:"chess_core_sha256"`
	PolyglotSourceSHA256     string                 `json:"polyglot_source_sha256"`
	PythonChessVersion       string                 `json:"python_chess_version"`
	ReusedFrozenOracleSHA256 string                 `json:"reused_frozen_oracle_sha256"`
	Scope                    string                 `json:"scope"`
	Cases                    []polyglotBookMoveCase `json:"cases"`
}

type polyglotBookMoveCase struct {
	Name         string   `json:"name"`
	BeforeFEN    string   `json:"before_fen"`
	AfterFEN     string   `json:"after_fen"`
	BeforeKey    string   `json:"before_key"`
	AfterKey     string   `json:"after_key"`
	EntryHex     string   `json:"entry_hex"`
	RawMoveHex   string   `json:"raw_move_hex"`
	ExpectedUCI  string   `json:"expected_uci"`
	LegalBefore  []string `json:"legal_before"`
	LegalAfter   []string `json:"legal_after"`
	Perft2Before uint64   `json:"perft2_before"`
	Perft2After  uint64   `json:"perft2_after"`
}

func loadPolyglotBookMoveOracle(t *testing.T) polyglotBookMoveOracle {
	t.Helper()
	data, err := os.ReadFile(polyglotBookMoveFixture)
	if err != nil {
		t.Fatalf("read Polyglot book-move oracle: %v", err)
	}
	if got := fmt.Sprintf("%x", sha256.Sum256(data)); got != polyglotBookMoveFixtureID {
		t.Fatalf("Polyglot book-move oracle SHA-256 = %s, want %s", got, polyglotBookMoveFixtureID)
	}
	var oracle polyglotBookMoveOracle
	if err := json.Unmarshal(data, &oracle); err != nil {
		t.Fatalf("decode Polyglot book-move oracle: %v", err)
	}
	return oracle
}

func polyglotBookMoveFEN4(fen string) string {
	fields := strings.Fields(fen)
	if len(fields) < 4 {
		return fen
	}
	return strings.Join(fields[:4], " ")
}

func polyglotBookMoveLegal(t *testing.T, pos *Position, notation string) Move {
	t.Helper()
	for _, move := range GenerateLegalMoves(pos) {
		if move.ToString() == notation {
			return move
		}
	}
	t.Fatalf("expected oracle move %s is not legal in %s", notation, GenerateFEN(pos))
	return EmptyMove
}

func assertPolyglotBookMoveLegalSet(t *testing.T, label string, pos *Position, want []string) {
	t.Helper()
	got := polyglotContractLegalMoves(pos)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s legal moves = %v, want independent oracle %v", label, got, want)
	}
}

func TestExternalPolyglotBookMoveContract(t *testing.T) {
	oracle := loadPolyglotBookMoveOracle(t)
	if oracle.Schema != "ngn-independent-polyglot-book-move-oracle-v1" ||
		oracle.PythonChessVersion != "1.11.2" ||
		oracle.ChessCoreSHA256 != "1fde6a8e932508d14b31f6a584497b49c2969fc3c7cdc675347f819f0d667a5b" ||
		oracle.PolyglotSourceSHA256 != "8dc20733bdc1297a9e8993a76737ba4f328a99185ae94b311ddbb5342fce32fc" ||
		oracle.ReusedFrozenOracleSHA256 != "3e7acd53328562ab5fb6943441d586f8bb3723e269c1904c4f128ecee63d05fc" ||
		len(oracle.Cases) != 23 {
		t.Fatalf("unexpected independent book-move oracle provenance/dimensions: %+v", oracle)
	}

	oraclePerft2 := uint64(0)
	for _, testCase := range oracle.Cases {
		oraclePerft2 += testCase.Perft2Before + testCase.Perft2After
	}
	if oraclePerft2 != 7651 {
		t.Fatalf("oracle aggregate perft2 nodes = %d, want 7651", oraclePerft2)
	}

	runtimePerft2 := uint64(0)
	exactDecodes := 0
	decodeFailures := make([]string, 0)
	fenNormalizations := make([]string, 0)
	for _, testCase := range oracle.Cases {
		testCase := testCase
		t.Run(testCase.Name, func(t *testing.T) {
			entry, err := hex.DecodeString(testCase.EntryHex)
			if err != nil || len(entry) != 16 {
				t.Fatalf("decode literal entry %q: bytes=%d err=%v", testCase.EntryHex, len(entry), err)
			}
			if got := hex.EncodeToString(entry[:8]); got != testCase.BeforeKey {
				t.Fatalf("literal entry key = %s, want %s", got, testCase.BeforeKey)
			}
			if got := hex.EncodeToString(entry[8:10]); got != testCase.RawMoveHex {
				t.Fatalf("literal entry move = %s, want %s", got, testCase.RawMoveHex)
			}

			pos, err := ParseFEN(testCase.BeforeFEN)
			if err != nil {
				t.Fatalf("parse before FEN: %v", err)
			}
			if got := PolyglotHash(pos); got != polyglotContractKey(t, testCase.BeforeKey) {
				t.Fatalf("before Polyglot key = %016x, want %s", got, testCase.BeforeKey)
			}
			assertPolyglotBookMoveLegalSet(t, "before", pos, append([]string(nil), testCase.LegalBefore...))
			beforePerft := polyglotContractSnapshot(pos)
			if got := RunPerftTest(pos, 2).Nodes; got != testCase.Perft2Before {
				t.Fatalf("before perft2 = %d, want %d", got, testCase.Perft2Before)
			}
			runtimePerft2 += testCase.Perft2Before
			assertPolyglotContractRawRestored(t, "before/perft", pos, beforePerft)

			expected := polyglotBookMoveLegal(t, pos, testCase.ExpectedUCI)
			path := filepath.Join(t.TempDir(), "literal-standard-entry.bin")
			if err := os.WriteFile(path, entry, 0o600); err != nil {
				t.Fatalf("write literal book entry: %v", err)
			}
			book, err := LoadPolyglotBook(path)
			if err != nil {
				t.Fatalf("load literal book entry: %v", err)
			}
			if book.Size() != 1 {
				t.Fatalf("literal book size = %d, want 1", book.Size())
			}

			beforeProbe := polyglotContractSnapshot(pos)
			decoded, found := book.ProbeBook(pos)
			// This assertion must precede any ordinary Hash call: ProbeBook must
			// not mutate the caller's raw board/tag/EP/cache/clock/FEN/counts.
			assertPolyglotContractRawRestored(t, "probe", pos, beforeProbe)
			if !found {
				decodeFailures = append(decodeFailures, testCase.Name+":not-found")
				t.Errorf("literal book entry was not found for key %s", testCase.BeforeKey)
				return
			}
			if decoded != expected {
				decodeFailures = append(decodeFailures, testCase.Name)
				t.Errorf("decoded move = %s raw=%08x tag=%04b captured=%s, want exact legal %s raw=%08x tag=%04b captured=%s",
					decoded.ToString(), uint32(decoded), decoded.Tag(), decoded.CapturedPiece().Name(),
					expected.ToString(), uint32(expected), expected.Tag(), expected.CapturedPiece().Name())
				// Never apply a decoded move that disagrees with the legal oracle.
				return
			}
			exactDecodes++

			callerBefore := polyglotContractSnapshot(pos)
			work := pos.Copy()
			assertPolyglotContractCopyMapIndependent(t, pos)
			workBefore := polyglotContractSnapshot(work)
			ep, tag, clock, ok := work.MakeMove(decoded)
			if !ok {
				t.Fatal("exact decoded legal move was rejected by MakeMove")
			}
			rawOracleFEN4 := polyglotBookMoveFEN4(testCase.AfterFEN)
			wantAfterFEN4 := rawOracleFEN4
			if testCase.Name == "standard-start-e2e4" {
				// Pinned python-chess xfen projects the unused e3 target to '-'.
				// All other oracle fields and raw inverse state remain exact.
				if rawOracleFEN4 != "rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq e3" {
					t.Fatalf("unexpected raw ordinary-move oracle FEN: %q", rawOracleFEN4)
				}
				wantAfterFEN4 = "rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq -"
				fenNormalizations = append(fenNormalizations, testCase.Name)
			}
			if got := polyglotContractFEN4(work); got != wantAfterFEN4 {
				t.Fatalf("after FEN4 = %q, want independently specified %q (raw oracle %q)",
					got, wantAfterFEN4, rawOracleFEN4)
			}
			if got := PolyglotHash(work); got != polyglotContractKey(t, testCase.AfterKey) {
				t.Fatalf("after Polyglot key = %016x, want %s", got, testCase.AfterKey)
			}
			assertPolyglotBookMoveLegalSet(t, "after", work, append([]string(nil), testCase.LegalAfter...))
			afterPerft := polyglotContractSnapshot(work)
			if got := RunPerftTest(work, 2).Nodes; got != testCase.Perft2After {
				t.Fatalf("after perft2 = %d, want %d", got, testCase.Perft2After)
			}
			runtimePerft2 += testCase.Perft2After
			assertPolyglotContractRawRestored(t, "after/perft", work, afterPerft)

			work.UnMakeMove(decoded, tag, ep, clock)
			assertPolyglotContractRawRestored(t, "inverse", work, workBefore)
			assertPolyglotContractRawRestored(t, "caller", pos, callerBefore)
		})
	}

	t.Logf("external literal-book move summary: cases=%d exact_decodes=%d decode_failures=%d failure_names=%v fen_normalizations=%v runtime_perft2_nodes=%d oracle_perft2_nodes=%d",
		len(oracle.Cases), exactDecodes, len(decodeFailures), decodeFailures, fenNormalizations, runtimePerft2, oraclePerft2)
	if len(decodeFailures) != 0 {
		t.Errorf("literal standard book move decode mismatches=%d/%d", len(decodeFailures), len(oracle.Cases))
	}
	if len(decodeFailures) == 0 && runtimePerft2 != oraclePerft2 {
		t.Errorf("runtime perft2 nodes = %d, want complete oracle aggregate %d", runtimePerft2, oraclePerft2)
	}
}

func TestPolyglotOrthodoxCastleDestinationCompatibility(t *testing.T) {
	tests := []struct {
		name string
		fen  string
		raw  uint16
		uci  string
	}{
		{"white kingside king-to-g", "r3k2r/pppppppp/8/8/8/8/PPPPPPPP/R3K2R w KQkq - 0 1", 0x0106, "e1g1"},
		{"white queenside king-to-c", "r3k2r/pppppppp/8/8/8/8/PPPPPPPP/R3K2R w KQkq - 0 1", 0x0102, "e1c1"},
		{"black kingside king-to-g", "r3k2r/pppppppp/8/8/8/8/PPPPPPPP/R3K2R b KQkq - 0 1", 0x0f3e, "e8g8"},
		{"black queenside king-to-c", "r3k2r/pppppppp/8/8/8/8/PPPPPPPP/R3K2R b KQkq - 0 1", 0x0f3a, "e8c8"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pos, err := ParseFEN(test.fen)
			if err != nil {
				t.Fatal(err)
			}
			want := polyglotBookMoveLegal(t, pos, test.uci)
			if got := polyglotMoveToMove(pos, test.raw); got != want {
				t.Fatalf("legacy king-destination decode = %s raw=%08x tag=%04b, want exact %s raw=%08x tag=%04b",
					got.ToString(), uint32(got), got.Tag(), want.ToString(), uint32(want), want.Tag())
			}
		})
	}
}
