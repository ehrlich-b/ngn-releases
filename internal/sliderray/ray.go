// Package sliderray supplies NGN's coordinate reference for sliding attacks.
// It is shared by the magic finder and table construction, never a foreign table.
package sliderray

var rookSteps = [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}
var bishopSteps = [4][2]int{{1, 1}, {-1, -1}, {1, -1}, {-1, 1}}

func inside(file, rank int) bool {
	return file >= 0 && file < 8 && rank >= 0 && rank < 8
}

// walk includes the first occupied square and stops immediately after it.
func walk(square int, occupied uint64, steps [4][2]int) uint64 {
	if square < 0 || square >= 64 {
		return 0
	}
	var result uint64
	for _, step := range steps {
		file, rank := square%8+step[0], square/8+step[1]
		for inside(file, rank) {
			bit := uint64(1) << (rank*8 + file)
			result |= bit
			if occupied&bit != 0 {
				break
			}
			file += step[0]
			rank += step[1]
		}
	}
	return result
}

func Rook(square int, occupied uint64) uint64   { return walk(square, occupied, rookSteps) }
func Bishop(square int, occupied uint64) uint64 { return walk(square, occupied, bishopSteps) }

// Mask omits each ray's terminal square: occupying it cannot change the attack
// set. The origin and squares outside the rays are irrelevant too.
func Mask(square int, diagonal bool) uint64 {
	if square < 0 || square >= 64 {
		return 0
	}
	steps := rookSteps
	if diagonal {
		steps = bishopSteps
	}
	var result uint64
	for _, step := range steps {
		file, rank := square%8+step[0], square/8+step[1]
		for inside(file, rank) && inside(file+step[0], rank+step[1]) {
			result |= uint64(1) << (rank*8 + file)
			file += step[0]
			rank += step[1]
		}
	}
	return result
}
