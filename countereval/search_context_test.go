package countereval

import (
	"bytes"
	"math"
	"strings"
	"testing"
)

func loadedSearchModel(t *testing.T, values map[int]float32) *Model {
	t.Helper()
	model, _, err := LoadCounter55Legacy(bytes.NewReader(makeLegacyFile(values)))
	if err != nil {
		t.Fatal(err)
	}
	return model
}

func TestSearchContextRequiresLoaderValidatedModel(t *testing.T) {
	if _, err := (*Model)(nil).NewSearchContext(Board{}); err == nil || !strings.Contains(err.Error(), "loader-validated") {
		t.Fatalf("nil model error=%v", err)
	}
	if _, err := new(Model).NewSearchContext(Board{}); err == nil || !strings.Contains(err.Error(), "loader-validated") {
		t.Fatalf("zero model error=%v", err)
	}

	model := loadedSearchModel(t, nil)
	metadata, err := model.ValidatedMetadata()
	if err != nil {
		t.Fatal(err)
	}
	if metadata.SHA256 == "" || metadata.Bytes != LegacyFileSize || metadata.Values != payloadValues {
		t.Fatalf("metadata=%+v", metadata)
	}
}

func TestSearchContextGrowsAndRestoresBitExactNullFrames(t *testing.T) {
	model := loadedSearchModel(t, map[int]float32{
		hiddenBiasesStart:  1.25,
		outputWeightsStart: -0.5,
		outputBiasIndex:    9,
	})
	root := boardWith(pieceAt(5, 4), pieceAt(11, 60))
	context, err := model.NewSearchContext(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := context.FrameCapacity(); got != CompatibilityFrameCount {
		t.Fatalf("initial capacity=%d", got)
	}
	rootAccumulator := context.accumulators[0]
	rootRaw := context.EvaluateRaw()
	for push := 0; push < CompatibilityFrameCount; push++ {
		if err := context.PushNull(); err != nil {
			t.Fatalf("push %d: %v", push, err)
		}
	}
	if context.Depth() != CompatibilityFrameCount || context.FrameCapacity() != 2*CompatibilityFrameCount {
		t.Fatalf("depth=%d capacity=%d", context.Depth(), context.FrameCapacity())
	}
	if context.Board() != root {
		t.Fatal("null growth changed board")
	}
	requireAccumulatorBits(t, context.accumulators[context.depth], rootAccumulator)
	if math.Float32bits(context.EvaluateRaw()) != math.Float32bits(rootRaw) {
		t.Fatal("null growth changed raw value")
	}
	for context.Depth() != 0 {
		if err := context.Pop(); err != nil {
			t.Fatal(err)
		}
	}
	if context.Board() != root || math.Float32bits(context.EvaluateRaw()) != math.Float32bits(rootRaw) {
		t.Fatal("pop did not restore root")
	}
}

func TestSearchContextRejectsBeforeCapacityGrowth(t *testing.T) {
	model := loadedSearchModel(t, nil)
	root := boardWith(pieceAt(5, 4), pieceAt(11, 60), pieceAt(0, 8))
	context, err := model.NewSearchContext(root)
	if err != nil {
		t.Fatal(err)
	}
	for context.Depth() < CompatibilityFrameCount-1 {
		if err := context.PushNull(); err != nil {
			t.Fatal(err)
		}
	}
	beforeDepth := context.Depth()
	beforeCapacity := context.FrameCapacity()
	beforeBoard := context.Board()
	beforeRaw := context.EvaluateRaw()
	beforeAccumulator := context.accumulators[context.depth]

	invalid := MoveDelta{MovingPlane: 0, From: 9, To: 17}
	if err := context.PushMove(invalid, root); err == nil {
		t.Fatal("missing mover unexpectedly accepted")
	}
	if context.Depth() != beforeDepth || context.FrameCapacity() != beforeCapacity || context.Board() != beforeBoard ||
		math.Float32bits(context.EvaluateRaw()) != math.Float32bits(beforeRaw) {
		t.Fatal("rejected transition changed public context state or capacity")
	}
	requireAccumulatorBits(t, context.accumulators[context.depth], beforeAccumulator)
}

func TestSearchContextUnsafeGrowthIsTransactional(t *testing.T) {
	const updateWeight = float32(1e35)
	model := loadedSearchModel(t, map[int]float32{hiddenWeightsStart: updateWeight})
	context, err := model.NewSearchContext(Board{})
	if err != nil {
		t.Fatalf("initial capacity unexpectedly unsafe: %v", err)
	}
	for context.Depth() < CompatibilityFrameCount-1 {
		if err := context.PushNull(); err != nil {
			t.Fatal(err)
		}
	}
	beforeDepth := context.Depth()
	beforeCapacity := context.FrameCapacity()
	beforeBoard := context.Board()
	beforeAccumulator := context.accumulators[context.depth]
	if err := context.PushNull(); err == nil || !strings.Contains(err.Error(), "unsafe") {
		t.Fatalf("unsafe growth error=%v", err)
	}
	if context.Depth() != beforeDepth || context.FrameCapacity() != beforeCapacity || context.Board() != beforeBoard {
		t.Fatal("unsafe growth changed context state or capacity")
	}
	requireAccumulatorBits(t, context.accumulators[context.depth], beforeAccumulator)
}

func TestSearchContextUnsafeResetIsTransactional(t *testing.T) {
	const updateWeight = float32(1.6e35)
	values := make(map[int]float32, 64)
	for square := 0; square < 64; square++ {
		values[hiddenWeightsStart+square*HiddenSize] = updateWeight
	}
	model := loadedSearchModel(t, values)
	root := boardWith(pieceAt(5, 4), pieceAt(11, 60))
	context, err := model.NewSearchContext(root)
	if err != nil {
		t.Fatalf("initial root unexpectedly unsafe: %v", err)
	}
	if err := context.PushNull(); err != nil {
		t.Fatal(err)
	}
	beforeDepth := context.Depth()
	beforeCapacity := context.FrameCapacity()
	beforeBoard := context.Board()
	beforeRaw := context.EvaluateRaw()
	beforeAccumulator := context.accumulators[context.depth]
	unsafeRoot := Board{}
	unsafeRoot[0] = ^uint64(0)
	if err := context.Reset(unsafeRoot); err == nil || !strings.Contains(err.Error(), "unsafe") {
		t.Fatalf("unsafe reset error=%v", err)
	}
	if context.Depth() != beforeDepth || context.FrameCapacity() != beforeCapacity || context.Board() != beforeBoard ||
		math.Float32bits(context.EvaluateRaw()) != math.Float32bits(beforeRaw) {
		t.Fatal("unsafe reset changed active state or capacity")
	}
	requireAccumulatorBits(t, context.accumulators[context.depth], beforeAccumulator)
}

func TestSearchContextFrameDoublingRejectsOverflow(t *testing.T) {
	maximumInt := int(^uint(0) >> 1)
	if _, err := doubledSearchFrameCount(0); err == nil {
		t.Fatal("zero frames doubled")
	}
	if _, err := doubledSearchFrameCount(maximumInt/2 + 1); err == nil || !strings.Contains(err.Error(), "overflow") {
		t.Fatalf("overflow error=%v", err)
	}
	if got, err := doubledSearchFrameCount(CompatibilityFrameCount); err != nil || got != 2*CompatibilityFrameCount {
		t.Fatalf("ordinary doubling=%d, %v", got, err)
	}
}
