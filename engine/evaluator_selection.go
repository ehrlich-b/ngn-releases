package engine

import "fmt"

func (e *SearchEngine) selectedEvaluatorModel(generation uint64) evaluatorModel {
	model := e.evaluatorModel
	if model.identity.adapterRevision == 0 {
		return hceEvaluatorModel(generation)
	}
	model.identity.hceGeneration = generation
	return model
}

func prepareWorkerEvaluator(worker *searchWorker, model evaluatorModel, pos *Position, generation uint64) (*workerEvaluator, error) {
	hce := worker.prepareHCEGeneration(pos, generation)
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

const EvaluatorBackendHCEName = "hce"

func evaluatorBackendName(backend evaluatorBackend) string {
	if backend == evaluatorBackendHCE {
		return EvaluatorBackendHCEName
	}
	return fmt.Sprintf("unknown-%d", backend)
}

func (e *SearchEngine) SelectedEvaluatorBackend() string {
	e.sessionMu.Lock()
	defer e.sessionMu.Unlock()
	generation := mustAcquireHCEModelUse()
	defer releaseHCEModelUse()
	return evaluatorBackendName(e.selectedEvaluatorModel(generation).identity.backend)
}

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
