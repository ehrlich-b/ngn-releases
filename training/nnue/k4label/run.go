package k4label

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/ehrlich-b/ngn/engine"
)

const (
	journalSyncRecords = 256
	maximumOutputBytes = 512 << 20
)

type RunReceipt struct {
	Schema               string            `json:"schema"`
	ContractVersion      string            `json:"contract_version"`
	Command              []string          `json:"command"`
	InputPath            string            `json:"input_path"`
	InputSHA256          string            `json:"input_sha256"`
	OutputPath           string            `json:"output_path"`
	OutputBytes          int64             `json:"output_bytes"`
	OutputSHA256         string            `json:"output_sha256"`
	Records              int               `json:"records"`
	Accepted             int               `json:"accepted"`
	Rejected             int               `json:"rejected"`
	Rejections           map[string]int    `json:"rejections"`
	RecoveredPartialByte int64             `json:"recovered_partial_bytes"`
	Teacher              TeacherProvenance `json:"teacher"`
}

// WriteReceiptNew atomically publishes a receipt without replacing any prior
// evidence at the requested path.
func WriteReceiptNew(path string, receipt RunReceipt) error {
	if path == "" {
		return fmt.Errorf("%w: empty receipt path", ErrContract)
	}
	data, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	temporary, err := os.CreateTemp(filepath.Dir(path), ".ngnk4label-receipt-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o644); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Link(temporaryPath, path); err != nil {
		return err
	}
	return nil
}

type journal struct {
	path       string
	file       *os.File
	writer     *bufio.Writer
	header     OutputHeader
	positions  []InputPosition
	completed  int
	accepted   int
	rejected   int
	rejections map[string]int
	previous   string
	recovered  int64
	unsynced   int
	finalized  bool
}

func marshalLine(value any) ([]byte, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func headerDigest(header OutputHeader) (string, error) {
	data, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), nil
}

func recordDigest(record LabelRecord) (string, error) {
	record.RecordSHA256 = ""
	data, err := json.Marshal(record)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), nil
}

func validateSavedRecord(record LabelRecord, input InputPosition, split, previous string) error {
	reset, _, err := resetClockFEN(input.FEN)
	if err != nil {
		return err
	}
	if record.Type != "label" || record.ID != input.ID || record.K4InputSHA256 != input.K4InputSHA256 || record.Split != split ||
		record.OriginalFEN != input.FEN || record.ResetFEN != reset || record.SourceMove != input.SourceMove {
		return fmt.Errorf("%w: resumed label identity mismatch at %s", ErrContract, input.ID)
	}
	if record.PreviousRecordSHA256 != previous || !validLowerHexSHA(record.RecordSHA256) {
		return fmt.Errorf("%w: resumed label chain mismatch at %s", ErrContract, input.ID)
	}
	want, err := recordDigest(record)
	if err != nil || want != record.RecordSHA256 {
		return fmt.Errorf("%w: resumed label digest mismatch at %s", ErrContract, input.ID)
	}
	switch record.Status {
	case "accepted":
		if record.RejectionReason != "" || record.BestMove == "" || record.InfoLineSHA256 == "" ||
			record.PVMove != record.BestMove || record.Depth < MinimumTeacherDepth ||
			record.Target <= 0 || record.Target >= 1 {
			return fmt.Errorf("%w: malformed accepted label %s", ErrContract, input.ID)
		}
		infoDigest := sha256.Sum256([]byte(record.TeacherInfoLine))
		if record.TeacherInfoLine == "" || record.InfoLineSHA256 != hex.EncodeToString(infoDigest[:]) {
			return fmt.Errorf("%w: teacher info digest mismatch at %s", ErrContract, input.ID)
		}
		parsed, err := parseInfo(record.TeacherInfoLine)
		if err != nil || !parsed.hasScore || parsed.mate || parsed.bound || parsed.cp != record.UCICP ||
			parsed.depth != record.Depth || parsed.seldepth != record.SelDepth || parsed.nodes != record.Nodes ||
			parsed.pv != record.PVMove {
			return fmt.Errorf("%w: teacher info fields mismatch at %s", ErrContract, input.ID)
		}
		_, position, err := resetClockFEN(record.OriginalFEN)
		if err != nil {
			return err
		}
		move, err := engine.ParseUCIMove(position, record.BestMove)
		if err != nil || move.IsCapture() || move.PromoType() != engine.NoType {
			return fmt.Errorf("%w: accepted label %s has nonquiet/illegal bestmove", ErrContract, input.ID)
		}
		score, target, normalizer, material, err := ScoreTarget(position, record.UCICP)
		if err != nil || score != int(record.NGNScore) || material != record.MaterialUnits ||
			math.Float64bits(target) != math.Float64bits(record.Target) ||
			math.Float64bits(normalizer) != math.Float64bits(record.MaterialNormalizer) {
			return fmt.Errorf("%w: accepted label %s score contract mismatch", ErrContract, input.ID)
		}
	case "rejected":
		if record.RejectionReason == "" {
			return fmt.Errorf("%w: rejected label %s lacks reason", ErrContract, input.ID)
		}
	default:
		return fmt.Errorf("%w: invalid label status %q", ErrContract, record.Status)
	}
	return nil
}

func expectedHeader(shard InputShard, provenance TeacherProvenance) OutputHeader {
	return OutputHeader{
		Type: "header", Schema: OutputSchema, ContractVersion: ContractVersion,
		InputSHA256: shard.SHA256, Input: shard.Header, Teacher: provenance,
		Search:        FrozenSearchConfig(),
		ScoreContract: "c=round(uci_cp*a(material)/208); target=1/(1+exp(-c/400)); STM; root_halfmove=0",
	}
}

func openJournal(path string, header OutputHeader, positions []InputPosition) (*journal, error) {
	if path == "" {
		return nil, fmt.Errorf("%w: empty journal path", ErrContract)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	fail := func(cause error) (*journal, error) {
		_ = file.Close()
		return nil, cause
	}
	info, err := file.Stat()
	if err != nil {
		return fail(err)
	}
	if info.Size() > maximumOutputBytes {
		return fail(fmt.Errorf("%w: partial output exceeds %d bytes", ErrContract, maximumOutputBytes))
	}
	state := &journal{path: path, file: file, header: header, positions: positions, rejections: make(map[string]int)}
	previous, err := headerDigest(header)
	if err != nil {
		return fail(err)
	}
	state.previous = previous
	if info.Size() == 0 {
		line, err := marshalLine(header)
		if err != nil {
			return fail(err)
		}
		if _, err := file.Write(line); err != nil {
			return fail(err)
		}
		if err := file.Sync(); err != nil {
			return fail(err)
		}
	} else {
		data, err := io.ReadAll(io.LimitReader(file, maximumOutputBytes+1))
		if err != nil {
			return fail(err)
		}
		if len(data) > maximumOutputBytes {
			return fail(fmt.Errorf("%w: partial output exceeds bound", ErrContract))
		}
		validLength := len(data)
		if len(data) > 0 && data[len(data)-1] != '\n' {
			last := bytes.LastIndexByte(data, '\n')
			if last < 0 {
				return fail(fmt.Errorf("%w: partial output has no complete header", ErrContract))
			}
			validLength = last + 1
			state.recovered = int64(len(data) - validLength)
			if err := file.Truncate(int64(validLength)); err != nil {
				return fail(err)
			}
			data = data[:validLength]
		}
		lines := bytes.Split(bytes.TrimSuffix(data, []byte{'\n'}), []byte{'\n'})
		var savedHeader OutputHeader
		if len(lines) == 0 || unmarshalStrict(lines[0], &savedHeader) != nil || !reflect.DeepEqual(savedHeader, header) {
			return fail(fmt.Errorf("%w: partial output header does not match this run", ErrContract))
		}
		for index, line := range lines[1:] {
			var envelope struct {
				Type string `json:"type"`
			}
			if err := json.Unmarshal(line, &envelope); err != nil {
				return fail(fmt.Errorf("partial output line %d: %w", index+2, err))
			}
			if envelope.Type == "footer" {
				if index != len(lines)-2 || state.completed != len(positions) {
					return fail(fmt.Errorf("%w: misplaced/incomplete partial footer", ErrContract))
				}
				var footer OutputFooter
				if err := unmarshalStrict(line, &footer); err != nil || footer.Type != "footer" || footer.Schema != OutputSchema ||
					footer.LastRecordSHA256 != state.previous ||
					footer.Records != state.completed || footer.Accepted != state.accepted || footer.Rejected != state.rejected ||
					!reflect.DeepEqual(footer.Rejections, state.rejections) {
					return fail(fmt.Errorf("%w: invalid partial footer", ErrContract))
				}
				state.recovered = footer.RecoveredPartialByte
				state.finalized = true
				continue
			}
			if state.completed >= len(positions) {
				return fail(fmt.Errorf("%w: partial output has excess records", ErrContract))
			}
			var record LabelRecord
			if err := unmarshalStrict(line, &record); err != nil {
				return fail(err)
			}
			if err := validateSavedRecord(record, positions[state.completed], header.Input.Split, state.previous); err != nil {
				return fail(err)
			}
			state.completed++
			state.previous = record.RecordSHA256
			if record.Status == "accepted" {
				state.accepted++
			} else {
				state.rejected++
				state.rejections[record.RejectionReason]++
			}
		}
	}
	if _, err := file.Seek(0, io.SeekEnd); err != nil {
		return fail(err)
	}
	state.writer = bufio.NewWriterSize(file, 256*1024)
	return state, nil
}

func (state *journal) append(record LabelRecord) error {
	if state.finalized || state.completed >= len(state.positions) {
		return fmt.Errorf("%w: append after journal completion", ErrContract)
	}
	input := state.positions[state.completed]
	record.PreviousRecordSHA256 = state.previous
	digest, err := recordDigest(record)
	if err != nil {
		return err
	}
	record.RecordSHA256 = digest
	if err := validateSavedRecord(record, input, state.header.Input.Split, state.previous); err != nil {
		return err
	}
	line, err := marshalLine(record)
	if err != nil {
		return err
	}
	if _, err := state.writer.Write(line); err != nil {
		return err
	}
	state.completed++
	state.previous = record.RecordSHA256
	if record.Status == "accepted" {
		state.accepted++
	} else {
		state.rejected++
		state.rejections[record.RejectionReason]++
	}
	state.unsynced++
	if state.unsynced >= journalSyncRecords {
		return state.sync()
	}
	return nil
}

func (state *journal) sync() error {
	if err := state.writer.Flush(); err != nil {
		return err
	}
	if err := state.file.Sync(); err != nil {
		return err
	}
	state.unsynced = 0
	return nil
}

func (state *journal) finalize() error {
	if state.finalized {
		return nil
	}
	if state.completed != len(state.positions) {
		return fmt.Errorf("%w: cannot finalize %d/%d records", ErrContract, state.completed, len(state.positions))
	}
	footer := OutputFooter{
		Type: "footer", Schema: OutputSchema, Records: state.completed, Accepted: state.accepted,
		Rejected: state.rejected, Rejections: state.rejections, LastRecordSHA256: state.previous,
		RecoveredPartialByte: state.recovered,
	}
	line, err := marshalLine(footer)
	if err != nil {
		return err
	}
	if _, err := state.writer.Write(line); err != nil {
		return err
	}
	state.finalized = true
	return state.sync()
}

func (state *journal) close() error {
	var errs []error
	if state.writer != nil {
		errs = append(errs, state.writer.Flush())
	}
	if state.file != nil {
		errs = append(errs, state.file.Sync(), state.file.Close())
	}
	return errors.Join(errs...)
}

func fileReceipt(path string) (int64, string, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, "", err
	}
	defer file.Close()
	digest := sha256.New()
	bytes, err := io.Copy(digest, file)
	if err != nil {
		return 0, "", err
	}
	return bytes, hex.EncodeToString(digest.Sum(nil)), nil
}

func publishNew(partial, output string) error {
	if filepath.Clean(partial) == filepath.Clean(output) {
		return fmt.Errorf("%w: partial and output paths match", ErrContract)
	}
	if err := os.Link(partial, output); err != nil {
		return err
	}
	return os.Remove(partial)
}

// RunShard labels one immutable sampler shard. A fatal teacher/protocol error
// leaves a synced .partial journal; rerunning with identical inputs and teacher
// provenance verifies the hash chain and resumes at the next record.
func RunShard(ctx context.Context, inputPath, outputPath string, analyzer Analyzer) (receipt RunReceipt, err error) {
	if analyzer == nil {
		return RunReceipt{}, fmt.Errorf("%w: nil analyzer", ErrContract)
	}
	if session, ok := analyzer.(*Session); ok && session.config != FrozenSearchConfig() {
		return RunReceipt{}, fmt.Errorf("%w: diagnostic teacher configuration cannot label training data", ErrContract)
	}
	if outputPath == "" || strings.HasSuffix(outputPath, ".partial") {
		return RunReceipt{}, fmt.Errorf("%w: invalid output path", ErrContract)
	}
	if _, statErr := os.Stat(outputPath); statErr == nil {
		return RunReceipt{}, fmt.Errorf("refusing existing output %q", outputPath)
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return RunReceipt{}, statErr
	}
	shard, err := ReadInputShard(inputPath)
	if err != nil {
		return RunReceipt{}, err
	}
	header := expectedHeader(shard, analyzer.Provenance())
	partial := outputPath + ".partial"
	state, err := openJournal(partial, header, shard.Positions)
	if err != nil {
		return RunReceipt{}, err
	}
	defer func() {
		if closeErr := state.close(); err == nil && closeErr != nil {
			err = closeErr
		}
	}()
	for state.completed < len(shard.Positions) {
		input := shard.Positions[state.completed]
		resetFEN, _, err := resetClockFEN(input.FEN)
		if err != nil {
			return RunReceipt{}, err
		}
		result, rejection, err := analyzer.Analyze(ctx, resetFEN)
		if err != nil {
			return RunReceipt{}, fmt.Errorf("label %s: %w", input.ID, err)
		}
		record, err := buildLabelRecord(input, shard.Header.Split, result, rejection)
		if err != nil {
			return RunReceipt{}, err
		}
		if err := state.append(record); err != nil {
			return RunReceipt{}, err
		}
	}
	if err := state.finalize(); err != nil {
		return RunReceipt{}, err
	}
	if err := state.close(); err != nil {
		state.file = nil
		state.writer = nil
		return RunReceipt{}, err
	}
	state.file = nil
	state.writer = nil
	if err := publishNew(partial, outputPath); err != nil {
		return RunReceipt{}, err
	}
	bytes, sha, err := fileReceipt(outputPath)
	if err != nil {
		return RunReceipt{}, err
	}
	return RunReceipt{
		Schema: OutputSchema + "-receipt", ContractVersion: ContractVersion,
		InputPath: inputPath, InputSHA256: shard.SHA256, OutputPath: outputPath,
		OutputBytes: bytes, OutputSHA256: sha, Records: state.completed,
		Accepted: state.accepted, Rejected: state.rejected, Rejections: state.rejections,
		RecoveredPartialByte: state.recovered, Teacher: analyzer.Provenance(),
	}, nil
}
