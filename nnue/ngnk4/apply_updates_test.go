package ngnk4

import (
	"math/rand"
	"testing"
)

var k4ApplyUpdatesSink int16

type k4UpdateKernel func(
	*[HiddenSize]int16,
	*[HiddenSize]int16,
	*[HiddenSize]int16,
	*[HiddenSize]int16,
	*[HiddenSize]int16,
)

func TestK4ApplyUpdatesMatchPortableForAdversarialLanes(t *testing.T) {
	values := [...]int16{
		-32768, -32767, -21846, -16384, -257, -256, -255, -2, -1,
		0, 1, 2, 254, 255, 256, 257, 16383, 21845, 32766, 32767,
	}
	var initial, row0, row1, row2, row3 [HiddenSize]int16
	for lane := 0; lane < HiddenSize; lane++ {
		initial[lane] = values[lane%len(values)]
		row0[lane] = values[(lane*3+1)%len(values)]
		row1[lane] = values[(lane*5+2)%len(values)]
		row2[lane] = values[(lane*7+3)%len(values)]
		row3[lane] = values[(lane*11+4)%len(values)]
	}
	assertK4UpdateKernelsMatchPortable(t, initial, row0, row1, row2, row3)
}

func TestK4ApplyUpdatesMatchPortableForRandomWraparound(t *testing.T) {
	random := rand.New(rand.NewSource(0x5631325570646174))
	for trial := 0; trial < 64; trial++ {
		var initial, row0, row1, row2, row3 [HiddenSize]int16
		arrays := [...]*[HiddenSize]int16{&initial, &row0, &row1, &row2, &row3}
		for _, array := range arrays {
			for lane := range array {
				array[lane] = int16(random.Uint32())
			}
		}
		assertK4UpdateKernelsMatchPortable(t, initial, row0, row1, row2, row3)
	}
}

func TestK4ApplyUpdatesTouchEveryLaneAndPreserveRows(t *testing.T) {
	for _, lane := range []int{0, 1, 15, 16, 127, 128, 255, 256, 511, 512, 766, 767} {
		var destination, add0, subtract1, add2, subtract3 [HiddenSize]int16
		destination[lane] = 32767
		add0[lane] = 1
		subtract1[lane] = -1
		add2[lane] = 32767
		subtract3[lane] = -32768
		beforeRows := [...][HiddenSize]int16{add0, subtract1, add2, subtract3}

		k4ApplyUpdates4(&destination, &add0, &subtract1, &add2, &subtract3)
		want := int16(32767)
		want += int16(1)
		want -= int16(-1)
		want += int16(32767)
		want -= int16(-32768)
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
		if add0 != beforeRows[0] || subtract1 != beforeRows[1] ||
			add2 != beforeRows[2] || subtract3 != beforeRows[3] {
			t.Fatalf("lane %d: update kernel mutated a model row", lane)
		}
	}
}

func TestK4ApplyUpdatesDoNotAllocate(t *testing.T) {
	var destination, row0, row1, row2, row3 [HiddenSize]int16
	if got := testing.AllocsPerRun(100, func() {
		k4ApplyUpdates2(&destination, &row0, &row1)
		k4ApplyUpdates3(&destination, &row0, &row1, &row2)
		k4ApplyUpdates4(&destination, &row0, &row1, &row2, &row3)
	}); got != 0 {
		t.Fatalf("K4 update kernels allocate %.2f objects per run", got)
	}
}

func assertK4UpdateKernelsMatchPortable(
	t *testing.T,
	initial, row0, row1, row2, row3 [HiddenSize]int16,
) {
	t.Helper()
	rowsBefore := [...][HiddenSize]int16{row0, row1, row2, row3}

	got2, want2 := initial, initial
	k4ApplyUpdates2(&got2, &row0, &row1)
	k4ApplyUpdates2Portable(&want2, &row0, &row1)
	if got2 != want2 {
		t.Fatal("two-row update differs from portable oracle")
	}

	got3, want3 := initial, initial
	k4ApplyUpdates3(&got3, &row0, &row1, &row2)
	k4ApplyUpdates3Portable(&want3, &row0, &row1, &row2)
	if got3 != want3 {
		t.Fatal("three-row update differs from portable oracle")
	}

	got4, want4 := initial, initial
	k4ApplyUpdates4(&got4, &row0, &row1, &row2, &row3)
	k4ApplyUpdates4Portable(&want4, &row0, &row1, &row2, &row3)
	if got4 != want4 {
		t.Fatal("four-row update differs from portable oracle")
	}

	if row0 != rowsBefore[0] || row1 != rowsBefore[1] ||
		row2 != rowsBefore[2] || row3 != rowsBefore[3] {
		t.Fatal("update kernel mutated a model row")
	}
}

func BenchmarkK4ApplyUpdates(b *testing.B) {
	var initial, row0, row1, row2, row3 [HiddenSize]int16
	for lane := 0; lane < HiddenSize; lane++ {
		initial[lane] = int16(lane*193 + 32760)
		row0[lane] = int16(lane*257 + 1)
		row1[lane] = int16(lane*509 - 32768)
		row2[lane] = int16(lane*769 + 32767)
		row3[lane] = int16(lane*1021 - 1)
	}
	benchmarks := []struct {
		name     string
		portable k4UpdateKernel
		selected k4UpdateKernel
	}{
		{"as", func(d, a, s, _, _ *[HiddenSize]int16) { k4ApplyUpdates2Portable(d, a, s) }, func(d, a, s, _, _ *[HiddenSize]int16) { k4ApplyUpdates2(d, a, s) }},
		{"ass", func(d, a, s1, s2, _ *[HiddenSize]int16) { k4ApplyUpdates3Portable(d, a, s1, s2) }, func(d, a, s1, s2, _ *[HiddenSize]int16) { k4ApplyUpdates3(d, a, s1, s2) }},
		{"asas", k4ApplyUpdates4Portable, k4ApplyUpdates4},
	}
	for _, role := range []struct {
		name     string
		selected bool
	}{
		{name: "portable"},
		{name: "selected", selected: true},
	} {
		b.Run(role.name, func(b *testing.B) {
			for index := range benchmarks {
				benchmark := &benchmarks[index]
				b.Run(benchmark.name, func(b *testing.B) {
					run := benchmark.portable
					if role.selected {
						run = benchmark.selected
					}
					destination := initial
					b.ReportAllocs()
					for iteration := 0; iteration < b.N; iteration++ {
						run(&destination, &row0, &row1, &row2, &row3)
					}
					k4ApplyUpdatesSink = destination[b.N%HiddenSize]
				})
			}
		})
	}
}
