package sf18big

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"unsafe"

	base "github.com/ehrlich-b/ngn/nnue"
)

var (
	benchmarkThreatListSink   ThreatIndexList
	benchmarkStateSink        ReferenceState
	benchmarkTransformedSink  [transformerLanes]uint8
	benchmarkInt32Sink        int32
	benchmarkSelectedSink     SelectedEvaluation
	benchmarkContextStatsSink ContextStats
)

type n1bBenchmarkFixture struct {
	model        *Model
	positions    []base.Position
	states       []ReferenceState
	buckets      []int
	transformed  [][transformerLanes]uint8
	features     []uint32
	baseFeatures []int
	transitions  []threatListPair
}

type threatListPair struct {
	before ThreatIndexList
	after  ThreatIndexList
}

type n1cContextStep struct {
	delta  base.Delta
	isNull bool
}

type n1cContextBlock struct {
	context   *Context
	steps     []n1cContextStep
	positions []base.Position
}

func loadN1bBenchmarkFixture(b *testing.B) n1bBenchmarkFixture {
	b.Helper()
	networkPath := os.Getenv(officialFileEnvironment)
	oraclePath := os.Getenv(referenceGameOracleEnvironment)
	if networkPath == "" || oraclePath == "" {
		b.Skip("official BIG network and representative oracle are required")
	}
	networkFile, err := os.Open(networkPath)
	if err != nil {
		b.Fatal(err)
	}
	model, err := Load(networkFile)
	networkFile.Close()
	if err != nil {
		b.Fatal(err)
	}
	oracleFile, err := os.Open(oraclePath)
	if err != nil {
		b.Fatal(err)
	}
	defer oracleFile.Close()
	var fixture n1bBenchmarkFixture
	fixture.model = model
	previousCase := ""
	var previousState ReferenceState
	scanner := bufio.NewScanner(oracleFile)
	scanner.Buffer(make([]byte, 64<<10), 2<<20)
	for scanner.Scan() {
		var row referenceOracleRow
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			b.Fatal(err)
		}
		position := base.Position{SideToMove: row.State.SideToMove, Pieces: row.State.Pieces}
		state, occupied, err := model.ReferenceRefresh(position)
		if err != nil {
			b.Fatal(err)
		}
		bucket := (occupied - 1) / 4
		transformed, _ := transformReferenceState(&state, position.SideToMove, bucket)
		fixture.positions = append(fixture.positions, position)
		fixture.states = append(fixture.states, state)
		fixture.buckets = append(fixture.buckets, bucket)
		fixture.transformed = append(fixture.transformed, transformed)
		board, kings, _, _, err := validatePosition(position)
		if err != nil {
			b.Fatal(err)
		}
		for perspective := base.White; perspective <= base.Black; perspective++ {
			for square, piece := range board {
				if piece.set {
					fixture.baseFeatures = append(
						fixture.baseFeatures,
						baseFeatureIndex(piece, base.Square(square), kings[perspective], perspective),
					)
				}
			}
		}
		if row.Case == previousCase {
			for perspective := base.White; perspective <= base.Black; perspective++ {
				fixture.transitions = append(fixture.transitions, threatListPair{
					before: previousState.Threats[perspective],
					after:  state.Threats[perspective],
				})
			}
		}
		for perspective := base.White; perspective <= base.Black; perspective++ {
			fixture.features = append(
				fixture.features,
				state.Threats[perspective].Indices[:state.Threats[perspective].Count]...,
			)
		}
		previousCase = row.Case
		previousState = state
	}
	if err := scanner.Err(); err != nil {
		b.Fatal(err)
	}
	if len(fixture.positions) == 0 || len(fixture.features) == 0 ||
		len(fixture.baseFeatures) == 0 || len(fixture.transitions) == 0 {
		b.Fatal("empty representative benchmark fixture")
	}
	return fixture
}

func BenchmarkN1bReference(b *testing.B) {
	fixture := loadN1bBenchmarkFixture(b)

	b.Run("threat-enumeration", func(b *testing.B) {
		b.ReportAllocs()
		var result ThreatIndexList
		for i := 0; i < b.N; i++ {
			position := fixture.positions[i%len(fixture.positions)]
			var err error
			result, err = ActiveThreats(position, base.Color(i&1))
			if err != nil {
				b.Fatal(err)
			}
		}
		benchmarkThreatListSink = result
	})

	b.Run("sorted-threat-diff", func(b *testing.B) {
		b.ReportAllocs()
		var removed, added ThreatIndexList
		for i := 0; i < b.N; i++ {
			transition := fixture.transitions[i%len(fixture.transitions)]
			removed, added = diffThreatIndexLists(transition.before, transition.after)
		}
		benchmarkThreatListSink = removed
		benchmarkThreatListSink = added
	})

	b.Run("threat-row-update-1024", func(b *testing.B) {
		b.ReportAllocs()
		var values [transformerLanes]int16
		var psqt [psqtBuckets]int32
		for i := 0; i < b.N; i++ {
			fixture.model.updateThreatFeature(
				&values,
				&psqt,
				fixture.features[i%len(fixture.features)],
				i&1 == 0,
			)
		}
		benchmarkInt32Sink = int32(values[0]) + psqt[0]
	})

	b.Run("base-row-update-1024", func(b *testing.B) {
		b.ReportAllocs()
		var values [transformerLanes]int16
		var psqt [psqtBuckets]int32
		for i := 0; i < b.N; i++ {
			fixture.model.updateBaseFeature(
				&values,
				&psqt,
				fixture.baseFeatures[i%len(fixture.baseFeatures)],
				i&1 == 0,
			)
		}
		benchmarkInt32Sink = int32(values[0]) + psqt[0]
	})

	b.Run("transform-1024", func(b *testing.B) {
		b.ReportAllocs()
		var transformed [transformerLanes]uint8
		for i := 0; i < b.N; i++ {
			index := i % len(fixture.states)
			transformed, benchmarkInt32Sink = transformReferenceState(
				&fixture.states[index],
				fixture.positions[index].SideToMove,
				fixture.buckets[index],
			)
		}
		benchmarkTransformedSink = transformed
	})

	b.Run("selected-head-1024", func(b *testing.B) {
		b.ReportAllocs()
		var result int32
		for i := 0; i < b.N; i++ {
			index := i % len(fixture.transformed)
			result = fixture.model.propagateSelectedStack(
				fixture.buckets[index],
				&fixture.transformed[index],
			)
		}
		benchmarkInt32Sink = result
	})

	b.Run("full-refresh", func(b *testing.B) {
		b.ReportAllocs()
		var state ReferenceState
		for i := 0; i < b.N; i++ {
			var err error
			state, _, err = fixture.model.ReferenceRefresh(fixture.positions[i%len(fixture.positions)])
			if err != nil {
				b.Fatal(err)
			}
		}
		benchmarkStateSink = state
	})

	b.Run("selected-full-refresh", func(b *testing.B) {
		b.ReportAllocs()
		var result SelectedEvaluation
		for i := 0; i < b.N; i++ {
			var err error
			result, err = fixture.model.EvaluateSelected(fixture.positions[i%len(fixture.positions)])
			if err != nil {
				b.Fatal(err)
			}
		}
		benchmarkSelectedSink = result
	})
}

func BenchmarkN1cDemandContext(b *testing.B) {
	fixture := loadN1bBenchmarkFixture(b)
	blocks := loadN1cContextBlocks(b, fixture.model)

	b.Run("push-pop-eight-dirty", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			block := &blocks[i%len(blocks)]
			pushN1cBlock(b, block, false)
			popN1cBlock(b, block)
		}
		b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N*8), "ns/transition")
		b.ReportMetric(float64(unsafe.Sizeof(contextFrame{})), "frame-bytes")
		b.ReportMetric(float64(unsafe.Sizeof(Context{})), "context-bytes")
		benchmarkContextStatsSink, _ = blocks[0].context.Stats()
	})

	b.Run("evaluate-every-ply-eight", func(b *testing.B) {
		b.ReportAllocs()
		var result SelectedEvaluation
		for i := 0; i < b.N; i++ {
			block := &blocks[i%len(blocks)]
			for _, step := range block.steps {
				pushN1cStep(b, block.context, step)
				var err error
				result, err = block.context.EvaluateSelected()
				if err != nil {
					b.Fatal(err)
				}
			}
			popN1cBlock(b, block)
		}
		b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N*8), "ns/evaluated-node")
		benchmarkSelectedSink = result
	})

	for _, dirtyPlies := range []int{2, 3, 4, 8} {
		b.Run(fmt.Sprintf("evaluate-after-%d-dirty", dirtyPlies), func(b *testing.B) {
			b.ReportAllocs()
			var result SelectedEvaluation
			for i := 0; i < b.N; i++ {
				block := &blocks[i%len(blocks)]
				for _, step := range block.steps[:dirtyPlies] {
					pushN1cStep(b, block.context, step)
				}
				var err error
				result, err = block.context.EvaluateSelected()
				if err != nil {
					b.Fatal(err)
				}
				popN1cSteps(b, block.context, dirtyPlies)
			}
			b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N*dirtyPlies), "ns/amortized-transition")
			benchmarkSelectedSink = result
		})
	}

	b.Run("selected-full-refresh-block-destination", func(b *testing.B) {
		b.ReportAllocs()
		var result SelectedEvaluation
		for i := 0; i < b.N; i++ {
			block := &blocks[i%len(blocks)]
			var err error
			result, err = fixture.model.EvaluateSelected(block.positions[len(block.positions)-1])
			if err != nil {
				b.Fatal(err)
			}
		}
		benchmarkSelectedSink = result
	})

	b.Run("repeat-computed-output", func(b *testing.B) {
		b.ReportAllocs()
		context := blocks[0].context
		var result SelectedEvaluation
		for i := 0; i < b.N; i++ {
			var err error
			result, err = context.EvaluateSelected()
			if err != nil {
				b.Fatal(err)
			}
		}
		benchmarkSelectedSink = result
	})
}

func loadN1cContextBlocks(b *testing.B, model *Model) []n1cContextBlock {
	b.Helper()
	oraclePath := os.Getenv(referenceGameOracleEnvironment)
	if oraclePath == "" {
		b.Skip("representative oracle is required")
	}
	oracleFile, err := os.Open(oraclePath)
	if err != nil {
		b.Fatal(err)
	}
	defer oracleFile.Close()

	type contextRow struct {
		position base.Position
		action   string
		isNull   bool
	}
	var cases [][]contextRow
	var currentCase string
	scanner := bufio.NewScanner(oracleFile)
	scanner.Buffer(make([]byte, 64<<10), 2<<20)
	for scanner.Scan() {
		var row referenceOracleRow
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			b.Fatal(err)
		}
		if row.Case != currentCase {
			cases = append(cases, nil)
			currentCase = row.Case
		}
		cases[len(cases)-1] = append(cases[len(cases)-1], contextRow{
			position: base.Position{SideToMove: row.State.SideToMove, Pieces: row.State.Pieces},
			action:   row.Action,
			isNull:   row.Operation == "push_null",
		})
	}
	if err := scanner.Err(); err != nil {
		b.Fatal(err)
	}

	blocks := make([]n1cContextBlock, 0, len(cases))
	for caseIndex, rows := range cases {
		if len(rows) < 9 {
			continue
		}
		start := (caseIndex * 5) % (len(rows) - 8)
		context, err := NewContext(model)
		if err != nil {
			b.Fatal(err)
		}
		if err := context.Reset(rows[start].position); err != nil {
			b.Fatal(err)
		}
		block := n1cContextBlock{
			context:   context,
			steps:     make([]n1cContextStep, 0, 8),
			positions: make([]base.Position, 0, 8),
		}
		for index := start + 1; index <= start+8; index++ {
			block.positions = append(block.positions, rows[index].position)
			if rows[index].isNull {
				block.steps = append(block.steps, n1cContextStep{isNull: true})
				continue
			}
			delta, err := oracleContextDelta(rows[index-1].position, rows[index].position, rows[index].action)
			if err != nil {
				b.Fatal(err)
			}
			block.steps = append(block.steps, n1cContextStep{delta: delta})
		}
		blocks = append(blocks, block)
	}
	if len(blocks) == 0 {
		b.Fatal("representative oracle has no eight-ply blocks")
	}
	return blocks
}

func pushN1cBlock(b *testing.B, block *n1cContextBlock, evaluate bool) {
	b.Helper()
	for _, step := range block.steps {
		pushN1cStep(b, block.context, step)
		if evaluate {
			if _, err := block.context.EvaluateSelected(); err != nil {
				b.Fatal(err)
			}
		}
	}
}

func pushN1cStep(b *testing.B, context *Context, step n1cContextStep) {
	b.Helper()
	if step.isNull {
		facts, err := context.Facts()
		if err != nil {
			b.Fatal(err)
		}
		if err := context.PushNullFacts(facts); err != nil {
			b.Fatal(err)
		}
		return
	}
	if err := context.PushDelta(step.delta); err != nil {
		b.Fatal(err)
	}
}

func popN1cBlock(b *testing.B, block *n1cContextBlock) {
	b.Helper()
	popN1cSteps(b, block.context, len(block.steps))
}

func popN1cSteps(b *testing.B, context *Context, count int) {
	b.Helper()
	for index := 0; index < count; index++ {
		if err := context.Pop(); err != nil {
			b.Fatal(err)
		}
	}
}
