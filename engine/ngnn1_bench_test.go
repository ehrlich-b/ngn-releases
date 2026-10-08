package engine

import (
	"bytes"
	"fmt"
	"testing"
)

func ngnn1BenchNetwork(b *testing.B, h int) *NGNN1Network {
	b.Helper()
	n, err := ReadNGNN1(bytes.NewReader(ngnn1TestBytes(h, 19, false)))
	if err != nil {
		b.Fatal(err)
	}
	return n
}

func BenchmarkNGNN1Push(b *testing.B) {
	for _, h := range []int{256, 512} {
		b.Run(fmt.Sprintf("H%d", h), func(b *testing.B) {
			benchmarkNGNN1Push(b, h)
		})
	}
}

func benchmarkNGNN1Push(b *testing.B, h int) {
	n := ngnn1BenchNetwork(b, h)
	pos := newUCIStartingPosition()
	s := ngnn1State{network: n}
	s.reset(pos)
	s.ensure(1)
	delta := ngnn1MoveDelta(NewMove(E2, E4, WhitePawn, NoPiece, NoType, 0))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.push(1, delta)
	}
}

var ngnn1BenchScore int

func BenchmarkNGNN1Output(b *testing.B) {
	for _, h := range []int{256, 512} {
		b.Run(fmt.Sprintf("H%d", h), func(b *testing.B) {
			benchmarkNGNN1Output(b, h)
		})
	}
}

func benchmarkNGNN1Output(b *testing.B, h int) {
	n := ngnn1BenchNetwork(b, h)
	a := make([]int32, 2*n.hidden)
	n.refresh(newUCIStartingPosition(), a)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ngnn1BenchScore = n.evaluate(a, White)
	}
}

func BenchmarkNGNN1Rows(b *testing.B) {
	for _, h := range []int{256, 512} {
		b.Run(fmt.Sprintf("H%d", h), func(b *testing.B) {
			n := ngnn1BenchNetwork(b, h)
			src, dst := make([]int32, h), make([]int32, h)
			remove, capture, add, rook := n.row(0), n.row(1), n.row(2), n.row(3)
			for _, shape := range []string{"move", "capture", "castle"} {
				b.Run(shape, func(b *testing.B) {
					b.ReportAllocs()
					for i := 0; i < b.N; i++ {
						switch shape {
						case "move":
							ngnn1MoveRows(dst, src, remove, add)
						case "capture":
							ngnn1CaptureRows(dst, src, remove, capture, add)
						case "castle":
							ngnn1CastleRows(dst, src, remove, capture, add, rook)
						}
					}
				})
			}
		})
	}
}

func BenchmarkNGNN1OutputWide(b *testing.B) {
	for _, h := range []int{256, 512} {
		b.Run(fmt.Sprintf("H%d", h), func(b *testing.B) {
			n, err := ReadNGNN1(bytes.NewReader(ngnn1TestBytes(h, 19, true)))
			if err != nil {
				b.Fatal(err)
			}
			a := make([]int32, 2*h)
			n.refresh(newUCIStartingPosition(), a)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ngnn1BenchScore = n.evaluate(a, White)
			}
		})
	}
}
