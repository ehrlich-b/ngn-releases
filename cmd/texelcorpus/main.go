// texelcorpus creates an auditable, game-grouped JSONL corpus for classical
// evaluation tuning. It intentionally has no tune mode; loading/splitting is in
// internal/texeldata so the optimizer cannot accidentally consume flat data.
package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	"github.com/ehrlich-b/ngn/engine"
	"github.com/ehrlich-b/ngn/internal/texeldata"
)

func main() {
	openingsPath := flag.String("openings", "", "required UCI opening lines, one startpos sequence per line")
	start := flag.Int("opening-start", 0, "first opening-line index")
	games := flag.Int("games", 0, "number of distinct opening lines to play")
	nodes := flag.Int("nodes", 8000, "fixed nodes per searched move")
	maxPlies := flag.Int("maxplies", 240, "maximum total plies, opening included")
	seed := flag.Int64("seed", 1, "deterministic sample-selection seed")
	out := flag.String("out", "", "required JSONL destination; written atomically")
	source := flag.String("source", buildSource(), "source identity recorded in every row")
	flag.Parse()
	if *openingsPath == "" || *out == "" || *games <= 0 {
		fmt.Fprintln(os.Stderr, "-openings, -games, and -out are required")
		os.Exit(2)
	}
	openings, err := loadOpenings(*openingsPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "openings:", err)
		os.Exit(1)
	}
	exeHash, err := executableHash()
	if err != nil {
		fmt.Fprintln(os.Stderr, "executable:", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(filepath.Dir(*out), 0755); err != nil {
		fmt.Fprintln(os.Stderr, "output directory:", err)
		os.Exit(1)
	}
	tmp, err := os.CreateTemp(filepath.Dir(*out), ".texelcorpus-*.jsonl")
	if err != nil {
		fmt.Fprintln(os.Stderr, "output:", err)
		os.Exit(1)
	}
	tmpName := tmp.Name()
	bw := bufio.NewWriter(tmp)
	summary, genErr := texeldata.Generate(bw, texeldata.GenerateOptions{Openings: openings, Start: *start, Games: *games, Nodes: *nodes, MaxPlies: *maxPlies, Seed: *seed, Generator: texeldata.GeneratorIdentity{ExecutableSHA256: exeHash, SourceIdentity: *source}})
	if genErr == nil {
		genErr = bw.Flush()
	}
	if genErr == nil {
		genErr = tmp.Sync()
	}
	// A temporary sibling is published only after all writes and close succeed.
	if closeErr := tmp.Close(); genErr == nil && closeErr != nil {
		genErr = closeErr
	}
	if genErr != nil {
		_ = os.Remove(tmpName)
		fmt.Fprintln(os.Stderr, "generate:", genErr)
		os.Exit(1)
	}
	if err := os.Rename(tmpName, *out); err != nil {
		_ = os.Remove(tmpName)
		fmt.Fprintln(os.Stderr, "publish:", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "attempted=%d completed=%d unresolved=%d rows=%d output=%s\n", summary.Attempted, summary.Completed, summary.Unresolved, summary.Rows, *out)
}

func loadOpenings(path string) ([][]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 4096), 1024*1024)
	var out [][]string
	for line := 1; s.Scan(); line++ {
		text := strings.TrimSpace(s.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		moves := strings.Fields(text)
		pos, err := engine.ParseFEN(texeldata.StartFEN)
		if err != nil {
			return nil, err
		}
		for _, move := range moves {
			m, err := engine.ParseUCIMove(pos, move)
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", line, err)
			}
			if _, _, _, ok := pos.GameMakeMove(m); !ok {
				return nil, fmt.Errorf("line %d: rejected %q", line, move)
			}
		}
		if len(moves) == 0 {
			return nil, fmt.Errorf("line %d: empty opening", line)
		}
		out = append(out, moves)
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no opening lines")
	}
	return out, nil
}

func executableHash() (string, error) {
	p, err := os.Executable()
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}
func buildSource() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		rev := "unknown"
		for _, s := range info.Settings {
			if s.Key == "vcs.revision" {
				rev = s.Value
			}
		}
		return info.Main.Path + "@" + rev
	}
	return "github.com/ehrlich-b/ngn@unknown"
}
