package claim

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/ehrlich-b/ngn/engine"
)

func LegalMove(p *engine.Position, token string) (engine.Move, error) {
	for _, m := range engine.GenerateLegalMoves(p) {
		if m.ToString() == token {
			return m, nil
		}
	}
	return 0, fmt.Errorf("illegal move %q in %s", token, engine.GenerateFEN(p))
}

// Repetition keys are exact board/STM/rights/legally capturable EP strings;
// audit never relies on the collector's repetition counters or hash keys.
func PositionKey(p *engine.Position) string {
	f := strings.Fields(engine.GenerateFEN(p))
	f = f[:4]
	f[3] = "-"
	for _, m := range engine.GenerateLegalMoves(p) {
		if m.IsEnPassant() {
			f[3] = m.Destination().String()
			break
		}
	}
	return strings.Join(f, " ")
}
func Insufficient(p *engine.Position) bool {
	if p.Board.Pawns() != 0 || p.Board.Rooks() != 0 || p.Board.Queens() != 0 {
		return false
	}
	n := engine.PopCount(p.Board.Knights())
	b := p.Board.GetBitboardOf(engine.WhiteBishop) | p.Board.GetBitboardOf(engine.BlackBishop)
	if n+engine.PopCount(b) <= 1 {
		return true
	}
	if n != 0 {
		return false
	}
	color := -1
	for s := 0; s < 64; s++ {
		if b&(uint64(1)<<s) != 0 {
			c := (s/8 + s%8) % 2
			if color < 0 {
				color = c
			} else if c != color {
				return false
			}
		}
	}
	return true // all bishops confined to one square color, including promotions
}

// Runner terminal policy. Mate/stalemate first, then rule draws, then total
// played+opening ply cap. Threefold/fifty are treated as immediate rule draws.
func RunnerTerminal(p *engine.Position, total, cap int) (string, string) {
	if len(engine.GenerateLegalMoves(p)) == 0 {
		if p.IsInCheck() {
			if p.Turn() == engine.White {
				return "0-1", "checkmate"
			}
			return "1-0", "checkmate"
		}
		return "1/2-1/2", "stalemate"
	}
	if p.Positions[p.Hash()] >= 3 {
		return "1/2-1/2", "repetition"
	}
	if p.HalfMoveClock >= 100 {
		return "1/2-1/2", "fifty-move"
	}
	if Insufficient(p) {
		return "1/2-1/2", "insufficient-material"
	}
	if total >= cap {
		return "1/2-1/2", "ply-cap"
	}
	return "", ""
}
func LoadOpenings(path string) ([]Opening, []byte, error) {
	f, e := os.Open(path)
	if e != nil {
		return nil, nil, e
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 65536), 4<<20)
	var ops []Opening
	var lines []string
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if len(ops) == 80 {
			break
		}
		o := Opening{Index: len(ops), FEN: StartFEN, Line: line, History: []string{}}
		if strings.Contains(line, "/") {
			return nil, nil, fmt.Errorf("opening %d: FEN-only line lacks prior history", o.Index)
		}
		o.History = strings.Fields(line)
		p, e := engine.ParseFEN(o.FEN)
		if e != nil {
			return nil, nil, e
		}
		for i, t := range o.History {
			if _, r := RunnerTerminal(p, i, 400); r != "" {
				return nil, nil, fmt.Errorf("opening %d continues terminal %s", o.Index, r)
			}
			m, e := LegalMove(p, t)
			if e != nil {
				return nil, nil, e
			}
			if _, _, _, ok := p.GameMakeMove(m); !ok {
				return nil, nil, fmt.Errorf("opening legal move refused %s", t)
			}
		}
		if _, r := RunnerTerminal(p, len(o.History), 400); r != "" {
			return nil, nil, fmt.Errorf("opening %d ends terminal %s", o.Index, r)
		}
		ops = append(ops, o)
		lines = append(lines, line)
	}
	if e = s.Err(); e != nil {
		return nil, nil, e
	}
	if len(ops) != 80 {
		return nil, nil, fmt.Errorf("need 80 usable openings, got %d", len(ops))
	}
	return ops, []byte(strings.Join(lines, "\n") + "\n"), nil
}
