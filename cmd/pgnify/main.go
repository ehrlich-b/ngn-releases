// Command pgnify converts a gauntlet -pgn capture (custom one-line
// "GAME <anchor> <W|D|L> <w|b> <reason> | <uci moves>" records) into standard PGN
// with proper headers + SAN movetext, importable into the Lichess analyzer.
//
//	pgnify -in output/capture.txt -out output/games.pgn -oppelo 2532 -opp "Blunder 7.4.0"
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ehrlich-b/ngn/engine"
)

const startFEN = "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"

func main() {
	in := flag.String("in", "", "gauntlet capture file (GAME ... lines)")
	out := flag.String("out", "", "output PGN file (default: stdout)")
	ngnName := flag.String("ngn", "NGN", "NGN player name")
	ngnElo := flag.Int("ngnelo", 2447, "NGN rating (gauntlet-pooled CCRL)")
	oppName := flag.String("opp", "Blunder", "opponent player name")
	oppElo := flag.Int("oppelo", 2532, "opponent rating")
	event := flag.String("event", "NGN vs Blunder", "PGN Event tag")
	tc := flag.String("tc", "", "TimeControl tag (e.g. 'movetime 400ms')")
	flag.Parse()

	if *in == "" {
		fmt.Fprintln(os.Stderr, "usage: pgnify -in <capture> [-out <pgn>] [-opp NAME -oppelo M -ngnelo N]")
		os.Exit(2)
	}
	data, err := os.ReadFile(*in)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read %s: %v\n", *in, err)
		os.Exit(1)
	}

	w := bufio.NewWriter(os.Stdout)
	if *out != "" {
		f, err := os.Create(*out)
		if err != nil {
			fmt.Fprintf(os.Stderr, "create %s: %v\n", *out, err)
			os.Exit(1)
		}
		defer f.Close()
		w = bufio.NewWriter(f)
	}
	defer w.Flush()

	date := time.Now().Format("2006.01.02")
	round, written := 0, 0
	for _, ln := range strings.Split(string(data), "\n") {
		ln = strings.TrimSpace(ln)
		if !strings.HasPrefix(ln, "GAME ") {
			continue
		}
		parts := strings.SplitN(ln, " | ", 2)
		if len(parts) != 2 {
			continue
		}
		f := strings.Fields(parts[0]) // GAME anchor result color reason...
		if len(f) < 5 {
			continue
		}
		anchor, ngnRes, ngnColor := f[1], f[2], f[3]
		reason := strings.Join(f[4:], " ")
		round++

		san, ok := replaySAN(strings.Fields(parts[1]))
		if !ok {
			fmt.Fprintf(os.Stderr, "WARNING: skipping game %d (replay failed)\n", round)
			continue
		}

		white, black := *ngnName, *oppName+" "+anchor
		welo, belo := *ngnElo, *oppElo
		if ngnColor == "b" {
			white, black = *oppName+" "+anchor, *ngnName
			welo, belo = *oppElo, *ngnElo
		}
		result := "1/2-1/2"
		if ngnRes == "W" {
			result = pickResult(ngnColor == "w")
		} else if ngnRes == "L" {
			result = pickResult(ngnColor != "w")
		}

		fmt.Fprintf(w, "[Event %q]\n", *event)
		fmt.Fprintf(w, "[Site %q]\n", "local")
		fmt.Fprintf(w, "[Date %q]\n", date)
		fmt.Fprintf(w, "[Round %q]\n", fmt.Sprint(round))
		fmt.Fprintf(w, "[White %q]\n", white)
		fmt.Fprintf(w, "[Black %q]\n", black)
		fmt.Fprintf(w, "[Result %q]\n", result)
		fmt.Fprintf(w, "[WhiteElo %q]\n", fmt.Sprint(welo))
		fmt.Fprintf(w, "[BlackElo %q]\n", fmt.Sprint(belo))
		if *tc != "" {
			fmt.Fprintf(w, "[TimeControl %q]\n", *tc)
		}
		fmt.Fprintf(w, "[Termination %q]\n\n", reason)
		fmt.Fprintln(w, movetext(san, result))
		fmt.Fprintln(w)
		written++
	}
	fmt.Fprintf(os.Stderr, "wrote %d games to ", written)
	if *out != "" {
		fmt.Fprintln(os.Stderr, *out)
	} else {
		fmt.Fprintln(os.Stderr, "stdout")
	}
}

// replaySAN replays UCI moves from the standard start, returning SAN strings.
func replaySAN(uci []string) ([]string, bool) {
	pos, err := engine.ParseFEN(startFEN)
	if err != nil {
		return nil, false
	}
	san := make([]string, 0, len(uci))
	for _, u := range uci {
		m, err := engine.ParseUCIMove(pos, u)
		if err != nil {
			return nil, false
		}
		san = append(san, engine.MoveToSAN(m, pos))
		if _, _, _, ok := pos.GameMakeMove(m); !ok {
			return nil, false
		}
	}
	return san, true
}

func pickResult(whiteWon bool) string {
	if whiteWon {
		return "1-0"
	}
	return "0-1"
}

func movetext(san []string, result string) string {
	var sb strings.Builder
	for i, m := range san {
		if i%2 == 0 {
			if i > 0 {
				sb.WriteByte(' ')
			}
			fmt.Fprintf(&sb, "%d. %s", i/2+1, m)
		} else {
			sb.WriteByte(' ')
			sb.WriteString(m)
		}
	}
	if len(san) > 0 {
		sb.WriteByte(' ')
	}
	sb.WriteString(result)
	return sb.String()
}
