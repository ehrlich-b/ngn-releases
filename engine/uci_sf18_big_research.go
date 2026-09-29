package engine

import (
	"fmt"
	"os"
	"strings"

	"github.com/ehrlich-b/ngn/nnue/sf18big"
)

// ConfigureSF18BIGResearchStartup installs an exact BIG model into a dedicated
// research UCI process. Once installed, evaluator options are hidden and locked
// so a match cannot silently switch either role mid-run. The normal process may
// select the same validated implementation explicitly through EvalBackend.
func (uci *UCIEngine) ConfigureSF18BIGResearchStartup(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("SF18 BIG research model path is empty")
	}
	if strings.ContainsAny(path, "\r\n") {
		return fmt.Errorf("SF18 BIG research model path contains a line break")
	}
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open SF18 BIG research model %q: %w", path, err)
	}
	model, loadErr := sf18big.Load(file)
	closeErr := file.Close()
	if loadErr != nil {
		return fmt.Errorf("load SF18 BIG research model %q: %w", path, loadErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close SF18 BIG research model %q: %w", path, closeErr)
	}
	if err := uci.searcher.SelectSF18BIGEvaluator(model); err != nil {
		return fmt.Errorf("select SF18 BIG research evaluator: %w", err)
	}

	uci.lifecycleMu.Lock()
	uci.researchEvaluatorLocked = true
	uci.engineName = "ngn-sf18-big-research"
	uci.lifecycleMu.Unlock()
	return nil
}

func (uci *UCIEngine) researchEvaluatorIsLocked() bool {
	uci.lifecycleMu.Lock()
	defer uci.lifecycleMu.Unlock()
	return uci.researchEvaluatorLocked
}
