package ngnk4

import (
	"math/rand"
	"strconv"
	"testing"
)

var k4RefreshSink int16

func TestK4RefreshPerspectiveMatchesPortableForAdversarialLanes(t *testing.T) {
	values := [...]int16{
		-32768, -32767, -21846, -16384, -257, -256, -255, -2, -1,
		0, 1, 2, 254, 255, 256, 257, 16383, 21845, 32766, 32767,
	}
	var biases [HiddenSize]int16
	rowStorage := make([][HiddenSize]int16, k4MaximumRefreshRows)
	var rows [k4MaximumRefreshRows]*[HiddenSize]int16
	for lane := 0; lane < HiddenSize; lane++ {
		biases[lane] = values[lane%len(values)]
		for row := range rowStorage {
			rowStorage[row][lane] = values[(lane*(row*2+3)+row+1)%len(values)]
		}
	}
	for row := range rowStorage {
		rows[row] = &rowStorage[row]
	}

	for _, rowCount := range []int{0, 1, 2, 16, 32, 64} {
		assertK4RefreshMatchesPortable(t, biases, &rows, rowCount)
	}
}

func TestK4RefreshPerspectiveMatchesPortableForRandomWraparound(t *testing.T) {
	random := rand.New(rand.NewSource(0x5631325265667265))
	var biases [HiddenSize]int16
	rowStorage := make([][HiddenSize]int16, k4MaximumRefreshRows)
	var rows [k4MaximumRefreshRows]*[HiddenSize]int16
	for trial := 0; trial < 32; trial++ {
		for lane := range biases {
			biases[lane] = int16(random.Uint32())
		}
		for row := range rowStorage {
			rows[row] = &rowStorage[row]
			for lane := range rowStorage[row] {
				rowStorage[row][lane] = int16(random.Uint32())
			}
		}
		rowCount := random.Intn(k4MaximumRefreshRows + 1)
		assertK4RefreshMatchesPortable(t, biases, &rows, rowCount)
	}
}

func TestK4RefreshPerspectiveTouchesEveryLaneAndPreservesInputs(t *testing.T) {
	for _, lane := range []int{0, 1, 15, 16, 127, 128, 255, 256, 511, 512, 766, 767} {
		var biases, row0, row1 [HiddenSize]int16
		biases[lane] = 32767
		row0[lane] = 1
		row1[lane] = -32768
		biasesBefore, row0Before, row1Before := biases, row0, row1
		rows := [k4MaximumRefreshRows]*[HiddenSize]int16{&row0, &row1}
		var destination [HiddenSize]int16

		k4RefreshPerspective(&destination, &biases, &rows, 2)
		want := int16(32767)
		want += int16(1)
		want += int16(-32768)
		for gotLane, got := range destination {
			if gotLane == lane {
				if got != want {
					t.Fatalf("lane %d: got %d, want %d", lane, got, want)
				}
				continue
			}
			if got != 0 {
				t.Fatalf("lane %d changed to %d while targeting lane %d", gotLane, got, lane)
			}
		}
		if biases != biasesBefore || row0 != row0Before || row1 != row1Before {
			t.Fatalf("lane %d: refresh kernel mutated an input", lane)
		}
	}
}

func TestK4RefreshPerspectiveDoesNotAllocate(t *testing.T) {
	var destination, biases, row0, row1 [HiddenSize]int16
	rows := [k4MaximumRefreshRows]*[HiddenSize]int16{&row0, &row1}
	if got := testing.AllocsPerRun(100, func() {
		k4RefreshPerspective(&destination, &biases, &rows, 2)
	}); got != 0 {
		t.Fatalf("K4 refresh kernel allocates %.2f objects per run", got)
	}
}

func TestK4ModelRefreshPerspectiveDoesNotAllocate(t *testing.T) {
	model := new(Model)
	board := Board{
		WhitePawn: 0x000000000000ff00,
		WhiteKing: 0x0000000000000010,
		BlackPawn: 0x00ff000000000000,
		BlackKing: 0x1000000000000000,
	}
	var destination [HiddenSize]int16
	if got := testing.AllocsPerRun(100, func() {
		model.refreshPerspective(&destination, board, White, 4)
	}); got != 0 {
		t.Fatalf("K4 model refresh allocates %.2f objects per run", got)
	}
}

func TestK4ModelRefreshPerspectiveAcceptsFullBoard(t *testing.T) {
	model := new(Model)
	for hidden := range model.inputBiases {
		model.inputBiases[hidden] = int16(hidden*193 + 32760)
	}
	for feature := range model.inputWeights {
		for hidden := range model.inputWeights[feature] {
			model.inputWeights[feature][hidden] = int16(feature*257 + hidden*509 + 1)
		}
	}

	var board Board
	board[WhiteKing] = uint64(1) << 4
	board[BlackKing] = uint64(1) << 60
	nonKingPlanes := [...]int{
		WhitePawn, WhiteKnight, WhiteBishop, WhiteRook, WhiteQueen,
		BlackPawn, BlackKnight, BlackBishop, BlackRook, BlackQueen,
	}
	index := 0
	for square := 0; square < 64; square++ {
		if square == 4 || square == 60 {
			continue
		}
		board[nonKingPlanes[index%len(nonKingPlanes)]] |= uint64(1) << square
		index++
	}

	want := model.fullRefresh(board)
	for perspective, kingSquare := range [...]int{4, 60} {
		var got [HiddenSize]int16
		model.refreshPerspective(&got, board, Color(perspective), kingSquare)
		if got != want[perspective] {
			t.Fatalf("perspective %d full-board refresh differs from scalar full refresh", perspective)
		}
	}
}

func assertK4RefreshMatchesPortable(
	t *testing.T,
	biases [HiddenSize]int16,
	rows *[k4MaximumRefreshRows]*[HiddenSize]int16,
	rowCount int,
) {
	t.Helper()
	rowsBefore := make([][HiddenSize]int16, rowCount)
	for row := 0; row < rowCount; row++ {
		rowsBefore[row] = *rows[row]
	}
	biasesBefore := biases
	var got, want [HiddenSize]int16
	k4RefreshPerspective(&got, &biases, rows, rowCount)
	k4RefreshPerspectivePortable(&want, &biases, rows, rowCount)
	if got != want {
		t.Fatalf("%d-row refresh differs from portable oracle", rowCount)
	}
	if biases != biasesBefore {
		t.Fatalf("%d-row refresh mutated biases", rowCount)
	}
	for row := 0; row < rowCount; row++ {
		if *rows[row] != rowsBefore[row] {
			t.Fatalf("%d-row refresh mutated row %d", rowCount, row)
		}
	}
}

func BenchmarkK4RefreshPerspective(b *testing.B) {
	var biases [HiddenSize]int16
	rowStorage := make([][HiddenSize]int16, k4MaximumRefreshRows)
	var rows [k4MaximumRefreshRows]*[HiddenSize]int16
	for lane := 0; lane < HiddenSize; lane++ {
		biases[lane] = int16(lane*193 + 32760)
		for row := range rowStorage {
			rowStorage[row][lane] = int16(lane*(row*2+257) + row + 1)
		}
	}
	for row := range rowStorage {
		rows[row] = &rowStorage[row]
	}
	for _, rowCount := range []int{2, 16, 32} {
		b.Run("portable/rows="+strconv.Itoa(rowCount), func(b *testing.B) {
			var destination [HiddenSize]int16
			b.ReportAllocs()
			for iteration := 0; iteration < b.N; iteration++ {
				k4RefreshPerspectivePortable(&destination, &biases, &rows, rowCount)
			}
			k4RefreshSink = destination[b.N%HiddenSize]
		})
		b.Run("selected/rows="+strconv.Itoa(rowCount), func(b *testing.B) {
			var destination [HiddenSize]int16
			b.ReportAllocs()
			for iteration := 0; iteration < b.N; iteration++ {
				k4RefreshPerspective(&destination, &biases, &rows, rowCount)
			}
			k4RefreshSink = destination[b.N%HiddenSize]
		})
	}
}
