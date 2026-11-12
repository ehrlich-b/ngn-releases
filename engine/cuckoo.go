package engine

// Upcoming repetitions use Marcel van Kervinck's published cuckoo-table idea:
// XOR two position keys to recover a reversible move's key difference. This
// implementation enumerates NGN's own pieces, squares and Zobrist keys; no
// foreign engine tables, key stream or source are used.
const cuckooSize = 8192

type cuckooEntry struct {
	key  uint64
	move Move // Unordered endpoints; the board determines the direction.
}

var cuckooTable [cuckooSize]cuckooEntry

func cuckooSlots(key uint64) (int, int) {
	return int(key & (cuckooSize - 1)), int((key >> 32) & (cuckooSize - 1))
}

// initCuckoo is called only after hash.go has assigned all position keys.
// Geometry is computed directly so startup does not depend on attack-table
// initialization order. Pawns, captures, promotions and castling are excluded.
func initCuckoo() {
	for _, piece := range Pieces {
		if piece.Type() == Pawn {
			continue
		}
		for from := A1; from <= H8; from++ {
			for to := from + 1; to <= H8; to++ {
				df := abs(int(from.File()) - int(to.File()))
				dr := abs(int(from.Rank()) - int(to.Rank()))
				var reachable bool
				switch piece.Type() {
				case Knight:
					reachable = df*dr == 2
				case Bishop:
					reachable = df == dr
				case Rook:
					reachable = df == 0 || dr == 0
				case Queen:
					reachable = df == dr || df == 0 || dr == 0
				case King:
					reachable = df <= 1 && dr <= 1
				}
				if !reachable {
					continue
				}
				entry := cuckooEntry{
					key:  occupantKey(piece, from) ^ occupantKey(piece, to) ^ whiteSideKey,
					move: NewMove(from, to, piece, NoPiece, NoType, 0),
				}
				slot, _ := cuckooSlots(entry.key)
				for displaced := 0; ; displaced++ {
					if displaced == cuckooSize {
						panic("NGN repetition cuckoo table cannot place key")
					}
					entry, cuckooTable[slot] = cuckooTable[slot], entry
					if entry.move == EmptyMove {
						break
					}
					first, second := cuckooSlots(entry.key)
					if slot == first {
						slot = second
					} else {
						slot = first
					}
				}
			}
		}
	}
}

func cuckooMove(key uint64) Move {
	first, second := cuckooSlots(key)
	for _, slot := range [2]int{first, second} {
		if entry := cuckooTable[slot]; entry.move != EmptyMove && entry.key == key {
			return entry.move
		}
	}
	return EmptyMove
}

// upcomingRepetition inspects ancestors only, before the current node is
// pushed. An ancestor three plies back is the earliest possible match: a
// reversible move closes a cycle of at least four plies. Null moves start a
// new segment, and the halfmove clock bounds the scan after pawn moves/captures.
func upcomingRepetition(pos *Position, hash uint64, info *SearchInfo) Move {
	lo := info.RepStackLen - int(pos.HalfMoveClock)
	if lo < info.cuckooStart {
		lo = info.cuckooStart
	}
	for i := info.RepStackLen - 3; i >= lo; i -= 2 {
		target := info.RepStack[i]
		move := cuckooMove(hash ^ target)
		if move == EmptyMove {
			continue
		}
		piece := move.MovingPiece()
		if pos.Board.PieceAt(move.Source()) != piece {
			move = NewMove(move.Destination(), move.Source(), piece, NoPiece, NoType, 0)
		}
		// This checks side to move, the empty destination and slider blockers.
		if !moveIsPseudoLegal(pos, move) {
			continue
		}
		// Validate king safety and the complete resulting key. The latter also
		// rejects a change to castling rights or legally relevant en passant.
		ep, tag, clock, _ := pos.MakeMove(move)
		repeats := !isInCheck(pos, piece.Color()) && pos.Hash() == target
		pos.UnMakeMove(move, tag, ep, clock)
		if repeats {
			return move
		}
	}
	return EmptyMove
}

// cuckooRepetitionMove leaves root and move-excluded verification searches alone.
func cuckooRepetitionMove(pos *Position, ply, alpha int, hash uint64, info *SearchInfo) Move {
	if T1104Cuckoo == 0 || ply == 0 || alpha >= 0 ||
		(info.ExcludedMove != EmptyMove && ply == info.ExcludedPly) {
		return EmptyMove
	}
	return upcomingRepetition(pos, hash, info)
}
