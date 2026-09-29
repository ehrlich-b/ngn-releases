package sf18big

import (
	"errors"
	"fmt"
	"math/bits"

	base "github.com/ehrlich-b/ngn/nnue"
)

var ErrContext = errors.New("invalid Stockfish 18 BIG context operation")

// With exact dirty-threat transitions, replay wins through two dirty plies on
// the pinned representative corpus; three plies still cost more than rebuilding
// the current position. Keep the cutoff tied to measured end-to-end context cost.
const maxIncrementalDonorPlies = 2

type noCopy struct{}

func (*noCopy) Lock()   {}
func (*noCopy) Unlock() {}

type contextTransition struct {
	removedCount uint8
	addedCount   uint8
	removed      [base.MaxDeltaPieces]base.PieceOnSquare
	added        [base.MaxDeltaPieces]base.PieceOnSquare
}

func (t contextTransition) delta() base.Delta {
	return base.Delta{
		RemovedCount: t.removedCount,
		AddedCount:   t.addedCount,
		Removed:      t.removed,
		Added:        t.added,
	}
}

func (t contextTransition) changedSquares() uint64 {
	var changed uint64
	for index := 0; index < int(t.removedCount); index++ {
		changed |= uint64(1) << t.removed[index].Square
	}
	for index := 0; index < int(t.addedCount); index++ {
		changed |= uint64(1) << t.added[index].Square
	}
	return changed
}

type contextFrame struct {
	state          ReferenceState
	facts          base.PositionFacts
	pieces         [2][6]uint64
	sideToMove     base.Color
	transition     contextTransition
	baseComputed   [2]bool
	threatComputed [2]bool
}

// ContextStats exposes exact demand-driven work performed since Reset. Pushes
// count both real and null frames; incremental counts are materialized frames,
// not calls to EvaluateSelected.
type ContextStats struct {
	Pushes                  uint64
	Evaluations             uint64
	AdaptiveFullRefreshes   uint64
	BaseRefreshes           [2]uint64
	ThreatRefreshes         [2]uint64
	BaseIncrementalFrames   [2]uint64
	ThreatIncrementalFrames [2]uint64
}

// Context owns one worker's mutable BIG accumulator stack. Push stores a dirty
// transition and never copies the numeric accumulator. EvaluateSelected finds
// a usable donor (or the nearest refresh boundary), materializes the missing
// path, and retains it for later descendants. A Context must not be copied or
// shared; its immutable Model may be shared by workers.
type Context struct {
	noCopy      noCopy
	model       *Model
	frames      [base.MaxContextPly + 1]contextFrame
	board       [64]boardPiece
	depth       uint16
	initialized bool
	stats       ContextStats
}

func NewContext(model *Model) (*Context, error) {
	if model == nil || !model.loaded {
		return nil, fmt.Errorf("%w: model was not returned by Load", ErrContext)
	}
	return &Context{model: model}, nil
}

func (c *Context) Depth() int {
	if c == nil {
		return 0
	}
	return int(c.depth)
}

func (c *Context) Facts() (base.PositionFacts, error) {
	if err := c.ready(); err != nil {
		return base.PositionFacts{}, err
	}
	return c.frames[c.depth].facts, nil
}

func (c *Context) Stats() (ContextStats, error) {
	if err := c.ready(); err != nil {
		return ContextStats{}, err
	}
	return c.stats, nil
}

// Reset transactionally installs a fully-computed root.
func (c *Context) Reset(position base.Position) error {
	if c == nil || c.model == nil || !c.model.loaded {
		return fmt.Errorf("%w: context has no validated model", ErrContext)
	}
	state, _, err := c.model.ReferenceRefresh(position)
	if err != nil {
		return err
	}
	board, facts, err := contextBoardAndFacts(position)
	if err != nil {
		return err
	}
	c.frames[0] = contextFrame{
		state:          state,
		facts:          facts,
		pieces:         boardPieceBitboards(&board),
		sideToMove:     position.SideToMove,
		baseComputed:   [2]bool{true, true},
		threatComputed: [2]bool{true, true},
	}
	c.board = board
	c.depth = 0
	c.initialized = true
	c.stats = ContextStats{
		BaseRefreshes:   [2]uint64{1, 1},
		ThreatRefreshes: [2]uint64{1, 1},
	}
	return nil
}

// Push validates a real transition against the complete after-position. It
// changes no live state unless the board and all compact facts agree.
func (c *Context) Push(delta base.Delta, afterPosition base.Position) error {
	if err := c.readyForPush(); err != nil {
		return err
	}
	current := &c.frames[c.depth]
	if afterPosition.SideToMove != current.sideToMove^1 {
		return fmt.Errorf("%w: real move did not toggle side", ErrContext)
	}
	if err := validateContextDelta(delta); err != nil {
		return err
	}
	if delta.Removed[0].Color != current.sideToMove {
		return fmt.Errorf("%w: mover colour does not match side to move", ErrContext)
	}
	if delta.Before != current.facts {
		return fmt.Errorf("%w: delta before-facts mismatch", ErrContext)
	}
	afterBoard, afterFacts, err := contextBoardAndFacts(afterPosition)
	if err != nil {
		return err
	}
	if delta.After != afterFacts {
		return fmt.Errorf("%w: delta after-facts mismatch", ErrContext)
	}
	reconstructed, err := reconstructContextBefore(afterBoard, delta)
	if err != nil {
		return err
	}
	if reconstructed != c.board {
		return fmt.Errorf("%w: after-position plus delta does not reconstruct current board", ErrContext)
	}
	c.commitReal(delta, afterBoard, afterFacts)
	return nil
}

// PushDelta applies a complete semantic transition without scanning a public
// position. All checks are staged before the context is changed.
func (c *Context) PushDelta(delta base.Delta) error {
	if err := c.readyForPush(); err != nil {
		return err
	}
	current := &c.frames[c.depth]
	if err := validateContextDelta(delta); err != nil {
		return err
	}
	if delta.Removed[0].Color != current.sideToMove {
		return fmt.Errorf("%w: mover colour does not match side to move", ErrContext)
	}
	if delta.Before != current.facts {
		return fmt.Errorf("%w: delta before-facts mismatch", ErrContext)
	}
	afterBoard, afterFacts, err := applyContextDelta(c.board, current.facts, delta)
	if err != nil {
		return err
	}
	if delta.After != afterFacts {
		return fmt.Errorf("%w: delta after-facts mismatch", ErrContext)
	}
	c.commitReal(delta, afterBoard, afterFacts)
	return nil
}

func (c *Context) commitReal(delta base.Delta, afterBoard [64]boardPiece, afterFacts base.PositionFacts) {
	nextDepth := c.depth + 1
	next := &c.frames[nextDepth]
	next.facts = afterFacts
	next.pieces = c.frames[c.depth].pieces
	for index := 0; index < int(delta.RemovedCount); index++ {
		piece := delta.Removed[index]
		next.pieces[piece.Color][piece.Piece] &^= uint64(1) << piece.Square
	}
	for index := 0; index < int(delta.AddedCount); index++ {
		piece := delta.Added[index]
		next.pieces[piece.Color][piece.Piece] |= uint64(1) << piece.Square
	}
	next.sideToMove = c.frames[c.depth].sideToMove ^ 1
	next.transition = contextTransition{
		removedCount: delta.RemovedCount,
		addedCount:   delta.AddedCount,
		removed:      delta.Removed,
		added:        delta.Added,
	}
	next.baseComputed = [2]bool{}
	next.threatComputed = [2]bool{}
	c.board = afterBoard
	c.depth = nextDepth
	c.stats.Pushes++
}

// PushNull validates a board-identical null frame. The numeric state is left
// dirty, so even null push is independent of accumulator size.
func (c *Context) PushNull(afterPosition base.Position) error {
	if err := c.readyForPush(); err != nil {
		return err
	}
	current := &c.frames[c.depth]
	if afterPosition.SideToMove != current.sideToMove^1 {
		return fmt.Errorf("%w: null move did not toggle side", ErrContext)
	}
	afterBoard, afterFacts, err := contextBoardAndFacts(afterPosition)
	if err != nil {
		return err
	}
	if afterBoard != c.board || afterFacts != current.facts {
		return fmt.Errorf("%w: null move changed board features", ErrContext)
	}
	c.commitNull()
	return nil
}

func (c *Context) PushNullFacts(after base.PositionFacts) error {
	if err := c.readyForPush(); err != nil {
		return err
	}
	if after != c.frames[c.depth].facts {
		return fmt.Errorf("%w: null move changed board features", ErrContext)
	}
	c.commitNull()
	return nil
}

func (c *Context) commitNull() {
	nextDepth := c.depth + 1
	next := &c.frames[nextDepth]
	next.facts = c.frames[c.depth].facts
	next.pieces = c.frames[c.depth].pieces
	next.sideToMove = c.frames[c.depth].sideToMove ^ 1
	next.transition = contextTransition{}
	next.baseComputed = [2]bool{}
	next.threatComputed = [2]bool{}
	c.depth = nextDepth
	c.stats.Pushes++
}

func (c *Context) Pop() error {
	if err := c.ready(); err != nil {
		return err
	}
	if c.depth == 0 {
		return fmt.Errorf("%w: stack underflow", ErrContext)
	}
	before, err := reconstructContextBefore(c.board, c.frames[c.depth].transition.delta())
	if err != nil {
		return fmt.Errorf("%w: corrupt stored transition: %v", ErrContext, err)
	}
	c.board = before
	c.depth--
	return nil
}

// EvaluateSelected materializes only missing accumulator paths and propagates
// the current material-selected output head.
func (c *Context) EvaluateSelected() (SelectedEvaluation, error) {
	if err := c.ready(); err != nil {
		return SelectedEvaluation{}, err
	}
	if c.shouldRefreshCurrent() {
		if err := c.refreshCurrent(); err != nil {
			return SelectedEvaluation{}, err
		}
	}
	for perspective := base.White; perspective <= base.Black; perspective++ {
		if err := c.ensureBase(perspective); err != nil {
			return SelectedEvaluation{}, err
		}
		if err := c.ensureThreats(perspective); err != nil {
			return SelectedEvaluation{}, err
		}
	}
	current := &c.frames[c.depth]
	c.stats.Evaluations++
	return c.model.evaluateSelectedState(
		&current.state,
		current.sideToMove,
		bits.OnesCount64(current.facts.Occupied),
	), nil
}

func (c *Context) shouldRefreshCurrent() bool {
	for depth, distance := c.depth, 0; ; depth, distance = depth-1, distance+1 {
		frame := &c.frames[depth]
		if frame.baseComputed == [2]bool{true, true} &&
			frame.threatComputed == [2]bool{true, true} {
			return false
		}
		if distance >= maxIncrementalDonorPlies || depth == 0 {
			return true
		}
	}
}

func (c *Context) refreshCurrent() error {
	frame := &c.frames[c.depth]
	for perspective := base.White; perspective <= base.Black; perspective++ {
		p := int(perspective)
		c.model.refreshBasePerspective(&frame.state, c.board, perspective, frame.facts.KingSquare[p])
		if err := c.model.refreshThreatPerspective(&frame.state, c.board, perspective, frame.facts); err != nil {
			return err
		}
		frame.baseComputed[p] = true
		frame.threatComputed[p] = true
		c.stats.BaseRefreshes[p]++
		c.stats.ThreatRefreshes[p]++
	}
	c.stats.AdaptiveFullRefreshes++
	return nil
}

func (c *Context) ensureBase(perspective base.Color) error {
	p := int(perspective)
	if c.frames[c.depth].baseComputed[p] {
		return nil
	}
	start, refresh := c.findBaseStart(perspective)
	board, err := c.boardAt(start)
	if err != nil {
		return err
	}
	if refresh {
		c.model.refreshBasePerspective(&c.frames[start].state, board, perspective, c.frames[start].facts.KingSquare[p])
		c.frames[start].baseComputed[p] = true
		c.stats.BaseRefreshes[p]++
	}
	for depth := start + 1; depth <= c.depth; depth++ {
		frame := &c.frames[depth]
		nextBoard, _, err := applyContextDelta(board, c.frames[depth-1].facts, frame.transition.delta())
		if err != nil {
			return fmt.Errorf("%w: corrupt stored transition at depth %d: %v", ErrContext, depth, err)
		}
		previous := &c.frames[depth-1]
		frame.state.BaseValues[p] = previous.state.BaseValues[p]
		frame.state.BasePSQT[p] = previous.state.BasePSQT[p]
		king := frame.facts.KingSquare[p]
		for index := 0; index < int(frame.transition.removedCount); index++ {
			c.model.updateBasePiece(&frame.state, perspective, frame.transition.removed[index], king, false)
		}
		for index := 0; index < int(frame.transition.addedCount); index++ {
			c.model.updateBasePiece(&frame.state, perspective, frame.transition.added[index], king, true)
		}
		frame.baseComputed[p] = true
		c.stats.BaseIncrementalFrames[p]++
		board = nextBoard
	}
	return nil
}

func (c *Context) ensureThreats(perspective base.Color) error {
	p := int(perspective)
	if c.frames[c.depth].threatComputed[p] {
		return nil
	}
	start, refresh := c.findThreatStart(perspective)
	board, err := c.boardAt(start)
	if err != nil {
		return err
	}
	if refresh {
		if err := c.model.refreshThreatPerspective(&c.frames[start].state, board, perspective, c.frames[start].facts); err != nil {
			return err
		}
		c.frames[start].threatComputed[p] = true
		c.stats.ThreatRefreshes[p]++
	}
	for depth := start + 1; depth <= c.depth; depth++ {
		frame := &c.frames[depth]
		nextBoard, _, err := applyContextDelta(board, c.frames[depth-1].facts, frame.transition.delta())
		if err != nil {
			return fmt.Errorf("%w: corrupt stored transition at depth %d: %v", ErrContext, depth, err)
		}
		previous := &c.frames[depth-1]
		removed, added, err := dirtyThreatIndexDiff(
			&board,
			&nextBoard,
			previous.pieces,
			frame.pieces,
			previous.facts.Occupied,
			frame.facts.Occupied,
			frame.transition.changedSquares(),
			frame.facts.KingSquare[p],
			perspective,
		)
		if err != nil {
			return err
		}
		frame.state.ThreatValues[p] = previous.state.ThreatValues[p]
		frame.state.ThreatPSQT[p] = previous.state.ThreatPSQT[p]
		for _, feature := range removed.Indices[:removed.Count] {
			c.model.updateThreatFeature(&frame.state.ThreatValues[p], &frame.state.ThreatPSQT[p], feature, false)
		}
		for _, feature := range added.Indices[:added.Count] {
			c.model.updateThreatFeature(&frame.state.ThreatValues[p], &frame.state.ThreatPSQT[p], feature, true)
		}
		frame.state.Threats[p] = previous.state.Threats[p]
		applyThreatIndexDiffInPlace(&frame.state.Threats[p], ThreatDiff{
			Removed: removed,
			Added:   added,
		})
		frame.threatComputed[p] = true
		c.stats.ThreatIncrementalFrames[p]++
		board = nextBoard
	}
	return nil
}

func (c *Context) findBaseStart(perspective base.Color) (uint16, bool) {
	p := int(perspective)
	for depth := c.depth; depth > 0; depth-- {
		if c.frames[depth].baseComputed[p] {
			return depth, false
		}
		if c.frames[depth].facts.KingSquare[p] != c.frames[depth-1].facts.KingSquare[p] {
			return depth, true
		}
	}
	return 0, false
}

func (c *Context) findThreatStart(perspective base.Color) (uint16, bool) {
	p := int(perspective)
	for depth := c.depth; depth > 0; depth-- {
		if c.frames[depth].threatComputed[p] {
			return depth, false
		}
		before := c.frames[depth-1].facts.KingSquare[p]
		after := c.frames[depth].facts.KingSquare[p]
		if before&4 != after&4 {
			return depth, true
		}
	}
	return 0, false
}

func (c *Context) boardAt(depth uint16) ([64]boardPiece, error) {
	board := c.board
	for cursor := c.depth; cursor > depth; cursor-- {
		var err error
		board, err = reconstructContextBefore(board, c.frames[cursor].transition.delta())
		if err != nil {
			return board, fmt.Errorf("%w: corrupt stored transition at depth %d: %v", ErrContext, cursor, err)
		}
	}
	return board, nil
}

func (c *Context) ready() error {
	if c == nil || c.model == nil || !c.model.loaded {
		return fmt.Errorf("%w: context has no validated model", ErrContext)
	}
	if !c.initialized {
		return fmt.Errorf("%w: context was not reset", ErrContext)
	}
	return nil
}

func (c *Context) readyForPush() error {
	if err := c.ready(); err != nil {
		return err
	}
	if c.depth >= base.MaxContextPly {
		return fmt.Errorf("%w: stack overflow at depth %d", ErrContext, c.depth)
	}
	return nil
}

func (m *Model) refreshBasePerspective(
	state *ReferenceState,
	board [64]boardPiece,
	perspective base.Color,
	king base.Square,
) {
	p := int(perspective)
	copy(state.BaseValues[p][:], m.featureBias[:])
	state.BasePSQT[p] = [psqtBuckets]int32{}
	for square, piece := range board {
		if !piece.set {
			continue
		}
		feature := baseFeatureIndex(piece, base.Square(square), king, perspective)
		m.updateBaseFeature(&state.BaseValues[p], &state.BasePSQT[p], feature, true)
	}
}

func (m *Model) updateBasePiece(
	state *ReferenceState,
	perspective base.Color,
	piece base.PieceOnSquare,
	king base.Square,
	add bool,
) {
	entry := boardPiece{piece: piece.Piece, color: piece.Color, set: true}
	feature := baseFeatureIndex(entry, piece.Square, king, perspective)
	p := int(perspective)
	m.updateBaseFeature(&state.BaseValues[p], &state.BasePSQT[p], feature, add)
}

func (m *Model) refreshThreatPerspective(
	state *ReferenceState,
	board [64]boardPiece,
	perspective base.Color,
	facts base.PositionFacts,
) error {
	p := int(perspective)
	state.ThreatValues[p] = [transformerLanes]int16{}
	state.ThreatPSQT[p] = [psqtBuckets]int32{}
	active, err := activeThreatsBoard(&board, facts.KingSquare[p], facts.Occupied, perspective)
	if err != nil {
		return err
	}
	state.Threats[p] = active
	for _, feature := range active.Indices[:active.Count] {
		m.updateThreatFeature(&state.ThreatValues[p], &state.ThreatPSQT[p], feature, true)
	}
	return nil
}

func contextBoardAndFacts(position base.Position) ([64]boardPiece, base.PositionFacts, error) {
	board, kings, occupied, _, err := validatePosition(position)
	facts := base.PositionFacts{KingSquare: [2]base.Square{base.NoSquare, base.NoSquare}}
	if err != nil {
		return board, facts, err
	}
	facts.Occupied = occupied
	facts.KingSquare = kings
	for square, piece := range board {
		if piece.set && piece.piece == base.Pawn {
			facts.Pawns[piece.color] |= uint64(1) << square
		}
	}
	return board, facts, nil
}

func contextPiece(piece base.PieceOnSquare) (boardPiece, error) {
	if piece.Color > base.Black || piece.Piece > base.King || piece.Square >= 64 {
		return boardPiece{}, fmt.Errorf("%w: invalid piece %+v", ErrContext, piece)
	}
	return boardPiece{piece: piece.Piece, color: piece.Color, set: true}, nil
}

func reconstructContextBefore(after [64]boardPiece, delta base.Delta) ([64]boardPiece, error) {
	before := after
	for index := 0; index < int(delta.AddedCount); index++ {
		piece := delta.Added[index]
		entry, err := contextPiece(piece)
		if err != nil {
			return before, err
		}
		if before[piece.Square] != entry {
			return before, fmt.Errorf("%w: added piece %d is absent from after-position", ErrContext, index)
		}
		before[piece.Square] = boardPiece{}
	}
	for index := 0; index < int(delta.RemovedCount); index++ {
		piece := delta.Removed[index]
		entry, err := contextPiece(piece)
		if err != nil {
			return before, err
		}
		if before[piece.Square].set {
			return before, fmt.Errorf("%w: removed piece %d collides while reconstructing", ErrContext, index)
		}
		before[piece.Square] = entry
	}
	return before, nil
}

func applyContextDelta(
	before [64]boardPiece,
	beforeFacts base.PositionFacts,
	delta base.Delta,
) ([64]boardPiece, base.PositionFacts, error) {
	after := before
	facts := beforeFacts
	for index := 0; index < int(delta.RemovedCount); index++ {
		piece := delta.Removed[index]
		entry, err := contextPiece(piece)
		if err != nil {
			return after, facts, err
		}
		if after[piece.Square] != entry {
			return after, facts, fmt.Errorf("%w: removed piece %d is absent from current position", ErrContext, index)
		}
		if err := removeContextFacts(&facts, piece); err != nil {
			return after, facts, err
		}
		after[piece.Square] = boardPiece{}
	}
	for index := 0; index < int(delta.AddedCount); index++ {
		piece := delta.Added[index]
		entry, err := contextPiece(piece)
		if err != nil {
			return after, facts, err
		}
		if after[piece.Square].set {
			return after, facts, fmt.Errorf("%w: added piece %d collides with current position", ErrContext, index)
		}
		if err := addContextFacts(&facts, piece); err != nil {
			return after, facts, err
		}
		after[piece.Square] = entry
	}
	return after, facts, nil
}

func removeContextFacts(facts *base.PositionFacts, piece base.PieceOnSquare) error {
	bit := uint64(1) << piece.Square
	if facts.Occupied&bit == 0 {
		return fmt.Errorf("%w: removed square %d absent from facts", ErrContext, piece.Square)
	}
	facts.Occupied &^= bit
	if piece.Piece == base.Pawn {
		if facts.Pawns[piece.Color]&bit == 0 {
			return fmt.Errorf("%w: removed pawn absent from facts", ErrContext)
		}
		facts.Pawns[piece.Color] &^= bit
	}
	if piece.Piece == base.King {
		if facts.KingSquare[piece.Color] != piece.Square {
			return fmt.Errorf("%w: removed king square disagrees with facts", ErrContext)
		}
		facts.KingSquare[piece.Color] = base.NoSquare
	}
	return nil
}

func addContextFacts(facts *base.PositionFacts, piece base.PieceOnSquare) error {
	bit := uint64(1) << piece.Square
	if facts.Occupied&bit != 0 {
		return fmt.Errorf("%w: added square %d occupied in facts", ErrContext, piece.Square)
	}
	facts.Occupied |= bit
	if piece.Piece == base.Pawn {
		facts.Pawns[piece.Color] |= bit
	}
	if piece.Piece == base.King {
		if facts.KingSquare[piece.Color] != base.NoSquare {
			return fmt.Errorf("%w: added second king for colour %d", ErrContext, piece.Color)
		}
		facts.KingSquare[piece.Color] = piece.Square
	}
	return nil
}

func validateContextDelta(delta base.Delta) error {
	if delta.RemovedCount > base.MaxDeltaPieces || delta.AddedCount > base.MaxDeltaPieces {
		return fmt.Errorf("%w: delta piece count exceeds %d", ErrContext, base.MaxDeltaPieces)
	}
	for index := 0; index < int(delta.RemovedCount); index++ {
		if _, err := contextPiece(delta.Removed[index]); err != nil {
			return err
		}
	}
	for index := 0; index < int(delta.AddedCount); index++ {
		if _, err := contextPiece(delta.Added[index]); err != nil {
			return err
		}
	}
	switch delta.Kind {
	case base.MoveNormal:
		if delta.RemovedCount != 1 || delta.AddedCount != 1 || !sameContextPiece(delta.Removed[0], delta.Added[0]) {
			return badContextDelta(delta.Kind)
		}
	case base.MoveCapture:
		if delta.RemovedCount != 2 || delta.AddedCount != 1 || !sameContextPiece(delta.Removed[0], delta.Added[0]) || delta.Removed[1].Piece == base.King || delta.Removed[1].Color == delta.Removed[0].Color || delta.Removed[1].Square != delta.Added[0].Square {
			return badContextDelta(delta.Kind)
		}
	case base.MoveEnPassant:
		if delta.RemovedCount != 2 || delta.AddedCount != 1 || !sameContextPiece(delta.Removed[0], delta.Added[0]) || delta.Removed[0].Piece != base.Pawn || delta.Removed[1].Piece != base.Pawn || delta.Removed[1].Color == delta.Removed[0].Color || delta.Removed[1].Square == delta.Added[0].Square {
			return badContextDelta(delta.Kind)
		}
	case base.MoveCastle:
		if delta.RemovedCount != 2 || delta.AddedCount != 2 || delta.Removed[0].Piece != base.King || delta.Added[0].Piece != base.King || delta.Removed[1].Piece != base.Rook || delta.Added[1].Piece != base.Rook || !sameContextPiece(delta.Removed[0], delta.Added[0]) || !sameContextPiece(delta.Removed[1], delta.Added[1]) || delta.Removed[0].Color != delta.Removed[1].Color {
			return badContextDelta(delta.Kind)
		}
	case base.MovePromotion:
		if delta.RemovedCount != 1 || delta.AddedCount != 1 || !validContextPromotion(delta.Removed[0], delta.Added[0]) {
			return badContextDelta(delta.Kind)
		}
	case base.MovePromotionCapture:
		if delta.RemovedCount != 2 || delta.AddedCount != 1 || !validContextPromotion(delta.Removed[0], delta.Added[0]) || delta.Removed[1].Piece == base.King || delta.Removed[1].Color == delta.Removed[0].Color || delta.Removed[1].Square != delta.Added[0].Square {
			return badContextDelta(delta.Kind)
		}
	default:
		return fmt.Errorf("%w: unknown move kind %d", ErrContext, delta.Kind)
	}
	if delta.Removed[0].Square == delta.Added[0].Square {
		return fmt.Errorf("%w: mover source equals destination", ErrContext)
	}
	return nil
}

func sameContextPiece(before, after base.PieceOnSquare) bool {
	return before.Piece == after.Piece && before.Color == after.Color
}

func validContextPromotion(before, after base.PieceOnSquare) bool {
	return before.Piece == base.Pawn && after.Piece >= base.Knight && after.Piece <= base.Queen && before.Color == after.Color
}

func badContextDelta(kind base.MoveKind) error {
	return fmt.Errorf("%w: pieces do not match move kind %d", ErrContext, kind)
}
