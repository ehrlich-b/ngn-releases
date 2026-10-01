# Original HCE release candidate — 2026-10-01

This stage replaces the remaining borrowed classical evaluator with an original,
small hand-designed HCE and fresh coordinate-based sliding attacks. The fresh
personal native Luna max context received only the typed behavior contract and
previously independently generated piece/square/move/board helpers. No prior HCE,
magic-number implementation, tables, coefficients, pretrained model, labels,
training corpus or hidden baseline oracle was supplied. This documents the
process and is not a formal legal clean-room certification.

Native thread `01a0f923-a07e-7471-a7cb-31297a3d774f`, 20:24:17–20:38:38 UTC.
Personal ChatGPT Pro identity `ehrlich.bryan@gmail.com` and included weekly
allowance 77% verified before the model turn; no quota/payment denial. Requested
6 Luna was not in the complete personal native model catalog; the supported
actual model was `gpt-5.6-luna`, reasoning max. The worker completed at
122,693/300,000 goal tokens. No paid fallback, purchase, reset or work-account
credits were authorized or initiated.

The new material/PST values are analytic functions of board coordinates with
hand-chosen coefficients; no data tuning or table copying was used. Evaluation
is bounded and White-relative, symmetric under color/rank reflection and
allocation-free. The host's NGN-owned public evaluation/cache/session code now
calls this core; side-to-move conversion and rule-50 attenuation remain outside
the board-pure cache. Full old HCE, PeSTO/legacy square arrays, explicitly ported
Counter draw-factor routine, inherited magic constants, generic/reference NNUE
and Counter/Stockfish evaluator packages and adapters are removed additively.
Old model/tuning APIs and their tools are retired. No external evaluator/model
can be loaded or selected, including through the library-created UCI path.

Existing generic legality, make/unmake/null/castling/EP, repetition, key,
accumulator, transposition-table ownership, cancellation and search-worker
controls remain. Tests exclusively for retired evaluator/tuning features are
retired with those APIs; historical tests, failed integration checks and source
are retained in Git or this experiment. ProbCut/singular stack regressions use
a controller-owned constant-score oracle, replacing their old zero-NNUE fixture,
with evaluator-depth observation independent of the new HCE. Cold/warm cached
score policy, concurrent cache isolation and warm per-engine state controls
remain. The old fixed-score evaluator baseline is retired; the generic repeated
score-consistency check remains. No correctness oracle was relaxed to fit a
wrong move, key, castling, repetition or restoration output.

Generation tests/race/vet/gofmt passed. Independent controller replay against
parent `6ecc2699be13e75e651e03fd06c5c0e35d266716` produced identical outputs:

- Position state: 1,496,917 bytes, SHA-256
  `2ebb10d3fa5aeae511eca23d29e0baf13097e5ca8897e1a64c93b2bd29eefa9f`.
- Hash/EP/Polyglot restoration: 229,574 bytes, SHA-256
  `c21d87035a2b7023b323067ac3744c5339302ab7d991f816301cbfc87f242b64`.
- Slider replay, all origins/single blockers, every relevant interior blocker
  subset and deterministic random occupancies: 4,105,216 bytes, SHA-256
  `69eb04ee8a29c66bf167e532de716fb9939f13dd49fd65ad20c2ec4e30866d6a`.

The final retained checks.json must show all required short engine/all tests,
both race variants, vet, diff and compiled dependency checks passing before
commit. Package verification follows this exact head. No new strength games,
fixed-score/node identity, Elo range or 2800 floor is claimed for the new core.
The user explicitly prioritized an independent release and deferred Elo work.

The first large integration transport timed out without changing the candidate;
a compressed retry succeeded. Compile failures exposed removed-feature host
references (pawn geometry/UCI helper/shared test helper and old tuning corpus).
These were resolved with small host integration changes or explicit retirement
of unsupported feature tooling. Initial failed logs are retained, not rewritten.
An empty-file-list gofmt controller call was corrected before resumed validation.

All model/test/build work ran on the personal WSL host at the coordinated shared
CPUs 0/2, aggregate NGN CPUQuota=50%, Nice=10, MemoryMax=4 GiB and serial Go. The
existing Zahak/GoChess strength owner, protected benchmark CPUs, shared daemons,
default branches and published historical artifacts remain unchanged.

All seven identified Zahak-derived core components were replaced in preceding
stages. Historical GPL/MIT notices stay visible. Canonical Polyglot format data
remains from the precisely pinned Disservin MIT source with full permission text;
this is disclosed interoperability data, not an NNUE/evaluation implementation.
The engine uses published chess techniques and the Go standard library; the
release does not claim that every algorithm or constant was invented by NGN.
This stage removes the identified direct runtime source/data adaptations. It is
not a blanket legal certification or an assertion about the correctness of old
public artifacts. No public release, tag, merge or CCRL resubmission occurs here.
