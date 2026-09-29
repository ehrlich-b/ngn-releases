package sf18small

import (
	"errors"
	"os"
	"testing"

	base "github.com/ehrlich-b/ngn/nnue"
)

func bucketPosition(bucket int) base.Position {
	count := (bucket + 1) * 4
	pieces := []base.PieceOnSquare{
		sfPiece(base.King, base.White, 4),
		sfPiece(base.King, base.Black, 60),
	}
	for square := 0; len(pieces) < count; square++ {
		if square == 4 || square == 60 {
			continue
		}
		pieces = append(pieces, sfPiece(base.Pawn, base.Color(square&1), base.Square(square)))
	}
	return base.Position{SideToMove: base.Color(bucket & 1), Pieces: pieces}
}

func requireSelectedMatchesAll(t *testing.T, model *Model, position base.Position) {
	t.Helper()
	all, err := model.EvaluateAll(position)
	if err != nil {
		t.Fatal(err)
	}
	selected, err := model.EvaluateSelected(position)
	if err != nil {
		t.Fatal(err)
	}
	if selected.Bucket != all.CorrectBucket || selected.Components != all.Buckets[all.CorrectBucket] {
		t.Fatalf("selected = %+v, all bucket %d = %+v", selected, all.CorrectBucket, all.Buckets[all.CorrectBucket])
	}
}

func TestEvaluateSelectedMatchesAllStacks(t *testing.T) {
	model := incrementalContextModel()
	for bucket := 0; bucket < layerStacks; bucket++ {
		position := bucketPosition(bucket)
		t.Run(string(rune('0'+bucket)), func(t *testing.T) {
			requireSelectedMatchesAll(t, model, position)
			position.SideToMove ^= 1
			requireSelectedMatchesAll(t, model, position)
		})
	}
}

func TestEvaluateSelectedUsesDefinedWrapping(t *testing.T) {
	model := blankEvaluationModel()
	for lane := range model.featureBias {
		model.featureBias[lane] = int16(32760 - lane)
	}
	for index := range model.featureWeights {
		model.featureWeights[index] = int16((index%31)*211 - 3200)
	}
	for index := range model.psqtWeights {
		model.psqtWeights[index] = int32(uint32(index)*2654435761 + 0x7ffff000)
	}
	for bucket := range model.stacks {
		stack := &model.stacks[bucket]
		for index := range stack.fc0Bias {
			stack.fc0Bias[index] = int32(uint32(index+bucket)*0x70000001 + 0x7fffff00)
		}
		for index := range stack.fc0Weight {
			stack.fc0Weight[index] = int8(index*17 + bucket*31)
		}
		for index := range stack.fc1Bias {
			stack.fc1Bias[index] = int32(uint32(index+bucket)*0x60000001 + 0x7ffff000)
		}
		for index := range stack.fc1Weight {
			stack.fc1Weight[index] = int8(index*29 + bucket*11)
		}
		stack.fc2Bias[0] = int32(uint32(bucket)*0x50000001 + 0x7ffffff0)
		for index := range stack.fc2Weight {
			stack.fc2Weight[index] = int8(index*43 + bucket*7)
		}
	}
	for bucket := 0; bucket < layerStacks; bucket++ {
		requireSelectedMatchesAll(t, model, bucketPosition(bucket))
	}
}

func TestContextEvaluateSelectedMatchesFullRefresh(t *testing.T) {
	model := incrementalContextModel()
	for _, test := range contextRealCases() {
		t.Run(test.name, func(t *testing.T) {
			context, err := NewContext(model)
			if err != nil {
				t.Fatal(err)
			}
			if err := context.Reset(test.before); err != nil {
				t.Fatal(err)
			}
			delta := contextDelta(t, test.kind, test.before, test.after, test.removed, test.added)
			if err := context.PushDelta(delta); err != nil {
				t.Fatal(err)
			}
			got, err := context.EvaluateSelected()
			if err != nil {
				t.Fatal(err)
			}
			want, err := model.EvaluateSelected(test.after)
			if err != nil {
				t.Fatal(err)
			}
			if got != want {
				t.Fatalf("incremental selected = %+v, full refresh %+v", got, want)
			}
		})
	}
}

func TestEvaluateSelectedRejectsUnreadyInputs(t *testing.T) {
	valid := base.Position{SideToMove: base.White, Pieces: []base.PieceOnSquare{
		sfPiece(base.King, base.White, 4),
		sfPiece(base.King, base.Black, 60),
	}}
	if got, err := (*Model)(nil).EvaluateSelected(valid); got != (SelectedEvaluation{}) || !errors.Is(err, ErrEvaluation) {
		t.Fatalf("nil model = %+v, %v", got, err)
	}
	context, err := NewContext(blankEvaluationModel())
	if err != nil {
		t.Fatal(err)
	}
	if got, err := context.EvaluateSelected(); got != (SelectedEvaluation{}) || !errors.Is(err, ErrContext) {
		t.Fatalf("unreset context = %+v, %v", got, err)
	}
}

func officialBenchmarkModel(b *testing.B) *Model {
	b.Helper()
	path := os.Getenv(officialFileEnvironment)
	if path == "" {
		b.Skip(officialFileEnvironment + " is required")
	}
	file, err := os.Open(path)
	if err != nil {
		b.Fatal(err)
	}
	defer file.Close()
	model, err := Load(file)
	if err != nil {
		b.Fatal(err)
	}
	return model
}

func benchmarkPosition() base.Position {
	return base.Position{SideToMove: base.White, Pieces: []base.PieceOnSquare{
		sfPiece(base.Rook, base.White, 0), sfPiece(base.Knight, base.White, 1),
		sfPiece(base.Bishop, base.White, 2), sfPiece(base.Queen, base.White, 3),
		sfPiece(base.King, base.White, 4), sfPiece(base.Bishop, base.White, 5),
		sfPiece(base.Knight, base.White, 6), sfPiece(base.Rook, base.White, 7),
		sfPiece(base.Pawn, base.White, 8), sfPiece(base.Pawn, base.White, 9),
		sfPiece(base.Pawn, base.White, 10), sfPiece(base.Pawn, base.White, 11),
		sfPiece(base.Pawn, base.White, 12), sfPiece(base.Pawn, base.White, 13),
		sfPiece(base.Pawn, base.White, 14), sfPiece(base.Pawn, base.White, 15),
		sfPiece(base.Pawn, base.Black, 48), sfPiece(base.Pawn, base.Black, 49),
		sfPiece(base.Pawn, base.Black, 50), sfPiece(base.Pawn, base.Black, 51),
		sfPiece(base.Pawn, base.Black, 52), sfPiece(base.Pawn, base.Black, 53),
		sfPiece(base.Pawn, base.Black, 54), sfPiece(base.Pawn, base.Black, 55),
		sfPiece(base.Rook, base.Black, 56), sfPiece(base.Knight, base.Black, 57),
		sfPiece(base.Bishop, base.Black, 58), sfPiece(base.Queen, base.Black, 59),
		sfPiece(base.King, base.Black, 60), sfPiece(base.Bishop, base.Black, 61),
		sfPiece(base.Knight, base.Black, 62), sfPiece(base.Rook, base.Black, 63),
	}}
}

func BenchmarkOfficialFullRefreshOutput(b *testing.B) {
	model := officialBenchmarkModel(b)
	position := benchmarkPosition()
	b.ReportAllocs()
	b.Run("all-stacks", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if _, err := model.EvaluateAll(position); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("selected-stack", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if _, err := model.EvaluateSelected(position); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkOfficialIncrementalOutput(b *testing.B) {
	model := officialBenchmarkModel(b)
	context, err := NewContext(model)
	if err != nil {
		b.Fatal(err)
	}
	if err := context.Reset(benchmarkPosition()); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.Run("all-stacks", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if _, err := context.EvaluateAll(); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("selected-stack", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if _, err := context.EvaluateSelected(); err != nil {
				b.Fatal(err)
			}
		}
	})
}
