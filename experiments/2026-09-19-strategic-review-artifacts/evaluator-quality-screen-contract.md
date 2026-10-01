# What a richer-evaluator quality screen would actually establish

Source-only review of `f75b578`, 2026-09-19. No engine run, label generation,
build, or test was performed. This is a diagnostic contract, not a replacement
strength gate.

## The current ACPL command is useful machinery, not a turnkey NNUE comparison

[scripts/acpl.sh](../../scripts/acpl.sh), lines 22–27, rebuilds the oracle and
compares executable paths against `acpl_corpus.labels` at 200,000 nodes per
position. It measures **the moves selected by a searched engine**, not static
network prediction error or the accuracy of its reported centipawns.

[cmd/oracle/pipeline.go](../../cmd/oracle/pipeline.go), lines 307–327, starts
each executable, issues `ucinewgame` before each FEN, searches it, and averages
the labeled loss of its chosen move. The node budget is shared accounting when
both sides use the same NGN search implementation; this removes the immediate
wall-clock penalty of a scalar evaluator. It does not remove evaluator/search
interactions, represent warm game histories, or measure equal-clock strength.

Three source-proved limits matter before using it to reject V1.2:

1. **The requested evaluators are not selected.** The oracle flags accept new
   and base executable paths but no per-side arguments or UCI option maps
   ([cmd/oracle/main.go](../../cmd/oracle/main.go), lines 175–202). The subprocess helper
   executes the path without arguments and sends no `setoption` commands
   ([internal/uci/uci.go](../../internal/uci/uci.go), lines 121–158). Ordinary
   NGN defaults to HCE unless its explicit startup flags select another backend
   ([main.go](../../main.go), lines 21–36). Thus two ordinary current NGN
   executables do not become Anand-versus-V1.2 merely by passing their paths.
   A separately verified startup wrapper could configure them; this review
   does not claim that all historical runs lacked such configuration. The
   helper also does not disable OwnBook, while NGN starts with it enabled
   ([engine/uci.go](../../engine/uci.go), lines 201–226).

2. **Unlisted moves are censored, not evaluated.** `acplLoss` assigns every move
   absent from the label list the score of the last listed move, then clips
   loss to `[0, cap]` ([pipeline.go](../../cmd/oracle/pipeline.go), lines
   262–281). The default label settings are MultiPV 8, depth 16, and the ACPL
   cap is 1,000 cp ([cmd/oracle/main.go](../../cmd/oracle/main.go), lines 192–199); those
   defaults do not prove how the committed artifact was produced. Two omitted
   choices receive the same score even if one is much worse. The difference
   of two individually underestimated losses is not a bound on their true
   difference. Even the individual lower-bound interpretation assumes the
   finite-search reference ranking is reliable. Missing/invalid bestmoves are
   passed to this same lookup without a legality/error rejection at the ACPL
   call site (lines 315–318); they must not become ordinary scored choices.

3. **The label file does not establish reference validity by itself.** The
   writer emits only FEN and `move:score` pairs, without reference binary/model
   identity or per-score depth/bound metadata (`pipeline.go`, lines 208–214).
   The label parser retains the last observed score by MultiPV rank without
   checking a common completed depth or bound flags (lines 129–174). These are
   limitations of what the tool verifies, not evidence that existing labels
   are wrong. The shell comment calling them permanent “ground truth” is
   stronger than the implementation establishes.

## Minimum valid same-NGN comparison

- Pin the same NGN source/search configuration; vary only the named evaluator
  and its exact model/score adapter. Record binary/model hashes and verify the
  active backend, not just successful startup. Fix one thread, hash, book and
  tablebase policy, and reset policy. A/A repeatability is a preflight, not a
  strength result.
- Freeze a representative held-out position set and a node budget before
  examining candidate outcomes. Retain per-position moves, actual nodes and
  operational failures. Fixed nodes help isolate decision quality from scalar
  execution cost; retain that narrow interpretation.
- Verify reference provenance and label completeness for **both chosen moves**.
  If a move is absent, explicitly evaluate it under the same reference policy
  or mark the pair censored/unknown. Report missing-move rates by side; do not
  silently drop a candidate-dependent subset or substitute eighth place as an
  exact value. Reject operational failures separately from chess mistakes.
- Report paired position-level changes and their uncertainty, with correlated
  positions from one source game grouped appropriately. Inspect large
  disagreements and finite-reference instability before treating the mean as
  evidence against the model. No new universal ACPL threshold is justified.

## Triage versus stopping

A properly configured screen can flag a possible gross regression and identify
concrete positions worth investigating before substantial kernel work. It is
not an evaluator-rejection gate: [CLAUDE.md](../../CLAUDE.md), line 52, explicitly
requires real-clock games for changes to evaluation values and permits proxy
rejection only for pure ordering/EBF mechanisms. A negative same-NGN screen can
guide that investigation, but does not itself authorize shelving V1.2. Wrong
evaluator selection, censored choices, or stale/unverified reference scores can
also dominate the present command's result.

Passing the screen does not establish an Elo gain. Failing a small or censored
screen does not close the architecture. Final adoption still requires a
competitive implementation and paired equal-clock games; a clear valid loss
there can stop the candidate regardless of its donor release claims.

## Addendum: strongest tested configuration is not the default product

There is a documented adoption/configuration boundary, not just a possible
search-quality gap. [TODO.md](../../TODO.md), lines 27–32, names optimized Rodent
as the stronger tested one-thread configuration while explicitly retaining the
HCE default and installed classical binary. Lines 188–193 likewise make V1.2
opt-in. [README.md](../../README.md), lines 174–182, instructs `make build-release`
then `./build/ngn`, correctly describing that launch as HCE; its neural examples
and option list at lines 198–217 still omit both Rodent backends. The
[September 12 audit](../2026-09-12-release-state-audit.md) records an installed
WSL classical `53e4d1b` binary. That was only historical evidence during this
initial source-only pass. A subsequent root
[read-only hash check](installed-binary-hashes.md) confirmed the three recorded
WSL/Windows installation paths still match the classical release. GUI selection
and alternate launchers remain unverified.

There is a separate build distinction: [Makefile](../../Makefile), line 36,
does not request `GOAMD64=v3` for `build-release`; the Rodent output/update AVX2
implementations require the compile-time `amd64 && amd64.v3` constraints
([output](../../rodenteval/output_dot_amd64_v3.go), line 1;
[updates](../../rodenteval/apply_updates_amd64_v3.go), line 1). Thus selecting the
right network does not ensure the measured optimized implementation when the
build environment does not enable that target. Windows targets at Makefile
lines 24 and 43 do request v3 and can copy the executable for Arena, but do not
select or bundle Rodent. Source defaults, documented launch, research candidate,
and the user's actual runtime must be kept distinct. This gap could explain a
perception based on an older/default configuration; its occurrence and Elo cost
in the user's current playing setup are unverified. The subsequent check read
installed binary hashes only; no installed engine was executed, replaced or
reconfigured during this review.
