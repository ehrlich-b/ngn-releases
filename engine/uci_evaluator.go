package engine

import (
	"fmt"
	"io"
	"strings"
)

const uciEvaluatorHCE = "hce"

type uciEvaluatorDefaults struct {
	backend, file string
	useNNUE       bool
}
type uciEvaluatorConfig struct {
	backend, file string
	useNNUE       bool
	network       *NGNN1Network
}

func newUCIEvaluatorConfig() uciEvaluatorConfig { return uciEvaluatorConfig{backend: uciEvaluatorHCE} }
func (c uciEvaluatorConfig) backendName() string {
	if c.useNNUE && c.network != nil {
		return c.network.backendName()
	}
	return uciEvaluatorHCE
}

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

// ConfigureEmbeddedNNUEStartup selects the embedded network before Run. An empty,
// <empty> or <embedded> EvalFile reselects it, so GUIs that echo option defaults
// keep the network; UseNNUE=false still selects the HCE.
func (uci *UCIEngine) ConfigureEmbeddedNNUEStartup() error {
	network, err := LoadDefaultNet()
	if err != nil {
		return err
	}
	if err := uci.searcher.SelectNGNN1Evaluator(network); err != nil {
		return err
	}
	uci.lifecycleMu.Lock()
	uci.evaluatorConfig = uciEvaluatorConfig{backend: uciEvaluatorHCE, file: DefaultNetName, useNNUE: true, network: network}
	uci.startupDefaults = uciEvaluatorDefaults{backend: uciEvaluatorHCE, file: DefaultNetName, useNNUE: true}
	uci.embeddedNet = network
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
	uci.joinSearch(true)
	path := strings.TrimSpace(value)
	if strings.EqualFold(path, "<empty>") {
		path = ""
	}
	uci.lifecycleMu.Lock()
	embedded := uci.embeddedNet
	uci.lifecycleMu.Unlock()
	var network *NGNN1Network
	var err error
	if embedded != nil && (path == "" || strings.EqualFold(path, DefaultNetName)) {
		network, path = embedded, DefaultNetName
	} else if path != "" {
		network, err = LoadNGNN1(path)
		if err != nil && embedded != nil {
			network, path = embedded, DefaultNetName
		}
	}
	uci.lifecycleMu.Lock()
	uci.evaluatorConfig.file = path
	uci.evaluatorConfig.network = network
	config := uci.evaluatorConfig
	uci.lifecycleMu.Unlock()
	uci.applyNNUEConfig(config, output)
	if err != nil && embedded != nil {
		uci.sendEvalOptionError(output, "%v; using embedded network", err)
	} else if err != nil {
		uci.sendEvalOptionError(output, "%v; using hce", err)
	}
}

func (uci *UCIEngine) handleUseNNUEOption(value string, output io.Writer) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value != "true" && value != "false" {
		uci.sendEvalOptionError(output, "UseNNUE expects true or false")
		return
	}
	uci.joinSearch(true)
	uci.lifecycleMu.Lock()
	uci.evaluatorConfig.useNNUE = value == "true"
	config := uci.evaluatorConfig
	uci.lifecycleMu.Unlock()
	uci.applyNNUEConfig(config, output)
	if config.useNNUE && config.network == nil {
		uci.sendEvalOptionError(output, "UseNNUE has no valid EvalFile; using hce")
	}
}

func (uci *UCIEngine) applyNNUEConfig(config uciEvaluatorConfig, output io.Writer) {
	var err error
	if config.useNNUE && config.network != nil {
		err = uci.searcher.SelectNGNN1Evaluator(config.network)
	} else {
		err = uci.searcher.SelectHCEEvaluator()
	}
	if err != nil {
		uci.sendEvalOptionError(output, "%v", err)
	}
}

func (uci *UCIEngine) handleEvalDiagnostic(output io.Writer) {
	uci.joinSearch(true)
	score, backend, err := uci.searcher.EvaluateSelected(uci.position)
	if err != nil {
		uci.sendUCIMessage(output, fmt.Sprintf("info string error eval: %v", err))
		return
	}
	uci.sendUCIMessage(output, fmt.Sprintf(
		"info string eval backend %s score_cp %d pov side-to-move policy base-SearchSTM correction-history excluded",
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
