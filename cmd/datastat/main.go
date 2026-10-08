package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ehrlich-b/ngn/internal/ngnp"
)

type outcomes struct{ Loss, Draw, Win uint64 }

func (o *outcomes) add(counts [3]uint64) {
	o.Loss += counts[0]
	o.Draw += counts[1]
	o.Win += counts[2]
}

// Keep a bounded, deterministic hash sample of distinct board/stm keys. When
// it fills, halve the sample and retain occurrence counts for surviving keys.
type duplicates struct {
	counts map[uint64]uint64
	mask   uint64
}

func (d *duplicates) add(r ngnp.Record) {
	var key [25]byte
	binary.LittleEndian.PutUint64(key[:8], r.Occupancy)
	copy(key[8:24], r.Pieces[:])
	key[24] = r.STM
	digest := sha256.Sum256(key[:])
	h := binary.LittleEndian.Uint64(digest[:8])
	if h&d.mask != 0 {
		return
	}
	d.counts[h]++
	if len(d.counts) > 100000 {
		d.mask = d.mask*2 + 1
		for hash := range d.counts {
			if hash&d.mask != 0 {
				delete(d.counts, hash)
			}
		}
	}
}

type statistics struct {
	Format                 string            `json:"format"`
	Files                  int               `json:"files"`
	Records                uint64            `json:"records"`
	Bytes                  uint64            `json:"bytes"`
	BytesPerPosition       float64           `json:"bytes_per_position"`
	RecordResults          outcomes          `json:"record_results_white"`
	ScoreHistogram         map[string]uint64 `json:"score_histogram_cp_100"`
	PieceHistogram         map[string]uint64 `json:"piece_count_histogram"`
	PlyHistogram           map[string]uint64 `json:"ply_histogram_20"`
	MeanScore              float64           `json:"mean_score_cp"`
	MeanRecordPly          float64           `json:"mean_record_ply"`
	FlaggedRecords         uint64            `json:"flagged_records"`
	DuplicateRate          float64           `json:"duplicate_position_rate_estimate"`
	DuplicateSampleRecords uint64            `json:"duplicate_sample_records"`
	DuplicateSampleUnique  uint64            `json:"duplicate_sample_unique"`
	DuplicateSampleBits    int               `json:"duplicate_sample_bits"`
	CompletedGames         uint64            `json:"completed_games"`
	GameResults            outcomes          `json:"game_results_white"`
	AverageGamePlies       float64           `json:"average_game_plies"`
	RejectedOpenings       uint64            `json:"rejected_openings"`
	DroppedGames           uint64            `json:"dropped_games"`
	EarlySearches          uint64            `json:"early_searches"`
	UnscoredSearches       uint64            `json:"unscored_searches"`
	Terminations           map[string]uint64 `json:"terminations"`
	Filtered               map[string]uint64 `json:"filtered"`
	WallSeconds            float64           `json:"wall_seconds"`
	PositionsPerHour       float64           `json:"positions_per_hour"`
	GamesPerHour           float64           `json:"games_per_hour"`
	ValidatedSidecars      int               `json:"validated_sidecars"`
}

func files(paths []string) ([]string, error) {
	seen := make(map[string]bool)
	for _, path := range paths {
		absolute, err := filepath.Abs(path)
		if err != nil {
			return nil, err
		}
		st, err := os.Stat(absolute)
		if err != nil {
			return nil, err
		}
		if !st.IsDir() {
			seen[absolute] = true
			continue
		}
		if err := filepath.WalkDir(absolute, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !entry.IsDir() && strings.HasSuffix(path, ".ngnp") {
				seen[path] = true
			}
			return nil
		}); err != nil {
			return nil, err
		}
	}
	list := make([]string, 0, len(seen))
	for path := range seen {
		list = append(list, path)
	}
	sort.Strings(list)
	if len(list) == 0 {
		return nil, fmt.Errorf("no shard files found")
	}
	return list, nil
}

func inspect(paths []string, requireSidecar bool, wallSeconds float64) (statistics, error) {
	s := statistics{Format: ngnp.Format, ScoreHistogram: make(map[string]uint64), PieceHistogram: make(map[string]uint64),
		PlyHistogram: make(map[string]uint64), Terminations: make(map[string]uint64), Filtered: make(map[string]uint64)}
	d := duplicates{counts: make(map[uint64]uint64)}
	var scoreSum int64
	var plySum, gamePlies uint64
	var first, last time.Time
	list, err := files(paths)
	if err != nil {
		return s, err
	}
	for _, path := range list {
		f, err := os.Open(path)
		if err != nil {
			return s, err
		}
		h := sha256.New()
		startRecords := s.Records
		err = ngnp.Iterate(io.TeeReader(bufio.NewReaderSize(f, 256*1024), h), func(r ngnp.Record) error {
			s.Records++
			var counts [3]uint64
			counts[r.Result] = 1
			s.RecordResults.add(counts)
			bin := int(math.Floor(float64(r.Score)/100)) * 100
			s.ScoreHistogram[fmt.Sprintf("%d..%d", bin, bin+99)]++
			s.PieceHistogram[strconv.Itoa(r.PieceCount())]++
			ply := int(r.Ply) / 20 * 20
			s.PlyHistogram[fmt.Sprintf("%d..%d", ply, ply+19)]++
			scoreSum += int64(r.Score)
			plySum += uint64(r.Ply)
			if r.Flags != 0 {
				s.FlaggedRecords++
			}
			d.add(r)
			return nil
		})
		closeErr := f.Close()
		if err != nil {
			return s, fmt.Errorf("%s: %w", path, err)
		}
		if closeErr != nil {
			return s, closeErr
		}
		s.Files++
		raw, err := os.ReadFile(path + ".json")
		if os.IsNotExist(err) && !requireSidecar {
			continue
		}
		if err != nil {
			return s, err
		}
		var meta ngnp.Metadata
		if err := json.Unmarshal(raw, &meta); err != nil {
			return s, fmt.Errorf("%s.json: %w", path, err)
		}
		count := s.Records - startRecords
		if meta.Format != ngnp.Format || !meta.Closed || (meta.Status != "complete" && meta.Status != "interrupted") || meta.Positions != count ||
			meta.Bytes != count*ngnp.Size || meta.SHA256 != hex.EncodeToString(h.Sum(nil)) ||
			meta.GameResults[0]+meta.GameResults[1]+meta.GameResults[2] != meta.Games {
			return s, fmt.Errorf("%s.json: incomplete, failed, or inconsistent sidecar/checksum", path)
		}
		if len(meta.EngineCommit) != 40 || len(meta.BinarySHA256) != 64 || meta.StartedAt.IsZero() || meta.ClosedAt.Before(meta.StartedAt) {
			return s, fmt.Errorf("%s.json: invalid provenance or timestamps", path)
		}
		for _, digest := range []string{meta.EngineCommit, meta.BinarySHA256} {
			if _, err := hex.DecodeString(digest); err != nil {
				return s, fmt.Errorf("%s.json: invalid provenance hash", path)
			}
		}
		s.ValidatedSidecars++
		if first.IsZero() || meta.StartedAt.Before(first) {
			first = meta.StartedAt
		}
		if last.IsZero() || meta.ClosedAt.After(last) {
			last = meta.ClosedAt
		}
		s.CompletedGames += meta.Games
		gamePlies += meta.GamePlies
		s.GameResults.add(meta.GameResults)
		s.RejectedOpenings += meta.RejectedOpenings
		s.DroppedGames += meta.DroppedGames
		s.EarlySearches += meta.EarlySearches
		s.UnscoredSearches += meta.UnscoredSearches
		for reason, count := range meta.Terminations {
			s.Terminations[reason] += count
		}
		for reason, count := range meta.Filtered {
			s.Filtered[reason] += count
		}
	}
	s.Bytes = s.Records * ngnp.Size
	if s.Records != 0 {
		s.BytesPerPosition = float64(s.Bytes) / float64(s.Records)
		s.MeanScore = float64(scoreSum) / float64(s.Records)
		s.MeanRecordPly = float64(plySum) / float64(s.Records)
	}
	for _, count := range d.counts {
		s.DuplicateSampleRecords += count
	}
	s.DuplicateSampleUnique = uint64(len(d.counts))
	if s.DuplicateSampleRecords > 0 {
		s.DuplicateRate = 1 - float64(s.DuplicateSampleUnique)/float64(s.DuplicateSampleRecords)
	}
	for mask := d.mask; mask != 0; mask >>= 1 {
		s.DuplicateSampleBits++
	}
	if s.CompletedGames > 0 {
		s.AverageGamePlies = float64(gamePlies) / float64(s.CompletedGames)
	}
	s.WallSeconds = wallSeconds
	if s.WallSeconds == 0 && !first.IsZero() {
		s.WallSeconds = last.Sub(first).Seconds()
	}
	if s.WallSeconds > 0 {
		s.PositionsPerHour = float64(s.Records) * 3600 / s.WallSeconds
		s.GamesPerHour = float64(s.CompletedGames) * 3600 / s.WallSeconds
	}
	return s, nil
}

func run() error {
	require := flag.Bool("require-sidecar", true, "require closed sidecars and verify their checksums/counts")
	wall := flag.Float64("wall-seconds", 0, "throughput denominator; 0 uses sidecar timestamp span")
	flag.Parse()
	if flag.NArg() == 0 || *wall < 0 || math.IsNaN(*wall) || math.IsInf(*wall, 0) {
		return fmt.Errorf("usage: datastat [flags] shard-or-directory ...")
	}
	s, err := inspect(flag.Args(), *require, *wall)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(s)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "datastat:", err)
		os.Exit(1)
	}
}
