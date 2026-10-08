// Package datagen produces self-play labels using NGN's evaluator and rules.
package datagen

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/bits"
	"math/rand"
	"os"
	"time"

	"github.com/ehrlich-b/ngn/engine"
	"github.com/ehrlich-b/ngn/internal/ngnp"
)

const StartFEN = "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"

type Settings struct {
	NetPath           string `json:"net,omitempty"`
	Seed              int64  `json:"seed"`
	Games             uint64 `json:"games"` // Zero means run until canceled.
	Nodes             uint64 `json:"nodes"`
	RandomPlies       int    `json:"random_plies"` // -1 chooses 8 or 9 independently per game.
	MaxPly            int    `json:"maxply"`
	HashMB            int    `json:"hash_mb"`
	VerificationNodes uint64 `json:"verification_nodes"`
	OpeningLimitCP    int    `json:"opening_limit_cp"`
	WinCP             int    `json:"win_cp"`
	WinPlies          int    `json:"win_plies"`
	DrawCP            int    `json:"draw_cp"`
	DrawPlies         int    `json:"draw_plies"`
	DrawMinPly        int    `json:"draw_min_ply"`
}

func Defaults() Settings {
	return Settings{Seed: 1, Nodes: 5000, RandomPlies: -1, MaxPly: 400, HashMB: 16,
		VerificationNodes: 1000, OpeningLimitCP: 400, WinCP: 2000, WinPlies: 6,
		DrawCP: 8, DrawPlies: 12, DrawMinPly: 80}
}

func (s Settings) Validate() error {
	openingMax := s.RandomPlies
	if openingMax == -1 {
		openingMax = 9
	}
	if s.Nodes == 0 || s.VerificationNodes == 0 || s.RandomPlies < -1 ||
		s.MaxPly < 1 || s.MaxPly > 65535 || openingMax >= s.MaxPly ||
		s.HashMB < engine.MinHashMB || s.HashMB > engine.MaxHashMB ||
		s.OpeningLimitCP < 0 || s.WinCP < 1 || s.WinPlies < 1 ||
		s.DrawCP < 0 || s.DrawPlies < 1 || s.DrawMinPly < 0 {
		return fmt.Errorf("invalid generator settings (positive node budgets, hash 1..1024 MB, maxply 1..65535 and greater than opening plies required)")
	}
	return nil
}

type Provenance struct{ EngineCommit, BinarySHA256 string }

// Load and hash one stream so the digest identifies the weights actually used.
// Validate before reserving output files; an invalid request never falls back.
func newSearcher(s Settings) (*engine.SearchEngine, string, error) {
	var network *engine.NGNN1Network
	var digest string
	if s.NetPath != "" {
		f, err := os.Open(s.NetPath)
		if err != nil {
			return nil, "", fmt.Errorf("load -net: %w", err)
		}
		defer f.Close()
		h := sha256.New()
		network, err = engine.ReadNGNN1(io.TeeReader(f, h))
		if err != nil {
			return nil, "", fmt.Errorf("load -net: %w", err)
		}
		digest = fmt.Sprintf("%x", h.Sum(nil))
	}
	e, err := engine.NewSearchEngineWithHash(s.HashMB)
	if err != nil {
		return nil, "", err
	}
	if network != nil {
		err = e.SelectNGNN1Evaluator(network)
	} else {
		err = e.SelectHCEEvaluator()
	}
	if err != nil {
		return nil, "", err
	}
	return e, digest, nil
}

type adjudicator struct{ winSign, winStreak, drawStreak int }

func (a *adjudicator) observe(s Settings, whiteScore, ply int) (ngnp.Result, bool) {
	sign := 0
	if whiteScore >= s.WinCP {
		sign = 1
	}
	if whiteScore <= -s.WinCP {
		sign = -1
	}
	if sign == 0 {
		a.winSign, a.winStreak = 0, 0
	} else if sign == a.winSign {
		a.winStreak++
	} else {
		a.winSign, a.winStreak = sign, 1
	}
	if a.winStreak >= s.WinPlies {
		if sign > 0 {
			return ngnp.Win, true
		}
		return ngnp.Loss, true
	}
	// The entire twelve-ply draw streak must be at or beyond ply eighty.
	if ply >= s.DrawMinPly && whiteScore >= -s.DrawCP && whiteScore <= s.DrawCP {
		a.drawStreak++
	} else {
		a.drawStreak = 0
	}
	return ngnp.Draw, a.drawStreak >= s.DrawPlies
}

func assignResult(records []ngnp.Record, result ngnp.Result) {
	for i := range records {
		records[i].Result = result
	}
}

func insufficient(pos *engine.Position) bool {
	if pos.Board.Pawns()|pos.Board.Rooks()|pos.Board.Queens() != 0 {
		return false
	}
	minor := pos.Board.Bishops() | pos.Board.Knights()
	if bits.OnesCount64(minor) <= 1 {
		return true
	}
	if pos.Board.Knights() != 0 {
		return false
	}
	// Any number of bishops on one square color cannot deliver checkmate.
	color := -1
	for minor != 0 {
		sq := bits.TrailingZeros64(minor)
		minor &= minor - 1
		c := (sq/8 + sq%8) % 2
		if color != -1 && color != c {
			return false
		}
		color = c
	}
	return true
}

func terminal(pos *engine.Position, ply, maxPly int) ([]engine.Move, ngnp.Result, string, bool) {
	moves := engine.GenerateLegalMoves(pos)
	if len(moves) == 0 {
		if pos.IsInCheck() {
			if pos.Turn() == engine.White {
				return moves, ngnp.Loss, "checkmate", true
			}
			return moves, ngnp.Win, "checkmate", true
		}
		return moves, ngnp.Draw, "stalemate", true
	}
	if pos.IsFIDEDrawRule() {
		if pos.HalfMoveClock >= 100 {
			return moves, ngnp.Draw, "fifty_move", true
		}
		return moves, ngnp.Draw, "repetition", true
	}
	if insufficient(pos) {
		return moves, ngnp.Draw, "insufficient_material", true
	}
	if ply >= maxPly {
		return moves, ngnp.Draw, "maxply", true
	}
	return moves, ngnp.Draw, "", false
}

var errUnscored = errors.New("no completed search iteration")

type searched struct {
	move       engine.Move
	whiteScore int
}

func search(e *engine.SearchEngine, pos *engine.Position, nodes uint64, meta *ngnp.Metadata) (searched, error) {
	e.SetMaxNodes(nodes)
	info := e.Search(pos, engine.MaximumDepth)
	meta.Searches++
	meta.SearchNodes += info.Nodes
	if info.Nodes > nodes || info.EffectiveThreads != 1 {
		return searched{}, fmt.Errorf("search contract: nodes=%d cap=%d threads=%d", info.Nodes, nodes, info.EffectiveThreads)
	}
	if info.Nodes < nodes {
		meta.EarlySearches++
	}
	if info.BestMove == engine.EmptyMove || info.BestScore <= -engine.INFINITY || info.BestScore >= engine.INFINITY {
		meta.UnscoredSearches++
		return searched{}, errUnscored
	}
	if !engine.IsLegalMove(pos, info.BestMove) {
		return searched{}, fmt.Errorf("search returned illegal move %s", info.BestMove.ToString())
	}
	score := info.BestScore
	if pos.Turn() == engine.Black {
		score = -score
	}
	return searched{info.BestMove, score}, nil
}

func sample(pos *engine.Position, found searched, ply int, meta *ngnp.Metadata) (ngnp.Record, bool, error) {
	var flags uint8
	if pos.IsInCheck() {
		flags |= ngnp.FlagInCheck
		meta.Filtered["in_check"]++
	}
	if found.move.IsCapture() || found.move.IsEnPassant() || found.move.CapturedPiece() != engine.NoPiece {
		flags |= ngnp.FlagCapture
		meta.Filtered["capture"]++
	}
	if found.move.PromoType() != engine.NoType {
		flags |= ngnp.FlagPromotion
		meta.Filtered["promotion"]++
	}
	if found.whiteScore >= engine.MATE_IN_MAX || found.whiteScore <= -engine.MATE_IN_MAX {
		flags |= ngnp.FlagClampedOrMate
		meta.Filtered["mate"]++
	}
	if flags != 0 {
		return ngnp.Record{}, false, nil
	}
	r, err := ngnp.FromPosition(pos, found.whiteScore, ngnp.Draw, uint16(ply), 0)
	return r, err == nil, err
}

func play(ctx context.Context, e *engine.SearchEngine, pos *engine.Position, ply int, s Settings, meta *ngnp.Metadata) ([]ngnp.Record, ngnp.Result, int, string, error) {
	records := make([]ngnp.Record, 0, s.MaxPly-ply)
	var adj adjudicator
	var pending bool
	var result ngnp.Result
	for {
		if err := ctx.Err(); err != nil {
			return nil, ngnp.Draw, ply, "", err
		}
		_, ruleResult, reason, done := terminal(pos, ply, s.MaxPly)
		if done {
			return records, ruleResult, ply, reason, nil
		}
		if pending {
			reason = "score_win"
			if result == ngnp.Draw {
				reason = "score_draw"
			}
			return records, result, ply, reason, nil
		}
		found, err := search(e, pos, s.Nodes, meta)
		if err != nil {
			return nil, ngnp.Draw, ply, "", err
		}
		r, keep, err := sample(pos, found, ply, meta)
		if err != nil {
			return nil, ngnp.Draw, ply, "", err
		}
		if keep {
			records = append(records, r)
		}
		result, pending = adj.observe(s, found.whiteScore, ply)
		pos.GameMakeMove(found.move)
		e.SetLastMovePlayed(found.move)
		ply++
	}
}

// Run drops the current game on cancellation at the next bounded-search
// boundary. Only games with known results enter the append-only shard.
func Run(ctx context.Context, out string, s Settings, provenance Provenance) (meta ngnp.Metadata, runErr error) {
	if err := s.Validate(); err != nil {
		return meta, err
	}
	if out == "" {
		return meta, fmt.Errorf("-out must name a new shard file")
	}
	e, netSHA256, err := newSearcher(s)
	if err != nil {
		return meta, err
	}
	settings, err := json.Marshal(s)
	if err != nil {
		return meta, err
	}
	meta = ngnp.Metadata{Format: ngnp.Format, EngineCommit: provenance.EngineCommit,
		BinarySHA256: provenance.BinarySHA256, NetSHA256: netSHA256, Settings: settings, Seed: s.Seed,
		StartedAt: time.Now().UTC(), Status: "running", Filtered: make(map[string]uint64), Terminations: make(map[string]uint64)}
	shard, err := ngnp.Create(out, meta)
	if err != nil {
		return meta, err
	}
	defer func() {
		if runErr != nil {
			meta.Status = "error"
		}
		runErr = errors.Join(runErr, shard.Close(&meta))
	}()
	random := rand.New(rand.NewSource(s.Seed))
	for s.Games == 0 || meta.Games < s.Games {
		if ctx.Err() != nil {
			meta.Status = "interrupted"
			return meta, nil
		}
		meta.Attempts++
		e.NewGame()
		pos, err := engine.ParseFEN(StartFEN)
		if err != nil {
			return meta, err
		}
		openingPlies := s.RandomPlies
		if openingPlies == -1 {
			openingPlies = 8 + random.Intn(2)
		}
		ply, rejected := 0, false
		for ply < openingPlies {
			if ctx.Err() != nil {
				meta.DroppedGames++
				meta.Status = "interrupted"
				return meta, nil
			}
			moves, _, _, done := terminal(pos, ply, s.MaxPly)
			if done {
				rejected = true
				break
			}
			move := moves[random.Intn(len(moves))]
			pos.GameMakeMove(move)
			e.SetLastMovePlayed(move)
			ply++
		}
		if _, _, _, done := terminal(pos, ply, s.MaxPly); done {
			rejected = true
		}
		if !rejected {
			found, err := search(e, pos, s.VerificationNodes, &meta)
			if errors.Is(err, errUnscored) {
				rejected = true
			} else if err != nil {
				return meta, err
			} else if found.whiteScore > s.OpeningLimitCP || found.whiteScore < -s.OpeningLimitCP {
				rejected = true
			}
		}
		if rejected {
			meta.RejectedOpenings++
			continue
		}
		records, result, plies, reason, err := play(ctx, e, pos, ply, s, &meta)
		if errors.Is(err, errUnscored) {
			meta.DroppedGames++
			continue
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			meta.DroppedGames++
			meta.Status = "interrupted"
			return meta, nil
		}
		if err != nil {
			meta.DroppedGames++
			return meta, err
		}
		assignResult(records, result)
		if err := shard.AppendGame(records); err != nil {
			return meta, err
		}
		meta.Games++
		meta.GamePlies += uint64(plies)
		meta.GameResults[result]++
		meta.Terminations[reason]++
	}
	meta.Status = "complete"
	return meta, nil
}
