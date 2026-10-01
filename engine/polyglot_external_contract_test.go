package engine

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const (
	polyglotContractFixture   = "testdata/polyglot_special_move_oracle.json"
	polyglotContractFixtureID = "3e7acd53328562ab5fb6943441d586f8bb3723e269c1904c4f128ecee63d05fc"
	polyglotCanonicalTableID  = "9aee840377ba74c9a7fe8e254958ad312aa15766257d18823f866b070cbcc9e8"
)

type polyglotContractOracle struct {
	Schema                      string                    `json:"schema"`
	ChessCoreSHA256             string                    `json:"chess_core_sha256"`
	PolyglotSourceSHA256        string                    `json:"polyglot_source_sha256"`
	PythonChessVersion          string                    `json:"python_chess_version"`
	StartposReferenceKey        string                    `json:"startpos_reference_key"`
	ReusedStockfishOracleSHA256 string                    `json:"reused_stockfish_oracle_sha256"`
	Histories                   []polyglotContractHistory `json:"histories"`
	Vectors                     int                       `json:"vectors"`
	TotalOraclePerft2Nodes      uint64                    `json:"total_oracle_perft2_nodes"`
}

type polyglotContractHistory struct {
	Name     string                  `json:"name"`
	StartFEN string                  `json:"start_fen"`
	Actions  []string                `json:"actions"`
	Stages   []polyglotContractStage `json:"stages"`
}

type polyglotContractStage struct {
	Ply         int      `json:"ply"`
	Action      string   `json:"action"`
	FEN4        string   `json:"fen4"`
	PolyglotHex string   `json:"polyglot_hex"`
	LegalMoves  []string `json:"legal_moves"`
	Perft2Nodes uint64   `json:"perft2_nodes"`
}

type polyglotContractRawSnapshot struct {
	board         Bitboard
	tag           PositionTag
	enPassant     Square
	rawHash       uint64
	halfMoveClock uint8
	positions     map[uint64]int
	fen           string
}

type polyglotContractUndo struct {
	move       Move
	enPassant  Square
	tag        PositionTag
	clock      uint8
	null       bool
	game       bool
	before     polyglotContractRawSnapshot
	stageIndex int
}

type polyglotContractMismatch struct {
	label string
	want  uint64
	got   uint64
}

type polyglotContractTracker struct {
	digest     hash.Hash
	checks     int
	mismatches []polyglotContractMismatch
}

func loadPolyglotContractOracle(t *testing.T) polyglotContractOracle {
	t.Helper()
	data, err := os.ReadFile(polyglotContractFixture)
	if err != nil {
		t.Fatalf("read Polyglot oracle: %v", err)
	}
	if got := fmt.Sprintf("%x", sha256.Sum256(data)); got != polyglotContractFixtureID {
		t.Fatalf("Polyglot oracle SHA-256 = %s, want %s", got, polyglotContractFixtureID)
	}
	var oracle polyglotContractOracle
	if err := json.Unmarshal(data, &oracle); err != nil {
		t.Fatalf("decode Polyglot oracle: %v", err)
	}
	return oracle
}

func polyglotContractSnapshot(pos *Position) polyglotContractRawSnapshot {
	counts := make(map[uint64]int, len(pos.Positions))
	for key, count := range pos.Positions {
		counts[key] = count
	}
	return polyglotContractRawSnapshot{
		board: pos.Board, tag: pos.Tag, enPassant: pos.EnPassant,
		rawHash: pos.hash, halfMoveClock: pos.HalfMoveClock,
		positions: counts, fen: GenerateFEN(pos),
	}
}

// This assertion deliberately reads the raw cached key rather than invoking
// Hash. Restoration must be proved before any cache initialization/refresh.
func assertPolyglotContractRawRestored(t *testing.T, label string, pos *Position, want polyglotContractRawSnapshot) {
	t.Helper()
	if pos.Board != want.board || pos.Tag != want.tag || pos.EnPassant != want.enPassant ||
		pos.hash != want.rawHash || pos.HalfMoveClock != want.halfMoveClock ||
		!reflect.DeepEqual(pos.Positions, want.positions) || GenerateFEN(pos) != want.fen {
		t.Fatalf("%s raw restoration failed before Hash:\n got fen=%s tag=%08b ep=%s raw=%016x clock=%d counts=%v\nwant fen=%s tag=%08b ep=%s raw=%016x clock=%d counts=%v",
			label, GenerateFEN(pos), pos.Tag, pos.EnPassant, pos.hash, pos.HalfMoveClock, pos.Positions,
			want.fen, want.tag, want.enPassant, want.rawHash, want.halfMoveClock, want.positions)
	}
}

func polyglotContractFEN4(pos *Position) string {
	fields := strings.Fields(GenerateFEN(pos))
	return strings.Join(fields[:4], " ")
}

func polyglotContractLegalMoves(pos *Position) []string {
	moves := GenerateLegalMoves(pos)
	result := make([]string, 0, len(moves))
	for _, move := range moves {
		result = append(result, move.ToString())
	}
	sort.Strings(result)
	return result
}

func polyglotContractMove(t *testing.T, pos *Position, notation string) Move {
	t.Helper()
	for _, move := range GenerateLegalMoves(pos) {
		if move.ToString() == notation {
			return move
		}
	}
	t.Fatalf("%s is not legal in %s (legal %v)", notation, GenerateFEN(pos), polyglotContractLegalMoves(pos))
	return EmptyMove
}

func polyglotContractKey(t *testing.T, value string) uint64 {
	t.Helper()
	key, err := strconv.ParseUint(value, 16, 64)
	if err != nil {
		t.Fatalf("parse Polyglot key %q: %v", value, err)
	}
	return key
}

func (tracker *polyglotContractTracker) compare(label string, want, got uint64) {
	tracker.checks++
	fmt.Fprintf(tracker.digest, "%s=%016x\n", label, got)
	if got != want {
		tracker.mismatches = append(tracker.mismatches, polyglotContractMismatch{label: label, want: want, got: got})
	}
}

func assertPolyglotContractStage(t *testing.T, tracker *polyglotContractTracker, label string, pos *Position, stage polyglotContractStage) {
	t.Helper()
	if got := polyglotContractFEN4(pos); got != stage.FEN4 {
		t.Fatalf("%s FEN4 = %q, want %q", label, got, stage.FEN4)
	}
	tracker.compare(label, polyglotContractKey(t, stage.PolyglotHex), PolyglotHash(pos))
	if got := polyglotContractLegalMoves(pos); !reflect.DeepEqual(got, stage.LegalMoves) {
		t.Fatalf("%s legal moves = %v, want external oracle %v", label, got, stage.LegalMoves)
	}

	// Perft internally makes and unmakes real moves. Check its restoration from
	// raw state before asking Hash to initialize or refresh anything.
	beforePerft := polyglotContractSnapshot(pos)
	if got := RunPerftTest(pos, 2).Nodes; got != stage.Perft2Nodes {
		t.Fatalf("%s perft2 = %d, want external oracle %d", label, got, stage.Perft2Nodes)
	}
	assertPolyglotContractRawRestored(t, label+"/perft", pos, beforePerft)

	// Ordinary/repetition keys intentionally are not compared numerically to
	// Polyglot. They must only agree between incremental, cold-copy, and FEN
	// reconstruction paths for this build.
	incremental := pos.Hash()
	cold := pos.Copy()
	cold.hash = 0
	if got := cold.Hash(); got != incremental {
		t.Fatalf("%s cold ordinary key %016x != incremental %016x", label, got, incremental)
	}
	rebuilt, err := ParseFEN(GenerateFEN(pos))
	if err != nil {
		t.Fatalf("%s rebuild FEN: %v", label, err)
	}
	if got := rebuilt.Hash(); got != incremental {
		t.Fatalf("%s FEN ordinary key %016x != incremental %016x", label, got, incremental)
	}
}

func assertPolyglotContractCopyMapIndependent(t *testing.T, pos *Position) {
	t.Helper()
	copyPos := pos.Copy()
	const sentinel = ^uint64(0)
	if _, exists := pos.Positions[sentinel]; exists {
		t.Fatal("unexpected occurrence-map sentinel collision")
	}
	copyPos.Positions[sentinel] = 37
	if _, leaked := pos.Positions[sentinel]; leaked {
		t.Fatal("Position.Copy shares its occurrence map with the caller")
	}
	delete(copyPos.Positions, sentinel)
}

func TestCanonicalPolyglotRandomTable(t *testing.T) {
	hash := sha256.New()
	var encoded [8]byte
	for _, value := range polyglotRandom {
		binary.BigEndian.PutUint64(encoded[:], value)
		_, _ = hash.Write(encoded[:])
	}
	if got := hex.EncodeToString(hash.Sum(nil)); got != polyglotCanonicalTableID {
		t.Fatalf("canonical Polyglot table SHA-256 = %s, want %s", got, polyglotCanonicalTableID)
	}
}

func TestExternalPolyglotSpecialMoveContract(t *testing.T) {
	oracle := loadPolyglotContractOracle(t)
	if oracle.Schema != "ngn-independent-polyglot-special-move-oracle-v1" ||
		oracle.PythonChessVersion != "1.11.2" ||
		oracle.ChessCoreSHA256 != "1fde6a8e932508d14b31f6a584497b49c2969fc3c7cdc675347f819f0d667a5b" ||
		oracle.PolyglotSourceSHA256 != "8dc20733bdc1297a9e8993a76737ba4f328a99185ae94b311ddbb5342fce32fc" ||
		oracle.ReusedStockfishOracleSHA256 != "897f7ebd0c3adc602839d7782673ddbecdd7c1bc72c58dc73fb424f8884b283e" {
		t.Fatalf("unexpected independent-oracle provenance: %+v", oracle)
	}
	if oracle.Vectors != 99 || len(oracle.Histories) != 29 || oracle.TotalOraclePerft2Nodes != 10836 {
		t.Fatalf("oracle dimensions histories=%d vectors=%d perft2=%d, want 29/99/10836",
			len(oracle.Histories), oracle.Vectors, oracle.TotalOraclePerft2Nodes)
	}

	tracker := polyglotContractTracker{digest: sha256.New()}
	start, err := ParseFEN("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	tracker.compare("standard-startpos", polyglotContractKey(t, oracle.StartposReferenceKey), PolyglotHash(start))

	states := 0
	perftNodes := uint64(0)
	nullHistories := 0
	for _, history := range oracle.Histories {
		if len(history.Stages) != len(history.Actions)+1 {
			t.Fatalf("%s has %d stages for %d actions", history.Name, len(history.Stages), len(history.Actions))
		}
		states += len(history.Stages)
		for _, stage := range history.Stages {
			perftNodes += stage.Perft2Nodes
		}

		pos, err := ParseFEN(history.StartFEN)
		if err != nil {
			t.Fatalf("%s start FEN: %v", history.Name, err)
		}
		assertPolyglotContractCopyMapIndependent(t, pos)
		assertPolyglotContractStage(t, &tracker, history.Name+"/forward/0", pos, history.Stages[0])

		searchHistory := false
		for _, action := range history.Actions {
			if action == "0000" {
				searchHistory = true
				nullHistories++
			}
		}
		undos := make([]polyglotContractUndo, 0, len(history.Actions))
		for i, action := range history.Actions {
			before := polyglotContractSnapshot(pos)
			if action == "0000" {
				ep := pos.MakeNullMove()
				undos = append(undos, polyglotContractUndo{enPassant: ep, null: true, before: before, stageIndex: i})
				assertPolyglotContractStage(t, &tracker, fmt.Sprintf("%s/forward/%d/null", history.Name, i+1), pos, history.Stages[i+1])
				continue
			}

			// Exercise the search make/unmake pair independently at every real
			// transition before advancing the played/game path.
			searchCopy := pos.Copy()
			searchBefore := polyglotContractSnapshot(searchCopy)
			searchMove := polyglotContractMove(t, searchCopy, action)
			ep, tag, clock, ok := searchCopy.MakeMove(searchMove)
			if !ok {
				t.Fatalf("%s search MakeMove(%s) rejected", history.Name, action)
			}
			assertPolyglotContractStage(t, &tracker, fmt.Sprintf("%s/search-forward/%d", history.Name, i+1), searchCopy, history.Stages[i+1])
			searchCopy.UnMakeMove(searchMove, tag, ep, clock)
			assertPolyglotContractRawRestored(t, history.Name+"/search-inverse", searchCopy, searchBefore)
			assertPolyglotContractStage(t, &tracker, fmt.Sprintf("%s/search-inverse/%d", history.Name, i), searchCopy, history.Stages[i])

			move := polyglotContractMove(t, pos, action)
			if searchHistory {
				ep, tag, clock, ok = pos.MakeMove(move)
			} else {
				ep, tag, clock, ok = pos.GameMakeMove(move)
			}
			if !ok {
				t.Fatalf("%s played move %s rejected", history.Name, action)
			}
			undos = append(undos, polyglotContractUndo{
				move: move, enPassant: ep, tag: tag, clock: clock,
				game: !searchHistory, before: before, stageIndex: i,
			})
			assertPolyglotContractStage(t, &tracker, fmt.Sprintf("%s/forward/%d", history.Name, i+1), pos, history.Stages[i+1])
			if !searchHistory {
				if count := pos.Positions[pos.Hash()]; count < 1 {
					t.Fatalf("%s game occurrence count after %s = %d", history.Name, action, count)
				}
			}
		}

		for i := len(undos) - 1; i >= 0; i-- {
			undo := undos[i]
			if undo.null {
				pos.UnMakeNullMove(undo.enPassant)
			} else if undo.game {
				pos.GameUnMakeMove(undo.move, undo.tag, undo.enPassant, undo.clock)
			} else {
				pos.UnMakeMove(undo.move, undo.tag, undo.enPassant, undo.clock)
			}
			assertPolyglotContractRawRestored(t, history.Name+"/played-inverse", pos, undo.before)
			assertPolyglotContractStage(t, &tracker, fmt.Sprintf("%s/inverse/%d", history.Name, undo.stageIndex), pos, history.Stages[undo.stageIndex])
		}
	}

	if states != oracle.Vectors || perftNodes != oracle.TotalOraclePerft2Nodes || nullHistories != 4 {
		t.Fatalf("executed states=%d perft2=%d null histories=%d, want 99/10836/4", states, perftNodes, nullHistories)
	}
	t.Logf("external Polyglot contract summary: histories=%d frozen_states=%d frozen_perft2_nodes=%d book_checks=%d mismatches=%d actual_sequence_sha256=%s",
		len(oracle.Histories), states, perftNodes, tracker.checks, len(tracker.mismatches), hex.EncodeToString(tracker.digest.Sum(nil)))
	if len(tracker.mismatches) != 0 {
		limit := len(tracker.mismatches)
		if limit > 8 {
			limit = 8
		}
		for _, mismatch := range tracker.mismatches[:limit] {
			t.Logf("Polyglot mismatch %s: got=%016x want=%016x", mismatch.label, mismatch.got, mismatch.want)
		}
		t.Errorf("external Polyglot numeric contract mismatches=%d/%d checks",
			len(tracker.mismatches), tracker.checks)
	}
}
