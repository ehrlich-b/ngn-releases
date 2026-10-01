package engine

import (
	"fmt"
	"io"
	"strings"
)

const uciEvaluatorHCE = "hce"

type uciEvaluatorDefaults struct{ backend, file string }
type uciEvaluatorConfig struct{ backend, file string }

func newUCIEvaluatorConfig() uciEvaluatorConfig  { return uciEvaluatorConfig{backend: uciEvaluatorHCE} }
func (c uciEvaluatorConfig) backendName() string { return uciEvaluatorHCE }

func (uci *UCIEngine) ConfigureStartupEvaluator(backend, path string) error {
	backend = strings.ToLower(strings.TrimSpace(backend))
	path = strings.TrimSpace(path)
	if backend != "" && backend != uciEvaluatorHCE {
		return fmt.Errorf("only hce is supported")
	}
	if path != "" && !strings.EqualFold(path, "<empty>") {
		return fmt.Errorf("external evaluator files are unsupported")
	}
	if err := uci.searcher.SelectHCEEvaluator(); err != nil {
		return err
	}
	uci.lifecycleMu.Lock()
	uci.evaluatorConfig = newUCIEvaluatorConfig()
	uci.startupDefaults = uciEvaluatorDefaults{backend: uciEvaluatorHCE}
	uci.researchEvaluatorLocked = true
	uci.lifecycleMu.Unlock()
	return nil
}

func (uci *UCIEngine) evaluatorConfigSnapshot() uciEvaluatorConfig {
	uci.lifecycleMu.Lock()
	defer uci.lifecycleMu.Unlock()
	config := uci.evaluatorConfig
	if config.backend == "" {
		config.backend = uciEvaluatorHCE
	}
	return config
}

func (uci *UCIEngine) sendEvalOptionError(output io.Writer, format string, args ...interface{}) {
	uci.sendUCIMessage(output, fmt.Sprintf("info string error eval option: "+format, args...))
}

func (uci *UCIEngine) handleEvalBackendOption(value string, output io.Writer) {
	uci.sendEvalOptionError(output, "evaluator selection is locked to hce")
}
func (uci *UCIEngine) handleEvalFileOption(value string, output io.Writer) {
	uci.sendEvalOptionError(output, "external evaluator files are unsupported")
}

func (uci *UCIEngine) handleEvalDiagnostic(output io.Writer) {
	uci.joinSearch(true)
	score, backend, err := uci.searcher.EvaluateSelected(uci.position)
	if err != nil {
		uci.sendUCIMessage(output, fmt.Sprintf("info string error eval: %v", err))
		return
	}
	uci.sendUCIMessage(output, fmt.Sprintf(
		"info string eval backend %s score_cp %d pov side-to-move policy base-SearchSTM rule50-and-backend-adapter correction-history excluded",
		backend, score,
	))
}

func (uci *UCIEngine) ConfigureHCEStartup() error {
	if err := uci.ConfigureStartupEvaluator(uciEvaluatorHCE, ""); err != nil {
		return err
	}
	uci.lifecycleMu.Lock()
	uci.researchEvaluatorLocked = true
	uci.lifecycleMu.Unlock()
	return nil
}

func (uci *UCIEngine) researchEvaluatorIsLocked() bool {
	uci.lifecycleMu.Lock()
	defer uci.lifecycleMu.Unlock()
	return uci.researchEvaluatorLocked
}
