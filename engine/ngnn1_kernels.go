package engine

// The Go kernels remain the portable implementation and the differential
// reference. Validated widths are multiples of 16. Move kernels fuse the parent
// copy with row arithmetic; dst may equal src, or they must be disjoint.
func ngnn1AddRowGo(acc []int32, row []int16) {
	for offset := 0; offset < len(row); offset += 16 {
		a, r := (*[16]int32)(acc[offset:]), (*[16]int16)(row[offset:])
		for i := range a {
			a[i] += int32(r[i])
		}
	}
}

func ngnn1SubRowGo(acc []int32, row []int16) {
	for offset := 0; offset < len(row); offset += 16 {
		a, r := (*[16]int32)(acc[offset:]), (*[16]int16)(row[offset:])
		for i := range a {
			a[i] -= int32(r[i])
		}
	}
}

// The usual move shapes copy and update each child lane in one traversal.
func ngnn1MoveRowsGo(acc, src []int32, remove, add []int16) {
	if len(add) == 256 {
		a, parent := (*[256]int32)(acc), (*[256]int32)(src)
		r, s := (*[256]int16)(remove), (*[256]int16)(add)
		for i := range a {
			a[i] = parent[i] + int32(s[i]) - int32(r[i])
		}
		return
	}
	for offset := 0; offset < len(add); offset += 16 {
		a, parent := (*[16]int32)(acc[offset:]), (*[16]int32)(src[offset:])
		r, s := (*[16]int16)(remove[offset:]), (*[16]int16)(add[offset:])
		for i := range a {
			a[i] = parent[i] + int32(s[i]) - int32(r[i])
		}
	}
}

func ngnn1CaptureRowsGo(acc, src []int32, remove, capture, add []int16) {
	if len(add) == 256 {
		a, parent := (*[256]int32)(acc), (*[256]int32)(src)
		r, c, s := (*[256]int16)(remove), (*[256]int16)(capture), (*[256]int16)(add)
		for i := range a {
			a[i] = parent[i] + int32(s[i]) - int32(r[i]) - int32(c[i])
		}
		return
	}
	for offset := 0; offset < len(add); offset += 16 {
		a, parent := (*[16]int32)(acc[offset:]), (*[16]int32)(src[offset:])
		r, c, s := (*[16]int16)(remove[offset:]), (*[16]int16)(capture[offset:]), (*[16]int16)(add[offset:])
		for i := range a {
			a[i] = parent[i] + int32(s[i]) - int32(r[i]) - int32(c[i])
		}
	}
}

func ngnn1CastleRowsGo(acc, src []int32, removeKing, removeRook, addKing, addRook []int16) {
	if len(addKing) == 256 {
		a, parent := (*[256]int32)(acc), (*[256]int32)(src)
		rk, rr := (*[256]int16)(removeKing), (*[256]int16)(removeRook)
		sk, sr := (*[256]int16)(addKing), (*[256]int16)(addRook)
		for i := range a {
			a[i] = parent[i] + int32(sk[i]) + int32(sr[i]) - int32(rk[i]) - int32(rr[i])
		}
		return
	}
	for offset := 0; offset < len(addKing); offset += 16 {
		a, parent := (*[16]int32)(acc[offset:]), (*[16]int32)(src[offset:])
		rk, rr := (*[16]int16)(removeKing[offset:]), (*[16]int16)(removeRook[offset:])
		sk, sr := (*[16]int16)(addKing[offset:]), (*[16]int16)(addRook[offset:])
		for i := range a {
			a[i] = parent[i] + int32(sk[i]) + int32(sr[i]) - int32(rk[i]) - int32(rr[i])
		}
	}
}

func ngnn1DotGo(acc []int32, weights []int16) int64 {
	var sum int64
	if len(weights) == 256 {
		a, w := (*[256]int32)(acc), (*[256]int16)(weights)
		for i := range a {
			x := min(max(a[i], 0), ngnn1QA)
			sum += int64(x*x) * int64(w[i])
		}
		return sum
	}
	for offset := 0; offset < len(weights); offset += 16 {
		a, w := (*[16]int32)(acc[offset:]), (*[16]int16)(weights[offset:])
		for i := range a {
			x := min(max(a[i], 0), ngnn1QA)
			sum += int64(x*x) * int64(w[i])
		}
	}
	return sum
}
