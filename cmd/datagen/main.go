package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"syscall"

	"github.com/ehrlich-b/ngn/internal/datagen"
)

// Set by -ldflags '-X main.engineCommit=<git rev-parse HEAD>'; ordinary clean
// go builds can also use Go's embedded VCS revision.
var engineCommit string

func provenance() (datagen.Provenance, error) {
	commit := engineCommit
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" && commit == "" {
				commit = setting.Value
			}
			if setting.Key == "vcs.modified" && setting.Value == "true" {
				return datagen.Provenance{}, fmt.Errorf("build has uncommitted changes; build from a clean committed worktree")
			}
		}
	}
	if len(commit) != 40 {
		return datagen.Provenance{}, fmt.Errorf("missing engine commit; build with VCS metadata or -X main.engineCommit=<40-hex commit>")
	}
	if _, err := hex.DecodeString(commit); err != nil {
		return datagen.Provenance{}, fmt.Errorf("invalid engine commit: %w", err)
	}
	path, err := os.Executable()
	if err != nil {
		return datagen.Provenance{}, err
	}
	f, err := os.Open(path)
	if err != nil {
		return datagen.Provenance{}, err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return datagen.Provenance{}, err
	}
	return datagen.Provenance{EngineCommit: commit, BinarySHA256: hex.EncodeToString(h.Sum(nil))}, nil
}

func run() error {
	s := datagen.Defaults()
	var out string
	flag.StringVar(&out, "out", "", "new shard file (never overwritten)")
	flag.StringVar(&s.NetPath, "net", "", "NGNN1/2/3 network file; empty uses HCE")
	flag.Int64Var(&s.Seed, "seed", s.Seed, "opening RNG seed")
	flag.Uint64Var(&s.Games, "games", 0, "completed games; 0 runs until SIGTERM/SIGINT")
	flag.Uint64Var(&s.Nodes, "nodes", s.Nodes, "exact maximum search nodes per move")
	flag.IntVar(&s.RandomPlies, "random-plies", s.RandomPlies, "random legal opening plies; -1 chooses 8 or 9 per game")
	flag.IntVar(&s.MaxPly, "maxply", s.MaxPly, "maximum total game plies")
	flag.IntVar(&s.HashMB, "hash", s.HashMB, "transposition-table MB")
	flag.Parse()
	if flag.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments")
	}
	if err := s.Validate(); err != nil {
		return err
	}
	if out == "" {
		return fmt.Errorf("-out is required")
	}
	runtime.GOMAXPROCS(1)
	p, err := provenance()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	meta, err := datagen.Run(ctx, out, s, p)
	if meta.Format != "" {
		json.NewEncoder(os.Stdout).Encode(meta)
	}
	return err
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "datagen:", err)
		os.Exit(1)
	}
}
