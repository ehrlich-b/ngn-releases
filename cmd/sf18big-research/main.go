// Command sf18big-research is a dedicated, startup-locked UCI process for the
// bounded SF18 BIG strength screen. The normal process also exposes BIG as an
// opt-in backend, but this command locks it at startup for controlled matches.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ehrlich-b/ngn/engine"
)

func main() {
	modelPath := flag.String(
		"eval-file",
		os.Getenv("NGN_SF18_BIG_OFFICIAL_FILE"),
		"exact official SF18 BIG model (defaults to NGN_SF18_BIG_OFFICIAL_FILE)",
	)
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintf(os.Stderr, "sf18big-research: unexpected positional arguments: %q\n", flag.Args())
		os.Exit(2)
	}

	uci := engine.NewUCIEngine()
	if err := uci.ConfigureSF18BIGResearchStartup(*modelPath); err != nil {
		fmt.Fprintf(os.Stderr, "sf18big-research: startup failed: %v\n", err)
		os.Exit(2)
	}
	uci.Run(os.Stdin, os.Stdout)
}
