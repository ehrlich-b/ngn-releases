package countereval

import (
	"fmt"
	"math"
)

const initialSearchFrameCount = CompatibilityFrameCount

// SearchContext is a dynamically growing, worker-owned Counter accumulator
// stack. It is separate from the fixed compatibility Context so search depth is
// never inferred from Counter's historical 128-frame allocation.
type SearchContext struct {
	model        *Model
	accumulators [][HiddenSize]float32
	boards       []Board
	depth        int
}

// NewSearchContext validates a strict-loader model, refreshes root, and creates
// an initial stack. Future pushes grow it transactionally when needed.
func (model *Model) NewSearchContext(root Board) (*SearchContext, error) {
	if _, err := model.ValidatedMetadata(); err != nil {
		return nil, err
	}
	_, accumulator, _, err := model.evaluateFullRefreshTrace(root)
	if err != nil {
		return nil, err
	}
	if err := model.validateSearchCapacity(&accumulator, initialSearchFrameCount); err != nil {
		return nil, err
	}
	accumulators := make([][HiddenSize]float32, initialSearchFrameCount)
	boards := make([]Board, initialSearchFrameCount)
	accumulators[0] = accumulator
	boards[0] = root
	return &SearchContext{model: model, accumulators: accumulators, boards: boards}, nil
}

// Reset refreshes a new admitted root while retaining already validated frame
// storage. Failure leaves the current root, stack and depth unchanged.
func (context *SearchContext) Reset(root Board) error {
	if err := context.validate(); err != nil {
		return err
	}
	_, accumulator, _, err := context.model.evaluateFullRefreshTrace(root)
	if err != nil {
		return err
	}
	if err := context.model.validateSearchCapacity(&accumulator, len(context.accumulators)); err != nil {
		return err
	}
	context.accumulators[0] = accumulator
	context.boards[0] = root
	context.depth = 0
	return nil
}

// Depth reports pushes above the root frame.
func (context *SearchContext) Depth() int {
	if context == nil {
		return 0
	}
	return context.depth
}

// FrameCapacity reports currently allocated frames. It is diagnostic state for
// capacity and ownership tests; search must not treat it as a recursion bound.
func (context *SearchContext) FrameCapacity() int {
	if context == nil {
		return 0
	}
	return len(context.accumulators)
}

// Board returns the current overlap-free board.
func (context *SearchContext) Board() Board {
	return context.boards[context.depth]
}

// EvaluateRaw returns the current White-perspective portable raw value.
func (context *SearchContext) EvaluateRaw() float32 {
	return context.model.evaluateAccumulator(&context.accumulators[context.depth])
}

// PushMove validates the complete semantic transition before considering stack
// growth. Rejected moves therefore preserve both position state and capacity.
func (context *SearchContext) PushMove(delta MoveDelta, expectedPost Board) error {
	if err := context.validate(); err != nil {
		return err
	}
	if err := validateBoard(expectedPost); err != nil {
		return fmt.Errorf("counter evaluator search context: expected post board: %w", err)
	}
	nextBoard, updates, updateCount, err := deriveTransition(context.boards[context.depth], delta)
	if err != nil {
		return err
	}
	if nextBoard != expectedPost {
		return fmt.Errorf("counter evaluator search context: derived post board differs from expected post board")
	}
	if err := context.ensurePushCapacity(); err != nil {
		return err
	}

	next := context.depth + 1
	context.accumulators[next] = context.accumulators[context.depth]
	applyFeatureUpdates(&context.accumulators[next], context.model, &updates, updateCount)
	context.boards[next] = nextBoard
	context.depth = next
	return nil
}

// PushNull grows if necessary and advances with bit-exact state copies.
func (context *SearchContext) PushNull() error {
	if err := context.validate(); err != nil {
		return err
	}
	if err := context.ensurePushCapacity(); err != nil {
		return err
	}
	next := context.depth + 1
	context.accumulators[next] = context.accumulators[context.depth]
	context.boards[next] = context.boards[context.depth]
	context.depth = next
	return nil
}

// Pop restores the preceding frame without inverse arithmetic.
func (context *SearchContext) Pop() error {
	if err := context.validate(); err != nil {
		return err
	}
	if context.depth == 0 {
		return fmt.Errorf("counter evaluator search context: cannot pop root frame")
	}
	context.depth--
	return nil
}

func (context *SearchContext) validate() error {
	if context == nil || context.model == nil {
		return fmt.Errorf("counter evaluator search context: nil context or model")
	}
	if len(context.accumulators) == 0 || len(context.accumulators) != len(context.boards) ||
		context.depth < 0 || context.depth >= len(context.accumulators) {
		return fmt.Errorf("counter evaluator search context: invalid frame state")
	}
	return nil
}

func (context *SearchContext) ensurePushCapacity() error {
	if context.depth+1 < len(context.accumulators) {
		return nil
	}
	oldFrames := len(context.accumulators)
	newFrames, err := doubledSearchFrameCount(oldFrames)
	if err != nil {
		return err
	}
	if err := context.model.validateSearchCapacity(&context.accumulators[0], newFrames); err != nil {
		return err
	}

	grownAccumulators := make([][HiddenSize]float32, newFrames)
	copy(grownAccumulators, context.accumulators)
	grownBoards := make([]Board, newFrames)
	copy(grownBoards, context.boards)
	context.accumulators = grownAccumulators
	context.boards = grownBoards
	return nil
}

func doubledSearchFrameCount(oldFrames int) (int, error) {
	maximumInt := int(^uint(0) >> 1)
	if oldFrames <= 0 || oldFrames > maximumInt/2 {
		return 0, fmt.Errorf("counter evaluator search context: frame capacity overflow at %d", oldFrames)
	}
	return oldFrames * 2, nil
}

func (model *Model) validateSearchCapacity(root *[HiddenSize]float32, frames int) error {
	if _, err := model.ValidatedMetadata(); err != nil {
		return err
	}
	if root == nil || frames <= 0 {
		return fmt.Errorf("counter evaluator search context: invalid capacity root or frames %d", frames)
	}
	updateRows := outwardBoundMul(float64(frames-1), 4)
	outputBound := math.Abs(float64(model.outputBias))
	if !finiteAndWithinMargin(outputBound) {
		return fmt.Errorf("counter evaluator search context: output bias bound is unsafe: %g", outputBound)
	}
	for hidden := 0; hidden < HiddenSize; hidden++ {
		growth := outwardBoundMul(updateRows, model.maxAbsUpdateWeight[hidden])
		laneBound := outwardBoundAdd(math.Abs(float64(root[hidden])), growth)
		if !finiteAndWithinMargin(laneBound) {
			return fmt.Errorf("counter evaluator search context: frame capacity %d accumulator lane %d is unsafe: %g", frames, hidden, laneBound)
		}
		product := outwardBoundMul(laneBound, math.Abs(float64(model.outputWeights[hidden])))
		if !finiteAndWithinMargin(product) {
			return fmt.Errorf("counter evaluator search context: frame capacity %d output product lane %d is unsafe: %g", frames, hidden, product)
		}
		outputBound = outwardBoundAdd(outputBound, product)
		if !finiteAndWithinMargin(outputBound) {
			return fmt.Errorf("counter evaluator search context: frame capacity %d cumulative output bound after lane %d is unsafe: %g", frames, hidden, outputBound)
		}
	}
	return nil
}

func outwardBoundAdd(left, right float64) float64 {
	return math.Nextafter(left+right, math.Inf(1))
}

func outwardBoundMul(left, right float64) float64 {
	return math.Nextafter(left*right, math.Inf(1))
}
