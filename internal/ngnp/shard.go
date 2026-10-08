package ngnp

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"time"
)

// Metadata is the JSON sidecar contract. Counts describe committed whole games.
type Metadata struct {
	Format           string            `json:"format"`
	EngineCommit     string            `json:"engine_commit"`
	BinarySHA256     string            `json:"binary_sha256"`
	NetSHA256        string            `json:"net_sha256,omitempty"`
	Settings         json.RawMessage   `json:"settings"`
	Seed             int64             `json:"seed"`
	StartedAt        time.Time         `json:"started_at"`
	ClosedAt         time.Time         `json:"closed_at,omitempty"`
	ElapsedSeconds   float64           `json:"elapsed_seconds"`
	Closed           bool              `json:"closed"`
	Status           string            `json:"status"`
	Games            uint64            `json:"games"`
	Positions        uint64            `json:"positions"`
	GamePlies        uint64            `json:"game_plies"`
	GameResults      [3]uint64         `json:"game_results"`
	Attempts         uint64            `json:"attempts"`
	RejectedOpenings uint64            `json:"rejected_openings"`
	DroppedGames     uint64            `json:"dropped_games"`
	Searches         uint64            `json:"searches"`
	SearchNodes      uint64            `json:"search_nodes"`
	EarlySearches    uint64            `json:"early_searches"`
	UnscoredSearches uint64            `json:"unscored_searches"`
	Filtered         map[string]uint64 `json:"filtered"`
	Terminations     map[string]uint64 `json:"terminations"`
	Bytes            uint64            `json:"bytes"`
	SHA256           string            `json:"sha256"`
}

type Shard struct {
	data, sidecar *os.File
	digest        hash.Hash
	positions     uint64
	closed        bool
	closeErr      error
}

func Create(path string, meta Metadata) (*Shard, error) {
	// Reserve both names. An existing sidecar also forbids reusing a data name.
	sidecar, err := os.OpenFile(path+".json", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	data, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		sidecar.Close()
		os.Remove(path + ".json") // Only the reservation created by this call.
		return nil, err
	}
	s := &Shard{data: data, sidecar: sidecar, digest: sha256.New()}
	meta.Format, meta.Closed = Format, false
	if err := s.writeMetadata(meta); err != nil {
		data.Close()
		sidecar.Close()
		os.Remove(path)
		os.Remove(path + ".json")
		return nil, err
	}
	return s, nil
}

// AppendGame encodes before writing. A failed append is rolled back to its
// original length, so orderly close never publishes a partial game or record.
func (s *Shard) AppendGame(records []Record) error {
	if s.closed {
		return fmt.Errorf("shard is closed")
	}
	buf := make([]byte, 0, len(records)*Size)
	for _, r := range records {
		raw, err := Encode(r)
		if err != nil {
			return err
		}
		buf = append(buf, raw[:]...)
	}
	n, err := s.data.Write(buf)
	if err == nil && n != len(buf) {
		err = io.ErrShortWrite
	}
	if err != nil {
		return errors.Join(err, s.data.Truncate(int64(s.positions*Size)))
	}
	s.digest.Write(buf)
	s.positions += uint64(len(records))
	return nil
}

func (s *Shard) writeMetadata(meta Metadata) error {
	raw, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	if _, err := s.sidecar.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if err := s.sidecar.Truncate(0); err != nil {
		return err
	}
	if _, err := s.sidecar.Write(append(raw, '\n')); err != nil {
		return err
	}
	return s.sidecar.Sync()
}

func (s *Shard) Close(meta *Metadata) error {
	if s.closed {
		return s.closeErr
	}
	s.closed = true
	dataErr := errors.Join(s.data.Sync(), s.data.Close())
	meta.Format, meta.Positions, meta.Bytes = Format, s.positions, s.positions*Size
	meta.SHA256 = hex.EncodeToString(s.digest.Sum(nil))
	meta.Closed = dataErr == nil
	meta.ClosedAt = time.Now().UTC()
	meta.ElapsedSeconds = meta.ClosedAt.Sub(meta.StartedAt).Seconds()
	s.closeErr = errors.Join(dataErr, s.writeMetadata(*meta), s.sidecar.Close())
	return s.closeErr
}
