//go:build amd64 && amd64.v3 && !purego && !race

package engine

const ngnn1KernelBackend = "avx2"

// All slices have the validated network width (16..2048, multiple of 16).
// Move destinations may alias their source exactly, or must be disjoint.
// Unaligned loads/stores require no allocator or stack alignment assumptions.

//go:noescape
func ngnn1AddRow(acc []int32, row []int16)

//go:noescape
func ngnn1SubRow(acc []int32, row []int16)

//go:noescape
func ngnn1MoveRows(dst, src []int32, remove, add []int16)

// Update both contiguous perspectives with their respective feature rows.
// dst and src contain 2H int32 values, and each row contains H int16 values.
// The exact-alias or disjoint-source contract also applies to the full frame.
//
//go:noescape
func ngnn1MovePair(dst, src []int32, removeUs, addUs, removeThem, addThem []int16)

//go:noescape
func ngnn1CaptureRows(dst, src []int32, remove, capture, add []int16)

// Update both contiguous H-wide perspectives; source and destination may
// alias exactly or be disjoint. Each feature row has H signed int16 lanes.
//
//go:noescape
func ngnn1CapturePair(dst, src []int32, removeUs, captureUs, addUs, removeThem, captureThem, addThem []int16)

//go:noescape
func ngnn1CastleRows(dst, src []int32, removeKing, removeRook, addKing, addRook []int16)

// ngnn1Dot accepts the entire int16 output weight range. Each individual
// clamped, squared product fits int32: 255^2 * 32768 = 2,130,739,200 < 2^31.
// Sign extension to int64 precedes every addition of products.
//
//go:noescape
func ngnn1Dot(acc []int32, weights []int16) int64

// ngnn1OutputSmall requires all weights within [-128,128], verified once
// by ReadNGNN1. x*w fits int16; madd pair sums fit int32. Each perspective
// accumulates separately in eight int32 lanes: at H=2048 the absolute bound
// per lane is (2048/8)*255^2*128 = 2,130,739,200 < 2^31. Both perspectives
// are widened to int64 before combining or reducing lanes.
//
//go:noescape
func ngnn1OutputSmall(us, them []int32, weights []int16) int64
