// source_mechanism_model.go is a dependency-free model copied from the pinned
// official-tag move encoding, renderer, TT packing, and pawn validation logic.
// It is source-derived evidence only; it is not the historical binary.
package main

import (
	"encoding/json"
	"fmt"
	"os"
)

const (
	normal = 0
	nProm  = 4
	bProm  = 5
	rProm  = 6
	qProm  = 7
	g1     = 6
	g2     = 14
)

func moveFrom(move int) int { return move & 63 }
func moveTo(move int) int   { return (move >> 6) & 63 }
func moveType(move int) int { return move >> 12 }
func isProm(move int) bool  { return move&0x4000 != 0 }
func rankOf(square int) int { return square >> 3 }
func fileOf(square int) int { return square & 7 }

func writeMove(move int) string {
	result := []byte{
		byte('a' + fileOf(moveFrom(move))),
		byte('1' + rankOf(moveFrom(move))),
		byte('a' + fileOf(moveTo(move))),
		byte('1' + rankOf(moveTo(move))),
	}
	if isProm(move) {
		const promotionCharacters = "nbrq"
		result = append(result, promotionCharacters[(move>>12)&3])
	}
	return string(result)
}

// pinnedBlackPawnValidation models the exact black NORMAL/promotion pawn
// branch in pinned legal.go. The source tests rankOf(from)==0, although a black
// pawn moves from rank index 1 to index 0 when it promotes.
func pinnedBlackPawnValidation(move int, targetEmpty bool) bool {
	from, to := moveFrom(move), moveTo(move)
	if rankOf(from) == 0 && !isProm(move) {
		return false
	}
	if from-to == 8 {
		return targetEmpty
	}
	return false
}

func packTTData(key uint64, move, score, depth, bound, date int) uint64 {
	return uint64(uint16(int16(move))) |
		uint64(uint16(int16(score)))<<16 |
		uint64(uint8(depth))<<32 |
		uint64(uint8(bound))<<40 |
		uint64(uint8(date&63))<<42 |
		(key & 0xFFFF000000000000)
}

func unpackTTMove(data uint64) int { return int(int16(data)) }

func require(condition bool, message string) {
	if !condition {
		panic(message)
	}
}

func main() {
	normalMove := (normal << 12) | (g1 << 6) | g2
	promotionTypes := []int{nProm, bProm, rProm, qProm}
	promotionStrings := make([]string, 0, len(promotionTypes))
	for _, kind := range promotionTypes {
		move := (kind << 12) | (g1 << 6) | g2
		promotionStrings = append(promotionStrings, writeMove(move))
		roundTrip := unpackTTMove(packTTData(0xA55A000000000000, move, 123, 9, 3, 7))
		require(roundTrip == move, "TT changed a promotion encoding")
		require(writeMove(roundTrip) == writeMove(move), "TT changed a promotion suffix")
	}
	require(writeMove(normalMove) == "g2g1", "NORMAL renderer result changed")
	require(pinnedBlackPawnValidation(normalMove, true), "pinned validator did not admit NORMAL g2g1")
	require(rankOf(g2) == 1, "g2 rank model is wrong")
	require(fmt.Sprint(promotionStrings) == "[g2g1n g2g1b g2g1r g2g1q]", "promotion renderer results changed")
	packed := packTTData(0xA55A000000000000, normalMove, 123, 9, 3, 7)
	require(unpackTTMove(packed) == normalMove, "TT did not preserve the complete 16-bit move")

	result := map[string]any{
		"classification":                       "source-derived model, not exact-binary proof",
		"normal_move_integer":                  normalMove,
		"normal_move_type":                     moveType(normalMove),
		"normal_is_promotion":                  isProm(normalMove),
		"normal_rendered":                      writeMove(normalMove),
		"g2_rank_index":                        rankOf(g2),
		"pinned_validator_accepts_normal_g2g1": pinnedBlackPawnValidation(normalMove, true),
		"killer_stage_would_accept_if_empty":   pinnedBlackPawnValidation(normalMove, true),
		"promotion_renderings":                 promotionStrings,
		"tt_round_trip_move_integer":           unpackTTMove(packed),
		"tt_preserves_all_move_bits":           unpackTTMove(packed) == normalMove,
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		panic(err)
	}
}
