package engine

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"sort"
	"testing"
)

func TestPrivatePrimitiveCompatibilityDump(t *testing.T) {
	path := os.Getenv("NGN_PRIVATE_PRIMITIVE_DUMP")
	if path == "" { t.Skip("private compatibility dump only") }
	var output bytes.Buffer
	put := func(value interface{}) { if err := binary.Write(&output, binary.LittleEndian, value); err != nil { t.Fatal(err) } }
	text := func(value string) { put(uint16(len(value))); output.WriteString(value) }
	for i := -128; i < 128; i++ {
		p, typ, color, square := Piece(i), PieceType(i), Color(i), Square(i)
		put(int8(p.Type())); put(int8(p.Color())); put(p.Weight()); put(p.seeWeight()); text(p.Name()); text(typ.Name()); put(int8(color.Other()))
		put(int8(square.File())); put(int8(square.Rank())); put(int8(square.GetColor())); text(square.Name()); text(square.String())
		text(File(i).Name()); put(int32(Rank(i).Name()))
		for j := -128; j < 128; j++ {
			put(int8(GetPiece(typ, Color(j))))
			put(int8(SquareOf(File(i), Rank(j))))
		}
	}
	for _, p := range Pieces { put(int8(p)) }
	for _, f := range Files { put(int8(f)) }
	for _, r := range Ranks { put(int8(r)) }
	keys := make([]string, 0, len(NameToSquareMap))
	for key := range NameToSquareMap { keys = append(keys, key) }
	sort.Strings(keys)
	for _, key := range keys { text(key); put(int8(NameToSquareMap[key])) }
	for r := rune(-16); r < 1024; r++ { put(int8(pieceFromName(r))) }
	put(int8(pieceFromName(0x10ffff)))
	seed := uint32(0x418392ab)
	for i := 0; i < 100000; i++ {
		seed ^= seed << 13; seed ^= seed >> 17; seed ^= seed << 5
		m := Move(seed)
		put(int8(m.Source())); put(int8(m.Destination())); put(int8(m.MovingPiece())); put(int8(m.CapturedPiece())); put(int8(m.PromoType())); put(uint8(m.Tag()))
		put(m.IsKingSideCastle()); put(m.IsQueenSideCastle()); put(m.IsCastle()); put(m.IsCapture()); put(m.IsEnPassant()); text(m.ToString())
		put(uint32(NewMove(m.Source(), m.Destination(), m.MovingPiece(), m.CapturedPiece(), m.PromoType(), m.Tag())))
	}
	if err := os.WriteFile(path, output.Bytes(), 0600); err != nil { t.Fatal(err) }
}

func TestPrivateHCENodeAndPerftDump(t *testing.T) {
	path := os.Getenv("NGN_PRIVATE_HCE_DUMP")
	if path == "" { t.Skip("private HCE proof only") }
	fens := []string{
		"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
		"r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1",
		"5k2/8/8/8/8/5q2/8/4KR2 w - - 0 1",
		"8/2p5/3p4/KP5r/1R3p1k/8/4P1P1/8 w - - 0 1",
		"4k3/P7/8/8/8/8/7p/4K3 w - - 0 1",
		"4k3/8/8/3pP3/8/8/8/4K3 w - d6 0 1",
	}
	rows := []map[string]interface{}{}
	for i, fen := range fens {
		position, err := ParseFEN(fen); if err != nil { t.Fatal(err) }
		hash := position.Hash()
		original := position.Copy()
		perft := RunPerftTest(position, 3)
		if i == 0 && perft.Nodes != 8902 { t.Fatalf("start perft=%d", perft.Nodes) }
		if i == 1 && perft.Nodes != 97862 { t.Fatalf("Kiwipete perft=%d", perft.Nodes) }
		searcher, err := NewSearchEngineWithHash(16); if err != nil { t.Fatal(err) }
		if err := searcher.SelectHCEEvaluator(); err != nil { t.Fatal(err) }
		score, backend, err := searcher.EvaluateSelected(position); if err != nil { t.Fatal(err) }
		info := searcher.SearchFixed(position, 4, nil)
		pv := make([]string,len(info.PV)); for i,move := range info.PV { pv[i]=move.ToString() }
		if GenerateFEN(position) != GenerateFEN(original) || position.Hash() != hash || !reflect.DeepEqual(position.Board, original.Board) {
			t.Fatalf("position %d was not restored",i)
		}
		rows = append(rows, map[string]interface{}{"fen":fen,"perft":perft,"static":score,"backend":backend,"score":info.BestScore,"best_move":info.BestMove.ToString(),"nodes":info.Nodes,"depth":info.Depth,"pv":pv,"hash":fmt.Sprintf("%016x",hash)})
	}
	data,err := json.MarshalIndent(rows,"","  "); if err != nil { t.Fatal(err) }
	if err := os.WriteFile(path,append(data,'\n'),0600);err != nil {t.Fatal(err)}
}
