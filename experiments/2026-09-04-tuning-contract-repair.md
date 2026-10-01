# Restore the tuning model and data-filter contract

The [classical review](2026-09-04-classical-review-findings.md) reproduced a
missing gradient feature and two defective quiet-data classifications. This
repair changes training machinery only. It does not install new evaluation
weights or modify playing search. No Elo gain is claimed.

## Changes and evidence

- Include backward-pawn MG/EG penalties in the trace, float model, derivative,
  integer reconstruction and named weight list. The full vector has 798 entries,
  including 20 scalar coordinates. Existing names remain valid; the two new
  names are `backwardPawnPenalty` and `backwardPawnPenaltyEG`. The CLI exports
  named weights; no positional checkpoint loader was found.
- Replace production qsearch in `IsQuietPosition` with a local bounded capture
  search using the same undamped static model as training. It does not consult
  TT, correction histories, stop requests or the game repetition map. It searches
  legal evasions after checking captures, rejects legal quiet promotions at any
  visited node, and conservatively rejects unresolved six-ply horizons. The caller
  position is copied, with balanced make/unmake inside the local search.
- Include the original vector in gradient checkpoint selection. Previously
  `bestMonitor = +Inf` made the first update win unconditionally. A regression
  with conflicting training/validation labels and an initially perfect validation
  fit produced loss **0 -> 0.0286936**, yet changed pawn material **82 -> 102**.
  The repaired tuner retains the original vector when all updates are worse.

Terra implemented the trace/filter repair in an isolated checkout; root reviewed
it, requested conservative horizon handling and stronger trace/derivative tests,
then integrated it. Root reproduced and fixed the checkpoint-selection defect.

## Validation

- Deterministic 16 legal random walks, 100 positions each: trace mismatches
  **542/1600 -> 0/1600**. Frozen traces also reproduce live integer evaluation
  after nonzero perturbations to every parameter, without rebuilding the traces.
- Numerical finite differences directly check both backward-pawn coordinates
  on a mixed-phase position with a nonzero feature count.
- Quiet-filter cases cover both promotion colors, blocked and pinned promotions,
  hanging material, check, caller-state preservation, clocks 0/20/99, populated
  TT/correction history, a requested stop, repetition history and an unresolved
  six-ply exchange. Original review probes now pass unchanged.
- Full short suite and full short/race suite pass; `go vet ./...` passes.
- Playing node/score/PV/bestmove identity against the frozen release was checked
  on Kiwipete d12 (295507 nodes), middlegame d12 (112109), and rook ending d16
  (858052). The later checkpoint fix changes only the offline optimizer.
- Local random-walk filter benchmark: **4.94–5.08 microseconds/sample** on Apple
  M4. Conservative rejection shortens complex capture trees; this is not a
  measurement of acceptance rate or a comparison with the old filter's throughput.

Raw verification: `output/recovery-2026-09-04/tuning-repair-*`,
`tuning-initial-vector-before.txt`. Portable review probes are
`python3 experiments/2026-09-04-review-probes.py`. Generated overlay sources now
use `.go.txt` suffixes so `go test ./...` does not try compiling them as a separate
package; a prior local generation of `.go` files exposed and motivated that fix.

## Limits and next use

The quiet predicate is a conservative bounded capture-stability filter, not a
proof that a position has no tactic. Measure its acceptance distribution on a
real corpus before regenerating millions of samples. The gradient remains a
floating-point relaxation of integer evaluation; the existing rounding-fidelity
test still applies. Mobility's two advertised scalar coordinates remain inactive;
mobility tables, threats and several king-danger inputs remain fixed. This patch
does not pretend that an accurate trace makes every useful feature trainable.

The first defects entered June 21/22 and cannot explain earlier failed tunes.
Checkpoint selection fails only when the best attempted update is worse than the
initial vector; its historical incidence was not measured. Better validation
loss is still insufficient for a playing-strength claim. Use a game-disjoint
validation split and untouched confirmation openings for a future fitted candidate.
