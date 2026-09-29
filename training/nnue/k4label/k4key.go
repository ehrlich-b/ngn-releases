package k4label

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/bits"
	"sort"

	"github.com/ehrlich-b/ngn/engine"
)

const K4InputKeyContract = "sha256(ngn-k4-input-v1\\0 || stm_count:u16le || sorted_stm_rows:u16le[] || ntm_count:u16le || sorted_ntm_rows:u16le[] || material_head:u8)"

var k4KingBuckets = [64]int{
	1, 1, 0, 0, 0, 0, 1, 1,
	2, 2, 2, 2, 2, 2, 2, 2,
	3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3,
}

func k4Feature(color, pieceType, square, kingSquare, perspective int) uint16 {
	orientedSquare, orientedKing := square, kingSquare
	if perspective == 1 {
		orientedSquare ^= 56
		orientedKing ^= 56
	}
	if kingSquare%8 > 3 {
		orientedSquare ^= 7
	}
	return uint16(k4KingBuckets[orientedKing]*768 + (color^perspective)*384 + pieceType*64 + orientedSquare)
}

func k4InputComponents(position *engine.Position) ([2][]uint16, byte, error) {
	var result [2][]uint16
	if position == nil {
		return result, 0, fmt.Errorf("%w: nil K4 input position", ErrContract)
	}
	whiteKings := position.Board.GetBitboardOf(engine.WhiteKing)
	blackKings := position.Board.GetBitboardOf(engine.BlackKing)
	if bits.OnesCount64(whiteKings) != 1 || bits.OnesCount64(blackKings) != 1 {
		return result, 0, fmt.Errorf("%w: K4 input requires exactly one king per color", ErrContract)
	}
	kingSquares := [2]int{bits.TrailingZeros64(whiteKings), bits.TrailingZeros64(blackKings)}
	pieceCount := 0
	for piece := engine.WhitePawn; piece <= engine.BlackKing; piece++ {
		plane := int(piece - engine.WhitePawn)
		color, pieceType := plane/6, plane%6
		pieces := position.Board.GetBitboardOf(piece)
		pieceCount += bits.OnesCount64(pieces)
		for pieces != 0 {
			square := bits.TrailingZeros64(pieces)
			pieces &= pieces - 1
			result[0] = append(result[0], k4Feature(color, pieceType, square, kingSquares[0], 0))
			result[1] = append(result[1], k4Feature(color, pieceType, square, kingSquares[1], 1))
		}
	}
	if pieceCount > 32 {
		return result, 0, fmt.Errorf("%w: K4 input has %d pieces", ErrContract, pieceCount)
	}
	for perspective := range result {
		sort.Slice(result[perspective], func(i, j int) bool { return result[perspective][i] < result[perspective][j] })
	}
	head := (pieceCount - 2) / 4
	if head < 0 {
		head = 0
	}
	if head > 7 {
		head = 7
	}
	if position.Turn() == engine.Black {
		result[0], result[1] = result[1], result[0]
	}
	return result, byte(head), nil
}

// K4InputSHA256 returns the architecture-equivalent identity used for split
// isolation. It includes both ordered perspective inputs and the selected
// material output head, while intentionally ignoring state the evaluator does
// not consume (clocks, castling and en-passant rights).
func K4InputSHA256(position *engine.Position) (string, error) {
	features, head, err := k4InputComponents(position)
	if err != nil {
		return "", err
	}
	hasher := sha256.New()
	hasher.Write([]byte("ngn-k4-input-v1\x00"))
	var word [2]byte
	for perspective := 0; perspective < 2; perspective++ {
		binary.LittleEndian.PutUint16(word[:], uint16(len(features[perspective])))
		hasher.Write(word[:])
		for _, feature := range features[perspective] {
			binary.LittleEndian.PutUint16(word[:], feature)
			hasher.Write(word[:])
		}
	}
	hasher.Write([]byte{head})
	return hex.EncodeToString(hasher.Sum(nil)), nil
}
