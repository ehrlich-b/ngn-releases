// Package ngnp reads and writes NGN-owned NGNP1 training records.
package ngnp

import (
	"encoding/binary"
	"fmt"
	"io"
	"math/bits"

	"github.com/ehrlich-b/ngn/engine"
)

const (
	Format = "NGNP1"
	Size   = 32
)

const (
	FlagInCheck uint8 = 1 << iota
	FlagCapture
	FlagPromotion
	FlagClampedOrMate
)

type Result uint8

const (
	Loss Result = iota
	Draw
	Win
)

// Record stores board pieces and side to move, without castling, en passant,
// clocks, or repetition history. Score and Result always use White's view.
type Record struct {
	Occupancy uint64
	Pieces    [16]byte
	Score     int16
	Result    Result
	STM       uint8 // 0 White, 1 Black; engine.Color uses a different convention.
	Ply       uint16
	Flags     uint8
}

func FromPosition(pos *engine.Position, score int, result Result, ply uint16, flags uint8) (Record, error) {
	r := Record{Result: result, Ply: ply, Flags: flags}
	if pos.Turn() == engine.Black {
		r.STM = 1
	}
	if engine.IsInCheckDirect(pos, pos.Turn()) {
		r.Flags |= FlagInCheck
	}
	if score >= engine.MATE_IN_MAX || score <= -engine.MATE_IN_MAX {
		r.Flags |= FlagClampedOrMate
	}
	if score > 32767 {
		score = 32767
		r.Flags |= FlagClampedOrMate
	} else if score < -32768 {
		score = -32768
		r.Flags |= FlagClampedOrMate
	}
	r.Score = int16(score)
	i := 0
	for sq := engine.A1; sq <= engine.H8; sq++ {
		piece := pos.Board.PieceAt(sq)
		if piece == engine.NoPiece {
			continue
		}
		if i == 32 || piece < engine.WhitePawn || piece > engine.BlackKing {
			return Record{}, fmt.Errorf("invalid piece count or piece at square %d", sq)
		}
		r.Occupancy |= uint64(1) << uint(sq)
		r.Pieces[i/2] |= byte(piece-1) << uint(4*(i%2))
		i++
	}
	return r, r.Validate()
}

func (r Record) PieceCount() int { return bits.OnesCount64(r.Occupancy) }

func (r Record) validateShape() error {
	if r.PieceCount() > 32 {
		return fmt.Errorf("more than 32 pieces")
	}
	if r.Result > Win || r.STM > 1 || r.Flags&0xf0 != 0 {
		return fmt.Errorf("invalid result, side to move, or reserved flags")
	}
	if (r.Score >= engine.MATE_IN_MAX || r.Score <= -engine.MATE_IN_MAX) && r.Flags&FlagClampedOrMate == 0 {
		return fmt.Errorf("mate/clamped score without flag")
	}
	var count [2][6]int
	var bishops [2][2]int
	occupied := r.Occupancy
	for i := 0; i < 32; i++ {
		code := (r.Pieces[i/2] >> uint(4*(i%2))) & 15
		if occupied == 0 {
			if code != 0 {
				return fmt.Errorf("nonzero unused piece nibble %d", i)
			}
			continue
		}
		if code > 11 {
			return fmt.Errorf("invalid piece code %d", code)
		}
		sq := bits.TrailingZeros64(occupied)
		occupied &= occupied - 1
		color, kind := int(code/6), int(code%6)
		count[color][kind]++
		if kind == 0 && (sq/8 == 0 || sq/8 == 7) {
			return fmt.Errorf("pawn on rank 1 or 8")
		}
		if kind == 2 {
			bishops[color][(sq/8+sq%8)%2]++
		}
	}
	for c := range count {
		n := count[c]
		if n[5] != 1 || n[0] > 8 {
			return fmt.Errorf("side %d has invalid kings or pawns", c)
		}
		total := 0
		for _, v := range n {
			total += v
		}
		promotions := excess(n[1], 2) + excess(n[3], 2) + excess(n[4], 1) +
			excess(bishops[c][0], 1) + excess(bishops[c][1], 1)
		if total > 16 || promotions > 8-n[0] {
			return fmt.Errorf("side %d has impossible material counts", c)
		}
	}
	return nil
}

func excess(count, initial int) int {
	if count > initial {
		return count - initial
	}
	return 0
}

func (r Record) position() *engine.Position {
	pos := &engine.Position{EnPassant: engine.NoSquare, Positions: make(map[uint64]int)}
	pos.SetTag(engine.WhiteToMove)
	if r.STM == 1 {
		pos.ClearTag(engine.WhiteToMove)
		pos.SetTag(engine.BlackToMove)
	}
	occupied := r.Occupancy
	for i := 0; occupied != 0; i++ {
		sq := engine.Square(bits.TrailingZeros64(occupied))
		occupied &= occupied - 1
		code := (r.Pieces[i/2] >> uint(4*(i%2))) & 15
		pos.Board.UpdateSquare(sq, engine.Piece(code+1), engine.NoPiece)
	}
	if r.Flags&FlagInCheck != 0 {
		pos.SetTag(engine.InCheck)
	}
	return pos
}

// Validate checks every encoded field and necessary chess legality conditions.
// It does not claim to prove reachability from the initial position.
func (r Record) Validate() error {
	if err := r.validateShape(); err != nil {
		return err
	}
	pos := r.position()
	if engine.IsInCheckDirect(pos, pos.Turn().Other()) {
		return fmt.Errorf("side not to move is in check")
	}
	if engine.IsInCheckDirect(pos, pos.Turn()) != (r.Flags&FlagInCheck != 0) {
		return fmt.Errorf("in-check flag disagrees with board")
	}
	return nil
}

// Position reconstructs the stored board/stm. Unstored FEN fields are neutral.
func (r Record) Position() (*engine.Position, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}
	pos := r.position()
	pos.Positions[pos.Hash()] = 1
	return pos, nil
}

func Encode(r Record) ([Size]byte, error) {
	var raw [Size]byte
	if err := r.Validate(); err != nil {
		return raw, err
	}
	binary.LittleEndian.PutUint64(raw[0:8], r.Occupancy)
	copy(raw[8:24], r.Pieces[:])
	binary.LittleEndian.PutUint16(raw[24:26], uint16(r.Score))
	raw[26], raw[27] = byte(r.Result), r.STM
	binary.LittleEndian.PutUint16(raw[28:30], r.Ply)
	raw[30] = r.Flags // raw[31] is reserved and zero.
	return raw, nil
}

func Decode(raw []byte) (Record, error) {
	if len(raw) != Size || raw[31] != 0 {
		return Record{}, fmt.Errorf("expected 32 bytes with zero reserved byte")
	}
	r := Record{Occupancy: binary.LittleEndian.Uint64(raw[0:8]),
		Score: int16(binary.LittleEndian.Uint16(raw[24:26])), Result: Result(raw[26]),
		STM: raw[27], Ply: binary.LittleEndian.Uint16(raw[28:30]), Flags: raw[30]}
	copy(r.Pieces[:], raw[8:24])
	return r, r.Validate()
}

// Iterate validates all records, rejecting truncated tails with a byte offset.
func Iterate(reader io.Reader, visit func(Record) error) error {
	var raw [Size]byte
	for index := uint64(0); ; index++ {
		n, err := io.ReadFull(reader, raw[:])
		if err == io.EOF && n == 0 {
			return nil
		}
		if err != nil {
			return fmt.Errorf("record %d at byte %d: %w", index, index*Size, err)
		}
		r, err := Decode(raw[:])
		if err != nil {
			return fmt.Errorf("record %d at byte %d: %w", index, index*Size, err)
		}
		if err := visit(r); err != nil {
			return fmt.Errorf("record %d: %w", index, err)
		}
	}
}
