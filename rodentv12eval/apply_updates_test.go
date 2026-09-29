package rodentv12eval

import (
	"math/rand"
	"testing"
)

var rodentV12ApplyUpdatesSink int16

type rodentV12UpdateKernel func(
	*[HiddenSize]int16,
	*[HiddenSize]int16,
	*[HiddenSize]int16,
	*[HiddenSize]int16,
	*[HiddenSize]int16,
)

func TestRodentV12ApplyUpdatesMatchPortableForAdversarialLanes(t *testing.T) {
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
	assertRodentV12UpdateKernelsMatchPortable(t, initial, row0, row1, row2, row3)
}

func TestRodentV12ApplyUpdatesMatchPortableForRandomWraparound(t *testing.T) {
	random := rand.New(rand.NewSource(0x5631325570646174))
	for trial := 0; trial < 64; trial++ {
		var initial, row0, row1, row2, row3 [HiddenSize]int16
		arrays := [...]*[HiddenSize]int16{&initial, &row0, &row1, &row2, &row3}
		for _, array := range arrays {
			for lane := range array {
				array[lane] = int16(random.Uint32())
			}
		}
		assertRodentV12UpdateKernelsMatchPortable(t, initial, row0, row1, row2, row3)
	}
}

func TestRodentV12ApplyUpdatesTouchEveryLaneAndPreserveRows(t *testing.T) {
	for _, lane := range []int{0, 1, 15, 16, 127, 128, 255, 256, 511, 512, 766, 767} {
		var destination, add0, subtract1, add2, subtract3 [HiddenSize]int16
		destination[lane] = 32767
		add0[lane] = 1
		subtract1[lane] = -1
		add2[lane] = 32767
		subtract3[lane] = -32768
		beforeRows := [...][HiddenSize]int16{add0, subtract1, add2, subtract3}

		rodentV12ApplyUpdates4(&destination, &add0, &subtract1, &add2, &subtract3)
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

func TestRodentV12ApplyUpdatesDoNotAllocate(t *testing.T) {
	var destination, row0, row1, row2, row3 [HiddenSize]int16
	if got := testing.AllocsPerRun(100, func() {
		rodentV12ApplyUpdates2(&destination, &row0, &row1)
		rodentV12ApplyUpdates3(&destination, &row0, &row1, &row2)
		rodentV12ApplyUpdates4(&destination, &row0, &row1, &row2, &row3)
	}); got != 0 {
		t.Fatalf("Rodent V1.2 update kernels allocate %.2f objects per run", got)
	}
}

func assertRodentV12UpdateKernelsMatchPortable(
	t *testing.T,
	initial, row0, row1, row2, row3 [HiddenSize]int16,
) {
	t.Helper()
	rowsBefore := [...][HiddenSize]int16{row0, row1, row2, row3}

	got2, want2 := initial, initial
	rodentV12ApplyUpdates2(&got2, &row0, &row1)
	rodentV12ApplyUpdates2Portable(&want2, &row0, &row1)
	if got2 != want2 {
		t.Fatal("two-row update differs from portable oracle")
	}

	got3, want3 := initial, initial
	rodentV12ApplyUpdates3(&got3, &row0, &row1, &row2)
	rodentV12ApplyUpdates3Portable(&want3, &row0, &row1, &row2)
	if got3 != want3 {
		t.Fatal("three-row update differs from portable oracle")
	}

	got4, want4 := initial, initial
	rodentV12ApplyUpdates4(&got4, &row0, &row1, &row2, &row3)
	rodentV12ApplyUpdates4Portable(&want4, &row0, &row1, &row2, &row3)
	if got4 != want4 {
		t.Fatal("four-row update differs from portable oracle")
	}

	if row0 != rowsBefore[0] || row1 != rowsBefore[1] ||
		row2 != rowsBefore[2] || row3 != rowsBefore[3] {
		t.Fatal("update kernel mutated a model row")
	}
}

func BenchmarkRodentV12ApplyUpdates(b *testing.B) {
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
		portable rodentV12UpdateKernel
		selected rodentV12UpdateKernel
	}{
		{"as", func(d, a, s, _, _ *[HiddenSize]int16) { rodentV12ApplyUpdates2Portable(d, a, s) }, func(d, a, s, _, _ *[HiddenSize]int16) { rodentV12ApplyUpdates2(d, a, s) }},
		{"ass", func(d, a, s1, s2, _ *[HiddenSize]int16) { rodentV12ApplyUpdates3Portable(d, a, s1, s2) }, func(d, a, s1, s2, _ *[HiddenSize]int16) { rodentV12ApplyUpdates3(d, a, s1, s2) }},
		{"asas", rodentV12ApplyUpdates4Portable, rodentV12ApplyUpdates4},
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
					rodentV12ApplyUpdatesSink = destination[b.N%HiddenSize]
				})
			}
		})
	}
}
