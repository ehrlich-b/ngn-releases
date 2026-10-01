package engine

import "sync"

// CHECKMATE_EVAL is the score used for a checkmated position.
const CHECKMATE_EVAL int16 = 30000

// MAX_NON_CHECKMATE and MIN_NON_CHECKMATE bound ordinary position scores.
const MAX_NON_CHECKMATE float32 = 25000
const MIN_NON_CHECKMATE float32 = -MAX_NON_CHECKMATE

// PositionTag stores the state which is not represented by the board itself.
// The turn bits intentionally occupy two separate flags: callers of the
// legacy interface use ToggleTurn to flip both of them together.
type PositionTag uint8

const (
	WhiteCanCastleKingSide PositionTag = 1 << iota
	WhiteCanCastleQueenSide
	BlackCanCastleKingSide
	BlackCanCastleQueenSide
	InCheck
	BlackToMove
	WhiteToMove
	enPassantHash
)

// CASTLING_FLAG is the mask of all four castling-right flags.
const CASTLING_FLAG PositionTag = WhiteCanCastleKingSide |
	WhiteCanCastleQueenSide |
	BlackCanCastleKingSide |
	BlackCanCastleQueenSide

// Status is the coarse result status used by callers of the position layer.
type Status uint8

const (
	Checkmate Status = iota
	Draw
	Unknown
)

// Position is a board together with the state needed to update and undo it.
// positionsMutex protects Positions when the game-facing move methods and
// draw policy inspect or update the repetition table.
type Position struct {
	Board          Bitboard
	EnPassant      Square
	Tag            PositionTag
	hash           uint64
	Positions      map[uint64]int
	positionsMutex sync.RWMutex
	HalfMoveClock  uint8
}

// SetTag sets every bit present in tag.
func (position *Position) SetTag(tag PositionTag) {
	position.Tag |= tag
}

// ClearTag clears every bit present in tag.
func (position *Position) ClearTag(tag PositionTag) {
	position.Tag &^= tag
}

// ToggleTag flips every bit present in tag.
func (position *Position) ToggleTag(tag PositionTag) {
	position.Tag ^= tag
}

// HasTag reports whether any bit in tag is set.
func (position *Position) HasTag(tag PositionTag) bool {
	return position.Tag&tag != 0
}

// HasCastling reports whether either side has any castling right remaining.
func (position *Position) HasCastling() bool {
	return position.HasTag(CASTLING_FLAG)
}

// Turn returns White when the white-turn flag is set and Black otherwise.
func (position *Position) Turn() Color {
	if position.HasTag(WhiteToMove) {
		return White
	}
	return Black
}

// ToggleTurn flips both legacy turn flags.
func (position *Position) ToggleTurn() {
	position.ToggleTag(BlackToMove | WhiteToMove)
}

// IsEndGame delegates the material question to the board for the side to
// move.
func (position *Position) IsEndGame() bool {
	return position.Board.IsEndGame(position.Turn())
}

// IsInCheck reports the cached check flag.
func (position *Position) IsInCheck() bool {
	return position.HasTag(InCheck)
}

// Hash returns the cached position key, initializing it lazily.  The helper
// owns the en-passant eligibility rules and the key stream, so this method
// deliberately delegates both pieces to it.
func (position *Position) Hash() uint64 {
	if position.hash == 0 {
		position.ensureEnPassantHash()
		position.hash = generateZobristHash(position)
	}
	return position.hash
}

// findEnPassantCaptureSquare returns the square of the pawn removed by an
// en-passant move.  The board is rank-major, so the victim is eight squares
// behind the destination for either mover.
func findEnPassantCaptureSquare(move Move) Square {
	return move.Destination() ^ 8
}

// applyMoveBoard applies just the board part of a packed move.  The caller is
// responsible for metadata and the turn.  Board.Move also performs the
// forward rook movement for a castling king move.
func (position *Position) applyMoveBoard(move Move) {
	source := move.Source()
	destination := move.Destination()
	moving := move.MovingPiece()
	captured := move.CapturedPiece()

	destinationPiece := captured
	if move.IsEnPassant() {
		destinationPiece = NoPiece
	}
	position.Board.Move(source, destination, moving, destinationPiece)

	if move.IsEnPassant() {
		position.Board.Clear(findEnPassantCaptureSquare(move), captured)
	}

	if promotion := GetPiece(move.PromoType(), moving.Color()); promotion != NoPiece {
		position.Board.UpdateSquare(destination, promotion, moving)
	}
}

// restoreCastlingRook explicitly returns the rook which Board.Move moved
// while applying a castle.  Board.Move intentionally only has forward
// castling behavior.
func (position *Position) restoreCastlingRook(move Move, mover Color) {
	if move.IsKingSideCastle() {
		switch mover {
		case White:
			position.Board.Move(F1, H1, WhiteRook, NoPiece)
		case Black:
			position.Board.Move(F8, H8, BlackRook, NoPiece)
		}
		return
	}
	if move.IsQueenSideCastle() {
		switch mover {
		case White:
			position.Board.Move(D1, A1, WhiteRook, NoPiece)
		case Black:
			position.Board.Move(D8, A8, BlackRook, NoPiece)
		}
	}
}

// undoMoveBoard undoes just the board part of a packed move.  The mover is
// supplied by the caller because partial undo first recovers the original
// side to move.
func (position *Position) undoMoveBoard(move Move, mover Color) {
	source := move.Source()
	destination := move.Destination()
	moving := move.MovingPiece()
	captured := move.CapturedPiece()

	if promotion := GetPiece(move.PromoType(), mover); promotion != NoPiece {
		position.Board.UpdateSquare(destination, moving, promotion)
	}

	position.Board.Move(destination, source, moving, NoPiece)
	if move.IsCastle() {
		position.restoreCastlingRook(move, mover)
	}

	if move.IsEnPassant() {
		position.Board.UpdateSquare(findEnPassantCaptureSquare(move), captured, NoPiece)
	} else if captured != NoPiece {
		position.Board.UpdateSquare(destination, captured, NoPiece)
	}
}

// partialMakeMove changes only the board and the side to move.
func (position *Position) partialMakeMove(move Move) {
	position.applyMoveBoard(move)
	position.ToggleTurn()
}

// partialUnMakeMove changes only the board and the side to move.  Toggling
// first recovers the original mover's color for a promotion undo.
func (position *Position) partialUnMakeMove(move Move) {
	position.ToggleTurn()
	position.undoMoveBoard(move, position.Turn())
}

// clearCastlingRightsForMove applies the four corner/king rules without
// looking at the captured mailbox value.  A move landing on an opponent's
// original rook corner therefore also removes that opponent's right when the
// corner did not contain a rook.
func (position *Position) clearCastlingRightsForMove(move Move) {
	moving := move.MovingPiece()
	mover := moving.Color()
	source := move.Source()
	destination := move.Destination()

	switch mover {
	case White:
		switch moving.Type() {
		case King:
			position.ClearTag(WhiteCanCastleKingSide | WhiteCanCastleQueenSide)
		case Rook:
			switch source {
			case A1:
				position.ClearTag(WhiteCanCastleQueenSide)
			case H1:
				position.ClearTag(WhiteCanCastleKingSide)
			}
		}
		switch destination {
		case A8:
			position.ClearTag(BlackCanCastleQueenSide)
		case H8:
			position.ClearTag(BlackCanCastleKingSide)
		}
	case Black:
		switch moving.Type() {
		case King:
			position.ClearTag(BlackCanCastleKingSide | BlackCanCastleQueenSide)
		case Rook:
			switch source {
			case A8:
				position.ClearTag(BlackCanCastleQueenSide)
			case H8:
				position.ClearTag(BlackCanCastleKingSide)
			}
		}
		switch destination {
		case A1:
			position.ClearTag(WhiteCanCastleQueenSide)
		case H1:
			position.ClearTag(WhiteCanCastleKingSide)
		}
	}
}

// doublePawnPushTarget returns the raw intervening square for a standard
// two-square pawn move.
func doublePawnPushTarget(move Move) Square {
	moving := move.MovingPiece()
	if moving.Type() != Pawn {
		return NoSquare
	}
	source := move.Source()
	destination := move.Destination()
	if source.File() != destination.File() {
		return NoSquare
	}

	switch moving.Color() {
	case White:
		if source.Rank() == Rank2 && destination.Rank() == Rank4 {
			return SquareOf(source.File(), Rank3)
		}
	case Black:
		if source.Rank() == Rank7 && destination.Rank() == Rank5 {
			return SquareOf(source.File(), Rank6)
		}
	}
	return NoSquare
}

// updateEnPassantAfterMove installs the raw target only when the independent
// adjacency predicate accepts it, then refreshes the separate legal-EP hash
// bit.  The two predicates intentionally remain independent.
func (position *Position) updateEnPassantAfterMove(move Move) {
	position.EnPassant = doublePawnPushTarget(move)
	if position.EnPassant != NoSquare && !canCaptureEnPassant(position) {
		position.EnPassant = NoSquare
	}
	position.refreshEnPassantHash()
}

// updateCheckForSideToMove refreshes only the cached check flag.
func (position *Position) updateCheckForSideToMove() {
	position.ClearTag(InCheck)
	if isInCheck(position, position.Turn()) {
		position.SetTag(InCheck)
	}
}

// makeMoveHelper applies the complete search move update and returns the
// metadata needed by its inverse.  Move generation has already established
// legality; this layer deliberately does not validate the move.
func (position *Position) makeMoveHelper(move Move) (oldEnPassant Square, oldTag PositionTag, oldClock uint8, legal bool) {
	// A manually assembled cold root may have a raw EP target but no legal-EP
	// hash bit yet.  Establish that bit before saving the old metadata.
	if position.hash == 0 {
		position.ensureEnPassantHash()
	}

	oldEnPassant = position.EnPassant
	oldTag = position.Tag
	oldClock = position.HalfMoveClock

	moving := move.MovingPiece()
	captured := move.CapturedPiece()
	position.applyMoveBoard(move)

	if moving.Type() == Pawn || captured != NoPiece {
		position.HalfMoveClock = 0
	} else {
		position.HalfMoveClock++
	}

	position.clearCastlingRightsForMove(move)
	position.ToggleTurn()
	position.updateEnPassantAfterMove(move)
	position.updateCheckForSideToMove()

	captureSquare := move.Destination()
	if move.IsEnPassant() {
		captureSquare = findEnPassantCaptureSquare(move)
	}
	promotion := GetPiece(move.PromoType(), moving.Color())
	updateHash(position, move, captureSquare, position.EnPassant, oldEnPassant, promotion, oldTag)

	return oldEnPassant, oldTag, oldClock, true
}

// MakeMove is the search-facing complete move update.
func (position *Position) MakeMove(move Move) (oldEnPassant Square, oldTag PositionTag, oldClock uint8, legal bool) {
	return position.makeMoveHelper(move)
}

// unMakeMoveHelper reverses a complete move.  Hash reversal happens while all
// post-move metadata is still installed, as required by updateHash.
func (position *Position) unMakeMoveHelper(move Move, oldTag PositionTag, oldEnPassant Square, oldClock uint8) {
	captureSquare := move.Destination()
	if move.IsEnPassant() {
		captureSquare = findEnPassantCaptureSquare(move)
	}
	moving := move.MovingPiece()
	promotion := GetPiece(move.PromoType(), moving.Color())
	updateHash(position, move, captureSquare, position.EnPassant, oldEnPassant, promotion, oldTag)

	position.Tag = oldTag
	position.EnPassant = oldEnPassant
	position.HalfMoveClock = oldClock

	// The saved tag already contains the original turn.  Use it to recover the
	// mover's color for promotions and castling-rook restoration; unlike the
	// partial inverse, the complete inverse must not toggle after restoring the
	// saved tag.
	position.undoMoveBoard(move, position.Turn())
}

// UnMakeMove is the search-facing inverse of MakeMove.
func (position *Position) UnMakeMove(move Move, oldTag PositionTag, oldEnPassant Square, oldClock uint8) {
	position.unMakeMoveHelper(move, oldTag, oldEnPassant, oldClock)
}

// GameMakeMove applies a move and records the resulting position when a
// repetition table has been supplied.
func (position *Position) GameMakeMove(move Move) (oldEnPassant Square, oldTag PositionTag, oldClock uint8, legal bool) {
	oldEnPassant, oldTag, oldClock, legal = position.makeMoveHelper(move)
	if legal && position.Positions != nil {
		currentHash := position.Hash()
		position.positionsMutex.Lock()
		position.Positions[currentHash]++
		position.positionsMutex.Unlock()
	}
	return oldEnPassant, oldTag, oldClock, legal
}

// GameUnMakeMove removes the current position from the repetition table and
// then performs the ordinary search undo.  Missing entries are deliberately
// left untouched.
func (position *Position) GameUnMakeMove(move Move, oldTag PositionTag, oldEnPassant Square, oldClock uint8) {
	if position.Positions != nil {
		currentHash := position.Hash()
		position.positionsMutex.Lock()
		if count, ok := position.Positions[currentHash]; ok {
			if count <= 1 {
				delete(position.Positions, currentHash)
			} else {
				position.Positions[currentHash] = count - 1
			}
		}
		position.positionsMutex.Unlock()
	}
	position.unMakeMoveHelper(move, oldTag, oldEnPassant, oldClock)
}

// MakeNullMove performs a search null move and returns the raw target which
// was present before it.  The legal-EP flag is intentionally retained so the
// inverse can use the supplied hash helper without an extra metadata record.
func (position *Position) MakeNullMove() Square {
	position.ensureEnPassantHash()
	oldEnPassant := position.EnPassant
	position.EnPassant = NoSquare
	position.HalfMoveClock++
	position.ToggleTurn()
	updateHashForNullMove(position, position.EnPassant, oldEnPassant)
	return oldEnPassant
}

// UnMakeNullMove reverses MakeNullMove while retaining the helper's cold/warm
// hash behavior.
func (position *Position) UnMakeNullMove(oldEnPassant Square) {
	updateHashForNullMove(position, position.EnPassant, oldEnPassant)
	position.EnPassant = oldEnPassant
	position.HalfMoveClock--
	position.ToggleTurn()
}

// Copy returns an independent state copy.  A nil source repetition map is
// represented by an allocated empty map for compatibility with the legacy
// callers.
func (position *Position) Copy() *Position {
	position.positionsMutex.RLock()

	copyPosition := &Position{
		Board:         position.Board,
		EnPassant:     position.EnPassant,
		Tag:           position.Tag,
		hash:          position.hash,
		Positions:     make(map[uint64]int, len(position.Positions)),
		HalfMoveClock: position.HalfMoveClock,
	}
	for key, count := range position.Positions {
		copyPosition.Positions[key] = count
	}

	position.positionsMutex.RUnlock()
	return copyPosition
}

// IsFIDEDrawRule implements the compatibility FIDE-style clock/repetition
// policy.  It intentionally does not infer material draws.
func (position *Position) IsFIDEDrawRule() bool {
	if position.HalfMoveClock >= 100 {
		return true
	}
	if position.Positions == nil {
		return false
	}

	currentHash := position.Hash()
	position.positionsMutex.RLock()
	count := position.Positions[currentHash]
	position.positionsMutex.RUnlock()
	return count >= 3
}

// IsDraw is the legacy material helper.  It is intentionally narrower than a
// complete FIDE adjudicator and does not consult the repetition table.
func (position *Position) IsDraw() bool {
	if position.HalfMoveClock > 100 {
		return true
	}
	if position.Board.Pawns() != 0 || position.Board.Rooks() != 0 || position.Board.Queens() != 0 {
		return false
	}

	minorMasks := [...]uint64{
		position.Board.GetBitboardOf(WhiteKnight),
		position.Board.GetBitboardOf(BlackKnight),
		position.Board.GetBitboardOf(WhiteBishop),
		position.Board.GetBitboardOf(BlackBishop),
	}
	nonEmptyMasks := 0
	for _, mask := range minorMasks {
		if mask != 0 {
			nonEmptyMasks++
		}
	}
	if nonEmptyMasks == 0 {
		return true
	}
	if nonEmptyMasks == 1 {
		for _, mask := range minorMasks {
			if mask != 0 {
				return PopCount(mask) == 1
			}
		}
	}

	whiteBishops := position.Board.GetBitboardOf(WhiteBishop)
	blackBishops := position.Board.GetBitboardOf(BlackBishop)
	if position.Board.Knights() == 0 &&
		PopCount(whiteBishops) == 1 && PopCount(blackBishops) == 1 {
		whiteSquare := Square(bitScanForward(whiteBishops))
		blackSquare := Square(bitScanForward(blackBishops))
		if whiteSquare.GetColor() == blackSquare.GetColor() {
			return true
		}
	}

	return false
}
