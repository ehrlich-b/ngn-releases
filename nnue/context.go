package nnue

import (
	"errors"
	"fmt"
)

const (
	// MaxContextPly bounds the preallocated real/null transition stack.
	MaxContextPly = 256
	// MaxDeltaPieces covers every orthodox chess transition: captures,
	// en-passant, promotions, and castling.
	MaxDeltaPieces = 2
	// NoSquare marks an absent king in PositionFacts.
	NoSquare Square = 64
)

var ErrContext = errors.New("invalid NNUE context operation")

// MoveKind identifies the semantic shape of a real chess transition.
type MoveKind uint8

const (
	MoveNormal MoveKind = iota
	MoveCapture
	MoveEnPassant
	MoveCastle
	MovePromotion
	MovePromotionCapture
)

// PositionFacts are compact before/after invariants supplied by the engine
// adapter. Kings use NoSquare when absent in a deliberately incomplete fixture.
type PositionFacts struct {
	Occupied   uint64
	Pawns      [PerspectiveCount]uint64
	KingSquare [PerspectiveCount]Square
}

// Delta describes all board changes made by one real move. Removed[0] and
// Added[0] are the mover before and after the move. A capture is Removed[1].
// Castling uses element 1 for the rook. Unused array elements are ignored.
type Delta struct {
	Kind         MoveKind
	RemovedCount uint8
	AddedCount   uint8
	Removed      [MaxDeltaPieces]PieceOnSquare
	Added        [MaxDeltaPieces]PieceOnSquare
	Before       PositionFacts
	After        PositionFacts
}

type encodedPiece uint8

const emptyPiece encodedPiece = 0

type contextTransition struct {
	removedCount uint8
	addedCount   uint8
	removed      [MaxDeltaPieces]PieceOnSquare
	added        [MaxDeltaPieces]PieceOnSquare
}

type contextState struct {
	accumulators [PerspectiveCount][HiddenSize]int64
	facts        PositionFacts
	sideToMove   Color
	transition   contextTransition
}

// Context is one worker's mutable, preallocated incremental state. A Context
// must never be shared by workers. Its Model may be shared because Model is
// immutable after a validated load.
type Context struct {
	model       *Model
	states      [MaxContextPly + 1]contextState
	board       [64]encodedPiece
	depth       uint16
	initialized bool
}

// NewContext binds an immutable model to a new worker-private context.
func NewContext(model *Model) (*Context, error) {
	if model == nil || !model.loaded {
		return nil, fmt.Errorf("%w: model was not loaded from a validated container", ErrContext)
	}
	return &Context{model: model}, nil
}

// Depth returns the number of live real and null transition frames.
func (c *Context) Depth() int {
	if c == nil {
		return 0
	}
	return int(c.depth)
}

// Facts returns a copy of the current compact board facts.
func (c *Context) Facts() (PositionFacts, error) {
	if err := c.ready(); err != nil {
		return PositionFacts{}, err
	}
	return c.states[c.depth].facts, nil
}

// Reset transactionally rebuilds the root accumulators from a full position.
func (c *Context) Reset(position Position) error {
	if c == nil || c.model == nil || !c.model.loaded {
		return fmt.Errorf("%w: context has no validated model", ErrContext)
	}
	state, board, err := c.model.refreshState(position)
	if err != nil {
		return err
	}
	c.states[0] = state
	c.board = board
	c.depth = 0
	c.initialized = true
	return nil
}

// Push validates a real transition by reconstructing the current board from
// afterPosition and delta, then updates both perspective accumulators. No
// Context state changes unless all validation succeeds.
func (c *Context) Push(delta Delta, afterPosition Position) error {
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

// PushDelta applies a fully semantic real-move delta without requiring a full
// after-position scan. Before and After facts are checked against the private
// board while every change is staged locally; a failure leaves the Context
// byte-for-byte unchanged.
func (c *Context) PushDelta(delta Delta) error {
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

func (c *Context) commitReal(delta Delta, afterBoard [64]encodedPiece, afterFacts PositionFacts) {
	current := &c.states[c.depth]
	nextDepth := c.depth + 1
	c.states[nextDepth] = *current
	next := &c.states[nextDepth]
	for i := 0; i < int(delta.RemovedCount); i++ {
		c.model.updateAccumulators(&next.accumulators, delta.Removed[i], -1)
	}
	for i := 0; i < int(delta.AddedCount); i++ {
		c.model.updateAccumulators(&next.accumulators, delta.Added[i], 1)
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

// PushNull adds a null frame. The board and accumulators must be unchanged;
// only side-to-move flips. Pop symmetrically removes the alias-equivalent frame.
func (c *Context) PushNull(afterPosition Position) error {
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

// PushNullFacts adds a null frame from compact facts. A null move must preserve
// every board feature, so the supplied facts must equal the current state.
func (c *Context) PushNullFacts(after PositionFacts) error {
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

func (c *Context) commitNull() {
	current := &c.states[c.depth]
	nextDepth := c.depth + 1
	c.states[nextDepth] = *current
	c.states[nextDepth].sideToMove = current.sideToMove ^ 1
	c.states[nextDepth].transition = contextTransition{}
	c.depth = nextDepth
}

// Pop removes the most recent real or null frame.
func (c *Context) Pop() error {
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

// Evaluate returns the current raw side-to-move score without refreshing.
func (c *Context) Evaluate() (int64, error) {
	if err := c.ready(); err != nil {
		return 0, err
	}
	state := &c.states[c.depth]
	return c.model.evaluateAccumulators(&state.accumulators, state.sideToMove), nil
}

// Facts derives compact adapter facts without retaining or parsing a FEN.
func Facts(position Position) (PositionFacts, error) {
	_, facts, err := boardAndFacts(position)
	return facts, err
}

func (c *Context) ready() error {
	if c == nil || c.model == nil || !c.model.loaded {
		return fmt.Errorf("%w: context has no validated model", ErrContext)
	}
	if !c.initialized {
		return fmt.Errorf("%w: context is not reset", ErrContext)
	}
	return nil
}

func (m *Model) refreshState(position Position) (contextState, [64]encodedPiece, error) {
	var state contextState
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
			state.accumulators[perspective][hidden] = int64(bias)
		}
	}
	for square, code := range board {
		if code == emptyPiece {
			continue
		}
		m.updateAccumulators(&state.accumulators, decodePiece(code, Square(square)), 1)
	}
	state.facts = facts
	state.sideToMove = position.SideToMove
	return state, board, nil
}

func (m *Model) updateAccumulators(
	accumulators *[PerspectiveCount][HiddenSize]int64,
	piece PieceOnSquare,
	direction int64,
) {
	for perspective := White; perspective <= Black; perspective++ {
		feature := featureIndexUnchecked(piece, perspective)
		for hidden, weight := range m.featureWeights[feature] {
			accumulators[perspective][hidden] += direction * int64(weight)
		}
	}
}

func boardAndFacts(position Position) ([64]encodedPiece, PositionFacts, error) {
	var board [64]encodedPiece
	var facts PositionFacts
	facts.KingSquare = [PerspectiveCount]Square{NoSquare, NoSquare}
	if position.SideToMove > Black {
		return board, facts, fmt.Errorf("%w: side to move %d", ErrContext, position.SideToMove)
	}
	if len(position.Pieces) > 64 {
		return board, facts, fmt.Errorf("%w: %d occupied squares", ErrContext, len(position.Pieces))
	}
	for _, piece := range position.Pieces {
		code, err := encodePiece(piece)
		if err != nil {
			return board, facts, err
		}
		if board[piece.Square] != emptyPiece {
			return board, facts, fmt.Errorf("%w: duplicate square %d", ErrContext, piece.Square)
		}
		board[piece.Square] = code
		bit := uint64(1) << piece.Square
		facts.Occupied |= bit
		if piece.Piece == Pawn {
			facts.Pawns[piece.Color] |= bit
		}
		if piece.Piece == King {
			if facts.KingSquare[piece.Color] != NoSquare {
				return board, facts, fmt.Errorf("%w: multiple kings for colour %d", ErrContext, piece.Color)
			}
			facts.KingSquare[piece.Color] = piece.Square
		}
	}
	return board, facts, nil
}

func encodePiece(piece PieceOnSquare) (encodedPiece, error) {
	if piece.Color > Black {
		return emptyPiece, fmt.Errorf("%w: piece colour %d", ErrContext, piece.Color)
	}
	if piece.Piece > King {
		return emptyPiece, fmt.Errorf("%w: piece type %d", ErrContext, piece.Piece)
	}
	if piece.Square >= 64 {
		return emptyPiece, fmt.Errorf("%w: square %d", ErrContext, piece.Square)
	}
	return encodedPiece(1 + int(piece.Color)*6 + int(piece.Piece)), nil
}

func decodePiece(code encodedPiece, square Square) PieceOnSquare {
	value := int(code) - 1
	return PieceOnSquare{
		Piece:  PieceType(value % 6),
		Color:  Color(value / 6),
		Square: square,
	}
}

func reconstructBefore(after [64]encodedPiece, delta Delta) ([64]encodedPiece, error) {
	before := after
	for i := 0; i < int(delta.AddedCount); i++ {
		piece := delta.Added[i]
		code, err := encodePiece(piece)
		if err != nil {
			return before, err
		}
		if before[piece.Square] != code {
			return before, fmt.Errorf("%w: added piece %d is absent from after-position", ErrContext, i)
		}
		before[piece.Square] = emptyPiece
	}
	for i := 0; i < int(delta.RemovedCount); i++ {
		piece := delta.Removed[i]
		code, err := encodePiece(piece)
		if err != nil {
			return before, err
		}
		if before[piece.Square] != emptyPiece {
			return before, fmt.Errorf("%w: removed piece %d collides while reconstructing", ErrContext, i)
		}
		before[piece.Square] = code
	}
	return before, nil
}

func applyDeltaForward(
	before [64]encodedPiece,
	beforeFacts PositionFacts,
	delta Delta,
) ([64]encodedPiece, PositionFacts, error) {
	after := before
	facts := beforeFacts
	for i := 0; i < int(delta.RemovedCount); i++ {
		piece := delta.Removed[i]
		code, err := encodePiece(piece)
		if err != nil {
			return after, facts, err
		}
		if after[piece.Square] != code {
			return after, facts, fmt.Errorf("%w: removed piece %d is absent from current position", ErrContext, i)
		}
		if err := removePieceFacts(&facts, piece); err != nil {
			return after, facts, err
		}
		after[piece.Square] = emptyPiece
	}
	for i := 0; i < int(delta.AddedCount); i++ {
		piece := delta.Added[i]
		code, err := encodePiece(piece)
		if err != nil {
			return after, facts, err
		}
		if after[piece.Square] != emptyPiece {
			return after, facts, fmt.Errorf("%w: added piece %d collides with current position", ErrContext, i)
		}
		if err := addPieceFacts(&facts, piece); err != nil {
			return after, facts, err
		}
		after[piece.Square] = code
	}
	return after, facts, nil
}

func removePieceFacts(facts *PositionFacts, piece PieceOnSquare) error {
	bit := uint64(1) << piece.Square
	if facts.Occupied&bit == 0 {
		return fmt.Errorf("%w: removed square %d is absent from before-facts", ErrContext, piece.Square)
	}
	facts.Occupied &^= bit
	if piece.Piece == Pawn {
		if facts.Pawns[piece.Color]&bit == 0 {
			return fmt.Errorf("%w: removed pawn is absent from before-facts", ErrContext)
		}
		facts.Pawns[piece.Color] &^= bit
	}
	if piece.Piece == King {
		if facts.KingSquare[piece.Color] != piece.Square {
			return fmt.Errorf("%w: removed king square disagrees with before-facts", ErrContext)
		}
		facts.KingSquare[piece.Color] = NoSquare
	}
	return nil
}

func addPieceFacts(facts *PositionFacts, piece PieceOnSquare) error {
	bit := uint64(1) << piece.Square
	if facts.Occupied&bit != 0 {
		return fmt.Errorf("%w: added square %d is occupied in intermediate facts", ErrContext, piece.Square)
	}
	facts.Occupied |= bit
	if piece.Piece == Pawn {
		if facts.Pawns[piece.Color]&bit != 0 {
			return fmt.Errorf("%w: added pawn is already present in intermediate facts", ErrContext)
		}
		facts.Pawns[piece.Color] |= bit
	}
	if piece.Piece == King {
		if facts.KingSquare[piece.Color] != NoSquare {
			return fmt.Errorf("%w: added a second king for colour %d", ErrContext, piece.Color)
		}
		facts.KingSquare[piece.Color] = piece.Square
	}
	return nil
}

func validateDeltaShape(delta Delta) error {
	if delta.RemovedCount > MaxDeltaPieces || delta.AddedCount > MaxDeltaPieces {
		return fmt.Errorf("%w: delta piece count exceeds %d", ErrContext, MaxDeltaPieces)
	}
	for i := 0; i < int(delta.RemovedCount); i++ {
		if _, err := encodePiece(delta.Removed[i]); err != nil {
			return err
		}
	}
	for i := 0; i < int(delta.AddedCount); i++ {
		if _, err := encodePiece(delta.Added[i]); err != nil {
			return err
		}
	}

	switch delta.Kind {
	case MoveNormal:
		if delta.RemovedCount != 1 || delta.AddedCount != 1 ||
			!samePiece(delta.Removed[0], delta.Added[0]) {
			return badDeltaKind(delta.Kind)
		}
	case MoveCapture:
		if delta.RemovedCount != 2 || delta.AddedCount != 1 ||
			!samePiece(delta.Removed[0], delta.Added[0]) ||
			delta.Removed[1].Color == delta.Removed[0].Color ||
			delta.Removed[1].Square != delta.Added[0].Square {
			return badDeltaKind(delta.Kind)
		}
	case MoveEnPassant:
		if delta.RemovedCount != 2 || delta.AddedCount != 1 ||
			!samePiece(delta.Removed[0], delta.Added[0]) ||
			delta.Removed[0].Piece != Pawn || delta.Removed[1].Piece != Pawn ||
			delta.Removed[1].Color == delta.Removed[0].Color ||
			delta.Removed[1].Square == delta.Added[0].Square {
			return badDeltaKind(delta.Kind)
		}
	case MoveCastle:
		if delta.RemovedCount != 2 || delta.AddedCount != 2 ||
			delta.Removed[0].Piece != King || delta.Added[0].Piece != King ||
			delta.Removed[1].Piece != Rook || delta.Added[1].Piece != Rook ||
			!samePiece(delta.Removed[0], delta.Added[0]) ||
			!samePiece(delta.Removed[1], delta.Added[1]) ||
			delta.Removed[0].Color != delta.Removed[1].Color {
			return badDeltaKind(delta.Kind)
		}
	case MovePromotion:
		if delta.RemovedCount != 1 || delta.AddedCount != 1 ||
			!validPromotion(delta.Removed[0], delta.Added[0]) {
			return badDeltaKind(delta.Kind)
		}
	case MovePromotionCapture:
		if delta.RemovedCount != 2 || delta.AddedCount != 1 ||
			!validPromotion(delta.Removed[0], delta.Added[0]) ||
			delta.Removed[1].Color == delta.Removed[0].Color ||
			delta.Removed[1].Square != delta.Added[0].Square {
			return badDeltaKind(delta.Kind)
		}
	default:
		return fmt.Errorf("%w: unknown move kind %d", ErrContext, delta.Kind)
	}
	if delta.Removed[0].Square == delta.Added[0].Square {
		return fmt.Errorf("%w: mover source equals destination", ErrContext)
	}
	return nil
}

func samePiece(before, after PieceOnSquare) bool {
	return before.Piece == after.Piece && before.Color == after.Color
}

func validPromotion(before, after PieceOnSquare) bool {
	return before.Piece == Pawn && after.Piece >= Knight && after.Piece <= Queen && before.Color == after.Color
}

func badDeltaKind(kind MoveKind) error {
	return fmt.Errorf("%w: pieces do not match move kind %d", ErrContext, kind)
}
