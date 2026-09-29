// ngnk4staticdump scores the frozen 100k calibration set with three admitted
// evaluators and writes a no-clobber, provenance-bound per-position table.
package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"math/bits"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ehrlich-b/ngn/engine"
	"github.com/ehrlich-b/ngn/nnue/ngnk4"
	"github.com/ehrlich-b/ngn/rodenteval"
	"github.com/ehrlich-b/ngn/training/nnue/k4finalize"
	"github.com/ehrlich-b/ngn/training/nnue/k4label"
)

const (
	expectedK4SHA     = "034559653a83a7e64d4407badff334f66e3eae6a3a1f33147e3516b3c88c3e69"
	expectedRodentSHA = "5f7480b56538e9e64ee02fb5b4842bf0394ee44b707dae3a63ec7f902abd0afb"
)

type fileReceipt struct {
	Path   string `json:"path"`
	Bytes  int64  `json:"bytes"`
	SHA256 string `json:"sha256"`
}

type receipt struct {
	Schema        string        `json:"schema"`
	Manifest      fileReceipt   `json:"manifest"`
	Calibration   fileReceipt   `json:"calibration_bf"`
	Labels        []fileReceipt `json:"label_shards"`
	K4Model       fileReceipt   `json:"k4_model"`
	RodentModel   fileReceipt   `json:"rodent_model"`
	Predictions   fileReceipt   `json:"predictions"`
	Rows          int           `json:"rows"`
	ScoreContract string        `json:"score_contract"`
}

func shaFile(path string) (fileReceipt, error) {
	file, err := os.Open(path)
	if err != nil {
		return fileReceipt{}, err
	}
	defer file.Close()
	hash := sha256.New()
	n, err := io.Copy(hash, file)
	if err != nil {
		return fileReceipt{}, err
	}
	return fileReceipt{Path: path, Bytes: n, SHA256: hex.EncodeToString(hash.Sum(nil))}, nil
}

func verify(path string, want k4finalize.FileReceipt) (fileReceipt, error) {
	got, err := shaFile(path)
	if err != nil {
		return fileReceipt{}, err
	}
	if got.Bytes != want.Bytes || got.SHA256 != want.SHA256 || path != want.Path {
		return fileReceipt{}, fmt.Errorf("file receipt mismatch: %s", path)
	}
	return got, nil
}

func strictJSON(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("trailing JSON")
	}
	return nil
}

func outputHead(position *engine.Position) int {
	count := 0
	for piece := engine.WhitePawn; piece <= engine.BlackKing; piece++ {
		count += bits.OnesCount64(position.Board.GetBitboardOf(piece))
	}
	head := (count - 2) / 4
	if head < 0 {
		return 0
	}
	if head > 7 {
		return 7
	}
	return head
}

func pieceCount(position *engine.Position) int {
	count := 0
	for piece := engine.WhitePawn; piece <= engine.BlackKing; piece++ {
		count += bits.OnesCount64(position.Board.GetBitboardOf(piece))
	}
	return count
}

// This is the frozen K4 king-bucket table after each perspective's rank flip
// and horizontal mirror. It is reported for diagnostics, not used in inference.
func kingBucket(square int, black bool) int {
	if black {
		square ^= 56
	}
	if square&7 > 3 {
		square ^= 7
	}
	if square >= 16 {
		return 3
	}
	if square >= 8 {
		return 2
	}
	if square < 2 {
		return 1
	}
	return 0
}

func sourcePly(fen string) (int, error) {
	fields := strings.Fields(fen)
	if len(fields) != 6 {
		return 0, fmt.Errorf("not a six-field original FEN")
	}
	fullmove, err := strconv.Atoi(fields[5])
	if err != nil || fullmove < 1 {
		return 0, fmt.Errorf("bad original fullmove %q", fields[5])
	}
	ply := 2 * (fullmove - 1)
	if fields[1] == "b" {
		ply++
	} else if fields[1] != "w" {
		return 0, fmt.Errorf("bad original side")
	}
	return ply, nil
}

type scorers struct {
	hce    *engine.SearchEngine
	k4     *engine.SearchEngine
	rodent *engine.SearchEngine
}

func newScorers(k4Path, rodentPath string) (scorers, error) {
	k4Model, err := ngnk4.LoadFile(k4Path)
	if err != nil {
		return scorers{}, err
	}
	rodentModel, err := rodenteval.LoadV11Anand(rodentPath)
	if err != nil {
		return scorers{}, err
	}
	s := scorers{hce: engine.NewSearchEngine(), k4: engine.NewSearchEngine(), rodent: engine.NewSearchEngine()}
	if err := s.k4.SelectNGNK4Evaluator(k4Model); err != nil {
		return scorers{}, err
	}
	if err := s.rodent.SelectRodentV11AnandEvaluator(rodentModel); err != nil {
		return scorers{}, err
	}
	return s, nil
}

func scoreRecord(record k4label.LabelRecord, s scorers) ([]string, error) {
	position, err := engine.ParseFEN(record.ResetFEN)
	if err != nil {
		return nil, err
	}
	score, target, _, _, err := k4label.ScoreTarget(position, record.UCICP)
	if err != nil || score != int(record.NGNScore) || math.Float64bits(target) != math.Float64bits(record.Target) {
		return nil, fmt.Errorf("5k teacher score contract mismatch for %s", record.ID)
	}
	hce, backend, err := s.hce.EvaluateSelected(position)
	if err != nil || backend != "hce" {
		return nil, fmt.Errorf("HCE score %s: %v (backend %s)", record.ID, err, backend)
	}
	k4, backend, err := s.k4.EvaluateSelected(position)
	if err != nil || backend != "ngn-k4-768-v1" {
		return nil, fmt.Errorf("K4 score %s: %v (backend %s)", record.ID, err, backend)
	}
	rodent, backend, err := s.rodent.EvaluateSelected(position)
	if err != nil || backend != "rodent-v1.1-anand" {
		return nil, fmt.Errorf("Rodent score %s: %v (backend %s)", record.ID, err, backend)
	}
	ply, err := sourcePly(record.OriginalFEN)
	if err != nil {
		return nil, err
	}
	whiteKing := bits.TrailingZeros64(position.Board.GetBitboardOf(engine.WhiteKing))
	blackKing := bits.TrailingZeros64(position.Board.GetBitboardOf(engine.BlackKing))
	side := "w"
	if position.Turn() == engine.Black {
		side = "b"
	}
	return []string{record.ID, strconv.Itoa(score), strconv.Itoa(hce), strconv.Itoa(k4), strconv.Itoa(rodent),
		side, strconv.Itoa(outputHead(position)), strconv.Itoa(kingBucket(whiteKing, false)),
		strconv.Itoa(kingBucket(blackKing, true)), strconv.Itoa(pieceCount(position)), strconv.Itoa(ply)}, nil
}

func dumpShard(shard k4finalize.UsedShard, writer *csv.Writer, s scorers) (int, fileReceipt, error) {
	if shard.AcceptedUsed <= 0 || shard.AcceptedUsed > shard.AcceptedAvailable {
		return 0, fileReceipt{}, fmt.Errorf("invalid accepted count %s", shard.ShardID)
	}
	labelFile, err := verify(shard.Labels.Path, shard.Labels)
	if err != nil {
		return 0, fileReceipt{}, err
	}
	file, err := os.Open(shard.Labels.Path)
	if err != nil {
		return 0, fileReceipt{}, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 4<<20)
	if !scanner.Scan() {
		return 0, fileReceipt{}, fmt.Errorf("missing label header %s", shard.ShardID)
	}
	var header k4label.OutputHeader
	if err := strictJSON(scanner.Bytes(), &header); err != nil {
		return 0, fileReceipt{}, err
	}
	if header.Type != "header" || header.Schema != k4label.OutputSchema || header.ContractVersion != k4label.ContractVersion ||
		header.Input.ShardID != shard.ShardID || header.Input.Split != "calibration" || header.InputSHA256 != shard.SamplerInput.SHA256 ||
		header.Search != k4label.FrozenSearchConfig() || header.Teacher.ExecutableSHA256 != k4finalize.TeacherExecutableSHA ||
		header.Teacher.SourceCommit != k4label.TeacherSourceCommit || header.Teacher.BigNetworkSHA256 != k4label.TeacherBigNetworkSHA ||
		header.Teacher.SmallNetworkSHA256 != k4label.TeacherSmallNetSHA {
		return 0, fileReceipt{}, fmt.Errorf("label header contract mismatch %s", shard.ShardID)
	}
	used := 0
	for scanner.Scan() && used < shard.AcceptedUsed {
		var envelope struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &envelope); err != nil {
			return 0, fileReceipt{}, err
		}
		if envelope.Type != "label" {
			return 0, fileReceipt{}, fmt.Errorf("early footer in %s", shard.ShardID)
		}
		var record k4label.LabelRecord
		if err := strictJSON(scanner.Bytes(), &record); err != nil {
			return 0, fileReceipt{}, err
		}
		if record.Split != "calibration" {
			return 0, fileReceipt{}, fmt.Errorf("wrong split %s", record.ID)
		}
		if record.Status != "accepted" {
			continue
		}
		row, err := scoreRecord(record, s)
		if err != nil {
			return 0, fileReceipt{}, err
		}
		if err := writer.Write(row); err != nil {
			return 0, fileReceipt{}, err
		}
		used++
	}
	if err := scanner.Err(); err != nil {
		return 0, fileReceipt{}, err
	}
	if used != shard.AcceptedUsed {
		return 0, fileReceipt{}, fmt.Errorf("accepted count %s: %d != %d", shard.ShardID, used, shard.AcceptedUsed)
	}
	return used, labelFile, nil
}

func writeNewJSON(path string, value any) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		file.Close()
		os.Remove(path)
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		os.Remove(path)
		return err
	}
	return file.Close()
}

func run(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("ngnk4staticdump", flag.ContinueOnError)
	manifestPath := flags.String("manifest", "", "completed pilot finalizer manifest")
	manifestSHA := flags.String("manifest-sha256", "", "expected finalizer manifest SHA-256")
	k4Path := flags.String("k4-model", "", "selected K4 model file")
	k4SHA := flags.String("k4-sha256", expectedK4SHA, "expected selected K4 model SHA-256")
	rodentPath := flags.String("rodent-model", "", "pinned borrowed Rodent Anand model file")
	outputPath := flags.String("output", "", "new predictions CSV")
	receiptPath := flags.String("receipt", "", "new JSON receipt")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || *manifestPath == "" || *manifestSHA == "" || *k4Path == "" || *rodentPath == "" || *outputPath == "" || *receiptPath == "" {
		return errors.New("required: -manifest -manifest-sha256 -k4-model -rodent-model -output -receipt")
	}
	for _, path := range []string{*outputPath, *receiptPath} {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("refusing existing %s", path)
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	manifestFile, err := shaFile(*manifestPath)
	if err != nil {
		return err
	}
	if manifestFile.SHA256 != *manifestSHA {
		return errors.New("finalizer manifest SHA mismatch")
	}
	data, err := os.ReadFile(*manifestPath)
	if err != nil {
		return err
	}
	var manifest k4finalize.Manifest
	if err := strictJSON(data, &manifest); err != nil {
		return err
	}
	if manifest.Schema != "ngn-k4-finalized-corpus-v1" || manifest.State != "COMPLETE" || manifest.Mode != "pilot" {
		return errors.New("not a completed K4 pilot manifest")
	}
	var calibration k4finalize.CorpusReceipt
	for _, item := range manifest.Corpora {
		if item.Name == "calibration" && item.Split == "calibration" {
			calibration = item
		}
	}
	if calibration.Records != 100_000 || calibration.RecordBytes != 32 {
		return errors.New("calibration corpus contract mismatch")
	}
	calibrationFile, err := verify(calibration.File.Path, calibration.File)
	if err != nil {
		return err
	}
	k4File, err := shaFile(*k4Path)
	if err != nil {
		return err
	}
	if k4File.SHA256 != *k4SHA {
		return errors.New("selected K4 model SHA mismatch")
	}
	rodentFile, err := shaFile(*rodentPath)
	if err != nil {
		return err
	}
	if rodentFile.SHA256 != expectedRodentSHA {
		return errors.New("Rodent model SHA mismatch")
	}
	s, err := newScorers(*k4Path, *rodentPath)
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(*outputPath), ".ngnk4staticdump-*")
	if err != nil {
		return err
	}
	defer os.Remove(temporary.Name())
	writer := csv.NewWriter(temporary)
	if err := writer.Write([]string{"id", "teacher_cp", "hce_cp", "k4_cp", "rodent_cp", "side", "output_head", "white_king_bucket", "black_king_bucket", "piece_count", "source_ply"}); err != nil {
		temporary.Close()
		return err
	}
	result := receipt{Schema: "ngn-k4-static-predictions-v1", Manifest: manifestFile, Calibration: calibrationFile, K4Model: k4File, RodentModel: rodentFile,
		ScoreContract: "teacher and selected evaluator base SearchSTM on halfmove-reset FEN; no search correction history"}
	for _, shard := range manifest.UsedShards {
		if shard.Stage != "fixed" || shard.Split != "calibration" {
			continue
		}
		count, labelFile, err := dumpShard(shard, writer, s)
		if err != nil {
			temporary.Close()
			return err
		}
		result.Rows += count
		result.Labels = append(result.Labels, labelFile)
	}
	if result.Rows != 100_000 {
		temporary.Close()
		return fmt.Errorf("dumped %d rows, want 100000", result.Rows)
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Link(temporary.Name(), *outputPath); err != nil {
		return err
	}
	result.Predictions, err = shaFile(*outputPath)
	if err != nil {
		return err
	}
	if err := writeNewJSON(*receiptPath, result); err != nil {
		return err
	}
	return json.NewEncoder(stdout).Encode(result)
}

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "ngnk4staticdump:", err)
		os.Exit(1)
	}
}
