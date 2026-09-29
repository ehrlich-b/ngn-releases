package sf18small

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"testing"
)

const officialFileEnvironment = "NGN_SF18_SMALL_OFFICIAL_FILE"

type modelLayout struct {
	transformerHash int
	biasMagic       int
	biasLength      int
	biasData        int
	weightMagic     int
	psqtMagic       int
	stacks          [layerStacks]int
}

type zeroModelFixture struct {
	data   []byte
	layout modelLayout
	desc   string
}

var zeroFixture = sync.OnceValue(func() zeroModelFixture {
	description := []byte{'s', 'y', 'n', 't', 'h', 'e', 't', 'i', 'c', 0, 0xff}
	data := make([]byte, 0, 3090447+len(description))
	data = appendU32(data, fileVersion)
	data = appendU32(data, networkHash)
	data = appendU32(data, uint32(len(description)))
	data = append(data, description...)
	layout := modelLayout{transformerHash: len(data)}
	data = appendU32(data, transformerHash)
	layout.biasMagic, layout.biasLength, layout.biasData, data = appendZeroBlock(data, transformerLanes)
	layout.weightMagic, _, _, data = appendZeroBlock(data, inputFeatures*transformerLanes)
	layout.psqtMagic, _, _, data = appendZeroBlock(data, inputFeatures*psqtBuckets)
	for stack := range layout.stacks {
		layout.stacks[stack] = len(data)
		data = appendU32(data, architectureHash)
		data = append(data, make([]byte, 16*4+16*transformerLanes+32*4+32*32+4+32)...)
	}
	if got, want := len(data), 3090447+len(description); got != want {
		panic("synthetic model size mismatch")
	}
	return zeroModelFixture{data: data, layout: layout, desc: string(description)}
})

func appendU32(dst []byte, value uint32) []byte {
	var encoded [4]byte
	binary.LittleEndian.PutUint32(encoded[:], value)
	return append(dst, encoded[:]...)
}

func appendZeroBlock(dst []byte, count int) (magic, length, data int, result []byte) {
	magic = len(dst)
	dst = append(dst, leb128Magic...)
	length = len(dst)
	dst = appendU32(dst, uint32(count))
	data = len(dst)
	dst = append(dst, make([]byte, count)...)
	return magic, length, data, dst
}

func cloneBytes(src []byte) []byte {
	return append([]byte(nil), src...)
}

func replaceFirstBiasToken(fixture zeroModelFixture, token []byte) []byte {
	delta := len(token) - 1
	result := make([]byte, 0, len(fixture.data)+delta)
	result = append(result, fixture.data[:fixture.layout.biasData]...)
	result = append(result, token...)
	result = append(result, fixture.data[fixture.layout.biasData+1:]...)
	binary.LittleEndian.PutUint32(result[fixture.layout.biasLength:], uint32(transformerLanes+delta))
	return result
}

func requireLoadError(t *testing.T, data []byte, target error) {
	t.Helper()
	model, err := Load(bytes.NewReader(data))
	if model != nil {
		t.Fatal("failed load published a model")
	}
	if !errors.Is(err, target) {
		t.Fatalf("Load error = %v, want errors.Is(..., %v)", err, target)
	}
}

func TestLoadCanonicalSyntheticModel(t *testing.T) {
	fixture := zeroFixture()
	model, err := Load(bytes.NewReader(fixture.data))
	if err != nil {
		t.Fatal(err)
	}
	metadata := model.Metadata()
	if metadata.FileSize != int64(len(fixture.data)) || metadata.Description != fixture.desc {
		t.Fatalf("metadata size/description = %d/%q", metadata.FileSize, metadata.Description)
	}
	if metadata.Version != fileVersion || metadata.NetworkHash != networkHash ||
		metadata.TransformerHash != transformerHash || metadata.ArchitectureHash != architectureHash {
		t.Fatalf("metadata structural identity = %+v", metadata)
	}
	if metadata.FileSHA256 != sha256.Sum256(fixture.data) {
		t.Fatal("metadata file digest mismatch")
	}
	if metadata.ArchitectureID != architectureID || metadata.FeatureSetID != featureSetID ||
		metadata.QuantizationID != quantizationID || metadata.ScoreContractID != scoreContractID {
		t.Fatalf("metadata semantic identity = %+v", metadata)
	}
	if len(model.featureWeights) != inputFeatures*transformerLanes || len(model.psqtWeights) != inputFeatures*psqtBuckets {
		t.Fatal("decoded tensor dimensions mismatch")
	}
	if model.featureBias[0] != 0 || model.featureWeights[len(model.featureWeights)-1] != 0 ||
		model.psqtWeights[len(model.psqtWeights)-1] != 0 {
		t.Fatal("zero tensor changed while loading")
	}
}

func TestLoadRejectsMalformedAndNeverPublishes(t *testing.T) {
	fixture := zeroFixture()
	tests := []struct {
		name string
		data func() []byte
		want error
	}{
		{"empty", func() []byte { return nil }, ErrFormat},
		{"short header", func() []byte { return fixture.data[:11] }, ErrFormat},
		{"unsupported version", func() []byte {
			b := cloneBytes(fixture.data)
			binary.LittleEndian.PutUint32(b, fileVersion+1)
			return b
		}, ErrUnsupported},
		{"unsupported network", func() []byte {
			b := cloneBytes(fixture.data)
			binary.LittleEndian.PutUint32(b[4:], networkHash+1)
			return b
		}, ErrUnsupported},
		{"description cap", func() []byte {
			b := cloneBytes(fixture.data[:12])
			binary.LittleEndian.PutUint32(b[8:], maxDescriptionBytes+1)
			return b
		}, ErrFormat},
		{"transformer hash", func() []byte {
			b := cloneBytes(fixture.data)
			binary.LittleEndian.PutUint32(b[fixture.layout.transformerHash:], transformerHash+1)
			return b
		}, ErrIntegrity},
		{"bias length too short", func() []byte {
			b := cloneBytes(fixture.data)
			binary.LittleEndian.PutUint32(b[fixture.layout.biasLength:], transformerLanes-1)
			return b
		}, ErrFormat},
		{"bias length too long", func() []byte {
			b := cloneBytes(fixture.data)
			binary.LittleEndian.PutUint32(b[fixture.layout.biasLength:], transformerLanes*3+1)
			return b
		}, ErrFormat},
		{"noncanonical zero", func() []byte { return replaceFirstBiasToken(fixture, []byte{0x80, 0x00}) }, ErrFormat},
		{"unterminated int16", func() []byte {
			b := cloneBytes(fixture.data)
			copy(b[fixture.layout.biasData:], []byte{0x80, 0x80, 0x80})
			return b
		}, ErrFormat},
		{"int16 target overflow", func() []byte { return replaceFirstBiasToken(fixture, []byte{0x80, 0x80, 0x02}) }, ErrRange},
		{"checked doubling overflow", func() []byte { return replaceFirstBiasToken(fixture, []byte{0x80, 0x80, 0x01}) }, ErrRange},
		{"truncated final weight", func() []byte { return fixture.data[:len(fixture.data)-1] }, ErrFormat},
		{"trailing byte", func() []byte { b := cloneBytes(fixture.data); return append(b, 0) }, ErrFormat},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			requireLoadError(t, test.data(), test.want)
		})
	}
	for name, offset := range map[string]int{
		"bias":   fixture.layout.biasMagic,
		"weight": fixture.layout.weightMagic,
		"PSQT":   fixture.layout.psqtMagic,
	} {
		t.Run(name+" block magic", func(t *testing.T) {
			b := cloneBytes(fixture.data)
			b[offset] ^= 1
			requireLoadError(t, b, ErrFormat)
		})
	}
	for stack, offset := range fixture.layout.stacks {
		t.Run(fmt.Sprintf("stack %d hash", stack), func(t *testing.T) {
			b := cloneBytes(fixture.data)
			binary.LittleEndian.PutUint32(b[offset:], architectureHash+1)
			requireLoadError(t, b, ErrIntegrity)
		})
		t.Run(fmt.Sprintf("stack %d truncated hash", stack), func(t *testing.T) {
			requireLoadError(t, fixture.data[:offset+3], ErrFormat)
		})
	}
}

func TestLoadNilAndNoProgressReader(t *testing.T) {
	model, err := Load(nil)
	if model != nil || !errors.Is(err, ErrFormat) {
		t.Fatalf("Load(nil) = %v, %v", model, err)
	}
	model, err = Load(stuckReader{})
	if model != nil || !errors.Is(err, ErrFormat) || !errors.Is(err, io.ErrNoProgress) {
		t.Fatalf("Load(stuck) = %v, %v", model, err)
	}
}

type stuckReader struct{}

func (stuckReader) Read([]byte) (int, error) { return 0, nil }

type chunkReader struct {
	data []byte
	max  int
}

func (r *chunkReader) Read(dst []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, io.EOF
	}
	if len(dst) > r.max {
		dst = dst[:r.max]
	}
	n := copy(dst, r.data)
	r.data = r.data[n:]
	return n, nil
}

func TestLoadChunkedReader(t *testing.T) {
	fixture := zeroFixture()
	reader := &chunkReader{data: fixture.data, max: 8191}
	model, err := Load(reader)
	if err != nil {
		t.Fatal(err)
	}
	if model.Metadata().FileSHA256 != sha256.Sum256(fixture.data) {
		t.Fatal("chunked-reader digest mismatch")
	}
}

func TestLoadedModelDoesNotAliasInputOrMetadata(t *testing.T) {
	fixture := zeroFixture()
	input := cloneBytes(fixture.data)
	model, err := Load(bytes.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	before := model.Metadata()
	input[0] ^= 0xff
	copy(input[12:], []byte("overwritten"))
	copyMetadata := model.Metadata()
	copyMetadata.FileSHA256[0] ^= 0xff
	copyMetadata.Description = "changed"
	if model.Metadata() != before {
		t.Fatal("model metadata aliases caller or returned metadata")
	}
	if model.featureBias[0] != 0 || model.featureWeights[0] != 0 {
		t.Fatal("model tensors alias caller input")
	}
}

func TestConcurrentIndependentLoadsAndMetadataReads(t *testing.T) {
	fixture := zeroFixture()
	const workers = 4
	models := make(chan *Model, workers)
	errorsSeen := make(chan error, workers)
	var loadGroup sync.WaitGroup
	for range [workers]struct{}{} {
		loadGroup.Add(1)
		go func() {
			defer loadGroup.Done()
			model, err := Load(bytes.NewReader(fixture.data))
			if err != nil {
				errorsSeen <- err
				return
			}
			models <- model
		}()
	}
	loadGroup.Wait()
	close(models)
	close(errorsSeen)
	for err := range errorsSeen {
		t.Fatal(err)
	}
	if len(models) != workers {
		t.Fatalf("loaded models = %d, want %d", len(models), workers)
	}
	var readGroup sync.WaitGroup
	for model := range models {
		model := model
		for range [8]struct{}{} {
			readGroup.Add(1)
			go func() {
				defer readGroup.Done()
				if got := model.Metadata(); got.FileSHA256 != sha256.Sum256(fixture.data) || got.Description != fixture.desc {
					t.Errorf("concurrent metadata = %+v", got)
				}
			}()
		}
	}
	readGroup.Wait()
}

func TestLoadOfficialSF18Small(t *testing.T) {
	path := os.Getenv(officialFileEnvironment)
	if path == "" {
		t.Skip(officialFileEnvironment + " is required for the explicit official-file gate")
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
	wantDigest := [sha256.Size]byte{0x37, 0xf1, 0x8f, 0x62, 0xd7, 0x72, 0xf3, 0x10, 0x7e, 0x1d, 0x6a, 0xac, 0xa3, 0x89, 0x8c, 0x13, 0x0c, 0x3c, 0x86, 0xf2, 0xab, 0x63, 0xe6, 0x55, 0x5f, 0xbb, 0xca, 0x20, 0x63, 0x5a, 0x89, 0x9d}
	metadata := model.Metadata()
	if metadata.FileSHA256 != wantDigest || metadata.FileSize != 3519630 {
		t.Fatalf("official identity = %x/%d", metadata.FileSHA256, metadata.FileSize)
	}
	if metadata.Description != "Network trained with the https://github.com/official-stockfish/nnue-pytorch trainer." {
		t.Fatalf("official description = %q", metadata.Description)
	}
	biasMin, biasMax := minMaxInt16(model.featureBias[:])
	weightMin, weightMax := minMaxInt16(model.featureWeights)
	psqtMin, psqtMax := minMaxInt32(model.psqtWeights)
	if biasMin != -412 || biasMax != 254 || weightMin != -1190 || weightMax != 1826 || psqtMin != -49757 || psqtMax != 49421 {
		t.Fatalf("official tensor ranges = bias[%d,%d] weight[%d,%d] psqt[%d,%d]", biasMin, biasMax, weightMin, weightMax, psqtMin, psqtMax)
	}
	if min, max := minMaxInt8(model.stacks[0].fc0Weight[:]); min != -127 || max != 127 {
		t.Fatalf("official stack0 FC0 weight range = [%d,%d]", min, max)
	}
	if model.stacks[0].fc2Bias[0] != 2377 || model.stacks[7].fc2Bias[0] != 922 {
		t.Fatalf("official boundary stack FC2 biases = %d/%d", model.stacks[0].fc2Bias[0], model.stacks[7].fc2Bias[0])
	}
}

func minMaxInt16(values []int16) (int16, int16) {
	minimum, maximum := values[0], values[0]
	for _, value := range values[1:] {
		if value < minimum {
			minimum = value
		}
		if value > maximum {
			maximum = value
		}
	}
	return minimum, maximum
}

func minMaxInt32(values []int32) (int32, int32) {
	minimum, maximum := values[0], values[0]
	for _, value := range values[1:] {
		if value < minimum {
			minimum = value
		}
		if value > maximum {
			maximum = value
		}
	}
	return minimum, maximum
}

func minMaxInt8(values []int8) (int8, int8) {
	minimum, maximum := values[0], values[0]
	for _, value := range values[1:] {
		if value < minimum {
			minimum = value
		}
		if value > maximum {
			maximum = value
		}
	}
	return minimum, maximum
}
