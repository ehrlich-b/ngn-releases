package engine

// Every square contains either no feature or one of twelve piece features.
// Summing each square's independent extrema overbounds even malformed boards.
// Bucket, rank and file transforms only permute these square terms. Once the
// complete accumulator fits int16, modular word adds/subtracts recover it
// exactly even if an intermediate update wraps. Saturating adds cannot do so.
func (n *NGNN1Network) compactInputBound() bool {
	for bucket := 0; bucket < n.kingBuckets; bucket++ {
		for lane, bias := range n.bias {
			low, high := int(bias), int(bias)
			for square := 0; square < 64; square++ {
				lo, hi := 0, 0
				for piece := 0; piece < 12; piece++ {
					w := int(n.weights[(bucket*768+piece*64+square)*n.hidden+lane])
					lo, hi = min(lo, w), max(hi, w)
				}
				low, high = low+lo, high+hi
			}
			if low < -32768 || high > 32767 {
				return false
			}
		}
	}
	return true
}

func (s *ngnn1State) ensureCompact(depth int) {
	width := 2 * s.network.hidden
	if (depth+1)*width <= len(s.compactStack) {
		return
	}
	plies := max(128, 2*len(s.compactStack)/width, depth+1)
	next := make([]int16, plies*width)
	copy(next, s.compactStack)
	frames := make([]ngnn1Frame, plies)
	for ply := range frames {
		slot := ply
		if ply < len(s.frames) {
			slot = s.frames[ply].slot
		}
		frames[ply] = ngnn1Frame{slot: slot, compact: next[slot*width : (slot+1)*width]}
	}
	s.compactStack, s.frames = next, frames
	pending := make([]ngnnCompactPending, plies)
	copy(pending, s.pending)
	s.pending = pending
}

// Existing diagnostics read int32 accumulators. This view is never touched
// by search, and aliases by physical slot just as null frames do.
func (s *ngnn1State) compactView(depth int) []int32 {
	if len(s.wideView) < len(s.compactStack) {
		s.wideView = make([]int32, len(s.compactStack))
	}
	frame := s.frames[depth]
	width := 2 * s.network.hidden
	view := s.wideView[frame.slot*width : (frame.slot+1)*width]
	for i, x := range frame.compact {
		view[i] = int32(x)
	}
	return view
}

func (s *ngnn1State) resetCompact(pos *Position) {
	h := s.network.hidden
	s.pending[0] = ngnnCompactPending{valid: true}
	// Reset the root slot after prior searches containing null aliases.
	s.frames[0] = ngnn1Frame{slot: 0, compact: s.compactStack[:2*h]}
	for p, color := range [...]Color{White, Black} {
		s.cachedRefreshCompact(p, s.network.kingContext(pos, color), pos, s.frames[0].compact[p*h:(p+1)*h])
	}
}

// Pending refreshes retain only the piece bitboards needed by the refresh cache.
type ngnnCompactPending struct {
	delta  ngnn1Delta
	pieces [13]uint64
	valid  bool
}

func (s *ngnn1State) recordCompact(depth int, delta ngnn1Delta, pos *Position) {
	p := &s.pending[depth]
	*p = ngnnCompactPending{delta: delta}
	if delta.refreshMask != 0 {
		p.pieces = pos.Board.pieces
	}
	if delta.removes == 0 && delta.adds == 0 {
		s.frames[depth] = s.frames[depth-1]
		p.valid = s.pending[depth-1].valid
	}
}

func (s *ngnn1State) materializeCompact(depth int) {
	start := depth
	for !s.pending[start].valid {
		start--
	}
	for ply := start + 1; ply <= depth; ply++ {
		p := &s.pending[ply]
		var pos Position
		pos.Board.pieces = p.pieces
		s.pushCompact(ply, p.delta, &pos)
		p.valid = true
	}
}

func (s *ngnn1State) pushCompact(depth int, delta ngnn1Delta, pos *Position) {
	if delta.removes == 0 && delta.adds == 0 {
		s.frames[depth] = s.frames[depth-1]
		return
	}
	n := s.network
	width := 2 * n.hidden
	child := s.compactStack[depth*width : (depth+1)*width]
	s.frames[depth] = ngnn1Frame{slot: depth, compact: child}
	parent := s.frames[depth-1].compact
	if delta.refreshMask == 0 && delta.removes == 1 && delta.adds == 1 {
		ngnn16MovePair(child, parent,
			n.row(int(delta.remove[0][0])), n.row(int(delta.add[0][0])),
			n.row(int(delta.remove[1][0])), n.row(int(delta.add[1][0])))
		return
	}
	if delta.refreshMask == 0 && delta.removes == 2 && delta.adds == 1 {
		ngnn16CapturePair(child, parent,
			n.row(int(delta.remove[0][0])), n.row(int(delta.remove[0][1])), n.row(int(delta.add[0][0])),
			n.row(int(delta.remove[1][0])), n.row(int(delta.remove[1][1])), n.row(int(delta.add[1][0])))
		return
	}
	for p := 0; p < 2; p++ {
		a := child[p*n.hidden : (p+1)*n.hidden]
		if delta.refreshMask&(1<<p) != 0 {
			s.cachedRefreshCompact(p, delta.contexts[p], pos, a)
			continue
		}
		src := parent[p*n.hidden : (p+1)*n.hidden]
		switch {
		case delta.removes == 1 && delta.adds == 1:
			ngnn16MoveRows(a, src, n.row(int(delta.remove[p][0])), n.row(int(delta.add[p][0])))
		case delta.removes == 2 && delta.adds == 2:
			ngnn16CastleRows(a, src, n.row(int(delta.remove[p][0])), n.row(int(delta.remove[p][1])), n.row(int(delta.add[p][0])), n.row(int(delta.add[p][1])))
		default:
			copy(a, src)
			for i := 0; i < delta.removes; i++ {
				ngnn16SubRow(a, n.row(int(delta.remove[p][i])))
			}
			for i := 0; i < delta.adds; i++ {
				ngnn16AddRow(a, n.row(int(delta.add[p][i])))
			}
		}
	}
}

func (s *ngnn1State) cachedRefreshCompact(p int, context ngnn3Context, pos *Position, dst []int16) {
	n := s.network
	if s.refreshCache == nil {
		s.refreshCache = make([]ngnn3CacheEntry, 4*n.kingBuckets)
	}
	index := (p*n.kingBuckets + int(context.bucket)) * 2
	if context.mirror != 0 {
		index++
	}
	entry := &s.refreshCache[index]
	if entry.compact == nil {
		entry.compact = append([]int16(nil), n.bias...)
	}
	color := White
	if p == 1 {
		color = Black
	}
	for piece := WhitePawn; piece <= BlackKing; piece++ {
		before, after := entry.pieces[piece], pos.Board.pieces[piece]
		removed, added := before&^after, after&^before
		for removed != 0 && added != 0 {
			r := context.feature(ngnn1Feature(piece, Square(bitScanForward(removed)), color))
			a := context.feature(ngnn1Feature(piece, Square(bitScanForward(added)), color))
			ngnn16MoveRows(entry.compact, entry.compact, n.row(r), n.row(a))
			removed &= removed - 1
			added &= added - 1
		}
		for ; removed != 0; removed &= removed - 1 {
			feature := context.feature(ngnn1Feature(piece, Square(bitScanForward(removed)), color))
			ngnn16SubRow(entry.compact, n.row(feature))
		}
		for ; added != 0; added &= added - 1 {
			feature := context.feature(ngnn1Feature(piece, Square(bitScanForward(added)), color))
			ngnn16AddRow(entry.compact, n.row(feature))
		}
	}
	entry.pieces = pos.Board.pieces
	copy(dst, entry.compact)
}

func (s *ngnn1State) evaluatePosition(depth int, pos *Position, n *NGNN1Network) int {
	if !n.compactInput {
		return n.evaluateBucket(s.at(depth), pos.Turn(), n.bucket(pos))
	}
	s.materializeCompact(depth)
	acc := s.frames[depth].compact
	us, them := acc[:n.hidden], acc[n.hidden:]
	if pos.Turn() == Black {
		us, them = them, us
	}
	bucket := n.bucket(pos)
	start := bucket * 2 * n.hidden
	sum := ngnn16OutputSmall(us, them, n.output[start:start+2*n.hidden])
	bias := n.outputBias
	if bucket > 0 {
		bias = n.otherBiases[bucket-1]
	}
	v := sum/ngnn1QA + int64(bias)
	cp := v * ngnn1Scale / (ngnn1QA * ngnn1QB)
	if n.residual != nil {
		cp += ngnn6ResidualCP(n.residual, us, them, bucket)
	}
	return int(min(ngnn1ScoreLimit, max(-ngnn1ScoreLimit, cp)))
}
