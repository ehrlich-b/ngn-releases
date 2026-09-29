package sf18big

import "testing"

func TestSelectedAffine1024MatchesPortable(t *testing.T) {
	testCases := []struct {
		name string
		bias int32
		fill func(int) (uint8, int8)
	}{
		{
			name: "mixed",
			bias: int32(0x76543210),
			fill: func(index int) (uint8, int8) {
				return uint8((index*97 + 31) & 127), int8(uint8(index*73 + 191))
			},
		},
		{
			name: "positive_limit",
			bias: int32(0x7fffffff),
			fill: func(int) (uint8, int8) { return 127, 127 },
		},
		{
			name: "negative_limit",
			bias: -0x7fffffff - 1,
			fill: func(int) (uint8, int8) { return 127, -128 },
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			var input [transformerLanes]uint8
			var weights [transformerLanes]int8
			for index := range input {
				input[index], weights[index] = testCase.fill(index)
			}
			want := affineRowValue1024Portable(&input, &weights[0], testCase.bias)
			got := affineRowValue1024(&input, &weights[0], testCase.bias)
			if got != want {
				t.Fatalf("selected affine value = %d, want %d", got, want)
			}
		})
	}
}

func TestSelectedAffine32MatchesPortable(t *testing.T) {
	var input [32]uint8
	var weights [32 * 32]int8
	var biases [32]int32
	for index := range input {
		input[index] = uint8((index*97 + 31) & 127)
	}
	for index := range weights {
		weights[index] = int8(uint8(index*73 + 191))
	}
	for index := range biases {
		biases[index] = int32(uint32(index)*0x71234569 + 0x6f123457)
	}
	input[0], input[1] = 127, 127
	weights[0], weights[1] = -128, -128
	weights[32], weights[33] = 127, 127

	var want, got [32]int32
	affineLayer32x32Portable(&want, &input, &weights[0], &biases)
	affineLayer32x32(&got, &input, &weights[0], &biases)
	if got != want {
		t.Fatalf("selected 32x32 affine layer = %v, want %v", got, want)
	}

	wantRow := affineRowValue32Portable(&input, &weights[31*32], biases[31])
	gotRow := affineRowValue32(&input, &weights[31*32], biases[31])
	if gotRow != wantRow {
		t.Fatalf("selected 32-lane affine row = %d, want %d", gotRow, wantRow)
	}
}
