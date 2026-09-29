//go:build counteroracle

package countereval

import (
	"bytes"
	"encoding/json"
	"math"
	"math/big"
	"os"
	"testing"
)

const (
	float32UnitRoundoff  = 0x1p-24
	float32HalfSubnormal = 0x1p-150
)

type driftUpdate struct {
	Feature     int  `json:"feature"`
	Coefficient int8 `json:"coefficient"`
}

type driftRecord struct {
	ID              string        `json:"id"`
	Sequence        string        `json:"sequence"`
	Operation       string        `json:"operation"`
	Depth           int           `json:"depth"`
	Updates         []driftUpdate `json:"updates"`
	Features        []int         `json:"features"`
	AccumulatorBits []uint32      `json:"accumulator_bits"`
	RawBits         uint32        `json:"raw_white_bits"`
}

type driftOracle struct {
	Schema        string        `json:"schema"`
	CounterCommit string        `json:"counter_commit"`
	ModelSHA256   string        `json:"model_sha256"`
	Records       []driftRecord `json:"records"`
}

type driftFrame struct {
	rootFeatures []int
	updates      []driftUpdate
}

func TestCounter55IncrementalDriftBounds(t *testing.T) {
	modelBytes, err := os.ReadFile(requireTestEnv(t, "COUNTER_MODEL"))
	if err != nil {
		t.Fatal(err)
	}
	model, metadata, err := LoadCounter55Legacy(bytes.NewReader(modelBytes))
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := os.ReadFile(requireTestEnv(t, "COUNTER_TRANSITION_ORACLE_JSON"))
	if err != nil {
		t.Fatal(err)
	}
	var oracle driftOracle
	if err := json.Unmarshal(encoded, &oracle); err != nil {
		t.Fatal(err)
	}
	if oracle.Schema != "counter55-transition-oracle-v1" ||
		oracle.CounterCommit != "63c487ca724c620f71c129d62129c6fb9109c872" ||
		oracle.ModelSHA256 != metadata.SHA256 {
		t.Fatalf("oracle identity mismatch: %+v", oracle)
	}

	frames := make(map[string][]driftFrame)
	nonzeroAccumulatorDrift := 0
	nonzeroRawDrift := 0
	for _, record := range oracle.Records {
		stack := frames[record.Sequence]
		switch record.Operation {
		case "root":
			stack = []driftFrame{{rootFeatures: append([]int(nil), record.Features...)}}
		case "move":
			if len(stack) == 0 {
				t.Fatalf("%s has no root frame", record.ID)
			}
			parent := stack[len(stack)-1]
			next := driftFrame{
				rootFeatures: append([]int(nil), parent.rootFeatures...),
				updates:      append(append([]driftUpdate(nil), parent.updates...), record.Updates...),
			}
			stack = append(stack, next)
		case "null":
			if len(stack) == 0 {
				t.Fatalf("%s has no root frame", record.ID)
			}
			parent := stack[len(stack)-1]
			stack = append(stack, driftFrame{
				rootFeatures: append([]int(nil), parent.rootFeatures...),
				updates:      append([]driftUpdate(nil), parent.updates...),
			})
		case "pop":
			if len(stack) <= 1 {
				t.Fatalf("%s pops root", record.ID)
			}
			stack = stack[:len(stack)-1]
		default:
			t.Fatalf("%s operation=%q", record.ID, record.Operation)
		}
		if len(stack)-1 != record.Depth {
			t.Fatalf("%s reconstructed depth=%d want=%d", record.ID, len(stack)-1, record.Depth)
		}
		frames[record.Sequence] = stack
		frame := stack[len(stack)-1]

		exactIncremental, incBounds := exactAccumulatorAndBounds(t, model, frame.rootFeatures, frame.updates)
		exactFresh, freshBounds := exactAccumulatorAndBounds(t, model, record.Features, nil)
		if len(record.AccumulatorBits) != HiddenSize {
			t.Fatalf("%s lanes=%d", record.ID, len(record.AccumulatorBits))
		}
		_, freshAccumulator, freshRaw, err := model.evaluateFullRefreshTrace(boardFromFeatures(t, record.Features))
		if err != nil {
			t.Fatalf("%s fresh: %v", record.ID, err)
		}
		for lane := 0; lane < HiddenSize; lane++ {
			if exactIncremental[lane].Cmp(exactFresh[lane]) != 0 {
				t.Fatalf("%s lane %d exact incremental and fresh states differ", record.ID, lane)
			}
			incremental := math.Float32frombits(record.AccumulatorBits[lane])
			incError := rationalDifference(incremental, exactIncremental[lane])
			freshError := rationalDifference(freshAccumulator[lane], exactFresh[lane])
			if incError > incBounds[lane] {
				t.Fatalf("%s lane %d incremental error %g > %g", record.ID, lane, incError, incBounds[lane])
			}
			if freshError > freshBounds[lane] {
				t.Fatalf("%s lane %d fresh error %g > %g", record.ID, lane, freshError, freshBounds[lane])
			}
			drift := math.Abs(float64(incremental) - float64(freshAccumulator[lane]))
			if drift > outwardAdd(incBounds[lane], freshBounds[lane]) {
				t.Fatalf("%s lane %d drift %g exceeds joined bound", record.ID, lane, drift)
			}
			if math.Float32bits(incremental) != math.Float32bits(freshAccumulator[lane]) {
				nonzeroAccumulatorDrift++
			}
		}

		incrementalRaw := math.Float32frombits(record.RawBits)
		exactRaw := exactNetworkOutput(model, exactIncremental)
		incRawBound := rawErrorBound(model, exactIncremental, incBounds)
		freshRawBound := rawErrorBound(model, exactFresh, freshBounds)
		if err := rationalDifference(incrementalRaw, exactRaw); err > incRawBound {
			t.Fatalf("%s incremental raw error %g > %g", record.ID, err, incRawBound)
		}
		if err := rationalDifference(freshRaw, exactRaw); err > freshRawBound {
			t.Fatalf("%s fresh raw error %g > %g", record.ID, err, freshRawBound)
		}
		if drift := math.Abs(float64(incrementalRaw) - float64(freshRaw)); drift > outwardAdd(incRawBound, freshRawBound) {
			t.Fatalf("%s raw drift %g exceeds joined bound", record.ID, drift)
		}
		if math.Float32bits(incrementalRaw) != math.Float32bits(freshRaw) {
			nonzeroRawDrift++
		}
	}
	if nonzeroAccumulatorDrift == 0 {
		t.Fatal("fixtures did not exercise path-order accumulator drift")
	}
	t.Logf("bounded path-order drift: accumulator_bit_differences=%d raw_bit_differences=%d", nonzeroAccumulatorDrift, nonzeroRawDrift)
}

func exactAccumulatorAndBounds(
	t *testing.T,
	model *Model,
	rootFeatures []int,
	updates []driftUpdate,
) ([HiddenSize]*big.Rat, [HiddenSize]float64) {
	t.Helper()
	var exact [HiddenSize]*big.Rat
	var bounds [HiddenSize]float64
	operationCount := len(rootFeatures) + len(updates)
	for lane := 0; lane < HiddenSize; lane++ {
		sum := exactFloat32(model.hiddenBiases[lane])
		absoluteSum := math.Abs(float64(model.hiddenBiases[lane]))
		for _, feature := range rootFeatures {
			if feature < 0 || feature >= InputSize {
				t.Fatalf("root feature %d out of range", feature)
			}
			value := model.hiddenWeights[feature*HiddenSize+lane]
			sum.Add(sum, exactFloat32(value))
			absoluteSum = outwardAdd(absoluteSum, math.Abs(float64(value)))
		}
		for _, update := range updates {
			if update.Feature < 0 || update.Feature >= InputSize ||
				(update.Coefficient != -1 && update.Coefficient != 1) {
				t.Fatalf("bad update %+v", update)
			}
			value := model.hiddenWeights[update.Feature*HiddenSize+lane]
			term := exactFloat32(value)
			if update.Coefficient < 0 {
				term.Neg(term)
			}
			sum.Add(sum, term)
			absoluteSum = outwardAdd(absoluteSum, math.Abs(float64(value)))
		}
		exact[lane] = sum
		bounds[lane] = additionErrorBound(operationCount, absoluteSum)
	}
	return exact, bounds
}

func additionErrorBound(operations int, absoluteSum float64) float64 {
	if operations == 0 {
		return 0
	}
	n := float64(operations)
	nu := n * float32UnitRoundoff
	reciprocal := math.Nextafter(1/(1-nu), math.Inf(1))
	gamma := math.Nextafter(nu*reciprocal, math.Inf(1))
	relative := outwardMul(gamma, absoluteSum)
	absolute := outwardMul(outwardMul(n, float32HalfSubnormal), reciprocal)
	return outwardAdd(relative, absolute)
}

func exactNetworkOutput(model *Model, accumulator [HiddenSize]*big.Rat) *big.Rat {
	result := exactFloat32(model.outputBias)
	for lane, value := range accumulator {
		if value.Sign() <= 0 {
			continue
		}
		product := new(big.Rat).Mul(new(big.Rat).Set(value), exactFloat32(model.outputWeights[lane]))
		result.Add(result, product)
	}
	return result
}

func rawErrorBound(model *Model, exactAccumulator [HiddenSize]*big.Rat, accumulatorBounds [HiddenSize]float64) float64 {
	inputError := 0.0
	absoluteTerms := math.Abs(float64(model.outputBias))
	for lane, exact := range exactAccumulator {
		weight := math.Abs(float64(model.outputWeights[lane]))
		inputError = outwardAdd(inputError, outwardMul(weight, accumulatorBounds[lane]))
		exactAbs := rationalAbsFloat64Upper(exact)
		absoluteTerms = outwardAdd(absoluteTerms,
			outwardMul(weight, outwardAdd(exactAbs, accumulatorBounds[lane])))
	}
	reduction := additionErrorBound(2*HiddenSize+1, absoluteTerms)
	return outwardAdd(inputError, reduction)
}

func exactFloat32(value float32) *big.Rat {
	result := new(big.Rat)
	if result.SetFloat64(float64(value)) == nil {
		panic("nonfinite float32 in exact oracle")
	}
	return result
}

func rationalAbsFloat64Upper(value *big.Rat) float64 {
	absolute := new(big.Rat).Abs(new(big.Rat).Set(value))
	if absolute.Sign() == 0 {
		return 0
	}
	result, _ := absolute.Float64()
	if exactFloat64 := new(big.Rat).SetFloat64(result); exactFloat64.Cmp(absolute) < 0 {
		result = math.Nextafter(result, math.Inf(1))
	}
	return result
}

func rationalDifference(observed float32, exact *big.Rat) float64 {
	difference := new(big.Rat).Sub(exactFloat32(observed), exact)
	difference.Abs(difference)
	if difference.Sign() == 0 {
		return 0
	}
	value, _ := difference.Float64()
	return math.Nextafter(value, math.Inf(1))
}

func outwardAdd(a, b float64) float64 {
	return math.Nextafter(a+b, math.Inf(1))
}

func outwardMul(a, b float64) float64 {
	return math.Nextafter(a*b, math.Inf(1))
}
