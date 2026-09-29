package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ehrlich-b/ngn/engine"
)

// Release builds set these with -ldflags -X. The ordinary developer build
// retains its explicit research configuration.
var releaseProfile = "research"
var releaseVersion = "0.1.0"
var releaseNetwork = "ngn.nnue"

func releaseDefaults() (backend, model string, scale int, book bool, err error) {
	if releaseProfile == "research" {
		return engine.EvaluatorBackendHCEName, "", 100, true, nil
	}
	if releaseProfile != "owned" {
		return "", "", 0, false, fmt.Errorf("unknown build profile %q", releaseProfile)
	}
	if releaseNetwork == "" || releaseNetwork == "." || releaseNetwork == ".." || strings.ContainsAny(releaseNetwork, "/\\:") {
		return "", "", 0, false, fmt.Errorf("release network must be a filename")
	}
	executable, err := os.Executable()
	if err != nil {
		return "", "", 0, false, err
	}
	return engine.EvaluatorBackendNGNK4Name, filepath.Join(filepath.Dir(executable), releaseNetwork), 60, false, nil
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, input io.Reader, output, errorOutput io.Writer) int {
	defaultBackend, defaultModel, defaultScale, defaultBook, err := releaseDefaults()
	if err != nil {
		fmt.Fprintf(errorOutput, "ngn: release configuration failed: %v\n", err)
		return 2
	}
	flags := flag.NewFlagSet("ngn", flag.ContinueOnError)
	flags.SetOutput(errorOutput)
	showVersion := flags.Bool("version", false, "Print the engine version and exit")
	allowResignation := flags.Bool("allow-resignation", false, "Allow engine to resign in hopeless positions")
	evalBackend := flags.String("eval-backend", defaultBackend,
		"Evaluator backend: hce, ngn-v1, ngn-k4-768-v1, sf18-big, counter-5.5, rodent-v1.1-anand, or rodent-v1.2-default (non-HCE requires -eval-file)")
	evalFile := flags.String("eval-file", defaultModel, "Evaluator model path for a non-HCE backend")
	k4EvalScale := flags.Int("k4-eval-scale", defaultScale, "Owned K4 evaluation scale in percent (10 to 400)")
	ownBook := flags.Bool("own-book", defaultBook, "Use the engine's opening book")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(errorOutput, "ngn: unexpected positional arguments: %q\n", flags.Args())
		return 2
	}
	if *showVersion {
		fmt.Fprintln(output, releaseVersion)
		return 0
	}

	uciEngine := engine.NewUCIEngine()
	if err := uciEngine.ConfigureStartupVersion(releaseVersion); err != nil {
		fmt.Fprintf(errorOutput, "ngn: release identity failed: %v\n", err)
		return 2
	}
	if releaseProfile == "owned" && os.Getenv("GOMAXPROCS") == "" {
		uciEngine.ConfigureStartupThreadScheduler(func(count int) { runtime.GOMAXPROCS(count) })
	}
	if err := uciEngine.ConfigureStartupK4EvalScale(*k4EvalScale); err != nil {
		fmt.Fprintf(errorOutput, "ngn: K4 score scale configuration failed: %v\n", err)
		return 2
	}
	if err := uciEngine.ConfigureStartupEvaluator(*evalBackend, *evalFile); err != nil {
		fmt.Fprintf(errorOutput, "ngn: evaluator startup configuration failed: %v\n", err)
		return 2
	}
	uciEngine.SetResignationPolicy(*allowResignation)
	uciEngine.ConfigureStartupOwnBook(*ownBook)
	uciEngine.Run(input, output)
	return 0
}
