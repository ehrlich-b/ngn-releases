# E0 — Rodent V1.2 versus optimized Anand

Status: **COMPLETE — ADOPT Rodent V1.2 as the competitive evaluator.**

This is the single bounded strength gate admitted after the V1–V3 exact speed
program and G0b harness admission. It asks whether the release-static Rodent
V1.2 network is stronger than the accepted optimized Rodent V1.1 Anand network
when both receive the same current search, executable, clock, hash and one-worker
configuration.

## Result

Fresh Anand/Anand A/A passed: 23W/58D/19L from engine A's perspective,
pentanomial `[1,8,28,12,1]`, paired +13.90 Elo with the predeclared
paired-bootstrap 95% interval `[−20.87,+48.96]`. The interval includes zero, so
the candidate phase was admitted.

Rodent V1.2 then scored **145W/188D/67L**, 239/400 (59.75%), against optimized
Anand. Its pentanomial was `[3,35,64,77,21]`; the predeclared whole-opening-pair
bootstrap measured **+68.63 Elo `[+45.42,+92.46]`**. The strictly positive lower
bound satisfies the frozen rule, so the decision is **ADOPT**.

All 400 games, 200 reversed-color pairs and 61,890 plies passed the independent
legal/terminal audit. Operational audit passed 2,082,664 trace lines, 800 exact
option refreshes, eight clean engine exits, strict startup-log counts, zero
warnings and zero probable embedded-book signatures. The process witness made
481,666 observations over four exact instances per role with no violations;
both supervisors completed normally and left no survivors.

The hopper remained live. Candidate telemetry covered 660 five-second samples,
with 0.0418 mean and 0.6137 peak non-owned cores; even the old counterfactual
exclusivity gate did not trigger. Candidate match supervision took 3,299.1
seconds; A/A took 818.8 seconds. Exact structured results and terminal hashes are
in [`result.json`](result.json), and the immutable WSL run is retained at
`/home/ehrli/e0-v12-anand-strength-gate-20260919/run-001`.

This is a same-search, same-clock model result—not an absolute rating, a 3300
claim, or Elo that can be added to another opponent's published rating. It
selects V1.2 as the next optimization/search target. Changing the global
default, packaging the external weights, or installing a binary remains a
separate D0 release-authority decision.

## Frozen protocol

- One exact Linux/amd64 GOAMD64=v3 binary for both roles, built from `117be11`
  with Go 1.25.5, `CGO_ENABLED=0` and `-trimpath`.
- A/A100 uses Anand in both roles. Only if its paired-bootstrap 95% interval
  includes zero does the candidate phase start.
- Candidate400 uses V1.2 as audited engine A and Anand as engine B over 200
  sequential reversed-color opening pairs.
- Threads1, `GOMAXPROCS=1`, Hash128, OwnBook=false and Move Overhead100 for every
  role; EvalFile is staged before EvalBackend with a ready barrier after each
  option.
- 10+0.1, concurrency four, physical cores 0/2/4/6, no adjudication, 100,000
  whole-pair bootstrap resamples with seed 2026091201.
- ADOPT only if every operational/legal/process audit passes and the candidate
  paired-bootstrap lower 95% bound is strictly positive. Otherwise SHELVE at
  400 games. No score-based extension or rerun.

The host remains shared with the Lean hopper. Prelaunch and five-second
continuous background-CPU telemetry are retained, including the old exclusivity
threshold as counterfactual metadata, but background load alone is not a
rejection condition. The result is therefore shared-host observational evidence,
not an absolute rating claim.

## Frozen identities

The held contract is [`manifest-held-v1.json`](manifest-held-v1.json); the
reviewed executable contract is
[`manifest-release-v1.json`](manifest-release-v1.json). Their only difference is
the `release_state` field, as recorded in
[`static-review-v1.json`](static-review-v1.json). The
shared binary SHA-256 is
`dcde045792bba6b6d4f3fc83c9153763b94ff1ba1cba1d9df564a1ad69eb15aa`.
It is bit-identical to the binary used in the successful G0b configured
preflights. The Anand and V1.2 model hashes are respectively
`5f7480b56538e9e64ee02fb5b4842bf0394ee44b707dae3a63ec7f902abd0afb`
and `c35a1abc1b8c1cb1d5f4221454d494c1a6da1ed9088fd51ab27038bfa74b5053`.

The driver is separately hash-bound. It freezes the accepted role launcher,
process witness, supervisor, corrected `(trace thread, role name)` trace state,
strict interleaved startup-record counts, exact configured preflight, the
established full legal/terminal replay auditor, fastchess, Stockfish, openings
and opening-prefix inventory. The generic HCE/NGN-v1 candidate manifest was not
broadened.

## Pre-release review

The exact WSL checkout passed all four focused driver tests, Python compilation,
and all 29 candidate-match tests. The held check hash-checked all 13 frozen
sources plus the shared engine while confirming that `run-001` was absent. The
held-to-release diff changes only `release_state`. This was a root static review,
not an independent-agent review.

Immediately before launch, the checks confirmed that the hopper controller was
alive, the run root was absent, and no E0 engine or fastchess process existed.
The driver subsequently completed both phases and wrote its atomic decision,
inventory and terminal chain.
