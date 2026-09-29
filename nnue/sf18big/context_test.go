package sf18big

import (
	"errors"
	"testing"

	base "github.com/ehrlich-b/ngn/nnue"
)

func TestContextDirtyCommitDoesNotTouchNumericPayload(t *testing.T) {
	context, err := NewContext(&Model{loaded: true})
	if err != nil {
		t.Fatal(err)
	}
	context.initialized = true
	context.frames[0].sideToMove = base.White
	context.frames[1].state.BaseValues[0][17] = 1234
	context.frames[1].state.ThreatValues[1][23] = -2345
	context.frames[1].state.BasePSQT[0][3] = 3456
	context.frames[1].state.ThreatPSQT[1][5] = -4567
	context.frames[1].state.Threats[0].Count = 1
	context.frames[1].state.Threats[0].Indices[0] = 5678
	context.frames[1].baseComputed = [2]bool{true, true}
	context.frames[1].threatComputed = [2]bool{true, true}

	context.commitReal(base.Delta{}, context.board, base.PositionFacts{})
	frame := &context.frames[1]
	if frame.state.BaseValues[0][17] != 1234 ||
		frame.state.ThreatValues[1][23] != -2345 ||
		frame.state.BasePSQT[0][3] != 3456 ||
		frame.state.ThreatPSQT[1][5] != -4567 ||
		frame.state.Threats[0].Count != 1 ||
		frame.state.Threats[0].Indices[0] != 5678 {
		t.Fatal("dirty commit wrote the numeric payload")
	}
	if frame.baseComputed != [2]bool{} || frame.threatComputed != [2]bool{} {
		t.Fatal("dirty commit retained computed flags")
	}
}

func TestContextLifecycleErrors(t *testing.T) {
	if _, err := NewContext(nil); !errors.Is(err, ErrContext) {
		t.Fatalf("nil model error = %v", err)
	}
	if _, err := NewContext(&Model{}); !errors.Is(err, ErrContext) {
		t.Fatalf("unloaded model error = %v", err)
	}
	context, err := NewContext(&Model{loaded: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := context.EvaluateSelected(); !errors.Is(err, ErrContext) {
		t.Fatalf("evaluate before reset error = %v", err)
	}
	if err := context.Pop(); !errors.Is(err, ErrContext) {
		t.Fatalf("pop before reset error = %v", err)
	}
	context.initialized = true
	context.depth = base.MaxContextPly
	if err := context.PushDelta(base.Delta{}); !errors.Is(err, ErrContext) {
		t.Fatalf("stack overflow error = %v", err)
	}
	if context.Depth() != base.MaxContextPly {
		t.Fatalf("failed overflow changed depth to %d", context.Depth())
	}
}
