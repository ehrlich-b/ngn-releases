package countereval

import (
	"fmt"
	"math"
)

// CompatibilityFrameCount matches Counter 5.5's fixed compatibility stack.
const CompatibilityFrameCount = 128

const (
	pawnPlane     = 0
	rookTypePlane = 3
	kingPlane     = 5
	colorPlanes   = 6
)

// MoveDelta describes the ordered feature changes for one already-validated move.
type MoveDelta struct {
	MovingPlane uint8
	From        uint8
	To          uint8

	HasCapture    bool
	CapturedPlane uint8
	CaptureSquare uint8

	HasPromotion   bool
	PromotionPlane uint8

	HasCastleRook  bool
	CastleRookFrom uint8
	CastleRookTo   uint8
}

// Context holds one model's portable incremental accumulator stack.
// A Context is mutable, belongs to one worker, and must be created by NewContext.
type Context struct {
	model        *Model
	accumulators [CompatibilityFrameCount][HiddenSize]float32
	boards       [CompatibilityFrameCount]Board
	depth        int
}

type featureUpdate struct {
	feature int
	add     bool
}

// NewContext validates the model's worst compatibility-stack range and fully
// refreshes the root board before publishing a context.
func (model *Model) NewContext(root Board) (*Context, error) {
	if model == nil {
		return nil, fmt.Errorf("counter evaluator context: nil model")
	}
	_, accumulator, _, err := model.evaluateFullRefreshTrace(root)
	if err != nil {
		return nil, err
	}
	if err := validateContextBounds(model, &accumulator); err != nil {
		return nil, err
	}
	context := &Context{model: model}
	context.accumulators[0] = accumulator
	context.boards[0] = root
	return context, nil
}

func validateContextBounds(model *Model, rootAccumulator *[HiddenSize]float32) error {
	const maxUpdatesPerMove = 4
	maxUpdateRows := float64((CompatibilityFrameCount - 1) * maxUpdatesPerMove)
	var laneBounds [HiddenSize]float64
	for hidden := 0; hidden < HiddenSize; hidden++ {
		maxWeight := 0.0
		for feature := 0; feature < InputSize; feature++ {
			weight := math.Abs(float64(model.hiddenWeights[feature*HiddenSize+hidden]))
			if weight > maxWeight {
				maxWeight = weight
			}
		}
		bound := math.Abs(float64(rootAccumulator[hidden])) + maxUpdateRows*maxWeight
		if !finiteAndWithinMargin(bound) {
			return fmt.Errorf("counter evaluator context: incremental accumulator bound lane %d is unsafe: %g", hidden, bound)
		}
		laneBounds[hidden] = bound
	}
	outputBound := math.Abs(float64(model.outputBias))
	for hidden, laneBound := range laneBounds {
		productBound := laneBound * math.Abs(float64(model.outputWeights[hidden]))
		if !finiteAndWithinMargin(productBound) {
			return fmt.Errorf("counter evaluator context: incremental output product bound lane %d is unsafe: %g", hidden, productBound)
		}
		outputBound += productBound
		if !finiteAndWithinMargin(outputBound) {
			return fmt.Errorf("counter evaluator context: incremental cumulative output bound after lane %d is unsafe: %g", hidden, outputBound)
		}
	}
	return nil
}

// Depth reports pushes above the root frame.
func (context *Context) Depth() int {
	if context == nil {
		return 0
	}
	return context.depth
}

// Board returns the current overlap-free board.
func (context *Context) Board() Board {
	return context.boards[context.depth]
}

// EvaluateRaw returns the current White-perspective portable raw value.
func (context *Context) EvaluateRaw() float32 {
	return context.model.evaluateAccumulator(&context.accumulators[context.depth])
}

// PushNull advances with a bit-exact accumulator and board copy.
func (context *Context) PushNull() error {
	if err := context.validatePush(); err != nil {
		return err
	}
	next := context.depth + 1
	context.accumulators[next] = context.accumulators[context.depth]
	context.boards[next] = context.boards[context.depth]
	context.depth = next
	return nil
}

// PushMove transactionally validates and applies one semantic move delta.
func (context *Context) PushMove(delta MoveDelta, expectedPost Board) error {
	if err := context.validatePush(); err != nil {
		return err
	}
	if err := validateBoard(expectedPost); err != nil {
		return fmt.Errorf("counter evaluator context: expected post board: %w", err)
	}

	nextBoard, updates, updateCount, err := deriveTransition(context.boards[context.depth], delta)
	if err != nil {
		return err
	}
	if nextBoard != expectedPost {
		return fmt.Errorf("counter evaluator context: derived post board differs from expected post board")
	}

	next := context.depth + 1
	context.accumulators[next] = context.accumulators[context.depth]
	applyFeatureUpdates(&context.accumulators[next], context.model, &updates, updateCount)
	context.boards[next] = nextBoard
	context.depth = next
	return nil
}

func applyFeatureUpdates(
	accumulator *[HiddenSize]float32,
	model *Model,
	updates *[4]featureUpdate,
	updateCount int,
) {
	for _, update := range updates[:updateCount] {
		base := update.feature * HiddenSize
		weights := (*[HiddenSize]float32)(model.hiddenWeights[base : base+HiddenSize])
		if update.add {
			addFeatureRow(accumulator, weights)
		} else {
			subFeatureRow(accumulator, weights)
		}
	}
}

// Pop restores the preceding frame without inverse arithmetic.
func (context *Context) Pop() error {
	if err := context.validate(); err != nil {
		return err
	}
	if context.depth == 0 {
		return fmt.Errorf("counter evaluator context: cannot pop root frame")
	}
	context.depth--
	return nil
}

func (context *Context) validate() error {
	if context == nil {
		return fmt.Errorf("counter evaluator context: nil context")
	}
	if context.model == nil {
		return fmt.Errorf("counter evaluator context: nil model")
	}
	if context.depth < 0 || context.depth >= CompatibilityFrameCount {
		return fmt.Errorf("counter evaluator context: invalid depth %d", context.depth)
	}
	return nil
}

func (context *Context) validatePush() error {
	if err := context.validate(); err != nil {
		return err
	}
	if context.depth+1 >= CompatibilityFrameCount {
		return fmt.Errorf("counter evaluator context: frame capacity %d exceeded", CompatibilityFrameCount)
	}
	return nil
}

func deriveTransition(current Board, delta MoveDelta) (Board, [4]featureUpdate, int, error) {
	if err := validateBoard(current); err != nil {
		return Board{}, [4]featureUpdate{}, 0, fmt.Errorf("counter evaluator context: current board: %w", err)
	}
	if delta.MovingPlane >= FeaturePlaneCount {
		return Board{}, [4]featureUpdate{}, 0, fmt.Errorf("counter evaluator context: moving plane %d out of range", delta.MovingPlane)
	}
	if delta.From >= 64 || delta.To >= 64 || delta.From == delta.To {
		return Board{}, [4]featureUpdate{}, 0, fmt.Errorf("counter evaluator context: invalid move squares %d -> %d", delta.From, delta.To)
	}
	if !delta.HasCapture && (delta.CapturedPlane != 0 || delta.CaptureSquare != 0) {
		return Board{}, [4]featureUpdate{}, 0, fmt.Errorf("counter evaluator context: capture fields set without capture")
	}
	if !delta.HasPromotion && delta.PromotionPlane != 0 {
		return Board{}, [4]featureUpdate{}, 0, fmt.Errorf("counter evaluator context: promotion plane set without promotion")
	}
	if !delta.HasCastleRook && (delta.CastleRookFrom != 0 || delta.CastleRookTo != 0) {
		return Board{}, [4]featureUpdate{}, 0, fmt.Errorf("counter evaluator context: rook fields set without castling")
	}

	movingPlane := int(delta.MovingPlane)
	from := int(delta.From)
	to := int(delta.To)
	targetPlane := movingPlane
	if delta.HasPromotion {
		promotionPlane := int(delta.PromotionPlane)
		if promotionPlane >= FeaturePlaneCount ||
			movingPlane%colorPlanes != pawnPlane ||
			promotionPlane/colorPlanes != movingPlane/colorPlanes ||
			promotionPlane%colorPlanes < 1 || promotionPlane%colorPlanes > 4 {
			return Board{}, [4]featureUpdate{}, 0, fmt.Errorf("counter evaluator context: invalid promotion plane %d for moving plane %d", promotionPlane, movingPlane)
		}
		targetPlane = promotionPlane
	}
	if delta.HasCastleRook {
		if delta.CastleRookFrom >= 64 || delta.CastleRookTo >= 64 ||
			delta.CastleRookFrom == delta.CastleRookTo ||
			movingPlane%colorPlanes != kingPlane ||
			delta.HasCapture || delta.HasPromotion {
			return Board{}, [4]featureUpdate{}, 0, fmt.Errorf("counter evaluator context: invalid castling metadata")
		}
	}

	next := current
	var updates [4]featureUpdate
	updateCount := 0
	if err := removePiece(&next, movingPlane, from); err != nil {
		return Board{}, [4]featureUpdate{}, 0, fmt.Errorf("counter evaluator context: moving piece: %w", err)
	}
	updates[updateCount] = featureUpdate{feature: movingPlane*64 + from}
	updateCount++

	if delta.HasCapture {
		capturedPlane := int(delta.CapturedPlane)
		captureSquare := int(delta.CaptureSquare)
		if capturedPlane >= FeaturePlaneCount || captureSquare >= 64 ||
			capturedPlane/colorPlanes == movingPlane/colorPlanes ||
			capturedPlane%colorPlanes == kingPlane {
			return Board{}, [4]featureUpdate{}, 0, fmt.Errorf("counter evaluator context: invalid captured plane/square")
		}
		if captureSquare != to &&
			(movingPlane%colorPlanes != pawnPlane || capturedPlane%colorPlanes != pawnPlane) {
			return Board{}, [4]featureUpdate{}, 0, fmt.Errorf("counter evaluator context: displaced capture is not pawn en passant")
		}
		if err := removePiece(&next, capturedPlane, captureSquare); err != nil {
			return Board{}, [4]featureUpdate{}, 0, fmt.Errorf("counter evaluator context: captured piece: %w", err)
		}
		updates[updateCount] = featureUpdate{feature: capturedPlane*64 + captureSquare}
		updateCount++
	}
	if err := addPiece(&next, targetPlane, to); err != nil {
		return Board{}, [4]featureUpdate{}, 0, fmt.Errorf("counter evaluator context: destination: %w", err)
	}
	updates[updateCount] = featureUpdate{feature: targetPlane*64 + to, add: true}
	updateCount++

	if delta.HasCastleRook {
		castleRookPlane := movingPlane/colorPlanes*colorPlanes + rookTypePlane
		rookFrom := int(delta.CastleRookFrom)
		rookTo := int(delta.CastleRookTo)
		if err := removePiece(&next, castleRookPlane, rookFrom); err != nil {
			return Board{}, [4]featureUpdate{}, 0, fmt.Errorf("counter evaluator context: castling rook: %w", err)
		}
		updates[updateCount] = featureUpdate{feature: castleRookPlane*64 + rookFrom}
		updateCount++
		if err := addPiece(&next, castleRookPlane, rookTo); err != nil {
			return Board{}, [4]featureUpdate{}, 0, fmt.Errorf("counter evaluator context: castling rook destination: %w", err)
		}
		updates[updateCount] = featureUpdate{feature: castleRookPlane*64 + rookTo, add: true}
		updateCount++
	}

	return next, updates, updateCount, nil
}

func removePiece(board *Board, plane, square int) error {
	mask := uint64(1) << square
	if board[plane]&mask == 0 {
		return fmt.Errorf("plane %d has no piece at square %d", plane, square)
	}
	board[plane] &^= mask
	return nil
}

func addPiece(board *Board, plane, square int) error {
	mask := uint64(1) << square
	if occupied(*board)&mask != 0 {
		return fmt.Errorf("square %d is occupied", square)
	}
	board[plane] |= mask
	return nil
}

func occupied(board Board) uint64 {
	var result uint64
	for _, plane := range board {
		result |= plane
	}
	return result
}

func validateBoard(board Board) error {
	var seen uint64
	for plane, pieces := range board {
		if seen&pieces != 0 {
			return fmt.Errorf("plane %d overlaps another piece plane", plane)
		}
		seen |= pieces
	}
	return nil
}

func (model *Model) evaluateAccumulator(accumulator *[HiddenSize]float32) float32 {
	return counterOutputDot(accumulator, &model.outputWeights) + model.outputBias
}
