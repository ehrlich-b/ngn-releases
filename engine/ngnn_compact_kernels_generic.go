//go:build !amd64 || !amd64.v3 || purego || race

package engine

func ngnn16AddRow(acc, row []int16) {
	for i, x := range row {
		acc[i] += x
	}
}
func ngnn16SubRow(acc, row []int16) {
	for i, x := range row {
		acc[i] -= x
	}
}
func ngnn16MoveRows(dst, src, remove, add []int16) {
	for i := range add {
		dst[i] = src[i] - remove[i] + add[i]
	}
}
func ngnn16MovePair(dst, src, removeUs, addUs, removeThem, addThem []int16) {
	h := len(addUs)
	ngnn16MoveRows(dst[:h], src[:h], removeUs, addUs)
	ngnn16MoveRows(dst[h:], src[h:], removeThem, addThem)
}
func ngnn16CapturePair(dst, src, removeUs, captureUs, addUs, removeThem, captureThem, addThem []int16) {
	h := len(addUs)
	for i := range addUs {
		dst[i] = src[i] - removeUs[i] - captureUs[i] + addUs[i]
		dst[h+i] = src[h+i] - removeThem[i] - captureThem[i] + addThem[i]
	}
}
func ngnn16CastleRows(dst, src, removeKing, removeRook, addKing, addRook []int16) {
	for i := range addKing {
		dst[i] = src[i] - removeKing[i] - removeRook[i] + addKing[i] + addRook[i]
	}
}
func ngnn16OutputSmall(us, them, weights []int16) int64 {
	var sum int64
	for i := range us {
		x, y := int64(min(255, max(0, us[i]))), int64(min(255, max(0, them[i])))
		sum += x*x*int64(weights[i]) + y*y*int64(weights[len(us)+i])
	}
	return sum
}
