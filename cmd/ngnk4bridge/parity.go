package main

import (
	"bufio"
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
	"sort"
	"strconv"
	"strings"

	"github.com/ehrlich-b/ngn/engine"
	"github.com/ehrlich-b/ngn/nnue/ngnk4"
)

const (
	deployedRawBytes   = ngnk4.PayloadSize / 2 * 4
	maximumProbeDelta  = 0.0002
	meanQuantDeltaGate = 8.0
	p99QuantDeltaGate  = 32.0
	maxQuantDeltaGate  = 64.0
)

type floatTensors struct {
	inputWeights  [ngnk4.TotalInputFeatures][ngnk4.HiddenSize]float32
	inputBiases   [ngnk4.HiddenSize]float32
	outputWeights [ngnk4.OutputBuckets][2][ngnk4.HiddenSize]float32
	outputBiases  [ngnk4.OutputBuckets]float32
}

type probeRecord struct {
	Z   float64
	FEN string
}

type k4ParityRecord struct {
	FEN                  string  `json:"fen"`
	OutputBucket         int     `json:"output_bucket"`
	BulletZ              float64 `json:"bullet_z"`
	ReferenceZ           float64 `json:"reference_z"`
	ProbeDeltaZ          float64 `json:"probe_delta_z"`
	ReferenceCP          float64 `json:"reference_cp"`
	GoScalarCP           int64   `json:"go_scalar_cp"`
	GoContextCP          int64   `json:"go_context_cp"`
	QuantizationDeltaCP  float64 `json:"quantization_delta_cp"`
	ScalarContextMatch   bool    `json:"scalar_context_match"`
	WithinProbeTolerance bool    `json:"within_probe_tolerance"`
	WithinQuantTolerance bool    `json:"within_quantization_tolerance"`
	Pass                 bool    `json:"pass"`
}

type k4ParityReport struct {
	BridgeVersion          string           `json:"bridge_version"`
	BulletCommit           string           `json:"bullet_commit"`
	BulletPatchSHA256      string           `json:"bullet_patch_sha256"`
	Command                []string         `json:"command"`
	RawPath                string           `json:"raw_path"`
	RawSHA256              string           `json:"raw_sha256"`
	ModelPath              string           `json:"model_path"`
	ModelSHA256            string           `json:"model_sha256"`
	ManifestSHA256         string           `json:"manifest_sha256"`
	FENPath                string           `json:"fen_path"`
	FENSHA256              string           `json:"fen_sha256"`
	ProbePath              string           `json:"probe_path"`
	ProbeSHA256            string           `json:"probe_sha256"`
	TensorQuantizationPass bool             `json:"tensor_quantization_pass"`
	MaximumProbeDeltaZ     float64          `json:"maximum_probe_delta_z"`
	MeanQuantDeltaCP       float64          `json:"mean_absolute_quantization_delta_cp"`
	P99QuantDeltaCP        float64          `json:"p99_absolute_quantization_delta_cp"`
	MaximumQuantDeltaCP    float64          `json:"maximum_absolute_quantization_delta_cp"`
	ProbeToleranceZ        float64          `json:"probe_tolerance_z"`
	MeanQuantGateCP        float64          `json:"mean_absolute_quantization_gate_cp"`
	P99QuantGateCP         float64          `json:"p99_absolute_quantization_gate_cp"`
	MaximumQuantGateCP     float64          `json:"maximum_absolute_quantization_gate_cp"`
	Pass                   bool             `json:"pass"`
	Records                []k4ParityRecord `json:"records"`
}

func readBounded(path string, size int) ([]byte, [sha256.Size]byte, error) {
	data, err := readExactFile(path, size)
	if err != nil {
		return nil, [sha256.Size]byte{}, err
	}
	return data, sha256.Sum256(data), nil
}

func decodeRaw(data []byte) (*floatTensors, error) {
	if len(data) != deployedRawBytes {
		return nil, fmt.Errorf("deployed raw bytes %d, want %d", len(data), deployedRawBytes)
	}
	tensors := new(floatTensors)
	offset := 0
	read := func() (float32, error) {
		value := math.Float32frombits(binary.LittleEndian.Uint32(data[offset : offset+4]))
		offset += 4
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			return 0, fmt.Errorf("nonfinite deployed raw tensor index %d", offset/4-1)
		}
		return value, nil
	}
	for input := range tensors.inputWeights {
		for hidden := range tensors.inputWeights[input] {
			value, err := read()
			if err != nil {
				return nil, err
			}
			tensors.inputWeights[input][hidden] = value
		}
	}
	for hidden := range tensors.inputBiases {
		value, err := read()
		if err != nil {
			return nil, err
		}
		tensors.inputBiases[hidden] = value
	}
	for bucket := range tensors.outputWeights {
		for perspective := range tensors.outputWeights[bucket] {
			for hidden := range tensors.outputWeights[bucket][perspective] {
				value, err := read()
				if err != nil {
					return nil, err
				}
				tensors.outputWeights[bucket][perspective][hidden] = value
			}
		}
	}
	for bucket := range tensors.outputBiases {
		value, err := read()
		if err != nil {
			return nil, err
		}
		tensors.outputBiases[bucket] = value
	}
	if offset != len(data) {
		return nil, fmt.Errorf("decoded raw bytes %d, want %d", offset, len(data))
	}
	return tensors, nil
}

func verifyRawQuantization(raw, modelFile []byte) error {
	if len(raw) != deployedRawBytes || len(modelFile) != ngnk4.FileSize {
		return errors.New("raw/model size mismatch before tensor verification")
	}
	payload := modelFile[ngnk4.HeaderSize:]
	sections := []struct {
		count int
		scale float64
	}{
		{ngnk4.TotalInputFeatures * ngnk4.HiddenSize, ngnk4.InputScale},
		{ngnk4.HiddenSize, ngnk4.InputScale},
		{ngnk4.OutputBuckets * 2 * ngnk4.HiddenSize, ngnk4.LayerScale},
		{ngnk4.OutputBuckets, ngnk4.InputScale * ngnk4.LayerScale},
	}
	index := 0
	for _, section := range sections {
		for remaining := section.count; remaining > 0; remaining-- {
			value := math.Float32frombits(binary.LittleEndian.Uint32(raw[index*4:]))
			quantized := math.Round(float64(value) * section.scale)
			if quantized < math.MinInt16 || quantized > math.MaxInt16 {
				return fmt.Errorf("tensor %d quantizes outside i16", index)
			}
			got := int16(binary.LittleEndian.Uint16(payload[index*2:]))
			if got != int16(quantized) {
				return fmt.Errorf("tensor %d quantized=%d, want %d", index, got, int16(quantized))
			}
			index++
		}
	}
	if index*4 != len(raw) || index*2 != len(payload) {
		return fmt.Errorf("verified tensor count %d does not consume raw/model payload", index)
	}
	return nil
}

func readFENs(path string) ([]string, [sha256.Size]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, [sha256.Size]byte{}, err
	}
	defer file.Close()
	hash := sha256.New()
	scanner := bufio.NewScanner(io.TeeReader(file, hash))
	scanner.Buffer(make([]byte, 4096), 1<<20)
	var fens []string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			fens = append(fens, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, [sha256.Size]byte{}, err
	}
	if len(fens) == 0 {
		return nil, [sha256.Size]byte{}, errors.New("FEN file contains no positions")
	}
	var digest [sha256.Size]byte
	copy(digest[:], hash.Sum(nil))
	return fens, digest, nil
}

func readProbe(path string, fens []string) ([]probeRecord, [sha256.Size]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, [sha256.Size]byte{}, err
	}
	records := make([]probeRecord, 0, len(fens))
	passLine := false
	expectedPass := fmt.Sprintf(
		"NGN_K4_PROBE_PASS bullet_commit=%s patch=%s positions=%d",
		bulletCommit, bulletPatchSHA, len(fens),
	)
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "NGN_K4_PROBE_PASS ") {
			if line != expectedPass {
				return nil, [sha256.Size]byte{}, fmt.Errorf("invalid probe pass line %q", line)
			}
			passLine = true
			continue
		}
		if !strings.HasPrefix(line, "NGN_K4_PROBE\t") {
			continue
		}
		fields := strings.SplitN(line, "\t", 4)
		if len(fields) != 4 {
			return nil, [sha256.Size]byte{}, fmt.Errorf("invalid probe line %q", line)
		}
		index, err := strconv.Atoi(fields[1])
		if err != nil || index != len(records) || index >= len(fens) {
			return nil, [sha256.Size]byte{}, fmt.Errorf("invalid probe index %q", fields[1])
		}
		value, err := strconv.ParseFloat(fields[2], 64)
		if err != nil || math.IsNaN(value) || math.IsInf(value, 0) || fields[3] != fens[index] {
			return nil, [sha256.Size]byte{}, fmt.Errorf("probe record %d does not match FEN/value", index)
		}
		records = append(records, probeRecord{Z: value, FEN: fields[3]})
	}
	if err := scanner.Err(); err != nil {
		return nil, [sha256.Size]byte{}, err
	}
	if len(records) != len(fens) || !passLine {
		return nil, [sha256.Size]byte{}, fmt.Errorf("probe records=%d pass_line=%t, want %d/true", len(records), passLine, len(fens))
	}
	return records, sha256.Sum256(data), nil
}

func productionPosition(position *engine.Position) ngnk4.Position {
	var board ngnk4.Board
	for piece := engine.WhitePawn; piece <= engine.BlackKing; piece++ {
		board[int(piece-engine.WhitePawn)] = position.Board.GetBitboardOf(piece)
	}
	side := ngnk4.White
	if position.Turn() == engine.Black {
		side = ngnk4.Black
	}
	return ngnk4.Position{Board: board, SideToMove: side}
}

func referenceFeatureIndex(color ngnk4.Color, pieceType, square, kingSquare int, perspective ngnk4.Color) int {
	orientedSquare := square
	orientedKing := kingSquare
	if perspective == ngnk4.Black {
		orientedSquare ^= 56
		orientedKing ^= 56
	}
	if kingSquare%8 > 3 {
		orientedSquare ^= 7
	}
	return referenceKingBuckets[orientedKing]*ngnk4.InputSize +
		int(color^perspective)*384 + pieceType*64 + orientedSquare
}

func referenceOutputBucket(board ngnk4.Board) int {
	var occupied uint64
	for _, pieces := range board {
		occupied |= pieces
	}
	bucket := (bits.OnesCount64(occupied) - 2) / 4
	if bucket < 0 {
		return 0
	}
	if bucket >= ngnk4.OutputBuckets {
		return ngnk4.OutputBuckets - 1
	}
	return bucket
}

func evaluateFloat(tensors *floatTensors, position ngnk4.Position) (float64, int) {
	var accumulators [2][ngnk4.HiddenSize]float32
	for perspective := range accumulators {
		copy(accumulators[perspective][:], tensors.inputBiases[:])
	}
	kingSquares := [2]int{
		bits.TrailingZeros64(position.Board[ngnk4.WhiteKing]),
		bits.TrailingZeros64(position.Board[ngnk4.BlackKing]),
	}
	for plane, pieceBits := range position.Board {
		color := ngnk4.Color(plane / 6)
		pieceType := plane % 6
		for pieceBits != 0 {
			square := bits.TrailingZeros64(pieceBits)
			pieceBits &= pieceBits - 1
			for perspective := ngnk4.White; perspective <= ngnk4.Black; perspective++ {
				row := &tensors.inputWeights[referenceFeatureIndex(color, pieceType, square, kingSquares[perspective], perspective)]
				for hidden, weight := range row {
					accumulators[perspective][hidden] += weight
				}
			}
		}
	}
	bucket := referenceOutputBucket(position.Board)
	stm := position.SideToMove
	nonSTM := stm ^ 1
	var output float32
	for hidden := 0; hidden < ngnk4.HiddenSize; hidden++ {
		for perspective, values := range [2]float32{
			accumulators[stm][hidden], accumulators[nonSTM][hidden],
		} {
			value := min(max(values, 0), 1)
			output += value * value * tensors.outputWeights[bucket][perspective][hidden]
		}
	}
	output += tensors.outputBiases[bucket]
	return float64(output), bucket
}

func absoluteFloat(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}

func runParity(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("parity", flag.ContinueOnError)
	flags.SetOutput(stderr)
	rawPath := flags.String("raw", "", "deployed transformed f32 tensor stream")
	modelPath := flags.String("model", "", "strict NGN K4 model")
	fenPath := flags.String("fens", "", "newline-delimited FEN corpus")
	probePath := flags.String("probe", "", "pinned Bullet probe output")
	receiptPath := flags.String("out", "", "optional new JSON receipt path")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || *rawPath == "" || *modelPath == "" || *fenPath == "" || *probePath == "" {
		return errors.New("parity requires -raw, -model, -fens, and -probe")
	}
	if *receiptPath != "" {
		if _, err := os.Stat(*receiptPath); err == nil {
			return fmt.Errorf("refusing existing parity receipt %q", *receiptPath)
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	raw, rawSHA, err := readBounded(*rawPath, deployedRawBytes)
	if err != nil {
		return fmt.Errorf("read deployed raw: %w", err)
	}
	tensors, err := decodeRaw(raw)
	if err != nil {
		return err
	}
	modelFile, modelSHA, err := readBounded(*modelPath, ngnk4.FileSize)
	if err != nil {
		return fmt.Errorf("read model: %w", err)
	}
	model, err := ngnk4.Load(bytes.NewReader(modelFile))
	if err != nil {
		return err
	}
	if err := verifyRawQuantization(raw, modelFile); err != nil {
		return fmt.Errorf("raw/model tensor mismatch: %w", err)
	}
	fens, fenSHA, err := readFENs(*fenPath)
	if err != nil {
		return err
	}
	probes, probeSHA, err := readProbe(*probePath, fens)
	if err != nil {
		return err
	}
	metadata := model.Metadata()
	report := k4ParityReport{
		BridgeVersion:          bridgeVersion,
		BulletCommit:           bulletCommit,
		BulletPatchSHA256:      bulletPatchSHA,
		Command:                append([]string{"ngnk4bridge", "parity"}, args...),
		RawPath:                *rawPath,
		RawSHA256:              hex.EncodeToString(rawSHA[:]),
		ModelPath:              *modelPath,
		ModelSHA256:            hex.EncodeToString(modelSHA[:]),
		ManifestSHA256:         hex.EncodeToString(metadata.ManifestSHA256[:]),
		FENPath:                *fenPath,
		FENSHA256:              hex.EncodeToString(fenSHA[:]),
		ProbePath:              *probePath,
		ProbeSHA256:            hex.EncodeToString(probeSHA[:]),
		TensorQuantizationPass: true,
		ProbeToleranceZ:        maximumProbeDelta,
		MeanQuantGateCP:        meanQuantDeltaGate,
		P99QuantGateCP:         p99QuantDeltaGate,
		MaximumQuantGateCP:     maxQuantDeltaGate,
		Pass:                   true,
		Records:                make([]k4ParityRecord, 0, len(fens)),
	}
	absoluteQuantDeltas := make([]float64, 0, len(fens))
	for index, fen := range fens {
		enginePosition, err := engine.ParseFEN(fen)
		if err != nil {
			return fmt.Errorf("parse FEN %d: %w", index, err)
		}
		position := productionPosition(enginePosition)
		referenceZ, bucket := evaluateFloat(tensors, position)
		scalar, err := model.EvaluateRaw(position)
		if err != nil {
			return err
		}
		context, err := model.NewSearchContext(position)
		if err != nil {
			return err
		}
		contextScore, err := context.EvaluateRaw()
		if err != nil {
			return err
		}
		probeDelta := referenceZ - probes[index].Z
		referenceCP := referenceZ * ngnk4.OutputScale
		quantDelta := float64(scalar) - referenceCP
		record := k4ParityRecord{
			FEN: fen, OutputBucket: bucket, BulletZ: probes[index].Z, ReferenceZ: referenceZ,
			ProbeDeltaZ: probeDelta, ReferenceCP: referenceCP, GoScalarCP: scalar,
			GoContextCP: contextScore, QuantizationDeltaCP: quantDelta,
			ScalarContextMatch:   scalar == contextScore,
			WithinProbeTolerance: absoluteFloat(probeDelta) <= maximumProbeDelta,
			WithinQuantTolerance: absoluteFloat(quantDelta) <= maxQuantDeltaGate,
		}
		record.Pass = record.ScalarContextMatch && record.WithinProbeTolerance && record.WithinQuantTolerance
		if !record.Pass {
			report.Pass = false
		}
		if delta := absoluteFloat(probeDelta); delta > report.MaximumProbeDeltaZ {
			report.MaximumProbeDeltaZ = delta
		}
		if delta := absoluteFloat(quantDelta); delta > report.MaximumQuantDeltaCP {
			report.MaximumQuantDeltaCP = delta
		}
		absoluteQuantDeltas = append(absoluteQuantDeltas, absoluteFloat(quantDelta))
		report.Records = append(report.Records, record)
	}
	for _, delta := range absoluteQuantDeltas {
		report.MeanQuantDeltaCP += delta / float64(len(absoluteQuantDeltas))
	}
	sort.Float64s(absoluteQuantDeltas)
	p99Index := int(math.Ceil(0.99*float64(len(absoluteQuantDeltas)))) - 1
	report.P99QuantDeltaCP = absoluteQuantDeltas[p99Index]
	if report.MeanQuantDeltaCP > meanQuantDeltaGate || report.P99QuantDeltaCP > p99QuantDeltaGate ||
		report.MaximumQuantDeltaCP > maxQuantDeltaGate {
		report.Pass = false
	}
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	if _, err := stdout.Write(encoded); err != nil {
		return err
	}
	if *receiptPath != "" {
		if err := writeAtomicNew(*receiptPath, encoded); err != nil {
			return err
		}
	}
	if !report.Pass {
		return errors.New("NGN K4 parity gate failed")
	}
	return nil
}
