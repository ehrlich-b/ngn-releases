//go:build !amd64 || !amd64.v3 || purego || race

package engine

// Race builds use instrumented Go memory accesses.
const ngnn1KernelBackend = "go"

func ngnn1AddRow(acc []int32, row []int16) { ngnn1AddRowGo(acc, row) }
func ngnn1SubRow(acc []int32, row []int16) { ngnn1SubRowGo(acc, row) }
func ngnn1MoveRows(dst, src []int32, remove, add []int16) {
	ngnn1MoveRowsGo(dst, src, remove, add)
}
func ngnn1CaptureRows(dst, src []int32, remove, capture, add []int16) {
	ngnn1CaptureRowsGo(dst, src, remove, capture, add)
}
func ngnn1CastleRows(dst, src []int32, removeKing, removeRook, addKing, addRook []int16) {
	ngnn1CastleRowsGo(dst, src, removeKing, removeRook, addKing, addRook)
}
func ngnn1Dot(acc []int32, weights []int16) int64 { return ngnn1DotGo(acc, weights) }
func ngnn1OutputSmall(us, them []int32, weights []int16) int64 {
	h := len(us)
	return ngnn1DotGo(us, weights[:h]) + ngnn1DotGo(them, weights[h:])
}
