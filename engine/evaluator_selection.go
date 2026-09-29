package engine

import (
	"fmt"

	"github.com/ehrlich-b/ngn/countereval"
	"github.com/ehrlich-b/ngn/nnue"
	"github.com/ehrlich-b/ngn/nnue/ngnk4"
	"github.com/ehrlich-b/ngn/nnue/sf18big"
	"github.com/ehrlich-b/ngn/rodenteval"
	"github.com/ehrlich-b/ngn/rodentv12eval"
)

// selectedEvaluatorModel returns the receiver's immutable evaluator selection
// with the process HCE/PST generation admitted for this operation. A zero-value
// SearchEngine preserves the historical HCE default.
func (e *SearchEngine) selectedEvaluatorModel(generation uint64) evaluatorModel {
	model := e.evaluatorModel
	if model.identity.adapterRevision == 0 {
		return hceEvaluatorModel(generation)
	}
	model.identity.hceGeneration = generation
	return model
}

// prepareWorkerEvaluator prepares one explicit worker against an already selected
// immutable model. The caller holds the owning engine session lock and an HCE
// model-use lease. Keeping the worker and model explicit lets pool construction
// validate new workers off to the side before publishing a new immutable pool.
func prepareWorkerEvaluator(worker *searchWorker, model evaluatorModel, pos *Position, generation uint64) (*workerEvaluator, error) {
	var hce *hceEvaluator
	if model.identity.backend == evaluatorBackendHCE {
		hce = worker.prepareHCEGeneration(pos, generation)
	} else {
		// NNUE still needs the admitted process PST generation and an unconditional
		// root accumulator rebuild, but it does not allocate the 16 MiB HCE cache.
		if worker.hce != nil && worker.hce.seenGeneration != generation {
			worker.hce.clearForGeneration(generation)
			worker.history = workerHistory{}
		}
		if pos != nil {
			pos.Board.recomputeAccumulator()
		}
	}
	identity := model.identity
	if worker.evaluatorIdentity != identity {
		// A never-admitted zero-value/default-HCE worker has no stale model state.
		// Preserve history seeded through the legacy same-model setup path. An
		// explicit selection prebuilds its evaluator, so first admission after a
		// selection still clears state belonging to the prior/default backend.
		if worker.evaluator != nil || worker.evaluatorIdentity.adapterRevision != 0 {
			worker.history = workerHistory{}
		}
		worker.evaluatorIdentity = identity
	}
	if worker.evaluator == nil || worker.evaluator.Identity() != identity {
		evaluator, err := model.newWorker(hce)
		if err != nil {
			return nil, fmt.Errorf("%w: construct admitted worker: %v", errWorkerEvaluator, err)
		}
		worker.evaluator = evaluator
	}
	if pos != nil {
		if err := worker.evaluator.Reset(pos); err != nil {
			return nil, fmt.Errorf("%w: reset admitted worker: %v", errWorkerEvaluator, err)
		}
	}
	return worker.evaluator, nil
}

func (e *SearchEngine) mustPreparePrimaryEvaluator(pos *Position, generation uint64) *workerEvaluator {
	evaluator, err := prepareWorkerEvaluator(&e.worker, e.selectedEvaluatorModel(generation), pos, generation)
	if err != nil {
		panic(err)
	}
	return evaluator
}

// SelectHCEEvaluator transactionally selects the historical HCE backend for
// this receiver. It is idle/session serialized and returns ErrHCEModelBusy
// rather than waiting if a process-wide HCE mutation is active.
func (e *SearchEngine) SelectHCEEvaluator() error {
	e.sessionMu.Lock()
	defer e.sessionMu.Unlock()
	generation, err := acquireHCEModelUse()
	if err != nil {
		return err
	}
	defer releaseHCEModelUse()

	hce := e.worker.hce
	if hce == nil {
		hce = &hceEvaluator{seenGeneration: generation}
	}
	model := hceEvaluatorModel(generation)
	worker, err := model.newWorker(hce)
	if err != nil {
		return err
	}

	// Publish only after every fallible validation/allocation has succeeded.
	e.worker.hce = hce
	e.evaluatorModel = model
	e.worker.evaluator = worker
	return nil
}

// SelectNGNV1Evaluator transactionally selects one exact validated NGN-v1
// model for this receiver. The immutable model may be shared; the candidate
// worker Context is built privately before publication. Arbitrary Stockfish
// .nnue formats are outside this API and are rejected by nnue.Load.
func (e *SearchEngine) SelectNGNV1Evaluator(model *nnue.Model) error {
	e.sessionMu.Lock()
	defer e.sessionMu.Unlock()
	generation, err := acquireHCEModelUse()
	if err != nil {
		return err
	}
	defer releaseHCEModelUse()

	candidate, err := ngnV1EvaluatorModel(model, generation)
	if err != nil {
		return err
	}
	worker, err := candidate.newWorker(nil)
	if err != nil {
		return err
	}

	// Publication is one receiver-local critical section. TT/history invalidation
	// occurs lazily before the next receiver operation observes this identity.
	e.evaluatorModel = candidate
	e.worker.evaluator = worker
	return nil
}

// SelectNGNK4Evaluator transactionally selects one strict-loader-validated
// NGN-owned king-bucketed model. Mutable accumulator state remains worker-local.
func (e *SearchEngine) SelectNGNK4Evaluator(model *ngnk4.Model) error {
	e.sessionMu.Lock()
	defer e.sessionMu.Unlock()
	generation, err := acquireHCEModelUse()
	if err != nil {
		return err
	}
	defer releaseHCEModelUse()

	candidate, err := ngnK4EvaluatorModel(model, generation, e.k4EvalScale())
	if err != nil {
		return err
	}
	worker, err := candidate.newWorker(nil)
	if err != nil {
		return err
	}
	e.evaluatorModel = candidate
	e.worker.evaluator = worker
	return nil
}

// SelectSF18BIGEvaluator transactionally selects one exact validated
// Stockfish 18 BIG model. The backend is opt-in and never changes the default
// evaluator.
func (e *SearchEngine) SelectSF18BIGEvaluator(model *sf18big.Model) error {
	e.sessionMu.Lock()
	defer e.sessionMu.Unlock()
	generation, err := acquireHCEModelUse()
	if err != nil {
		return err
	}
	defer releaseHCEModelUse()

	candidate, err := sf18BigEvaluatorModel(model, generation)
	if err != nil {
		return err
	}
	worker, err := candidate.newWorker(nil)
	if err != nil {
		return err
	}
	e.evaluatorModel = candidate
	e.worker.evaluator = worker
	return nil
}

// SelectSF18BIGResearchEvaluator preserves the bounded-research API used by
// the dedicated startup-locked process and historical experiments.
func (e *SearchEngine) SelectSF18BIGResearchEvaluator(model *sf18big.Model) error {
	return e.SelectSF18BIGEvaluator(model)
}

// SelectCounter55Evaluator transactionally selects one exact loader-validated
// Counter 5.5 model. Each worker receives private dynamically growing state.
func (e *SearchEngine) SelectCounter55Evaluator(model *countereval.Model) error {
	e.sessionMu.Lock()
	defer e.sessionMu.Unlock()
	generation, err := acquireHCEModelUse()
	if err != nil {
		return err
	}
	defer releaseHCEModelUse()

	candidate, err := counter55EvaluatorModel(model, generation)
	if err != nil {
		return err
	}
	worker, err := candidate.newWorker(nil)
	if err != nil {
		return err
	}
	e.evaluatorModel = candidate
	e.worker.evaluator = worker
	return nil
}

// SelectRodentV11AnandEvaluator transactionally selects the exact
// strict-loader-validated Anand artifact. Each worker receives private state.
func (e *SearchEngine) SelectRodentV11AnandEvaluator(model *rodenteval.Model) error {
	e.sessionMu.Lock()
	defer e.sessionMu.Unlock()
	generation, err := acquireHCEModelUse()
	if err != nil {
		return err
	}
	defer releaseHCEModelUse()

	candidate, err := rodentV11AnandEvaluatorModel(model, generation)
	if err != nil {
		return err
	}
	worker, err := candidate.newWorker(nil)
	if err != nil {
		return err
	}
	e.evaluatorModel = candidate
	e.worker.evaluator = worker
	return nil
}

// SelectRodentV12DefaultEvaluator transactionally selects the exact
// strict-loader-validated V1.2 default artifact. Each worker receives private
// dynamically growing state.
func (e *SearchEngine) SelectRodentV12DefaultEvaluator(model *rodentv12eval.Model) error {
	e.sessionMu.Lock()
	defer e.sessionMu.Unlock()
	generation, err := acquireHCEModelUse()
	if err != nil {
		return err
	}
	defer releaseHCEModelUse()

	candidate, err := rodentV12DefaultEvaluatorModel(model, generation)
	if err != nil {
		return err
	}
	worker, err := candidate.newWorker(nil)
	if err != nil {
		return err
	}
	e.evaluatorModel = candidate
	e.worker.evaluator = worker
	return nil
}

const (
	EvaluatorBackendHCEName              = "hce"
	EvaluatorBackendNGNV1Name            = "ngn-v1"
	EvaluatorBackendSF18BIGName          = "sf18-big"
	EvaluatorBackendSF18BIGResearchName  = EvaluatorBackendSF18BIGName
	EvaluatorBackendCounter55Name        = "counter-5.5"
	EvaluatorBackendRodentV11AnandName   = "rodent-v1.1-anand"
	EvaluatorBackendRodentV12DefaultName = "rodent-v1.2-default"
	EvaluatorBackendNGNK4Name            = "ngn-k4-768-v1"
)

func evaluatorBackendName(backend evaluatorBackend) string {
	switch backend {
	case evaluatorBackendHCE:
		return EvaluatorBackendHCEName
	case evaluatorBackendNGNV1:
		return EvaluatorBackendNGNV1Name
	case evaluatorBackendSF18BIGResearch:
		return EvaluatorBackendSF18BIGName
	case evaluatorBackendCounter55:
		return EvaluatorBackendCounter55Name
	case evaluatorBackendRodentV11Anand:
		return EvaluatorBackendRodentV11AnandName
	case evaluatorBackendRodentV12Default:
		return EvaluatorBackendRodentV12DefaultName
	case evaluatorBackendNGNK4:
		return EvaluatorBackendNGNK4Name
	default:
		return fmt.Sprintf("unknown-%d", backend)
	}
}

// SelectedEvaluatorBackend reports the receiver's immutable evaluator selection.
// It is session serialized and participates in the process HCE/PST read lease.
func (e *SearchEngine) SelectedEvaluatorBackend() string {
	e.sessionMu.Lock()
	defer e.sessionMu.Unlock()
	generation := mustAcquireHCEModelUse()
	defer releaseHCEModelUse()
	return evaluatorBackendName(e.selectedEvaluatorModel(generation).identity.backend)
}

// EvaluateSelected reports the selected backend's base SearchSTM score for pos:
// side-to-move centipawns with the backend's declared rule-50/score adapter, and
// without search correction-history terms.
func (e *SearchEngine) EvaluateSelected(pos *Position) (score int, backend string, err error) {
	if pos == nil {
		return 0, "", fmt.Errorf("%w: nil evaluation position", errWorkerEvaluator)
	}
	e.sessionMu.Lock()
	defer e.sessionMu.Unlock()
	generation, err := acquireHCEModelUse()
	if err != nil {
		return 0, "", err
	}
	defer releaseHCEModelUse()
	model := e.selectedEvaluatorModel(generation)
	worker, err := prepareWorkerEvaluator(&e.worker, model, pos, generation)
	if err != nil {
		return 0, "", err
	}
	return worker.SearchSTM(pos), evaluatorBackendName(worker.Identity().backend), nil
}
