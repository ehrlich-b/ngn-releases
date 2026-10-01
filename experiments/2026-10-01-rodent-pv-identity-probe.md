# Rodent V1.2 promotion-PV identity probe — 2026-10-01

## Question and answer

**Can the exact historical Rodent testers binary report a suffix-less pawn
promotion at the frozen root? Yes.** The immutable historical trace records the
exact engine reporting `g2g1` at PV index 9 at both depth 17 and depth 18. The
trace SHA-256 matches the retained `fastchess.log.zst`; the match-stage config
and final-file manifest bind the Rodent role to binary SHA-256
`9cfb8195207ee5695c1973a89664ab73b34b5bcbc10ad3bc0f0afe28b9713cbc`.
The independently pinned chess oracle establishes that the preceding nine
plies are legal, that the black pawn is then on g2, and that UCI requires one
of `g2g1q`, `g2g1r`, `g2g1b`, or `g2g1n`. `g2g1` by itself is not legal UCI.

This is **historically recorded exact-artifact runtime evidence**, not a newly
reproduced line. Nine newly run, predeclared searches of the same binary did
not emit the defect. That non-reproduction does not erase the artifact-bound
witness and does not identify the state-dependent trigger.

The pinned official-tag source proves an admission/rendering path consistent
with this output:

1. A move packs from/to/type in bits 0–5/6–11/12–15; promotion types are 4–7
   and `NORMAL` is 0 (`tables.go:81-90,291-307`).
2. Normal generation emits all four promotion-flagged moves and excludes
   last-rank pawn moves from its ordinary move sets (`gen.go:59-104,132-175`).
3. The stale-move validator attempts to reject an unflagged promotion, but its
   zero-based rank tests are off by one: it checks rank index 7 for White and
   rank index 0 for Black. A black pawn promoting from g2 is on rank index 1,
   so a `NORMAL` integer with from g2/to g1 passes the straight-empty-push test
   (`legal.go:125-150`).
4. TT and killer move-picker stages admit stored moves through this validator
   (`movepick.go:86-90,118-132`). TT packing preserves the entire 16-bit move
   and probing returns it as a move hint (`trans.go:94-109,226-245`); it does
   not strip a valid promotion flag.
5. `buildPV` copies the move integer unchanged, while `writeMove` appends a
   suffix only when the promotion bit is present (`uci.go:714-747,767-780`). A
   wrongly admitted `NORMAL g2g1` therefore renders as the observed four-byte
   token.

This proves the official source's admission/rendering path. It does **not**
prove whether TT, a killer slot, or another stored-move route supplied the
historical integer. It also does not prove that these exact sources produced
the released executable: the binary contains Go 1.26.1 build metadata but no
VCS revision. Official tag commit
`b53ffaf670590932957cb63b7b6d871f6f33b7d8`, tree
`deefc1307d5f32dafb42da6e1d7afbd80cf95307`, and eight relevant files have
independently matching Git blob identities. That strong association is still
not a compiled-source provenance proof.

## Frozen witness and ambiguity

Root:

```text
8/7R/4K3/8/1p6/1P4p1/5k2/8 w - - 4 79
```

Both recorded PVs reach this position after their common legal prefix:

```text
h7f7 f2e1 e6d5 g3g2 f7g7 e1f2 d5c4 f2f3 c4b5
8/6R1/8/1K6/1p6/1P3k2/6p1/8 b - - 5 83
```

The next recorded token is `g2g1`. The depth-17 remainder is legal under rook
or knight promotion; the depth-18 remainder is legal under all four promotion
choices. The recorded actual `bestmove h7f7` is legal. Therefore the intended
piece is not recoverable and was not guessed. The historical failed trial
remains unscored.

The frozen source records are byte-for-byte unchanged. Their hashes and the
historical trace/binary chain are in
[`artifact-identity.json`](2026-10-01-rodent-pv-identity-probe/evidence/artifact-identity.json).

## Bounded direct runtime probe

The [plan](2026-10-01-rodent-pv-identity-probe/predeclared-plan.md) was written
before executing the binary. The binary and embedded default net were verified
before every probe run. It was launched directly—not through `role_exec.py`—in
private empty working directories. Each process acknowledged `Threads=1`,
`Hash=128`, `UCI_Chess960=false`, `UCI_LimitStrength=false`, and
`UCI_Elo=3000`. `OwnBook` was not advertised; startup attempted and failed to
open `books/empty.bin`, and every private working directory remained empty.

Results from [`probe-result.json`](2026-10-01-rodent-pv-identity-probe/evidence/attempt2/probe-result.json):

- 9/9 planned searches completed; maximum reported nodes in any search was
  248,974, maximum depth was 18.
- Reported search-process CPU deltas were 0.34 seconds and aggregate measured search
  wall time was 0.803 seconds, below the 120/180-second bounds.
- Fresh root depth 17/18, exact full-history depth 17/18, and the conditional
  warm-TT depth 18 then depth 17 repeat emitted only suffixed promotions.
- All 167 newly reported PVs and every new best move were legal under the
  verified python-chess 1.11.2 oracle. No new suffix-less witness occurred.
- Forced-promotion positive control
  `k7/7R/8/4B3/8/8/6p1/K7 b - - 0 1`, whose only legal moves are the four
  promotions, reported `bestmove g2g1q` and suffixed PVs.
- Ordinary start-position negative control reported a legal ordinary PV and
  no false promotion classification.
- At the frozen promotion node the binary legally chose king move `f3f2`;
  the oracle independently retained all four legal promotion alternatives.

The first wrapper had incorrectly predeclared that the promotion-node *best
move* must be a promotion. It exited 1 after the first seven searches even
though `f3f2` is legal. No engine output was discarded or rerun. The assertion
was fixed, the first seven receipts were reused byte-for-byte, and only the two
already-declared conditional warm searches ran. The exact deviation is
[retained](2026-10-01-rodent-pv-identity-probe/predeclared-plan-deviation.md).

## Independent and source-derived controls

- Pinned oracle: python-chess 1.11.2 at SHA-256
  `1fde6a8e932508d14b31f6a584497b49c2969fc3c7cdc675347f819f0d667a5b`.
- Classifier regressions passed: two historical positives, one synthetic
  missing-suffix positive, four legal-promotion negatives, eight completion
  checks, and 167 newly emitted legal-PV negatives. See
  [`classifier-regression.stdout`](2026-10-01-rodent-pv-identity-probe/evidence/classifier-regression.stdout).
- A dependency-free, explicitly source-derived Go model shows the pinned
  validator accepts integer 398 as `NORMAL g2g1`, the renderer emits `g2g1`,
  the four flagged encodings emit `g2g1n/b/r/q`, and TT packing preserves all
  bits. It is a model of official source, **not exact-binary evidence**. See
  [`source-model.stdout`](2026-10-01-rodent-pv-identity-probe/evidence/source-model.stdout).
- The model's first execution produced the expected JSON but exited 1 only
  because the default Go cache was outside the writable task root. The retained
  rerun used a private task cache and exited 0; no engine search was repeated.
- The classifier and probe passed Python syntax checks without bytecode. The Go
  model passed `gofmt -d` and the installed Go 1.25.5 with
  `GOTOOLCHAIN=local`, `GOMAXPROCS=1`, and `GOFLAGS='-mod=readonly -p=1'`.

Raw UCI input/output/stderr, exits, timestamps, process observations, exact
commands, source identities, and a SHA-256 manifest are retained under
[`evidence/`](2026-10-01-rodent-pv-identity-probe/evidence/).

## Controls, limits, and gate

The probe inherited CPU list `0,2`, cgroup `cpu.max=50000 100000`, nice 10,
`memory.max=4294967296`, `GOMAXPROCS=1`, and serial Go. It did not run games,
benchmarks, strength measurements, random positions, Mac chess compute, or any
owner launcher; it made no engine, harness, adapter, production, match, daemon,
or resource change.

The historical exact-binary witness and source-admissible mechanism are now
documented, but the **native benchmark/compatibility gate remains closed**.
There is no basis to fill in a suffix, rescore the failed trial, or authorize a
compatibility exception. The smallest bounded next gate, if pursued, is to
obtain precise compiled-source provenance or a controlled instrumented build
of the pinned official source that captures the origin and type bits of the
stale move at this position, with positive/negative promotion controls. That
must remain separate from the immutable release binary and from any scored
match.

## Independent operator review

The controller rechecked the binary, historical compressed trace, all eight
source blob identities and the frozen independent chess oracle, then reran
the classifier, syntax checks and source model under the same coordinated
CPU0/2, 50%, nice10, 4GiB and serial Go caps. The source model now also checks
all four promotion encodings and suffixes after TT packing/unpacking, in
addition to the normal-move witness. All focused review checks passed. No
engine search was repeated and no production source changed.

The original native report/model bytes and manifest are preserved as gzip and
`native-input-encodings.json`; GPL source attribution is retained with
`SOURCE-LICENSE-GPL-3.0`. Probe timing reports search deltas sampled immediately
after the go command, excludes startup and is not full worker CPU accounting
or performance evidence. The native task goal reached its configured cumulative
token limit during finalization; the turn completed and its proof was retained.
That task metadata limit was not a provider quota/payment denial. No credit
fallback, purchase, allowance reset or work account was used.

The native Git metadata mount remained read-only. The authorized controller
committed only this scoped task on its isolated branch; the external completion
receipt records the final commit and bundle. Owner/default branches and live
matches remain under the original owner's control.
