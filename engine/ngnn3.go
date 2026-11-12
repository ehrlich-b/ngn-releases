package engine

// A context is the perspective's input row block and horizontal orientation.
// NGNN1/2 use the zero context, preserving their feature indices.
type ngnn3Context struct {
	bucket uint8
	mirror uint8 // 0 or 7, XORed into the square bits only
}

func (c ngnn3Context) feature(base int) int {
	return int(c.bucket)*ngnn1Features + (base ^ int(c.mirror))
}

func (n *NGNN1Network) kingInputs() bool { return n.version == 3 || n.version == 6 }

func (n *NGNN1Network) featureCount() int {
	if n.kingInputs() || n.version == 5 {
		return n.kingBuckets * ngnn1Features
	}
	return ngnn1Features
}

func (n *NGNN1Network) validKingLayout() bool {
	if !n.kingInputs() && n.version != 5 {
		return true
	}
	if n.kingBuckets < 1 || n.kingBuckets > 8 {
		return false
	}
	for _, b := range n.kingMap {
		if int(b) >= n.kingBuckets {
			return false
		}
	}
	return true
}

func (n *NGNN1Network) contextAt(king Square, perspective Color) ngnn3Context {
	k := int(king)
	if perspective == Black {
		k ^= 56
	}
	mirror := uint8(0)
	if k&7 >= 4 {
		mirror = 7
		k ^= 7
	}
	return ngnn3Context{bucket: n.kingMap[k], mirror: mirror}
}

func (n *NGNN1Network) kingContext(pos *Position, perspective Color) ngnn3Context {
	k := bitScanForward(pos.Board.pieces[GetPiece(King, perspective)])
	// Defensive fallback for diagnostic positions without kings. Training
	// requires exactly one king per side; legal engine positions never use this.
	if k == 64 {
		k = 0
	}
	return n.contextAt(Square(k), perspective)
}

// Build bucket-relative rows directly instead of returning an ordinary delta
// and then walking its rows again. Removed and added rows use the old king
// context; a crossing king move refreshes that perspective in the new context.
func (n *NGNN1Network) ngnn3MoveDelta(pos *Position, move Move) ngnn1Delta {
	var d ngnn1Delta
	var contexts [2]ngnn3Context
	for p, color := range [...]Color{White, Black} {
		context := n.kingContext(pos, color)
		contexts[p] = context
		if move.MovingPiece() == GetPiece(King, color) {
			next := n.contextAt(move.Destination(), color)
			if next != context {
				d.refreshMask |= 1 << p
				d.contexts[p] = next
			}
		}
	}
	remove := func(piece Piece, square Square) {
		d.remove[0][d.removes] = uint16(contexts[0].feature(ngnn1Feature(piece, square, White)))
		d.remove[1][d.removes] = uint16(contexts[1].feature(ngnn1Feature(piece, square, Black)))
		d.removes++
	}
	add := func(piece Piece, square Square) {
		d.add[0][d.adds] = uint16(contexts[0].feature(ngnn1Feature(piece, square, White)))
		d.add[1][d.adds] = uint16(contexts[1].feature(ngnn1Feature(piece, square, Black)))
		d.adds++
	}
	moving := move.MovingPiece()
	remove(moving, move.Source())
	placed := moving
	if move.PromoType() != NoType {
		placed = GetPiece(move.PromoType(), moving.Color())
	}
	add(placed, move.Destination())
	if captured := move.CapturedPiece(); captured != NoPiece {
		square := move.Destination()
		if move.IsEnPassant() {
			square = enPassantVictimSquare(move)
		}
		remove(captured, square)
	}
	if move.IsCastle() {
		from, to := H1, F1
		if move.IsQueenSideCastle() {
			from, to = A1, D1
		}
		if moving.Color() == Black {
			from ^= 56
			to ^= 56
		}
		remove(GetPiece(Rook, moving.Color()), from)
		add(GetPiece(Rook, moving.Color()), to)
	}
	return d
}

// Each worker owns a lazily populated (perspective, bucket, mirror) cache.
// The masks are in board coordinates; only changed pieces touch input rows.
// Cache entries may describe any previous branch, independent of stack undo.
type ngnn3CacheEntry struct {
	acc     []int32
	compact []int16
	pieces  [13]uint64
}

func (s *ngnn1State) cachedRefresh(p int, context ngnn3Context, pos *Position, dst []int32) {
	n := s.network
	if s.refreshCache == nil {
		s.refreshCache = make([]ngnn3CacheEntry, 4*n.kingBuckets)
	}
	index := (p*n.kingBuckets + int(context.bucket)) * 2
	if context.mirror != 0 {
		index++
	}
	entry := &s.refreshCache[index]
	if entry.acc == nil {
		entry.acc = make([]int32, n.hidden)
		for i, b := range n.bias {
			entry.acc[i] = int32(b)
		}
	}
	color := White
	if p == 1 {
		color = Black
	}
	for piece := WhitePawn; piece <= BlackKing; piece++ {
		before, after := entry.pieces[piece], pos.Board.pieces[piece]
		removed, added := before&^after, after&^before
		// Pair changed squares of the same piece type to read and write the
		// accumulator once. The cache can come from any previous search branch;
		// unmatched removals/additions still use the independent row kernels.
		for removed != 0 && added != 0 {
			r := context.feature(ngnn1Feature(piece, Square(bitScanForward(removed)), color))
			a := context.feature(ngnn1Feature(piece, Square(bitScanForward(added)), color))
			ngnn1MoveRows(entry.acc, entry.acc, n.row(r), n.row(a))
			removed &= removed - 1
			added &= added - 1
		}
		for ; removed != 0; removed &= removed - 1 {
			feature := context.feature(ngnn1Feature(piece, Square(bitScanForward(removed)), color))
			ngnn1SubRow(entry.acc, n.row(feature))
		}
		for ; added != 0; added &= added - 1 {
			feature := context.feature(ngnn1Feature(piece, Square(bitScanForward(added)), color))
			ngnn1AddRow(entry.acc, n.row(feature))
		}
	}
	entry.pieces = pos.Board.pieces
	copy(dst, entry.acc)
}
