package engine

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"math/rand"
	"reflect"
	"testing"
)

// Misaligned views and guards catch wrong strides and writes past the view.
func ngnn1Guarded32(h, offset int) ([]int32, []int32) {
	storage := make([]int32, h+16)
	for i := range storage {
		storage[i] = 0x12345678
	}
	return storage, storage[offset : offset+h]
}

func TestNGNN1KernelRowsDifferential(t *testing.T) {
	rng := rand.New(rand.NewSource(610051))
	limits := [...]int32{-2147483648, -2130000, -1, 0, 1, 254, 255, 256, 2130000, 2147483647}
	for h := 16; h <= ngnn1MaxHidden; h += 16 {
		t.Run(fmt.Sprint(h), func(t *testing.T) {
			for sample := 0; sample < 32; sample++ {
				_, src := ngnn1Guarded32(h, 1+sample%7)
				for i := range src {
					src[i] = int32(rng.Uint32())
					if sample < 10 {
						src[i] = limits[(i+sample)%len(limits)]
					}
				}
				before := append([]int32(nil), src...)
				var rows [4][]int16
				for r := range rows {
					storage := make([]int16, h+16)
					rows[r] = storage[1+r : h+1+r]
					for i := range rows[r] {
						rows[r][i] = int16(rng.Uint32())
						if sample < 2 {
							rows[r][i] = []int16{-32768, 32767}[(i+r+sample)%2]
						}
					}
				}
				cases := []struct {
					name string
					got  func([]int32, []int32)
					want func([]int32, []int32)
				}{
					{"move", func(d, s []int32) { ngnn1MoveRows(d, s, rows[0], rows[2]) }, func(d, s []int32) { ngnn1MoveRowsGo(d, s, rows[0], rows[2]) }},
					{"capture", func(d, s []int32) { ngnn1CaptureRows(d, s, rows[0], rows[1], rows[2]) }, func(d, s []int32) { ngnn1CaptureRowsGo(d, s, rows[0], rows[1], rows[2]) }},
					{"castle", func(d, s []int32) { ngnn1CastleRows(d, s, rows[0], rows[1], rows[2], rows[3]) }, func(d, s []int32) { ngnn1CastleRowsGo(d, s, rows[0], rows[1], rows[2], rows[3]) }},
				}
				for _, c := range cases {
					for _, alias := range []bool{false, true} {
						storage, dst := ngnn1Guarded32(h, 1+(sample+3)%7)
						wantStorage := append([]int32(nil), storage...)
						want := wantStorage[1+(sample+3)%7 : 1+(sample+3)%7+h]
						if alias {
							copy(dst, src)
							copy(want, src)
							c.got(dst, dst)
							c.want(want, want)
						} else {
							c.got(dst, src)
							c.want(want, src)
						}
						if !reflect.DeepEqual(storage, wantStorage) {
							t.Fatalf("%s sample=%d alias=%v backend=%s", c.name, sample, alias, ngnn1KernelBackend)
						}
					}
				}
				for _, sub := range []bool{false, true} {
					storage, dst := ngnn1Guarded32(h, 3)
					copy(dst, src)
					wantStorage := append([]int32(nil), storage...)
					want := wantStorage[3 : h+3]
					if sub {
						ngnn1SubRow(dst, rows[0])
						ngnn1SubRowGo(want, rows[0])
					} else {
						ngnn1AddRow(dst, rows[0])
						ngnn1AddRowGo(want, rows[0])
					}
					if !reflect.DeepEqual(storage, wantStorage) {
						t.Fatalf("single row sample=%d sub=%v", sample, sub)
					}
				}
				if !reflect.DeepEqual(src, before) {
					t.Fatal("changed source")
				}
			}
		})
	}
}

func TestNGNN1KernelOutputDifferential(t *testing.T) {
	rng := rand.New(rand.NewSource(610052))
	limits := [...]int32{-2147483648, -1, 0, 1, 127, 181, 182, 254, 255, 256, 32767, 65535, 2147483647}
	for h := 16; h <= ngnn1MaxHidden; h += 16 {
		t.Run(fmt.Sprint(h), func(t *testing.T) {
			// Each sample is a valid sparse random NGNN1 network, with four
			// random transformer rows and random output weights/biases.
			n := &NGNN1Network{hidden: h, weights: make([]int16, ngnn1Features*h), bias: make([]int16, h)}
			for sample := 0; sample < 32; sample++ {
				storage := make([]int32, 2*h+16)
				acc := storage[1+sample%7 : 1+sample%7+2*h]
				wStorage := make([]int16, 2*h+16)
				n.output = wStorage[1+sample%7 : 1+sample%7+2*h]
				n.outputBias = int32(rng.Uint32())
				for i := range n.bias {
					n.bias[i] = int16(rng.Uint32())
				}
				for i := 0; i < 4*h; i++ {
					n.weights[i] = int16(rng.Uint32())
				}
				for i := range acc {
					acc[i] = int32(rng.Uint32())
					if sample%3 == 0 {
						acc[i] = limits[(i+sample)%len(limits)]
					} else if sample%3 == 1 {
						acc[i] = int32(n.bias[i%h])
						for r := 0; r < 4; r++ {
							acc[i] += int32(n.weights[r*h+i%h])
						}
					}
					n.output[i] = int16(rng.Uint32())
					if sample < 4 {
						acc[i] = 255
						n.output[i] = []int16{-32768, 32767}[sample%2]
						if sample >= 2 && i >= h {
							n.output[i] = []int16{32767, -32768}[sample%2]
						}
					}
				}
				if !n.valid() {
					t.Fatal("invalid differential network")
				}
				for half := 0; half < 2; half++ {
					a, w := acc[half*h:(half+1)*h], n.output[half*h:(half+1)*h]
					if got, want := ngnn1Dot(a, w), ngnn1DotGo(a, w); got != want {
						t.Fatalf("wide sample=%d half=%d got=%d want=%d", sample, half, got, want)
					}
				}
				n.smallOutput = false
				ngnn1AssertKernelEvaluation(t, n, acc)
				for i := range n.output {
					n.output[i] = int16(rng.Intn(257) - 128)
					if sample < 4 {
						acc[i] = 255
						n.output[i] = []int16{-128, 128}[sample%2]
						if sample >= 2 && i >= h {
							n.output[i] = -n.output[i]
						}
					}
				}
				n.smallOutput = true
				want := ngnn1DotGo(acc[:h], n.output[:h]) + ngnn1DotGo(acc[h:], n.output[h:])
				if got := ngnn1OutputSmall(acc[:h], acc[h:], n.output); got != want {
					t.Fatalf("packed sample=%d got=%d want=%d", sample, got, want)
				}
				ngnn1AssertKernelEvaluation(t, n, acc)
			}
		})
	}
}

func ngnn1AssertKernelEvaluation(t *testing.T, n *NGNN1Network, acc []int32) {
	t.Helper()
	h := n.hidden
	for _, turn := range []Color{White, Black} {
		us, them := acc[:h], acc[h:]
		if turn == Black {
			us, them = them, us
		}
		sum := ngnn1DotGo(us, n.output[:h]) + ngnn1DotGo(them, n.output[h:])
		want := (sum/ngnn1QA + int64(n.outputBias)) * ngnn1Scale / (ngnn1QA * ngnn1QB)
		want = min(max(want, -ngnn1ScoreLimit), ngnn1ScoreLimit)
		if got := n.evaluate(acc, turn); int64(got) != want {
			t.Fatalf("evaluation turn=%d got=%d want=%d", turn, got, want)
		}
	}
}

func TestNGNN1OutputDispatchBounds(t *testing.T) {
	for _, w := range []int16{-32768, -129, -128, 128, 129, 32767} {
		data := ngnn1TestBytes(16, 7, false)
		binary.LittleEndian.PutUint16(data[24+(ngnn1Features+1)*16*2:], uint16(w))
		binary.LittleEndian.PutUint32(data[len(data)-4:], crc32.ChecksumIEEE(data[:len(data)-4]))
		n, err := ReadNGNN1(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		if n.smallOutput != (w >= -128 && w <= 128) {
			t.Fatalf("dispatch for weight %d", w)
		}
		acc := make([]int32, 32)
		for i := range acc {
			acc[i] = 255
		}
		ngnn1AssertKernelEvaluation(t, n, acc)
	}
}

func TestNGNN1AccumulatorFullInt16Range(t *testing.T) {
	for _, h := range []int{16, 256, 512, 2048} {
		for _, w := range []int16{-32768, 32767} {
			data := ngnn1TestBytes(h, 13, false)
			for offset := 24; offset < 24+(ngnn1Features+1)*h*2; offset += 2 {
				binary.LittleEndian.PutUint16(data[offset:], uint16(w))
			}
			binary.LittleEndian.PutUint32(data[len(data)-4:], crc32.ChecksumIEEE(data[:len(data)-4]))
			n, err := ReadNGNN1(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			pos := &Position{EnPassant: NoSquare}
			for square := 0; square < 64; square++ {
				pos.Board.UpdateSquare(Square(square), WhitePawn, NoPiece)
			}
			acc := make([]int32, 2*h)
			n.refresh(pos, acc)
			for _, x := range acc {
				if want := int32(w) * 65; x != want {
					t.Fatalf("H=%d weight=%d got=%d want=%d", h, w, x, want)
				}
			}
		}
	}
}
