package engine

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"os"
)

const (
	ngnn1Features   = 768
	ngnn1MaxHidden  = 2048
	ngnn1QA         = 255
	ngnn1QB         = 64
	ngnn1Scale      = 400
	ngnn1ScoreLimit = 24999
)

// NGNN1Network owns validated, immutable NGNN1, NGNN2 or NGNN3 weights. Workers share this
// object; their accumulators never live here. All arithmetic remains exact
// even for the full int16 weight range permitted by the file format.
type NGNN1Network struct {
	hidden      int
	version     int
	buckets     int // zero means one for existing in-package single-row networks
	kingBuckets int
	kingMap     [64]uint8
	weights     []int16
	bias        []int16
	output      []int16
	outputBias  int32 // first bucket, also the sole NGNN1 output bias
	otherBiases []int32
	smallOutput bool // immutable loader-derived bound: every output weight is within [-128, 128]
}

func (n *NGNN1Network) valid() bool {
	return n != nil && n.hidden > 0 && n.hidden <= ngnn1MaxHidden && n.hidden%16 == 0 &&
		(n.bucketCount() == 1 || n.buckets == 2 || n.buckets == 4 || n.buckets == 8) &&
		n.validKingLayout() && len(n.weights) == n.featureCount()*n.hidden && len(n.bias) == n.hidden &&
		len(n.output) == n.bucketCount()*2*n.hidden && len(n.otherBiases) == n.bucketCount()-1
}

func (n *NGNN1Network) bucketCount() int {
	if n.buckets == 0 {
		return 1
	}
	return n.buckets
}

// LoadNGNN1 accepts NGNN1/2/3, preserving the original API. No partial network is
// returned after an I/O, header, length, or checksum error.
func LoadNGNN1(path string) (*NGNN1Network, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("NGNN1: %w", err)
	}
	defer f.Close()
	return ReadNGNN1(f)
}

// ReadNGNN1 bounds allocation from a validated header before reading weights.
func ReadNGNN1(r io.Reader) (*NGNN1Network, error) {
	var storage [96]byte
	header := storage[:24]
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, fmt.Errorf("NGNN1 header: %w", err)
	}
	version, nb, kb, constants := int(binary.LittleEndian.Uint32(header[4:8])), 1, 1, 12
	var kingMap [64]uint8
	switch {
	case string(header[:4]) == "NGNN" && version == 1:
	case string(header[:4]) == "NGN2" && version == 2:
		header = storage[:28]
		if _, err := io.ReadFull(r, header[24:]); err != nil {
			return nil, fmt.Errorf("NGNN2 header: %w", err)
		}
		nb = int(binary.LittleEndian.Uint32(header[12:16]))
		if nb != 1 && nb != 2 && nb != 4 && nb != 8 {
			return nil, fmt.Errorf("NGNN2: invalid bucket count %d", nb)
		}
		constants = 16
	case string(header[:4]) == "NGN3" && version == 3:
		header = storage[:96]
		if _, err := io.ReadFull(r, header[24:]); err != nil {
			return nil, fmt.Errorf("NGNN3 header: %w", err)
		}
		kb = int(binary.LittleEndian.Uint32(header[12:16]))
		nb = int(binary.LittleEndian.Uint32(header[16:20]))
		if kb < 1 || kb > 8 || (nb != 1 && nb != 2 && nb != 4 && nb != 8) {
			return nil, fmt.Errorf("NGNN3: invalid king/output bucket counts %d/%d", kb, nb)
		}
		copy(kingMap[:], header[32:96])
		for _, bucket := range kingMap {
			if int(bucket) >= kb {
				return nil, fmt.Errorf("NGNN3: king bucket map entry out of range")
			}
		}
		constants = 20
	default:
		return nil, fmt.Errorf("NGNN: unsupported magic/version")
	}
	h := int(binary.LittleEndian.Uint32(header[8:12]))
	if h <= 0 || h > ngnn1MaxHidden || h%16 != 0 {
		return nil, fmt.Errorf("NGNN1: invalid hidden size %d", h)
	}
	if binary.LittleEndian.Uint32(header[constants:]) != ngnn1QA || binary.LittleEndian.Uint32(header[constants+4:]) != ngnn1QB || binary.LittleEndian.Uint32(header[constants+8:]) != ngnn1Scale {
		return nil, fmt.Errorf("NGNN1: invalid quantization constants")
	}
	size := len(header) + 2*(kb*ngnn1Features*h+h+nb*2*h) + nb*4 + 4
	data := make([]byte, size)
	copy(data, header)
	if _, err := io.ReadFull(r, data[len(header):]); err != nil {
		return nil, fmt.Errorf("NGNN1 weights: %w", err)
	}
	var extra [1]byte
	if count, err := io.ReadFull(r, extra[:]); count != 0 || err != io.EOF {
		return nil, fmt.Errorf("NGNN1: trailing data or read error: %v", err)
	}
	if crc32.ChecksumIEEE(data[:size-4]) != binary.LittleEndian.Uint32(data[size-4:]) {
		return nil, fmt.Errorf("NGNN1: CRC mismatch")
	}
	offset := len(header)
	readWeights := func(count int) []int16 {
		values := make([]int16, count)
		for i := range values {
			values[i] = int16(binary.LittleEndian.Uint16(data[offset : offset+2]))
			offset += 2
		}
		return values
	}
	n := &NGNN1Network{hidden: h, version: version, buckets: nb, kingBuckets: kb, kingMap: kingMap}
	n.weights = readWeights(kb * ngnn1Features * h)
	n.bias = readWeights(h)
	n.output = readWeights(nb * 2 * h)
	n.smallOutput = true
	for _, w := range n.output {
		if w < -128 || w > 128 {
			n.smallOutput = false
			break
		}
	}
	n.outputBias = int32(binary.LittleEndian.Uint32(data[offset : offset+4]))
	n.otherBiases = make([]int32, nb-1)
	for i := range n.otherBiases {
		offset += 4
		n.otherBiases[i] = int32(binary.LittleEndian.Uint32(data[offset : offset+4]))
	}
	return n, nil
}

func ngnn1Feature(piece Piece, square Square, perspective Color) int {
	s := int(square)
	if perspective == Black {
		s ^= 56
	}
	rel := 0
	if piece.Color() != perspective {
		rel = 384
	}
	return rel + (int(piece.Type())-1)*64 + s
}

func (n *NGNN1Network) row(feature int) []int16 {
	start := feature * n.hidden
	return n.weights[start : start+n.hidden]
}

// refresh writes White then Black, independently of engine Color numbering.
// int32 covers B1 + all 64 squares at the extreme int16 weight values.
func (n *NGNN1Network) refresh(pos *Position, accum []int32) {
	for p, color := range [...]Color{White, Black} {
		context := ngnn3Context{}
		if n.version == 3 {
			context = n.kingContext(pos, color)
		}
		a := accum[p*n.hidden : (p+1)*n.hidden]
		for i, b := range n.bias {
			a[i] = int32(b)
		}
		for square, piece := range pos.Board.mailbox {
			if validPiece(piece) {
				ngnn1AddRow(a, n.row(context.feature(ngnn1Feature(piece, Square(square), color))))
			}
		}
	}
}

func (n *NGNN1Network) evaluate(acc []int32, turn Color) int {
	return n.evaluateBucket(acc, turn, 0)
}

// bucket uses b = min(NB-1, (popcount(all pieces)-2)*NB/32), with integer
// division. Both kings are counted. Malformed positions below two pieces map
// to zero defensively; positions above 32 pieces saturate at the last bucket.
func (n *NGNN1Network) bucket(pos *Position) int {
	nb := n.bucketCount()
	if nb == 1 {
		return 0
	}
	count := PopCount(pos.Board.whitePieces | pos.Board.blackPieces)
	return min(nb-1, max(0, (count-2)*nb/32))
}

func (n *NGNN1Network) evaluateBucket(acc []int32, turn Color, bucket int) int {
	us, them := acc[:n.hidden], acc[n.hidden:2*n.hidden]
	if turn == Black {
		us, them = them, us
	}
	start := bucket * 2 * n.hidden
	output := n.output[start : start+2*n.hidden]
	bias := n.outputBias
	if bucket > 0 {
		bias = n.otherBiases[bucket-1]
	}
	// At H=2048 even extreme output weights give |sum| < 2^44.
	var sum int64
	if n.smallOutput {
		sum = ngnn1OutputSmall(us, them, output)
	} else {
		sum = ngnn1Dot(us, output[:n.hidden]) + ngnn1Dot(them, output[n.hidden:])
	}
	v := sum/ngnn1QA + int64(bias)
	cp := v * ngnn1Scale / (ngnn1QA * ngnn1QB)
	if cp > ngnn1ScoreLimit {
		return ngnn1ScoreLimit
	}
	if cp < -ngnn1ScoreLimit {
		return -ngnn1ScoreLimit
	}
	return int(cp)
}

type ngnn1Delta struct {
	remove        [2][3]uint16
	add           [2][2]uint16
	removes, adds int
	refreshMask   uint8
	contexts      [2]ngnn3Context
}

func ngnn1MoveDelta(move Move) ngnn1Delta {
	var d ngnn1Delta
	remove := func(piece Piece, square Square) {
		d.remove[0][d.removes] = uint16(ngnn1Feature(piece, square, White))
		d.remove[1][d.removes] = uint16(ngnn1Feature(piece, square, Black))
		d.removes++
	}
	add := func(piece Piece, square Square) {
		d.add[0][d.adds] = uint16(ngnn1Feature(piece, square, White))
		d.add[1][d.adds] = uint16(ngnn1Feature(piece, square, Black))
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
			square = findEnPassantCaptureSquare(move)
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

type ngnn1State struct {
	network      *NGNN1Network
	stack        []int32
	refreshCache []ngnn3CacheEntry
}

func (s *ngnn1State) at(depth int) []int32 {
	width := 2 * s.network.hidden
	return s.stack[depth*width : (depth+1)*width]
}

func (s *ngnn1State) ensure(depth int) {
	width := 2 * s.network.hidden
	if (depth+1)*width > len(s.stack) {
		plies := 128
		if size := 2 * len(s.stack) / width; size > plies {
			plies = size
		}
		if depth+1 > plies {
			plies = depth + 1
		}
		next := make([]int32, plies*width)
		copy(next, s.stack)
		s.stack = next
	}
}

func (s *ngnn1State) reset(pos *Position) {
	s.ensure(0)
	if s.network.version == 3 {
		for p, color := range [...]Color{White, Black} {
			s.cachedRefresh(p, s.network.kingContext(pos, color), pos, s.at(0)[p*s.network.hidden:(p+1)*s.network.hidden])
		}
		return
	}
	s.network.refresh(pos, s.at(0))
}

func (s *ngnn1State) push(depth int, delta ngnn1Delta) {
	s.pushPosition(depth, delta, nil)
}

func (s *ngnn1State) pushPosition(depth int, delta ngnn1Delta, pos *Position) {
	s.ensure(depth)
	parent, child := s.at(depth-1), s.at(depth)
	if delta.removes == 0 && delta.adds == 0 {
		copy(child, parent)
		return // Null move: the turn changes; the features do not.
	}
	n := s.network
	for p := 0; p < 2; p++ {
		a := child[p*n.hidden : (p+1)*n.hidden]
		if delta.refreshMask&(1<<p) != 0 {
			s.cachedRefresh(p, delta.contexts[p], pos, a)
			continue
		}
		src := parent[p*n.hidden : (p+1)*n.hidden]
		switch {
		case delta.removes == 1 && delta.adds == 1:
			ngnn1MoveRows(a, src, n.row(int(delta.remove[p][0])), n.row(int(delta.add[p][0])))
		case delta.removes == 2 && delta.adds == 1:
			ngnn1CaptureRows(a, src, n.row(int(delta.remove[p][0])), n.row(int(delta.remove[p][1])), n.row(int(delta.add[p][0])))
		case delta.removes == 2 && delta.adds == 2:
			ngnn1CastleRows(a, src, n.row(int(delta.remove[p][0])), n.row(int(delta.remove[p][1])), n.row(int(delta.add[p][0])), n.row(int(delta.add[p][1])))
		default:
			copy(a, src)
			for i := 0; i < delta.removes; i++ {
				ngnn1SubRow(a, n.row(int(delta.remove[p][i])))
			}
			for i := 0; i < delta.adds; i++ {
				ngnn1AddRow(a, n.row(int(delta.add[p][i])))
			}
		}
	}
}
