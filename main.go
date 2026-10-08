package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/ehrlich-b/ngn/engine"
)

// Release builds set releaseVersion with -ldflags -X. The default profile embeds
// the NGN network; -X main.releaseProfile=hce builds the locked-HCE engine.
var releaseProfile = "nnue"
var releaseVersion = "0.3.0-dev"

func releaseDefaults() (backend, model string, book bool, err error) {
	switch releaseProfile {
	case "nnue", "hce":
		return engine.EvaluatorBackendHCEName, "", false, nil
	default:
		return "", "", false, fmt.Errorf("unknown or retired build profile %q", releaseProfile)
	}
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, input io.Reader, output, errorOutput io.Writer) int {
	defaultBackend, defaultModel, defaultBook, err := releaseDefaults()
	if err != nil {
		fmt.Fprintf(errorOutput, "ngn: release configuration failed: %v\n", err)
		return 2
	}
	flags := flag.NewFlagSet("ngn", flag.ContinueOnError)
	flags.SetOutput(errorOutput)
	showVersion := flags.Bool("version", false, "Print the engine version and exit")
	allowResignation := flags.Bool("allow-resignation", false, "Allow engine to resign in hopeless positions")
	evalBackend := flags.String("eval-backend", defaultBackend, "Evaluator backend (hce in the default build)")
	evalFile := flags.String("eval-file", defaultModel, "External evaluator files are unsupported")
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
	if strings.ToLower(strings.TrimSpace(*evalBackend)) != engine.EvaluatorBackendHCEName ||
		(strings.TrimSpace(*evalFile) != "" && !strings.EqualFold(strings.TrimSpace(*evalFile), "<empty>")) {
		if releaseProfile == "hce" {
			fmt.Fprintln(errorOutput, "ngn: HCE build requires -eval-backend hce and no -eval-file")
		} else {
			fmt.Fprintln(errorOutput, "ngn: evaluator flags are unsupported; use the UCI EvalFile and UseNNUE options")
		}
		return 2
	}

	uciEngine := engine.NewUCIEngine()
	if err := uciEngine.ConfigureStartupVersion(releaseVersion); err != nil {
		fmt.Fprintf(errorOutput, "ngn: release identity failed: %v\n", err)
		return 2
	}
	if os.Getenv("GOMAXPROCS") == "" {
		uciEngine.ConfigureStartupThreadScheduler(func(count int) { runtime.GOMAXPROCS(count) })
	}
	err = uciEngine.ConfigureHCEStartup()
	if err == nil && releaseProfile == "nnue" {
		err = uciEngine.ConfigureEmbeddedNNUEStartup()
	}
	if err != nil {
		fmt.Fprintf(errorOutput, "ngn: evaluator startup configuration failed: %v\n", err)
		return 2
	}
	uciEngine.SetResignationPolicy(*allowResignation)
	uciEngine.ConfigureStartupOwnBook(*ownBook)
	uciEngine.Run(input, output)
	return 0
}
