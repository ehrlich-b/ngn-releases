package engine

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ehrlich-b/ngn/countereval"
	"github.com/ehrlich-b/ngn/nnue"
	"github.com/ehrlich-b/ngn/nnue/ngnk4"
	"github.com/ehrlich-b/ngn/nnue/sf18big"
	"github.com/ehrlich-b/ngn/rodenteval"
	"github.com/ehrlich-b/ngn/rodentv12eval"
)

const (
	uciEvaluatorHCE              = "hce"
	uciEvaluatorNGNV1            = "ngn-v1"
	uciEvaluatorSF18BIG          = "sf18-big"
	uciEvaluatorCounter55        = "counter-5.5"
	uciEvaluatorRodentV11Anand   = "rodent-v1.1-anand"
	uciEvaluatorRodentV12Default = "rodent-v1.2-default"
	uciEvaluatorNGNK4            = "ngn-k4-768-v1"
)

type stagedEvaluatorKind uint8

const (
	stagedEvaluatorNone stagedEvaluatorKind = iota
	stagedEvaluatorNGNV1
	stagedEvaluatorSF18BIG
	stagedEvaluatorCounter55
	stagedEvaluatorRodentV11Anand
	stagedEvaluatorRodentV12Default
	stagedEvaluatorNGNK4
)

type stagedEvaluatorModel struct {
	kind             stagedEvaluatorKind
	ngnV1            *nnue.Model
	sf18Big          *sf18big.Model
	counter55        *countereval.Model
	rodentV11Anand   *rodenteval.Model
	rodentV12Default *rodentv12eval.Model
	ngnK4            *ngnk4.Model
}

func (model stagedEvaluatorModel) backendName() string {
	switch model.kind {
	case stagedEvaluatorNGNV1:
		return uciEvaluatorNGNV1
	case stagedEvaluatorSF18BIG:
		return uciEvaluatorSF18BIG
	case stagedEvaluatorCounter55:
		return uciEvaluatorCounter55
	case stagedEvaluatorRodentV11Anand:
		return uciEvaluatorRodentV11Anand
	case stagedEvaluatorRodentV12Default:
		return uciEvaluatorRodentV12Default
	case stagedEvaluatorNGNK4:
		return uciEvaluatorNGNK4
	default:
		return ""
	}
}

type uciEvaluatorDefaults struct {
	backend string
	file    string
}

type uciEvaluatorConfig struct {
	backend string
	file    string
	staged  stagedEvaluatorModel
}

func newUCIEvaluatorConfig() uciEvaluatorConfig {
	return uciEvaluatorConfig{backend: uciEvaluatorHCE}
}

func (c uciEvaluatorConfig) backendName() string {
	if c.backend == "" {
		return uciEvaluatorHCE
	}
	return c.backend
}

func loadUCIEvaluatorModel(path string) (stagedEvaluatorModel, error) {
	file, err := os.Open(path)
	if err != nil {
		return stagedEvaluatorModel{}, fmt.Errorf("open %q: %w", path, err)
	}
	defer file.Close()

	var prefix [countereval.LegacyHeaderSize]byte
	count, readErr := io.ReadFull(file, prefix[:])
	if readErr != nil && readErr != io.ErrUnexpectedEOF {
		return stagedEvaluatorModel{}, fmt.Errorf("read evaluator header %q: %w", path, readErr)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return stagedEvaluatorModel{}, fmt.Errorf("rewind evaluator %q: %w", path, err)
	}
	switch {
	case nnue.RecognizesHeader(prefix[:count]):
		model, err := nnue.Load(file)
		if err != nil {
			return stagedEvaluatorModel{}, fmt.Errorf("load exact ngn-v1 %q: %w", path, err)
		}
		return stagedEvaluatorModel{kind: stagedEvaluatorNGNV1, ngnV1: model}, nil
	case ngnk4.RecognizesHeader(prefix[:count]):
		model, err := ngnk4.Load(file)
		if err != nil {
			return stagedEvaluatorModel{}, fmt.Errorf("load exact ngn-k4-768-v1 %q: %w", path, err)
		}
		return stagedEvaluatorModel{kind: stagedEvaluatorNGNK4, ngnK4: model}, nil
	case sf18big.RecognizesHeader(prefix[:count]):
		model, err := sf18big.Load(file)
		if err != nil {
			return stagedEvaluatorModel{}, fmt.Errorf("load exact sf18-big %q: %w", path, err)
		}
		return stagedEvaluatorModel{kind: stagedEvaluatorSF18BIG, sf18Big: model}, nil
	case countereval.RecognizesLegacyHeader(prefix[:count]):
		model, _, err := countereval.LoadCounter55Legacy(file)
		if err != nil {
			return stagedEvaluatorModel{}, fmt.Errorf("load exact counter-5.5 %q: %w", path, err)
		}
		return stagedEvaluatorModel{kind: stagedEvaluatorCounter55, counter55: model}, nil
	default:
		model, v11Err := rodenteval.LoadV11Anand(path)
		if v11Err == nil {
			return stagedEvaluatorModel{kind: stagedEvaluatorRodentV11Anand, rodentV11Anand: model}, nil
		}
		v12Model, v12Err := rodentv12eval.LoadV12Default(path)
		if v12Err == nil {
			return stagedEvaluatorModel{kind: stagedEvaluatorRodentV12Default, rodentV12Default: v12Model}, nil
		}
		return stagedEvaluatorModel{}, fmt.Errorf("unsupported evaluator format %q: V1.1: %v; V1.2: %v", path, v11Err, v12Err)
	}
}

// ConfigureStartupEvaluator selects the evaluator configuration exposed as the
// UCI defaults for this process. It must be called before Run. Unlike the UCI
// staging interface, a startup model path must name the model that is activated:
// explicit startup configuration never silently leaves HCE selected.
func (uci *UCIEngine) ConfigureStartupEvaluator(backend, path string) error {
	backend = strings.ToLower(strings.TrimSpace(backend))
	path = strings.TrimSpace(path)
	if backend == "" {
		backend = uciEvaluatorHCE
	}
	if strings.ContainsAny(path, "\r\n") {
		return fmt.Errorf("EvalFile contains a line break")
	}

	switch backend {
	case uciEvaluatorHCE:
		if path != "" && !strings.EqualFold(path, "<empty>") {
			return fmt.Errorf("EvalFile requires a non-HCE EvalBackend")
		}
		if err := uci.searcher.SelectHCEEvaluator(); err != nil {
			return fmt.Errorf("select hce: %w", err)
		}
		uci.lifecycleMu.Lock()
		uci.evaluatorConfig = newUCIEvaluatorConfig()
		uci.startupDefaults = uciEvaluatorDefaults{backend: uciEvaluatorHCE}
		uci.lifecycleMu.Unlock()
		return nil
	case uciEvaluatorNGNV1, uciEvaluatorSF18BIG, uciEvaluatorCounter55, uciEvaluatorRodentV11Anand, uciEvaluatorRodentV12Default, uciEvaluatorNGNK4:
		if path == "" || strings.EqualFold(path, "<empty>") {
			return fmt.Errorf("EvalBackend %s requires EvalFile", backend)
		}
	default:
		return fmt.Errorf("unsupported EvalBackend %q (want hce, ngn-v1, ngn-k4-768-v1, sf18-big, counter-5.5, rodent-v1.1-anand, or rodent-v1.2-default)", backend)
	}

	staged, err := loadUCIEvaluatorModel(path)
	if err != nil {
		return err
	}
	if staged.backendName() != backend {
		return fmt.Errorf("EvalFile contains %s model, not requested %s", staged.backendName(), backend)
	}
	if backend == uciEvaluatorNGNV1 {
		err = uci.searcher.SelectNGNV1Evaluator(staged.ngnV1)
	} else if backend == uciEvaluatorNGNK4 {
		err = uci.searcher.SelectNGNK4Evaluator(staged.ngnK4)
	} else if backend == uciEvaluatorSF18BIG {
		err = uci.searcher.SelectSF18BIGEvaluator(staged.sf18Big)
	} else if backend == uciEvaluatorCounter55 {
		err = uci.searcher.SelectCounter55Evaluator(staged.counter55)
	} else if backend == uciEvaluatorRodentV11Anand {
		err = uci.searcher.SelectRodentV11AnandEvaluator(staged.rodentV11Anand)
	} else {
		err = uci.searcher.SelectRodentV12DefaultEvaluator(staged.rodentV12Default)
	}
	if err != nil {
		return fmt.Errorf("select %s: %w", backend, err)
	}
	uci.lifecycleMu.Lock()
	uci.evaluatorConfig = uciEvaluatorConfig{backend: backend, file: path, staged: staged}
	uci.startupDefaults = uciEvaluatorDefaults{backend: backend, file: path}
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
	uci.joinSearch(true)
	backend := strings.ToLower(strings.TrimSpace(value))
	if backend != uciEvaluatorHCE && backend != uciEvaluatorNGNV1 && backend != uciEvaluatorNGNK4 && backend != uciEvaluatorSF18BIG && backend != uciEvaluatorCounter55 &&
		backend != uciEvaluatorRodentV11Anand && backend != uciEvaluatorRodentV12Default {
		uci.sendEvalOptionError(output, "unsupported EvalBackend %q (want hce, ngn-v1, ngn-k4-768-v1, sf18-big, counter-5.5, rodent-v1.1-anand, or rodent-v1.2-default)", value)
		return
	}

	config := uci.evaluatorConfigSnapshot()
	var err error
	switch backend {
	case uciEvaluatorHCE:
		err = uci.searcher.SelectHCEEvaluator()
	case uciEvaluatorNGNV1:
		if config.file == "" || config.staged.kind != stagedEvaluatorNGNV1 || config.staged.ngnV1 == nil {
			uci.sendEvalOptionError(output, "EvalBackend ngn-v1 requires a matching valid staged EvalFile")
			return
		}
		err = uci.searcher.SelectNGNV1Evaluator(config.staged.ngnV1)
	case uciEvaluatorNGNK4:
		if config.file == "" || config.staged.kind != stagedEvaluatorNGNK4 || config.staged.ngnK4 == nil {
			uci.sendEvalOptionError(output, "EvalBackend ngn-k4-768-v1 requires a matching valid staged EvalFile")
			return
		}
		err = uci.searcher.SelectNGNK4Evaluator(config.staged.ngnK4)
	case uciEvaluatorSF18BIG:
		if config.file == "" || config.staged.kind != stagedEvaluatorSF18BIG || config.staged.sf18Big == nil {
			uci.sendEvalOptionError(output, "EvalBackend sf18-big requires a matching valid staged EvalFile")
			return
		}
		err = uci.searcher.SelectSF18BIGEvaluator(config.staged.sf18Big)
	case uciEvaluatorCounter55:
		if config.file == "" || config.staged.kind != stagedEvaluatorCounter55 || config.staged.counter55 == nil {
			uci.sendEvalOptionError(output, "EvalBackend counter-5.5 requires a matching valid staged EvalFile")
			return
		}
		err = uci.searcher.SelectCounter55Evaluator(config.staged.counter55)
	case uciEvaluatorRodentV11Anand:
		if config.file == "" || config.staged.kind != stagedEvaluatorRodentV11Anand || config.staged.rodentV11Anand == nil {
			uci.sendEvalOptionError(output, "EvalBackend rodent-v1.1-anand requires a matching valid staged EvalFile")
			return
		}
		err = uci.searcher.SelectRodentV11AnandEvaluator(config.staged.rodentV11Anand)
	case uciEvaluatorRodentV12Default:
		if config.file == "" || config.staged.kind != stagedEvaluatorRodentV12Default || config.staged.rodentV12Default == nil {
			uci.sendEvalOptionError(output, "EvalBackend rodent-v1.2-default requires a matching valid staged EvalFile")
			return
		}
		err = uci.searcher.SelectRodentV12DefaultEvaluator(config.staged.rodentV12Default)
	}
	if err != nil {
		uci.sendEvalOptionError(output, "select %s: %v", backend, err)
		return
	}

	uci.lifecycleMu.Lock()
	uci.evaluatorConfig.backend = backend
	uci.lifecycleMu.Unlock()
}

func (uci *UCIEngine) handleEvalFileOption(value string, output io.Writer) {
	uci.joinSearch(true)
	path := strings.TrimSpace(value)
	config := uci.evaluatorConfigSnapshot()
	if path == "" || strings.EqualFold(path, "<empty>") {
		if config.backendName() != uciEvaluatorHCE {
			uci.sendEvalOptionError(output, "cannot clear EvalFile while EvalBackend is %s", config.backendName())
			return
		}
		uci.lifecycleMu.Lock()
		uci.evaluatorConfig.file = ""
		uci.evaluatorConfig.staged = stagedEvaluatorModel{}
		uci.lifecycleMu.Unlock()
		return
	}

	staged, err := loadUCIEvaluatorModel(path)
	if err != nil {
		uci.sendEvalOptionError(output, "%v", err)
		return
	}
	active := config.backendName()
	if active != uciEvaluatorHCE && active != staged.backendName() {
		uci.sendEvalOptionError(output, "cannot replace active %s with %s EvalFile; select hce first", active, staged.backendName())
		return
	}
	switch active {
	case uciEvaluatorNGNV1:
		err = uci.searcher.SelectNGNV1Evaluator(staged.ngnV1)
	case uciEvaluatorNGNK4:
		err = uci.searcher.SelectNGNK4Evaluator(staged.ngnK4)
	case uciEvaluatorSF18BIG:
		err = uci.searcher.SelectSF18BIGEvaluator(staged.sf18Big)
	case uciEvaluatorCounter55:
		err = uci.searcher.SelectCounter55Evaluator(staged.counter55)
	case uciEvaluatorRodentV11Anand:
		err = uci.searcher.SelectRodentV11AnandEvaluator(staged.rodentV11Anand)
	case uciEvaluatorRodentV12Default:
		err = uci.searcher.SelectRodentV12DefaultEvaluator(staged.rodentV12Default)
	}
	if err != nil {
		uci.sendEvalOptionError(output, "replace active %s model: %v", active, err)
		return
	}

	uci.lifecycleMu.Lock()
	uci.evaluatorConfig.file = path
	uci.evaluatorConfig.staged = staged
	uci.lifecycleMu.Unlock()
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
