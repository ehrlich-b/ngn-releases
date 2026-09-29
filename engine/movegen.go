package engine

import "math/bits"

// GenerateMoves generates all pseudo-legal moves for the current position
func GenerateMoves(pos *Position) []Move {
	return WrapMoveGeneration("GenerateMoves", func() []Move {
		return generateMovesUnsafe(pos)
	}, pos)
}

func generateMovesUnsafe(pos *Position) []Move {
	moves := make([]Move, 0, 64)
	turn := pos.Turn()

	if turn == White {
		moves = append(moves, generateWhitePawnMoves(pos)...)
		moves = append(moves, generatePieceMoves(pos, WhiteKnight, generateKnightMoves)...)
		moves = append(moves, generatePieceMoves(pos, WhiteBishop, generateBishopMoves)...)
		moves = append(moves, generatePieceMoves(pos, WhiteRook, generateRookMoves)...)
		moves = append(moves, generatePieceMoves(pos, WhiteQueen, generateQueenMoves)...)
		moves = append(moves, generateKingMoves(pos, WhiteKing)...)
	} else {
		moves = append(moves, generateBlackPawnMoves(pos)...)
		moves = append(moves, generatePieceMoves(pos, BlackKnight, generateKnightMoves)...)
		moves = append(moves, generatePieceMoves(pos, BlackBishop, generateBishopMoves)...)
		moves = append(moves, generatePieceMoves(pos, BlackRook, generateRookMoves)...)
		moves = append(moves, generatePieceMoves(pos, BlackQueen, generateQueenMoves)...)
		moves = append(moves, generateKingMoves(pos, BlackKing)...)
	}

	return moves
}

// GenerateLegalMoves generates all legal moves for the current position
func GenerateLegalMoves(pos *Position) []Move {
	return WrapMoveGeneration("GenerateLegalMoves", func() []Move {
		return generateLegalMovesUnsafe(pos)
	}, pos)
}

// validatePosition checks if a position has both kings (basic validity check)
func validatePosition(pos *Position) bool {
	whiteKingBB := pos.Board.GetBitboardOf(WhiteKing)
	blackKingBB := pos.Board.GetBitboardOf(BlackKing)

	// Both kings must exist
	if whiteKingBB == 0 || blackKingBB == 0 {
		return false
	}

	// Each king should appear exactly once
	if bits.OnesCount64(whiteKingBB) != 1 || bits.OnesCount64(blackKingBB) != 1 {
		return false
	}

	return true
}

func generateLegalMovesUnsafe(pos *Position) []Move {
	// Validate position before generating any moves
	if !validatePosition(pos) {
		// Invalid position (e.g., missing kings) - return no legal moves
		return []Move{}
	}

	pseudoLegalMoves := generateMovesUnsafe(pos)
	legalMoves := make([]Move, 0, len(pseudoLegalMoves))

	for _, move := range pseudoLegalMoves {
		if IsLegalMove(pos, move) {
			legalMoves = append(legalMoves, move)
		}
	}

	return legalMoves
}

// GenerateCaptures generates only capture moves (for quiescence search)
func GenerateCaptures(pos *Position) []Move {
	return WrapMoveGeneration("GenerateCaptures", func() []Move {
		return generateCapturesUnsafe(pos)
	}, pos)
}

func generateCapturesUnsafe(pos *Position) []Move {
	moves := generateMovesUnsafe(pos)
	captures := make([]Move, 0, len(moves))

	for _, move := range moves {
		if move.IsCapture() {
			captures = append(captures, move)
		}
	}

	return captures
}

// generatePieceMoves generates moves for a specific piece type using the provided generator function
func generatePieceMoves(pos *Position, piece Piece, generator func(*Position, Square) []Move) []Move {
	moves := make([]Move, 0, 32)
	pieceBitboard := pos.Board.GetBitboardOf(piece)

	for pieceBitboard != 0 {
		square := Square(bits.TrailingZeros64(pieceBitboard))
		moves = append(moves, generator(pos, square)...)
		pieceBitboard &^= SquareMask[int(square)]
	}

	return moves
}

// generateWhitePawnMoves generates all white pawn moves
func generateWhitePawnMoves(pos *Position) []Move {
	moves := make([]Move, 0, 16)
	whitePawns := pos.Board.GetBitboardOf(WhitePawn)
	allPieces := pos.Board.GetWhitePieces() | pos.Board.GetBlackPieces()
	enemyPieces := pos.Board.GetBlackPieces()

	// Single pawn pushes
	singlePushes := (whitePawns << 8) & ^allPieces
	singlePushesCopy := singlePushes // Save copy for double push calculation

	for singlePushes != 0 {
		to := Square(bits.TrailingZeros64(singlePushes))
		from := Square(int(to) - 8)

		// Check for promotion
		if to.Rank() == Rank8 {
			moves = append(moves, NewMove(from, to, WhitePawn, NoPiece, Queen, 0))
			moves = append(moves, NewMove(from, to, WhitePawn, NoPiece, Rook, 0))
			moves = append(moves, NewMove(from, to, WhitePawn, NoPiece, Bishop, 0))
			moves = append(moves, NewMove(from, to, WhitePawn, NoPiece, Knight, 0))
		} else {
			moves = append(moves, NewMove(from, to, WhitePawn, NoPiece, NoType, 0))
		}

		singlePushes &^= SquareMask[int(to)]
	}

	// Double pawn pushes - only from starting rank, with both intermediate and destination empty
	doublePushes := ((singlePushesCopy & RankMasks[Rank3]) << 8) & ^allPieces
	for doublePushes != 0 {
		to := Square(bits.TrailingZeros64(doublePushes))
		from := Square(int(to) - 16)
		moves = append(moves, NewMove(from, to, WhitePawn, NoPiece, NoType, 0))
		doublePushes &^= SquareMask[int(to)]
	}

	// Pawn captures (diagonal left)
	leftCaptures := ((whitePawns & ^FileMasks[FileA]) << 7) & enemyPieces
	for leftCaptures != 0 {
		to := Square(bits.TrailingZeros64(leftCaptures))
		from := Square(int(to) - 7)
		capturedPiece := pos.Board.PieceAt(to)

		// Check for promotion
		if to.Rank() == Rank8 {
			moves = append(moves, NewMove(from, to, WhitePawn, capturedPiece, Queen, Capture))
			moves = append(moves, NewMove(from, to, WhitePawn, capturedPiece, Rook, Capture))
			moves = append(moves, NewMove(from, to, WhitePawn, capturedPiece, Bishop, Capture))
			moves = append(moves, NewMove(from, to, WhitePawn, capturedPiece, Knight, Capture))
		} else {
			moves = append(moves, NewMove(from, to, WhitePawn, capturedPiece, NoType, Capture))
		}

		leftCaptures &^= SquareMask[int(to)]
	}

	// Pawn captures (diagonal right)
	rightCaptures := ((whitePawns & ^FileMasks[FileH]) << 9) & enemyPieces
	for rightCaptures != 0 {
		to := Square(bits.TrailingZeros64(rightCaptures))
		from := Square(int(to) - 9)

		// Bounds check
		if int(to) < 0 || int(to) > 63 || int(from) < 0 || int(from) > 63 {
			rightCaptures &^= SquareMask[int(to)]
			continue
		}

		capturedPiece := pos.Board.PieceAt(to)

		// Check for promotion
		if to.Rank() == Rank8 {
			moves = append(moves, NewMove(from, to, WhitePawn, capturedPiece, Queen, Capture))
			moves = append(moves, NewMove(from, to, WhitePawn, capturedPiece, Rook, Capture))
			moves = append(moves, NewMove(from, to, WhitePawn, capturedPiece, Bishop, Capture))
			moves = append(moves, NewMove(from, to, WhitePawn, capturedPiece, Knight, Capture))
		} else {
			moves = append(moves, NewMove(from, to, WhitePawn, capturedPiece, NoType, Capture))
		}

		rightCaptures &^= SquareMask[int(to)]
	}

	// En passant captures
	if pos.EnPassant != NoSquare {
		enPassantTarget := pos.EnPassant
		// Check left diagonal for en passant
		if enPassantTarget.File() > FileA {
			leftSquare := SquareOf(enPassantTarget.File()-1, enPassantTarget.Rank()-1)
			if pos.Board.PieceAt(leftSquare) == WhitePawn {
				moves = append(moves, NewMove(leftSquare, enPassantTarget, WhitePawn, BlackPawn, NoType, EnPassant|Capture))
			}
		}
		// Check right diagonal for en passant
		if enPassantTarget.File() < FileH {
			rightSquare := SquareOf(enPassantTarget.File()+1, enPassantTarget.Rank()-1)
			if pos.Board.PieceAt(rightSquare) == WhitePawn {
				moves = append(moves, NewMove(rightSquare, enPassantTarget, WhitePawn, BlackPawn, NoType, EnPassant|Capture))
			}
		}
	}

	return moves
}

// generateBlackPawnMoves generates all black pawn moves
func generateBlackPawnMoves(pos *Position) []Move {
	moves := make([]Move, 0, 16)
	blackPawns := pos.Board.GetBitboardOf(BlackPawn)
	allPieces := pos.Board.GetWhitePieces() | pos.Board.GetBlackPieces()
	enemyPieces := pos.Board.GetWhitePieces()

	// Single pawn pushes (black pawns move "down" the board)
	singlePushes := (blackPawns >> 8) & ^allPieces
	singlePushesCopy := singlePushes // Save copy for double push calculation

	for singlePushes != 0 {
		to := Square(bits.TrailingZeros64(singlePushes))
		from := Square(int(to) + 8)

		// Check for promotion
		if to.Rank() == Rank1 {
			moves = append(moves, NewMove(from, to, BlackPawn, NoPiece, Queen, 0))
			moves = append(moves, NewMove(from, to, BlackPawn, NoPiece, Rook, 0))
			moves = append(moves, NewMove(from, to, BlackPawn, NoPiece, Bishop, 0))
			moves = append(moves, NewMove(from, to, BlackPawn, NoPiece, Knight, 0))
		} else {
			moves = append(moves, NewMove(from, to, BlackPawn, NoPiece, NoType, 0))
		}

		singlePushes &^= SquareMask[int(to)]
	}

	// Double pawn pushes - only from starting rank, with both intermediate and destination empty
	doublePushes := ((singlePushesCopy & RankMasks[Rank6]) >> 8) & ^allPieces
	for doublePushes != 0 {
		to := Square(bits.TrailingZeros64(doublePushes))
		from := Square(int(to) + 16)
		moves = append(moves, NewMove(from, to, BlackPawn, NoPiece, NoType, 0))
		doublePushes &^= SquareMask[int(to)]
	}

	// Pawn captures (diagonal left for black)
	leftCaptures := ((blackPawns & ^FileMasks[FileH]) >> 7) & enemyPieces
	for leftCaptures != 0 {
		to := Square(bits.TrailingZeros64(leftCaptures))
		from := Square(int(to) + 7)
		capturedPiece := pos.Board.PieceAt(to)

		// Check for promotion
		if to.Rank() == Rank1 {
			moves = append(moves, NewMove(from, to, BlackPawn, capturedPiece, Queen, Capture))
			moves = append(moves, NewMove(from, to, BlackPawn, capturedPiece, Rook, Capture))
			moves = append(moves, NewMove(from, to, BlackPawn, capturedPiece, Bishop, Capture))
			moves = append(moves, NewMove(from, to, BlackPawn, capturedPiece, Knight, Capture))
		} else {
			moves = append(moves, NewMove(from, to, BlackPawn, capturedPiece, NoType, Capture))
		}

		leftCaptures &^= SquareMask[int(to)]
	}

	// Pawn captures (diagonal right for black)
	rightCaptures := ((blackPawns & ^FileMasks[FileA]) >> 9) & enemyPieces
	for rightCaptures != 0 {
		to := Square(bits.TrailingZeros64(rightCaptures))
		from := Square(int(to) + 9)
		capturedPiece := pos.Board.PieceAt(to)

		// Check for promotion
		if to.Rank() == Rank1 {
			moves = append(moves, NewMove(from, to, BlackPawn, capturedPiece, Queen, Capture))
			moves = append(moves, NewMove(from, to, BlackPawn, capturedPiece, Rook, Capture))
			moves = append(moves, NewMove(from, to, BlackPawn, capturedPiece, Bishop, Capture))
			moves = append(moves, NewMove(from, to, BlackPawn, capturedPiece, Knight, Capture))
		} else {
			moves = append(moves, NewMove(from, to, BlackPawn, capturedPiece, NoType, Capture))
		}

		rightCaptures &^= SquareMask[int(to)]
	}

	// En passant captures
	if pos.EnPassant != NoSquare && pos.EnPassant.Rank() == Rank3 { // Black can only en passant on rank 3 (6th rank from black's perspective)
		enPassantTarget := pos.EnPassant
		capturedPawnSquare := SquareOf(enPassantTarget.File(), enPassantTarget.Rank()+1) // White pawn is on rank 4, beyond the rank-3 target.

		// Verify there's actually a white pawn to capture
		if pos.Board.PieceAt(capturedPawnSquare) == WhitePawn {
			// Check left diagonal for en passant: pawn at (target-1 file, target+1 rank) captures to target
			if enPassantTarget.File() > FileA {
				leftSquare := SquareOf(enPassantTarget.File()-1, enPassantTarget.Rank()+1)
				if pos.Board.PieceAt(leftSquare) == BlackPawn {
					moves = append(moves, NewMove(leftSquare, enPassantTarget, BlackPawn, WhitePawn, NoType, EnPassant|Capture))
				}
			}
			// Check right diagonal for en passant: pawn at (target+1 file, target+1 rank) captures to target
			if enPassantTarget.File() < FileH {
				rightSquare := SquareOf(enPassantTarget.File()+1, enPassantTarget.Rank()+1)
				if pos.Board.PieceAt(rightSquare) == BlackPawn {
					moves = append(moves, NewMove(rightSquare, enPassantTarget, BlackPawn, WhitePawn, NoType, EnPassant|Capture))
				}
			}
		}
	}

	return moves
}

// generateKnightMoves generates moves for a knight at the given square
func generateKnightMoves(pos *Position, from Square) []Move {
	moves := make([]Move, 0, 8)
	attacks := KnightAttacks[from]
	piece := pos.Board.PieceAt(from)
	friendlyPieces := pos.Board.GetWhitePieces()
	if piece.Color() == Black {
		friendlyPieces = pos.Board.GetBlackPieces()
	}

	// Remove squares occupied by friendly pieces
	attacks &^= friendlyPieces

	for attacks != 0 {
		to := Square(bits.TrailingZeros64(attacks))
		capturedPiece := pos.Board.PieceAt(to)
		tag := MoveTag(0)
		if capturedPiece != NoPiece {
			tag = Capture
		}
		moves = append(moves, NewMove(from, to, piece, capturedPiece, NoType, tag))
		attacks &^= SquareMask[int(to)]
	}

	return moves
}

// generateBishopMoves generates moves for a bishop at the given square
func generateBishopMoves(pos *Position, from Square) []Move {
	return generateSlidingMoves(pos, from, bishopDirections)
}

// generateRookMoves generates moves for a rook at the given square
func generateRookMoves(pos *Position, from Square) []Move {
	return generateSlidingMoves(pos, from, rookDirections)
}

// generateQueenMoves generates moves for a queen at the given square
func generateQueenMoves(pos *Position, from Square) []Move {
	return generateSlidingMoves(pos, from, queenDirections)
}

// generateSlidingMoves generates moves for sliding pieces (bishop, rook, queen)
// PERFORMANCE: Changed to return fixed-size array to eliminate heap allocations
// Was allocating 253MB in profiling! Now stack-allocated.
func generateSlidingMoves(pos *Position, from Square, directions []int) []Move {
	// Stack-allocated buffer - max 27 moves (queen on center square)
	var moveBuffer [28]Move
	numMoves := 0
	piece := pos.Board.PieceAt(from)
	friendlyPieces := pos.Board.GetWhitePieces()
	if piece.Color() == Black {
		friendlyPieces = pos.Board.GetBlackPieces()
	}
	allPieces := pos.Board.GetWhitePieces() | pos.Board.GetBlackPieces()

	for _, direction := range directions {
		for i := 1; i < 8; i++ {
			to := int(from) + direction*i

			// Check bounds
			if to < 0 || to > 63 {
				break
			}

			// Check for file wrapping on horizontal and diagonal moves
			fromFile := int(from) % 8
			toFile := to % 8

			// For horizontal moves, check direct file wrapping
			if direction == 1 && toFile < fromFile {
				break
			}
			if direction == -1 && toFile > fromFile {
				break
			}

			// CRITICAL FIX: Check for vertical moves wrapping around the board
			// Vertical moves should NOT change the file at all
			if (direction == 8 || direction == -8) && toFile != fromFile {
				break
			}

			// For diagonal moves, check file distance matches step count and prevent board wrapping
			if abs(direction) == 7 || abs(direction) == 9 {
				// Check if we've wrapped around the board horizontally
				expectedFileDiff := i
				if direction == 7 || direction == -9 {
					expectedFileDiff = -i
				}
				actualFileDiff := toFile - fromFile

				// If the actual file difference doesn't match expected, or if we wrapped, break
				if actualFileDiff != expectedFileDiff {
					break
				}
			}

			toSquare := Square(to)

			// If square is occupied by friendly piece, stop
			if SquareMask[to]&friendlyPieces != 0 {
				break
			}

			capturedPiece := pos.Board.PieceAt(toSquare)
			tag := MoveTag(0)
			if capturedPiece != NoPiece {
				tag = Capture
			}

			moveBuffer[numMoves] = NewMove(from, toSquare, piece, capturedPiece, NoType, tag)
			numMoves++

			// If we captured an enemy piece, stop sliding in this direction
			if SquareMask[to]&allPieces != 0 {
				break
			}
		}
	}

	return moveBuffer[:numMoves]
}

// generateKingMoves generates moves for a king at the given square (including castling)
func generateKingMoves(pos *Position, piece Piece) []Move {
	moves := make([]Move, 0, 8)
	kingBitboard := pos.Board.GetBitboardOf(piece)

	// Check if king exists on the board
	if kingBitboard == 0 {
		return moves // No king found, return empty moves
	}

	kingSquare := Square(bits.TrailingZeros64(kingBitboard))

	// Additional safety check (should never happen with valid positions)
	if kingSquare > 63 {
		return moves
	}

	attacks := KingAttacks[kingSquare]
	friendlyPieces := pos.Board.GetWhitePieces()
	if piece.Color() == Black {
		friendlyPieces = pos.Board.GetBlackPieces()
	}

	// Remove squares occupied by friendly pieces
	attacks &^= friendlyPieces

	for attacks != 0 {
		to := Square(bits.TrailingZeros64(attacks))
		capturedPiece := pos.Board.PieceAt(to)
		tag := MoveTag(0)
		if capturedPiece != NoPiece {
			tag = Capture
		}
		moves = append(moves, NewMove(kingSquare, to, piece, capturedPiece, NoType, tag))
		attacks &^= SquareMask[int(to)]
	}

	// Generate castling moves
	if piece == WhiteKing && kingSquare == E1 {
		// White king-side castling
		if pos.HasTag(WhiteCanCastleKingSide) &&
			pos.Board.PieceAt(F1) == NoPiece && pos.Board.PieceAt(G1) == NoPiece &&
			pos.Board.PieceAt(H1) == WhiteRook {
			moves = append(moves, NewMove(E1, G1, WhiteKing, NoPiece, NoType, KingSideCastle))
		}
		// White queen-side castling
		if pos.HasTag(WhiteCanCastleQueenSide) &&
			pos.Board.PieceAt(D1) == NoPiece && pos.Board.PieceAt(C1) == NoPiece && pos.Board.PieceAt(B1) == NoPiece &&
			pos.Board.PieceAt(A1) == WhiteRook {
			moves = append(moves, NewMove(E1, C1, WhiteKing, NoPiece, NoType, QueenSideCastle))
		}
	} else if piece == BlackKing && kingSquare == E8 {
		// Black king-side castling
		if pos.HasTag(BlackCanCastleKingSide) &&
			pos.Board.PieceAt(F8) == NoPiece && pos.Board.PieceAt(G8) == NoPiece &&
			pos.Board.PieceAt(H8) == BlackRook {
			moves = append(moves, NewMove(E8, G8, BlackKing, NoPiece, NoType, KingSideCastle))
		}
		// Black queen-side castling
		if pos.HasTag(BlackCanCastleQueenSide) &&
			pos.Board.PieceAt(D8) == NoPiece && pos.Board.PieceAt(C8) == NoPiece && pos.Board.PieceAt(B8) == NoPiece &&
			pos.Board.PieceAt(A8) == BlackRook {
			moves = append(moves, NewMove(E8, C8, BlackKing, NoPiece, NoType, QueenSideCastle))
		}
	}

	return moves
}

// Movement direction constants for sliding pieces
var (
	bishopDirections = []int{-9, -7, 7, 9}               // diagonal moves
	rookDirections   = []int{-8, -1, 1, 8}               // horizontal and vertical moves
	queenDirections  = []int{-9, -8, -7, -1, 1, 7, 8, 9} // all directions
)

// sliderAttacks returns the O(1) magic attack set for a slider at `from` given
// `occupied`, dispatching on the direction slice the caller passed (A4/T5). The
// magic getters (already used for check/SEE) replace the ray-walk; this changes
// move GENERATION ORDER (square order, not direction-by-direction), which is a
// load-bearing tie-break, so this is a [BEHAVIOUR] change gated by SPRT — but the
// move SET is identical, so perft is unchanged.
func sliderAttacks(directions []int, from Square, occupied uint64) uint64 {
	if len(directions) == 8 {
		return GetQueenAttacks(int(from), occupied)
	}
	if directions[0] == -8 { // rookDirections starts -8
		return GetRookAttacks(int(from), occupied)
	}
	return GetBishopAttacks(int(from), occupied) // bishopDirections
}

// Branchless absolute value - avoids branch prediction penalty
func abs(x int) int {
	mask := x >> 63
	return (x ^ mask) - mask
}

// Sliding piece move generators (zero allocation)
func generateSlidingMovesIntoBuffer(pos *Position, from Square, directions []int, buffer []Move) int {
	count := 0
	piece := pos.Board.PieceAt(from)
	friendly := pos.Board.GetWhitePieces()
	enemy := pos.Board.GetBlackPieces()
	if piece.Color() == Black {
		friendly, enemy = enemy, friendly
	}
	// A4: magic attack set instead of the ray-walk. attacks includes the first
	// blocker per ray; &^ friendly drops own-piece blockers, leaving empty squares
	// (quiet) and enemy blockers (captures). Same move SET, square order.
	occupied := friendly | enemy
	targets := sliderAttacks(directions, from, occupied) &^ friendly
	for targets != 0 {
		to := trailingZeros(targets)
		targets &= targets - 1
		toSq := Square(to)
		if enemy&SquareMask[to] != 0 {
			buffer[count] = NewMove(from, toSq, piece, pos.Board.PieceAt(toSq), NoType, Capture)
		} else {
			buffer[count] = NewMove(from, toSq, piece, NoPiece, NoType, 0)
		}
		count++
	}
	return count
}

func generateSlidingCapturesIntoBuffer(pos *Position, from Square, directions []int, buffer []Move) int {
	count := 0
	if len(buffer) == 0 {
		return 0
	}
	piece := pos.Board.PieceAt(from)
	friendly := pos.Board.GetWhitePieces()
	enemy := pos.Board.GetBlackPieces()
	if piece.Color() == Black {
		friendly, enemy = enemy, friendly
	}
	// A4: captures = magic attack set intersected with enemy pieces (the first
	// blocker per ray is in the attack set, so this is exactly the capturable set).
	occupied := friendly | enemy
	caps := sliderAttacks(directions, from, occupied) & enemy
	for caps != 0 {
		to := trailingZeros(caps)
		caps &= caps - 1
		if count < len(buffer) {
			toSq := Square(to)
			buffer[count] = NewMove(from, toSq, piece, pos.Board.PieceAt(toSq), NoType, Capture)
			count++
		}
	}
	return count
}

// moveIsPseudoLegal reports whether move can be played in pos: the moving piece is
// actually on its source square, belongs to the side to move, the destination is
// reachable by that piece given the current occupancy, and any capture target
// matches the board. It does NOT verify the move leaves the king out of check —
// that is caught after make, exactly as in the normal move loop. "Pseudo-legal" is
// precisely the guarantee needed to safely make a hash-table move (which may be a
// collision) before generating the rest of the moves.
//
// Special moves (promotion, castling, en passant, double pawn push) return false:
// their side effects (rook hop, ep capture square, double-push ep flag) are left to
// full generation. They are a small fraction of TT moves, so deferring them keeps
// this validator simple and safe at negligible cost to the staged-search win.
func moveIsPseudoLegal(pos *Position, move Move) bool {
	if move == EmptyMove {
		return false
	}
	mover := move.MovingPiece()
	src := move.Source()
	dst := move.Destination()
	stm := pos.Turn()

	// The mover must actually sit on src and belong to the side to move.
	if mover == NoPiece || mover.Color() != stm || pos.Board.PieceAt(src) != mover {
		return false
	}
	// Defer promotions / castles / en passant (side effects) to full generation.
	if move.PromoType() != NoType || move.IsCastle() || move.IsEnPassant() {
		return false
	}
	// The encoded capture must match the board, and a capture must land on an enemy.
	captured := pos.Board.PieceAt(dst)
	if captured != move.CapturedPiece() {
		return false
	}
	if captured != NoPiece && captured.Color() == stm {
		return false
	}

	dstMask := SquareMask[int(dst)]
	occupied := pos.Board.GetWhitePieces() | pos.Board.GetBlackPieces()

	switch mover.Type() {
	case Knight:
		return KnightAttacks[src]&dstMask != 0
	case King:
		return KingAttacks[src]&dstMask != 0
	case Bishop:
		return getBishopAttacksBB(int(src), occupied)&dstMask != 0
	case Rook:
		return getRookAttacksBB(int(src), occupied)&dstMask != 0
	case Queen:
		return (getBishopAttacksBB(int(src), occupied)|getRookAttacksBB(int(src), occupied))&dstMask != 0
	case Pawn:
		// Forward distance normalized so "toward promotion" is positive.
		fwd := int(dst) - int(src)
		if stm == Black {
			fwd = -fwd
		}
		srcFile := int(src) & 7
		dstFile := int(dst) & 7
		if captured == NoPiece {
			// Single quiet push, same file, one rank forward, dst empty. Double
			// pushes have fwd==16 -> false -> deferred to full generation.
			return fwd == 8 && srcFile == dstFile
		}
		// Diagonal capture one rank forward onto an adjacent file holding an enemy.
		// En passant lands on an EMPTY square (captured==NoPiece -> the push branch
		// above rejects it -> deferred), so this only matches real diagonal captures.
		return (fwd == 7 || fwd == 9) && (srcFile-dstFile == 1 || dstFile-srcFile == 1)
	}
	return false
}

// isLegalCastle verifies king-path safety for a structurally pseudo-legal castle.
// Its callers obtain the move from the generator, which already established the
// rights, rook, and empty-path requirements. MakeMove alone cannot do this: after
// castling, the rook may block an attack on the king's original square. Keep this
// direct rather than relying on PositionTag while inspecting a position.
func isLegalCastle(pos *Position, move Move) bool {
	color := move.MovingPiece().Color()
	if isInCheck(pos, color) {
		return false
	}
	var intermediateSquare, finalSquare Square
	if move.IsKingSideCastle() {
		if color == White {
			intermediateSquare, finalSquare = F1, G1
		} else {
			intermediateSquare, finalSquare = F8, G8
		}
	} else if color == White {
		intermediateSquare, finalSquare = D1, C1
	} else {
		intermediateSquare, finalSquare = D8, C8
	}
	return !isSquareAttacked(pos, intermediateSquare, color.Other()) &&
		!isSquareAttacked(pos, finalSquare, color.Other())
}

// IsLegalMove checks if a move is legal (doesn't leave king in check)
func IsLegalMove(pos *Position, move Move) bool {
	if move.IsCastle() {
		return isLegalCastle(pos, move)
	}

	// For non-castling moves, use the standard approach
	// Make the move
	ep, tag, hc, _ := pos.MakeMove(move)

	// Check if our king is in check after the move
	legal := !isInCheck(pos, move.MovingPiece().Color())

	// Unmake the move
	pos.UnMakeMove(move, tag, ep, hc)

	return legal
}

// isSquareAttacked checks if a square is attacked by pieces of the given color
func isSquareAttacked(pos *Position, square Square, attackingColor Color) bool {
	// Temporarily place a king of the opposite color on the square
	originalPiece := pos.Board.PieceAt(square)
	var testKing Piece
	if attackingColor == White {
		testKing = BlackKing
	} else {
		testKing = WhiteKing
	}

	// Find and temporarily remove the existing king to avoid having two kings
	testKingColor := testKing.Color()
	var existingKingSquare Square = 64 // Invalid square initially
	var existingKingPiece Piece

	kingBitboard := pos.Board.GetBitboardOf(testKing)
	if kingBitboard != 0 {
		existingKingSquare = Square(bits.TrailingZeros64(kingBitboard))
		existingKingPiece = pos.Board.PieceAt(existingKingSquare)
		pos.Board.UpdateSquare(existingKingSquare, NoPiece, existingKingPiece)
	}

	// Place test king on target square
	pos.Board.UpdateSquare(square, testKing, originalPiece)

	// Check if test king is attacked
	attacked := isInCheck(pos, testKingColor)

	// Restore both pieces
	pos.Board.UpdateSquare(square, originalPiece, testKing)
	if existingKingSquare < 64 {
		pos.Board.UpdateSquare(existingKingSquare, existingKingPiece, NoPiece)
	}

	return attacked
}

// IsInCheckDirect checks if the king of the given color is in check (public version)
func IsInCheckDirect(pos *Position, color Color) bool {
	return isInCheck(pos, color)
}

// isInCheck checks if the king of the given color is in check
func isInCheck(pos *Position, color Color) bool {
	// Find the king
	var kingPiece Piece
	if color == White {
		kingPiece = WhiteKing
	} else {
		kingPiece = BlackKing
	}

	kingBitboard := pos.Board.GetBitboardOf(kingPiece)
	if kingBitboard == 0 {
		return false // No king found
	}

	kingSquare := Square(bits.TrailingZeros64(kingBitboard))

	// Safety check for valid square
	if kingSquare > 63 {
		return false // Invalid square
	}

	// Check if any enemy piece can attack the king
	enemyColor := color.Other()

	// Check pawn attacks
	if color == White {
		// White king being attacked by black pawns - check squares where black pawns would be to attack king
		// Black pawns attack from squares king+7 and king+9
		leftAttack := (SquareMask[kingSquare] << 7) & ^FileMasks[FileH]  // mask FileH to prevent H->A wrap
		rightAttack := (SquareMask[kingSquare] << 9) & ^FileMasks[FileA] // mask FileA to prevent A->H wrap
		if (leftAttack|rightAttack)&pos.Board.GetBitboardOf(BlackPawn) != 0 {
			return true
		}
	} else {
		// Black king being attacked by white pawns - check squares where white pawns would be to attack king
		// White pawns attack from squares king-7 and king-9, but we need to prevent wrapping
		// For king-7: prevent wrapping from file A to file H
		leftAttack := (SquareMask[kingSquare] >> 7) & ^FileMasks[FileA] // Exclude if king is on file A
		// For king-9: prevent wrapping from file H to file A
		rightAttack := (SquareMask[kingSquare] >> 9) & ^FileMasks[FileH] // Exclude if king is on file H
		if (leftAttack|rightAttack)&pos.Board.GetBitboardOf(WhitePawn) != 0 {
			return true
		}
	}

	// Enemy pieces are the White piece constants plus a color offset (Black pieces are
	// +6 in the enum), so compute the offset once instead of re-branching inside
	// GetPiece for each of the five lookups below.
	var enemyBase Piece
	if enemyColor == Black {
		enemyBase = 6
	}

	// Check knight attacks
	enemyKnights := pos.Board.GetBitboardOf(WhiteKnight + enemyBase)
	if KnightAttacks[kingSquare]&enemyKnights != 0 {
		return true
	}

	// Check king attacks (for adjacent king positions)
	enemyKing := pos.Board.GetBitboardOf(WhiteKing + enemyBase)
	if KingAttacks[kingSquare]&enemyKing != 0 {
		return true
	}

	// Check sliding piece attacks using magic bitboards - O(1) lookups
	occupied := pos.Board.GetWhitePieces() | pos.Board.GetBlackPieces()

	// Enemy queens attack on both diagonals and ranks/files; fetch once and reuse.
	enemyQueens := pos.Board.GetBitboardOf(WhiteQueen + enemyBase)

	// Check diagonal attacks (bishop, queen)
	diagonalAttackers := pos.Board.GetBitboardOf(WhiteBishop+enemyBase) | enemyQueens
	if GetBishopAttacks(int(kingSquare), occupied)&diagonalAttackers != 0 {
		return true
	}

	// Check straight attacks (rook, queen)
	straightAttackers := pos.Board.GetBitboardOf(WhiteRook+enemyBase) | enemyQueens
	if GetRookAttacks(int(kingSquare), occupied)&straightAttackers != 0 {
		return true
	}

	return false
}

// Buffer-based move generation to eliminate allocations (safe optimization)

// GenerateMovesIntoBuffer generates moves into the provided buffer, returning the number of moves added
func GenerateMovesIntoBuffer(pos *Position, buffer []Move) int {
	moveCount := 0
	turn := pos.Turn()

	if turn == White {
		moveCount += generateWhitePawnMovesIntoBuffer(pos, buffer[moveCount:])
		moveCount += generatePieceMovesIntoBuffer(pos, WhiteKnight, generateKnightMovesIntoBuffer, buffer[moveCount:])
		moveCount += generatePieceMovesIntoBuffer(pos, WhiteBishop, generateBishopMovesIntoBuffer, buffer[moveCount:])
		moveCount += generatePieceMovesIntoBuffer(pos, WhiteRook, generateRookMovesIntoBuffer, buffer[moveCount:])
		moveCount += generatePieceMovesIntoBuffer(pos, WhiteQueen, generateQueenMovesIntoBuffer, buffer[moveCount:])
		moveCount += generateKingMovesIntoBuffer(pos, WhiteKing, buffer[moveCount:])
	} else {
		moveCount += generateBlackPawnMovesIntoBuffer(pos, buffer[moveCount:])
		moveCount += generatePieceMovesIntoBuffer(pos, BlackKnight, generateKnightMovesIntoBuffer, buffer[moveCount:])
		moveCount += generatePieceMovesIntoBuffer(pos, BlackBishop, generateBishopMovesIntoBuffer, buffer[moveCount:])
		moveCount += generatePieceMovesIntoBuffer(pos, BlackRook, generateRookMovesIntoBuffer, buffer[moveCount:])
		moveCount += generatePieceMovesIntoBuffer(pos, BlackQueen, generateQueenMovesIntoBuffer, buffer[moveCount:])
		moveCount += generateKingMovesIntoBuffer(pos, BlackKing, buffer[moveCount:])
	}

	return moveCount
}

// GenerateCapturesIntoBuffer generates captures into the provided buffer, returning the number of captures added
func GenerateCapturesIntoBuffer(pos *Position, buffer []Move) int {
	captureCount := 0
	turn := pos.Turn()

	if turn == White {
		if captureCount < len(buffer) {
			captureCount += generateWhitePawnCapturesIntoBuffer(pos, buffer[captureCount:])
		}
		if captureCount < len(buffer) {
			captureCount += generatePieceCapturesIntoBuffer(pos, WhiteKnight, generateKnightCapturesIntoBuffer, buffer[captureCount:])
		}
		if captureCount < len(buffer) {
			captureCount += generatePieceCapturesIntoBuffer(pos, WhiteBishop, generateBishopCapturesIntoBuffer, buffer[captureCount:])
		}
		if captureCount < len(buffer) {
			captureCount += generatePieceCapturesIntoBuffer(pos, WhiteRook, generateRookCapturesIntoBuffer, buffer[captureCount:])
		}
		if captureCount < len(buffer) {
			captureCount += generatePieceCapturesIntoBuffer(pos, WhiteQueen, generateQueenCapturesIntoBuffer, buffer[captureCount:])
		}
		if captureCount < len(buffer) {
			captureCount += generateKingCapturesIntoBuffer(pos, WhiteKing, buffer[captureCount:])
		}
	} else {
		if captureCount < len(buffer) {
			captureCount += generateBlackPawnCapturesIntoBuffer(pos, buffer[captureCount:])
		}
		if captureCount < len(buffer) {
			captureCount += generatePieceCapturesIntoBuffer(pos, BlackKnight, generateKnightCapturesIntoBuffer, buffer[captureCount:])
		}
		if captureCount < len(buffer) {
			captureCount += generatePieceCapturesIntoBuffer(pos, BlackBishop, generateBishopCapturesIntoBuffer, buffer[captureCount:])
		}
		if captureCount < len(buffer) {
			captureCount += generatePieceCapturesIntoBuffer(pos, BlackRook, generateRookCapturesIntoBuffer, buffer[captureCount:])
		}
		if captureCount < len(buffer) {
			captureCount += generatePieceCapturesIntoBuffer(pos, BlackQueen, generateQueenCapturesIntoBuffer, buffer[captureCount:])
		}
		if captureCount < len(buffer) {
			captureCount += generateKingCapturesIntoBuffer(pos, BlackKing, buffer[captureCount:])
		}
	}

	return captureCount
}

// Helper functions for buffer-based generation

func generatePieceMovesIntoBuffer(pos *Position, piece Piece, generator func(*Position, Square, []Move) int, buffer []Move) int {
	moveCount := 0
	pieceBitboard := pos.Board.GetBitboardOf(piece)

	for pieceBitboard != 0 {
		square := Square(bits.TrailingZeros64(pieceBitboard))
		moveCount += generator(pos, square, buffer[moveCount:])
		pieceBitboard &^= SquareMask[int(square)]
	}

	return moveCount
}

func generatePieceCapturesIntoBuffer(pos *Position, piece Piece, generator func(*Position, Square, []Move) int, buffer []Move) int {
	captureCount := 0
	if len(buffer) == 0 {
		return 0
	}
	pieceBitboard := pos.Board.GetBitboardOf(piece)

	for pieceBitboard != 0 && captureCount < len(buffer) {
		square := Square(bits.TrailingZeros64(pieceBitboard))
		captureCount += generator(pos, square, buffer[captureCount:])
		pieceBitboard &^= SquareMask[int(square)]
	}

	return captureCount
}

// Buffer-based piece generators (using existing move generation - safe fallback)

func generateKnightMovesIntoBuffer(pos *Position, from Square, buffer []Move) int {
	count := 0
	attacks := KnightAttacks[from]
	piece := pos.Board.PieceAt(from)
	friendlyPieces := pos.Board.GetWhitePieces()
	if piece.Color() == Black {
		friendlyPieces = pos.Board.GetBlackPieces()
	}

	attacks &^= friendlyPieces

	for attacks != 0 {
		to := Square(bits.TrailingZeros64(attacks))
		capturedPiece := pos.Board.PieceAt(to)
		tag := MoveTag(0)
		if capturedPiece != NoPiece {
			tag = Capture
		}
		buffer[count] = NewMove(from, to, piece, capturedPiece, NoType, tag)
		count++
		attacks &^= SquareMask[int(to)]
	}

	return count
}

func generateBishopMovesIntoBuffer(pos *Position, from Square, buffer []Move) int {
	return generateSlidingMovesIntoBuffer(pos, from, bishopDirections, buffer)
}

func generateRookMovesIntoBuffer(pos *Position, from Square, buffer []Move) int {
	return generateSlidingMovesIntoBuffer(pos, from, rookDirections, buffer)
}

func generateQueenMovesIntoBuffer(pos *Position, from Square, buffer []Move) int {
	return generateSlidingMovesIntoBuffer(pos, from, queenDirections, buffer)
}

func generateKnightCapturesIntoBuffer(pos *Position, from Square, buffer []Move) int {
	count := 0
	if len(buffer) == 0 {
		return 0
	}
	attacks := KnightAttacks[from]
	piece := pos.Board.PieceAt(from)
	enemyPieces := pos.Board.GetBlackPieces()
	if piece.Color() == Black {
		enemyPieces = pos.Board.GetWhitePieces()
	}
	attacks &= enemyPieces
	for attacks != 0 && count < len(buffer) {
		to := Square(bits.TrailingZeros64(attacks))
		capturedPiece := pos.Board.PieceAt(to)
		buffer[count] = NewMove(from, to, piece, capturedPiece, NoType, Capture)
		count++
		attacks &^= SquareMask[int(to)]
	}
	return count
}

func generateBishopCapturesIntoBuffer(pos *Position, from Square, buffer []Move) int {
	return generateSlidingCapturesIntoBuffer(pos, from, bishopDirections, buffer)
}

func generateRookCapturesIntoBuffer(pos *Position, from Square, buffer []Move) int {
	return generateSlidingCapturesIntoBuffer(pos, from, rookDirections, buffer)
}

func generateQueenCapturesIntoBuffer(pos *Position, from Square, buffer []Move) int {
	return generateSlidingCapturesIntoBuffer(pos, from, queenDirections, buffer)
}

func generateWhitePawnMovesIntoBuffer(pos *Position, buffer []Move) int {
	count := 0
	whitePawns := pos.Board.GetBitboardOf(WhitePawn)
	allPieces := pos.Board.GetWhitePieces() | pos.Board.GetBlackPieces()
	enemyPieces := pos.Board.GetBlackPieces()

	// Single pushes
	singlePushes := (whitePawns << 8) & ^allPieces
	sp := singlePushes
	for sp != 0 {
		to := Square(bits.TrailingZeros64(sp))
		from := Square(int(to) - 8)
		if to.Rank() == Rank8 {
			buffer[count] = NewMove(from, to, WhitePawn, NoPiece, Queen, 0)
			count++
			buffer[count] = NewMove(from, to, WhitePawn, NoPiece, Rook, 0)
			count++
			buffer[count] = NewMove(from, to, WhitePawn, NoPiece, Bishop, 0)
			count++
			buffer[count] = NewMove(from, to, WhitePawn, NoPiece, Knight, 0)
			count++
		} else {
			buffer[count] = NewMove(from, to, WhitePawn, NoPiece, NoType, 0)
			count++
		}
		sp &^= SquareMask[int(to)]
	}
	// Double pushes
	doublePushes := ((singlePushes & RankMasks[Rank3]) << 8) & ^allPieces
	dp := doublePushes
	for dp != 0 {
		to := Square(bits.TrailingZeros64(dp))
		from := Square(int(to) - 16)
		buffer[count] = NewMove(from, to, WhitePawn, NoPiece, NoType, 0)
		count++
		dp &^= SquareMask[int(to)]
	}

	// Captures
	leftCaptures := ((whitePawns & ^FileMasks[FileA]) << 7) & enemyPieces
	lc := leftCaptures
	for lc != 0 {
		to := Square(bits.TrailingZeros64(lc))
		from := Square(int(to) - 7)
		captured := pos.Board.PieceAt(to)
		if to.Rank() == Rank8 {
			buffer[count] = NewMove(from, to, WhitePawn, captured, Queen, Capture)
			count++
			buffer[count] = NewMove(from, to, WhitePawn, captured, Rook, Capture)
			count++
			buffer[count] = NewMove(from, to, WhitePawn, captured, Bishop, Capture)
			count++
			buffer[count] = NewMove(from, to, WhitePawn, captured, Knight, Capture)
			count++
		} else {
			buffer[count] = NewMove(from, to, WhitePawn, captured, NoType, Capture)
			count++
		}
		lc &^= SquareMask[int(to)]
	}
	rightCaptures := ((whitePawns & ^FileMasks[FileH]) << 9) & enemyPieces
	rc := rightCaptures
	for rc != 0 {
		to := Square(bits.TrailingZeros64(rc))
		from := Square(int(to) - 9)
		captured := pos.Board.PieceAt(to)
		if to.Rank() == Rank8 {
			buffer[count] = NewMove(from, to, WhitePawn, captured, Queen, Capture)
			count++
			buffer[count] = NewMove(from, to, WhitePawn, captured, Rook, Capture)
			count++
			buffer[count] = NewMove(from, to, WhitePawn, captured, Bishop, Capture)
			count++
			buffer[count] = NewMove(from, to, WhitePawn, captured, Knight, Capture)
			count++
		} else {
			buffer[count] = NewMove(from, to, WhitePawn, captured, NoType, Capture)
			count++
		}
		rc &^= SquareMask[int(to)]
	}

	// En passant
	if pos.EnPassant != NoSquare {
		ep := pos.EnPassant
		if ep.File() > FileA {
			left := SquareOf(ep.File()-1, ep.Rank()-1)
			if pos.Board.PieceAt(left) == WhitePawn {
				buffer[count] = NewMove(left, ep, WhitePawn, BlackPawn, NoType, EnPassant|Capture)
				count++
			}
		}
		if ep.File() < FileH {
			right := SquareOf(ep.File()+1, ep.Rank()-1)
			if pos.Board.PieceAt(right) == WhitePawn {
				buffer[count] = NewMove(right, ep, WhitePawn, BlackPawn, NoType, EnPassant|Capture)
				count++
			}
		}
	}

	return count
}

func generateBlackPawnMovesIntoBuffer(pos *Position, buffer []Move) int {
	count := 0
	blackPawns := pos.Board.GetBitboardOf(BlackPawn)
	allPieces := pos.Board.GetWhitePieces() | pos.Board.GetBlackPieces()
	enemyPieces := pos.Board.GetWhitePieces()

	// Single pushes
	singlePushes := (blackPawns >> 8) & ^allPieces
	sp := singlePushes
	for sp != 0 {
		to := Square(bits.TrailingZeros64(sp))
		from := Square(int(to) + 8)
		if to.Rank() == Rank1 {
			buffer[count] = NewMove(from, to, BlackPawn, NoPiece, Queen, 0)
			count++
			buffer[count] = NewMove(from, to, BlackPawn, NoPiece, Rook, 0)
			count++
			buffer[count] = NewMove(from, to, BlackPawn, NoPiece, Bishop, 0)
			count++
			buffer[count] = NewMove(from, to, BlackPawn, NoPiece, Knight, 0)
			count++
		} else {
			buffer[count] = NewMove(from, to, BlackPawn, NoPiece, NoType, 0)
			count++
		}
		sp &^= SquareMask[int(to)]
	}
	// Double pushes
	doublePushes := ((singlePushes & RankMasks[Rank6]) >> 8) & ^allPieces
	dp := doublePushes
	for dp != 0 {
		to := Square(bits.TrailingZeros64(dp))
		from := Square(int(to) + 16)
		buffer[count] = NewMove(from, to, BlackPawn, NoPiece, NoType, 0)
		count++
		dp &^= SquareMask[int(to)]
	}

	// Captures
	leftCaptures := ((blackPawns & ^FileMasks[FileH]) >> 7) & enemyPieces
	lc := leftCaptures
	for lc != 0 {
		to := Square(bits.TrailingZeros64(lc))
		from := Square(int(to) + 7)
		captured := pos.Board.PieceAt(to)
		if to.Rank() == Rank1 {
			buffer[count] = NewMove(from, to, BlackPawn, captured, Queen, Capture)
			count++
			buffer[count] = NewMove(from, to, BlackPawn, captured, Rook, Capture)
			count++
			buffer[count] = NewMove(from, to, BlackPawn, captured, Bishop, Capture)
			count++
			buffer[count] = NewMove(from, to, BlackPawn, captured, Knight, Capture)
			count++
		} else {
			buffer[count] = NewMove(from, to, BlackPawn, captured, NoType, Capture)
			count++
		}
		lc &^= SquareMask[int(to)]
	}
	rightCaptures := ((blackPawns & ^FileMasks[FileA]) >> 9) & enemyPieces
	rc := rightCaptures
	for rc != 0 {
		to := Square(bits.TrailingZeros64(rc))
		from := Square(int(to) + 9)
		captured := pos.Board.PieceAt(to)
		if to.Rank() == Rank1 {
			buffer[count] = NewMove(from, to, BlackPawn, captured, Queen, Capture)
			count++
			buffer[count] = NewMove(from, to, BlackPawn, captured, Rook, Capture)
			count++
			buffer[count] = NewMove(from, to, BlackPawn, captured, Bishop, Capture)
			count++
			buffer[count] = NewMove(from, to, BlackPawn, captured, Knight, Capture)
			count++
		} else {
			buffer[count] = NewMove(from, to, BlackPawn, captured, NoType, Capture)
			count++
		}
		rc &^= SquareMask[int(to)]
	}

	// En passant
	if pos.EnPassant != NoSquare {
		ep := pos.EnPassant
		if ep.File() > FileA {
			left := SquareOf(ep.File()-1, ep.Rank()+1)
			if pos.Board.PieceAt(left) == BlackPawn {
				buffer[count] = NewMove(left, ep, BlackPawn, WhitePawn, NoType, EnPassant|Capture)
				count++
			}
		}
		if ep.File() < FileH {
			right := SquareOf(ep.File()+1, ep.Rank()+1)
			if pos.Board.PieceAt(right) == BlackPawn {
				buffer[count] = NewMove(right, ep, BlackPawn, WhitePawn, NoType, EnPassant|Capture)
				count++
			}
		}
	}

	return count
}

func generateWhitePawnCapturesIntoBuffer(pos *Position, buffer []Move) int {
	count := 0
	if len(buffer) == 0 {
		return 0
	}
	whitePawns := pos.Board.GetBitboardOf(WhitePawn)
	enemyPieces := pos.Board.GetBlackPieces()

	leftCaptures := ((whitePawns & ^FileMasks[FileA]) << 7) & enemyPieces
	lc := leftCaptures
	for lc != 0 && count < len(buffer) {
		to := Square(bits.TrailingZeros64(lc))
		from := Square(int(to) - 7)
		captured := pos.Board.PieceAt(to)
		if to.Rank() == Rank8 {
			if count+3 < len(buffer) {
				buffer[count] = NewMove(from, to, WhitePawn, captured, Queen, Capture)
				count++
				buffer[count] = NewMove(from, to, WhitePawn, captured, Rook, Capture)
				count++
				buffer[count] = NewMove(from, to, WhitePawn, captured, Bishop, Capture)
				count++
				buffer[count] = NewMove(from, to, WhitePawn, captured, Knight, Capture)
				count++
			}
		} else {
			buffer[count] = NewMove(from, to, WhitePawn, captured, NoType, Capture)
			count++
		}
		lc &^= SquareMask[int(to)]
	}

	rightCaptures := ((whitePawns & ^FileMasks[FileH]) << 9) & enemyPieces
	rc := rightCaptures
	for rc != 0 && count < len(buffer) {
		to := Square(bits.TrailingZeros64(rc))
		from := Square(int(to) - 9)
		captured := pos.Board.PieceAt(to)
		if to.Rank() == Rank8 {
			if count+3 < len(buffer) {
				buffer[count] = NewMove(from, to, WhitePawn, captured, Queen, Capture)
				count++
				buffer[count] = NewMove(from, to, WhitePawn, captured, Rook, Capture)
				count++
				buffer[count] = NewMove(from, to, WhitePawn, captured, Bishop, Capture)
				count++
				buffer[count] = NewMove(from, to, WhitePawn, captured, Knight, Capture)
				count++
			}
		} else {
			buffer[count] = NewMove(from, to, WhitePawn, captured, NoType, Capture)
			count++
		}
		rc &^= SquareMask[int(to)]
	}

	// En passant
	if pos.EnPassant != NoSquare && count < len(buffer) {
		ep := pos.EnPassant
		if ep.File() > FileA {
			left := SquareOf(ep.File()-1, ep.Rank()-1)
			if pos.Board.PieceAt(left) == WhitePawn {
				buffer[count] = NewMove(left, ep, WhitePawn, BlackPawn, NoType, EnPassant|Capture)
				count++
			}
		}
		if ep.File() < FileH && count < len(buffer) {
			right := SquareOf(ep.File()+1, ep.Rank()-1)
			if pos.Board.PieceAt(right) == WhitePawn {
				buffer[count] = NewMove(right, ep, WhitePawn, BlackPawn, NoType, EnPassant|Capture)
				count++
			}
		}
	}

	return count
}

func generateBlackPawnCapturesIntoBuffer(pos *Position, buffer []Move) int {
	count := 0
	if len(buffer) == 0 {
		return 0
	}
	blackPawns := pos.Board.GetBitboardOf(BlackPawn)
	enemyPieces := pos.Board.GetWhitePieces()

	leftCaptures := ((blackPawns & ^FileMasks[FileH]) >> 7) & enemyPieces
	lc := leftCaptures
	for lc != 0 && count < len(buffer) {
		to := Square(bits.TrailingZeros64(lc))
		from := Square(int(to) + 7)
		captured := pos.Board.PieceAt(to)
		if to.Rank() == Rank1 {
			if count+3 < len(buffer) {
				buffer[count] = NewMove(from, to, BlackPawn, captured, Queen, Capture)
				count++
				buffer[count] = NewMove(from, to, BlackPawn, captured, Rook, Capture)
				count++
				buffer[count] = NewMove(from, to, BlackPawn, captured, Bishop, Capture)
				count++
				buffer[count] = NewMove(from, to, BlackPawn, captured, Knight, Capture)
				count++
			}
		} else {
			buffer[count] = NewMove(from, to, BlackPawn, captured, NoType, Capture)
			count++
		}
		lc &^= SquareMask[int(to)]
	}

	rightCaptures := ((blackPawns & ^FileMasks[FileA]) >> 9) & enemyPieces
	rc := rightCaptures
	for rc != 0 && count < len(buffer) {
		to := Square(bits.TrailingZeros64(rc))
		from := Square(int(to) + 9)
		captured := pos.Board.PieceAt(to)
		if to.Rank() == Rank1 {
			if count+3 < len(buffer) {
				buffer[count] = NewMove(from, to, BlackPawn, captured, Queen, Capture)
				count++
				buffer[count] = NewMove(from, to, BlackPawn, captured, Rook, Capture)
				count++
				buffer[count] = NewMove(from, to, BlackPawn, captured, Bishop, Capture)
				count++
				buffer[count] = NewMove(from, to, BlackPawn, captured, Knight, Capture)
				count++
			}
		} else {
			buffer[count] = NewMove(from, to, BlackPawn, captured, NoType, Capture)
			count++
		}
		rc &^= SquareMask[int(to)]
	}

	// En passant
	if pos.EnPassant != NoSquare && count < len(buffer) {
		ep := pos.EnPassant
		if ep.File() > FileA {
			left := SquareOf(ep.File()-1, ep.Rank()+1)
			if pos.Board.PieceAt(left) == BlackPawn {
				buffer[count] = NewMove(left, ep, BlackPawn, WhitePawn, NoType, EnPassant|Capture)
				count++
			}
		}
		if ep.File() < FileH && count < len(buffer) {
			right := SquareOf(ep.File()+1, ep.Rank()+1)
			if pos.Board.PieceAt(right) == BlackPawn {
				buffer[count] = NewMove(right, ep, BlackPawn, WhitePawn, NoType, EnPassant|Capture)
				count++
			}
		}
	}

	return count
}

func generateKingMovesIntoBuffer(pos *Position, piece Piece, buffer []Move) int {
	count := 0
	kingBB := pos.Board.GetBitboardOf(piece)
	if kingBB == 0 {
		return 0
	}
	from := Square(bits.TrailingZeros64(kingBB))
	attacks := KingAttacks[from]
	friendly := pos.Board.GetWhitePieces()
	if piece.Color() == Black {
		friendly = pos.Board.GetBlackPieces()
	}
	attacks &^= friendly
	for attacks != 0 {
		to := Square(bits.TrailingZeros64(attacks))
		captured := pos.Board.PieceAt(to)
		tag := MoveTag(0)
		if captured != NoPiece {
			tag = Capture
		}
		buffer[count] = NewMove(from, to, piece, captured, NoType, tag)
		count++
		attacks &^= SquareMask[int(to)]
	}
	// Castling (pseudo-legal)
	if piece == WhiteKing && from == E1 {
		if pos.HasTag(WhiteCanCastleKingSide) && pos.Board.PieceAt(F1) == NoPiece && pos.Board.PieceAt(G1) == NoPiece && pos.Board.PieceAt(H1) == WhiteRook {
			buffer[count] = NewMove(E1, G1, WhiteKing, NoPiece, NoType, KingSideCastle)
			count++
		}
		if pos.HasTag(WhiteCanCastleQueenSide) && pos.Board.PieceAt(D1) == NoPiece && pos.Board.PieceAt(C1) == NoPiece && pos.Board.PieceAt(B1) == NoPiece && pos.Board.PieceAt(A1) == WhiteRook {
			buffer[count] = NewMove(E1, C1, WhiteKing, NoPiece, NoType, QueenSideCastle)
			count++
		}
	} else if piece == BlackKing && from == E8 {
		if pos.HasTag(BlackCanCastleKingSide) && pos.Board.PieceAt(F8) == NoPiece && pos.Board.PieceAt(G8) == NoPiece && pos.Board.PieceAt(H8) == BlackRook {
			buffer[count] = NewMove(E8, G8, BlackKing, NoPiece, NoType, KingSideCastle)
			count++
		}
		if pos.HasTag(BlackCanCastleQueenSide) && pos.Board.PieceAt(D8) == NoPiece && pos.Board.PieceAt(C8) == NoPiece && pos.Board.PieceAt(B8) == NoPiece && pos.Board.PieceAt(A8) == BlackRook {
			buffer[count] = NewMove(E8, C8, BlackKing, NoPiece, NoType, QueenSideCastle)
			count++
		}
	}
	return count
}

func generateKingCapturesIntoBuffer(pos *Position, piece Piece, buffer []Move) int {
	count := 0
	if len(buffer) == 0 {
		return 0
	}
	kingBB := pos.Board.GetBitboardOf(piece)
	if kingBB == 0 {
		return 0
	}
	from := Square(bits.TrailingZeros64(kingBB))
	attacks := KingAttacks[from]
	enemy := pos.Board.GetBlackPieces()
	if piece.Color() == Black {
		enemy = pos.Board.GetWhitePieces()
	}
	attacks &= enemy
	for attacks != 0 && count < len(buffer) {
		to := Square(bits.TrailingZeros64(attacks))
		captured := pos.Board.PieceAt(to)
		buffer[count] = NewMove(from, to, piece, captured, NoType, Capture)
		count++
		attacks &^= SquareMask[int(to)]
	}
	return count
}
