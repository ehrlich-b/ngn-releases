package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/ehrlich-b/ngn/nnue"
)

func putI16(data []byte, offset int, value int16) {
	binary.LittleEndian.PutUint16(data[offset:], uint16(value))
}

func putF32(data []byte, offset int, value float32) {
	binary.LittleEndian.PutUint32(data[offset:], math.Float32bits(value))
}

func quantizedTransport(network *quantizedNetwork) []byte {
	data := make([]byte, bulletQuantizedSize)
	at := 0
	for feature := 0; feature < nnue.InputSize; feature++ {
		for hidden := 0; hidden < nnue.HiddenSize; hidden++ {
			putI16(data, at, network.featureWeights[feature][hidden])
			at += 2
		}
	}
	for _, value := range network.featureBias {
		putI16(data, at, value)
		at += 2
	}
	for _, value := range network.outputWeights {
		putI16(data, at, value)
		at += 2
	}
	putI16(data, at, network.outputBias)
	at += 2
	if at != nnue.PayloadSize {
		panic("bad test quantized layout")
	}
	for i := 0; i < bulletPaddingSize; i++ {
		data[at+i] = bulletPadWord[i%len(bulletPadWord)]
	}
	return data
}

func rawTransport(network *floatNetwork) []byte {
	data := make([]byte, bulletRawSize)
	at := 0
	for feature := 0; feature < nnue.InputSize; feature++ {
		for hidden := 0; hidden < nnue.HiddenSize; hidden++ {
			putF32(data, at, network.featureWeights[feature][hidden])
			at += 4
		}
	}
	for _, value := range network.featureBias {
		putF32(data, at, value)
		at += 4
	}
	for _, value := range network.outputWeights {
		putF32(data, at, value)
		at += 4
	}
	putF32(data, at, network.outputBias)
	at += 4
	if at != bulletRawSize {
		panic("bad test raw layout")
	}
	return data
}

func TestPinnedBulletTransportSizes(t *testing.T) {
	if bulletRawSize != 394756 {
		t.Fatalf("raw size = %d", bulletRawSize)
	}
	if bulletQuantizedSize != 197440 || bulletPaddingSize != 62 {
		t.Fatalf("quantized size/padding = %d/%d", bulletQuantizedSize, bulletPaddingSize)
	}
}

func TestReadBulletQuantizedLiteralLayoutAndPadding(t *testing.T) {
	data := make([]byte, bulletQuantizedSize)
	checks := []struct {
		offset int
		value  int16
	}{
		{0, 0x1234},
		{3442, -0x123},
		{196606, -2},
		{196608, 0x2345},
		{196754, -0x234},
		{196864, 0x3456},
		{197210, -0x345},
		{197374, -3},
		{197376, -4},
	}
	for _, check := range checks {
		putI16(data, check.offset, check.value)
	}
	for i := 0; i < bulletPaddingSize; i++ {
		data[nnue.PayloadSize+i] = bulletPadWord[i%len(bulletPadWord)]
	}
	network, err := readBulletQuantized(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	got := []int16{
		network.featureWeights[0][0], network.featureWeights[13][57], network.featureWeights[767][127],
		network.featureBias[0], network.featureBias[73], network.outputWeights[0], network.outputWeights[173],
		network.outputWeights[255], network.outputBias,
	}
	for i, check := range checks {
		if got[i] != check.value {
			t.Errorf("literal offset %d decoded %d, want %d", check.offset, got[i], check.value)
		}
	}
	encoded, _, _, _, err := convertBullet(bytes.NewReader(data), formatBulletQuantized)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(encoded[nnue.HeaderSize:], data[:nnue.PayloadSize]) {
		t.Fatal("conversion did not preserve the exact quantized tensor prefix and order")
	}
}

func TestReadBulletRawLiteralLayout(t *testing.T) {
	data := make([]byte, bulletRawSize)
	type check struct {
		offset int
		value  float32
	}
	checks := []check{
		{0, 1.25},
		{(13*128 + 57) * 4, -2.5},
		{(767*128 + 127) * 4, 3.75},
		{(98304 + 73) * 4, -4.25},
		{(98304 + 128 + 173) * 4, 5.5},
		{394752, -6.75},
	}
	for _, check := range checks {
		putF32(data, check.offset, check.value)
	}
	network, err := readBulletRaw(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	got := []float32{
		network.featureWeights[0][0], network.featureWeights[13][57], network.featureWeights[767][127],
		network.featureBias[73], network.outputWeights[173], network.outputBias,
	}
	for i, check := range checks {
		if got[i] != check.value {
			t.Errorf("literal offset %d decoded %g, want %g", check.offset, got[i], check.value)
		}
	}
}

func TestBulletReadersRejectMalformedTransport(t *testing.T) {
	quantized := quantizedTransport(new(quantizedNetwork))
	badPadding := bytes.Clone(quantized)
	badPadding[nnue.PayloadSize+17] ^= 1
	raw := rawTransport(new(floatNetwork))
	nonFinite := bytes.Clone(raw)
	putF32(nonFinite, (98304+9)*4, float32(math.Inf(1)))

	quantizedCases := map[string][]byte{
		"truncated": quantized[:len(quantized)-1],
		"extra":     append(bytes.Clone(quantized), 0),
		"padding":   badPadding,
	}
	for name, data := range quantizedCases {
		t.Run("quantized-"+name, func(t *testing.T) {
			if _, err := readBulletQuantized(bytes.NewReader(data)); !errors.Is(err, errBulletTransport) {
				t.Fatalf("error = %v", err)
			}
		})
	}
	rawCases := map[string][]byte{
		"truncated":  raw[:len(raw)-1],
		"extra":      append(bytes.Clone(raw), 0),
		"non-finite": nonFinite,
	}
	for name, data := range rawCases {
		t.Run("raw-"+name, func(t *testing.T) {
			if _, err := readBulletRaw(bytes.NewReader(data)); !errors.Is(err, errBulletTransport) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestIndependentQuantizationBoundaries(t *testing.T) {
	positive := float32(0.5039215683937073)
	negative := -positive
	for _, test := range []struct {
		value float32
		want  int16
	}{{positive, 128}, {negative, -128}} {
		got, err := quantizeValueReference(test.value, nnue.QA)
		if err != nil {
			t.Fatal(err)
		}
		if got != test.want {
			t.Fatalf("quantize(%0.20g) = %d, want %d", test.value, got, test.want)
		}
	}
	biasWitness := float32(0.007873774506151676)
	for _, test := range []struct {
		value float32
		want  int16
	}{{biasWitness, 128}, {-biasWitness, -128}} {
		got, err := quantizeValueReference(test.value, nnue.QA*nnue.QB)
		if err != nil || got != test.want {
			t.Fatalf("bias witness quantize(%0.20g) = %d, %v; want %d", test.value, got, err, test.want)
		}
	}
	for _, value := range []float32{float32(math.NaN()), float32(math.Inf(1)), 1000, -1000} {
		if _, err := quantizeValueReference(value, nnue.QA); !errors.Is(err, errBulletTransport) {
			t.Errorf("quantize(%g) error = %v", value, err)
		}
	}
}

func productionPositionFromReference(position referencePosition) nnue.Position {
	production := nnue.Position{SideToMove: nnue.Color(position.sideToMove), Pieces: make([]nnue.PieceOnSquare, 0, len(position.pieces))}
	for _, piece := range position.pieces {
		production.Pieces = append(production.Pieces, nnue.PieceOnSquare{
			Piece: nnue.PieceType(piece.piece), Color: nnue.Color(piece.color), Square: nnue.Square(piece.square),
		})
	}
	return production
}

func requireManualReferenceGoParity(t *testing.T, network *quantizedNetwork, position referencePosition, want int64) {
	t.Helper()
	reference := evaluateQuantizedReference(network, position)
	if reference != want {
		t.Fatalf("independent score = %d, want %d", reference, want)
	}
	_, model, _, _, err := convertBullet(bytes.NewReader(quantizedTransport(network)), formatBulletQuantized)
	if err != nil {
		t.Fatal(err)
	}
	production := productionPositionFromReference(position)
	scalar, err := model.Evaluate(production)
	if err != nil || scalar != reference {
		t.Fatalf("Go scalar = %d, %v; independent = %d", scalar, err, reference)
	}
	context, err := nnue.NewContext(model)
	if err != nil {
		t.Fatal(err)
	}
	if err := context.Reset(production); err != nil {
		t.Fatal(err)
	}
	contextScore, err := context.Evaluate()
	if err != nil || contextScore != reference {
		t.Fatalf("Go context = %d, %v; independent = %d", contextScore, err, reference)
	}
}

func TestManualAsymmetryClippingAndTruncation(t *testing.T) {
	pawnA2White := referencePosition{sideToMove: uint8(nnue.White), pieces: []referencePiece{{piece: uint8(nnue.Pawn), color: uint8(nnue.White), square: 8}}}
	pawnA2Black := pawnA2White
	pawnA2Black.sideToMove = uint8(nnue.Black)

	asymmetric := new(quantizedNetwork)
	asymmetric.featureWeights[8][0] = 255
	asymmetric.outputWeights[0] = 64
	asymmetric.outputWeights[128] = -64
	requireManualReferenceGoParity(t, asymmetric, pawnA2White, 400)
	requireManualReferenceGoParity(t, asymmetric, pawnA2Black, -400)

	firstDivision := new(quantizedNetwork)
	firstDivision.featureBias[0] = 1
	firstDivision.outputWeights[0] = -254
	firstDivision.outputBias = 16320
	requireManualReferenceGoParity(t, firstDivision, referencePosition{}, 400)
	secondDivision := new(quantizedNetwork)
	secondDivision.outputBias = -41
	requireManualReferenceGoParity(t, secondDivision, referencePosition{}, -1)
	clipped := new(quantizedNetwork)
	clipped.featureBias[0] = 300
	clipped.featureBias[1] = -5
	clipped.outputWeights[0] = 64
	clipped.outputWeights[1] = math.MaxInt16
	requireManualReferenceGoParity(t, clipped, referencePosition{}, 400)

	extreme := new(quantizedNetwork)
	for hidden := 0; hidden < nnue.HiddenSize; hidden++ {
		extreme.featureBias[hidden] = math.MaxInt16
	}
	for hidden := 0; hidden < nnue.PerspectiveCount*nnue.HiddenSize; hidden++ {
		extreme.outputWeights[hidden] = math.MaxInt16
	}
	extreme.outputBias = math.MaxInt16
	requireManualReferenceGoParity(t, extreme, referencePosition{}, 52428003)
}

func TestRawAndQuantizedConversionsAgreeWithIndependentReference(t *testing.T) {
	raw := new(floatNetwork)
	raw.featureWeights[8][0] = 1
	raw.outputWeights[0] = 1
	raw.outputWeights[128] = -1
	raw.outputBias = -41.0 / float32(nnue.QA*nnue.QB)
	rawBytes := rawTransport(raw)
	rawEncoded, rawModel, independent, _, err := convertBullet(bytes.NewReader(rawBytes), formatBulletRaw)
	if err != nil {
		t.Fatal(err)
	}
	quantizedBytes := quantizedTransport(independent)
	quantizedEncoded, quantizedModel, _, _, err := convertBullet(bytes.NewReader(quantizedBytes), formatBulletQuantized)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(rawEncoded, quantizedEncoded) {
		t.Fatal("raw and quantized conversion produced different NGN v1 files")
	}
	position := referencePosition{sideToMove: uint8(nnue.White), pieces: []referencePiece{{piece: 0, color: 0, square: 8}}}
	production := nnue.Position{SideToMove: nnue.White, Pieces: []nnue.PieceOnSquare{{Piece: nnue.Pawn, Color: nnue.White, Square: 8}}}
	want := evaluateQuantizedReference(independent, position)
	for name, model := range map[string]*nnue.Model{"raw": rawModel, "quantized": quantizedModel} {
		got, err := model.Evaluate(production)
		if err != nil || got != want {
			t.Fatalf("%s scalar = %d, %v; want %d", name, got, err, want)
		}
		context, err := nnue.NewContext(model)
		if err != nil {
			t.Fatal(err)
		}
		if err := context.Reset(production); err != nil {
			t.Fatal(err)
		}
		contextScore, err := context.Evaluate()
		if err != nil || contextScore != want {
			t.Fatalf("%s context = %d, %v; want %d", name, contextScore, err, want)
		}
	}
}

func TestRawConversionRejectsQuantizedRangeOverflow(t *testing.T) {
	raw := new(floatNetwork)
	raw.featureWeights[13][57] = 1000
	if _, _, _, _, err := convertBullet(bytes.NewReader(rawTransport(raw)), formatBulletRaw); !errors.Is(err, nnue.ErrQuantization) {
		t.Fatalf("error = %v, want ErrQuantization", err)
	}
}

func TestCLIConvertAndParityWithFENFile(t *testing.T) {
	directory := t.TempDir()
	network := new(quantizedNetwork)
	network.featureWeights[8][0] = 255
	network.outputWeights[0] = 64
	network.outputWeights[128] = -64
	input := filepath.Join(directory, "quantized.bin")
	output := filepath.Join(directory, "model.ngnnue")
	fenPath := filepath.Join(directory, "fixtures.fen")
	if err := os.WriteFile(input, quantizedTransport(network), 0o644); err != nil {
		t.Fatal(err)
	}
	fens := "# asymmetric turns\n4k3/8/8/8/8/8/P7/4K3 w - - 0 1\n\n4k3/8/8/8/8/8/P7/4K3 b - - 0 1\n"
	if err := os.WriteFile(fenPath, []byte(fens), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if err := run([]string{"convert", "-format", "bullet-quantized", "-in", input, "-out", output}, &stdout, &stderr); err != nil {
		t.Fatalf("convert: %v, stderr=%s", err, stderr.String())
	}
	encoded, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) != nnue.FileSize {
		t.Fatalf("output size = %d", len(encoded))
	}
	if _, err := nnue.Load(bytes.NewReader(encoded)); err != nil {
		t.Fatalf("load CLI output: %v", err)
	}
	stdout.Reset()
	stderr.Reset()
	if err := run([]string{"parity", "-format", "bullet-quantized", "-in", input, "-fens", fenPath}, &stdout, &stderr); err != nil {
		t.Fatalf("parity: %v, stderr=%s", err, stderr.String())
	}
	var report parityReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode parity JSON: %v\n%s", err, stdout.String())
	}
	if !report.Pass || report.MaxAbsoluteIntegerGoDelta != 0 || len(report.Records) != 2 {
		t.Fatalf("parity report = %+v", report)
	}
	if report.Records[0].ReferenceQuantized != 400 || report.Records[1].ReferenceQuantized != -400 {
		t.Fatalf("scores = %d, %d", report.Records[0].ReferenceQuantized, report.Records[1].ReferenceQuantized)
	}
}

func TestCLILeavesNoOutputForMalformedInput(t *testing.T) {
	directory := t.TempDir()
	input := filepath.Join(directory, "bad.bin")
	output := filepath.Join(directory, "must-not-exist.ngnnue")
	if err := os.WriteFile(input, make([]byte, bulletQuantizedSize-1), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"convert", "-format", "bullet-quantized", "-in", input, "-out", output}, &bytes.Buffer{}, &bytes.Buffer{}); err == nil {
		t.Fatal("malformed conversion succeeded")
	}
	if _, err := os.Stat(output); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("output exists or unexpected stat error: %v", err)
	}
}

func TestFloatReferenceAsymmetricAndNegative(t *testing.T) {
	network := new(floatNetwork)
	network.featureWeights[8][0] = 1
	network.outputWeights[0] = 1
	network.outputWeights[128] = -1
	white := referencePosition{sideToMove: uint8(nnue.White), pieces: []referencePiece{{piece: uint8(nnue.Pawn), color: uint8(nnue.White), square: 8}}}
	black := white
	black.sideToMove = uint8(nnue.Black)
	if got := evaluateFloatReference(network, white); got != 400 {
		t.Fatalf("white float score = %g", got)
	}
	if got := evaluateFloatReference(network, black); got != -400 {
		t.Fatalf("black float score = %g", got)
	}
	network = new(floatNetwork)
	network.outputBias = -0.25
	if got := evaluateFloatReference(network, referencePosition{}); got != -100 {
		t.Fatalf("negative float bias score = %g", got)
	}
}

func TestCLIParityRawReportsFloatDelta(t *testing.T) {
	directory := t.TempDir()
	network := new(floatNetwork)
	network.featureWeights[8][0] = 1
	network.outputWeights[0] = 1
	input := filepath.Join(directory, "raw.bin")
	if err := os.WriteFile(input, rawTransport(network), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	fen := "4k3/8/8/8/8/8/P7/4K3 w - - 0 1"
	if err := run([]string{"parity", "-format", "bullet-raw", "-in", input, "-fen", fen}, &stdout, &stderr); err != nil {
		t.Fatalf("raw parity: %v, stderr=%s", err, stderr.String())
	}
	var report parityReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if !report.Pass || len(report.Records) != 1 || report.MaxAbsoluteIntegerGoDelta != 0 {
		t.Fatalf("raw parity report = %+v", report)
	}
	record := report.Records[0]
	if record.ReferenceFloat == nil || *record.ReferenceFloat != 400 || record.FloatMinusInteger == nil || *record.FloatMinusInteger != 0 {
		t.Fatalf("raw float record = %+v", record)
	}
	if report.MaxAbsoluteFloatIntegerDelta == nil || *report.MaxAbsoluteFloatIntegerDelta != 0 {
		t.Fatalf("max float delta = %v", report.MaxAbsoluteFloatIntegerDelta)
	}
}
