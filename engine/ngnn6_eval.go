package engine

// ngnn6ResidualCP consumes STM-ordered accumulators from either storage path.
// The linear branch keeps its existing kernels and divisions. This head uses
// unsquared clipped inputs and int64 throughout, with no zero-projection bypass.
func ngnn6ResidualCP[T int16 | int32](n *NGNN6Network, us, them []T, bucket int) int64 {
	var x [2 * 768]int64
	h := len(us)
	for i := 0; i < h; i++ {
		x[i] = min(int64(255), max(int64(0), int64(us[i])))
		x[h+i] = min(int64(255), max(int64(0), int64(them[i])))
	}
	projection := int64(n.outputBias[bucket])
	for j, bias := range n.hiddenBias {
		v := int64(bias)
		for i, w := range n.hiddenWeight[j*2*h : (j+1)*2*h] {
			v += x[i] * int64(w)
		}
		z := min(int64(255), max(int64(0), v/ngnn6InputScale))
		projection += z * int64(n.output[bucket*ngnn6Units+j])
	}
	return projection * ngnn1Scale / (ngnn1QA * ngnn1QB)
}

// evaluateBucket is the scalar format reference used by format diagnostics.
func (n *NGNN6Network) evaluateBucket(acc []int32, turn Color, bucket int) int {
	h := n.linear.hidden
	us, them := acc[:h], acc[h:2*h]
	if turn == Black {
		us, them = them, us
	}
	var x [2 * 768]int64
	for i := 0; i < h; i++ {
		x[i] = int64(min(ngnn1QA, max(0, us[i])))
		x[h+i] = int64(min(ngnn1QA, max(0, them[i])))
	}
	// Preserve the original squared-clipped linear branch, including both
	// divisions and the bias placement. Do not add to its already-clamped cp.
	var sum int64
	for i, w := range n.linear.output[bucket*2*h : (bucket+1)*2*h] {
		sum += x[i] * x[i] * int64(w)
	}
	bias := n.linear.outputBias
	if bucket > 0 {
		bias = n.linear.otherBiases[bucket-1]
	}
	linearCP := (sum/ngnn1QA + int64(bias)) * ngnn1Scale / (ngnn1QA * ngnn1QB)
	projection := int64(n.outputBias[bucket])
	for j, bias := range n.hiddenBias {
		v := int64(bias)
		for i, w := range n.hiddenWeight[j*2*h : (j+1)*2*h] {
			v += x[i] * int64(w)
		}
		z := min(int64(ngnn1QA), max(int64(0), v/ngnn6InputScale))
		projection += z * int64(n.output[bucket*ngnn6Units+j])
	}
	residualCP := projection * ngnn1Scale / (ngnn1QA * ngnn1QB)
	cp := linearCP + residualCP
	return int(min(int64(ngnn1ScoreLimit), max(int64(-ngnn1ScoreLimit), cp)))
}
