// Package k4label implements the frozen Stockfish-18 teacher-label contract
// for NGN's owned K4 network. It deliberately produces an auditable JSONL
// intermediate; conversion to Bullet records is a separate independently
// checked stage.
package k4label

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/bits"
	"os"
	"strings"

	"github.com/ehrlich-b/ngn/engine"
)

const (
	InputSchema          = "ngn-k4-label-input-v2"
	OutputSchema         = "ngn-k4-label-output-v2"
	ContractVersion      = "sf18-5kn-material-inversion-v2"
	MaximumShardRecords  = 100_000
	MaximumInputLine     = 1 << 20
	MaximumInputBytes    = 256 << 20
	TeacherNodes         = 5_000
	TeacherThreads       = 1
	TeacherHashMiB       = 16
	TeacherMultiPV       = 1
	MinimumTeacherDepth  = 4
	TeacherTimeoutMillis = 2_000
	MaximumTargetScore   = 10_000
	TargetSigmoidScale   = 400.0
	TeacherSourceCommit  = "cb3d4ee9b47d0c5aae855b12379378ea1439675c"
	TeacherBigNetworkSHA = "c288c895ea924429ea9092e3f36b2b3c1f00f2a3a4c759ff7e57e79e3b43e4a7"
	TeacherSmallNetSHA   = "37f18f62d772f3107e1d6aaca3898c130c3c86f2ab63e6555fbbca20635a899d"
)

var ErrContract = errors.New("NGN K4 label contract violation")

type InputHeader struct {
	Type                 string `json:"type"`
	Schema               string `json:"schema"`
	ShardID              string `json:"shard_id"`
	Split                string `json:"split"`
	SourceManifestSHA256 string `json:"source_manifest_sha256"`
	RecordCount          int    `json:"record_count"`
}

type InputPosition struct {
	Type              string `json:"type"`
	ID                string `json:"id"`
	FEN               string `json:"fen"`
	SourceMove        string `json:"source_move"`
	EncodedChain      uint64 `json:"encoded_chain"`
	ChainEntry        uint32 `json:"chain_entry"`
	SourcePosition    uint64 `json:"source_position"`
	K4InputSHA256     string `json:"k4_input_sha256"`
	SourceArchiveSHA  string `json:"source_archive_sha256"`
	SourceManifestRef string `json:"source_manifest_ref,omitempty"`
}

type InputShard struct {
	Header    InputHeader
	Positions []InputPosition
	SHA256    string
}

type SearchConfig struct {
	Nodes         int    `json:"nodes"`
	Threads       int    `json:"threads"`
	HashMiB       int    `json:"hash_mib"`
	MultiPV       int    `json:"multipv"`
	MinimumDepth  int    `json:"minimum_depth"`
	TimeoutMillis int    `json:"timeout_millis"`
	SyzygyPath    string `json:"syzygy_path"`
}

func FrozenSearchConfig() SearchConfig {
	return SearchConfig{
		Nodes: TeacherNodes, Threads: TeacherThreads, HashMiB: TeacherHashMiB,
		MultiPV: TeacherMultiPV, MinimumDepth: MinimumTeacherDepth,
		TimeoutMillis: TeacherTimeoutMillis, SyzygyPath: "<empty>",
	}
}

type TeacherProvenance struct {
	Name                  string   `json:"name"`
	ExecutablePath        string   `json:"executable_path"`
	ExecutableSHA256      string   `json:"executable_sha256"`
	SourceCommit          string   `json:"source_commit"`
	BigNetworkSHA256      string   `json:"big_network_sha256"`
	SmallNetworkSHA256    string   `json:"small_network_sha256"`
	Command               []string `json:"command"`
	HandshakeOptionDigest string   `json:"handshake_option_sha256"`
}

type OutputHeader struct {
	Type            string            `json:"type"`
	Schema          string            `json:"schema"`
	ContractVersion string            `json:"contract_version"`
	InputSHA256     string            `json:"input_sha256"`
	Input           InputHeader       `json:"input"`
	Teacher         TeacherProvenance `json:"teacher"`
	Search          SearchConfig      `json:"search"`
	ScoreContract   string            `json:"score_contract"`
}

type SearchResult struct {
	BestMove string
	PVMove   string
	CP       int
	Depth    int
	SelDepth int
	Nodes    uint64
	InfoLine string
}

type LabelRecord struct {
	Type                 string  `json:"type"`
	ID                   string  `json:"id"`
	K4InputSHA256        string  `json:"k4_input_sha256"`
	Split                string  `json:"split"`
	OriginalFEN          string  `json:"original_fen"`
	ResetFEN             string  `json:"reset_fen"`
	SourceMove           string  `json:"source_move"`
	BestMove             string  `json:"best_move,omitempty"`
	PVMove               string  `json:"pv_move,omitempty"`
	UCICP                int     `json:"uci_cp,omitempty"`
	NGNScore             int16   `json:"ngn_score,omitempty"`
	Target               float64 `json:"target,omitempty"`
	MaterialUnits        int     `json:"material_units,omitempty"`
	MaterialNormalizer   float64 `json:"material_normalizer,omitempty"`
	Depth                int     `json:"depth,omitempty"`
	SelDepth             int     `json:"seldepth,omitempty"`
	Nodes                uint64  `json:"nodes,omitempty"`
	InfoLineSHA256       string  `json:"info_line_sha256,omitempty"`
	TeacherInfoLine      string  `json:"teacher_info_line,omitempty"`
	Status               string  `json:"status"`
	RejectionReason      string  `json:"rejection_reason,omitempty"`
	PreviousRecordSHA256 string  `json:"previous_record_sha256"`
	RecordSHA256         string  `json:"record_sha256"`
}

type OutputFooter struct {
	Type                 string         `json:"type"`
	Schema               string         `json:"schema"`
	Records              int            `json:"records"`
	Accepted             int            `json:"accepted"`
	Rejected             int            `json:"rejected"`
	Rejections           map[string]int `json:"rejections"`
	LastRecordSHA256     string         `json:"last_record_sha256"`
	RecoveredPartialByte int64          `json:"recovered_partial_bytes"`
}

func validLowerHexSHA(value string) bool {
	if len(value) != sha256.Size*2 || strings.ToLower(value) != value {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func validSplit(split string) bool {
	switch split {
	case "train", "validation", "calibration", "reserved-test":
		return true
	default:
		return false
	}
}

func validateHeader(header InputHeader) error {
	if header.Type != "header" || header.Schema != InputSchema {
		return fmt.Errorf("%w: input header type/schema %q/%q", ErrContract, header.Type, header.Schema)
	}
	if header.ShardID == "" || strings.ContainsAny(header.ShardID, "\r\n\x00") {
		return fmt.Errorf("%w: invalid shard id", ErrContract)
	}
	if !validSplit(header.Split) {
		return fmt.Errorf("%w: invalid split %q", ErrContract, header.Split)
	}
	if !validLowerHexSHA(header.SourceManifestSHA256) {
		return fmt.Errorf("%w: invalid source manifest SHA-256", ErrContract)
	}
	if header.RecordCount < 1 || header.RecordCount > MaximumShardRecords {
		return fmt.Errorf("%w: record count %d outside 1..%d", ErrContract, header.RecordCount, MaximumShardRecords)
	}
	return nil
}

func validatePosition(record InputPosition) error {
	if record.Type != "position" || !validLowerHexSHA(record.ID) ||
		!validLowerHexSHA(record.K4InputSHA256) || !validLowerHexSHA(record.SourceArchiveSHA) {
		return fmt.Errorf("%w: position identity fields are invalid", ErrContract)
	}
	if record.FEN == "" || strings.ContainsAny(record.FEN, "\r\n\x00") {
		return fmt.Errorf("%w: position %s has invalid FEN text", ErrContract, record.ID)
	}
	position, err := engine.ParseFEN(record.FEN)
	if err != nil {
		return fmt.Errorf("%w: position %s FEN: %v", ErrContract, record.ID, err)
	}
	fields := strings.Fields(record.FEN)
	canonical := strings.Fields(engine.GenerateFEN(position))
	if len(fields) != 6 || len(canonical) != 6 || strings.Join(fields[:4], " ") != strings.Join(canonical[:4], " ") {
		return fmt.Errorf("%w: position %s state is not canonical/preservable", ErrContract, record.ID)
	}
	occupied := uint64(0)
	for piece := engine.WhitePawn; piece <= engine.BlackKing; piece++ {
		occupied |= position.Board.GetBitboardOf(piece)
	}
	if bits.OnesCount64(occupied) > 32 ||
		bits.OnesCount64(position.Board.GetBitboardOf(engine.WhiteKing)) != 1 ||
		bits.OnesCount64(position.Board.GetBitboardOf(engine.BlackKing)) != 1 {
		return fmt.Errorf("%w: position %s violates legal-board structural bounds", ErrContract, record.ID)
	}
	if position.IsInCheck() {
		return fmt.Errorf("%w: position %s is in check", ErrContract, record.ID)
	}
	k4InputSHA, err := K4InputSHA256(position)
	if err != nil || record.K4InputSHA256 != k4InputSHA {
		return fmt.Errorf("%w: position %s K4 input identity mismatch", ErrContract, record.ID)
	}
	move, err := engine.ParseUCIMove(position, record.SourceMove)
	if err != nil || move.IsCapture() || move.PromoType() != engine.NoType || move.IsCastle() || move.IsEnPassant() {
		return fmt.Errorf("%w: position %s source move %q is not a legal normal quiet move", ErrContract, record.ID, record.SourceMove)
	}
	return nil
}

func unmarshalStrict(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}

// ReadInputShard validates the whole bounded JSONL shard before a teacher is
// started. This makes corrupt or semantically ineligible sampler output fatal,
// rather than silently shrinking the requested corpus.
func ReadInputShard(path string) (InputShard, error) {
	file, err := os.Open(path)
	if err != nil {
		return InputShard{}, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return InputShard{}, err
	}
	if info.Size() > MaximumInputBytes {
		return InputShard{}, fmt.Errorf("%w: input is %d bytes, maximum %d", ErrContract, info.Size(), MaximumInputBytes)
	}
	limited := io.LimitReader(file, MaximumInputBytes+1)
	hasher := sha256.New()
	scanner := bufio.NewScanner(io.TeeReader(limited, hasher))
	scanner.Buffer(make([]byte, 64*1024), MaximumInputLine)
	var shard InputShard
	seen := make(map[string]struct{})
	line := 0
	for scanner.Scan() {
		line++
		if line == 1 {
			if err := unmarshalStrict(scanner.Bytes(), &shard.Header); err != nil {
				return InputShard{}, fmt.Errorf("input line 1: %w", err)
			}
			if err := validateHeader(shard.Header); err != nil {
				return InputShard{}, err
			}
			continue
		}
		if len(shard.Positions) >= MaximumShardRecords {
			return InputShard{}, fmt.Errorf("%w: too many input positions", ErrContract)
		}
		var record InputPosition
		if err := unmarshalStrict(scanner.Bytes(), &record); err != nil {
			return InputShard{}, fmt.Errorf("input line %d: %w", line, err)
		}
		if err := validatePosition(record); err != nil {
			return InputShard{}, fmt.Errorf("input line %d: %w", line, err)
		}
		if _, exists := seen[record.ID]; exists {
			return InputShard{}, fmt.Errorf("%w: duplicate position id %s", ErrContract, record.ID)
		}
		seen[record.ID] = struct{}{}
		shard.Positions = append(shard.Positions, record)
	}
	if err := scanner.Err(); err != nil {
		return InputShard{}, err
	}
	if line == 0 {
		return InputShard{}, fmt.Errorf("%w: empty input", ErrContract)
	}
	if len(shard.Positions) != shard.Header.RecordCount {
		return InputShard{}, fmt.Errorf("%w: decoded %d positions, header declares %d", ErrContract, len(shard.Positions), shard.Header.RecordCount)
	}
	shard.SHA256 = hex.EncodeToString(hasher.Sum(nil))
	return shard, nil
}

func resetClockFEN(fen string) (string, *engine.Position, error) {
	position, err := engine.ParseFEN(fen)
	if err != nil {
		return "", nil, err
	}
	position.HalfMoveClock = 0
	return engine.GenerateFEN(position), position, nil
}

func materialUnits(position *engine.Position) int {
	return bits.OnesCount64(position.Board.Pawns()) +
		3*bits.OnesCount64(position.Board.Knights()) +
		3*bits.OnesCount64(position.Board.Bishops()) +
		5*bits.OnesCount64(position.Board.Rooks()) +
		9*bits.OnesCount64(position.Board.Queens())
}

// ScoreTarget inverts Stockfish 18's material-normalized UCI score into NGN
// score units, then applies the frozen natural sigmoid used by the trainer.
func ScoreTarget(position *engine.Position, uciCP int) (score int, target, normalizer float64, material int, err error) {
	if position == nil {
		return 0, 0, 0, 0, fmt.Errorf("%w: nil score position", ErrContract)
	}
	material = materialUnits(position)
	clamped := material
	if clamped < 17 {
		clamped = 17
	} else if clamped > 78 {
		clamped = 78
	}
	m := float64(clamped) / 58.0
	normalizer = ((-72.32565836*m+185.93832038)*m-144.58862193)*m + 416.44950446
	value := float64(uciCP) * normalizer / 208.0
	if math.IsNaN(value) || math.IsInf(value, 0) || math.Abs(value) > 1_000_000_000 {
		return 0, 0, normalizer, material, fmt.Errorf("%w: nonfinite/out-of-range score inversion", ErrContract)
	}
	score = int(math.Round(value))
	target = 1.0 / (1.0 + math.Exp(-float64(score)/TargetSigmoidScale))
	return score, target, normalizer, material, nil
}

func buildLabelRecord(input InputPosition, split string, result SearchResult, rejection string) (LabelRecord, error) {
	resetFEN, position, err := resetClockFEN(input.FEN)
	if err != nil {
		return LabelRecord{}, err
	}
	record := LabelRecord{
		Type: "label", ID: input.ID, K4InputSHA256: input.K4InputSHA256, Split: split, OriginalFEN: input.FEN,
		ResetFEN: resetFEN, SourceMove: input.SourceMove, BestMove: result.BestMove,
		PVMove: result.PVMove,
		UCICP:  result.CP, Depth: result.Depth, SelDepth: result.SelDepth, Nodes: result.Nodes,
		Status: "rejected", RejectionReason: rejection,
	}
	if result.InfoLine != "" {
		digest := sha256.Sum256([]byte(result.InfoLine))
		record.InfoLineSHA256 = hex.EncodeToString(digest[:])
		record.TeacherInfoLine = result.InfoLine
	}
	if rejection != "" {
		return record, nil
	}
	move, err := engine.ParseUCIMove(position, result.BestMove)
	if err != nil {
		record.RejectionReason = "illegal-bestmove"
		return record, nil
	}
	if move.IsCapture() || move.PromoType() != engine.NoType {
		record.RejectionReason = "nonquiet-bestmove"
		return record, nil
	}
	if result.PVMove == "" || result.PVMove != result.BestMove {
		record.RejectionReason = "pv-bestmove-mismatch"
		return record, nil
	}
	score, target, normalizer, material, err := ScoreTarget(position, result.CP)
	if err != nil {
		return LabelRecord{}, err
	}
	record.MaterialUnits = material
	record.MaterialNormalizer = normalizer
	if score < -MaximumTargetScore || score > MaximumTargetScore {
		record.RejectionReason = "saturated-tail"
		return record, nil
	}
	record.NGNScore = int16(score)
	record.Target = target
	record.Status = "accepted"
	record.RejectionReason = ""
	return record, nil
}
