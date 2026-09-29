package engine

import "sync/atomic"

const searchNodePublishInterval uint64 = 4096

// searchNodePublisher exposes a coherent, allocation-free snapshot of one
// worker's live node counters. All data fields are atomic; sequence guards the
// pair so readers never combine different publications.
type searchNodePublisher struct {
	sequence atomic.Uint64
	nodes    atomic.Uint64
	qnodes   atomic.Uint64
}

func (p *searchNodePublisher) publish(nodes, qnodes uint64) {
	p.sequence.Add(1)
	p.nodes.Store(nodes)
	p.qnodes.Store(qnodes)
	p.sequence.Add(1)
}

func (p *searchNodePublisher) load() (uint64, uint64) {
	for {
		before := p.sequence.Load()
		if before&1 != 0 {
			continue
		}
		nodes := p.nodes.Load()
		qnodes := p.qnodes.Load()
		if before == p.sequence.Load() {
			return nodes, qnodes
		}
	}
}

func (info *SearchInfo) publishNodesIfDue(force bool) {
	if info.nodePublisher == nil {
		return
	}
	if !force && info.Nodes-info.lastPublishedNodes < searchNodePublishInterval {
		return
	}
	info.nodePublisher.publish(info.Nodes, info.QNodes)
	info.lastPublishedNodes = info.Nodes
}

// addSearchMonotonicCounters merges only additive statistics. It deliberately
// excludes result, depth, PV, repetition, root, clock, and owner fields.
func addSearchMonotonicCounters(dst, src *SearchInfo) {
	dst.Nodes += src.Nodes
	dst.FutilityPrunes += src.FutilityPrunes
	dst.NullMoveCutoffs += src.NullMoveCutoffs
	dst.LMRReductions += src.LMRReductions
	dst.NullWindowScouts += src.NullWindowScouts
	dst.LMRReSearches += src.LMRReSearches
	dst.PVReSearches += src.PVReSearches
	dst.LMPPrunes += src.LMPPrunes
	dst.ProbcutPrunes += src.ProbcutPrunes
	dst.SEEQuietPrunes += src.SEEQuietPrunes
	dst.TTProbes += src.TTProbes
	dst.TTHits += src.TTHits
	dst.TTCutoffs += src.TTCutoffs
	dst.BetaCutoffs += src.BetaCutoffs
	dst.FirstMoveCutoffs += src.FirstMoveCutoffs
	dst.QNodes += src.QNodes
	dst.QSearchTTStores += src.QSearchTTStores
	dst.CheckExtensions += src.CheckExtensions
	dst.SingularExtensions += src.SingularExtensions
	dst.RecaptureExtensions += src.RecaptureExtensions
	dst.PassedPawnExtensions += src.PassedPawnExtensions
	dst.MoveLoopNodes += src.MoveLoopNodes
	dst.TTMoveListed += src.TTMoveListed
	for i := range dst.CutIdxHist {
		dst.CutIdxHist[i] += src.CutIdxHist[i]
	}
	dst.CutTriedSum += src.CutTriedSum
	for i := range dst.BcutBand {
		dst.BcutBand[i] += src.BcutBand[i]
		dst.FmcBand[i] += src.FmcBand[i]
	}
	dst.CutByTT += src.CutByTT
	dst.CutByCapture += src.CutByCapture
	dst.CutByPromo += src.CutByPromo
	dst.CutByKiller += src.CutByKiller
	dst.CutByCounter += src.CutByCounter
	dst.CutByQuietHist += src.CutByQuietHist
	for class := range dst.CutMissByClass {
		for band := range dst.CutMissByClass[class] {
			dst.CutMissByClass[class][band] += src.CutMissByClass[class][band]
		}
	}
	dst.RFPPrunes += src.RFPPrunes
	dst.NullMoveTries += src.NullMoveTries
	dst.SEECapPrunes += src.SEECapPrunes
	dst.HistPrunes += src.HistPrunes
	dst.AllNodes += src.AllNodes
	dst.PVNodesExact += src.PVNodesExact
	dst.AllTriedSum += src.AllTriedSum
	dst.LMRPliesSum += src.LMRPliesSum
	dst.QDepthCapHits += src.QDepthCapHits
	dst.QTTProbes += src.QTTProbes
	dst.QTTHits += src.QTTHits
	dst.QTTCutoffs += src.QTTCutoffs
	dst.QStandPatCuts += src.QStandPatCuts
	dst.QDeltaPrunes += src.QDeltaPrunes
	dst.QSEEPrunes += src.QSEEPrunes
	dst.QBetaCutoffs += src.QBetaCutoffs
	dst.AspFailLows += src.AspFailLows
	dst.AspFailHighs += src.AspFailHighs
	dst.AspAbandoned += src.AspAbandoned
	dst.RootBestMoveChanges += src.RootBestMoveChanges
	dst.IIDSearches += src.IIDSearches
	dst.SingularTries += src.SingularTries
	dst.ExtBudgetClamps += src.ExtBudgetClamps
}
