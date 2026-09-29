package nnue

import "fmt"

type int32ContextState struct {
	accumulators [PerspectiveCount][HiddenSize]int32
	facts        PositionFacts
	sideToMove   Color
	transition   contextTransition
}

// Int32Context is the explicit portable narrow-accumulator mode. It owns one
// worker's mutable preallocated stack and is numerically checked against the
// independent int64 Context; the immutable Model may be shared.
type Int32Context struct {
	model       *Model
	states      [MaxContextPly + 1]int32ContextState
	board       [64]encodedPiece
	depth       uint16
	initialized bool
}

// NewInt32Context selects portable int32 accumulator storage only when the
// validated model advertises the proven NGN-v1 accumulator bound.
func NewInt32Context(model *Model) (*Int32Context, error) {
	if model == nil || !model.loaded {
		return nil, fmt.Errorf("%w: model was not loaded from a validated container", ErrContext)
	}
	if !model.capabilities.PortableInt32Accumulator {
		return nil, fmt.Errorf("%w: model does not support portable int32 accumulators", ErrContext)
	}
	return &Int32Context{model: model}, nil
}

func (c *Int32Context) Depth() int {
	if c == nil {
		return 0
	}
	return int(c.depth)
}

func (c *Int32Context) Facts() (PositionFacts, error) {
	if err := c.ready(); err != nil {
		return PositionFacts{}, err
	}
	return c.states[c.depth].facts, nil
}

func (c *Int32Context) Reset(position Position) error {
	if c == nil || c.model == nil || !c.model.loaded ||
		!c.model.capabilities.PortableInt32Accumulator {
		return fmt.Errorf("%w: context has no compatible validated model", ErrContext)
	}
	state, board, err := c.model.refreshState32(position)
	if err != nil {
		return err
	}
	c.states[0] = state
	c.board = board
	c.depth = 0
	c.initialized = true
	return nil
}

// Push retains the full-position validation path for an int32 Context.
func (c *Int32Context) Push(delta Delta, afterPosition Position) error {
	if err := c.ready(); err != nil {
		return err
	}
	if c.depth >= MaxContextPly {
		return fmt.Errorf("%w: stack overflow at depth %d", ErrContext, c.depth)
	}
	current := &c.states[c.depth]
	if afterPosition.SideToMove != current.sideToMove^1 {
		return fmt.Errorf("%w: real move did not toggle side", ErrContext)
	}
	if err := validateDeltaShape(delta); err != nil {
		return err
	}
	if delta.Removed[0].Color != current.sideToMove {
		return fmt.Errorf("%w: mover colour does not match side to move", ErrContext)
	}
	if delta.Before != current.facts {
		return fmt.Errorf("%w: delta before-facts mismatch", ErrContext)
	}
	afterBoard, afterFacts, err := boardAndFacts(afterPosition)
	if err != nil {
		return err
	}
	if delta.After != afterFacts {
		return fmt.Errorf("%w: delta after-facts mismatch", ErrContext)
	}
	reconstructed, err := reconstructBefore(afterBoard, delta)
	if err != nil {
		return err
	}
	if reconstructed != c.board {
		return fmt.Errorf("%w: after-position plus delta does not reconstruct current board", ErrContext)
	}
	c.commitReal(delta, afterBoard, afterFacts)
	return nil
}

// PushDelta uses the checked allocation-free semantic transition path.
func (c *Int32Context) PushDelta(delta Delta) error {
	if err := c.ready(); err != nil {
		return err
	}
	if c.depth >= MaxContextPly {
		return fmt.Errorf("%w: stack overflow at depth %d", ErrContext, c.depth)
	}
	current := &c.states[c.depth]
	if err := validateDeltaShape(delta); err != nil {
		return err
	}
	if delta.Removed[0].Color != current.sideToMove {
		return fmt.Errorf("%w: mover colour does not match side to move", ErrContext)
	}
	if delta.Before != current.facts {
		return fmt.Errorf("%w: delta before-facts mismatch", ErrContext)
	}
	afterBoard, afterFacts, err := applyDeltaForward(c.board, current.facts, delta)
	if err != nil {
		return err
	}
	if delta.After != afterFacts {
		return fmt.Errorf("%w: delta after-facts mismatch", ErrContext)
	}
	c.commitReal(delta, afterBoard, afterFacts)
	return nil
}

func (c *Int32Context) commitReal(delta Delta, afterBoard [64]encodedPiece, afterFacts PositionFacts) {
	current := &c.states[c.depth]
	nextDepth := c.depth + 1
	c.states[nextDepth] = *current
	next := &c.states[nextDepth]
	for i := 0; i < int(delta.RemovedCount); i++ {
		c.model.updateAccumulators32(&next.accumulators, delta.Removed[i], -1)
	}
	for i := 0; i < int(delta.AddedCount); i++ {
		c.model.updateAccumulators32(&next.accumulators, delta.Added[i], 1)
	}
	next.facts = afterFacts
	next.sideToMove = current.sideToMove ^ 1
	next.transition = contextTransition{
		removedCount: delta.RemovedCount,
		addedCount:   delta.AddedCount,
		removed:      delta.Removed,
		added:        delta.Added,
	}
	c.board = afterBoard
	c.depth = nextDepth
}

func (c *Int32Context) PushNull(afterPosition Position) error {
	if err := c.ready(); err != nil {
		return err
	}
	if c.depth >= MaxContextPly {
		return fmt.Errorf("%w: stack overflow at depth %d", ErrContext, c.depth)
	}
	current := &c.states[c.depth]
	if afterPosition.SideToMove != current.sideToMove^1 {
		return fmt.Errorf("%w: null move did not toggle side", ErrContext)
	}
	afterBoard, afterFacts, err := boardAndFacts(afterPosition)
	if err != nil {
		return err
	}
	if afterBoard != c.board || afterFacts != current.facts {
		return fmt.Errorf("%w: null move changed board features", ErrContext)
	}
	c.commitNull()
	return nil
}

func (c *Int32Context) PushNullFacts(after PositionFacts) error {
	if err := c.ready(); err != nil {
		return err
	}
	if c.depth >= MaxContextPly {
		return fmt.Errorf("%w: stack overflow at depth %d", ErrContext, c.depth)
	}
	if after != c.states[c.depth].facts {
		return fmt.Errorf("%w: null move changed board features", ErrContext)
	}
	c.commitNull()
	return nil
}

func (c *Int32Context) commitNull() {
	current := &c.states[c.depth]
	nextDepth := c.depth + 1
	c.states[nextDepth] = *current
	c.states[nextDepth].sideToMove = current.sideToMove ^ 1
	c.states[nextDepth].transition = contextTransition{}
	c.depth = nextDepth
}

func (c *Int32Context) Pop() error {
	if err := c.ready(); err != nil {
		return err
	}
	if c.depth == 0 {
		return fmt.Errorf("%w: stack underflow", ErrContext)
	}
	transition := c.states[c.depth].transition
	before, err := reconstructBefore(c.board, Delta{
		RemovedCount: transition.removedCount,
		AddedCount:   transition.addedCount,
		Removed:      transition.removed,
		Added:        transition.added,
	})
	if err != nil {
		return fmt.Errorf("%w: corrupt stored transition: %v", ErrContext, err)
	}
	c.board = before
	c.depth--
	return nil
}

// Evaluate widens every activation and output multiply to int64, retaining the
// exact reference post-dot operation order.
func (c *Int32Context) Evaluate() (int64, error) {
	if err := c.ready(); err != nil {
		return 0, err
	}
	state := &c.states[c.depth]
	return c.model.evaluateAccumulators32(&state.accumulators, state.sideToMove), nil
}

func (c *Int32Context) ready() error {
	if c == nil || c.model == nil || !c.model.loaded ||
		!c.model.capabilities.PortableInt32Accumulator {
		return fmt.Errorf("%w: context has no compatible validated model", ErrContext)
	}
	if !c.initialized {
		return fmt.Errorf("%w: context is not reset", ErrContext)
	}
	return nil
}

func (m *Model) refreshState32(position Position) (int32ContextState, [64]encodedPiece, error) {
	var state int32ContextState
	var board [64]encodedPiece
	if position.SideToMove > Black {
		return state, board, fmt.Errorf("%w: side to move %d", ErrContext, position.SideToMove)
	}
	board, facts, err := boardAndFacts(position)
	if err != nil {
		return state, board, err
	}
	for perspective := 0; perspective < PerspectiveCount; perspective++ {
		for hidden, bias := range m.featureBias {
			state.accumulators[perspective][hidden] = int32(bias)
		}
	}
	for square, code := range board {
		if code == emptyPiece {
			continue
		}
		m.updateAccumulators32(&state.accumulators, decodePiece(code, Square(square)), 1)
	}
	state.facts = facts
	state.sideToMove = position.SideToMove
	return state, board, nil
}

func (m *Model) updateAccumulators32(
	accumulators *[PerspectiveCount][HiddenSize]int32,
	piece PieceOnSquare,
	direction int32,
) {
	for perspective := White; perspective <= Black; perspective++ {
		feature := featureIndexUnchecked(piece, perspective)
		for hidden, weight := range m.featureWeights[feature] {
			accumulators[perspective][hidden] += direction * int32(weight)
		}
	}
}

func (m *Model) evaluateAccumulators32(
	accumulators *[PerspectiveCount][HiddenSize]int32,
	sideToMove Color,
) int64 {
	us := int(sideToMove)
	them := us ^ 1
	var dot int64
	if m.capabilities.BoundedInt32Output {
		dot = int64(boundedOutputDot(
			&accumulators[us],
			&accumulators[them],
			&m.outputWeights,
		))
	} else {
		dot = m.outputDot32Reference(accumulators, sideToMove)
	}
	return m.finishOutput(dot)
}

func (m *Model) outputDot32Reference(
	accumulators *[PerspectiveCount][HiddenSize]int32,
	sideToMove Color,
) int64 {
	us := int(sideToMove)
	them := us ^ 1
	var dot int64
	for hidden := 0; hidden < HiddenSize; hidden++ {
		dot += squareClipped(int64(accumulators[us][hidden])) * int64(m.outputWeights[hidden])
		dot += squareClipped(int64(accumulators[them][hidden])) * int64(m.outputWeights[HiddenSize+hidden])
	}
	return dot
}

func (m *Model) finishOutput(dot int64) int64 {
	// Keep the score contract's two signed divisions separate and in order.
	// The bounded dot is widened before either division or bias arithmetic.
	value := dot / QA
	value += int64(m.outputBias)
	return value * ScoreScale / (QA * QB)
}
