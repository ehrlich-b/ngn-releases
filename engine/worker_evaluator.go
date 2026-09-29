package engine

import (
	"errors"
	"fmt"
	"math"
	"math/bits"

	"github.com/ehrlich-b/ngn/countereval"
	"github.com/ehrlich-b/ngn/nnue"
	"github.com/ehrlich-b/ngn/nnue/ngnk4"
	"github.com/ehrlich-b/ngn/nnue/sf18big"
	"github.com/ehrlich-b/ngn/rodenteval"
	"github.com/ehrlich-b/ngn/rodentv12eval"
)

const (
	nnueStaticEvalLimit      int64  = 25000
	evaluatorAdapterRevision uint32 = 1
	counter55AdapterRevision uint32 = 1
	sf18BigAdapterRevision   uint32 = 1
	rodentV11AdapterRevision uint32 = 1
	rodentV12AdapterRevision uint32 = 1
)

var errWorkerEvaluator = errors.New("invalid worker evaluator state")

type evaluatorBackend uint8

const (
	evaluatorBackendHCE evaluatorBackend = iota
	evaluatorBackendNGNV1
	evaluatorBackendSF18BIGResearch
	evaluatorBackendCounter55
	evaluatorBackendRodentV11Anand
	evaluatorBackendRodentV12Default
	evaluatorBackendNGNK4
)

// evaluatorModelIdentity is comparable and covers every input that can change
// a search score or the hidden HCE accumulator maintenance performed by board
// make/unmake. The HCE generation remains part of an NNUE identity until those
// process-global PST accumulators are removed.
type evaluatorModelIdentity struct {
	backend           evaluatorBackend
	adapterRevision   uint32
	hceGeneration     uint64
	nnueMetadata      nnue.Metadata
	sf18BigMetadata   sf18big.Metadata
	counterMetadata   countereval.LoadMetadata
	rodentMetadata    rodenteval.Metadata
	rodentV12Metadata rodentv12eval.Metadata
	ngnK4Metadata     ngnk4.Metadata
	ngnK4ScalePercent int64
}

// evaluatorModel is a cold immutable selection. Its NGN-v1 pointer is safe to
// share because a validated nnue.Model is immutable; mutable Context state is
// constructed separately for every worker.
type evaluatorModel struct {
	identity         evaluatorModelIdentity
	ngnV1            *nnue.Model
	sf18Big          *sf18big.Model
	counter55        *countereval.Model
	rodentV11Anand   *rodenteval.Model
	rodentV12Default *rodentv12eval.Model
	ngnK4            *ngnk4.Model
}

func hceEvaluatorModel(hceGeneration uint64) evaluatorModel {
	return evaluatorModel{identity: evaluatorModelIdentity{
		backend:         evaluatorBackendHCE,
		adapterRevision: evaluatorAdapterRevision,
		hceGeneration:   hceGeneration,
	}}
}

func ngnV1EvaluatorModel(model *nnue.Model, hceGeneration uint64) (evaluatorModel, error) {
	context, err := nnue.NewContext(model)
	if err != nil {
		return evaluatorModel{}, fmt.Errorf("%w: %v", errWorkerEvaluator, err)
	}
	_ = context // Validation only; each worker constructs its own Context below.
	return evaluatorModel{
		identity: evaluatorModelIdentity{
			backend:         evaluatorBackendNGNV1,
			adapterRevision: evaluatorAdapterRevision,
			hceGeneration:   hceGeneration,
			nnueMetadata:    model.Metadata(),
		},
		ngnV1: model,
	}, nil
}

func sf18BigEvaluatorModel(model *sf18big.Model, hceGeneration uint64) (evaluatorModel, error) {
	context, err := sf18big.NewContext(model)
	if err != nil {
		return evaluatorModel{}, fmt.Errorf("%w: %v", errWorkerEvaluator, err)
	}
	_ = context
	return evaluatorModel{
		identity: evaluatorModelIdentity{
			backend:         evaluatorBackendSF18BIGResearch,
			adapterRevision: sf18BigAdapterRevision,
			hceGeneration:   hceGeneration,
			sf18BigMetadata: model.Metadata(),
		},
		sf18Big: model,
	}, nil
}

func counter55EvaluatorModel(model *countereval.Model, hceGeneration uint64) (evaluatorModel, error) {
	metadata, err := model.ValidatedMetadata()
	if err != nil {
		return evaluatorModel{}, fmt.Errorf("%w: %v", errWorkerEvaluator, err)
	}
	return evaluatorModel{
		identity: evaluatorModelIdentity{
			backend:         evaluatorBackendCounter55,
			adapterRevision: counter55AdapterRevision,
			hceGeneration:   hceGeneration,
			counterMetadata: metadata,
		},
		counter55: model,
	}, nil
}

func rodentV11AnandEvaluatorModel(model *rodenteval.Model, hceGeneration uint64) (evaluatorModel, error) {
	metadata, err := model.Metadata()
	if err != nil {
		return evaluatorModel{}, fmt.Errorf("%w: %v", errWorkerEvaluator, err)
	}
	return evaluatorModel{
		identity: evaluatorModelIdentity{
			backend:         evaluatorBackendRodentV11Anand,
			adapterRevision: rodentV11AdapterRevision,
			hceGeneration:   hceGeneration,
			rodentMetadata:  metadata,
		},
		rodentV11Anand: model,
	}, nil
}

func rodentV12DefaultEvaluatorModel(model *rodentv12eval.Model, hceGeneration uint64) (evaluatorModel, error) {
	metadata, err := model.Metadata()
	if err != nil {
		return evaluatorModel{}, fmt.Errorf("%w: %v", errWorkerEvaluator, err)
	}
	return evaluatorModel{
		identity: evaluatorModelIdentity{
			backend:           evaluatorBackendRodentV12Default,
			adapterRevision:   rodentV12AdapterRevision,
			hceGeneration:     hceGeneration,
			rodentV12Metadata: metadata,
		},
		rodentV12Default: model,
	}, nil
}

func ngnK4EvaluatorModel(model *ngnk4.Model, hceGeneration uint64, scalePercent int64) (evaluatorModel, error) {
	if model == nil {
		return evaluatorModel{}, fmt.Errorf("%w: nil NGN K4 model", errWorkerEvaluator)
	}
	metadata := model.Metadata()
	if metadata.ArchitectureID != ngnk4.ArchitectureID {
		return evaluatorModel{}, fmt.Errorf("%w: NGN K4 model is not loader-validated", errWorkerEvaluator)
	}
	var seed ngnk4.Position
	seed.Board[ngnk4.WhiteKing] = uint64(1) << 4
	seed.Board[ngnk4.BlackKing] = uint64(1) << 60
	if _, err := model.NewSearchContext(seed); err != nil {
		return evaluatorModel{}, fmt.Errorf("%w: %v", errWorkerEvaluator, err)
	}
	return evaluatorModel{
		identity: evaluatorModelIdentity{
			backend:           evaluatorBackendNGNK4,
			adapterRevision:   evaluatorAdapterRevision,
			hceGeneration:     hceGeneration,
			ngnK4Metadata:     metadata,
			ngnK4ScalePercent: scalePercent,
		},
		ngnK4: model,
	}, nil
}

// workerEvaluator is a hot concrete tagged union. Backend selection happens
// once per score/transition call; NNUE's hidden-unit loops remain concrete and
// contain no interface dispatch. It owns all mutable NNUE state for one worker.
type workerEvaluator struct {
	model            evaluatorModel
	hce              *hceEvaluator
	context          *nnue.Context
	portableContext  *nnue.Int32Context
	sf18BigContext   *sf18big.Context
	counterContext   *countereval.SearchContext
	rodentContext    *rodenteval.SearchContext
	rodentV12Context *rodentv12eval.SearchContext
	ngnK4Context     *ngnk4.SearchContext
	rootBuffer       nnueRootBuffer

	// These receiver-private hooks are set only by same-package invariant tests:
	// one checks every committed incremental frame lane-by-lane, and one routes
	// score reads through a fresh full-position oracle. Public selection never
	// enables either mode.
	transitionObserver func(*Position, *workerEvaluator)
	fullRefreshOracle  bool
}

func (m evaluatorModel) newWorker(hce *hceEvaluator) (*workerEvaluator, error) {
	return m.newWorkerWithReferenceContext(hce, false)
}

// newWorkerWithReferenceContext is a private test seam that forces the retained
// int64 incremental oracle without weakening model validation or model identity.
func (m evaluatorModel) newWorkerWithReferenceContext(hce *hceEvaluator, forceReference bool) (*workerEvaluator, error) {
	worker := &workerEvaluator{model: m}
	switch m.identity.backend {
	case evaluatorBackendHCE:
		if hce == nil {
			return nil, fmt.Errorf("%w: nil HCE evaluator", errWorkerEvaluator)
		}
		worker.hce = hce
	case evaluatorBackendNGNV1:
		if m.ngnV1 == nil {
			return nil, fmt.Errorf("%w: nil NGN-v1 model", errWorkerEvaluator)
		}
		if m.ngnV1.Capabilities().PortableInt32Accumulator && !forceReference {
			context, err := nnue.NewInt32Context(m.ngnV1)
			if err != nil {
				return nil, fmt.Errorf("%w: %v", errWorkerEvaluator, err)
			}
			worker.portableContext = context
		} else {
			context, err := nnue.NewContext(m.ngnV1)
			if err != nil {
				return nil, fmt.Errorf("%w: %v", errWorkerEvaluator, err)
			}
			worker.context = context
		}
	case evaluatorBackendSF18BIGResearch:
		if forceReference {
			return nil, fmt.Errorf("%w: SF18 BIG has no alternate context", errWorkerEvaluator)
		}
		if m.sf18Big == nil {
			return nil, fmt.Errorf("%w: nil SF18 BIG model", errWorkerEvaluator)
		}
		context, err := sf18big.NewContext(m.sf18Big)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", errWorkerEvaluator, err)
		}
		worker.sf18BigContext = context
	case evaluatorBackendCounter55:
		if forceReference {
			return nil, fmt.Errorf("%w: Counter has no alternate worker context", errWorkerEvaluator)
		}
		context, err := m.counter55.NewSearchContext(countereval.Board{})
		if err != nil {
			return nil, fmt.Errorf("%w: %v", errWorkerEvaluator, err)
		}
		worker.counterContext = context
	case evaluatorBackendRodentV11Anand:
		if forceReference {
			return nil, fmt.Errorf("%w: Rodent has no alternate worker context", errWorkerEvaluator)
		}
		if m.rodentV11Anand == nil {
			return nil, fmt.Errorf("%w: nil Rodent V1.1 Anand model", errWorkerEvaluator)
		}
		var seed rodenteval.Position
		seed.Board[rodenteval.WhiteKing] = uint64(1) << 4
		seed.Board[rodenteval.BlackKing] = uint64(1) << 60
		seed.SideToMove = rodenteval.White
		context, err := m.rodentV11Anand.NewSearchContext(seed)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", errWorkerEvaluator, err)
		}
		worker.rodentContext = context
	case evaluatorBackendRodentV12Default:
		if forceReference {
			return nil, fmt.Errorf("%w: Rodent V1.2 has no alternate worker context", errWorkerEvaluator)
		}
		if m.rodentV12Default == nil {
			return nil, fmt.Errorf("%w: nil Rodent V1.2 default model", errWorkerEvaluator)
		}
		var seed rodentv12eval.Position
		seed.Board[rodentv12eval.WhiteKing] = uint64(1) << 4
		seed.Board[rodentv12eval.BlackKing] = uint64(1) << 60
		seed.SideToMove = rodentv12eval.White
		context, err := m.rodentV12Default.NewSearchContext(seed)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", errWorkerEvaluator, err)
		}
		worker.rodentV12Context = context
	case evaluatorBackendNGNK4:
		if forceReference {
			return nil, fmt.Errorf("%w: NGN K4 has no alternate context", errWorkerEvaluator)
		}
		if m.ngnK4 == nil {
			return nil, fmt.Errorf("%w: nil NGN K4 model", errWorkerEvaluator)
		}
		var seed ngnk4.Position
		seed.Board[ngnk4.WhiteKing] = uint64(1) << 4
		seed.Board[ngnk4.BlackKing] = uint64(1) << 60
		context, err := m.ngnK4.NewSearchContext(seed)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", errWorkerEvaluator, err)
		}
		worker.ngnK4Context = context
	default:
		return nil, fmt.Errorf("%w: unknown backend %d", errWorkerEvaluator, m.identity.backend)
	}
	return worker, nil
}

func (e *workerEvaluator) Identity() evaluatorModelIdentity {
	if e == nil {
		return evaluatorModelIdentity{}
	}
	return e.model.identity
}

// Reset binds this worker's mutable context to the admitted search root. HCE's
// root PST refresh remains owned by search admission and is intentionally not
// duplicated here.
func (e *workerEvaluator) Reset(pos *Position) error {
	if e == nil || pos == nil {
		return fmt.Errorf("%w: nil evaluator or root position", errWorkerEvaluator)
	}
	switch e.model.identity.backend {
	case evaluatorBackendHCE:
		if e.hce == nil {
			return fmt.Errorf("%w: nil HCE evaluator", errWorkerEvaluator)
		}
		return nil
	case evaluatorBackendNGNV1:
		root, err := e.rootBuffer.position(pos)
		if err != nil {
			return err
		}
		if err := e.resetNNUE(root); err != nil {
			return fmt.Errorf("%w: reset: %v", errWorkerEvaluator, err)
		}
		e.observeTransition(pos)
		return nil
	case evaluatorBackendSF18BIGResearch:
		root, err := e.rootBuffer.position(pos)
		if err != nil {
			return err
		}
		if e.sf18BigContext == nil {
			return fmt.Errorf("%w: nil SF18 BIG context", errWorkerEvaluator)
		}
		if err := e.sf18BigContext.Reset(root); err != nil {
			return fmt.Errorf("%w: SF18 BIG reset: %v", errWorkerEvaluator, err)
		}
		e.observeTransition(pos)
		return nil
	case evaluatorBackendCounter55:
		root, err := counterBoardFromPosition(pos)
		if err != nil {
			return err
		}
		if e.counterContext == nil {
			return fmt.Errorf("%w: nil Counter context", errWorkerEvaluator)
		}
		if err := e.counterContext.Reset(root); err != nil {
			return fmt.Errorf("%w: Counter reset: %v", errWorkerEvaluator, err)
		}
		e.observeTransition(pos)
		return nil
	case evaluatorBackendRodentV11Anand:
		root, err := rodentPositionFromPosition(pos)
		if err != nil {
			return err
		}
		if e.rodentContext == nil {
			return fmt.Errorf("%w: nil Rodent context", errWorkerEvaluator)
		}
		if err := e.rodentContext.Reset(root); err != nil {
			return fmt.Errorf("%w: Rodent reset: %v", errWorkerEvaluator, err)
		}
		e.observeTransition(pos)
		return nil
	case evaluatorBackendRodentV12Default:
		root, err := rodentV12PositionFromPosition(pos)
		if err != nil {
			return err
		}
		if e.rodentV12Context == nil {
			return fmt.Errorf("%w: nil Rodent V1.2 context", errWorkerEvaluator)
		}
		if err := e.rodentV12Context.Reset(root); err != nil {
			return fmt.Errorf("%w: Rodent V1.2 reset: %v", errWorkerEvaluator, err)
		}
		e.observeTransition(pos)
		return nil
	case evaluatorBackendNGNK4:
		root, err := ngnK4PositionFromPosition(pos)
		if err != nil {
			return err
		}
		if e.ngnK4Context == nil {
			return fmt.Errorf("%w: nil NGN K4 context", errWorkerEvaluator)
		}
		if err := e.ngnK4Context.Reset(root); err != nil {
			return fmt.Errorf("%w: NGN K4 reset: %v", errWorkerEvaluator, err)
		}
		e.observeTransition(pos)
		return nil
	default:
		return fmt.Errorf("%w: unknown backend %d", errWorkerEvaluator, e.model.identity.backend)
	}
}

type workerEvalMove struct {
	delta          nnue.Delta
	counterDelta   countereval.MoveDelta
	rodentDelta    rodenteval.MoveDelta
	rodentV12Delta rodentv12eval.MoveDelta
	ngnK4Delta     ngnk4.MoveDelta
	active         bool
}

type workerEvalNull struct {
	before       nnue.PositionFacts
	beforePieces nnuePieceBitboards
	beforeSide   nnue.Color
	active       bool
}

// PrepareMove captures an NNUE transition from the actual pre-move position.
// Search may discard the token for an illegal move without touching Context.
func (e *workerEvaluator) PrepareMove(pos *Position, move Move) (workerEvalMove, error) {
	if e == nil {
		return workerEvalMove{}, fmt.Errorf("%w: nil evaluator", errWorkerEvaluator)
	}
	switch e.model.identity.backend {
	case evaluatorBackendHCE:
		return workerEvalMove{}, nil
	case evaluatorBackendNGNV1, evaluatorBackendSF18BIGResearch:
		delta, err := nnueMoveDelta(pos, move)
		if err != nil {
			return workerEvalMove{}, err
		}
		return workerEvalMove{delta: delta, active: true}, nil
	case evaluatorBackendCounter55:
		delta, err := counterMoveDelta(pos, move)
		if err != nil {
			return workerEvalMove{}, err
		}
		return workerEvalMove{counterDelta: delta, active: true}, nil
	case evaluatorBackendRodentV11Anand:
		delta, err := rodentMoveDelta(pos, move)
		if err != nil {
			return workerEvalMove{}, err
		}
		return workerEvalMove{rodentDelta: delta, active: true}, nil
	case evaluatorBackendRodentV12Default:
		delta, err := rodentV12MoveDelta(pos, move)
		if err != nil {
			return workerEvalMove{}, err
		}
		return workerEvalMove{rodentV12Delta: delta, active: true}, nil
	case evaluatorBackendNGNK4:
		delta, err := ngnK4MoveDelta(pos, move)
		if err != nil {
			return workerEvalMove{}, err
		}
		return workerEvalMove{ngnK4Delta: delta, active: true}, nil
	default:
		return workerEvalMove{}, fmt.Errorf("%w: unknown backend %d", errWorkerEvaluator, e.model.identity.backend)
	}
}

// PushMove verifies the engine's real post-move facts and side before committing
// the prepared NNUE frame. Failures are transactional in Context.
func (e *workerEvaluator) PushMove(pos *Position, transition workerEvalMove) error {
	if e == nil {
		return fmt.Errorf("%w: nil evaluator", errWorkerEvaluator)
	}
	switch e.model.identity.backend {
	case evaluatorBackendHCE:
		return nil
	case evaluatorBackendNGNV1:
		if !transition.active {
			return fmt.Errorf("%w: missing NNUE move transition", errWorkerEvaluator)
		}
		if err := validateNNUEMoveAfter(pos, transition.delta); err != nil {
			return err
		}
		if err := e.pushNNUEDelta(transition.delta); err != nil {
			return fmt.Errorf("%w: push move: %v", errWorkerEvaluator, err)
		}
		e.observeTransition(pos)
		return nil
	case evaluatorBackendSF18BIGResearch:
		if !transition.active || e.sf18BigContext == nil {
			return fmt.Errorf("%w: missing SF18 BIG move transition or context", errWorkerEvaluator)
		}
		if err := validateNNUEMoveAfter(pos, transition.delta); err != nil {
			return err
		}
		if err := e.sf18BigContext.PushDelta(transition.delta); err != nil {
			return fmt.Errorf("%w: SF18 BIG push move: %v", errWorkerEvaluator, err)
		}
		e.observeTransition(pos)
		return nil
	case evaluatorBackendCounter55:
		if !transition.active || e.counterContext == nil {
			return fmt.Errorf("%w: missing Counter move transition or context", errWorkerEvaluator)
		}
		post, err := validateCounterMoveAfter(pos, transition.counterDelta)
		if err != nil {
			return err
		}
		if err := e.counterContext.PushMove(transition.counterDelta, post); err != nil {
			return fmt.Errorf("%w: Counter push move: %v", errWorkerEvaluator, err)
		}
		e.observeTransition(pos)
		return nil
	case evaluatorBackendRodentV11Anand:
		if !transition.active || e.rodentContext == nil {
			return fmt.Errorf("%w: missing Rodent move transition or context", errWorkerEvaluator)
		}
		post, err := validateRodentMoveAfter(pos, transition.rodentDelta)
		if err != nil {
			return err
		}
		if err := e.rodentContext.PushMove(transition.rodentDelta, post); err != nil {
			return fmt.Errorf("%w: Rodent push move: %v", errWorkerEvaluator, err)
		}
		e.observeTransition(pos)
		return nil
	case evaluatorBackendRodentV12Default:
		if !transition.active || e.rodentV12Context == nil {
			return fmt.Errorf("%w: missing Rodent V1.2 move transition or context", errWorkerEvaluator)
		}
		post, err := validateRodentV12MoveAfter(pos, transition.rodentV12Delta)
		if err != nil {
			return err
		}
		if err := e.rodentV12Context.PushMove(transition.rodentV12Delta, post); err != nil {
			return fmt.Errorf("%w: Rodent V1.2 push move: %v", errWorkerEvaluator, err)
		}
		e.observeTransition(pos)
		return nil
	case evaluatorBackendNGNK4:
		if !transition.active || e.ngnK4Context == nil {
			return fmt.Errorf("%w: missing NGN K4 move transition or context", errWorkerEvaluator)
		}
		post, err := validateNGNK4MoveAfter(pos, transition.ngnK4Delta)
		if err != nil {
			return err
		}
		if err := e.ngnK4Context.PushMove(transition.ngnK4Delta, post); err != nil {
			return fmt.Errorf("%w: NGN K4 push move: %v", errWorkerEvaluator, err)
		}
		e.observeTransition(pos)
		return nil
	default:
		return fmt.Errorf("%w: unknown backend %d", errWorkerEvaluator, e.model.identity.backend)
	}
}

func (e *workerEvaluator) PrepareNull(pos *Position) (workerEvalNull, error) {
	if e == nil {
		return workerEvalNull{}, fmt.Errorf("%w: nil evaluator", errWorkerEvaluator)
	}
	switch e.model.identity.backend {
	case evaluatorBackendHCE:
		return workerEvalNull{}, nil
	case evaluatorBackendNGNV1, evaluatorBackendSF18BIGResearch, evaluatorBackendCounter55, evaluatorBackendRodentV11Anand, evaluatorBackendRodentV12Default, evaluatorBackendNGNK4:
		before, err := nnuePositionFacts(pos)
		if err != nil {
			return workerEvalNull{}, err
		}
		beforePieces, err := nnuePositionPieceBitboards(pos)
		if err != nil {
			return workerEvalNull{}, err
		}
		return workerEvalNull{
			before:       before,
			beforePieces: beforePieces,
			beforeSide:   engineColorForNNUE(pos.Turn()),
			active:       true,
		}, nil
	default:
		return workerEvalNull{}, fmt.Errorf("%w: unknown backend %d", errWorkerEvaluator, e.model.identity.backend)
	}
}

// PushNull validates the actual engine side flip and unchanged board facts before
// committing the alias-equivalent NNUE null frame.
func (e *workerEvaluator) PushNull(pos *Position, transition workerEvalNull) error {
	if e == nil {
		return fmt.Errorf("%w: nil evaluator", errWorkerEvaluator)
	}
	switch e.model.identity.backend {
	case evaluatorBackendHCE:
		return nil
	case evaluatorBackendNGNV1:
		if !transition.active {
			return fmt.Errorf("%w: missing NNUE null transition", errWorkerEvaluator)
		}
		if err := validateNNUENullAfter(pos, transition.before, transition.beforePieces, transition.beforeSide); err != nil {
			return err
		}
		if err := e.pushNNUENull(transition.before); err != nil {
			return fmt.Errorf("%w: push null: %v", errWorkerEvaluator, err)
		}
		e.observeTransition(pos)
		return nil
	case evaluatorBackendSF18BIGResearch:
		if !transition.active || e.sf18BigContext == nil {
			return fmt.Errorf("%w: missing SF18 BIG null transition or context", errWorkerEvaluator)
		}
		if err := validateNNUENullAfter(pos, transition.before, transition.beforePieces, transition.beforeSide); err != nil {
			return err
		}
		if err := e.sf18BigContext.PushNullFacts(transition.before); err != nil {
			return fmt.Errorf("%w: SF18 BIG push null: %v", errWorkerEvaluator, err)
		}
		e.observeTransition(pos)
		return nil
	case evaluatorBackendCounter55:
		if !transition.active || e.counterContext == nil {
			return fmt.Errorf("%w: missing Counter null transition or context", errWorkerEvaluator)
		}
		if err := validateNNUENullAfter(pos, transition.before, transition.beforePieces, transition.beforeSide); err != nil {
			return err
		}
		if err := e.counterContext.PushNull(); err != nil {
			return fmt.Errorf("%w: Counter push null: %v", errWorkerEvaluator, err)
		}
		e.observeTransition(pos)
		return nil
	case evaluatorBackendRodentV11Anand:
		if !transition.active || e.rodentContext == nil {
			return fmt.Errorf("%w: missing Rodent null transition or context", errWorkerEvaluator)
		}
		if err := validateNNUENullAfter(pos, transition.before, transition.beforePieces, transition.beforeSide); err != nil {
			return err
		}
		if err := e.rodentContext.PushNull(); err != nil {
			return fmt.Errorf("%w: Rodent push null: %v", errWorkerEvaluator, err)
		}
		e.observeTransition(pos)
		return nil
	case evaluatorBackendRodentV12Default:
		if !transition.active || e.rodentV12Context == nil {
			return fmt.Errorf("%w: missing Rodent V1.2 null transition or context", errWorkerEvaluator)
		}
		if err := validateNNUENullAfter(pos, transition.before, transition.beforePieces, transition.beforeSide); err != nil {
			return err
		}
		if err := e.rodentV12Context.PushNull(); err != nil {
			return fmt.Errorf("%w: Rodent V1.2 push null: %v", errWorkerEvaluator, err)
		}
		e.observeTransition(pos)
		return nil
	case evaluatorBackendNGNK4:
		if !transition.active || e.ngnK4Context == nil {
			return fmt.Errorf("%w: missing NGN K4 null transition or context", errWorkerEvaluator)
		}
		if err := validateNNUENullAfter(pos, transition.before, transition.beforePieces, transition.beforeSide); err != nil {
			return err
		}
		if err := e.ngnK4Context.PushNull(); err != nil {
			return fmt.Errorf("%w: NGN K4 push null: %v", errWorkerEvaluator, err)
		}
		e.observeTransition(pos)
		return nil
	default:
		return fmt.Errorf("%w: unknown backend %d", errWorkerEvaluator, e.model.identity.backend)
	}
}

// Pop must run before the matching engine Position unmake. HCE owns no
// move-indexed evaluator state, so its branch is deliberately empty.
func (e *workerEvaluator) Pop() error {
	if e == nil {
		return fmt.Errorf("%w: nil evaluator", errWorkerEvaluator)
	}
	switch e.model.identity.backend {
	case evaluatorBackendHCE:
		return nil
	case evaluatorBackendNGNV1:
		if err := e.popNNUE(); err != nil {
			return fmt.Errorf("%w: pop: %v", errWorkerEvaluator, err)
		}
		return nil
	case evaluatorBackendSF18BIGResearch:
		if e.sf18BigContext == nil {
			return fmt.Errorf("%w: nil SF18 BIG context", errWorkerEvaluator)
		}
		if err := e.sf18BigContext.Pop(); err != nil {
			return fmt.Errorf("%w: SF18 BIG pop: %v", errWorkerEvaluator, err)
		}
		return nil
	case evaluatorBackendCounter55:
		if e.counterContext == nil {
			return fmt.Errorf("%w: nil Counter context", errWorkerEvaluator)
		}
		if err := e.counterContext.Pop(); err != nil {
			return fmt.Errorf("%w: Counter pop: %v", errWorkerEvaluator, err)
		}
		return nil
	case evaluatorBackendRodentV11Anand:
		if e.rodentContext == nil {
			return fmt.Errorf("%w: nil Rodent context", errWorkerEvaluator)
		}
		if err := e.rodentContext.Pop(); err != nil {
			return fmt.Errorf("%w: Rodent pop: %v", errWorkerEvaluator, err)
		}
		return nil
	case evaluatorBackendRodentV12Default:
		if e.rodentV12Context == nil {
			return fmt.Errorf("%w: nil Rodent V1.2 context", errWorkerEvaluator)
		}
		if err := e.rodentV12Context.Pop(); err != nil {
			return fmt.Errorf("%w: Rodent V1.2 pop: %v", errWorkerEvaluator, err)
		}
		return nil
	case evaluatorBackendNGNK4:
		if e.ngnK4Context == nil {
			return fmt.Errorf("%w: nil NGN K4 context", errWorkerEvaluator)
		}
		if err := e.ngnK4Context.Pop(); err != nil {
			return fmt.Errorf("%w: NGN K4 pop: %v", errWorkerEvaluator, err)
		}
		return nil
	default:
		return fmt.Errorf("%w: unknown backend %d", errWorkerEvaluator, e.model.identity.backend)
	}
}

// SearchSTM is the ordinary static-evaluation route. HCE delegates to the exact
// M2 policy. NGN-v1 already returns side-to-move centipawns: attenuate rule-50
// optimism once in int64, clamp to the non-mate band, then convert to int.
func (e *workerEvaluator) SearchSTM(pos *Position) int {
	if e == nil || pos == nil {
		panic(fmt.Errorf("%w: nil evaluator or position", errWorkerEvaluator))
	}
	switch e.model.identity.backend {
	case evaluatorBackendHCE:
		if e.hce == nil {
			panic(fmt.Errorf("%w: nil HCE evaluator", errWorkerEvaluator))
		}
		return e.hce.SearchSTM(pos)
	case evaluatorBackendNGNV1:
		var raw int64
		var err error
		if e.fullRefreshOracle {
			full, fullErr := e.rootBuffer.position(pos)
			if fullErr != nil {
				panic(fullErr)
			}
			raw, err = e.model.ngnV1.Evaluate(full)
		} else {
			raw, err = e.evaluateNNUE()
		}
		if err != nil {
			panic(fmt.Errorf("%w: NNUE evaluate: %v", errWorkerEvaluator, err))
		}
		return nnueSearchScore(raw, pos.HalfMoveClock)
	case evaluatorBackendSF18BIGResearch:
		if e.sf18BigContext == nil {
			panic(fmt.Errorf("%w: nil SF18 BIG context", errWorkerEvaluator))
		}
		var selected sf18big.SelectedEvaluation
		var err error
		if e.fullRefreshOracle {
			full, fullErr := e.rootBuffer.position(pos)
			if fullErr != nil {
				panic(fullErr)
			}
			selected, err = e.model.sf18Big.EvaluateSelected(full)
		} else {
			selected, err = e.sf18BigContext.EvaluateSelected()
		}
		if err != nil {
			panic(fmt.Errorf("%w: SF18 BIG evaluate: %v", errWorkerEvaluator, err))
		}
		return sf18BigSearchScore(selected.Components, pos)
	case evaluatorBackendCounter55:
		if e.counterContext == nil {
			panic(fmt.Errorf("%w: nil Counter context", errWorkerEvaluator))
		}
		raw := e.counterContext.EvaluateRaw()
		if e.fullRefreshOracle {
			board, boardErr := counterBoardFromPosition(pos)
			if boardErr != nil {
				panic(boardErr)
			}
			fresh, err := e.model.counter55.EvaluateFullRefresh(board)
			if err != nil {
				panic(fmt.Errorf("%w: Counter fresh evaluate: %v", errWorkerEvaluator, err))
			}
			raw = fresh
		}
		return counterSearchScore(raw, pos, true)
	case evaluatorBackendRodentV11Anand:
		if e.rodentContext == nil {
			panic(fmt.Errorf("%w: nil Rodent context", errWorkerEvaluator))
		}
		static, err := e.rodentContext.EvaluateReleaseStatic()
		if e.fullRefreshOracle {
			full, fullErr := rodentPositionFromPosition(pos)
			if fullErr != nil {
				panic(fullErr)
			}
			static, err = e.model.rodentV11Anand.EvaluateReleaseStatic(full)
		}
		if err != nil {
			panic(fmt.Errorf("%w: Rodent evaluate: %v", errWorkerEvaluator, err))
		}
		return rodentSearchScore(static)
	case evaluatorBackendNGNK4:
		if e.ngnK4Context == nil {
			panic(fmt.Errorf("%w: nil NGN K4 context", errWorkerEvaluator))
		}
		raw, err := e.ngnK4Context.EvaluateRaw()
		if e.fullRefreshOracle {
			full, fullErr := ngnK4PositionFromPosition(pos)
			if fullErr != nil {
				panic(fullErr)
			}
			raw, err = e.model.ngnK4.EvaluateRaw(full)
		}
		if err != nil {
			panic(fmt.Errorf("%w: NGN K4 evaluate: %v", errWorkerEvaluator, err))
		}
		return nnueSearchScore(scaleNGNK4(raw, e.model.identity.ngnK4ScalePercent), pos.HalfMoveClock)
	case evaluatorBackendRodentV12Default:
		if e.rodentV12Context == nil {
			panic(fmt.Errorf("%w: nil Rodent V1.2 context", errWorkerEvaluator))
		}
		static, err := e.rodentV12Context.EvaluateReleaseStatic()
		if e.fullRefreshOracle {
			full, fullErr := rodentV12PositionFromPosition(pos)
			if fullErr != nil {
				panic(fullErr)
			}
			static, err = e.model.rodentV12Default.EvaluateReleaseStatic(full)
		}
		if err != nil {
			panic(fmt.Errorf("%w: Rodent V1.2 evaluate: %v", errWorkerEvaluator, err))
		}
		return rodentSearchScore(static)
	default:
		panic(fmt.Errorf("%w: unknown backend %d", errWorkerEvaluator, e.model.identity.backend))
	}
}

// LegacyUndampedSTM preserves the three alphaBeta emergency-return semantics.
// NGN-v1 deliberately performs a fresh full-model evaluation of the actual
// engine board, skips rule-50 attenuation, and clamps before int conversion.
func (e *workerEvaluator) LegacyUndampedSTM(pos *Position) int {
	if e == nil || pos == nil {
		panic(fmt.Errorf("%w: nil evaluator or position", errWorkerEvaluator))
	}
	switch e.model.identity.backend {
	case evaluatorBackendHCE:
		if e.hce == nil {
			panic(fmt.Errorf("%w: nil HCE evaluator", errWorkerEvaluator))
		}
		return e.hce.LegacyUndampedSTM(pos)
	case evaluatorBackendNGNV1:
		full, err := e.rootBuffer.position(pos)
		if err != nil {
			panic(err)
		}
		raw, err := e.model.ngnV1.Evaluate(full)
		if err != nil {
			panic(fmt.Errorf("%w: fresh evaluate: %v", errWorkerEvaluator, err))
		}
		return clampNNUEStatic(raw)
	case evaluatorBackendSF18BIGResearch:
		full, err := e.rootBuffer.position(pos)
		if err != nil {
			panic(err)
		}
		selected, err := e.model.sf18Big.EvaluateSelected(full)
		if err != nil {
			panic(fmt.Errorf("%w: SF18 BIG fresh evaluate: %v", errWorkerEvaluator, err))
		}
		return sf18BigUndampedScore(selected.Components, pos)
	case evaluatorBackendCounter55:
		board, err := counterBoardFromPosition(pos)
		if err != nil {
			panic(err)
		}
		raw, err := e.model.counter55.EvaluateFullRefresh(board)
		if err != nil {
			panic(fmt.Errorf("%w: Counter fresh evaluate: %v", errWorkerEvaluator, err))
		}
		return counterSearchScore(raw, pos, false)
	case evaluatorBackendRodentV11Anand:
		full, err := rodentPositionFromPosition(pos)
		if err != nil {
			panic(err)
		}
		static, err := e.model.rodentV11Anand.EvaluateReleaseStatic(full)
		if err != nil {
			panic(fmt.Errorf("%w: Rodent fresh evaluate: %v", errWorkerEvaluator, err))
		}
		return rodentSearchScore(static)
	case evaluatorBackendRodentV12Default:
		full, err := rodentV12PositionFromPosition(pos)
		if err != nil {
			panic(err)
		}
		static, err := e.model.rodentV12Default.EvaluateReleaseStatic(full)
		if err != nil {
			panic(fmt.Errorf("%w: Rodent V1.2 fresh evaluate: %v", errWorkerEvaluator, err))
		}
		return rodentSearchScore(static)
	case evaluatorBackendNGNK4:
		full, err := ngnK4PositionFromPosition(pos)
		if err != nil {
			panic(err)
		}
		raw, err := e.model.ngnK4.EvaluateRaw(full)
		if err != nil {
			panic(fmt.Errorf("%w: NGN K4 fresh evaluate: %v", errWorkerEvaluator, err))
		}
		return clampNNUEStatic(scaleNGNK4(raw, e.model.identity.ngnK4ScalePercent))
	default:
		panic(fmt.Errorf("%w: unknown backend %d", errWorkerEvaluator, e.model.identity.backend))
	}
}

func counterSearchScore(raw float32, pos *Position, dampRule50 bool) int {
	if pos == nil {
		panic(fmt.Errorf("%w: invalid Counter score position", errWorkerEvaluator))
	}
	return counterSearchScoreInputs(raw, pos.Turn() == White, pos.HalfMoveClock, counterNonPawnMaterial(pos), dampRule50)
}

func counterSearchScoreInputs(raw float32, whiteMove bool, halfMoveClock uint8, nonPawnMaterial int64, dampRule50 bool) int {
	if math.IsNaN(float64(raw)) || math.IsInf(float64(raw), 0) {
		panic(fmt.Errorf("%w: invalid Counter score input", errWorkerEvaluator))
	}
	var score int64
	switch {
	case raw > 15000:
		score = 15000
	case raw < -15000:
		score = -15000
	default:
		score = int64(raw)
	}
	score = score * (160 + nonPawnMaterial) / 160
	if dampRule50 {
		score = score * (200 - int64(halfMoveClock)) / 200
	}
	if !whiteMove {
		score = -score
	}
	return clampNNUEStatic(score)
}

const (
	minK4EvalScalePercent = 10
	maxK4EvalScalePercent = 400
)

// scaleNGNK4 uses the scale frozen into this worker's evaluator identity.
func scaleNGNK4(raw, percent int64) int64 {
	if percent == 100 {
		return raw
	}
	return raw * percent / 100
}

func nnueSearchScore(raw int64, halfMoveClock uint8) int {
	const budget = int64(FiftyMoveDampBudget)
	factor := budget - int64(halfMoveClock)
	// Quotient/remainder form is exactly raw*factor/budget with truncation toward
	// zero, while remaining safe for every int64 raw score.
	attenuated := (raw/budget)*factor + (raw%budget)*factor/budget
	return clampNNUEStatic(attenuated)
}

func sf18BigSearchScore(components sf18big.Components, pos *Position) int {
	return nnueSearchScore(sf18BigCentipawns(components, sf18BigMaterial(pos)), pos.HalfMoveClock)
}

func sf18BigUndampedScore(components sf18big.Components, pos *Position) int {
	return clampNNUEStatic(sf18BigCentipawns(components, sf18BigMaterial(pos)))
}

// sf18BigCentipawns reproduces Stockfish 18's zero-optimism component,
// complexity and material scaling, then maps its fixed PawnValue units onto
// NGN centipawns. Rule-50 damping remains owned by nnueSearchScore so it is
// applied exactly once at the engine boundary.
func sf18BigCentipawns(components sf18big.Components, material int64) int64 {
	const stockfishPawnValue = int64(208)
	psqt, positional := int64(components.PSQT), int64(components.Positional)
	nnue := (125*psqt + 131*positional) / 128
	complexity := psqt - positional
	if complexity < 0 {
		complexity = -complexity
	}
	nnue -= nnue * complexity / 18236
	nnue = nnue * (77871 + material) / 77871
	return nnue * 100 / stockfishPawnValue
}

func sf18BigMaterial(pos *Position) int64 {
	if pos == nil {
		panic(fmt.Errorf("%w: nil SF18 BIG material position", errWorkerEvaluator))
	}
	count := func(white, black Piece) int64 {
		return int64(bits.OnesCount64(pos.Board.GetBitboardOf(white) | pos.Board.GetBitboardOf(black)))
	}
	return 534*count(WhitePawn, BlackPawn) +
		781*count(WhiteKnight, BlackKnight) +
		825*count(WhiteBishop, BlackBishop) +
		1276*count(WhiteRook, BlackRook) +
		2538*count(WhiteQueen, BlackQueen)
}

// rodentSearchScore preserves the exact release-static score throughout NGN's
// non-mate band. The engine boundary alone reserves larger values for mate.
func rodentSearchScore(releaseStatic int) int {
	return clampNNUEStatic(int64(releaseStatic))
}

func clampNNUEStatic(raw int64) int {
	if raw > nnueStaticEvalLimit {
		return int(nnueStaticEvalLimit)
	}
	if raw < -nnueStaticEvalLimit {
		return -int(nnueStaticEvalLimit)
	}
	return int(raw)
}

func (e *workerEvaluator) observeTransition(pos *Position) {
	if e.transitionObserver != nil {
		e.transitionObserver(pos, e)
	}
}

func (e *workerEvaluator) resetNNUE(position nnue.Position) error {
	if e.context != nil && e.portableContext == nil {
		return e.context.Reset(position)
	}
	if e.portableContext != nil && e.context == nil {
		return e.portableContext.Reset(position)
	}
	return fmt.Errorf("%w: expected exactly one NNUE context", errWorkerEvaluator)
}

func (e *workerEvaluator) pushNNUEDelta(delta nnue.Delta) error {
	if e.context != nil && e.portableContext == nil {
		return e.context.PushDelta(delta)
	}
	if e.portableContext != nil && e.context == nil {
		return e.portableContext.PushDelta(delta)
	}
	return fmt.Errorf("%w: expected exactly one NNUE context", errWorkerEvaluator)
}

func (e *workerEvaluator) pushNNUENull(facts nnue.PositionFacts) error {
	if e.context != nil && e.portableContext == nil {
		return e.context.PushNullFacts(facts)
	}
	if e.portableContext != nil && e.context == nil {
		return e.portableContext.PushNullFacts(facts)
	}
	return fmt.Errorf("%w: expected exactly one NNUE context", errWorkerEvaluator)
}

func (e *workerEvaluator) popNNUE() error {
	if e.context != nil && e.portableContext == nil {
		return e.context.Pop()
	}
	if e.portableContext != nil && e.context == nil {
		return e.portableContext.Pop()
	}
	return fmt.Errorf("%w: expected exactly one NNUE context", errWorkerEvaluator)
}

func (e *workerEvaluator) evaluateNNUE() (int64, error) {
	if e.context != nil && e.portableContext == nil {
		return e.context.Evaluate()
	}
	if e.portableContext != nil && e.context == nil {
		return e.portableContext.Evaluate()
	}
	return 0, fmt.Errorf("%w: expected exactly one NNUE context", errWorkerEvaluator)
}

func (e *workerEvaluator) nnueDepth() int {
	if e.sf18BigContext != nil && e.context == nil && e.portableContext == nil &&
		e.counterContext == nil && e.rodentContext == nil && e.rodentV12Context == nil && e.ngnK4Context == nil {
		return e.sf18BigContext.Depth()
	}
	if e.context != nil && e.portableContext == nil {
		return e.context.Depth()
	}
	if e.portableContext != nil && e.context == nil && e.counterContext == nil {
		return e.portableContext.Depth()
	}
	if e.counterContext != nil && e.context == nil && e.portableContext == nil {
		return e.counterContext.Depth()
	}
	if e.rodentContext != nil && e.context == nil && e.portableContext == nil && e.counterContext == nil {
		return e.rodentContext.Depth()
	}
	if e.rodentV12Context != nil && e.context == nil && e.portableContext == nil && e.counterContext == nil && e.rodentContext == nil && e.ngnK4Context == nil {
		return e.rodentV12Context.Depth()
	}
	if e.ngnK4Context != nil && e.context == nil && e.portableContext == nil && e.counterContext == nil &&
		e.rodentContext == nil && e.rodentV12Context == nil && e.sf18BigContext == nil {
		return e.ngnK4Context.Depth()
	}
	return -1
}

// The search core has no error return. Transition contract violations indicate
// engine/context divergence and fail fast at the exact boundary rather than
// silently falling back to a refreshed or different evaluator.
func (e *workerEvaluator) mustPrepareMove(pos *Position, move Move) workerEvalMove {
	transition, err := e.PrepareMove(pos, move)
	if err != nil {
		panic(err)
	}
	return transition
}

// mustPushMadeMove restores the already advanced engine board before surfacing
// a fail-fast evaluator error. Search state and board therefore remain one
// transaction even when dynamic Counter capacity validation rejects a push.
func (e *workerEvaluator) mustPushMadeMove(
	pos *Position,
	transition workerEvalMove,
	move Move,
	undoTag PositionTag,
	undoEP Square,
	undoClock uint8,
) {
	if err := e.PushMove(pos, transition); err != nil {
		pos.UnMakeMove(move, undoTag, undoEP, undoClock)
		panic(err)
	}
}

func (e *workerEvaluator) mustPrepareNull(pos *Position) workerEvalNull {
	transition, err := e.PrepareNull(pos)
	if err != nil {
		panic(err)
	}
	return transition
}

func (e *workerEvaluator) mustPushMadeNull(pos *Position, transition workerEvalNull, undoEP Square) {
	if err := e.PushNull(pos, transition); err != nil {
		pos.UnMakeNullMove(undoEP)
		panic(err)
	}
}

func (e *workerEvaluator) mustPop() {
	if err := e.Pop(); err != nil {
		panic(err)
	}
}
