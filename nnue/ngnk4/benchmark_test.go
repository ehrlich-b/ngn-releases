package ngnk4

import (
	"os"
	"testing"
)

func loadBenchmarkModel(b *testing.B) *Model {
	b.Helper()
	path := os.Getenv("NGN_K4_BENCH_MODEL")
	if path == "" {
		b.Skip("NGN_K4_BENCH_MODEL is not set")
	}
	model, err := LoadFile(path)
	if err != nil {
		b.Fatal(err)
	}
	return model
}

func benchmarkRoot() Position {
	return Position{
		Board: Board{
			0x000000000000ff00,
			0x0000000000000042,
			0x0000000000000024,
			0x0000000000000081,
			0x0000000000000008,
			0x0000000000000010,
			0x00ff000000000000,
			0x4200000000000000,
			0x2400000000000000,
			0x8100000000000000,
			0x0800000000000000,
			0x1000000000000000,
		},
		SideToMove: White,
	}
}

func BenchmarkEvaluateIncremental(b *testing.B) {
	model := loadBenchmarkModel(b)
	context, err := model.NewSearchContext(benchmarkRoot())
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	var score int64
	for i := 0; i < b.N; i++ {
		score, err = context.EvaluateRaw()
		if err != nil {
			b.Fatal(err)
		}
	}
	benchmarkScoreSink = score
}

func BenchmarkFullRefresh(b *testing.B) {
	model := loadBenchmarkModel(b)
	root := benchmarkRoot()
	b.ReportAllocs()
	b.ResetTimer()
	var state accumulator
	for i := 0; i < b.N; i++ {
		state = model.fullRefresh(root.Board)
	}
	benchmarkAccumulatorSink = state
}

func BenchmarkPushPopQuietMove(b *testing.B) {
	model := loadBenchmarkModel(b)
	root := benchmarkRoot()
	context, err := model.NewSearchContext(root)
	if err != nil {
		b.Fatal(err)
	}
	post := root
	post.Board[WhitePawn] &^= uint64(1) << 12
	post.Board[WhitePawn] |= uint64(1) << 28
	post.SideToMove = Black
	delta := MoveDelta{MovingPlane: WhitePawn, From: 12, To: 28}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := context.PushMove(delta, post); err != nil {
			b.Fatal(err)
		}
		if err := context.Pop(); err != nil {
			b.Fatal(err)
		}
	}
}

var benchmarkScoreSink int64
var benchmarkAccumulatorSink accumulator
