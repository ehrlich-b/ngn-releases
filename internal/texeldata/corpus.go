// Package texeldata defines the auditable, game-grouped corpus used for
// classical Texel experiments.  It deliberately accepts only this format: old
// anonymous FEN/result files cannot establish a game-disjoint holdout.
package texeldata

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/bits"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/ehrlich-b/ngn/engine"
)

const (
	SchemaVersion = 1
	StartFEN      = "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"
)

// GeneratorIdentity identifies the exact producer without making its mutable
// game ID the corpus's integrity key.
type GeneratorIdentity struct {
	ExecutableSHA256 string `json:"executable_sha256"`
	SourceIdentity   string `json:"source_identity"`
}

type SampleRecord struct {
	Ply int    `json:"ply"`
	FEN string `json:"fen"`
}

// GenerationSettings makes a row reproducible without trusting its caller's
// file name. Load validates this metadata but deliberately does not rerun its
// quiet filter: that classification is search/evaluation-weight dependent.
type GenerationSettings struct {
	Nodes              int    `json:"nodes"`
	MaxPlies           int    `json:"maxplies"`
	Seed               int64  `json:"seed"`
	OpeningIndex       int    `json:"opening_index"`
	SampleFilter       string `json:"sample_filter"`
	SampleFilterSource string `json:"sample_filter_source"`
}

// GameRecord is one JSONL row. Moves always start at standard startpos;
// OpeningPly identifies the prefix whose resulting canonical four-field FEN is OpeningID.
// Unresolved max-ply games retain their audit trail but have no Result/samples.
type GameRecord struct {
	Version        int                `json:"version"`
	Generator      GeneratorIdentity  `json:"generator"`
	GameID         string             `json:"game_id"`
	OpeningID      string             `json:"opening_id"`
	OpeningPly     int                `json:"opening_ply"`
	Generation     GenerationSettings `json:"generation"`
	Moves          []string           `json:"moves"`
	Result         *float64           `json:"result,omitempty"`
	TerminalReason string             `json:"terminal_reason"`
	Completed      bool               `json:"completed"`
	Samples        []SampleRecord     `json:"samples"`
}

// Partitions is ready to hand to the gradient command. Samples intentionally
// remain individual observations; up to 32 board-distinct rows per game are
// retained so a long game cannot dominate the fit.
type Partitions struct {
	Train      []engine.TexelSample
	Validation []engine.TexelSample
	Test       []engine.TexelSample
}

type Report struct {
	InputSHA256                                                                map[string]string
	Games                                                                      int
	CompletedGames                                                             int
	UnresolvedGames                                                            int
	Openings                                                                   int
	Rows                                                                       int
	Duplicates                                                                 int
	ExcludedUnresolved                                                         int
	ExcludedPerGameRepeat                                                      int
	ExcludedCrossPartition                                                     int
	TrainRows                                                                  int
	ValidationRows                                                             int
	TestRows                                                                   int
	TrainCompletedGames, ValidationCompletedGames, TestCompletedGames          int
	TrainCompletedOpenings, ValidationCompletedOpenings, TestCompletedOpenings int
	TrainWins, TrainDraws, TrainLosses                                         int
	ValidationWins, ValidationDraws, ValidationLosses                          int
	TestWins, TestDraws, TestLosses                                            int
}

type LoadOptions struct {
	// Seed partitions opening IDs. Empty is stable and intentional.
	Seed string
	// Percentages use [0,100], defaulting to 80/10/10.
	TrainPercent, ValidationPercent, TestPercent int
}

type parsedGame struct {
	record  GameRecord
	content string
	part    partition
	rows    []row
}
type row struct {
	sample engine.TexelSample
	board  string // placement only: the training model sees board only.
}
type partition uint8

const (
	train partition = iota
	validation
	test
)

// Load validates and replays every complete record before partitioning it by
// opening. It rejects partial JSONL, illegal move lists and all inconsistent
// labels. A board collision across splits is retained only in test, then
// validation, then train so evaluation inputs cannot leak across partitions.
func Load(paths []string, opts LoadOptions) (Partitions, Report, error) {
	var out Partitions
	report := Report{InputSHA256: make(map[string]string)}
	if len(paths) == 0 {
		return out, report, fmt.Errorf("no corpus files")
	}
	trainPct, valPct, testPct := opts.TrainPercent, opts.ValidationPercent, opts.TestPercent
	if trainPct == 0 && valPct == 0 && testPct == 0 {
		trainPct, valPct, testPct = 80, 10, 10
	}
	if trainPct < 0 || valPct < 0 || testPct < 0 || trainPct+valPct+testPct != 100 {
		return out, report, fmt.Errorf("split percentages must total 100")
	}

	var games []parsedGame
	byContent, byID, byMoves := make(map[string]GameRecord), make(map[string]GameRecord), make(map[string]GameRecord)
	openingSet := make(map[string]struct{})
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return out, report, err
		}
		sum := sha256.Sum256(data)
		report.InputSHA256[path] = hex.EncodeToString(sum[:])
		s := bufio.NewScanner(strings.NewReader(string(data)))
		s.Buffer(make([]byte, 64*1024), 4*1024*1024)
		line := 0
		for s.Scan() {
			line++
			raw := strings.TrimSpace(s.Text())
			if raw == "" {
				continue
			}
			var r GameRecord
			dec := json.NewDecoder(strings.NewReader(raw))
			dec.DisallowUnknownFields()
			if err := dec.Decode(&r); err != nil {
				return out, report, fmt.Errorf("%s:%d: invalid record: %w", path, line, err)
			}
			var extra any
			if err := dec.Decode(&extra); err != io.EOF {
				return out, report, fmt.Errorf("%s:%d: trailing JSON", path, line)
			}
			rows, content, err := validateRecord(r)
			if err != nil {
				return out, report, fmt.Errorf("%s:%d: %w", path, line, err)
			}
			if prior, ok := byID[r.GameID]; ok && !sameRecord(prior, r) {
				return out, report, fmt.Errorf("game_id %q has inconsistent opening/source/labels", r.GameID)
			}
			byID[r.GameID] = r
			moveKey := strings.Join(r.Moves, "\x00")
			if prior, ok := byMoves[moveKey]; ok && !consistentGame(prior, r) {
				return out, report, fmt.Errorf("identical move list has inconsistent opening/source/labels")
			}
			byMoves[moveKey] = r
			if prior, ok := byContent[content]; ok {
				if prior.Generator != r.Generator {
					return out, report, fmt.Errorf("identical game content has inconsistent source identity")
				}
				if !sameSamples(prior, r) {
					return out, report, fmt.Errorf("identical game content has inconsistent samples")
				}
				report.Duplicates++
				continue
			}
			byContent[content] = r
			p := partitionFor(opts.Seed, r.OpeningID, trainPct, valPct)
			games = append(games, parsedGame{record: r, content: content, part: p, rows: rows})
			openingSet[r.OpeningID] = struct{}{}
		}
		if err := s.Err(); err != nil {
			return out, report, fmt.Errorf("%s: read: %w", path, err)
		}
	}
	// A content-sorted traversal makes row order and the cross-partition gate
	// independent of caller file order.
	sort.Slice(games, func(i, j int) bool { return games[i].content < games[j].content })
	report.Games, report.Openings = len(games), len(openingSet)
	for _, g := range games {
		if g.record.Completed {
			report.CompletedGames++
			switch g.part {
			case train:
				report.TrainCompletedGames++
			case validation:
				report.ValidationCompletedGames++
			case test:
				report.TestCompletedGames++
			}
		} else {
			report.UnresolvedGames++
			report.ExcludedUnresolved++
		}
	}
	completedOpenings := [3]map[string]bool{make(map[string]bool), make(map[string]bool), make(map[string]bool)}
	for _, g := range games {
		if g.record.Completed {
			completedOpenings[g.part][g.record.OpeningID] = true
		}
	}
	report.TrainCompletedOpenings = len(completedOpenings[train])
	report.ValidationCompletedOpenings = len(completedOpenings[validation])
	report.TestCompletedOpenings = len(completedOpenings[test])

	// Keep at most one equivalent BOARD input per game, but preserve repeats from
	// separate games in a partition (their outcome weighting is explicit).
	byPart := [3][]row{}
	for _, g := range games {
		if !g.record.Completed {
			continue
		}
		seen := make(map[string]bool)
		for _, r := range g.rows {
			if seen[r.board] {
				report.ExcludedPerGameRepeat++
				continue
			}
			seen[r.board] = true
			byPart[g.part] = append(byPart[g.part], r)
		}
	}
	// Board-only model collision gate. Keep all distinct-game observations within
	// a partition; only boards already present in a higher-priority partition are
	// excluded. Priority makes this deterministic even when callers reorder files.
	occupied := make(map[string]bool)
	for _, p := range []partition{test, validation, train} {
		boards := make(map[string]bool)
		for _, r := range byPart[p] {
			if occupied[r.board] {
				report.ExcludedCrossPartition++
				continue
			}
			boards[r.board] = true
			switch p {
			case test:
				out.Test = append(out.Test, r.sample)
				countOutcome(&report.TestWins, &report.TestDraws, &report.TestLosses, r.sample.Result)
			case validation:
				out.Validation = append(out.Validation, r.sample)
				countOutcome(&report.ValidationWins, &report.ValidationDraws, &report.ValidationLosses, r.sample.Result)
			default:
				out.Train = append(out.Train, r.sample)
				countOutcome(&report.TrainWins, &report.TrainDraws, &report.TrainLosses, r.sample.Result)
			}
		}
		for board := range boards {
			occupied[board] = true
		}
	}
	report.TrainRows, report.ValidationRows, report.TestRows = len(out.Train), len(out.Validation), len(out.Test)
	report.Rows = report.TrainRows + report.ValidationRows + report.TestRows
	return out, report, nil
}

func validateRecord(r GameRecord) ([]row, string, error) {
	if r.Version != SchemaVersion {
		return nil, "", fmt.Errorf("unsupported schema version %d", r.Version)
	}
	if r.GameID == "" || r.Generator.ExecutableSHA256 == "" || r.Generator.SourceIdentity == "" {
		return nil, "", fmt.Errorf("missing identity metadata")
	}
	if r.Generation.Nodes <= 0 || r.Generation.MaxPlies <= 0 || r.Generation.OpeningIndex < 0 || r.Generation.SampleFilter == "" || r.Generation.SampleFilterSource == "" {
		return nil, "", fmt.Errorf("missing or invalid generation settings")
	}
	if len(r.Moves) > r.Generation.MaxPlies || r.OpeningPly > r.Generation.MaxPlies {
		return nil, "", fmt.Errorf("moves exceed recorded maxplies")
	}
	if r.OpeningPly < 0 || r.OpeningPly > len(r.Moves) {
		return nil, "", fmt.Errorf("invalid opening_ply")
	}
	if r.Completed {
		if r.Result == nil || !validResult(*r.Result) {
			return nil, "", fmt.Errorf("completed game has invalid result")
		}
		if !validReason(r.TerminalReason) {
			return nil, "", fmt.Errorf("unknown terminal reason %q", r.TerminalReason)
		}
	} else if r.Result != nil || r.TerminalReason != "maxplies" || len(r.Samples) != 0 || len(r.Moves) != r.Generation.MaxPlies {
		return nil, "", fmt.Errorf("unresolved game must be maxplies with no result or samples")
	}
	pos, err := engine.ParseFEN(StartFEN)
	if err != nil {
		return nil, "", err
	}
	openingFEN := ""
	if r.OpeningPly == 0 {
		openingFEN = engine.GenerateFEN(pos)
	}
	// Retain only the exact replay checkpoints we need. Position.Copy includes a
	// growing repetition map, so copying it at every ply would be quadratic for
	// long games.
	sampleFEN := make(map[int]string, len(r.Samples))
	sampleTerminal := make(map[int]string, len(r.Samples))
	sampleAt := make(map[int]bool, len(r.Samples))
	for _, s := range r.Samples {
		sampleAt[s.Ply] = true
	}
	for ply, text := range r.Moves {
		if reason, _, done := terminal(pos); done {
			return nil, "", fmt.Errorf("move %d follows terminal %s position", ply, reason)
		}
		m, err := engine.ParseUCIMove(pos, text)
		if err != nil {
			return nil, "", fmt.Errorf("move %d %q: %w", ply, text, err)
		}
		if _, _, _, ok := pos.GameMakeMove(m); !ok {
			return nil, "", fmt.Errorf("move %d %q rejected", ply, text)
		}
		at := ply + 1
		fen := corpusFEN(pos, at)
		if at == r.OpeningPly {
			openingFEN = fen
		}
		if sampleAt[at] {
			sampleFEN[at] = fen
			if reason, _, done := terminal(pos); done {
				sampleTerminal[at] = reason
			}
		}
	}
	if got := canonicalOpeningID(openingFEN); got != r.OpeningID {
		return nil, "", fmt.Errorf("opening_id does not match replay at ply %d", r.OpeningPly)
	}
	if _, err := validOpeningID(r.OpeningID); err != nil {
		return nil, "", fmt.Errorf("opening_id: %w", err)
	}
	if r.Completed {
		if err := verifyTerminal(pos, r.TerminalReason, *r.Result); err != nil {
			return nil, "", err
		}
	} else if reason, _, done := terminal(pos); done {
		return nil, "", fmt.Errorf("unresolved game ends at terminal %s position", reason)
	}
	if len(r.Samples) > 32 {
		return nil, "", fmt.Errorf("more than 32 samples")
	}
	rows := make([]row, 0, len(r.Samples))
	lastPly := -1
	for _, s := range r.Samples {
		if s.Ply <= r.OpeningPly || s.Ply < 12 || s.Ply > len(r.Moves) || s.Ply <= lastPly {
			return nil, "", fmt.Errorf("invalid sample ply %d", s.Ply)
		}
		lastPly = s.Ply
		if sampleFEN[s.Ply] != s.FEN {
			return nil, "", fmt.Errorf("sample ply %d FEN does not match replay", s.Ply)
		}
		sp, err := validPosition(s.FEN)
		if err != nil {
			return nil, "", fmt.Errorf("sample ply %d: %w", s.Ply, err)
		}
		if reason, terminal := sampleTerminal[s.Ply]; terminal {
			return nil, "", fmt.Errorf("sample ply %d is terminal %s position", s.Ply, reason)
		}
		rows = append(rows, row{sample: engine.TexelSample{Board: sp.Board, Result: *r.Result}, board: boardKey(s.FEN)})
	}
	contentBytes, _ := json.Marshal(struct {
		Opening    string             `json:"opening"`
		Moves      []string           `json:"moves"`
		OpeningPly int                `json:"opening_ply"`
		Generation generationIdentity `json:"generation"`
		Result     *float64           `json:"result"`
		Reason     string             `json:"reason"`
		Completed  bool               `json:"completed"`
	}{r.OpeningID, r.Moves, r.OpeningPly, generationKey(r.Generation), r.Result, r.TerminalReason, r.Completed})
	sum := sha256.Sum256(contentBytes)
	return rows, hex.EncodeToString(sum[:]), nil
}

func validPosition(fen string) (*engine.Position, error) {
	p, err := engine.ParseFEN(fen)
	if err != nil {
		return nil, err
	}
	if bits.OnesCount64(p.Board.GetBitboardOf(engine.WhiteKing)) != 1 || bits.OnesCount64(p.Board.GetBitboardOf(engine.BlackKing)) != 1 {
		return nil, fmt.Errorf("position must contain one king of each color")
	}
	return p, nil
}

// canonicalOpeningID is intentionally a four-field FEN: halfmove and fullmove
// counters are not part of opening grouping, while side, castling and EP are.
func canonicalOpeningID(fen string) string {
	parts := strings.Fields(fen)
	if len(parts) < 4 {
		return ""
	}
	return strings.Join(parts[:4], " ")
}

// GenerateFEN cannot recover a fullmove counter from Position (which does not
// store one). Corpus records start at startpos, so replay length supplies it.
func corpusFEN(pos *engine.Position, ply int) string {
	fields := strings.Fields(engine.GenerateFEN(pos))
	fields[5] = strconv.Itoa(ply/2 + 1)
	return strings.Join(fields, " ")
}

func validOpeningID(id string) (*engine.Position, error) {
	if len(strings.Fields(id)) != 4 {
		return nil, fmt.Errorf("opening ID must have placement, turn, castling, and EP fields")
	}
	return validPosition(id + " 0 1")
}
func validResult(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0) && (v == 0 || v == .5 || v == 1)
}
func validReason(s string) bool {
	switch s {
	case "checkmate", "stalemate", "fifty_move", "threefold", "insufficient_material":
		return true
	}
	return false
}
func verifyTerminal(pos *engine.Position, reason string, result float64) error {
	legal := engine.GenerateLegalMoves(pos)
	if len(legal) == 0 { // mate/stalemate must beat every draw rule.
		if pos.IsInCheck() {
			if reason != "checkmate" {
				return fmt.Errorf("terminal position is checkmate, not %s", reason)
			}
			if (pos.Turn() == engine.White && result != 0) || (pos.Turn() == engine.Black && result != 1) {
				return fmt.Errorf("checkmate result inconsistent")
			}
			return nil
		}
		if reason != "stalemate" || result != .5 {
			return fmt.Errorf("terminal position is stalemate")
		}
		return nil
	}
	if result != .5 {
		return fmt.Errorf("non-checkmate completed game must be draw")
	}
	switch reason {
	case "fifty_move":
		if pos.HalfMoveClock < 100 {
			return fmt.Errorf("fifty_move without clock")
		}
	case "threefold":
		if pos.HalfMoveClock >= 100 || !pos.IsFIDEDrawRule() {
			return fmt.Errorf("threefold not present")
		}
	case "insufficient_material":
		if pos.IsFIDEDrawRule() || !pos.IsDraw() {
			return fmt.Errorf("insufficient material not present")
		}
	default:
		return fmt.Errorf("%s is not a nonterminal draw reason", reason)
	}
	return nil
}
func boardKey(fen string) string { return strings.Fields(fen)[0] }

func countOutcome(wins, draws, losses *int, result float64) {
	switch result {
	case 1:
		*wins++
	case .5:
		*draws++
	case 0:
		*losses++
	}
}

func sameRecord(a, b GameRecord) bool {
	aa, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	return string(aa) == string(bb)
}

func sameSamples(a, b GameRecord) bool {
	aa, _ := json.Marshal(a.Samples)
	bb, _ := json.Marshal(b.Samples)
	return string(aa) == string(bb)
}

// generationIdentity intentionally omits the location of a duplicate opening
// in its source file. That index affects auditability and GameID, not the game
// which was played or which samples its canonical opening selects.
type generationIdentity struct {
	Nodes              int    `json:"nodes"`
	MaxPlies           int    `json:"maxplies"`
	Seed               int64  `json:"seed"`
	SampleFilter       string `json:"sample_filter"`
	SampleFilterSource string `json:"sample_filter_source"`
}

func generationKey(g GenerationSettings) generationIdentity {
	return generationIdentity{g.Nodes, g.MaxPlies, g.Seed, g.SampleFilter, g.SampleFilterSource}
}

// consistentGame ignores caller IDs and deterministic sampling choices: an
// identical full move list identifies the played game, whose opening, producer
// and outcome metadata may not change from file to file.
func consistentGame(a, b GameRecord) bool {
	if a.Generator != b.Generator || generationKey(a.Generation) != generationKey(b.Generation) || a.OpeningID != b.OpeningID || a.OpeningPly != b.OpeningPly || a.TerminalReason != b.TerminalReason || a.Completed != b.Completed {
		return false
	}
	if a.Result == nil || b.Result == nil {
		return a.Result == nil && b.Result == nil
	}
	return *a.Result == *b.Result
}
func partitionFor(seed, opening string, trainPct, valPct int) partition {
	sum := sha256.Sum256([]byte(seed + "\x00" + opening))
	n := int(sum[0]) * 100 / 256
	if n < trainPct {
		return train
	}
	if n < trainPct+valPct {
		return validation
	}
	return test
}

// EncodeRecord is a small streaming primitive for generators and tests.
func EncodeRecord(w io.Writer, r GameRecord) error {
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(b))
	return err
}

// SortedInputHashes returns a stable representation useful in manifests.
func SortedInputHashes(r Report) []string {
	keys := make([]string, 0, len(r.InputSHA256))
	for k := range r.InputSHA256 {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		out = append(out, k+"="+r.InputSHA256[k])
	}
	return out
}
