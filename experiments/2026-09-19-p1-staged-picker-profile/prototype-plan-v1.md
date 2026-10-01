# P1-R1 reference-style staged picker: frozen prototype contract

This contract was frozen after P1 passed its prospective profile gate and
before changing production search. It authorizes one bounded candidate. It does
not authorize reviving the June TT-first/global-fallback implementation.

## Why this candidate is different

The rejected prototype recorded at `6bf83e9` tried the TT move, then generated
and globally scored every sibling. When the TT move failed to cut, its subtree
had already mutated the global history tables, producing severe ordering drift
and fixed-depth tree inflation.

P1-R1 instead uses an explicit move-picker state machine in the ordinary
interior `alphaBetaPV` loop:

1. emit a structurally pseudo-legal TT move once;
2. generate and score tactical moves, with non-capture promotions and
   non-losing captures ahead of ordinary quiets;
3. generate and score ordinary quiets/refutations only if reached; and
4. emit losing captures last.

Relative order within a stage uses the existing score functions and stable
selection step. The picker filters the emitted TT move from every later stage.
Root search, ProbCut and quiescence remain unchanged. No new pruning margin,
history formula, evaluator behavior, clock policy or SMP behavior is bundled.
History reads at deferred stages are intentionally live, as in a conventional
stage machine; P1-R1 is therefore a search candidate requiring games.

The first implementation may filter the existing full pseudo-legal generator
to form its quiet stage. A new quiet-only generator is out of scope until P1-R1
passes. This caps correctness risk while still testing the decisive TT/tactical
staging mechanism.

## Required correctness coverage

- Every generated pseudo-legal move is emitted exactly once with no TT,
  ordinary TT, stale/collision TT, capture TT, promotion, castling and en
  passant fixtures.
- A valid TT move is first; an invalid TT move is ignored safely.
- Queen promotions precede non-losing captures; non-losing captures precede
  ordinary quiets; quiets precede losing captures. Existing score order is
  stable inside each band.
- Search preserves legal best moves, root restoration, evaluator depth, stop
  propagation, terminal mate/stalemate handling, singular exclusion, history
  accounting and race safety.
- Full short tests and the engine race suite pass on WSL.

## Mechanism and cost gates

Compare one-thread base and candidate builds from the same toolchain/model.

1. `searchsnapshot` on the accepted manifest supplies cold and warm fixed-depth
   trajectories. Reject before games if the geometric-mean final-node ratio is
   above 1.05, any fixture/pass ratio is above 1.15, any best move becomes
   illegal, or any terminal/root invariant changes. This is the predeclared
   falsifier for the historical history-order failure; exact node identity is
   not expected.
2. Run ten interleaved blocks of the six-position, 400,000-node V1.2 benchmark.
   Admit games only if every search completes and the paired aggregate median
   candidate/base time ratio is at most 0.97. Report allocations and stage
   counters; do not convert NPS directly to Elo.
3. Development cost is capped at this architecture plus one narrow correctness
   repair. Do not try a second picker architecture or add a quiet-only generator
   if either gate fails.

## Game gate

Only a candidate passing both mechanism gates may enter the existing supervised
same-model/equal-clock harness against the accepted base.

- Pilot: 64 color-reversed opening pairs (128 games), fixed before scores.
- Reject if the paired-bootstrap 95% upper bound is below zero, or if the point
  estimate is non-positive with no independent mechanism advantage beyond the
  frozen gates.
- Otherwise run one 200-pair confirmation. Accept only if its paired-bootstrap
  95% lower bound is above zero and all legal, process, telemetry and supervisor
  gates pass.

No result from this lane is an absolute 3300 claim, and its Elo must not be
arithmetically added to earlier internal estimates.

