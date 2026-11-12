//go:build amd64 && amd64.v3 && !purego && !race

package engine

// Exact word arithmetic, including modular intermediate overflow. Each row
// has H elements, H is a positive multiple of16. Source and destination
// either alias exactly or are disjoint; accesses may be unaligned.
//
//go:noescape
func ngnn16AddRow(acc, row []int16)

//go:noescape
func ngnn16SubRow(acc, row []int16)

//go:noescape
func ngnn16MoveRows(dst, src, remove, add []int16)

//go:noescape
func ngnn16MovePair(dst, src, removeUs, addUs, removeThem, addThem []int16)

//go:noescape
func ngnn16CapturePair(dst, src, removeUs, captureUs, addUs, removeThem, captureThem, addThem []int16)

//go:noescape
func ngnn16CastleRows(dst, src, removeKing, removeRook, addKing, addRook []int16)

// Same loader-proved output[-128,128] and H<=2048 lane-sum bound as ngnn1OutputSmall.
//
//go:noescape
func ngnn16OutputSmall(us, them, weights []int16) int64
