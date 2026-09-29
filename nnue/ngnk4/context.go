package ngnk4

import (
	"fmt"
	"math/bits"
)

const initialSearchFrameCount = 128

const (
	pawnTypePlane = 0
	rookTypePlane = 3
	kingTypePlane = 5
	colorPlanes   = 6
)

// MoveDelta describes one admitted semantic board transition. Chess legality
// remains the engine adapter's responsibility.
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

type featureUpdate struct {
	plane  int
	square int
	add    bool
}

// SearchContext owns one worker's dynamically growing, wide-accumulator stack.
type SearchContext struct {
	model        *Model
	accumulators []accumulator
	positions    []Position
	depth        int
}

// NewSearchContext validates and fully refreshes the root before publication.
func (model *Model) NewSearchContext(root Position) (*SearchContext, error) {
	if err := model.validate(); err != nil {
		return nil, err
	}
	if err := validatePosition(root); err != nil {
		return nil, err
	}
	accumulators := make([]accumulator, initialSearchFrameCount)
	positions := make([]Position, initialSearchFrameCount)
	accumulators[0] = model.fullRefresh(root.Board)
	positions[0] = root
	return &SearchContext{model: model, accumulators: accumulators, positions: positions}, nil
}

// Reset transactionally replaces the root while retaining frame storage.
func (context *SearchContext) Reset(root Position) error {
	if err := context.validate(); err != nil {
		return err
	}
	if err := validatePosition(root); err != nil {
		return err
	}
	refreshed := context.model.fullRefresh(root.Board)
	context.accumulators[0] = refreshed
	context.positions[0] = root
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

// FrameCapacity reports allocated frames for ownership and growth tests.
func (context *SearchContext) FrameCapacity() int {
	if context == nil {
		return 0
	}
	return len(context.accumulators)
}

// Position returns the current evaluator input value.
func (context *SearchContext) Position() Position {
	if context == nil || context.depth < 0 || context.depth >= len(context.positions) {
		return Position{}
	}
	return context.positions[context.depth]
}

// EvaluateRaw evaluates the current incremental frame.
func (context *SearchContext) EvaluateRaw() (int64, error) {
	if err := context.validate(); err != nil {
		return 0, err
	}
	position := context.positions[context.depth]
	return context.model.evaluateAccumulator(
		&context.accumulators[context.depth],
		position.SideToMove,
		outputBucket(position.Board),
	), nil
}

// PushMove validates the complete transition before any stack growth. A
// rejected transition preserves depth, capacity, position and accumulators.
func (context *SearchContext) PushMove(delta MoveDelta, expectedPost Position) error {
	if err := context.validate(); err != nil {
		return err
	}
	current := context.positions[context.depth]
	nextPosition, updates, updateCount, refreshPerspective, err := deriveTransition(current, delta)
	if err != nil {
		return err
	}
	if nextPosition != expectedPost {
		return fmt.Errorf("NGN K4 search context: derived post position differs from expected post position")
	}
	if err := context.ensurePushCapacity(); err != nil {
		return err
	}
	next := context.depth + 1
	context.accumulators[next] = context.accumulators[context.depth]
	context.model.applyUpdates(&context.accumulators[next], nextPosition.Board, &updates, updateCount, refreshPerspective)
	context.positions[next] = nextPosition
	context.depth = next
	return nil
}

// PushNull copies the accumulator and board, changing only side to move.
func (context *SearchContext) PushNull() error {
	if err := context.validate(); err != nil {
		return err
	}
	if err := context.ensurePushCapacity(); err != nil {
		return err
	}
	next := context.depth + 1
	context.accumulators[next] = context.accumulators[context.depth]
	context.positions[next] = context.positions[context.depth]
	context.positions[next].SideToMove ^= 1
	context.depth = next
	return nil
}

// Pop restores the parent frame without inverse arithmetic.
func (context *SearchContext) Pop() error {
	if err := context.validate(); err != nil {
		return err
	}
	if context.depth == 0 {
		return fmt.Errorf("NGN K4 search context: cannot pop root frame")
	}
	context.depth--
	return nil
}

func (context *SearchContext) validate() error {
	if context == nil || context.model == nil {
		return fmt.Errorf("NGN K4 search context: nil context or model")
	}
	if err := context.model.validate(); err != nil {
		return err
	}
	if len(context.accumulators) == 0 || len(context.accumulators) != len(context.positions) ||
		context.depth < 0 || context.depth >= len(context.accumulators) {
		return fmt.Errorf("NGN K4 search context: invalid frame state")
	}
	return nil
}

func (context *SearchContext) ensurePushCapacity() error {
	if context.depth+1 < len(context.accumulators) {
		return nil
	}
	oldFrames := len(context.accumulators)
	maximumInt := int(^uint(0) >> 1)
	if oldFrames <= 0 || oldFrames > maximumInt/2 {
		return fmt.Errorf("NGN K4 search context: frame capacity overflow at %d", oldFrames)
	}
	newFrames := oldFrames * 2
	accumulators := make([]accumulator, newFrames)
	copy(accumulators, context.accumulators)
	positions := make([]Position, newFrames)
	copy(positions, context.positions)
	context.accumulators = accumulators
	context.positions = positions
	return nil
}

func deriveTransition(current Position, delta MoveDelta) (Position, [4]featureUpdate, int, Color, error) {
	fail := func(format string, args ...interface{}) (Position, [4]featureUpdate, int, Color, error) {
		return Position{}, [4]featureUpdate{}, 0, Color(2), fmt.Errorf("NGN K4 search context: "+format, args...)
	}
	// SearchContext admits a validated root and stores only transitions derived
	// here, so current is already a valid invariant. The derived position cannot
	// gain overlaps, kings, or pieces: helpers require exact removals and empty
	// destinations, king captures are forbidden, and every move preserves both
	// kings and the <=32-piece bound. Comparing it with expectedPost in PushMove
	// therefore validates the engine adapter without rescanning 12 bitboards
	// three times on every searched edge.
	if delta.MovingPlane >= FeaturePlaneCount {
		return fail("moving plane %d out of range", delta.MovingPlane)
	}
	if delta.From >= 64 || delta.To >= 64 || delta.From == delta.To {
		return fail("invalid move squares %d -> %d", delta.From, delta.To)
	}
	if !delta.HasCapture && (delta.CapturedPlane != 0 || delta.CaptureSquare != 0) {
		return fail("capture fields set without capture")
	}
	if !delta.HasPromotion && delta.PromotionPlane != 0 {
		return fail("promotion plane set without promotion")
	}
	if !delta.HasCastleRook && (delta.CastleRookFrom != 0 || delta.CastleRookTo != 0) {
		return fail("rook fields set without castling")
	}

	movingPlane := int(delta.MovingPlane)
	from, to := int(delta.From), int(delta.To)
	movingColor := Color(movingPlane / colorPlanes)
	if movingColor != current.SideToMove {
		return fail("mover color %d differs from side to move %d", movingColor, current.SideToMove)
	}
	targetPlane := movingPlane
	if delta.HasPromotion {
		promotionPlane := int(delta.PromotionPlane)
		if promotionPlane >= FeaturePlaneCount || movingPlane%colorPlanes != pawnTypePlane ||
			promotionPlane/colorPlanes != movingPlane/colorPlanes ||
			promotionPlane%colorPlanes < 1 || promotionPlane%colorPlanes > 4 {
			return fail("invalid promotion plane %d for moving plane %d", promotionPlane, movingPlane)
		}
		targetPlane = promotionPlane
	}
	if delta.HasCastleRook && (delta.CastleRookFrom >= 64 || delta.CastleRookTo >= 64 ||
		delta.CastleRookFrom == delta.CastleRookTo || movingPlane%colorPlanes != kingTypePlane ||
		delta.HasCapture || delta.HasPromotion) {
		return fail("invalid castling metadata")
	}

	next := current
	if err := removeBoardPiece(&next.Board, movingPlane, from); err != nil {
		return fail("moving piece: %v", err)
	}
	if delta.HasCapture {
		capturedPlane, captureSquare := int(delta.CapturedPlane), int(delta.CaptureSquare)
		if capturedPlane >= FeaturePlaneCount || captureSquare >= 64 ||
			capturedPlane/colorPlanes == movingPlane/colorPlanes || capturedPlane%colorPlanes == kingTypePlane {
			return fail("invalid captured plane/square")
		}
		if captureSquare != to &&
			(delta.HasPromotion || movingPlane%colorPlanes != pawnTypePlane || capturedPlane%colorPlanes != pawnTypePlane) {
			return fail("displaced capture is not pawn en passant")
		}
		if err := removeBoardPiece(&next.Board, capturedPlane, captureSquare); err != nil {
			return fail("captured piece: %v", err)
		}
	}
	if err := addBoardPiece(&next.Board, targetPlane, to); err != nil {
		return fail("destination: %v", err)
	}
	if delta.HasCastleRook {
		rookPlane := movingPlane/colorPlanes*colorPlanes + rookTypePlane
		if err := removeBoardPiece(&next.Board, rookPlane, int(delta.CastleRookFrom)); err != nil {
			return fail("castling rook: %v", err)
		}
		if err := addBoardPiece(&next.Board, rookPlane, int(delta.CastleRookTo)); err != nil {
			return fail("castling rook destination: %v", err)
		}
	}
	next.SideToMove ^= 1

	updates := [4]featureUpdate{{plane: targetPlane, square: to, add: true}, {plane: movingPlane, square: from}}
	updateCount := 2
	if delta.HasCapture {
		updates[updateCount] = featureUpdate{plane: int(delta.CapturedPlane), square: int(delta.CaptureSquare)}
		updateCount++
	}
	if delta.HasCastleRook {
		rookPlane := movingPlane/colorPlanes*colorPlanes + rookTypePlane
		updates[updateCount] = featureUpdate{plane: rookPlane, square: int(delta.CastleRookTo), add: true}
		updates[updateCount+1] = featureUpdate{plane: rookPlane, square: int(delta.CastleRookFrom)}
		updateCount += 2
	}

	refreshPerspective := Color(2)
	if movingPlane%colorPlanes == kingTypePlane {
		orientedFrom, orientedTo := from, to
		if movingColor == Black {
			orientedFrom ^= 56
			orientedTo ^= 56
		}
		if kingBucketTable[orientedFrom] != kingBucketTable[orientedTo] ||
			(from%8 > 3) != (to%8 > 3) {
			refreshPerspective = movingColor
		}
	}
	return next, updates, updateCount, refreshPerspective, nil
}

func (model *Model) applyUpdates(destination *accumulator, postBoard Board, updates *[4]featureUpdate, updateCount int, refresh Color) {
	kingSquares := [2]int{bits.TrailingZeros64(postBoard[WhiteKing]), bits.TrailingZeros64(postBoard[BlackKing])}
	for perspective := White; perspective <= Black; perspective++ {
		if perspective == refresh {
			model.refreshPerspective(&destination[perspective], postBoard, perspective, kingSquares[perspective])
			continue
		}
		var rows [4]*[HiddenSize]int16
		for index, update := range updates[:updateCount] {
			color := Color(update.plane / colorPlanes)
			pieceType := update.plane % colorPlanes
			rows[index] = &model.inputWeights[featureIndex(color, pieceType, update.square, kingSquares[perspective], perspective)]
		}
		switch {
		case updateCount == 2 && updates[0].add && !updates[1].add:
			k4ApplyUpdates2(&destination[perspective], rows[0], rows[1])
		case updateCount == 3 && updates[0].add && !updates[1].add && !updates[2].add:
			k4ApplyUpdates3(&destination[perspective], rows[0], rows[1], rows[2])
		case updateCount == 4 && updates[0].add && !updates[1].add && updates[2].add && !updates[3].add:
			k4ApplyUpdates4(&destination[perspective], rows[0], rows[1], rows[2], rows[3])
		default:
			for hidden := 0; hidden < HiddenSize; hidden++ {
				value := destination[perspective][hidden]
				for index, update := range updates[:updateCount] {
					if update.add {
						value += rows[index][hidden]
					} else {
						value -= rows[index][hidden]
					}
				}
				destination[perspective][hidden] = value
			}
		}
	}
}

func (model *Model) refreshPerspective(destination *[HiddenSize]int16, board Board, perspective Color, kingSquare int) {
	var rows [k4MaximumRefreshRows]*[HiddenSize]int16
	rowCount := 0
	for plane, pieceBits := range board {
		color := Color(plane / colorPlanes)
		pieceType := plane % colorPlanes
		for pieceBits != 0 {
			square := bits.TrailingZeros64(pieceBits)
			pieceBits &= pieceBits - 1
			rows[rowCount] = &model.inputWeights[featureIndex(color, pieceType, square, kingSquare, perspective)]
			rowCount++
		}
	}
	k4RefreshPerspective(destination, &model.inputBiases, &rows, rowCount)
}

func removeBoardPiece(board *Board, plane, square int) error {
	mask := uint64(1) << square
	if board[plane]&mask == 0 {
		return fmt.Errorf("plane %d has no piece at square %d", plane, square)
	}
	board[plane] &^= mask
	return nil
}

func addBoardPiece(board *Board, plane, square int) error {
	mask := uint64(1) << square
	if occupiedBoard(*board)&mask != 0 {
		return fmt.Errorf("square %d is occupied", square)
	}
	board[plane] |= mask
	return nil
}

func occupiedBoard(board Board) uint64 {
	var occupied uint64
	for _, pieces := range board {
		occupied |= pieces
	}
	return occupied
}
