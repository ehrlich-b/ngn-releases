package engine

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"testing"
)

// TestZZEvalBiasExtract (throwaway): walk loss-game move lists from a lossxray-style
// PGN ("GAME anchor result color reason | m1 m2 ...") and emit, for many real
// positions, NGN's STATIC white-POV eval (evaluateUnsafe — no search) alongside the
// FEN and phase. A separate step feeds these FENs to Stockfish; joining the two
// measures the SIGNED static-eval bias (NGN - SF, white-POV) across real positions,
// which is the direct test of "NGN static eval systematically over-favors a side".
// Env: BIAS_PGN (input pgn), BIAS_OUT (tsv out), BIAS_EVERY (sample every Nth ply,
// default 4), BIAS_SKIP (skip first N plies, default 12).
func TestZZEvalBiasExtract(t *testing.T) {
	in := os.Getenv("BIAS_PGN")
	out := os.Getenv("BIAS_OUT")
	if in == "" || out == "" {
		t.Skip("set BIAS_PGN and BIAS_OUT")
	}
	every := 4
	if v := os.Getenv("BIAS_EVERY"); v != "" {
		fmt.Sscanf(v, "%d", &every)
	}
	skip := 12
	if v := os.Getenv("BIAS_SKIP"); v != "" {
		fmt.Sscanf(v, "%d", &skip)
	}

	f, err := os.Open(in)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	w, err := os.Create(out)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	bw := bufio.NewWriter(w)
	defer bw.Flush()
	fmt.Fprintln(bw, "lossidx\tply\tngncolor\tphase\tngnStaticWPOV\tfen")

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<22)
	lossN := 0
	for sc.Scan() {
		ln := strings.TrimSpace(sc.Text())
		if !strings.HasPrefix(ln, "GAME ") {
			continue
		}
		i := strings.Index(ln, " | ")
		if i < 0 {
			continue
		}
		head := strings.Fields(ln[:i])
		if len(head) < 5 || head[2] != "L" || strings.Contains(ln, "time-forfeit") {
			continue
		}
		ngnColor := head[3] // "w" or "b"
		moves := strings.Fields(ln[i+3:])
		lossN++
		pos := &Position{
			Board: StartingBoard(),
			Tag: WhiteCanCastleKingSide | WhiteCanCastleQueenSide |
				BlackCanCastleKingSide | BlackCanCastleQueenSide | WhiteToMove,
			EnPassant: NoSquare,
		}
		for ply, mv := range moves {
			pm, err := ParseUCIMove(pos, mv)
			if err != nil {
				break
			}
			pos.MakeMove(pm)
			if ply < skip || ply%every != 0 {
				continue
			}
			_, phase := evalCoreWhite(&pos.Board)
			ev := evaluateUnsafe(&pos.Board) // white-POV static
			fen := GenerateFEN(pos)
			// skip positions with a mate-ish material imbalance already (eval huge)
			if ev > 1500 || ev < -1500 {
				continue
			}
			fmt.Fprintf(bw, "%d\t%d\t%s\t%d\t%d\t%s\n", lossN, ply+1, ngnColor, phase, ev, fen)
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
}
