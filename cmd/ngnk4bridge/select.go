package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
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
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/ehrlich-b/ngn/nnue/ngnk4"
)

const (
	selectionSchema             = "ngn-k4-pilot-selection-v1"
	selectionSchemaS1           = "ngn-k4-pilot-s1-selection-v1"
	selectionSchemaLR1          = "ngn-k4-pilot-lr1-selection-v1"
	selectionSchemaProbe5M      = "ngn-k4-probe5m-selection-v1"
	selectionSchemaProbe5MLR1   = "ngn-k4-probe5m-lr1-selection-v1"
	selectionSchemaMainM1       = "ngn-k4-main-m1-selection-v1"
	selectionSchemaMainM1LR1    = "ngn-k4-main-m1-lr1-selection-v1"
	selectionSchemaMainM1LR132K = "ngn-k4-main-m1-lr1-32k-selection-v1"
	selectionSchemaArchiveA1E1  = "ngn-k4-archive-a1-e1-selection-v1"
	selectionSchemaArchiveA1E10 = "ngn-k4-archive-a1-e10-selection-v1"
	selectionSchemaArchiveA1E20 = "ngn-k4-archive-a1-e20-selection-v1"
	archiveCorpusSchema         = "ngn-k4-archive-corpus-v1"
	checkpointSchema            = "ngn-k4-bullet-checkpoint-v1"
	finalizedSchema             = "ngn-k4-finalized-corpus-v1"
	finalizedContract           = "sf18-5kn-material-inversion-v2"
	pilotTotalUpdates           = 1024
	pilotCheckpointUpdates      = 128
	pilotValidationRecords      = 100_000
	selectionScoreScale         = 400.0
	selectionImprovementGate    = 1.0e-5
	selectionPatience           = 4
	selectionOptimizerMax       = 64 << 20
	selectionOptimizerSize      = 8
)

type selectionFileReceipt struct {
	Path   string `json:"path"`
	Bytes  int64  `json:"bytes"`
	SHA256 string `json:"sha256"`
}

type selectionCorpus struct {
	Name        string               `json:"name"`
	Split       string               `json:"split"`
	Records     uint64               `json:"records"`
	RecordBytes int                  `json:"record_bytes"`
	File        selectionFileReceipt `json:"file"`
	Inherited   bool                 `json:"inherited,omitempty"`
}

type selectionManifest struct {
	Schema          string                `json:"schema"`
	ContractVersion string                `json:"contract_version"`
	State           string                `json:"state"`
	Mode            string                `json:"mode"`
	ParentPilot     *selectionFileReceipt `json:"parent_pilot,omitempty"`
	Corpora         []selectionCorpus     `json:"corpora"`
}

type selectionCheckpointFiles struct {
	Raw       selectionFileReceipt `json:"raw"`
	Quantised selectionFileReceipt `json:"quantised"`
	Weights   selectionFileReceipt `json:"weights"`
	Momentum  selectionFileReceipt `json:"momentum"`
	Velocity  selectionFileReceipt `json:"velocity"`
}

type selectionCheckpoint struct {
	Schema            string                   `json:"schema"`
	BulletCommit      string                   `json:"bullet_commit"`
	BulletPatchSHA256 string                   `json:"bullet_patch_sha256"`
	Mode              string                   `json:"mode"`
	FinalizedManifest selectionFileReceipt     `json:"finalized_manifest"`
	CompletedUpdates  uint64                   `json:"completed_updates"`
	TotalUpdates      uint64                   `json:"total_updates"`
	CheckpointUpdates uint64                   `json:"checkpoint_updates"`
	InputBuckets      int                      `json:"input_buckets"`
	Hidden            int                      `json:"hidden"`
	OutputBuckets     int                      `json:"output_buckets"`
	ModelSeed         uint64                   `json:"model_seed"`
	ScoreScale        int                      `json:"score_scale"`
	InitialLR         float32                  `json:"initial_lr"`
	FinalLR           float32                  `json:"final_lr"`
	Files             selectionCheckpointFiles `json:"files"`
}

type candidateMetric struct {
	Update                        uint64                      `json:"update"`
	Checkpoint                    selectionFileReceipt        `json:"checkpoint"`
	Raw                           selectionFileReceipt        `json:"raw"`
	Quantised                     selectionFileReceipt        `json:"quantised"`
	Positions                     int                         `json:"positions"`
	RawMeanMSE                    float64                     `json:"raw_mean_mse"`
	IntegerMeanMSE                float64                     `json:"integer_mean_mse"`
	IntegerMinusRawMSE            float64                     `json:"integer_minus_raw_mse"`
	MeanAbsoluteQuantizationCP    float64                     `json:"mean_absolute_quantization_cp"`
	P99AbsoluteQuantizationCP     float64                     `json:"p99_absolute_quantization_cp"`
	MaximumAbsoluteQuantizationCP float64                     `json:"maximum_absolute_quantization_cp"`
	PositionsByOutputBucket       [ngnk4.OutputBuckets]uint64 `json:"positions_by_output_bucket"`
	SaturatedValuesByOutputHead   [ngnk4.OutputBuckets]uint64 `json:"saturated_values_by_output_head"`
	FastOutputSafe                [ngnk4.OutputBuckets]bool   `json:"fast_output_safe"`
	OptimizerStateFinite          bool                        `json:"optimizer_state_finite"`
	Eligible                      bool                        `json:"eligible"`
}

type optimizerTensorSpec struct {
	Name  string
	Count uint64
}

type earlyStopReceipt struct {
	MinimumUpdate                uint64  `json:"minimum_update"`
	Patience                     int     `json:"patience"`
	MinimumIntegerMSEImprovement float64 `json:"minimum_integer_mse_improvement"`
	Triggered                    bool    `json:"triggered"`
	StopUpdate                   uint64  `json:"stop_update,omitempty"`
}

type selectionReceipt struct {
	Schema          string               `json:"schema"`
	Mode            string               `json:"mode,omitempty"`
	ContractVersion string               `json:"contract_version"`
	Command         []string             `json:"command"`
	Manifest        selectionFileReceipt `json:"manifest"`
	Validation      selectionFileReceipt `json:"validation"`
	Rank            string               `json:"rank"`
	TieBreak        string               `json:"tie_break"`
	MeanGateCP      float64              `json:"mean_absolute_quantization_gate_cp"`
	P99GateCP       float64              `json:"p99_absolute_quantization_gate_cp"`
	MaximumGateCP   float64              `json:"maximum_absolute_quantization_gate_cp"`
	EarlyStop       earlyStopReceipt     `json:"early_stop"`
	Candidates      []candidateMetric    `json:"candidates"`
	SelectedUpdate  uint64               `json:"selected_update"`
	Selected        selectionFileReceipt `json:"selected_checkpoint"`
}

type selectionMode struct {
	name              string
	manifestMode      string
	schema            string
	totalUpdates      int
	checkpointUpdates int
	minimumUpdate     uint64
	patience          int
}

func selectionModeFor(value string) (selectionMode, error) {
	switch value {
	case "pilot":
		return selectionMode{
			name: "pilot", manifestMode: "pilot", schema: selectionSchema, totalUpdates: pilotTotalUpdates,
			checkpointUpdates: pilotCheckpointUpdates, minimumUpdate: pilotTotalUpdates / 4,
			patience: selectionPatience,
		}, nil
	case "pilot-s1":
		return selectionMode{
			name: "pilot-s1", manifestMode: "pilot", schema: selectionSchemaS1, totalUpdates: 16_384,
			checkpointUpdates: 512, minimumUpdate: 4_096, patience: 8,
		}, nil
	case "pilot-lr1":
		return selectionMode{
			name: "pilot-lr1", manifestMode: "pilot", schema: selectionSchemaLR1, totalUpdates: 16_384,
			checkpointUpdates: 512, minimumUpdate: 4_096, patience: 8,
		}, nil
	case "probe5m":
		return selectionMode{
			name: "probe5m", manifestMode: "probe5m", schema: selectionSchemaProbe5M, totalUpdates: 32_768,
			checkpointUpdates: 1_024, minimumUpdate: 8_192, patience: 8,
		}, nil
	case "probe5m-lr1":
		return selectionMode{
			name: "probe5m-lr1", manifestMode: "probe5m", schema: selectionSchemaProbe5MLR1, totalUpdates: 32_768,
			checkpointUpdates: 1_024, minimumUpdate: 8_192, patience: 8,
		}, nil
	case "main-m1":
		return selectionMode{
			name: "main-m1", manifestMode: "main", schema: selectionSchemaMainM1, totalUpdates: 131_072,
			checkpointUpdates: 4_096, minimumUpdate: 32_768, patience: 6,
		}, nil
	case "main-m1-lr1":
		return selectionMode{
			name: "main-m1-lr1", manifestMode: "main", schema: selectionSchemaMainM1LR1, totalUpdates: 131_072,
			checkpointUpdates: 4_096, minimumUpdate: 32_768, patience: 6,
		}, nil
	case "main-m1-lr1-32k":
		return selectionMode{
			name: "main-m1-lr1-32k", manifestMode: "main", schema: selectionSchemaMainM1LR132K, totalUpdates: 32_768,
			checkpointUpdates: 4_096, minimumUpdate: 8_192, patience: 8,
		}, nil
	case "archive-a1-e1":
		return selectionMode{
			name: "archive-a1-e1", manifestMode: "archive", schema: selectionSchemaArchiveA1E1, totalUpdates: 163_840,
			checkpointUpdates: 16_384, minimumUpdate: 81_920, patience: 8,
		}, nil
	case "archive-a1-e10":
		return selectionMode{
			name: "archive-a1-e10", manifestMode: "archive", schema: selectionSchemaArchiveA1E10, totalUpdates: 1_638_400,
			checkpointUpdates: 16_384, minimumUpdate: 819_200, patience: 8,
		}, nil
	case "archive-a1-e20":
		return selectionMode{
			name: "archive-a1-e20", manifestMode: "archive", schema: selectionSchemaArchiveA1E20, totalUpdates: 3_276_800,
			checkpointUpdates: 16_384, minimumUpdate: 1_638_400, patience: 8,
		}, nil
	default:
		return selectionMode{}, fmt.Errorf("unknown selection mode %q", value)
	}
}

func selectionReceiptForBytes(path string, data []byte) selectionFileReceipt {
	digest := sha256.Sum256(data)
	return selectionFileReceipt{Path: path, Bytes: int64(len(data)), SHA256: hex.EncodeToString(digest[:])}
}

func verifySelectionFile(item selectionFileReceipt, expectedPath string, expectedBytes int) ([]byte, error) {
	if item.Path == "" || item.Bytes < 0 || len(item.SHA256) != sha256.Size*2 || !samePath(item.Path, expectedPath) {
		return nil, errors.New("malformed or path-mismatched file receipt")
	}
	data, err := readExactFile(expectedPath, expectedBytes)
	if err != nil {
		return nil, err
	}
	actual := selectionReceiptForBytes(expectedPath, data)
	if item.Bytes != actual.Bytes || item.SHA256 != actual.SHA256 {
		return nil, errors.New("file receipt size or SHA-256 mismatch")
	}
	return data, nil
}

func validateOptimizerState(data []byte) ([]optimizerTensorSpec, error) {
	if len(data) == 0 || len(data) > selectionOptimizerMax {
		return nil, fmt.Errorf("optimizer state bytes %d outside 1..%d", len(data), selectionOptimizerMax)
	}
	seen := make(map[string]struct{})
	var specs []optimizerTensorSpec
	for offset := 0; offset < len(data); {
		relativeEnd := bytes.IndexByte(data[offset:], '\n')
		if relativeEnd <= 0 || relativeEnd > 1024 {
			return nil, fmt.Errorf("optimizer tensor %d has invalid name", len(specs))
		}
		nameBytes := data[offset : offset+relativeEnd]
		if !utf8.Valid(nameBytes) {
			return nil, fmt.Errorf("optimizer tensor %d name is not UTF-8", len(specs))
		}
		name := string(nameBytes)
		if _, exists := seen[name]; exists {
			return nil, fmt.Errorf("duplicate optimizer tensor %q", name)
		}
		seen[name] = struct{}{}
		offset += relativeEnd + 1
		if len(data)-offset < selectionOptimizerSize {
			return nil, fmt.Errorf("optimizer tensor %q lacks a size", name)
		}
		count := binary.LittleEndian.Uint64(data[offset : offset+selectionOptimizerSize])
		offset += selectionOptimizerSize
		if count > uint64((len(data)-offset)/4) {
			return nil, fmt.Errorf("optimizer tensor %q is truncated", name)
		}
		valueBytes := int(count) * 4
		for index := 0; index < valueBytes; index += 4 {
			value := math.Float32frombits(binary.LittleEndian.Uint32(data[offset+index:]))
			if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
				return nil, fmt.Errorf("optimizer tensor %q value %d is nonfinite", name, index/4)
			}
		}
		offset += valueBytes
		specs = append(specs, optimizerTensorSpec{Name: name, Count: count})
	}
	if len(specs) == 0 {
		return nil, errors.New("optimizer state contains no tensors")
	}
	return specs, nil
}

func verifyOptimizerState(item selectionFileReceipt, expectedPath string) ([]optimizerTensorSpec, error) {
	if item.Bytes <= 0 || item.Bytes > selectionOptimizerMax {
		return nil, fmt.Errorf("optimizer receipt bytes %d outside 1..%d", item.Bytes, selectionOptimizerMax)
	}
	data, err := verifySelectionFile(item, expectedPath, int(item.Bytes))
	if err != nil {
		return nil, err
	}
	return validateOptimizerState(data)
}

func sameOptimizerLayout(first, second []optimizerTensorSpec) bool {
	if len(first) != len(second) {
		return false
	}
	for index := range first {
		if first[index] != second[index] {
			return false
		}
	}
	return true
}

func loadSelectionManifest(path, validationPath string, mode selectionMode) ([]byte, [sha256.Size]byte, selectionFileReceipt, []byte, error) {
	manifestBytes, manifestSHA, err := readManifest(path)
	if err != nil {
		return nil, [sha256.Size]byte{}, selectionFileReceipt{}, nil, err
	}
	if mode.manifestMode == "archive" {
		// Archive corpora carry their own training file; the frozen SF18-labeled
		// validation BF is supplied directly as the monitoring holdout.
		var archive struct {
			Schema  string               `json:"schema"`
			State   string               `json:"state"`
			Records uint64               `json:"records"`
			Train   selectionFileReceipt `json:"train"`
		}
		if err := json.Unmarshal(manifestBytes, &archive); err != nil {
			return nil, [sha256.Size]byte{}, selectionFileReceipt{}, nil, err
		}
		if archive.Schema != archiveCorpusSchema || archive.State != "COMPLETE" || archive.Records == 0 ||
			archive.Train.Bytes != int64(archive.Records*32) {
			return nil, [sha256.Size]byte{}, selectionFileReceipt{}, nil, errors.New("archive manifest contract mismatch")
		}
		validationBytes, err := readExactFile(validationPath, pilotValidationRecords*32)
		if err != nil {
			return nil, [sha256.Size]byte{}, selectionFileReceipt{}, nil, fmt.Errorf("read validation: %w", err)
		}
		return manifestBytes, manifestSHA, selectionReceiptForBytes(path, manifestBytes), validationBytes, nil
	}
	var manifest selectionManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return nil, [sha256.Size]byte{}, selectionFileReceipt{}, nil, err
	}
	if manifest.Schema != finalizedSchema || manifest.ContractVersion != finalizedContract ||
		manifest.State != "COMPLETE" || manifest.Mode != mode.manifestMode ||
		(manifest.ParentPilot != nil) != (mode.manifestMode != "pilot") {
		return nil, [sha256.Size]byte{}, selectionFileReceipt{}, nil, errors.New("finalized manifest contract mismatch")
	}
	if manifest.ParentPilot != nil {
		parent := *manifest.ParentPilot
		if parent.Bytes <= 0 || parent.Bytes > 16<<20 {
			return nil, [sha256.Size]byte{}, selectionFileReceipt{}, nil, errors.New("parent pilot receipt size mismatch")
		}
		if _, err := verifySelectionFile(parent, parent.Path, int(parent.Bytes)); err != nil {
			return nil, [sha256.Size]byte{}, selectionFileReceipt{}, nil, fmt.Errorf("verify parent pilot: %w", err)
		}
	}
	trainName, trainRecords := "train-pilot", uint64(1_000_000)
	switch mode.manifestMode {
	case "probe5m":
		trainName, trainRecords = "train-probe5m", 5_000_000
	case "main":
		trainName, trainRecords = "train-main", 20_000_000
	}
	var validation *selectionCorpus
	var train *selectionCorpus
	for index := range manifest.Corpora {
		if manifest.Corpora[index].Name == trainName {
			if train != nil {
				return nil, [sha256.Size]byte{}, selectionFileReceipt{}, nil, errors.New("duplicate training corpus")
			}
			train = &manifest.Corpora[index]
		}
		if manifest.Corpora[index].Name == "validation" {
			if validation != nil {
				return nil, [sha256.Size]byte{}, selectionFileReceipt{}, nil, errors.New("duplicate validation corpus")
			}
			validation = &manifest.Corpora[index]
		}
	}
	if len(manifest.Corpora) != 4 || train == nil || train.Split != "train" || train.Records != trainRecords ||
		train.RecordBytes != 32 || train.File.Bytes != int64(trainRecords*32) || train.Inherited {
		return nil, [sha256.Size]byte{}, selectionFileReceipt{}, nil, errors.New("training corpus contract mismatch")
	}
	if validation == nil || validation.Split != "validation" || validation.Records != pilotValidationRecords ||
		validation.RecordBytes != 32 || validation.File.Bytes != pilotValidationRecords*32 ||
		validation.Inherited != (mode.manifestMode != "pilot") ||
		!samePath(validation.File.Path, validationPath) {
		return nil, [sha256.Size]byte{}, selectionFileReceipt{}, nil, errors.New("validation corpus contract mismatch")
	}
	validationBytes, err := verifySelectionFile(validation.File, validationPath, pilotValidationRecords*32)
	if err != nil {
		return nil, [sha256.Size]byte{}, selectionFileReceipt{}, nil, fmt.Errorf("verify validation: %w", err)
	}
	return manifestBytes, manifestSHA, selectionReceiptForBytes(path, manifestBytes), validationBytes, nil
}

func decodeSelectionPosition(record []byte) (ngnk4.Position, int16, error) {
	if len(record) != 32 {
		return ngnk4.Position{}, 0, errors.New("BF record is not 32 bytes")
	}
	occupied := binary.LittleEndian.Uint64(record[:8])
	pieceCount := bits.OnesCount64(occupied)
	if pieceCount < 2 || pieceCount > 32 || record[26] != 1 || record[29] != 0 || record[30] != 0 || record[31] != 0 {
		return ngnk4.Position{}, 0, errors.New("BF record header/result/reserved contract mismatch")
	}
	position := ngnk4.Position{SideToMove: ngnk4.White}
	remaining := occupied
	for pieceIndex := 0; pieceIndex < pieceCount; pieceIndex++ {
		square := bits.TrailingZeros64(remaining)
		remaining &= remaining - 1
		piece := (record[8+pieceIndex/2] >> (4 * (pieceIndex & 1))) & 15
		kind := int(piece & 7)
		color := int(piece >> 3)
		if kind > 5 || color > 1 {
			return ngnk4.Position{}, 0, fmt.Errorf("invalid BF piece nibble %d", piece)
		}
		position.Board[color*6+kind] |= uint64(1) << square
	}
	if remaining != 0 || bits.OnesCount64(position.Board[ngnk4.WhiteKing]) != 1 ||
		bits.OnesCount64(position.Board[ngnk4.BlackKing]) != 1 {
		return ngnk4.Position{}, 0, errors.New("BF occupancy or king contract mismatch")
	}
	whiteKing := bits.TrailingZeros64(position.Board[ngnk4.WhiteKing])
	blackKing := bits.TrailingZeros64(position.Board[ngnk4.BlackKing])
	if int(record[27]) != whiteKing || int(record[28]) != blackKing^56 {
		return ngnk4.Position{}, 0, errors.New("BF cached king square mismatch")
	}
	return position, int16(binary.LittleEndian.Uint16(record[24:26])), nil
}

func selectionSigmoid(value float64) float64 {
	return 1 / (1 + math.Exp(-value))
}

func outputHeadSaturation(tensors *ngnk4.Tensors) [ngnk4.OutputBuckets]uint64 {
	var counts [ngnk4.OutputBuckets]uint64
	for bucket := range tensors.OutputWeights {
		for perspective := range tensors.OutputWeights[bucket] {
			for _, value := range tensors.OutputWeights[bucket][perspective] {
				if value == math.MinInt16 || value == math.MaxInt16 {
					counts[bucket]++
				}
			}
		}
		if tensors.OutputBiases[bucket] == math.MinInt16 || tensors.OutputBiases[bucket] == math.MaxInt16 {
			counts[bucket]++
		}
	}
	return counts
}

func evaluateSelectionCandidate(
	metric candidateMetric,
	rawData, quantisedData, validation []byte,
	manifestSHA [sha256.Size]byte,
) (candidateMetric, error) {
	raw, err := decodeRaw(rawData)
	if err != nil {
		return candidateMetric{}, err
	}
	tensors, _, err := decodeBullet(quantisedData)
	if err != nil {
		return candidateMetric{}, err
	}
	modelBytes, err := ngnk4.Marshal(tensors, manifestSHA)
	if err != nil {
		return candidateMetric{}, err
	}
	if err := verifyRawQuantization(rawData, modelBytes); err != nil {
		return candidateMetric{}, err
	}
	model, err := ngnk4.Load(bytes.NewReader(modelBytes))
	if err != nil {
		return candidateMetric{}, err
	}
	metadata := model.Metadata()
	metric.FastOutputSafe = metadata.FastOutputSafe
	metric.SaturatedValuesByOutputHead = outputHeadSaturation(tensors)
	errorsCP := make([]float64, 0, pilotValidationRecords)
	var rawLoss, integerLoss float64
	for offset := 0; offset < len(validation); offset += 32 {
		position, score, err := decodeSelectionPosition(validation[offset : offset+32])
		if err != nil {
			return candidateMetric{}, fmt.Errorf("validation record %d: %w", offset/32, err)
		}
		rawZ, bucket := evaluateFloat(raw, position)
		integerCP, err := model.EvaluateRaw(position)
		if err != nil {
			return candidateMetric{}, fmt.Errorf("integer validation record %d: %w", offset/32, err)
		}
		target := selectionSigmoid(float64(score) / selectionScoreScale)
		rawError := selectionSigmoid(rawZ) - target
		integerError := selectionSigmoid(float64(integerCP)/selectionScoreScale) - target
		rawLoss += rawError * rawError
		integerLoss += integerError * integerError
		errorsCP = append(errorsCP, math.Abs(rawZ*selectionScoreScale-float64(integerCP)))
		metric.PositionsByOutputBucket[bucket]++
	}
	sort.Float64s(errorsCP)
	metric.Positions = len(errorsCP)
	metric.RawMeanMSE = rawLoss / float64(metric.Positions)
	metric.IntegerMeanMSE = integerLoss / float64(metric.Positions)
	metric.IntegerMinusRawMSE = metric.IntegerMeanMSE - metric.RawMeanMSE
	for _, value := range errorsCP {
		metric.MeanAbsoluteQuantizationCP += value
	}
	metric.MeanAbsoluteQuantizationCP /= float64(metric.Positions)
	metric.P99AbsoluteQuantizationCP = errorsCP[(99*metric.Positions+99)/100-1]
	metric.MaximumAbsoluteQuantizationCP = errorsCP[len(errorsCP)-1]
	safe := true
	for _, value := range metric.FastOutputSafe {
		safe = safe && value
	}
	metric.Eligible = metric.Update > 0 && safe &&
		metric.MeanAbsoluteQuantizationCP <= meanQuantDeltaGate &&
		metric.P99AbsoluteQuantizationCP <= p99QuantDeltaGate &&
		metric.MaximumAbsoluteQuantizationCP <= maxQuantDeltaGate
	return metric, nil
}

func loadSelectionCandidates(
	root, manifestPath string,
	manifestBytes int64,
	manifestSHA [sha256.Size]byte,
	validation []byte,
	mode selectionMode,
) ([]candidateMetric, error) {
	initialLR, finalLR := float32(0.001), float32(0.00005)
	if mode.name == "pilot-lr1" || mode.name == "probe5m-lr1" || mode.name == "main-m1-lr1" || mode.name == "main-m1-lr1-32k" {
		initialLR, finalLR = 0.0002, 0.00001
	}
	if mode.manifestMode == "archive" {
		initialLR, finalLR = 0.001, 0.00001
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var metrics []candidateMetric
	var expectedOptimizerLayout []optimizerTensorSpec
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "candidate-") {
			continue
		}
		update, err := strconv.ParseUint(strings.TrimPrefix(entry.Name(), "candidate-"), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid candidate directory %q", entry.Name())
		}
		directory := filepath.Join(root, entry.Name())
		receiptPath := filepath.Join(directory, "receipt.json")
		receiptBytes, err := os.ReadFile(receiptPath)
		if err != nil {
			return nil, err
		}
		var checkpoint selectionCheckpoint
		if err := json.Unmarshal(receiptBytes, &checkpoint); err != nil {
			return nil, fmt.Errorf("decode %s: %w", receiptPath, err)
		}
		if checkpoint.Schema != checkpointSchema || checkpoint.BulletCommit != bulletCommit ||
			checkpoint.BulletPatchSHA256 != bulletPatchSHA || checkpoint.Mode != mode.name ||
			checkpoint.CompletedUpdates != update || checkpoint.TotalUpdates != uint64(mode.totalUpdates) ||
			checkpoint.CheckpointUpdates != uint64(mode.checkpointUpdates) || checkpoint.InputBuckets != ngnk4.InputBuckets ||
			checkpoint.Hidden != ngnk4.HiddenSize || checkpoint.OutputBuckets != ngnk4.OutputBuckets ||
			checkpoint.ModelSeed != 26_092_001 || checkpoint.ScoreScale != int(selectionScoreScale) ||
			checkpoint.InitialLR != initialLR || checkpoint.FinalLR != finalLR ||
			!samePath(checkpoint.FinalizedManifest.Path, manifestPath) ||
			checkpoint.FinalizedManifest.Bytes != manifestBytes || checkpoint.FinalizedManifest.SHA256 != hex.EncodeToString(manifestSHA[:]) {
			return nil, fmt.Errorf("checkpoint contract mismatch %s", receiptPath)
		}
		rawPath := filepath.Join(directory, "raw.bin")
		quantisedPath := filepath.Join(directory, "quantised.bin")
		rawData, err := verifySelectionFile(checkpoint.Files.Raw, rawPath, deployedRawBytes)
		if err != nil {
			return nil, fmt.Errorf("verify raw update %d: %w", update, err)
		}
		quantisedData, err := verifySelectionFile(checkpoint.Files.Quantised, quantisedPath, inputBytes)
		if err != nil {
			return nil, fmt.Errorf("verify quantised update %d: %w", update, err)
		}
		optimizerItems := []struct {
			name string
			item selectionFileReceipt
			path string
		}{
			{"weights", checkpoint.Files.Weights, filepath.Join(directory, "optimiser_state", "weights.bin")},
			{"momentum", checkpoint.Files.Momentum, filepath.Join(directory, "optimiser_state", "momentum.bin")},
			{"velocity", checkpoint.Files.Velocity, filepath.Join(directory, "optimiser_state", "velocity.bin")},
		}
		var optimizerLayout []optimizerTensorSpec
		for optimizerIndex, optimizer := range optimizerItems {
			layout, err := verifyOptimizerState(optimizer.item, optimizer.path)
			if err != nil {
				return nil, fmt.Errorf("verify %s update %d: %w", optimizer.name, update, err)
			}
			if optimizerIndex == 0 {
				optimizerLayout = layout
			} else if !sameOptimizerLayout(optimizerLayout, layout) {
				return nil, fmt.Errorf("optimizer state layout mismatch for %s update %d", optimizer.name, update)
			}
		}
		if expectedOptimizerLayout == nil {
			expectedOptimizerLayout = optimizerLayout
		} else if !sameOptimizerLayout(expectedOptimizerLayout, optimizerLayout) {
			return nil, fmt.Errorf("optimizer state layout mismatch across checkpoints at update %d", update)
		}
		metric := candidateMetric{
			Update: update, Checkpoint: selectionReceiptForBytes(receiptPath, receiptBytes),
			Raw: checkpoint.Files.Raw, Quantised: checkpoint.Files.Quantised,
			OptimizerStateFinite: true,
		}
		metric, err = evaluateSelectionCandidate(metric, rawData, quantisedData, validation, manifestSHA)
		if err != nil {
			return nil, fmt.Errorf("evaluate update %d: %w", update, err)
		}
		metrics = append(metrics, metric)
	}
	sort.Slice(metrics, func(i, j int) bool { return metrics[i].Update < metrics[j].Update })
	if len(metrics) != mode.totalUpdates/mode.checkpointUpdates+1 {
		return nil, fmt.Errorf("candidate count %d, want %d", len(metrics), mode.totalUpdates/mode.checkpointUpdates+1)
	}
	for index, metric := range metrics {
		if metric.Update != uint64(index*mode.checkpointUpdates) {
			return nil, fmt.Errorf("candidate update sequence mismatch at %d: %d", index, metric.Update)
		}
	}
	return metrics, nil
}

func chooseSelectionCandidate(metrics []candidateMetric) (candidateMetric, earlyStopReceipt, error) {
	mode, _ := selectionModeFor("pilot")
	return chooseSelectionCandidateForMode(metrics, mode)
}

func chooseSelectionCandidateForMode(metrics []candidateMetric, mode selectionMode) (candidateMetric, earlyStopReceipt, error) {
	stop := earlyStopReceipt{
		MinimumUpdate: mode.minimumUpdate, Patience: mode.patience,
		MinimumIntegerMSEImprovement: selectionImprovementGate,
	}
	bestLoss := math.Inf(1)
	withoutImprovement := 0
	limit := uint64(mode.totalUpdates)
	for _, metric := range metrics {
		if metric.Update == 0 {
			continue
		}
		if metric.Eligible && metric.IntegerMeanMSE <= bestLoss-selectionImprovementGate {
			bestLoss = metric.IntegerMeanMSE
			withoutImprovement = 0
		} else {
			withoutImprovement++
		}
		if metric.Update >= stop.MinimumUpdate && withoutImprovement >= stop.Patience {
			stop.Triggered = true
			stop.StopUpdate = metric.Update
			limit = metric.Update
			break
		}
	}
	var selected *candidateMetric
	for index := range metrics {
		metric := &metrics[index]
		if metric.Update == 0 || metric.Update > limit || !metric.Eligible {
			continue
		}
		if selected == nil || metric.IntegerMeanMSE < selected.IntegerMeanMSE {
			selected = metric
		}
	}
	if selected == nil {
		return candidateMetric{}, stop, errors.New("no eligible trained checkpoint")
	}
	return *selected, stop, nil
}

func runSelect(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("select", flag.ContinueOnError)
	flags.SetOutput(stderr)
	modeName := flags.String("mode", "pilot", "selection mode: pilot, pilot-s1, pilot-lr1, probe5m, probe5m-lr1, main-m1, main-m1-lr1, main-m1-lr1-32k, archive-a1-e1, archive-a1-e10, or archive-a1-e20")
	candidates := flags.String("candidates", "", "pilot candidate directory")
	manifestPath := flags.String("manifest", "", "frozen finalized-pilot manifest")
	validationPath := flags.String("validation", "", "frozen validation BF")
	outputPath := flags.String("out", "", "new JSON selection receipt")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || *candidates == "" || *manifestPath == "" || *validationPath == "" || *outputPath == "" {
		return errors.New("select requires -candidates, -manifest, -validation, and -out")
	}
	mode, err := selectionModeFor(*modeName)
	if err != nil {
		return err
	}
	if _, err := os.Stat(*outputPath); !os.IsNotExist(err) {
		return fmt.Errorf("refusing existing output %s", *outputPath)
	}
	_, manifestSHA, manifestReceipt, validation, err := loadSelectionManifest(*manifestPath, *validationPath, mode)
	if err != nil {
		return err
	}
	metrics, err := loadSelectionCandidates(*candidates, *manifestPath, manifestReceipt.Bytes, manifestSHA, validation, mode)
	if err != nil {
		return err
	}
	selected, stop, err := chooseSelectionCandidateForMode(metrics, mode)
	if err != nil {
		return err
	}
	receipt := selectionReceipt{
		Schema: mode.schema, ContractVersion: finalizedContract,
		Command:    append([]string{"ngnk4bridge", "select"}, args...),
		Manifest:   manifestReceipt,
		Validation: selectionReceiptForBytes(*validationPath, validation),
		Rank:       "lowest eligible validation integer MSE through the predeclared early-stop point",
		TieBreak:   "earliest update",
		MeanGateCP: meanQuantDeltaGate, P99GateCP: p99QuantDeltaGate, MaximumGateCP: maxQuantDeltaGate,
		EarlyStop: stop, Candidates: metrics, SelectedUpdate: selected.Update, Selected: selected.Checkpoint,
	}
	if mode.name != "pilot" {
		receipt.Mode = mode.name
	}
	encoded, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	if err := writeAtomicNew(*outputPath, encoded); err != nil {
		return err
	}
	_, err = stdout.Write(encoded)
	return err
}
