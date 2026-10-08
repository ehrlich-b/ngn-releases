package engine

import (
	"errors"
	"fmt"
)

const evaluatorAdapterRevision uint32 = 2

var errWorkerEvaluator = errors.New("invalid worker evaluator state")

type evaluatorBackend uint8

const (
	evaluatorBackendHCE evaluatorBackend = iota
	evaluatorBackendNGNN1
)

type evaluatorModelIdentity struct {
	backend         evaluatorBackend
	adapterRevision uint32
	hceGeneration   uint64
	network         *NGNN1Network
}
type evaluatorModel struct{ identity evaluatorModelIdentity }

func hceEvaluatorModel(hceGeneration uint64) evaluatorModel {
	return evaluatorModel{identity: evaluatorModelIdentity{
		backend:         evaluatorBackendHCE,
		adapterRevision: evaluatorAdapterRevision,
		hceGeneration:   hceGeneration,
	}}
}

type workerEvaluator struct {
	model              evaluatorModel
	hce                *hceEvaluator
	nnue               *ngnn1State
	depth              int
	staticOracle       func(*Position) int
	transitionObserver func(*Position, *workerEvaluator)
}

func (m evaluatorModel) newWorker(hce *hceEvaluator) (*workerEvaluator, error) {
	if hce == nil {
		return nil, fmt.Errorf("%w: unsupported backend or missing HCE", errWorkerEvaluator)
	}
	e := &workerEvaluator{model: m, hce: hce}
	switch m.identity.backend {
	case evaluatorBackendHCE:
	case evaluatorBackendNGNN1:
		if !m.identity.network.valid() {
			return nil, fmt.Errorf("%w: missing NGNN1 network", errWorkerEvaluator)
		}
		e.nnue = &ngnn1State{network: m.identity.network}
	default:
		return nil, fmt.Errorf("%w: unsupported backend", errWorkerEvaluator)
	}
	return e, nil
}

func (e *workerEvaluator) Identity() evaluatorModelIdentity {
	if e == nil {
		return evaluatorModelIdentity{}
	}
	return e.model.identity
}

func (e *workerEvaluator) Reset(pos *Position) error {
	if e == nil || pos == nil || e.hce == nil {
		return fmt.Errorf("%w: missing evaluator or root", errWorkerEvaluator)
	}
	e.depth = 0
	if e.nnue != nil {
		e.nnue.reset(pos)
	}
	return nil
}

type workerEvalMove struct{ delta ngnn1Delta }
type workerEvalNull struct{}

func (e *workerEvaluator) PrepareMove(pos *Position, move Move) (workerEvalMove, error) {
	if e == nil {
		return workerEvalMove{}, errWorkerEvaluator
	}
	if e.nnue != nil {
		if pos == nil || !validPiece(move.MovingPiece()) {
			return workerEvalMove{}, errWorkerEvaluator
		}
		delta := ngnn1MoveDelta(move)
		if e.nnue.network.version == 3 {
			delta = e.nnue.network.kingMoveDelta(pos, move, delta)
		}
		return workerEvalMove{delta: delta}, nil
	}
	return workerEvalMove{}, nil
}
func (e *workerEvaluator) PushMove(pos *Position, transition workerEvalMove) error {
	if e == nil {
		return errWorkerEvaluator
	}
	e.depth++
	if e.nnue != nil {
		e.nnue.pushPosition(e.depth, transition.delta, pos)
	}
	e.observeTransition(pos)
	return nil
}
func (e *workerEvaluator) PrepareNull(pos *Position) (workerEvalNull, error) {
	if e == nil {
		return workerEvalNull{}, errWorkerEvaluator
	}
	return workerEvalNull{}, nil
}
func (e *workerEvaluator) PushNull(pos *Position, transition workerEvalNull) error {
	if e == nil {
		return errWorkerEvaluator
	}
	e.depth++
	if e.nnue != nil {
		e.nnue.push(e.depth, ngnn1Delta{})
	}
	e.observeTransition(pos)
	return nil
}
func (e *workerEvaluator) Pop() error {
	if e == nil || e.depth == 0 {
		return fmt.Errorf("%w: unbalanced evaluator pop", errWorkerEvaluator)
	}
	e.depth--
	return nil
}
func (e *workerEvaluator) SearchSTM(pos *Position) int {
	if e == nil || pos == nil || e.hce == nil {
		panic(errWorkerEvaluator)
	}
	if e.staticOracle != nil {
		return e.staticOracle(pos)
	}
	if e.nnue != nil {
		n := e.nnue.network
		return n.evaluateBucket(e.nnue.at(e.depth), pos.Turn(), n.bucket(pos))
	}
	return e.hce.SearchSTM(pos)
}
func (e *workerEvaluator) LegacyUndampedSTM(pos *Position) int {
	if e == nil || pos == nil || e.hce == nil {
		panic(errWorkerEvaluator)
	}
	if e.staticOracle != nil {
		return e.staticOracle(pos)
	}
	if e.nnue != nil {
		n := e.nnue.network
		return n.evaluateBucket(e.nnue.at(e.depth), pos.Turn(), n.bucket(pos))
	}
	return e.hce.LegacyUndampedSTM(pos)
}
func (e *workerEvaluator) evaluationDepth() int {
	if e == nil {
		return -1
	}
	return e.depth
}

func (e *workerEvaluator) observeTransition(pos *Position) {
	if e.transitionObserver != nil {
		e.transitionObserver(pos, e)
	}
}

func (e *workerEvaluator) mustPrepareMove(pos *Position, move Move) workerEvalMove {
	transition, err := e.PrepareMove(pos, move)
	if err != nil {
		panic(err)
	}
	return transition
}

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
