package sf18big

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
	"os"
	"strings"
	"testing"
)

const (
	officialFileEnvironment        = "NGN_SF18_BIG_OFFICIAL_FILE"
	officialSpotsEnvironment       = "NGN_SF18_BIG_SPOTS_FILE"
	officialFileSize         int64 = 108919594
	officialFileSHA256             = "c288c895ea924429ea9092e3f36b2b3c1f00f2a3a4c759ff7e57e79e3b43e4a7"
)

func appendU32(dst []byte, value uint32) []byte {
	var encoded [4]byte
	binary.LittleEndian.PutUint32(encoded[:], value)
	return append(dst, encoded[:]...)
}

func TestRecognizesHeader(t *testing.T) {
	header := appendU32(nil, fileVersion)
	header = appendU32(header, networkHash)
	if !RecognizesHeader(header) {
		t.Fatal("exact SF18 BIG header was not recognized")
	}
	if RecognizesHeader(header[:7]) {
		t.Fatal("short header was recognized")
	}
	header[4] ^= 1
	if RecognizesHeader(header) {
		t.Fatal("wrong network hash was recognized")
	}
}

func appendI32(dst []byte, value int32) []byte {
	return appendU32(dst, uint32(value))
}

func appendSLEBBlock(dst []byte, values []int64) []byte {
	encoded := make([]byte, 0, len(values)*2)
	for _, value := range values {
		var token [5]byte
		length := encodeSLEB(value, token[:])
		encoded = append(encoded, token[:length]...)
	}
	dst = append(dst, leb128Magic...)
	dst = appendU32(dst, uint32(len(encoded)))
	return append(dst, encoded...)
}

func literalSLEBBlock(encoded ...byte) []byte {
	result := append([]byte(nil), leb128Magic...)
	result = appendU32(result, uint32(len(encoded)))
	return append(result, encoded...)
}

func decoder(data []byte) *fileDecoder {
	return &fileDecoder{reader: bufio.NewReader(bytes.NewReader(data))}
}

func TestStructuralConstants(t *testing.T) {
	if fileVersion != 0x7AF32F20 || transformerHash != 0x8F2344B8 ||
		architectureHash != 0x63336A4A || networkHash != 0xEC102EF2 {
		t.Fatal("Stockfish 18 BIG structural constants changed")
	}
	if baseInputFeatures != 22528 || threatInputFeatures != 79856 ||
		transformerLanes != 1024 || psqtBuckets != 8 || layerStacks != 8 {
		t.Fatal("Stockfish 18 BIG dimensions changed")
	}
	if maxFileBytes != 156266767 {
		t.Fatalf("maximum file bytes = %d", maxFileBytes)
	}
}

func TestReadCanonicalInt16BlockWithoutSmallNetScaling(t *testing.T) {
	want := []int16{-32768, -65, -64, -1, 0, 63, 64, 32767}
	// These canonical tokens are literal, so this decoder witness does not
	// share encodeSLEB with production.
	data := literalSLEBBlock(
		0x80, 0x80, 0x7e, // -32768
		0xbf, 0x7f, // -65
		0x40,       // -64
		0x7f,       // -1
		0x00,       // 0
		0x3f,       // 63
		0xc0, 0x00, // 64
		0xff, 0xff, 0x01, // 32767
	)
	var got [8]int16
	d := decoder(data)
	if err := d.readInt16Block("test", got[:]); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(int16Bytes(got[:]), int16Bytes(want)) {
		t.Fatalf("decoded int16 = %v, want %v", got, want)
	}
	if err := d.requireEOF(); err != nil {
		t.Fatal(err)
	}
}

func int16Bytes(values []int16) []byte {
	result := make([]byte, 2*len(values))
	for i, value := range values {
		binary.LittleEndian.PutUint16(result[2*i:], uint16(value))
	}
	return result
}

func TestReadSLEBRejectsMalformed(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want error
	}{
		{"wrong magic", append([]byte("XOMPRESSED_LEB128\x01\x00\x00\x00"), 0), ErrFormat},
		{"short declared length", append(append([]byte(leb128Magic), 0, 0, 0, 0), 0), ErrFormat},
		{"long declared length", append(append([]byte(leb128Magic), 4, 0, 0, 0), 0, 0, 0, 0), ErrFormat},
		{"noncanonical zero", append(append([]byte(leb128Magic), 2, 0, 0, 0), 0x80, 0x00), ErrFormat},
		{"unterminated int16", append(append([]byte(leb128Magic), 3, 0, 0, 0), 0x80, 0x80, 0x80), ErrFormat},
		{"int16 overflow", append(append([]byte(leb128Magic), 3, 0, 0, 0), 0x80, 0x80, 0x02), ErrRange},
		{"unused encoded byte", append(append([]byte(leb128Magic), 2, 0, 0, 0), 0x00, 0x00), ErrFormat},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var got [1]int16
			err := decoder(test.data).readInt16Block("test", got[:])
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want errors.Is(..., %v)", err, test.want)
			}
		})
	}
}

func TestReadCombinedPSQTThreatThenBase(t *testing.T) {
	data := literalSLEBBlock(
		0x80, 0x80, 0x80, 0x80, 0x78, // -2147483648
		0xff, 0xff, 0xff, 0xff, 0x07, // 2147483647
		0x0d, 0x72, 0x0f, // 13, -14, 15
	)
	first := make([]int32, 2)
	second := make([]int32, 3)
	d := decoder(data)
	if err := d.readSplitInt32Block("combined", first, second); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(first) != "[-2147483648 2147483647]" || fmt.Sprint(second) != "[13 -14 15]" {
		t.Fatalf("split order = %v / %v", first, second)
	}
}

func TestReadInt32BlockRejectsRangeOverflow(t *testing.T) {
	data := literalSLEBBlock(0x80, 0x80, 0x80, 0x80, 0x08)
	err := decoder(data).readSplitInt32Block("combined", make([]int32, 1), nil)
	if !errors.Is(err, ErrRange) {
		t.Fatalf("error = %v, want errors.Is(..., %v)", err, ErrRange)
	}
}

func TestReadRawInt8ChunkedAndTruncated(t *testing.T) {
	data := make([]byte, readChunkBytes+3)
	for i := range data {
		data[i] = byte(i*37 + 131)
	}
	got := make([]int8, len(data))
	if err := decoder(data).readInt8s("raw", got); err != nil {
		t.Fatal(err)
	}
	for _, index := range []int{0, 57, readChunkBytes - 1, readChunkBytes, len(data) - 1} {
		if got[index] != int8(data[index]) {
			t.Fatalf("raw[%d] = %d, want %d", index, got[index], int8(data[index]))
		}
	}
	if err := decoder(data[:len(data)-1]).readInt8s("raw", got); !errors.Is(err, ErrFormat) {
		t.Fatalf("truncated error = %v", err)
	}
}

func layerStackFixture() ([]byte, layerStack) {
	var want layerStack
	data := appendU32(nil, architectureHash)
	for i := range want.fc0Bias {
		want.fc0Bias[i] = int32(i*100003 - 700009)
		data = appendI32(data, want.fc0Bias[i])
	}
	for i := range want.fc0Weight {
		want.fc0Weight[i] = int8((i*29+17)%255 - 127)
		data = append(data, byte(want.fc0Weight[i]))
	}
	for i := range want.fc1Bias {
		want.fc1Bias[i] = int32(i*70001 - 900019)
		data = appendI32(data, want.fc1Bias[i])
	}
	for i := range want.fc1Weight {
		want.fc1Weight[i] = int8((i*43+3)%255 - 127)
		data = append(data, byte(want.fc1Weight[i]))
	}
	want.fc2Bias[0] = -123456789
	data = appendI32(data, want.fc2Bias[0])
	for i := range want.fc2Weight {
		want.fc2Weight[i] = int8(i*7 - 103)
		data = append(data, byte(want.fc2Weight[i]))
	}
	return data, want
}

func TestReadLayerStackExactInteriorOrder(t *testing.T) {
	data, want := layerStackFixture()
	var got layerStack
	d := decoder(data)
	if err := d.readLayerStack(5, &got); err != nil {
		t.Fatal(err)
	}
	if err := d.requireEOF(); err != nil {
		t.Fatal(err)
	}
	checks := []struct {
		label string
		got   int64
		want  int64
	}{
		{"fc0 bias 9", int64(got.fc0Bias[9]), int64(want.fc0Bias[9])},
		{"fc0 weight 13,57", int64(got.fc0Weight[13*transformerLanes+57]), int64(want.fc0Weight[13*transformerLanes+57])},
		{"fc1 bias 23", int64(got.fc1Bias[23]), int64(want.fc1Bias[23])},
		{"fc1 weight 17,9", int64(got.fc1Weight[17*32+9]), int64(want.fc1Weight[17*32+9])},
		{"fc2 bias", int64(got.fc2Bias[0]), int64(want.fc2Bias[0])},
		{"fc2 weight 19", int64(got.fc2Weight[19]), int64(want.fc2Weight[19])},
	}
	for _, check := range checks {
		if check.got != check.want {
			t.Errorf("%s = %d, want %d", check.label, check.got, check.want)
		}
	}
}

func TestReadLayerStackRejectsHashTruncationAndNil(t *testing.T) {
	data, _ := layerStackFixture()
	badHash := append([]byte(nil), data...)
	binary.LittleEndian.PutUint32(badHash, architectureHash+1)
	var stack layerStack
	if err := decoder(badHash).readLayerStack(0, &stack); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("hash error = %v", err)
	}
	if err := decoder(data[:len(data)-1]).readLayerStack(7, &stack); !errors.Is(err, ErrFormat) {
		t.Fatalf("truncation error = %v", err)
	}
	if err := decoder(data).readLayerStack(1, nil); !errors.Is(err, ErrFormat) {
		t.Fatalf("nil error = %v", err)
	}
}

func TestLoadRejectsEarlyMalformedWithoutPublishing(t *testing.T) {
	header := appendU32(nil, fileVersion)
	header = appendU32(header, networkHash)
	header = appendU32(header, 0)
	header = appendU32(header, transformerHash)
	tests := []struct {
		name string
		data []byte
		want error
	}{
		{"empty", nil, ErrFormat},
		{"short header", header[:11], ErrFormat},
		{"unsupported version", appendU32(appendU32(nil, fileVersion+1), networkHash), ErrUnsupported},
		{"unsupported network", appendU32(appendU32(nil, fileVersion), networkHash+1), ErrUnsupported},
		{"description cap", appendU32(appendU32(appendU32(nil, fileVersion), networkHash), maxDescriptionBytes+1), ErrFormat},
		{"short description", append(appendU32(appendU32(appendU32(nil, fileVersion), networkHash), 2), 'x'), ErrFormat},
		{"transformer hash", appendU32(header[:12], transformerHash+1), ErrIntegrity},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			model, err := Load(bytes.NewReader(test.data))
			if model != nil {
				t.Fatal("failed load published a model")
			}
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want errors.Is(..., %v)", err, test.want)
			}
		})
	}
	model, err := Load(nil)
	if model != nil || !errors.Is(err, ErrFormat) {
		t.Fatalf("Load(nil) = %v, %v", model, err)
	}
}

type stuckReader struct{}

func (stuckReader) Read([]byte) (int, error) { return 0, nil }

type oneEmptyRead struct {
	reader io.Reader
	empty  bool
}

func (r *oneEmptyRead) Read(dst []byte) (int, error) {
	if !r.empty {
		r.empty = true
		return 0, nil
	}
	return r.reader.Read(dst)
}

func TestReaderProgressAndLegalEmptyRead(t *testing.T) {
	model, err := Load(stuckReader{})
	if model != nil || !errors.Is(err, ErrFormat) || !errors.Is(err, io.ErrNoProgress) {
		t.Fatalf("Load(stuck) = %v, %v", model, err)
	}
	data := appendSLEBBlock(nil, []int64{17})
	var got [1]int16
	d := &fileDecoder{reader: bufio.NewReader(&noProgressReader{reader: &oneEmptyRead{reader: bytes.NewReader(data)}})}
	if err := d.readInt16Block("one-empty", got[:]); err != nil || got[0] != 17 {
		t.Fatalf("legal empty read = %d, %v", got[0], err)
	}
}

func TestRequireEOFRejectsTrailingAndNoProgress(t *testing.T) {
	if err := decoder(nil).requireEOF(); err != nil {
		t.Fatalf("empty EOF probe: %v", err)
	}
	if err := decoder([]byte{0x42}).requireEOF(); !errors.Is(err, ErrFormat) {
		t.Fatalf("trailing byte error = %v", err)
	}
	stuck := &fileDecoder{reader: bufio.NewReader(&noProgressReader{reader: stuckReader{}})}
	if err := stuck.requireEOF(); !errors.Is(err, ErrFormat) || !errors.Is(err, io.ErrNoProgress) {
		t.Fatalf("stuck EOF probe error = %v", err)
	}
}

func TestMetadataNilIsZero(t *testing.T) {
	var model *Model
	if model.Metadata() != (Metadata{}) {
		t.Fatal("nil model metadata is nonzero")
	}
}

const officialSourceCommit = "cb3d4ee9b47d0c5aae855b12379378ea1439675c"

type officialSpotFile struct {
	Schema        *string              `json:"schema"`
	SourceCommit  *string              `json:"source_commit"`
	NetworkSHA256 *string              `json:"network_sha256"`
	NetworkSize   *int64               `json:"network_size"`
	Header        *officialSpotHeader  `json:"header"`
	Bias          []tensorSpot16       `json:"bias"`
	Threat        []tensorSpot8        `json:"threat_weights"`
	Base          []tensorSpot16       `json:"base_weights"`
	ThreatPSQT    []tensorSpot32       `json:"threat_psqt"`
	BasePSQT      []tensorSpot32       `json:"base_psqt"`
	Stacks        []officialStackSpot  `json:"stacks"`
	Grammar       *officialSpotGrammar `json:"grammar"`
}

type officialSpotHeader struct {
	Version           *string `json:"version"`
	NetworkHash       *string `json:"network_hash"`
	TransformerHash   *string `json:"transformer_hash"`
	ArchitectureHash  *string `json:"architecture_hash"`
	DescriptionOffset *int64  `json:"description_offset"`
	DescriptionLength *int    `json:"description_length"`
	DescriptionUTF8   *string `json:"description_utf8"`
}

type tensorSpot8 struct {
	Index *int  `json:"index"`
	Value *int8 `json:"value"`
}
type tensorSpot16 struct {
	Index *int   `json:"index"`
	Value *int16 `json:"value"`
}
type tensorSpot32 struct {
	Index *int   `json:"index"`
	Value *int32 `json:"value"`
}
type officialStackSpot struct {
	Index     *int          `json:"index"`
	FC0Bias   *tensorSpot32 `json:"fc0_bias"`
	FC0Weight *tensorSpot8  `json:"fc0_weight"`
	FC1Bias   *tensorSpot32 `json:"fc1_bias"`
	FC1Weight *tensorSpot8  `json:"fc1_weight"`
	FC2Bias   *tensorSpot32 `json:"fc2_bias"`
	FC2Weight *tensorSpot8  `json:"fc2_weight"`
}

type officialSLEBGrammar struct {
	DataOffset   *int64 `json:"data_offset"`
	EncodedBytes *int64 `json:"encoded_bytes"`
	ValueCount   *int64 `json:"value_count"`
	Minimum      *int64 `json:"minimum"`
	Maximum      *int64 `json:"maximum"`
}

type officialRawGrammar struct {
	Offset *int64 `json:"offset"`
	Bytes  *int64 `json:"bytes"`
}

type officialStackSectionGrammar struct {
	Name    *string `json:"name"`
	Offset  *int64  `json:"offset"`
	Count   *int64  `json:"count"`
	Minimum *int64  `json:"minimum"`
	Maximum *int64  `json:"maximum"`
}

type officialStackGrammar struct {
	Index    *int                          `json:"index"`
	Offset   *int64                        `json:"offset"`
	Bytes    *int64                        `json:"bytes"`
	Sections []officialStackSectionGrammar `json:"sections"`
}

type officialSpotGrammar struct {
	Bias          *officialSLEBGrammar   `json:"bias"`
	ThreatWeights *officialRawGrammar    `json:"threat_weights"`
	BaseWeights   *officialSLEBGrammar   `json:"base_weights"`
	CombinedPSQT  *officialSLEBGrammar   `json:"combined_psqt"`
	Stacks        []officialStackGrammar `json:"stacks"`
	ExactEOF      *bool                  `json:"exact_eof"`
	CanonicalSLEB *bool                  `json:"canonical_sleb"`
}

func decodeOfficialSpots(encoded []byte) (officialSpotFile, error) {
	var spots officialSpotFile
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&spots); err != nil {
		return officialSpotFile{}, fmt.Errorf("decode official spots: %w", err)
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return officialSpotFile{}, fmt.Errorf("official spots trailing JSON: %w", err)
	}
	if err := validateOfficialSpots(&spots); err != nil {
		return officialSpotFile{}, err
	}
	return spots, nil
}

func validateOfficialSpots(spots *officialSpotFile) error {
	if spots == nil || spots.Schema == nil || *spots.Schema != "sf18-big-official-spots/v1" {
		return fmt.Errorf("official spots schema")
	}
	if spots.SourceCommit == nil || *spots.SourceCommit != officialSourceCommit {
		return fmt.Errorf("official spots source commit")
	}
	if spots.NetworkSHA256 == nil || *spots.NetworkSHA256 != officialFileSHA256 ||
		spots.NetworkSize == nil || *spots.NetworkSize != officialFileSize {
		return fmt.Errorf("official spots network identity")
	}
	if spots.Header == nil || spots.Header.Version == nil || *spots.Header.Version != "0x7AF32F20" ||
		spots.Header.NetworkHash == nil || *spots.Header.NetworkHash != "0xEC102EF2" ||
		spots.Header.TransformerHash == nil || *spots.Header.TransformerHash != "0x8F2344B8" ||
		spots.Header.ArchitectureHash == nil || *spots.Header.ArchitectureHash != "0x63336A4A" ||
		spots.Header.DescriptionOffset == nil || *spots.Header.DescriptionOffset != 12 ||
		spots.Header.DescriptionLength == nil || *spots.Header.DescriptionLength != 84 ||
		spots.Header.DescriptionUTF8 == nil || len([]byte(*spots.Header.DescriptionUTF8)) != 84 {
		return fmt.Errorf("official spots header")
	}
	if err := validateSpot16Indices("bias", spots.Bias, []int{0, 513, transformerLanes - 1}); err != nil {
		return err
	}
	if err := validateSpot8Indices("threat weights", spots.Threat, []int{0, 39856*transformerLanes + 511, threatInputFeatures*transformerLanes - 1}); err != nil {
		return err
	}
	if err := validateSpot16Indices("base weights", spots.Base, []int{0, 13*transformerLanes + 57, baseInputFeatures*transformerLanes - 1}); err != nil {
		return err
	}
	if err := validateSpot32Indices("threat PSQT", spots.ThreatPSQT, []int{0, 12345, threatInputFeatures*psqtBuckets - 1}); err != nil {
		return err
	}
	if err := validateSpot32Indices("base PSQT", spots.BasePSQT, []int{0, 13*psqtBuckets + 3, baseInputFeatures*psqtBuckets - 1}); err != nil {
		return err
	}
	if len(spots.Stacks) != layerStacks {
		return fmt.Errorf("official stack spots = %d, want %d", len(spots.Stacks), layerStacks)
	}
	for stackIndex, spot := range spots.Stacks {
		if spot.Index == nil || *spot.Index != stackIndex || spot.FC0Bias == nil || spot.FC0Weight == nil ||
			spot.FC1Bias == nil || spot.FC1Weight == nil || spot.FC2Bias == nil || spot.FC2Weight == nil {
			return fmt.Errorf("official stack spot %d shape", stackIndex)
		}
		expected := []int{
			(stackIndex*5 + 3) % 16,
			(3442 + stackIndex*1777) % (16 * transformerLanes),
			(stackIndex*7 + 11) % 32,
			(553 + stackIndex*101) % (32 * 32),
			0,
			(stackIndex*3 + 19) % 32,
		}
		if err := validateSpot32Pointer(fmt.Sprintf("stack %d FC0 bias", stackIndex), spot.FC0Bias, expected[0]); err != nil {
			return err
		}
		if err := validateSpot8Pointer(fmt.Sprintf("stack %d FC0 weight", stackIndex), spot.FC0Weight, expected[1]); err != nil {
			return err
		}
		if err := validateSpot32Pointer(fmt.Sprintf("stack %d FC1 bias", stackIndex), spot.FC1Bias, expected[2]); err != nil {
			return err
		}
		if err := validateSpot8Pointer(fmt.Sprintf("stack %d FC1 weight", stackIndex), spot.FC1Weight, expected[3]); err != nil {
			return err
		}
		if err := validateSpot32Pointer(fmt.Sprintf("stack %d FC2 bias", stackIndex), spot.FC2Bias, expected[4]); err != nil {
			return err
		}
		if err := validateSpot8Pointer(fmt.Sprintf("stack %d FC2 weight", stackIndex), spot.FC2Weight, expected[5]); err != nil {
			return err
		}
	}
	return validateOfficialGrammar(spots.Grammar)
}

func validateSpot8Indices(label string, spots []tensorSpot8, expected []int) error {
	if len(spots) != len(expected) {
		return fmt.Errorf("%s spots = %d, want %d", label, len(spots), len(expected))
	}
	for i := range spots {
		if err := validateSpot8Pointer(label, &spots[i], expected[i]); err != nil {
			return err
		}
	}
	return nil
}
func validateSpot16Indices(label string, spots []tensorSpot16, expected []int) error {
	if len(spots) != len(expected) {
		return fmt.Errorf("%s spots = %d, want %d", label, len(spots), len(expected))
	}
	for i := range spots {
		if spots[i].Index == nil || spots[i].Value == nil || *spots[i].Index != expected[i] {
			return fmt.Errorf("%s spot %d shape/index", label, i)
		}
	}
	return nil
}
func validateSpot32Indices(label string, spots []tensorSpot32, expected []int) error {
	if len(spots) != len(expected) {
		return fmt.Errorf("%s spots = %d, want %d", label, len(spots), len(expected))
	}
	for i := range spots {
		if err := validateSpot32Pointer(label, &spots[i], expected[i]); err != nil {
			return err
		}
	}
	return nil
}
func validateSpot8Pointer(label string, spot *tensorSpot8, expected int) error {
	if spot == nil || spot.Index == nil || spot.Value == nil || *spot.Index != expected {
		return fmt.Errorf("%s spot shape/index, want %d", label, expected)
	}
	return nil
}
func validateSpot32Pointer(label string, spot *tensorSpot32, expected int) error {
	if spot == nil || spot.Index == nil || spot.Value == nil || *spot.Index != expected {
		return fmt.Errorf("%s spot shape/index, want %d", label, expected)
	}
	return nil
}

func validateOfficialGrammar(grammar *officialSpotGrammar) error {
	if grammar == nil || grammar.ExactEOF == nil || !*grammar.ExactEOF ||
		grammar.CanonicalSLEB == nil || !*grammar.CanonicalSLEB || grammar.ThreatWeights == nil ||
		grammar.ThreatWeights.Offset == nil || grammar.ThreatWeights.Bytes == nil {
		return fmt.Errorf("official grammar shape")
	}
	biasEnd, err := validateSLEBGrammar("bias", grammar.Bias, 121, transformerLanes, 16)
	if err != nil {
		return err
	}
	if *grammar.ThreatWeights.Offset != biasEnd || *grammar.ThreatWeights.Bytes != int64(threatInputFeatures*transformerLanes) {
		return fmt.Errorf("official threat grammar")
	}
	baseDataOffset := *grammar.ThreatWeights.Offset + *grammar.ThreatWeights.Bytes + int64(len(leb128Magic)+4)
	baseEnd, err := validateSLEBGrammar("base weights", grammar.BaseWeights, baseDataOffset, baseInputFeatures*transformerLanes, 16)
	if err != nil {
		return err
	}
	psqtDataOffset := baseEnd + int64(len(leb128Magic)+4)
	psqtEnd, err := validateSLEBGrammar("combined PSQT", grammar.CombinedPSQT, psqtDataOffset,
		(threatInputFeatures+baseInputFeatures)*psqtBuckets, 32)
	if err != nil {
		return err
	}
	if len(grammar.Stacks) != layerStacks {
		return fmt.Errorf("official grammar stacks = %d", len(grammar.Stacks))
	}
	sections := []struct {
		name         string
		count, width int64
	}{
		{"fc0_bias", 16, 4}, {"fc0_weight", 16 * transformerLanes, 1},
		{"fc1_bias", 32, 4}, {"fc1_weight", 32 * 32, 1},
		{"fc2_bias", 1, 4}, {"fc2_weight", 32, 1},
	}
	offset := psqtEnd
	for stackIndex, stack := range grammar.Stacks {
		if stack.Index == nil || *stack.Index != stackIndex || stack.Offset == nil || *stack.Offset != offset ||
			stack.Bytes == nil || *stack.Bytes != 17640 || len(stack.Sections) != len(sections) {
			return fmt.Errorf("official grammar stack %d shape", stackIndex)
		}
		sectionOffset := offset + 4
		for sectionIndex, expected := range sections {
			section := stack.Sections[sectionIndex]
			if section.Name == nil || *section.Name != expected.name || section.Offset == nil || *section.Offset != sectionOffset ||
				section.Count == nil || *section.Count != expected.count || section.Minimum == nil || section.Maximum == nil ||
				*section.Minimum > *section.Maximum {
				return fmt.Errorf("official grammar stack %d section %d", stackIndex, sectionIndex)
			}
			sectionOffset += expected.count * expected.width
		}
		if sectionOffset != offset+*stack.Bytes {
			return fmt.Errorf("official grammar stack %d byte span", stackIndex)
		}
		offset += *stack.Bytes
	}
	if offset != officialFileSize {
		return fmt.Errorf("official grammar final offset = %d", offset)
	}
	return nil
}

func validateSLEBGrammar(label string, grammar *officialSLEBGrammar, expectedOffset int64, count int, bits int) (int64, error) {
	if grammar == nil || grammar.DataOffset == nil || *grammar.DataOffset != expectedOffset ||
		grammar.EncodedBytes == nil || grammar.ValueCount == nil || *grammar.ValueCount != int64(count) ||
		grammar.Minimum == nil || grammar.Maximum == nil || *grammar.Minimum > *grammar.Maximum {
		return 0, fmt.Errorf("official %s grammar shape", label)
	}
	maxToken := int64(3)
	if bits == 32 {
		maxToken = 5
	}
	if *grammar.EncodedBytes < int64(count) || *grammar.EncodedBytes > int64(count)*maxToken {
		return 0, fmt.Errorf("official %s encoded bytes", label)
	}
	minimum := -(int64(1) << (bits - 1))
	maximum := (int64(1) << (bits - 1)) - 1
	if *grammar.Minimum < minimum || *grammar.Maximum > maximum {
		return 0, fmt.Errorf("official %s value range", label)
	}
	return *grammar.DataOffset + *grammar.EncodedBytes, nil
}

func TestOfficialSpotsRejectMalformedEnvelope(t *testing.T) {
	mutations := []struct {
		name string
		edit func(map[string]any)
	}{
		{"unknown field", func(document map[string]any) { document["unexpected"] = true }},
		{"wrong schema", func(document map[string]any) { document["schema"] = "wrong" }},
		{"wrong source", func(document map[string]any) { document["source_commit"] = "wrong" }},
		{"duplicate index", func(document map[string]any) { document["bias"].([]any)[1].(map[string]any)["index"] = 0 }},
		{"missing index", func(document map[string]any) { document["bias"] = document["bias"].([]any)[:2] }},
		{"changed index", func(document map[string]any) { document["bias"].([]any)[1].(map[string]any)["index"] = 514 }},
		{"zero value null", func(document map[string]any) { document["bias"].([]any)[0].(map[string]any)["value"] = nil }},
		{"stack index changed", func(document map[string]any) { document["stacks"].([]any)[4].(map[string]any)["index"] = 3 }},
		{"grammar scalar null", func(document map[string]any) { document["grammar"].(map[string]any)["exact_eof"] = nil }},
	}
	valid, err := json.Marshal(validOfficialSpotDocument())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeOfficialSpots(valid); err != nil {
		t.Fatalf("valid envelope: %v", err)
	}
	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			document := validOfficialSpotDocument()
			mutation.edit(document)
			encoded, err := json.Marshal(document)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := decodeOfficialSpots(encoded); err == nil {
				t.Fatal("malformed official spots accepted")
			}
		})
	}
}

func validOfficialSpotDocument() map[string]any {
	spot := func(index int) map[string]any { return map[string]any{"index": index, "value": 0} }
	biasEncoded := int64(transformerLanes)
	biasOffset := int64(121)
	threatOffset := biasOffset + biasEncoded
	baseOffset := threatOffset + int64(threatInputFeatures*transformerLanes) + int64(len(leb128Magic)+4)
	baseEncoded := int64(baseInputFeatures * transformerLanes)
	psqtOffset := baseOffset + baseEncoded + int64(len(leb128Magic)+4)
	psqtEncoded := officialFileSize - psqtOffset - 17640*layerStacks
	sleb := func(offset, encoded, count int64) map[string]any {
		return map[string]any{"data_offset": offset, "encoded_bytes": encoded, "value_count": count, "minimum": -1, "maximum": 1}
	}
	stackSpots := make([]any, layerStacks)
	stackGrammar := make([]any, layerStacks)
	stackOffset := psqtOffset + psqtEncoded
	sections := []struct {
		name         string
		count, width int64
	}{
		{"fc0_bias", 16, 4}, {"fc0_weight", 16 * transformerLanes, 1},
		{"fc1_bias", 32, 4}, {"fc1_weight", 32 * 32, 1}, {"fc2_bias", 1, 4}, {"fc2_weight", 32, 1},
	}
	for i := 0; i < layerStacks; i++ {
		stackSpots[i] = map[string]any{
			"index": i, "fc0_bias": spot((i*5 + 3) % 16), "fc0_weight": spot((3442 + i*1777) % (16 * transformerLanes)),
			"fc1_bias": spot((i*7 + 11) % 32), "fc1_weight": spot((553 + i*101) % (32 * 32)),
			"fc2_bias": spot(0), "fc2_weight": spot((i*3 + 19) % 32),
		}
		sectionOffset := stackOffset + 4
		encodedSections := make([]any, len(sections))
		for j, section := range sections {
			encodedSections[j] = map[string]any{"name": section.name, "offset": sectionOffset, "count": section.count, "minimum": -1, "maximum": 1}
			sectionOffset += section.count * section.width
		}
		stackGrammar[i] = map[string]any{"index": i, "offset": stackOffset, "bytes": int64(17640), "sections": encodedSections}
		stackOffset += 17640
	}
	return map[string]any{
		"schema": "sf18-big-official-spots/v1", "source_commit": officialSourceCommit,
		"network_sha256": officialFileSHA256, "network_size": officialFileSize,
		"header":         map[string]any{"version": "0x7AF32F20", "network_hash": "0xEC102EF2", "transformer_hash": "0x8F2344B8", "architecture_hash": "0x63336A4A", "description_offset": 12, "description_length": 84, "description_utf8": strings.Repeat("x", 84)},
		"bias":           []any{spot(0), spot(513), spot(transformerLanes - 1)},
		"threat_weights": []any{spot(0), spot(39856*transformerLanes + 511), spot(threatInputFeatures*transformerLanes - 1)},
		"base_weights":   []any{spot(0), spot(13*transformerLanes + 57), spot(baseInputFeatures*transformerLanes - 1)},
		"threat_psqt":    []any{spot(0), spot(12345), spot(threatInputFeatures*psqtBuckets - 1)},
		"base_psqt":      []any{spot(0), spot(13*psqtBuckets + 3), spot(baseInputFeatures*psqtBuckets - 1)},
		"stacks":         stackSpots,
		"grammar": map[string]any{
			"bias":           sleb(biasOffset, biasEncoded, transformerLanes),
			"threat_weights": map[string]any{"offset": threatOffset, "bytes": threatInputFeatures * transformerLanes},
			"base_weights":   sleb(baseOffset, baseEncoded, baseInputFeatures*transformerLanes),
			"combined_psqt":  sleb(psqtOffset, psqtEncoded, (threatInputFeatures+baseInputFeatures)*psqtBuckets),
			"stacks":         stackGrammar, "exact_eof": true, "canonical_sleb": true,
		},
	}
}

func TestLoadOfficialModelAndIndependentSpots(t *testing.T) {
	path := os.Getenv(officialFileEnvironment)
	spotsPath := os.Getenv(officialSpotsEnvironment)
	if path == "" && spotsPath == "" {
		t.Skip("set both official BIG fixture environment variables")
	}
	if path == "" || spotsPath == "" {
		t.Fatalf("%s and %s must be set together", officialFileEnvironment, officialSpotsEnvironment)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	model, err := Load(file)
	if err != nil {
		t.Fatal(err)
	}
	encodedSpots, err := os.ReadFile(spotsPath)
	if err != nil {
		t.Fatal(err)
	}
	spots, err := decodeOfficialSpots(encodedSpots)
	if err != nil {
		t.Fatal(err)
	}
	metadata := model.Metadata()
	wantDigest, err := parseSHA256(officialFileSHA256)
	if err != nil {
		t.Fatal(err)
	}
	if metadata.FileSize != officialFileSize || metadata.FileSHA256 != wantDigest {
		t.Fatalf("official model identity mismatch: metadata=%+v", metadata)
	}
	if metadata.Version != fileVersion || metadata.NetworkHash != networkHash || metadata.TransformerHash != transformerHash || metadata.ArchitectureHash != architectureHash {
		t.Fatalf("official structural identity = %+v", metadata)
	}
	if metadata.ArchitectureID != architectureID || metadata.FeatureSetID != featureSetID || metadata.QuantizationID != quantizationID || metadata.ScoreContractID != scoreContractID {
		t.Fatalf("official semantic identity = %+v", metadata)
	}
	if metadata.Description != *spots.Header.DescriptionUTF8 {
		t.Fatal("official description differs from independent grammar")
	}
	if len(model.baseWeights) != baseInputFeatures*transformerLanes || len(model.threatWeights) != threatInputFeatures*transformerLanes || len(model.basePSQTWeights) != baseInputFeatures*psqtBuckets || len(model.threatPSQTWeights) != threatInputFeatures*psqtBuckets {
		t.Fatal("official tensor dimensions mismatch")
	}
	requireSpots16(t, "bias", model.featureBias[:], spots.Bias)
	requireSpots8(t, "threat", model.threatWeights, spots.Threat)
	requireSpots16(t, "base", model.baseWeights, spots.Base)
	requireSpots32(t, "threat PSQT", model.threatPSQTWeights, spots.ThreatPSQT)
	requireSpots32(t, "base PSQT", model.basePSQTWeights, spots.BasePSQT)
	for _, spot := range spots.Stacks {
		stack := &model.stacks[*spot.Index]
		requireSpot32(t, fmt.Sprintf("stack %d FC0 bias", *spot.Index), stack.fc0Bias[:], spot.FC0Bias)
		requireSpot8(t, fmt.Sprintf("stack %d FC0 weight", *spot.Index), stack.fc0Weight[:], spot.FC0Weight)
		requireSpot32(t, fmt.Sprintf("stack %d FC1 bias", *spot.Index), stack.fc1Bias[:], spot.FC1Bias)
		requireSpot8(t, fmt.Sprintf("stack %d FC1 weight", *spot.Index), stack.fc1Weight[:], spot.FC1Weight)
		requireSpot32(t, fmt.Sprintf("stack %d FC2 bias", *spot.Index), stack.fc2Bias[:], spot.FC2Bias)
		requireSpot8(t, fmt.Sprintf("stack %d FC2 weight", *spot.Index), stack.fc2Weight[:], spot.FC2Weight)
	}
}

func parseSHA256(text string) ([sha256.Size]byte, error) {
	var result [sha256.Size]byte
	decoded, err := hex.DecodeString(text)
	if err != nil {
		return result, fmt.Errorf("invalid SHA-256 %q: %w", text, err)
	}
	if len(decoded) != len(result) {
		return result, fmt.Errorf("invalid SHA-256 %q: decoded length %d", text, len(decoded))
	}
	copy(result[:], decoded)
	return result, nil
}
func requireSpots8(t *testing.T, label string, values []int8, spots []tensorSpot8) {
	t.Helper()
	for i := range spots {
		requireSpot8(t, label, values, &spots[i])
	}
}
func requireSpots16(t *testing.T, label string, values []int16, spots []tensorSpot16) {
	t.Helper()
	for _, spot := range spots {
		index, want := *spot.Index, *spot.Value
		if values[index] != want {
			t.Fatalf("%s[%d]=%d, want %d", label, index, values[index], want)
		}
	}
}
func requireSpots32(t *testing.T, label string, values []int32, spots []tensorSpot32) {
	t.Helper()
	for i := range spots {
		requireSpot32(t, label, values, &spots[i])
	}
}
func requireSpot8(t *testing.T, label string, values []int8, spot *tensorSpot8) {
	t.Helper()
	index, want := *spot.Index, *spot.Value
	if values[index] != want {
		t.Fatalf("%s[%d]=%d, want %d", label, index, values[index], want)
	}
}
func requireSpot32(t *testing.T, label string, values []int32, spot *tensorSpot32) {
	t.Helper()
	index, want := *spot.Index, *spot.Value
	if values[index] != want {
		t.Fatalf("%s[%d]=%d, want %d", label, index, values[index], want)
	}
}
