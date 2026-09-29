package sf18small

import (
    "errors"
    "fmt"
    "math/bits"
    "unsafe"

    base "github.com/ehrlich-b/ngn/nnue"
)

const initialContextFrameCapacity = 128

var ErrContext = errors.New("invalid Stockfish 18 SMALL context operation")

type noCopy struct{}

func (*noCopy) Lock()   {}
func (*noCopy) Unlock() {}

type contextTransition struct {
    removedCount uint8
    addedCount   uint8
    removed      [base.MaxDeltaPieces]base.PieceOnSquare
    added        [base.MaxDeltaPieces]base.PieceOnSquare
}

type contextFrame struct {
    values     [2][transformerLanes]int16
    psqt       [2][psqtBuckets]int32
    facts      base.PositionFacts
    sideToMove base.Color
    transition contextTransition
}

// Context owns one worker's mutable incremental SMALL state. A Context must not
// be copied after first use or called concurrently. Its Model is immutable and
// may be shared by any number of worker-private Contexts.
type Context struct {
    noCopy noCopy
    model  *Model
    board  [64]boardPiece
    frames []contextFrame
}

// NewContext binds a validated immutable model to a new worker-private context.
func NewContext(model *Model) (*Context, error) {
    if model == nil || !model.loaded {
        return nil, fmt.Errorf("%w: model was not returned by Load", ErrContext)
    }
    return &Context{
        model:  model,
        frames: make([]contextFrame, 0, initialContextFrameCapacity),
    }, nil
}

// Depth is the number of live real and null transition frames.
func (c *Context) Depth() int {
    if c == nil || len(c.frames) == 0 {
        return 0
    }
    return len(c.frames) - 1
}

// Reset transactionally replaces the stack root with a full refresh.
func (c *Context) Reset(position base.Position) error {
    if c == nil || c.model == nil || !c.model.loaded {
        return fmt.Errorf("%w: context has no validated model", ErrContext)
    }
    trace, err := c.model.evaluateAllTrace(position)
    if err != nil {
        return err
    }
    board, facts, err := contextBoardAndFacts(position)
    if err != nil {
        return err
    }
    root := contextFrame{
        values:     trace.accumulator.values,
        psqt:       trace.accumulator.psqt,
        facts:      facts,
        sideToMove: position.SideToMove,
    }
    if cap(c.frames) == 0 {
        c.frames = make([]contextFrame, 1, initialContextFrameCapacity)
    } else {
        c.frames = c.frames[:1]
    }
    c.frames[0] = root
    c.board = board
    return nil
}

// Push validates a real transition against the complete after-position. It
// changes no live state unless the board, facts and accumulator update all pass.
func (c *Context) Push(delta base.Delta, afterPosition base.Position) error {
    if err := c.ready(); err != nil {
        return err
    }
    current := c.frames[len(c.frames)-1]
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
    return c.commitReal(current, delta, afterBoard, afterFacts)
}

// PushDelta applies a complete semantic delta without scanning a public
// position. Capacity growth can allocate; calls below existing capacity do not.
func (c *Context) PushDelta(delta base.Delta) error {
    if err := c.ready(); err != nil {
        return err
    }
    current := c.frames[len(c.frames)-1]
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
    return c.commitReal(current, delta, afterBoard, afterFacts)
}

func (c *Context) commitReal(current contextFrame, delta base.Delta, afterBoard [64]boardPiece, afterFacts base.PositionFacts) error {
    next := current
    for perspective := base.White; perspective <= base.Black; perspective++ {
        p := int(perspective)
        if delta.Removed[0].Piece == base.King && delta.Removed[0].Color == perspective {
            var refreshed accumulatorTrace
            c.model.refreshPerspective(afterBoard, afterFacts.KingSquare[p], perspective, &refreshed)
            next.values[p] = refreshed.values[p]
            next.psqt[p] = refreshed.psqt[p]
            continue
        }
        king := current.facts.KingSquare[p]
        for index := 0; index < int(delta.RemovedCount); index++ {
            c.model.updatePerspective(&next.values[p], &next.psqt[p], delta.Removed[index], king, perspective, false)
        }
        for index := 0; index < int(delta.AddedCount); index++ {
            c.model.updatePerspective(&next.values[p], &next.psqt[p], delta.Added[index], king, perspective, true)
        }
    }
    next.facts = afterFacts
    next.sideToMove = current.sideToMove ^ 1
    next.transition = contextTransition{
        removedCount: delta.RemovedCount,
        addedCount:   delta.AddedCount,
        removed:      delta.Removed,
        added:        delta.Added,
    }
    if err := c.appendFrame(next); err != nil {
        return err
    }
    c.board = afterBoard
    return nil
}

func (m *Model) updatePerspective(values *[transformerLanes]int16, psqt *[psqtBuckets]int32, piece base.PieceOnSquare, king base.Square, perspective base.Color, add bool) {
    entry := boardPiece{piece: piece.Piece, color: piece.Color, set: true}
    feature := featureIndex(entry, piece.Square, king, perspective)
    weightOffset := feature * transformerLanes
    for lane := 0; lane < transformerLanes; lane++ {
        weight := m.featureWeights[weightOffset+lane]
        if add {
            values[lane] = wrapAdd16(values[lane], weight)
        } else {
            values[lane] = wrapSub16(values[lane], weight)
        }
    }
    psqtOffset := feature * psqtBuckets
    for bucket := 0; bucket < psqtBuckets; bucket++ {
        weight := m.psqtWeights[psqtOffset+bucket]
        if add {
            psqt[bucket] = wrapAdd32(psqt[bucket], weight)
        } else {
            psqt[bucket] = wrapSub32(psqt[bucket], weight)
        }
    }
}

// PushNull validates that every board feature is unchanged and only side to
// move flips. The new frame is accumulator-identical to its parent.
func (c *Context) PushNull(afterPosition base.Position) error {
    if err := c.ready(); err != nil {
        return err
    }
    current := c.frames[len(c.frames)-1]
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
    next := current
    next.sideToMove ^= 1
    next.transition = contextTransition{}
    return c.appendFrame(next)
}

// Pop restores the preceding real or null frame exactly.
func (c *Context) Pop() error {
    if err := c.ready(); err != nil {
        return err
    }
    if len(c.frames) == 1 {
        return fmt.Errorf("%w: stack underflow", ErrContext)
    }
    transition := c.frames[len(c.frames)-1].transition
    before, err := reconstructContextBefore(c.board, base.Delta{
        RemovedCount: transition.removedCount,
        AddedCount:   transition.addedCount,
        Removed:      transition.removed,
        Added:        transition.added,
    })
    if err != nil {
        return fmt.Errorf("%w: corrupt stored transition: %v", ErrContext, err)
    }
    c.board = before
    c.frames = c.frames[:len(c.frames)-1]
    return nil
}

// EvaluateAll returns raw PSQT and positional components for every stack from
// the current incremental state. Search score adaptation remains outside.
func (c *Context) EvaluateAll() (Trace, error) {
    if err := c.ready(); err != nil {
        return Trace{}, err
    }
    current := c.frames[len(c.frames)-1]
    trace := evaluationTrace{}
    trace.accumulator.values = current.values
    trace.accumulator.psqt = current.psqt
    trace = c.model.completeEvaluationTrace(trace, current.sideToMove, bits.OnesCount64(current.facts.Occupied))
    return trace.public, nil
}

// EvaluateSelected propagates only the material-selected layer stack from the
// current incremental state. EvaluateAll remains the diagnostic oracle.
func (c *Context) EvaluateSelected() (SelectedEvaluation, error) {
    if err := c.ready(); err != nil {
        return SelectedEvaluation{}, err
    }
    current := c.frames[len(c.frames)-1]
    return c.model.evaluateSelectedAccumulator(
        &current.values,
        &current.psqt,
        current.sideToMove,
        bits.OnesCount64(current.facts.Occupied),
    ), nil
}

func (c *Context) ready() error {
    if c == nil || c.model == nil || !c.model.loaded {
        return fmt.Errorf("%w: context has no validated model", ErrContext)
    }
    if len(c.frames) == 0 {
        return fmt.Errorf("%w: context was not reset", ErrContext)
    }
    return nil
}

func (c *Context) appendFrame(frame contextFrame) error {
    required, err := checkedNextContextLength(len(c.frames))
    if err != nil {
        return err
    }
    if required > cap(c.frames) {
        capacity, err := nextContextCapacity(cap(c.frames), required)
        if err != nil {
            return err
        }
        grown := make([]contextFrame, len(c.frames), capacity)
        copy(grown, c.frames)
        c.frames = grown
    }
    c.frames = c.frames[:required]
    c.frames[required-1] = frame
    return nil
}

func maxContextFrameCount() int {
    return int(^uint(0)>>1) / int(unsafe.Sizeof(contextFrame{}))
}

func checkedNextContextLength(length int) (int, error) {
    maximum := maxContextFrameCount()
    if length < 0 || length >= maximum {
        return 0, fmt.Errorf("%w: frame stack length overflow", ErrContext)
    }
    return length + 1, nil
}

func nextContextCapacity(current, required int) (int, error) {
    maximum := maxContextFrameCount()
    if current < 0 || required < 0 || required > maximum {
        return 0, fmt.Errorf("%w: invalid frame stack capacity", ErrContext)
    }
    if current >= required {
        return current, nil
    }
    capacity := current
    if capacity < initialContextFrameCapacity {
        capacity = initialContextFrameCapacity
    }
    for capacity < required {
        if capacity > maximum/2 {
            capacity = maximum
        } else {
            capacity *= 2
        }
        if capacity < required && capacity == maximum {
            return 0, fmt.Errorf("%w: frame stack capacity overflow", ErrContext)
        }
    }
    return capacity, nil
}

func contextBoardAndFacts(position base.Position) ([64]boardPiece, base.PositionFacts, error) {
    board, kings, _, err := validatePosition(position)
    var facts base.PositionFacts
    facts.KingSquare = [2]base.Square{base.NoSquare, base.NoSquare}
    if err != nil {
        return board, facts, err
    }
    facts.KingSquare = kings
    for square, piece := range board {
        if !piece.set {
            continue
        }
        bit := uint64(1) << square
        facts.Occupied |= bit
        if piece.piece == base.Pawn {
            facts.Pawns[piece.color] |= bit
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

func applyContextDelta(before [64]boardPiece, beforeFacts base.PositionFacts, delta base.Delta) ([64]boardPiece, base.PositionFacts, error) {
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

func wrapSub16(left, right int16) int16 { return int16(uint16(left) - uint16(right)) }
