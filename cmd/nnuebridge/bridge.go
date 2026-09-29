package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"

	"github.com/ehrlich-b/ngn/engine"
	"github.com/ehrlich-b/ngn/nnue"
)

const (
	bulletQuantizedSize = nnue.PayloadSize + bulletPaddingSize
	bulletPaddingSize   = 62
	bulletRawSize       = (nnue.InputSize*nnue.HiddenSize + nnue.HiddenSize + nnue.PerspectiveCount*nnue.HiddenSize + 1) * 4
)

var (
	errBulletTransport = errors.New("invalid Bullet simple-network transport")
	bulletPadWord      = [...]byte{'b', 'u', 'l', 'l', 'e', 't'}
)

type sourceFormat string

const (
	formatBulletQuantized sourceFormat = "bullet-quantized"
	formatBulletRaw       sourceFormat = "bullet-raw"
)

type quantizedNetwork struct {
	featureWeights [nnue.InputSize][nnue.HiddenSize]int16
	featureBias    [nnue.HiddenSize]int16
	outputWeights  [nnue.PerspectiveCount * nnue.HiddenSize]int16
	outputBias     int16
}

type floatNetwork struct {
	featureWeights [nnue.InputSize][nnue.HiddenSize]float32
	featureBias    [nnue.HiddenSize]float32
	outputWeights  [nnue.PerspectiveCount * nnue.HiddenSize]float32
	outputBias     float32
}

type referencePiece struct {
	piece  uint8
	color  uint8
	square uint8
}

type referencePosition struct {
	sideToMove uint8
	pieces     []referencePiece
}

func readExact(reader io.Reader, size int) ([]byte, error) {
	if reader == nil {
		return nil, fmt.Errorf("%w: nil reader", errBulletTransport)
	}
	data := make([]byte, size)
	if _, err := io.ReadFull(reader, data); err != nil {
		return nil, fmt.Errorf("%w: need exactly %d bytes: %v", errBulletTransport, size, err)
	}
	var extra [1]byte
	n, err := io.ReadFull(reader, extra[:])
	if n != 0 {
		return nil, fmt.Errorf("%w: trailing data after %d bytes", errBulletTransport, size)
	}
	if err != io.EOF {
		return nil, fmt.Errorf("%w: trailing-data probe: %v", errBulletTransport, err)
	}
	return data, nil
}

func readBulletQuantized(reader io.Reader) (*quantizedNetwork, error) {
	data, err := readExact(reader, bulletQuantizedSize)
	if err != nil {
		return nil, err
	}
	padding := data[nnue.PayloadSize:]
	for i, value := range padding {
		if want := bulletPadWord[i%len(bulletPadWord)]; value != want {
			return nil, fmt.Errorf("%w: padding byte %d is %#x, want %#x", errBulletTransport, i, value, want)
		}
	}

	network := new(quantizedNetwork)
	at := 0
	for feature := 0; feature < nnue.InputSize; feature++ {
		for hidden := 0; hidden < nnue.HiddenSize; hidden++ {
			network.featureWeights[feature][hidden] = int16(binary.LittleEndian.Uint16(data[at:]))
			at += 2
		}
	}
	for hidden := 0; hidden < nnue.HiddenSize; hidden++ {
		network.featureBias[hidden] = int16(binary.LittleEndian.Uint16(data[at:]))
		at += 2
	}
	for hidden := 0; hidden < nnue.PerspectiveCount*nnue.HiddenSize; hidden++ {
		network.outputWeights[hidden] = int16(binary.LittleEndian.Uint16(data[at:]))
		at += 2
	}
	network.outputBias = int16(binary.LittleEndian.Uint16(data[at:]))
	at += 2
	if at != nnue.PayloadSize {
		panic("nnuebridge: internal quantized layout mismatch")
	}
	return network, nil
}

func readBulletRaw(reader io.Reader) (*floatNetwork, error) {
	data, err := readExact(reader, bulletRawSize)
	if err != nil {
		return nil, err
	}
	network := new(floatNetwork)
	at := 0
	read := func(label string) (float32, error) {
		value := math.Float32frombits(binary.LittleEndian.Uint32(data[at:]))
		at += 4
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			return 0, fmt.Errorf("%w: %s is non-finite", errBulletTransport, label)
		}
		return value, nil
	}
	for feature := 0; feature < nnue.InputSize; feature++ {
		for hidden := 0; hidden < nnue.HiddenSize; hidden++ {
			value, err := read(fmt.Sprintf("feature weight [%d][%d]", feature, hidden))
			if err != nil {
				return nil, err
			}
			network.featureWeights[feature][hidden] = value
		}
	}
	for hidden := 0; hidden < nnue.HiddenSize; hidden++ {
		value, err := read(fmt.Sprintf("feature bias [%d]", hidden))
		if err != nil {
			return nil, err
		}
		network.featureBias[hidden] = value
	}
	for hidden := 0; hidden < nnue.PerspectiveCount*nnue.HiddenSize; hidden++ {
		value, err := read(fmt.Sprintf("output weight [%d]", hidden))
		if err != nil {
			return nil, err
		}
		network.outputWeights[hidden] = value
	}
	value, err := read("output bias")
	if err != nil {
		return nil, err
	}
	network.outputBias = value
	if at != bulletRawSize {
		panic("nnuebridge: internal raw layout mismatch")
	}
	return network, nil
}

func quantizeValueReference(value float32, multiplier int) (int16, error) {
	if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
		return 0, fmt.Errorf("%w: non-finite quantization input", errBulletTransport)
	}
	// Bullet 629ee500 promotes both the stored f32 and multiplier to f64
	// before Rust f64::round, whose halfway rule is away from zero.
	rounded := math.Round(float64(value) * float64(multiplier))
	if rounded < math.MinInt16 || rounded > math.MaxInt16 {
		return 0, fmt.Errorf("%w: rounded value %.0f exceeds i16", errBulletTransport, rounded)
	}
	return int16(rounded), nil
}

func quantizeReference(input *floatNetwork) (*quantizedNetwork, error) {
	if input == nil {
		return nil, fmt.Errorf("%w: nil float network", errBulletTransport)
	}
	output := new(quantizedNetwork)
	for feature := 0; feature < nnue.InputSize; feature++ {
		for hidden := 0; hidden < nnue.HiddenSize; hidden++ {
			value, err := quantizeValueReference(input.featureWeights[feature][hidden], nnue.QA)
			if err != nil {
				return nil, fmt.Errorf("feature weight [%d][%d]: %w", feature, hidden, err)
			}
			output.featureWeights[feature][hidden] = value
		}
	}
	for hidden := 0; hidden < nnue.HiddenSize; hidden++ {
		value, err := quantizeValueReference(input.featureBias[hidden], nnue.QA)
		if err != nil {
			return nil, fmt.Errorf("feature bias [%d]: %w", hidden, err)
		}
		output.featureBias[hidden] = value
	}
	for hidden := 0; hidden < nnue.PerspectiveCount*nnue.HiddenSize; hidden++ {
		value, err := quantizeValueReference(input.outputWeights[hidden], nnue.QB)
		if err != nil {
			return nil, fmt.Errorf("output weight [%d]: %w", hidden, err)
		}
		output.outputWeights[hidden] = value
	}
	value, err := quantizeValueReference(input.outputBias, nnue.QA*nnue.QB)
	if err != nil {
		return nil, fmt.Errorf("output bias: %w", err)
	}
	output.outputBias = value
	return output, nil
}

func tensorsFromQuantized(input *quantizedNetwork) *nnue.Tensors {
	output := new(nnue.Tensors)
	output.FeatureWeights = input.featureWeights
	output.FeatureBias = input.featureBias
	output.OutputWeights = input.outputWeights
	output.OutputBias = input.outputBias
	return output
}

func floatTensorsForProduction(input *floatNetwork) *nnue.FloatTensors {
	output := new(nnue.FloatTensors)
	output.FeatureWeights = input.featureWeights
	output.FeatureBias = input.featureBias
	output.OutputWeights = input.outputWeights
	output.OutputBias = input.outputBias
	return output
}

func marshalAndValidate(tensors *nnue.Tensors) ([]byte, *nnue.Model, error) {
	encoded, err := nnue.Marshal(tensors)
	if err != nil {
		return nil, nil, err
	}
	model, err := nnue.Load(bytes.NewReader(encoded))
	if err != nil {
		return nil, nil, fmt.Errorf("self-validate NGN v1: %w", err)
	}
	return encoded, model, nil
}

func convertBullet(reader io.Reader, format sourceFormat) ([]byte, *nnue.Model, *quantizedNetwork, *floatNetwork, error) {
	switch format {
	case formatBulletQuantized:
		reference, err := readBulletQuantized(reader)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		encoded, model, err := marshalAndValidate(tensorsFromQuantized(reference))
		return encoded, model, reference, nil, err
	case formatBulletRaw:
		raw, err := readBulletRaw(reader)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		// Conversion deliberately exercises the production N0 quantizer.
		tensors, err := nnue.Quantize(floatTensorsForProduction(raw))
		if err != nil {
			return nil, nil, nil, nil, err
		}
		independent, err := quantizeReference(raw)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		if *tensors != *tensorsFromQuantized(independent) {
			return nil, nil, nil, nil, fmt.Errorf("production and independent quantizers disagree")
		}
		encoded, model, err := marshalAndValidate(tensors)
		return encoded, model, independent, raw, err
	default:
		return nil, nil, nil, nil, fmt.Errorf("%w: unsupported source format %q", errBulletTransport, format)
	}
}

func referenceFeature(piece referencePiece, perspective uint8) int {
	relativeColor := 1
	if piece.color == perspective {
		relativeColor = 0
	}
	square := int(piece.square)
	if perspective == uint8(nnue.Black) {
		square ^= 56
	}
	return relativeColor*384 + int(piece.piece)*64 + square
}

func evaluateQuantizedReference(network *quantizedNetwork, position referencePosition) int64 {
	var accumulators [nnue.PerspectiveCount][nnue.HiddenSize]int64
	for perspective := 0; perspective < nnue.PerspectiveCount; perspective++ {
		for hidden, bias := range network.featureBias {
			accumulators[perspective][hidden] = int64(bias)
		}
		for _, piece := range position.pieces {
			feature := referenceFeature(piece, uint8(perspective))
			for hidden, weight := range network.featureWeights[feature] {
				accumulators[perspective][hidden] += int64(weight)
			}
		}
	}
	us := int(position.sideToMove)
	them := us ^ 1
	var dot int64
	for hidden := 0; hidden < nnue.HiddenSize; hidden++ {
		dot += screluReference(accumulators[us][hidden]) * int64(network.outputWeights[hidden])
		dot += screluReference(accumulators[them][hidden]) * int64(network.outputWeights[nnue.HiddenSize+hidden])
	}
	value := dot / int64(nnue.QA)
	value += int64(network.outputBias)
	value *= int64(nnue.ScoreScale)
	return value / int64(nnue.QA*nnue.QB)
}

func screluReference(value int64) int64 {
	if value <= 0 {
		return 0
	}
	if value >= nnue.QA {
		return nnue.QA * nnue.QA
	}
	return value * value
}

// evaluateFloatReference is a deterministic portable f32 diagnostic. It uses
// ascending-square feature order and ascending-hidden output order. Exact CUDA
// reduction parity remains a later trainer-pilot gate; integer parity is exact.
func evaluateFloatReference(network *floatNetwork, position referencePosition) float32 {
	var accumulators [nnue.PerspectiveCount][nnue.HiddenSize]float32
	for perspective := 0; perspective < nnue.PerspectiveCount; perspective++ {
		copy(accumulators[perspective][:], network.featureBias[:])
		for _, piece := range position.pieces {
			feature := referenceFeature(piece, uint8(perspective))
			for hidden, weight := range network.featureWeights[feature] {
				accumulators[perspective][hidden] += weight
			}
		}
	}
	us := int(position.sideToMove)
	them := us ^ 1
	var output float32
	for hidden := 0; hidden < nnue.HiddenSize; hidden++ {
		output += floatScrelu(accumulators[us][hidden]) * network.outputWeights[hidden]
		output += floatScrelu(accumulators[them][hidden]) * network.outputWeights[nnue.HiddenSize+hidden]
	}
	output += network.outputBias
	return output * nnue.ScoreScale
}

func floatScrelu(value float32) float32 {
	if value <= 0 {
		return 0
	}
	if value >= 1 {
		return 1
	}
	return value * value
}

func referenceFromEngine(position *engine.Position) (referencePosition, nnue.Position) {
	all := position.Board.AllPieces()
	reference := referencePosition{pieces: make([]referencePiece, 0, len(all))}
	production := nnue.Position{Pieces: make([]nnue.PieceOnSquare, 0, len(all))}
	if position.Turn() == engine.Black {
		reference.sideToMove = uint8(nnue.Black)
		production.SideToMove = nnue.Black
	}
	for square := engine.Square(0); square < 64; square++ {
		chessPiece, ok := all[square]
		if !ok {
			continue
		}
		color := uint8(nnue.Black)
		productionColor := nnue.Black
		if chessPiece.Color() == engine.White {
			color = uint8(nnue.White)
			productionColor = nnue.White
		}
		pieceType := uint8(chessPiece.Type() - engine.Pawn)
		reference.pieces = append(reference.pieces, referencePiece{piece: pieceType, color: color, square: uint8(square)})
		production.Pieces = append(production.Pieces, nnue.PieceOnSquare{
			Piece: nnue.PieceType(pieceType), Color: productionColor, Square: nnue.Square(square),
		})
	}
	return reference, production
}
