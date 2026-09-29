// Package k4pack independently verifies NGN K4 teacher-label shards and
// converts accepted positions to Bullet's fixed 32-byte ChessBoard format.
package k4pack

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/ehrlich-b/ngn/engine"
)

const (
	InputSchema          = "ngn-k4-label-input-v2"
	LabelSchema          = "ngn-k4-label-output-v2"
	ContractVersion      = "sf18-5kn-material-inversion-v2"
	ReceiptSchema        = "ngn-k4-pack-receipt-v2"
	TeacherSourceCommit  = "cb3d4ee9b47d0c5aae855b12379378ea1439675c"
	TeacherBigNetworkSHA = "c288c895ea924429ea9092e3f36b2b3c1f00f2a3a4c759ff7e57e79e3b43e4a7"
	TeacherSmallNetSHA   = "37f18f62d772f3107e1d6aaca3898c130c3c86f2ab63e6555fbbca20635a899d"
	ScoreContract        = "c=round(uci_cp*a(material)/208); target=1/(1+exp(-c/400)); STM; root_halfmove=0"
	K4InputKeyContract   = "sha256(ngn-k4-input-v1\\0 || stm_count:u16le || sorted_stm_rows:u16le[] || ntm_count:u16le || sorted_ntm_rows:u16le[] || material_head:u8)"
	RecordBytes          = 32
	maximumLineBytes     = 1 << 20
	maximumInputBytes    = 512 << 20
	maximumRecords       = 100_000
)

var ErrContract = errors.New("NGN K4 pack contract violation")

type inputHeader struct {
	Type                 string `json:"type"`
	Schema               string `json:"schema"`
	ShardID              string `json:"shard_id"`
	Split                string `json:"split"`
	SourceManifestSHA256 string `json:"source_manifest_sha256"`
	RecordCount          int    `json:"record_count"`
}

type searchConfig struct {
	Nodes         int    `json:"nodes"`
	Threads       int    `json:"threads"`
	HashMiB       int    `json:"hash_mib"`
	MultiPV       int    `json:"multipv"`
	MinimumDepth  int    `json:"minimum_depth"`
	TimeoutMillis int    `json:"timeout_millis"`
	SyzygyPath    string `json:"syzygy_path"`
}

type teacherProvenance struct {
	Name                  string   `json:"name"`
	ExecutablePath        string   `json:"executable_path"`
	ExecutableSHA256      string   `json:"executable_sha256"`
	SourceCommit          string   `json:"source_commit"`
	BigNetworkSHA256      string   `json:"big_network_sha256"`
	SmallNetworkSHA256    string   `json:"small_network_sha256"`
	Command               []string `json:"command"`
	HandshakeOptionDigest string   `json:"handshake_option_sha256"`
}

type outputHeader struct {
	Type            string            `json:"type"`
	Schema          string            `json:"schema"`
	ContractVersion string            `json:"contract_version"`
	InputSHA256     string            `json:"input_sha256"`
	Input           inputHeader       `json:"input"`
	Teacher         teacherProvenance `json:"teacher"`
	Search          searchConfig      `json:"search"`
	ScoreContract   string            `json:"score_contract"`
}

type labelRecord struct {
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

type outputFooter struct {
	Type                 string         `json:"type"`
	Schema               string         `json:"schema"`
	Records              int            `json:"records"`
	Accepted             int            `json:"accepted"`
	Rejected             int            `json:"rejected"`
	Rejections           map[string]int `json:"rejections"`
	LastRecordSHA256     string         `json:"last_record_sha256"`
	RecoveredPartialByte int64          `json:"recovered_partial_bytes"`
}

// Receipt binds the verified label shard, exact Bullet output, record count,
// ordering and model inputs. ResultConstant is one, Bullet's draw encoding;
// training uses score targets only.
type Receipt struct {
	Schema                    string         `json:"schema"`
	ContractVersion           string         `json:"contract_version"`
	Command                   []string       `json:"command"`
	InputPath                 string         `json:"input_path"`
	InputBytes                int64          `json:"input_bytes"`
	InputSHA256               string         `json:"input_sha256"`
	InputShardSHA256          string         `json:"input_shard_sha256"`
	ShardID                   string         `json:"shard_id"`
	Split                     string         `json:"split"`
	OutputPath                string         `json:"output_path"`
	OutputBytes               int64          `json:"output_bytes"`
	OutputSHA256              string         `json:"output_sha256"`
	Records                   int            `json:"records"`
	Accepted                  int            `json:"accepted"`
	Rejected                  int            `json:"rejected"`
	Rejections                map[string]int `json:"rejections"`
	RecordBytes               int            `json:"record_bytes"`
	ResultConstant            int            `json:"result_constant"`
	ScorePerspective          string         `json:"score_perspective"`
	K4InputKeyContract        string         `json:"k4_input_key_contract"`
	AcceptedStreamSHA256      string         `json:"accepted_stream_sha256"`
	TeacherExecutableSHA256   string         `json:"teacher_executable_sha256"`
	TeacherSourceCommit       string         `json:"teacher_source_commit"`
	TeacherBigNetworkSHA256   string         `json:"teacher_big_network_sha256"`
	TeacherSmallNetworkSHA256 string         `json:"teacher_small_network_sha256"`
}

type envelope struct {
	Type string `json:"type"`
}

type boardRecord struct {
	occ           uint64
	pcs           [16]byte
	score         int16
	result        byte
	ksq           byte
	oppKsq        byte
	extra         [3]byte
	k4InputSHA256 [32]byte
}

var k4KingBuckets = [64]int{
	1, 1, 0, 0, 0, 0, 1, 1,
	2, 2, 2, 2, 2, 2, 2, 2,
	3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3,
}

func validSHA(value string) bool {
	if len(value) != 64 || strings.ToLower(value) != value {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func validSplit(value string) bool {
	switch value {
	case "train", "validation", "calibration", "reserved-test":
		return true
	default:
		return false
	}
}

func decodeStrict(data []byte, target any) error {
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

func canonicalDigest(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), nil
}

func recordDigest(record labelRecord) (string, error) {
	record.RecordSHA256 = ""
	return canonicalDigest(record)
}

func validateHeader(header outputHeader) error {
	wantSearch := searchConfig{Nodes: 5_000, Threads: 1, HashMiB: 16, MultiPV: 1, MinimumDepth: 4, TimeoutMillis: 2_000, SyzygyPath: "<empty>"}
	if header.Type != "header" || header.Schema != LabelSchema || header.ContractVersion != ContractVersion || header.ScoreContract != ScoreContract {
		return fmt.Errorf("%w: label header contract mismatch", ErrContract)
	}
	if header.Input.Type != "header" || header.Input.Schema != InputSchema || header.Input.ShardID == "" ||
		strings.ContainsAny(header.Input.ShardID, "\r\n\x00") || !validSplit(header.Input.Split) ||
		!validSHA(header.Input.SourceManifestSHA256) || header.Input.RecordCount < 1 || header.Input.RecordCount > maximumRecords ||
		!validSHA(header.InputSHA256) {
		return fmt.Errorf("%w: embedded sampler header mismatch", ErrContract)
	}
	if header.Teacher.Name != "Stockfish 18" || header.Teacher.ExecutablePath == "" ||
		!validSHA(header.Teacher.ExecutableSHA256) || header.Teacher.SourceCommit != TeacherSourceCommit ||
		header.Teacher.BigNetworkSHA256 != TeacherBigNetworkSHA || header.Teacher.SmallNetworkSHA256 != TeacherSmallNetSHA ||
		len(header.Teacher.Command) == 0 || !validSHA(header.Teacher.HandshakeOptionDigest) {
		return fmt.Errorf("%w: teacher provenance mismatch", ErrContract)
	}
	if header.Search != wantSearch {
		return fmt.Errorf("%w: search configuration mismatch", ErrContract)
	}
	return nil
}

func resetPosition(fen string) (string, *engine.Position, error) {
	position, err := engine.ParseFEN(fen)
	if err != nil {
		return "", nil, err
	}
	fields := strings.Fields(fen)
	canonical := strings.Fields(engine.GenerateFEN(position))
	if len(fields) != 6 || len(canonical) != 6 || strings.Join(fields[:4], " ") != strings.Join(canonical[:4], " ") {
		return "", nil, fmt.Errorf("noncanonical or unpreservable FEN")
	}
	position.HalfMoveClock = 0
	return engine.GenerateFEN(position), position, nil
}

func validateQuietMove(position *engine.Position, text string, source bool) error {
	move, err := engine.ParseUCIMove(position, text)
	if err != nil || move.IsCapture() || move.PromoType() != engine.NoType ||
		(source && (move.IsCastle() || move.IsEnPassant())) {
		return fmt.Errorf("move %q is not an admitted quiet move", text)
	}
	return nil
}

type infoFields struct {
	depth, seldepth int
	nodes           uint64
	cp              int
	pv              string
}

func parseInfo(line string) (infoFields, error) {
	fields := strings.Fields(line)
	if len(fields) < 2 || fields[0] != "info" {
		return infoFields{}, errors.New("invalid info line")
	}
	var out infoFields
	var hasDepth, hasSelDepth, hasNodes, hasScore, hasPV bool
	for index := 1; index < len(fields); index++ {
		parseInt := func(name string) (int, error) {
			if index+1 >= len(fields) {
				return 0, fmt.Errorf("missing %s value", name)
			}
			index++
			return strconv.Atoi(fields[index])
		}
		switch fields[index] {
		case "depth":
			var err error
			out.depth, err = parseInt("depth")
			hasDepth = err == nil
			if err != nil {
				return infoFields{}, err
			}
		case "seldepth":
			var err error
			out.seldepth, err = parseInt("seldepth")
			hasSelDepth = err == nil
			if err != nil {
				return infoFields{}, err
			}
		case "nodes":
			if index+1 >= len(fields) {
				return infoFields{}, errors.New("missing nodes value")
			}
			index++
			value, err := strconv.ParseUint(fields[index], 10, 64)
			if err != nil {
				return infoFields{}, err
			}
			out.nodes, hasNodes = value, true
		case "score":
			if index+2 >= len(fields) || fields[index+1] != "cp" {
				return infoFields{}, errors.New("score is not exact cp")
			}
			index += 2
			value, err := strconv.Atoi(fields[index])
			if err != nil {
				return infoFields{}, err
			}
			out.cp, hasScore = value, true
		case "lowerbound", "upperbound":
			return infoFields{}, errors.New("bound score")
		case "pv":
			if index+1 >= len(fields) {
				return infoFields{}, errors.New("empty pv")
			}
			out.pv, hasPV = fields[index+1], true
			index = len(fields)
		}
	}
	if !hasDepth || !hasSelDepth || !hasNodes || !hasScore || !hasPV {
		return infoFields{}, errors.New("incomplete info line")
	}
	return out, nil
}

func scoreTarget(material, uciCP int) (int16, float64, float64, error) {
	clamped := material
	if clamped < 17 {
		clamped = 17
	}
	if clamped > 78 {
		clamped = 78
	}
	m := float64(clamped) / 58.0
	normalizer := ((-72.32565836*m+185.93832038)*m-144.58862193)*m + 416.44950446
	score := int(math.Round(float64(uciCP) * normalizer / 208.0))
	if score < -10_000 || score > 10_000 {
		return 0, 0, normalizer, errors.New("score outside admitted range")
	}
	target := 1.0 / (1.0 + math.Exp(-float64(score)/400.0))
	return int16(score), target, normalizer, nil
}

type piece struct {
	code, square byte
}

func parseBoard(fen string, score int16) (boardRecord, int, error) {
	fields := strings.Fields(fen)
	if len(fields) != 6 {
		return boardRecord{}, 0, errors.New("FEN does not have six fields")
	}
	black := fields[1] == "b"
	if fields[1] != "w" && !black {
		return boardRecord{}, 0, errors.New("invalid side to move")
	}
	rows := strings.Split(fields[0], "/")
	if len(rows) != 8 {
		return boardRecord{}, 0, errors.New("FEN does not have eight ranks")
	}
	var pieces []piece
	material := 0
	kings := [2]int{}
	for fenRank, row := range rows {
		file := 0
		for _, symbol := range []byte(row) {
			if symbol >= '1' && symbol <= '8' {
				file += int(symbol - '0')
				continue
			}
			if file >= 8 {
				return boardRecord{}, 0, errors.New("FEN rank overflow")
			}
			sourceBlack := symbol >= 'a' && symbol <= 'z'
			var kind byte
			switch symbol {
			case 'P', 'p':
				kind, material = 0, material+1
			case 'N', 'n':
				kind, material = 1, material+3
			case 'B', 'b':
				kind, material = 2, material+3
			case 'R', 'r':
				kind, material = 3, material+5
			case 'Q', 'q':
				kind, material = 4, material+9
			case 'K', 'k':
				kind = 5
				if sourceBlack {
					kings[1]++
				} else {
					kings[0]++
				}
			default:
				return boardRecord{}, 0, fmt.Errorf("invalid FEN symbol %q", symbol)
			}
			square := byte((7-fenRank)*8 + file)
			if black {
				square ^= 56
			}
			normalizedBlack := sourceBlack != black
			if normalizedBlack {
				kind |= 8
			}
			pieces = append(pieces, piece{code: kind, square: square})
			file++
		}
		if file != 8 {
			return boardRecord{}, 0, errors.New("FEN rank width mismatch")
		}
	}
	if len(pieces) > 32 || kings != [2]int{1, 1} {
		return boardRecord{}, 0, errors.New("invalid piece or king count")
	}
	sort.Slice(pieces, func(i, j int) bool { return pieces[i].square < pieces[j].square })
	board := boardRecord{score: score, result: 1}
	var ourKing, opponentKing byte
	var haveOurKing, haveOpponentKing bool
	for index, item := range pieces {
		board.occ |= uint64(1) << item.square
		board.pcs[index/2] |= item.code << (4 * (index & 1))
		color := int(item.code >> 3)
		if item.code&7 == 5 {
			if color == 0 {
				ourKing, haveOurKing = item.square, true
			} else {
				opponentKing, haveOpponentKing = item.square, true
			}
		}
	}
	if !haveOurKing || !haveOpponentKing {
		return boardRecord{}, 0, errors.New("missing normalized king")
	}
	board.ksq = ourKing
	board.oppKsq = opponentKing ^ 56
	var featureLists [2][]uint16
	kingSquares := [2]int{int(ourKing), int(opponentKing)}
	for perspective := 0; perspective < 2; perspective++ {
		for _, item := range pieces {
			color, pieceType := int(item.code>>3), int(item.code&7)
			square, kingSquare := int(item.square), kingSquares[perspective]
			orientedSquare, orientedKing := square, kingSquare
			if perspective == 1 {
				orientedSquare ^= 56
				orientedKing ^= 56
			}
			if kingSquare%8 > 3 {
				orientedSquare ^= 7
			}
			featureLists[perspective] = append(featureLists[perspective], uint16(
				k4KingBuckets[orientedKing]*768+(color^perspective)*384+pieceType*64+orientedSquare,
			))
		}
		sort.Slice(featureLists[perspective], func(i, j int) bool {
			return featureLists[perspective][i] < featureLists[perspective][j]
		})
	}
	head := (len(pieces) - 2) / 4
	if head < 0 {
		head = 0
	}
	if head > 7 {
		head = 7
	}
	hasher := sha256.New()
	hasher.Write([]byte("ngn-k4-input-v1\x00"))
	var word [2]byte
	for perspective := 0; perspective < 2; perspective++ {
		binary.LittleEndian.PutUint16(word[:], uint16(len(featureLists[perspective])))
		hasher.Write(word[:])
		for _, feature := range featureLists[perspective] {
			binary.LittleEndian.PutUint16(word[:], feature)
			hasher.Write(word[:])
		}
	}
	hasher.Write([]byte{byte(head)})
	copy(board.k4InputSHA256[:], hasher.Sum(nil))
	return board, material, nil
}

func (board boardRecord) bytes() []byte {
	data := make([]byte, RecordBytes)
	binary.LittleEndian.PutUint64(data[0:8], board.occ)
	copy(data[8:24], board.pcs[:])
	binary.LittleEndian.PutUint16(data[24:26], uint16(board.score))
	data[26], data[27], data[28] = board.result, board.ksq, board.oppKsq
	copy(data[29:32], board.extra[:])
	return data
}

func validateRecord(record labelRecord, header outputHeader, previous string) (boardRecord, bool, error) {
	if record.Type != "label" || !validSHA(record.ID) || !validSHA(record.K4InputSHA256) || record.Split != header.Input.Split || record.OriginalFEN == "" ||
		record.PreviousRecordSHA256 != previous || !validSHA(record.RecordSHA256) {
		return boardRecord{}, false, fmt.Errorf("%w: label identity/chain mismatch at %s", ErrContract, record.ID)
	}
	wantDigest, err := recordDigest(record)
	if err != nil || wantDigest != record.RecordSHA256 {
		return boardRecord{}, false, fmt.Errorf("%w: label digest mismatch at %s", ErrContract, record.ID)
	}
	reset, position, err := resetPosition(record.OriginalFEN)
	if err != nil || reset != record.ResetFEN || position.IsInCheck() {
		return boardRecord{}, false, fmt.Errorf("%w: FEN/reset mismatch at %s", ErrContract, record.ID)
	}
	if err := validateQuietMove(position, record.SourceMove, true); err != nil {
		return boardRecord{}, false, fmt.Errorf("%w: source %v", ErrContract, err)
	}
	board, material, err := parseBoard(record.OriginalFEN, record.NGNScore)
	if err != nil {
		return boardRecord{}, false, fmt.Errorf("%w: board conversion at %s: %v", ErrContract, record.ID, err)
	}
	if record.K4InputSHA256 != hex.EncodeToString(board.k4InputSHA256[:]) {
		return boardRecord{}, false, fmt.Errorf("%w: K4 input identity mismatch at %s", ErrContract, record.ID)
	}
	switch record.Status {
	case "rejected":
		if record.RejectionReason == "" {
			return boardRecord{}, false, fmt.Errorf("%w: rejected label lacks reason", ErrContract)
		}
		return boardRecord{}, false, nil
	case "accepted":
		if record.RejectionReason != "" || record.BestMove == "" || record.PVMove != record.BestMove ||
			record.Depth < 4 || record.TeacherInfoLine == "" || !validSHA(record.InfoLineSHA256) {
			return boardRecord{}, false, fmt.Errorf("%w: malformed accepted label %s", ErrContract, record.ID)
		}
	default:
		return boardRecord{}, false, fmt.Errorf("%w: invalid label status %q", ErrContract, record.Status)
	}
	if err := validateQuietMove(position, record.BestMove, false); err != nil {
		return boardRecord{}, false, fmt.Errorf("%w: best %v", ErrContract, err)
	}
	infoDigest := sha256.Sum256([]byte(record.TeacherInfoLine))
	if record.InfoLineSHA256 != hex.EncodeToString(infoDigest[:]) {
		return boardRecord{}, false, fmt.Errorf("%w: info-line digest mismatch at %s", ErrContract, record.ID)
	}
	info, err := parseInfo(record.TeacherInfoLine)
	if err != nil || info.depth != record.Depth || info.seldepth != record.SelDepth || info.nodes != record.Nodes ||
		info.cp != record.UCICP || info.pv != record.PVMove {
		return boardRecord{}, false, fmt.Errorf("%w: info-line fields mismatch at %s", ErrContract, record.ID)
	}
	wantScore, wantTarget, wantNormalizer, err := scoreTarget(material, record.UCICP)
	if err != nil || record.NGNScore != wantScore || record.MaterialUnits != material ||
		math.Float64bits(record.Target) != math.Float64bits(wantTarget) ||
		math.Float64bits(record.MaterialNormalizer) != math.Float64bits(wantNormalizer) {
		return boardRecord{}, false, fmt.Errorf("%w: score contract mismatch at %s", ErrContract, record.ID)
	}
	return board, true, nil
}

func fileAvailable(path string) error {
	if path == "" {
		return errors.New("empty output path")
	}
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("refusing existing output %q", path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func createTempFor(path, pattern string) (*os.File, error) {
	file, err := os.CreateTemp(filepath.Dir(path), pattern)
	if err != nil {
		return nil, err
	}
	if err := file.Chmod(0o644); err != nil {
		file.Close()
		os.Remove(file.Name())
		return nil, err
	}
	return file, nil
}

func writeReceiptTemp(path string, receipt Receipt) (*os.File, error) {
	file, err := createTempFor(path, ".ngnk4pack-receipt-*")
	if err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(receipt, "", "  ")
	if err == nil {
		data = append(data, '\n')
		_, err = file.Write(data)
	}
	if err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(file.Name())
		return nil, err
	}
	return file, nil
}

// Pack verifies the complete immutable label shard before publishing a new BF
// file and receipt. Neither destination is overwritten; a publication failure
// removes any link created by this invocation.
func Pack(inputPath, outputPath, receiptPath string, command []string) (receipt Receipt, err error) {
	if err := fileAvailable(outputPath); err != nil {
		return Receipt{}, err
	}
	if err := fileAvailable(receiptPath); err != nil {
		return Receipt{}, err
	}
	input, err := os.Open(inputPath)
	if err != nil {
		return Receipt{}, err
	}
	defer input.Close()
	info, err := input.Stat()
	if err != nil {
		return Receipt{}, err
	}
	if !info.Mode().IsRegular() || info.Size() < 2 || info.Size() > maximumInputBytes {
		return Receipt{}, fmt.Errorf("%w: input size/type", ErrContract)
	}
	last := []byte{0}
	if _, err := input.ReadAt(last, info.Size()-1); err != nil || last[0] != '\n' {
		return Receipt{}, fmt.Errorf("%w: input is not newline terminated", ErrContract)
	}
	if _, err := input.Seek(0, io.SeekStart); err != nil {
		return Receipt{}, err
	}

	output, err := createTempFor(outputPath, ".ngnk4pack-bf-*")
	if err != nil {
		return Receipt{}, err
	}
	outputTemp := output.Name()
	defer os.Remove(outputTemp)
	defer func() {
		if output != nil {
			closeErr := output.Close()
			if err == nil && closeErr != nil {
				err = closeErr
			}
		}
	}()
	outputHash := sha256.New()
	writer := bufio.NewWriter(io.MultiWriter(output, outputHash))
	inputHash := sha256.New()
	scanner := bufio.NewScanner(io.TeeReader(input, inputHash))
	scanner.Buffer(make([]byte, 64*1024), maximumLineBytes)

	line := 0
	var header outputHeader
	var footer outputFooter
	previous := ""
	accepted, rejected := 0, 0
	rejections := make(map[string]int)
	seen := make(map[string]struct{})
	streamHash := sha256.New()
	streamHash.Write([]byte("ngn-k4-pack-accepted-stream-v1\x00"))
	var ordinal uint64
	for scanner.Scan() {
		line++
		data := append([]byte(nil), scanner.Bytes()...)
		if line == 1 {
			if err := decodeStrict(data, &header); err != nil {
				return Receipt{}, fmt.Errorf("%w: header: %v", ErrContract, err)
			}
			if err := validateHeader(header); err != nil {
				return Receipt{}, err
			}
			previous, err = canonicalDigest(header)
			if err != nil {
				return Receipt{}, err
			}
			continue
		}
		var kind envelope
		if err := json.Unmarshal(data, &kind); err != nil {
			return Receipt{}, fmt.Errorf("%w: line %d: %v", ErrContract, line, err)
		}
		switch kind.Type {
		case "label":
			if footer.Type != "" {
				return Receipt{}, fmt.Errorf("%w: label after footer", ErrContract)
			}
			var record labelRecord
			if err := decodeStrict(data, &record); err != nil {
				return Receipt{}, fmt.Errorf("%w: label line %d: %v", ErrContract, line, err)
			}
			if _, exists := seen[record.ID]; exists {
				return Receipt{}, fmt.Errorf("%w: duplicate label id %s", ErrContract, record.ID)
			}
			seen[record.ID] = struct{}{}
			board, keep, err := validateRecord(record, header, previous)
			if err != nil {
				return Receipt{}, err
			}
			previous = record.RecordSHA256
			if keep {
				encoded := board.bytes()
				if _, err := writer.Write(encoded); err != nil {
					return Receipt{}, err
				}
				streamHash.Write([]byte(record.ID))
				streamHash.Write(board.k4InputSHA256[:])
				var word [8]byte
				binary.LittleEndian.PutUint64(word[:], ordinal)
				streamHash.Write(word[:])
				streamHash.Write(encoded[24:26])
				ordinal++
				accepted++
			} else {
				rejected++
				rejections[record.RejectionReason]++
			}
		case "footer":
			if footer.Type != "" {
				return Receipt{}, fmt.Errorf("%w: duplicate footer", ErrContract)
			}
			if err := decodeStrict(data, &footer); err != nil {
				return Receipt{}, fmt.Errorf("%w: footer: %v", ErrContract, err)
			}
		default:
			return Receipt{}, fmt.Errorf("%w: unexpected line type %q", ErrContract, kind.Type)
		}
	}
	if err := scanner.Err(); err != nil {
		return Receipt{}, err
	}
	if line == 0 || footer.Type == "" {
		return Receipt{}, fmt.Errorf("%w: missing header/footer", ErrContract)
	}
	if footer.Type != "footer" || footer.Schema != LabelSchema || footer.Records != accepted+rejected || footer.Records != header.Input.RecordCount ||
		footer.Accepted != accepted || footer.Rejected != rejected || footer.LastRecordSHA256 != previous || footer.RecoveredPartialByte < 0 ||
		!reflect.DeepEqual(footer.Rejections, rejections) {
		return Receipt{}, fmt.Errorf("%w: footer/count mismatch", ErrContract)
	}
	if accepted == 0 {
		return Receipt{}, fmt.Errorf("%w: shard has no accepted labels", ErrContract)
	}
	if err := writer.Flush(); err != nil {
		return Receipt{}, err
	}
	if err := output.Sync(); err != nil {
		return Receipt{}, err
	}
	if err := output.Close(); err != nil {
		return Receipt{}, err
	}
	output = nil
	outputBytes := int64(accepted * RecordBytes)
	if stat, err := os.Stat(outputTemp); err != nil || stat.Size() != outputBytes {
		return Receipt{}, fmt.Errorf("%w: BF size mismatch", ErrContract)
	}

	receipt = Receipt{
		Schema: ReceiptSchema, ContractVersion: ContractVersion, Command: append([]string(nil), command...),
		InputPath: inputPath, InputBytes: info.Size(), InputSHA256: hex.EncodeToString(inputHash.Sum(nil)), InputShardSHA256: header.InputSHA256,
		ShardID: header.Input.ShardID, Split: header.Input.Split, OutputPath: outputPath, OutputBytes: outputBytes,
		OutputSHA256: hex.EncodeToString(outputHash.Sum(nil)), Records: accepted + rejected, Accepted: accepted, Rejected: rejected,
		Rejections: rejections, RecordBytes: RecordBytes, ResultConstant: 1, ScorePerspective: "side-to-move", K4InputKeyContract: K4InputKeyContract,
		AcceptedStreamSHA256: hex.EncodeToString(streamHash.Sum(nil)), TeacherExecutableSHA256: header.Teacher.ExecutableSHA256,
		TeacherSourceCommit: header.Teacher.SourceCommit, TeacherBigNetworkSHA256: header.Teacher.BigNetworkSHA256,
		TeacherSmallNetworkSHA256: header.Teacher.SmallNetworkSHA256,
	}
	receiptTemp, err := writeReceiptTemp(receiptPath, receipt)
	if err != nil {
		return Receipt{}, err
	}
	defer os.Remove(receiptTemp.Name())
	if err := os.Link(outputTemp, outputPath); err != nil {
		return Receipt{}, err
	}
	if err := os.Link(receiptTemp.Name(), receiptPath); err != nil {
		removeErr := os.Remove(outputPath)
		return Receipt{}, errors.Join(err, removeErr)
	}
	return receipt, nil
}
