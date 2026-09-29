package texeldata

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"math/rand"
	"sort"

	"github.com/ehrlich-b/ngn/engine"
)

type GenerateOptions struct {
	Openings  [][]string
	Start     int
	Games     int
	Nodes     int
	MaxPlies  int
	Seed      int64
	Generator GeneratorIdentity
}

type GenerateSummary struct {
	Attempted, Completed, Unresolved, Rows int
}

// Generate self-plays a finite, disjoint range of supplied startpos opening
// lines. It never recycles an opening: callers that ask beyond the input range
// get an error instead of a deceptively larger corpus.
func Generate(w io.Writer, cfg GenerateOptions) (GenerateSummary, error) {
	var summary GenerateSummary
	if cfg.Games < 0 || cfg.Start < 0 || cfg.Start+cfg.Games > len(cfg.Openings) {
		return summary, fmt.Errorf("requested games [%d,%d) exceed %d opening lines", cfg.Start, cfg.Start+cfg.Games, len(cfg.Openings))
	}
	if cfg.Nodes <= 0 || cfg.MaxPlies <= 0 {
		return summary, fmt.Errorf("nodes and maxplies must be positive")
	}
	if cfg.Generator.ExecutableSHA256 == "" || cfg.Generator.SourceIdentity == "" {
		return summary, fmt.Errorf("generator identity is required")
	}
	for i := 0; i < cfg.Games; i++ {
		summary.Attempted++
		index := cfg.Start + i
		r, err := playOne(cfg, index)
		if err != nil {
			return summary, fmt.Errorf("opening %d: %w", cfg.Start+i, err)
		}
		if err := EncodeRecord(w, r); err != nil {
			return summary, fmt.Errorf("write game %d: %w", cfg.Start+i, err)
		}
		if r.Completed {
			summary.Completed++
			summary.Rows += len(r.Samples)
		} else {
			summary.Unresolved++
		}
	}
	return summary, nil
}

func playOne(cfg GenerateOptions, index int) (GameRecord, error) {
	pos, err := engine.ParseFEN(StartFEN)
	if err != nil {
		return GameRecord{}, err
	}
	// Match ucinewgame followed by position/moves: clear per-game search state
	// before replaying the opening, then hand every played move to the next root.
	engine.ClearHash()
	engine.ClearHistoryTable()
	engine.ClearKillerMoves()
	engine.ClearCounterMoves()
	engine.SetLastMovePlayed(engine.EmptyMove)
	engine.ClearStop()
	engine.SetMaxNodes(0)
	moves := make([]string, 0, cfg.MaxPlies)
	for openingPly, text := range cfg.Openings[index] {
		if reason, _, done := terminal(pos); done {
			return GameRecord{}, fmt.Errorf("opening move %d follows terminal %s position", openingPly, reason)
		}
		m, err := engine.ParseUCIMove(pos, text)
		if err != nil {
			return GameRecord{}, fmt.Errorf("illegal opening move %q: %w", text, err)
		}
		if _, _, _, ok := pos.GameMakeMove(m); !ok {
			return GameRecord{}, fmt.Errorf("opening move %q rejected", text)
		}
		engine.SetLastMovePlayed(m)
		moves = append(moves, m.ToString())
	}
	if len(moves) > cfg.MaxPlies {
		return GameRecord{}, fmt.Errorf("opening has %d plies, exceeds maxplies %d", len(moves), cfg.MaxPlies)
	}
	r := GameRecord{
		Version:    SchemaVersion,
		Generator:  cfg.Generator,
		GameID:     gameID(cfg, index),
		OpeningID:  canonicalOpeningID(engine.GenerateFEN(pos)),
		OpeningPly: len(moves),
		Generation: GenerationSettings{
			Nodes: cfg.Nodes, MaxPlies: cfg.MaxPlies, Seed: cfg.Seed, OpeningIndex: index,
			SampleFilter: "engine.IsQuietPosition/v1", SampleFilterSource: cfg.Generator.SourceIdentity + ":engine/texel.go",
		},
		Moves: moves,
	}
	rng := perGameRNG(cfg.Seed, r.OpeningID)
	candidates := make([]SampleRecord, 0, 64)
	for {
		if reason, result, done := terminal(pos); done {
			r.Completed, r.TerminalReason, r.Result = true, reason, &result
			r.Samples = spaced(candidates, rng)
			return r, nil
		}
		if len(moves) >= cfg.MaxPlies {
			r.Completed, r.TerminalReason, r.Samples = false, "maxplies", nil
			return r, nil
		}
		// The sample belongs to the position before the next searched move.
		if len(moves) > r.OpeningPly && len(moves) >= 12 && engine.IsQuietPosition(pos) {
			candidates = append(candidates, SampleRecord{Ply: len(moves), FEN: corpusFEN(pos, len(moves))})
		}
		legal := engine.GenerateLegalMoves(pos)
		// terminal() above generated the same legal list, but do not infer a draw
		// from an empty/invalid bestmove: every searched move is independently
		// checked against legal moves before it reaches the game record.
		if len(legal) == 0 {
			return GameRecord{}, fmt.Errorf("terminal state changed before search")
		}
		engine.SetMaxNodes(uint64(cfg.Nodes))
		info := engine.Search(pos, engine.MaximumDepth)
		engine.SetMaxNodes(0)
		// Search marks Stopped when its fixed node budget is spent; that is the
		// normal completion mode for this command, and its last completed ID move
		// remains usable. A missing move is never converted into a draw.
		if info == nil || info.BestMove == engine.EmptyMove {
			return GameRecord{}, fmt.Errorf("search failed or returned no move")
		}
		valid := false
		for _, m := range legal {
			if m == info.BestMove {
				valid = true
				break
			}
		}
		if !valid {
			return GameRecord{}, fmt.Errorf("search returned illegal move %q", info.BestMove.ToString())
		}
		if _, _, _, ok := pos.GameMakeMove(info.BestMove); !ok {
			return GameRecord{}, fmt.Errorf("cannot apply selected move %q", info.BestMove.ToString())
		}
		engine.SetLastMovePlayed(info.BestMove)
		moves = append(moves, info.BestMove.ToString())
		r.Moves = moves
	}
}

func terminal(pos *engine.Position) (string, float64, bool) {
	legal := engine.GenerateLegalMoves(pos)
	if len(legal) == 0 {
		if pos.IsInCheck() {
			if pos.Turn() == engine.White {
				return "checkmate", 0, true
			}
			return "checkmate", 1, true
		}
		return "stalemate", .5, true
	}
	if pos.HalfMoveClock >= 100 {
		return "fifty_move", .5, true
	}
	if pos.IsFIDEDrawRule() {
		return "threefold", .5, true
	}
	if pos.IsDraw() {
		return "insufficient_material", .5, true
	}
	return "", 0, false
}

func spaced(in []SampleRecord, rng *rand.Rand) []SampleRecord {
	if len(in) <= 32 {
		return in
	}
	// One deterministic phase offset avoids always favoring the same exact ply;
	// sorting afterward preserves replay order required by the schema.
	offset := rng.Intn(len(in))
	out := make([]SampleRecord, 0, 32)
	for i := 0; i < 32; i++ {
		out = append(out, in[(offset+i*len(in)/32)%len(in)])
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Ply < out[j].Ply })
	return out
}

func gameID(cfg GenerateOptions, index int) string {
	return fmt.Sprintf("%s:%d:n%d:p%d:s%d", cfg.Generator.SourceIdentity, index, cfg.Nodes, cfg.MaxPlies, cfg.Seed)
}

// perGameRNG derives randomness from seed and canonical opening. Duplicate book
// lines therefore produce identical samples, and an arbitrary worker shard does
// not change a game's record.
func perGameRNG(seed int64, opening string) *rand.Rand {
	var input [8]byte
	binary.LittleEndian.PutUint64(input[:8], uint64(seed))
	sum := sha256.Sum256(append(input[:], []byte(opening)...))
	return rand.New(rand.NewSource(int64(binary.LittleEndian.Uint64(sum[:8]))))
}
