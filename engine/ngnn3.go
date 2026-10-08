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

func (n *NGNN1Network) featureCount() int {
	if n.version == 3 {
		return n.kingBuckets * ngnn1Features
	}
	return ngnn1Features
}

func (n *NGNN1Network) validKingLayout() bool {
	if n.version != 3 {
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

func (n *NGNN1Network) kingMoveDelta(pos *Position, move Move, d ngnn1Delta) ngnn1Delta {
	for p, color := range [...]Color{White, Black} {
		context := n.kingContext(pos, color)
		if move.MovingPiece() == GetPiece(King, color) {
			next := n.contextAt(move.Destination(), color)
			if next != context {
				d.refreshMask |= 1 << p
				d.contexts[p] = next
			}
		}
		for i := 0; i < d.removes; i++ {
			d.remove[p][i] = uint16(context.feature(int(d.remove[p][i])))
		}
		for i := 0; i < d.adds; i++ {
			d.add[p][i] = uint16(context.feature(int(d.add[p][i])))
		}
	}
	return d
}

// Each worker owns a lazily populated (perspective, bucket, mirror) cache.
// The masks are in board coordinates; only changed pieces touch input rows.
// Cache entries may describe any previous branch, independent of stack undo.
type ngnn3CacheEntry struct {
	acc    []int32
	pieces [13]uint64
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
		for removed := before &^ after; removed != 0; removed &= removed - 1 {
			feature := context.feature(ngnn1Feature(piece, Square(bitScanForward(removed)), color))
			ngnn1SubRow(entry.acc, n.row(feature))
		}
		for added := after &^ before; added != 0; added &= added - 1 {
			feature := context.feature(ngnn1Feature(piece, Square(bitScanForward(added)), color))
			ngnn1AddRow(entry.acc, n.row(feature))
		}
	}
	entry.pieces = pos.Board.pieces
	copy(dst, entry.acc)
}
