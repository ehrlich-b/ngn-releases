package engine

import (
	"bytes"
	"encoding/binary"
	"os"
	"testing"
)

func TestPrivateSliderReplayDump(t *testing.T) {
	path := os.Getenv("NGN_PRIVATE_SLIDER_DUMP")
	if path == "" {
		t.Skip("independent slider proof only")
	}
	var out bytes.Buffer
	put := func(v uint64) {
		if err := binary.Write(&out, binary.LittleEndian, v); err != nil {
			t.Fatal(err)
		}
	}
	for sq := 0; sq < 64; sq++ {
		for blocker := 0; blocker < 64; blocker++ {
			occupied := uint64(1) << blocker
			put(GetRookAttacks(sq, occupied))
			put(GetBishopAttacks(sq, occupied))
			put(GetQueenAttacks(sq, occupied))
		}
		// Every subset of relevant interior ray blockers, independently enumerated.
		for _, diagonal := range []bool{false, true} {
			var mask uint64
			for other := 0; other < 64; other++ {
				file, rank := other%8, other/8
				df, dr := file-sq%8, rank-sq/8
				if df < 0 {
					df = -df
				}
				if dr < 0 {
					dr = -dr
				}
				aligned := (df == 0 || dr == 0)
				interior := (df == 0 && rank > 0 && rank < 7) || (dr == 0 && file > 0 && file < 7)
				if diagonal {
					aligned = df == dr
					interior = file > 0 && file < 7 && rank > 0 && rank < 7
				}
				if other != sq && aligned && interior {
					mask |= uint64(1) << other
				}
			}
			occupied := uint64(0)
			for {
				if diagonal {
					put(GetBishopAttacks(sq, occupied))
				} else {
					put(GetRookAttacks(sq, occupied))
				}
				occupied = (occupied - mask) & mask
				if occupied == 0 {
					break
				}
			}
		}
		random := uint64(0x452ad914b20e7a69) + uint64(sq)
		for i := 0; i < 2048; i++ {
			random ^= random << 13
			random ^= random >> 7
			random ^= random << 17
			put(GetRookAttacks(sq, random))
			put(GetBishopAttacks(sq, random))
			put(GetQueenAttacks(sq, random))
		}
	}
	if err := os.WriteFile(path, out.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
}
