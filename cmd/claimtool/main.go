// claimtool is NGN's fixed-sample runner, independent replay audit, identity
// receipt and predeclared analysis. Opponents are measurement binaries only.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/ehrlich-b/ngn/internal/claim"
)

const openingPath = "/home/ehrli/repos/ngn/output/corrected-pin-20260905/results-r0905matepin/inputs/sprt_openings.txt"
const lockPath = "/home/ehrli/ngn-data/claim-run-20261007/.execution-lock"

func lock() (*os.File, error) {
	if e := os.MkdirAll(filepath.Dir(lockPath), 0755); e != nil {
		return nil, e
	}
	f, e := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return nil, e
	}
	if e = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); e != nil {
		f.Close()
		return nil, fmt.Errorf("another claim preflight/match owns execution lock: %w", e)
	}
	return f, nil
}
func currentTool(r claim.Receipt) error {
	path, e := os.Executable()
	if e != nil {
		return e
	}
	a, e := claim.Bind(path)
	if e != nil {
		return e
	}
	if a != r.Plan.Runner {
		return fmt.Errorf("executing tool differs from frozen runner/audit/analysis")
	}
	return nil
}
func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: claimtool identity|prepare|run|audit|analyze|recheck [flags]")
	}
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	candidate := fs.String("candidate", "", "frozen Identity JSON (binary, net, hashes, source, build, provenance)")
	out := fs.String("out", "", "new output file or receipt directory")
	source := fs.String("source", "", "absolute clean source root")
	build := fs.String("build-argv", "", "JSON array with exact Go build argv")
	toolBuild := fs.String("tool-build-argv", "", "JSON array with exact claimtool build argv")
	engine := fs.String("engine", "", "absolute NGN binary")
	engineSHA := fs.String("engine-sha", "", "frozen binary SHA-256")
	net := fs.String("net", "", "absolute NGN net, or empty for explicit HCE")
	netSHA := fs.String("net-sha", "", "frozen net SHA-256")
	provenance := fs.String("provenance", "", "JSON map binding manifest/data/training/export/parity artifacts")
	mode := fs.String("mode", "", "screen, confirmation, tooling-check, zero-game identity-check, or anchor stability checks")
	bridge := fs.String("windows-bridge", "", "NGN-owned claimbridge.exe")
	openings := fs.String("openings", openingPath, "owned opening file; required full SHA is fixed")
	receipt := fs.String("receipt", "", "pre-game immutable receipt")
	games := fs.String("games", "", "fixed game directory")
	audit := fs.String("audit", "", "persisted admitted audit JSON")
	sr := fs.String("screen-receipt", "", "prior admitted screen receipt (confirmation only)")
	sa := fs.String("screen-analysis", "", "prior verified screen analysis (confirmation only)")
	dry := fs.Bool("dry-run", false, "check inputs and emit fixed plan; no engine launch, clock probes, or games")
	if e := fs.Parse(args[1:]); e != nil {
		return e
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments")
	}
	if _, e := claim.HostNow(); e != nil {
		return e
	}
	if *out == "" {
		return fmt.Errorf("--out is required except run")
	}
	switch args[0] {
	case "identity":
		id := claim.Identity{Engine: claim.Artifact{Path: *engine, SHA256: *engineSHA}, Net: claim.Artifact{Path: *net, SHA256: *netSHA}, SourceRoot: *source, GoVersion: "go1.25.5"}
		if !filepath.IsAbs(*source) {
			return fmt.Errorf("absolute --source required")
		}
		if e := claim.ReadJSON(*build, &id.BuildArgv); e != nil {
			return e
		}
		if e := claim.ReadJSON(*provenance, &id.Provenance); e != nil {
			return e
		}
		// Source commit is independently checked against binary build metadata by prepare.
		// git is invoked only by the receipt package through its source check.
		var e error
		id.SourceCommit, e = sourceCommit(*source)
		if e != nil {
			return e
		}
		if e = claim.Check(id.Engine); e != nil {
			return e
		}
		if *net != "" {
			if e = claim.Check(id.Net); e != nil {
				return e
			}
		} else if *netSHA != "" {
			return fmt.Errorf("HCE must have no net hash")
		}
		for _, key := range []string{"manifest", "data", "training", "export", "parity"} {
			a, ok := id.Provenance[key]
			if !ok {
				return fmt.Errorf("missing %s provenance", key)
			}
			if e = claim.Check(a); e != nil {
				return e
			}
		}
		return claim.WriteJSON(*out, id)
	case "prepare":
		var id claim.Identity
		var argv []string
		if e := claim.ReadJSON(*candidate, &id); e != nil {
			return e
		}
		if e := claim.ReadJSON(*toolBuild, &argv); e != nil {
			return e
		}
		self, e := os.Executable()
		if e != nil {
			return e
		}
		runner, e := claim.Bind(self)
		if e != nil {
			return e
		}
		p, e := claim.MakePlan(*mode, id, runner, *openings)
		if e != nil {
			return e
		}
		if *bridge != "" {
			p.WindowsBridge, e = claim.Bind(*bridge)
			if e != nil {
				return e
			}
		}
		if *dry {
			p, e = claim.PreflightPlan(p, *source, argv)
			if e != nil {
				return e
			}
			if e = os.Mkdir(*out, 0755); e != nil {
				return e
			}
			return claim.WriteJSON(filepath.Join(*out, "dry-run.json"), struct {
				Status   string
				Plan     claim.Plan
				Commands []string
			}{"inputs checked; no pre-run receipt or games; live clock/identity probes still required", p, []string{"prepare (without --dry-run): writes receipt and seal before any game", "run --receipt <out>/receipt.json --games <out>/games --out <out>/run-status.json", "audit --receipt <out>/receipt.json --games <out>/games --out <out>/audit.json", "analyze --receipt <out>/receipt.json --audit <out>/audit.json --games <out>/games --out <out>/analysis.json", "recheck --receipt <out>/receipt.json --out <out>/post-run-hashes.json"}})
		}
		l, e := lock()
		if e != nil {
			return e
		}
		defer l.Close()
		_, e = claim.CreateReceipt(p, *out, *source, argv, *sr, *sa)
		return e
	case "run", "audit", "analyze", "recheck":
		r, e := claim.ReadReceipt(*receipt)
		if e != nil {
			return e
		}
		if e = currentTool(r); e != nil {
			return e
		}
		switch args[0] {
		case "run":
			l, e := lock()
			if e != nil {
				return e
			}
			defer l.Close()
			if e = claim.Run(*receipt, *games); e != nil {
				return e
			}
			return claim.WriteJSON(*out, map[string]string{"status": "complete; independently audited; no score stopping or adjudication"})
		case "audit":
			a, _, e := claim.AuditDirectory(*receipt, *games)
			if e != nil {
				return e
			}
			return claim.WriteJSON(*out, a)
		case "analyze":
			a, e := claim.Analyze(*receipt, *audit, *games)
			if e != nil {
				return e
			}
			return claim.WriteJSON(*out, a)
		case "recheck":
			return claim.SaveRecheck(*receipt, *out)
		}
	}
	return fmt.Errorf("unknown command %q", args[0])
}
func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if s := os.Getenv("NGN_CLAIM_DEADLINE_UTC"); s != "" {
		deadline, e := time.Parse(time.RFC3339, s)
		if e != nil {
			fmt.Fprintln(os.Stderr, e)
			os.Exit(1)
		}
		var stop context.CancelFunc
		ctx, stop = context.WithDeadline(ctx, deadline)
		defer stop()
	}
	claim.ProcessContext = ctx
	if e := run(os.Args[1:]); e != nil {
		fmt.Fprintln(os.Stderr, "claimtool:", e)
		os.Exit(1)
	}
}
