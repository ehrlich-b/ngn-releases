// Standalone current-baseline diagnostic. This deliberately is not an active
// *_test.go file: the proved defect must not make the checked-in test suite red
// while production behavior is frozen for this analysis-only task.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/ehrlich-b/ngn/engine"
)

type fixture struct {
	Name     string   `json:"name"`
	Kind     string   `json:"kind"`
	StartFEN string   `json:"start_fen"`
	Push     string   `json:"push"`
	EPFEN    string   `json:"ep_fen"`
	NoEPFEN  string   `json:"no_ep_fen"`
	EPMove   string   `json:"ep_move"`
	Cycle    []string `json:"cycle,omitempty"`
}

var fixtures = []fixture{
	{
		Name: "pinned_white_capturer", Kind: "illegal_pin",
		StartFEN: "6k1/3p4/8/K3P2r/8/8/8/8 b - - 0 1", Push: "d7d5",
		EPFEN: "6k1/8/8/K2pP2r/8/8/8/8 w - d6 0 2", NoEPFEN: "6k1/8/8/K2pP2r/8/8/8/8 w - - 0 2",
		EPMove: "e5d6", Cycle: []string{"a5a4", "h5h6", "a4a5", "h6h5"},
	},
	{
		Name: "pinned_black_capturer", Kind: "illegal_pin",
		StartFEN: "8/8/8/8/R2p3k/8/4P3/1K6 w - - 0 1", Push: "e2e4",
		EPFEN: "8/8/8/8/R2pP2k/8/8/1K6 b - e3 0 1", NoEPFEN: "8/8/8/8/R2pP2k/8/8/1K6 b - - 0 1",
		EPMove: "d4e3", Cycle: []string{"h4h5", "a4a3", "h5h4", "a3a4"},
	},
	{
		Name: "unrelated_check_white_capturer", Kind: "illegal_unrelated_check",
		StartFEN: "4b1k1/3p4/8/4P3/K7/8/8/8 b - - 0 1", Push: "d7d5",
		EPFEN: "4b1k1/8/8/3pP3/K7/8/8/8 w - d6 0 2", NoEPFEN: "4b1k1/8/8/3pP3/K7/8/8/8 w - - 0 2",
		EPMove: "e5d6", Cycle: []string{"a4a3", "e8f7", "a3a4", "f7e8"},
	},
	{
		Name: "unrelated_check_black_capturer", Kind: "illegal_unrelated_check",
		StartFEN: "8/8/8/7k/3p4/8/4P3/1K1B4 w - - 0 1", Push: "e2e4",
		EPFEN: "8/8/8/7k/3pP3/8/8/1K1B4 b - e3 0 1", NoEPFEN: "8/8/8/7k/3pP3/8/8/1K1B4 b - - 0 1",
		EPMove: "d4e3", Cycle: []string{"h5h6", "d1c2", "h6h5", "c2d1"},
	},
	{
		Name: "legal_white_capturer", Kind: "legal_control",
		StartFEN: "6k1/3p4/7r/K3P3/8/8/8/8 b - - 0 1", Push: "d7d5",
		EPFEN: "6k1/8/7r/K2pP3/8/8/8/8 w - d6 0 2", NoEPFEN: "6k1/8/7r/K2pP3/8/8/8/8 w - - 0 2",
		EPMove: "e5d6",
	},
	{
		Name: "legal_black_capturer", Kind: "legal_control",
		StartFEN: "8/8/8/8/3p3k/R7/4P3/1K6 w - - 0 1", Push: "e2e4",
		EPFEN: "8/8/8/8/3pP2k/R7/8/1K6 b - e3 0 1", NoEPFEN: "8/8/8/8/3pP2k/R7/8/1K6 b - - 0 1",
		EPMove: "d4e3",
	},
}

type state struct {
	FEN             string   `json:"fen"`
	Hash            string   `json:"hash"`
	PolyglotHash    string   `json:"polyglot_hash"`
	EnPassant       string   `json:"en_passant"`
	LegalMoves      []string `json:"legal_moves"`
	Perft2          uint64   `json:"perft_2"`
	FIDEIdentity    string   `json:"fide_legal_identity"`
	FIDEHasLegalEP  bool     `json:"fide_has_legal_ep"`
	RepetitionCount int      `json:"ngn_repetition_count"`
	IsFIDEDrawRule  bool     `json:"ngn_is_fide_draw_rule"`
}

type restoration struct {
	SearchMakeUnmake bool `json:"search_make_unmake"`
	GameMakeUnmake   bool `json:"game_make_unmake"`
	NullMakeUnmake   bool `json:"null_make_unmake"`
}

type fixtureResult struct {
	Fixture           fixture        `json:"fixture"`
	Incremental       state          `json:"incremental"`
	FENEP             state          `json:"fen_ep"`
	FENNoEP           state          `json:"fen_no_ep"`
	Restoration       restoration    `json:"restoration"`
	PlayedHistory     []historyEntry `json:"played_history,omitempty"`
	TrueOccurrences   int            `json:"true_legal_identity_occurrences,omitempty"`
	NGNOccurrences    int            `json:"ngn_hash_occurrences,omitempty"`
	DefectReproduced  bool           `json:"defect_reproduced"`
	LegalControlValid bool           `json:"legal_control_valid"`
}

type historyEntry struct {
	Ply          int    `json:"ply"`
	Move         string `json:"move"`
	Hash         string `json:"hash"`
	FIDEIdentity string `json:"fide_legal_identity"`
	HashCount    int    `json:"ngn_hash_count"`
	FIDEDraw     bool   `json:"ngn_is_fide_draw_rule"`
}

func mustPosition(fen string) *engine.Position {
	pos, err := engine.ParseFEN(fen)
	if err != nil {
		panic(fmt.Sprintf("ParseFEN(%q): %v", fen, err))
	}
	return pos
}

func legalMoves(pos *engine.Position) []string {
	moves := engine.GenerateLegalMoves(pos)
	result := make([]string, 0, len(moves))
	for _, move := range moves {
		result = append(result, move.ToString())
	}
	sort.Strings(result)
	return result
}

func legalMove(pos *engine.Position, text string) engine.Move {
	for _, move := range engine.GenerateLegalMoves(pos) {
		if move.ToString() == text {
			return move
		}
	}
	panic(fmt.Sprintf("move %s is not legal in %s (legal=%v)", text, engine.GenerateFEN(pos), legalMoves(pos)))
}

func perft(pos *engine.Position, depth int) uint64 {
	if depth == 0 {
		return 1
	}
	var nodes uint64
	for _, move := range engine.GenerateLegalMoves(pos) {
		ep, tag, clock, ok := pos.MakeMove(move)
		if !ok {
			panic("MakeMove rejected generated legal move")
		}
		nodes += perft(pos, depth-1)
		pos.UnMakeMove(move, tag, ep, clock)
	}
	return nodes
}

// fideIdentity is test-only normalization. Unlike NGN's adjacency test, it
// retains the FEN EP field only when GenerateLegalMoves contains an EP move.
func fideIdentity(pos *engine.Position) (string, bool) {
	parts := strings.Fields(engine.GenerateFEN(pos))
	if len(parts) != 6 {
		panic("GenerateFEN did not return six fields")
	}
	hasLegalEP := false
	for _, move := range engine.GenerateLegalMoves(pos) {
		if move.IsEnPassant() {
			hasLegalEP = true
			break
		}
	}
	if !hasLegalEP {
		parts[3] = "-"
	}
	return strings.Join(parts[:4], " "), hasLegalEP
}

func inspect(pos *engine.Position) state {
	identity, hasLegalEP := fideIdentity(pos)
	hash := pos.Hash()
	return state{
		FEN: engine.GenerateFEN(pos), Hash: fmt.Sprintf("%016X", hash),
		PolyglotHash: fmt.Sprintf("%016X", engine.PolyglotHash(pos)),
		EnPassant:    pos.EnPassant.Name(), LegalMoves: legalMoves(pos), Perft2: perft(pos, 2),
		FIDEIdentity: identity, FIDEHasLegalEP: hasLegalEP,
		RepetitionCount: pos.Positions[hash], IsFIDEDrawRule: pos.IsFIDEDrawRule(),
	}
}

type snapshot struct {
	FEN       string
	Hash      uint64
	EP        engine.Square
	Tag       engine.PositionTag
	Clock     uint8
	Positions map[uint64]int
}

func snap(pos *engine.Position) snapshot {
	positions := make(map[uint64]int, len(pos.Positions))
	for key, count := range pos.Positions {
		positions[key] = count
	}
	return snapshot{engine.GenerateFEN(pos), pos.Hash(), pos.EnPassant, pos.Tag, pos.HalfMoveClock, positions}
}

func restored(pos *engine.Position, before snapshot) bool {
	return engine.GenerateFEN(pos) == before.FEN && pos.Hash() == before.Hash &&
		pos.EnPassant == before.EP && pos.Tag == before.Tag && pos.HalfMoveClock == before.Clock &&
		reflect.DeepEqual(pos.Positions, before.Positions)
}

func restorationChecks(f fixture) restoration {
	base := mustPosition(f.StartFEN)
	push := legalMove(base, f.Push)
	if _, _, _, ok := base.GameMakeMove(push); !ok {
		panic("GameMakeMove rejected push")
	}
	move := legalMove(base, f.Cycle[0])

	before := snap(base)
	ep, tag, clock, ok := base.MakeMove(move)
	if !ok {
		panic("MakeMove rejected cycle move")
	}
	base.UnMakeMove(move, tag, ep, clock)
	searchOK := restored(base, before)

	before = snap(base)
	ep, tag, clock, ok = base.GameMakeMove(move)
	if !ok {
		panic("GameMakeMove rejected cycle move")
	}
	base.GameUnMakeMove(move, tag, ep, clock)
	gameOK := restored(base, before)

	before = snap(base)
	nullEP := base.MakeNullMove()
	base.UnMakeNullMove(nullEP)
	nullOK := restored(base, before)

	return restoration{searchOK, gameOK, nullOK}
}

func contains(items []string, wanted string) bool {
	for _, item := range items {
		if item == wanted {
			return true
		}
	}
	return false
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: go run ngn_baseline_probe.go SOURCE_HEAD")
		os.Exit(2)
	}
	allDefects := true
	allControls := true
	results := make([]fixtureResult, 0, len(fixtures))
	for _, f := range fixtures {
		incrementalPos := mustPosition(f.StartFEN)
		push := legalMove(incrementalPos, f.Push)
		if _, _, _, ok := incrementalPos.GameMakeMove(push); !ok {
			panic(fmt.Sprintf("%s: push rejected", f.Name))
		}
		incremental := inspect(incrementalPos)
		fenEP := inspect(mustPosition(f.EPFEN))
		fenNoEP := inspect(mustPosition(f.NoEPFEN))
		result := fixtureResult{Fixture: f, Incremental: incremental, FENEP: fenEP, FENNoEP: fenNoEP}

		if strings.HasPrefix(f.Kind, "illegal_") {
			result.Restoration = restorationChecks(f)
			sequence := append([]string{f.Push}, append(append([]string{}, f.Cycle...), f.Cycle...)...)
			played := mustPosition(f.StartFEN)
			identityCounts := map[string]int{}
			for ply, text := range sequence {
				move := legalMove(played, text)
				if _, _, _, ok := played.GameMakeMove(move); !ok {
					panic(fmt.Sprintf("%s ply %d rejected", f.Name, ply+1))
				}
				identity, _ := fideIdentity(played)
				identityCounts[identity]++
				hash := played.Hash()
				result.PlayedHistory = append(result.PlayedHistory, historyEntry{
					Ply: ply + 1, Move: text, Hash: fmt.Sprintf("%016X", hash), FIDEIdentity: identity,
					HashCount: played.Positions[hash], FIDEDraw: played.IsFIDEDrawRule(),
				})
			}
			finalIdentity, _ := fideIdentity(played)
			result.TrueOccurrences = identityCounts[finalIdentity]
			result.NGNOccurrences = played.Positions[played.Hash()]
			result.DefectReproduced =
				!contains(incremental.LegalMoves, f.EPMove) &&
					reflect.DeepEqual(incremental.LegalMoves, fenEP.LegalMoves) &&
					reflect.DeepEqual(fenEP.LegalMoves, fenNoEP.LegalMoves) &&
					incremental.Perft2 == fenEP.Perft2 && fenEP.Perft2 == fenNoEP.Perft2 &&
					incremental.FIDEIdentity == fenNoEP.FIDEIdentity &&
					incremental.Hash == fenEP.Hash && incremental.Hash != fenNoEP.Hash &&
					incremental.PolyglotHash == fenEP.PolyglotHash && incremental.PolyglotHash != fenNoEP.PolyglotHash &&
					result.TrueOccurrences == 3 && result.NGNOccurrences == 2 && !played.IsFIDEDrawRule() &&
					result.Restoration.SearchMakeUnmake && result.Restoration.GameMakeUnmake && result.Restoration.NullMakeUnmake
			allDefects = allDefects && result.DefectReproduced
		} else {
			result.LegalControlValid =
				contains(incremental.LegalMoves, f.EPMove) && contains(fenEP.LegalMoves, f.EPMove) &&
					!contains(fenNoEP.LegalMoves, f.EPMove) &&
					incremental.FIDEIdentity != fenNoEP.FIDEIdentity &&
					incremental.Hash == fenEP.Hash && incremental.Hash != fenNoEP.Hash
			allControls = allControls && result.LegalControlValid
		}
		results = append(results, result)
	}

	receipt := map[string]any{
		"schema":                       "ngn-ep-repetition-current-baseline-probe-v1",
		"generated_at_utc":             time.Now().UTC().Format(time.RFC3339Nano),
		"source_head":                  os.Args[1],
		"go_version":                   runtime.Version(),
		"gomaxprocs":                   runtime.GOMAXPROCS(0),
		"goflags":                      os.Getenv("GOFLAGS"),
		"cgroup":                       strings.TrimSpace(string(mustRead("/proc/self/cgroup"))),
		"all_four_defects_reproduced":  allDefects,
		"both_legal_ep_controls_valid": allControls,
		"fixtures":                     results,
	}
	encoded, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		panic(err)
	}
	fmt.Println(string(encoded))
	if !allDefects || !allControls {
		os.Exit(1)
	}
}

func mustRead(path string) []byte {
	data, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return data
}
