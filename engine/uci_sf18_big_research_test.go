package engine

import (
	"os"
	"strings"
	"testing"
)

func TestSF18BIGResearchUCILockAndNewGamePersistence(t *testing.T) {
	path := os.Getenv(sf18BigResearchModelEnvironment)
	if path == "" {
		t.Skip("official SF18 BIG model is required for the research UCI gate")
	}
	uci := NewUCIEngine()
	if err := uci.ConfigureSF18BIGResearchStartup(path); err != nil {
		t.Fatal(err)
	}
	if got := uci.searcher.SelectedEvaluatorBackend(); got != EvaluatorBackendSF18BIGResearchName {
		t.Fatalf("selected backend = %q, want %q", got, EvaluatorBackendSF18BIGResearchName)
	}

	var handshake strings.Builder
	uci.handleCommand("uci", &handshake)
	if strings.Contains(handshake.String(), "option name EvalBackend") ||
		strings.Contains(handshake.String(), "option name EvalFile") {
		t.Fatalf("research UCI advertised evaluator mutation options:\n%s", handshake.String())
	}
	if !strings.Contains(handshake.String(), "id name ngn-sf18-big-research") {
		t.Fatalf("research UCI identity missing:\n%s", handshake.String())
	}

	uci.handleCommand("ucinewgame", &handshake)
	if got := uci.searcher.SelectedEvaluatorBackend(); got != EvaluatorBackendSF18BIGResearchName {
		t.Fatalf("new game changed selected backend to %q", got)
	}

	var optionOutput strings.Builder
	uci.handleCommand("setoption name EvalBackend value hce", &optionOutput)
	if !strings.Contains(optionOutput.String(), "research startup evaluator is locked") {
		t.Fatalf("locked backend mutation was not rejected: %s", optionOutput.String())
	}
	if got := uci.searcher.SelectedEvaluatorBackend(); got != EvaluatorBackendSF18BIGResearchName {
		t.Fatalf("rejected mutation changed selected backend to %q", got)
	}
}
