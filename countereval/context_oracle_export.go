//go:build counteroracle

package countereval

import "math"

// OracleAccumulatorBits returns a copy for the build-tagged compatibility oracle.
func (context *Context) OracleAccumulatorBits() ([HiddenSize]uint32, error) {
	if err := context.validate(); err != nil {
		return [HiddenSize]uint32{}, err
	}
	var result [HiddenSize]uint32
	for lane, value := range context.accumulators[context.depth] {
		result[lane] = math.Float32bits(value)
	}
	return result, nil
}
