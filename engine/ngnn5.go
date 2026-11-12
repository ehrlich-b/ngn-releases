package engine

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"sort"
)

// ConvertNGNN3ToNGNN5 preserves every NGNN3 coefficient and adds zero threats.
func ConvertNGNN3ToNGNN5(dst io.Writer, src io.Reader) error {
	n, err := ReadNGNN1(src)
	if err != nil {
		return err
	}
	if n.version != 3 {
		return fmt.Errorf("NGNN5 converter: input must be NGNN3")
	}
	n.version = 5
	return writeNGNN5(dst, n)
}

func writeNGNN5(dst io.Writer, n *NGNN1Network) error {
	if !n.valid() || n.version != 5 {
		return fmt.Errorf("NGNN5: invalid network")
	}
	keys := make([]int, 0, len(n.threatWeights))
	for key, weights := range n.threatWeights {
		if int(key) >= n.kingBuckets*ngnn5Features || len(weights) != n.hidden {
			return fmt.Errorf("NGNN5: invalid threat row")
		}
		nonzero := false
		for _, w := range weights {
			nonzero = nonzero || w != 0
		}
		if !nonzero {
			return fmt.Errorf("NGNN5: explicit zero row")
		}
		keys = append(keys, int(key))
	}
	sort.Ints(keys)
	header := make([]byte, 112)
	copy(header, "NGN5")
	for i, value := range []int{5, n.hidden, n.kingBuckets, n.bucketCount(), 255, 64, 400} {
		binary.LittleEndian.PutUint32(header[4+4*i:], uint32(value))
	}
	copy(header[32:96], n.kingMap[:])
	binary.LittleEndian.PutUint32(header[96:], uint32(ngnn5Features))
	binary.LittleEndian.PutUint32(header[100:], uint32(len(keys)))
	binary.LittleEndian.PutUint32(header[104:], 1)
	buffer := bufio.NewWriter(dst)
	crc := crc32.NewIEEE()
	w := io.MultiWriter(buffer, crc)
	if _, err := w.Write(header); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, n.weights); err != nil {
		return err
	}
	for _, key := range keys {
		if err := binary.Write(w, binary.LittleEndian, uint32(key)); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, n.threatWeights[uint32(key)]); err != nil {
			return err
		}
	}
	for _, values := range [][]int16{n.bias, n.output} {
		if err := binary.Write(w, binary.LittleEndian, values); err != nil {
			return err
		}
	}
	if err := binary.Write(w, binary.LittleEndian, n.outputBias); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, n.otherBiases); err != nil {
		return err
	}
	if err := binary.Write(buffer, binary.LittleEndian, crc.Sum32()); err != nil {
		return err
	}
	return buffer.Flush()
}

// NGNN5 keeps NGNN3's piece-square block and adds sparse int8 threat rows.
// An attacker belongs to the perspective; targets may be friends or enemies.
// This also represents defenses, without duplicating the attacker's color.
var ngnn5Layout, ngnn5Features = buildNGNN5Layout()

func ngnn5Geometry(kind PieceType, from, to int) bool {
	dr, df := to/8-from/8, to%8-from%8
	ar, af := abs(dr), abs(df)
	if from == to {
		return false
	}
	switch kind {
	case Pawn:
		return from/8 > 0 && from/8 < 7 && dr == 1 && af == 1
	case Knight:
		return ar*af == 2
	case Bishop:
		return ar == af
	case Rook:
		return dr == 0 || df == 0
	case Queen:
		return dr == 0 || df == 0 || ar == af
	case King:
		return ar <= 1 && af <= 1
	}
	return false
}

func buildNGNN5Layout() ([6][64][12][64]int32, int) {
	var layout [6][64][12][64]int32
	count := 0
	for a := range layout {
		for from := 0; from < 64; from++ {
			for victim := 0; victim < 12; victim++ {
				for to := 0; to < 64; to++ {
					layout[a][from][victim][to] = -1
					if !ngnn5Geometry(PieceType(a+1), from, to) ||
						(victim%6 == 0 && (to/8 == 0 || to/8 == 7)) ||
						(a == 5 && victim%6 == 5) {
						continue
					}
					layout[a][from][victim][to] = int32(count)
					count++
				}
			}
		}
	}
	return layout, count
}

func ngnn5Feature(attacker Piece, from Square, victim Piece, to Square, perspective Color, context ngnn3Context) int {
	if attacker.Color() != perspective {
		return -1
	}
	f, t := int(from), int(to)
	if perspective == Black {
		f ^= 56
		t ^= 56
	}
	f ^= int(context.mirror)
	t ^= int(context.mirror)
	v := int(victim.Type()) - 1
	if victim.Color() != perspective {
		v += 6
	}
	row := int(ngnn5Layout[int(attacker.Type())-1][f][v][t])
	if row < 0 {
		return -1
	}
	return int(context.bucket)*ngnn5Features + row
}

func ngnn5Attacks(piece Piece, from Square, occupied uint64) uint64 {
	switch piece.Type() {
	case Knight:
		return KnightAttacks[from]
	case Bishop:
		return GetBishopAttacks(int(from), occupied)
	case Rook:
		return GetRookAttacks(int(from), occupied)
	case Queen:
		return GetQueenAttacks(int(from), occupied)
	case King:
		return KingAttacks[from]
	case Pawn:
		step := 8
		if piece.Color() == Black {
			step = -8
		}
		rank := int(from) / 8
		if rank == 0 || rank == 7 {
			return 0
		}
		var attacks uint64
		if from%8 > 0 {
			attacks |= uint64(1) << uint(int(from)+step-1)
		}
		if from%8 < 7 {
			attacks |= uint64(1) << uint(int(from)+step+1)
		}
		return attacks
	}
	return 0
}

func (n *NGNN1Network) ngnn5AddThreat(acc []int32, attacker Piece, from Square, victim Piece, to Square, color Color, context ngnn3Context, sign int32) {
	row := ngnn5Feature(attacker, from, victim, to, color, context)
	if row < 0 {
		return
	}
	for i, weight := range n.threatWeights[uint32(row)] {
		acc[i] += sign * int32(weight)
	}
}

func (n *NGNN1Network) ngnn5RefreshPerspective(pos *Position, color Color, acc []int32) {
	context := n.kingContext(pos, color)
	for i, b := range n.bias {
		acc[i] = int32(b)
	}
	occupied := pos.Board.WhiteOccupancy() | pos.Board.BlackOccupancy()
	for square, piece := range pos.Board.mailbox {
		if !validPiece(piece) {
			continue
		}
		ngnn1AddRow(acc, n.row(context.feature(ngnn1Feature(piece, Square(square), color))))
		if piece.Color() != color || len(n.threatWeights) == 0 {
			continue
		}
		for targets := ngnn5Attacks(piece, Square(square), occupied) & occupied; targets != 0; targets &= targets - 1 {
			to := Square(bitScanForward(targets))
			n.ngnn5AddThreat(acc, piece, Square(square), pos.Board.mailbox[to], to, color, context, 1)
		}
	}
}

// Changed occupancy can expose distant sliders as well as move an attacker or
// replace a victim. Only attackers whose empty-board reach meets a changed
// square need examination. Compare their occupied attack edges exactly; this
// includes friendly defenses, promotion identities and both castling pieces.
// Undo restores the untouched parent frame, including across null aliases.
func (s *ngnn1State) pushNGNN5(depth int, delta ngnn1Delta, pos *Position, before *Bitboard) {
	s.ensure(depth)
	n := s.network
	width := 2 * n.hidden
	child := s.stack[depth*width : (depth+1)*width]
	s.frames[depth] = ngnn1Frame{values: child, slot: depth}
	parent := s.at(depth - 1)
	var changed uint64
	for square, piece := range before.mailbox {
		if piece != pos.Board.mailbox[square] {
			changed |= uint64(1) << uint(square)
		}
	}
	oldOccupied := before.WhiteOccupancy() | before.BlackOccupancy()
	newOccupied := pos.Board.WhiteOccupancy() | pos.Board.BlackOccupancy()
	for p, color := range [...]Color{White, Black} {
		acc := child[p*n.hidden : (p+1)*n.hidden]
		if delta.refreshMask&(1<<p) != 0 {
			n.ngnn5RefreshPerspective(pos, color, acc)
			continue
		}
		copy(acc, parent[p*n.hidden:(p+1)*n.hidden])
		for i := 0; i < delta.removes; i++ {
			ngnn1SubRow(acc, n.row(int(delta.remove[p][i])))
		}
		for i := 0; i < delta.adds; i++ {
			ngnn1AddRow(acc, n.row(int(delta.add[p][i])))
		}
		if len(n.threatWeights) == 0 {
			continue
		}
		context := n.kingContext(pos, color)
		attackers := before.occupiedByColor[color] | pos.Board.occupiedByColor[color]
		for ; attackers != 0; attackers &= attackers - 1 {
			from := Square(bitScanForward(attackers))
			oldPiece, newPiece := before.mailbox[from], pos.Board.mailbox[from]
			if oldPiece == newPiece && ngnn5Attacks(oldPiece, from, 0)&changed == 0 {
				continue
			}
			var oldTargets, newTargets uint64
			if validPiece(oldPiece) && oldPiece.Color() == color {
				oldTargets = ngnn5Attacks(oldPiece, from, oldOccupied) & oldOccupied
			}
			if validPiece(newPiece) && newPiece.Color() == color {
				newTargets = ngnn5Attacks(newPiece, from, newOccupied) & newOccupied
			}
			for targets := oldTargets | newTargets; targets != 0; targets &= targets - 1 {
				to := Square(bitScanForward(targets))
				mask := uint64(1) << uint(to)
				oldVictim, newVictim := before.mailbox[to], pos.Board.mailbox[to]
				if oldTargets&newTargets&mask != 0 && oldPiece == newPiece && oldVictim == newVictim {
					continue
				}
				if oldTargets&mask != 0 {
					n.ngnn5AddThreat(acc, oldPiece, from, oldVictim, to, color, context, -1)
				}
				if newTargets&mask != 0 {
					n.ngnn5AddThreat(acc, newPiece, from, newVictim, to, color, context, 1)
				}
			}
		}
	}
}

// ngnn5ActiveFeatures counts active piece and threat inputs over both
// perspectives, independent of whether a network stores a weight for a row.
func (n *NGNN1Network) ngnn5ActiveFeatures(pos *Position) (pieces, threats int) {
	occupied := pos.Board.WhiteOccupancy() | pos.Board.BlackOccupancy()
	for square, piece := range pos.Board.mailbox {
		if !validPiece(piece) {
			continue
		}
		pieces += 2
		color := piece.Color()
		context := n.kingContext(pos, color)
		for targets := ngnn5Attacks(piece, Square(square), occupied) & occupied; targets != 0; targets &= targets - 1 {
			to := Square(bitScanForward(targets))
			if ngnn5Feature(piece, Square(square), pos.Board.mailbox[to], to, color, context) >= 0 {
				threats++
			}
		}
	}
	return
}

// readNGNN5 validates dimensions before allocation, then ordered sparse rows,
// constants, CRC and EOF. A missing threat row is exactly zero.
func readNGNN5(r io.Reader, prefix []byte) (*NGNN1Network, error) {
	header := make([]byte, 112)
	copy(header, prefix)
	if _, err := io.ReadFull(r, header[len(prefix):]); err != nil {
		return nil, fmt.Errorf("NGNN5 header: %w", err)
	}
	u32 := func(i int) int { return int(binary.LittleEndian.Uint32(header[i:])) }
	h, kb, nb, rows := u32(8), u32(12), u32(16), u32(100)
	if h < 16 || h > ngnn1MaxHidden || h%16 != 0 || kb < 1 || kb > 8 ||
		(nb != 1 && nb != 2 && nb != 4 && nb != 8) || u32(20) != 255 || u32(24) != 64 ||
		u32(28) != 400 || u32(96) != ngnn5Features || rows > kb*ngnn5Features ||
		u32(104) != 1 || u32(108) != 0 {
		return nil, fmt.Errorf("NGNN5: invalid layout")
	}
	var kingMap [64]uint8
	copy(kingMap[:], header[32:96])
	for _, b := range kingMap {
		if int(b) >= kb {
			return nil, fmt.Errorf("NGNN5: invalid king map")
		}
	}
	n := &NGNN1Network{version: 5, hidden: h, kingBuckets: kb, buckets: nb, kingMap: kingMap, threatWeights: make(map[uint32][]int8)}
	crc := crc32.NewIEEE()
	_, _ = crc.Write(header)
	input := io.TeeReader(r, crc)
	read16 := func(count int) ([]int16, error) {
		data := make([]byte, 2*count)
		if _, err := io.ReadFull(input, data); err != nil {
			return nil, err
		}
		values := make([]int16, count)
		for i := range values {
			values[i] = int16(binary.LittleEndian.Uint16(data[2*i:]))
		}
		return values, nil
	}
	var err error
	if n.weights, err = read16(kb * ngnn1Features * h); err != nil {
		return nil, fmt.Errorf("NGNN5 pieces: %w", err)
	}
	previous := -1
	for i := 0; i < rows; i++ {
		var index [4]byte
		if _, err := io.ReadFull(input, index[:]); err != nil {
			return nil, fmt.Errorf("NGNN5 index: %w", err)
		}
		row := binary.LittleEndian.Uint32(index[:])
		if int(row) <= previous || int(row) >= kb*ngnn5Features {
			return nil, fmt.Errorf("NGNN5: unordered/out-of-range row")
		}
		previous = int(row)
		data := make([]byte, h)
		if _, err := io.ReadFull(input, data); err != nil {
			return nil, fmt.Errorf("NGNN5 threat: %w", err)
		}
		weights := make([]int8, h)
		nonzero := false
		for j, w := range data {
			weights[j] = int8(w)
			nonzero = nonzero || w != 0
		}
		if !nonzero {
			return nil, fmt.Errorf("NGNN5: explicit zero row")
		}
		n.threatWeights[row] = weights
	}
	if n.bias, err = read16(h); err != nil {
		return nil, fmt.Errorf("NGNN5 bias: %w", err)
	}
	if n.output, err = read16(nb * 2 * h); err != nil {
		return nil, fmt.Errorf("NGNN5 output: %w", err)
	}
	n.smallOutput = true
	for _, w := range n.output {
		if w < -128 || w > 128 {
			n.smallOutput = false
		}
	}
	biases := make([]byte, nb*4)
	if _, err := io.ReadFull(input, biases); err != nil {
		return nil, fmt.Errorf("NGNN5 output bias: %w", err)
	}
	n.outputBias = int32(binary.LittleEndian.Uint32(biases))
	n.otherBiases = make([]int32, nb-1)
	for i := range n.otherBiases {
		n.otherBiases[i] = int32(binary.LittleEndian.Uint32(biases[4*(i+1):]))
	}
	var checksum [4]byte
	if _, err := io.ReadFull(r, checksum[:]); err != nil {
		return nil, fmt.Errorf("NGNN5 CRC: %w", err)
	}
	if binary.LittleEndian.Uint32(checksum[:]) != crc.Sum32() {
		return nil, fmt.Errorf("NGNN5: CRC mismatch")
	}
	var extra [1]byte
	if count, err := io.ReadFull(r, extra[:]); count != 0 || err != io.EOF {
		return nil, fmt.Errorf("NGNN5: trailing data")
	}
	return n, nil
}
