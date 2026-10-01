# Sol implementation handoff: structural strength, not another polish loop

Prepared September 19, 2026 at the user's request. **This is a plan, not a
record of implemented fixes, a released match manifest, or deployment approval.**
The user will resume the existing 3300 goal with Sol. Do not change that goal's
status merely because this document is complete.

Source anchor: `20d110bc8858c3f8be3b69566617bef31dbcbba4`; production engine code
is the same as `f75b578`. The companion
[structural review](2026-09-19-structural-strength-review.md) contains the evidence
and contrary evidence. This document turns it into executable work boundaries.
`CLAUDE.md` owns operating policy; the new handoff block in `TODO.md` owns queue
order. Later accepted commits become each next ticket's immediate base.

## 1. What to do first, and what success means

The first deliverable is a small, proved repair to two ProbCut contracts. The
first major-strength program is efficient Rodent V1.2, with one bounded
clock-allocation experiment admitted ahead of it if its mechanism and safety
preflight are clean. Do not spend another cycle exclusively on tiny SMP/TT
variants or isolated margin sweeps.

| Order | Ticket | Class | Exit condition |
| --- | --- | --- | --- |
| 0 | S0: establish exact working base and validation envelope | Setup | Clean isolated WSL worktree; scope and artifact paths pinned |
| 1 | F1: singular verification cannot reuse its excluded capture through ProbCut | Proved correctness | Failing-before/passing-after regression and required non-regression checks |
| 2 | F2: ProbCut records the live ancestor move | Proved correctness | Descendant sees correct two-ply history context; all exits unwind |
| 3 | C0 + V0: clock mechanism and V1.2 cost preflights | Read-only/diagnostic | One bounded report per question; no strength verdict |
| 4 | G0a + C1: admit same-model candidate runner; split iteration admission from recursive abort | Harness, then time policy | One frozen real-clock ADOPT/SHELVE gate; otherwise move on |
| 5 | V1/V2/V3: exact V1.2 output, update and conditional refresh kernels | Pure speed | Independent exactness and whole-search performance gates |
| 6 | G0b + E0: admit V1.2 to the harness; compare against Anand | Harness, then evaluator | Fresh A/A and one frozen same-search/equal-clock strength gate |
| 7 | L0/L1: demand trace, then lazy accumulators if justified | Conditional exact speed | Actual avoidable work, then representative exact-behavior speed gain |
| 8 | H0/H1: completed-winner history learning plus matching consumer | Explicit search interaction | Warm-state mechanism evidence, then one whole-package game gate |
| Later | P1 staged picker; N1 one richer-network feasibility study | Separate research | Admit only from new evidence; do not start both by default |
| Separate | D0 release contract; Q0 external confirmation | Delivery/evidence | No installation or 3300 claim without their own requirements |

C0 and V0 may share read-only analysis and separate frozen diagnostic builds.
They must not create two live speculative search candidates. If C1 fails its
preflight or is shelved, V1.2 advances immediately: no sequence of clock constants
to rescue it. If profiling makes a kernel unnecessary, skip that kernel with a
written reason and proceed to the fair evaluator comparison.

The order is about information per hour, not a predicted Elo ranking. There is
no supported estimate that these tickets contain a particular number of Elo.
The old hundreds-of-Elo deficits are not measurements of today's optimized
Rodent configuration. Internal gains must not be added into an absolute rating.

## 2. S0 — safe resumption and exact baselines

Read `CLAUDE.md`, the new `TODO.md` handoff block, this plan, then only the
evidence needed for the active ticket. Preserve unrelated edits, especially the
pre-existing untracked `SHA256SUMS` on the Mac. Do not rediscover the entire
project or replay completed compatibility milestones.
Sol owns implementation; the primary coordinator owns scope, frozen manifests
and acceptance. Use a bounded independent source/test review for the proved
fixes, assembly, ownership changes and required harness adaptations. Delegate
read-only review or independent fixtures in parallel, not duplicate monitoring
or multiple unaccepted production candidates.

All implementation, builds, engine execution, tests, profiles and matches stay
on authorized WSL (`ehrli@192.168.4.108`, distro `Ubuntu`). The Mac remains an
editing/remote-control terminal. The Lean hopper stays running; never put it in
a test process group, stop it for exclusivity, change its queue, or clean its
files. The historical PID is evidence, not a permanent process identity.

Use a **new clean sibling WSL worktree at an exact accepted commit** for each
candidate. Do not reuse the dirty Slice-C research checkout or the installed
`build/ngn`. Inspect existing worktree ownership first; do not reset, clean or
overwrite an existing directory. Keep generated artifacts outside Go package
discovery: ignored `output/` snapshots have previously contained partial Go
source trees. Record remote commit, patch and all touched-file hashes.

On September 19 the user clarified that the two-minute machine-check limit
applied only to the review/planning session. Sol may run this plan's normal
bounded WSL validation gates on the shared host. This does not authorize
unbounded exploration, exclusive-host assumptions, installation, or any change
to the Lean hopper. Use the frozen limits and stop rules for each ticket.

Configuration identities to carry forward:

| Role | Backend / model | Status |
| --- | --- | --- |
| Competitive working reference | `rodent-v1.1-anand`; model SHA-256 `5f7480b56538e9e64ee02fb5b4842bf0394ee44b707dae3a63ec7f902abd0afb` | Accepted evaluator and optimized kernels; external checkpoint used source `52ee629` |
| Larger candidate | `rodent-v1.2-default`; `rodent_4kb_768hl_8ob_v2.bin`, 4,744,768 bytes; SHA-256 `c35a1abc1b8c1cb1d5f4221454d494c1a6da1ed9088fd51ab27038bfa74b5053` | Scalar full/incremental and opt-in integration accepted; speed/strength not established |
| Ordinary default | HCE | Must remain default throughout these research tickets |
| Recorded WSL/Windows installations | Classical `53e4d1b` binaries | Not the current research baseline; do not overwrite |

For new experiments, rebuild both roles from the **same immediate accepted
search base**, except for the one declared candidate diff. Do not compare a
fresh candidate to the old `52ee629` executable after F1/F2 or C1 changes search.
For E0, prefer one executable with two explicitly verified backend/model
configurations. Networks remain external, unmodified and unbundled.

Use `/usr/local/go/bin/go` on WSL; bare `go` is not reliably on its noninteractive
PATH. Record actual Go version, `CGO_ENABLED`, `GOARCH`, `GOAMD64`, build flags
and binary hashes. Optimized amd64 builds explicitly require `GOAMD64=v3`;
`make build-release` alone does not establish that. Preserve a tested v1 fallback.
Check supported ISA with a lightweight read-only check before v3 execution.
Do not use `make build` or `make build-windows` for research packaging: their
recipes can copy an executable into a Windows/Arena directory. Use explicit
`go build -o` destinations inside the new experiment artifact directory only.

### Shared validation ladder

Run the narrow failing-before/passing-after test first. Then, from the clean
WSL worktree, the repository baseline is:

```sh
/usr/local/go/bin/go test -short ./engine -count=1
/usr/local/go/bin/go test -short -race ./engine -count=1
/usr/local/go/bin/go test -short ./... -count=1
```

Use `-p 1` or `-p 2` and a bounded `GOMAXPROCS` in the supervisor to share the
host. Add `go test -short -race ./... -count=1` for UCI, harness, worker ownership
or concurrency changes. These are future WSL commands, not instructions to run
them on the Mac. A skipped configured-network test is not a pass; its build tag
and external model/oracle environment must actually be present.

Each ticket must leave: a short scope manifest, exact diff/source/binary hashes,
test commands and terminal receipts, result/verdict, and the next unblocked
ticket. Keep raw large artifacts on WSL with an inventory; commit compact durable
evidence under `experiments/`. Preserve rejected patches as clearly inert
artifacts, not live production code. Commit proved/tested fixes and accepted
changes in coherent chunks; do not commit speculative production candidates
before their verdict. Verify private destination and push accepted commits per
the user's working agreement.

## 3. F1 — exclude ProbCut at the singular-verification root

**Evidence:** `engine/search.go`, `alphaBetaPV`: `inSingular` near line 1425,
TT-cutoff exclusion near 1427, ProbCut block near 1648–1708, ordinary excluded
move skip near 1799. Line numbers refer to the anchor; follow symbols after edits.
The [witness receipt](2026-09-19-strategic-review-artifacts/search-contract-witness-results.md)
proves that excluded capture `a1a7` can certify its own verification cutoff.

Smallest recommended change: add `!inSingular` to the **ProbCut eligibility
guard at the excluded ply**. Descendants where `ply != ExcludedPly` retain normal
ProbCut. This cleanly prevents the selective shortcut from answering the
excluded-root verification question; do not globally disable ProbCut or copy
all of a donor engine's singular policy. A capture-only skip is a narrower
alternative, but use it only with an explicit proof of the desired verification
contract; the baseline recommendation is the root guard.

Add `engine/search_probcut_contract_test.go` (suggested name). Convert the inert
research witness into a real regression, not a test which passes when the bug is
present. Fixture: `4k3/p7/8/8/8/8/8/R3K3 w - - 0 1`, verification depth 5,
window `[-201,-200]`, excluded `a1a7`, deterministic zero model and exact child
TT witness as in the retained source. Required assertions:

- before the fix the forbidden root child is visited and causes ProbCut;
- after the fix it is never visited **as the excluded root move**, and no
  ProbCut cutoff is credited at that root; avoid asserting one arbitrary whole
  search score or a global zero prune count if descendants legitimately prune;
- a matched ordinary search still exercises ProbCut, so the patch has not
  disabled the mechanism everywhere;
- a quiet excluded move with another legal capture available also disables
  ProbCut at that root; this distinguishes the chosen root contract from merely
  filtering one capture;
- scope is the excluded ply: a legal descendant may still use ProbCut;
- board/hash, `LastMovePlayed`, repetition/frame lengths, live ancestor slots
  and evaluator depth restore on normal return and a forced stop. Do not
  require learned history tables or inactive stack slots to remain unchanged.
  An excluded root must not overwrite the ordinary position's TT entry with a
  restricted/move-excluded answer.

Use existing test helpers (`newN3CDirectSearchInfo`, `n3cPosition`,
`adapterLegalMove`, evaluator `transitionObserver`) rather than adding a public
debug API. Keep the zero-model witness, and run current Rodent lifecycle tests
as the integration check. Exact full-search node identity is not expected for
a corrected search contract.

Acceptance: failing-before/passing-after proof, narrow source review, baseline
suites, and a short behavior non-regression check on normal/stopped searches.
If routine game validation is authorized, a frozen 20-opening-pair operational
smoke is reasonable; it is not an Elo gate and cannot prove no small regression.
Investigate crashes/illegality/state damage immediately. Do not demand a positive
Elo interval before keeping a proved fix, or label it an Elo gain.

**Not part of F1:** reverse futility, null move, correction learning and the
no-remaining-legal-move singular return. Their contracts deserve a separate
bounded audit if a witness exposes another problem. The review did not prove
them wrong. No blanket `!inSingular` sprinkling, terminal-score change, pruning
margin change or singular-extension retune belongs in this commit.

## 4. F2 — write the live move stack during ProbCut

`engine/search.go` ProbCut sets `LastMovePlayed` near 1680 but not
`MoveStack[ply]`; ordinary moves set both near 1892–1893. A grandchild consumes
`MoveStack[ply-2]` near 1744–1749 and can read a stale ancestor.

Set the current stack slot to the ProbCut move at the same lifecycle boundary
as the normal real-move path, before recursive child search. Follow existing
stack ownership: active ancestor slots must be correct; do not invent a
different global save/restore policy or alter history learning. Preserve
`LastMovePlayed`'s existing restoration and all board/evaluator unwind paths.

Regression uses the retained depth-6 witness with root stack deliberately seeded
to `e1d1`. It must independently prove a ProbCut root capture was traversed, a
grandchild actually reads the context, and the live slot is `a1a7`, not stale.
Make observer progress independent of seeing the defect so the repaired test
does not silently skip its assertions. Stop at that descendant and require the
stopped sentinel and exact unwind. Also cover ordinary completion and sibling
reuse, and cancellation in both the quiescence and reduced main-search stages,
so correctness is not confined to one interrupt path. Clearing the stack slot
instead of recording the actual move must fail the regression.

Keep F2 separately reviewable after F1; their regression helpers can share the
same test file. Apply the same correctness gate. Commit only after the actual
WSL-tested source is preserved. Neither F1 nor F2 claims to explain hundreds of
Elo. Once both are accepted, freeze their new commit as the experimental base.

## 5. C0/C1 — one clock-allocation experiment, not a constants search

### C0: preserve the measured question

**Completed September 19:** the opt-in first-cause observer and frozen warm
diagnostic are recorded in
[the C0 result](2026-09-19-c0-clock-preflight/README.md). All 12 searches ended
through a soft active abort, none at soft admission or hard/emergency bounds;
the post-completion tail was 25.74% in aggregate. Identity, short and race gates
passed with no measurable observer overhead. This admits the one C1 boundary
below but is not itself a strength verdict.

Existing logs show a 25.529% sum-of-search-time tail after NGN's last completed
root report. The conservative hard-bound analysis excludes hard/emergency
exhaustion for 90.61% of substantial nonmate tail under its timestamp assumptions.
Neither number is recoverable time or an Elo estimate. Unfinished iterations
leave useful TT/history. Last completed iteration costs are poor next-cost
forecasts: the eligible last-observation median growth was 2.114x. Do not fit a
new multiplier to these logs.

Use `engine/time.go` and the root/main/qsearch call sites in `engine/search.go`.
Before policy edits, add a test-only/opt-in observer for actual stop reason and
completed-iteration timing, with no ordinary UCI chatter or production hot-path
allocation. Categories must distinguish soft admission/soft recursive abort,
hard deadline, emergency reserve, external stop, node cap and terminal/depth
completion. Track aspiration retries as part of one iteration. Verify the
observer preserves fixed-node decisions and state; record its timing overhead
rather than treating an instrumented build as the production candidate.
Latch the **first decisive termination cause**, with documented precedence when
conditions coincide. A soft abort must not turn into a hard expiry during unwind
or helper join; a node-limit stop must not be relabeled external UCI stop. Test
both attribution traps with fake time and explicit control causes. Do not infer
the cause afterward just from the final elapsed time.

One bounded representative warm-search diagnostic answers whether the existing
soft abort actually accounts for the suspected mechanism. Reuse recorded game
prefixes and budgets; freeze selection before examining new outcomes. This is
not another day of profiling or a match. If the mechanism is absent or the
instrument is unreliable, stop C1 admission and advance V1.2. Do not turn a
diagnostic into a time-policy strength rejection.

### C1: exact candidate boundary

**Shelved September 19 before games:** the exact split passed deterministic
boundary tests, but its frozen warm preflight produced one hard abort in 12
searches, discarded 2.571 seconds in that cell and increased sequence time by
57.1%. Per the no-rescue and hard-abort safety rules below, no constants change
or game launch follows. See
[the C1 record](2026-09-19-c1-iteration-admission-preflight/README.md). Work
advances to V1/V2.

Proposed API split (names may follow local conventions):

- `ShouldStartIteration(completedDepth int) bool`: **unthrottled** root admission;
  always honors elapsed hard/emergency bounds, and after a completed iteration
  applies the existing soft formula, composed stability/effort factor and 1x
  previous-iteration projection. Use explicit completed depth, not
  `SearchInfo.Depth`, which starts at the maximum requested depth.
- `ShouldStopSearch(...)`: retain rate-limited **hard/emergency-only** recursive
  cancellation for Tournament; preserve FixedTime/TimePerMove, Infinite,
  FixedDepth and external/node-control behavior.

Call the first once at the new-depth boundary. Keep `NewIteration` once per new
depth with the previous completed iteration's full cost available to admission.
Document its ordering so an unstarted iteration is not reported completed.
Aspiration-attempt checks and root-move checks become hard-only, just like
recursive main and quiescence checks. Do not soft-admit every aspiration retry.
Retain both interrupted-root guards: only a fully completed iteration may
replace the published result. Preserve primary-worker clock ownership; helpers
must not mutate a shared single-goroutine `TimeManager` counter.

Unchanged: bank allocation formulas, move overhead, emergency reserve, stability
and node-effort constants, aspiration windows, node caps, external stop, and
reported score/PV fallback. **This changes spending even with those constants
unchanged.** It is a time heuristic requiring games, not a proved correctness fix.

Deterministic fake-clock tests must cover:

1. soft admission fires on a non-1024 boundary after a real completion;
2. depth one is not incorrectly skipped by the soft forecast, while an already
   exhausted hard/emergency budget still stops safely;
3. recursive calls past soft but below hard do not stop the admitted iteration;
4. hard/emergency checks still stop within the defined poll schedule; already
   requested stop is immediate through search control;
5. aspiration fail-low/high retries retain one iteration cost and no extra soft
   admission; an interrupted retry retains the previous completed root result;
6. fixed depth/nodes, infinite, fixed time, low-bank and zero-increment modes;
7. stop during evaluator transitions, all context unwinds, and SMP primary/helper
   ownership under race. Reuse `uciFakeClock`/`newTimeManager(clock.Now)` helpers.

Pin boundary arithmetic explicitly: with neutral factors, soft=1000 ms,
previous iteration=200 ms and hard=4000 ms, boundary admission succeeds at
799/800 ms and fails at 801 ms, preserving the current strict `>` forecast.
An admitted iteration continues past 801 ms; the next scheduled hard check at
4000 ms stops. Test emergency independently with a later hard deadline. With
no previous duration, preserve the existing `elapsed >= soft` fallback when
soft admission is eligible. Aspiration advances of 30+40+50 ms must record
120 ms, not only the last 50 ms; use `TestAspirationProgressiveWidening` to
exercise real retries. Reset must clear timing, counters and stop state.
Retain the UCI setup-deadline tests: setup time is charged from `go` receipt,
not erased by the first new-iteration call.

### Budget safety and game gate

Before launch, freeze C1's budget appendix, not just its score rule. Record for
each move: bank/increment, raw/composed soft budget, hard deadline, elapsed,
last completed iteration cost, stop reason, completed depth, final tail and
aspiration retries. Offline analysis may emit these from a separate diagnostic
run; ordinary competitive builds must have equivalent instrumentation on both
roles or none. Do not add candidate-only logging overhead.

Hard/emergency formulas and reserve remain exact; no candidate-caused flag,
stuck stop or unbounded polling interval is acceptable. State an explicit
wall-time overshoot allowance in the frozen manifest from the shared-host
baseline preflight and polling policy, not after seeing a candidate failure.
Compare spending/soft-budget ratios, hard-abort frequency and low-bank frequency
on identical replay inputs; in games report retained-bank distributions by ply
and game length, acknowledging that the positions diverge. Tail reduction and
nominal depth alone never pass the candidate. A material shift toward hard
aborts or depleted banks must be explained before launch; do not silently add a
new forecast constant to rescue it.

If safe and affordable, use G0's one-thread paired real-clock protocol: fresh
A/A100 then candidate400 at 10+0.1, positive paired 95% lower bound plus clean
audits to ADOPT. Otherwise SHELVE, preserving the precise candidate and mechanism
result. No extension, no post-result budget adjustment, no retry of the old T1a
projection-removal policy. An accepted result still needs the later held-out
clock confirmation before a broad time-policy/general-strength claim.

## 6. V0/V1/V2 — make V1.2 competitive without changing its evaluator

V1.2's four king buckets, 768 lanes and eight material heads are already
integrated. Do not redo loader/full-refresh/incremental/UCI slices, use the Tal
model, change release scaling or add NGN rule-50 damping. The exact release is
`b53ffaf670590932957cb63b7b6d871f6f33b7d8`; Slice A/B/C remain the oracle contracts.

### V0: bounded portable cost profile

**Completed September 19:** the supervised fixed-budget profile and separate
transition-incidence observer are recorded in
[the V0 result](2026-09-19-v0-rodent-v12-profile/README.md). V1.2 costs 6.214x
the optimized V1.1 aggregate cold time and 6.489x the persistent-prefix time.
`applyUpdates` and output evaluation account for 83.90% of labeled V1.2 search
CPU; frame copying is only 0.26%. The evidence fixes the order below as V1
output, V2 updates, then refresh only if a post-V2 profile still justifies it.

Profile current portable V1.2 against optimized Anand on the same accepted
search source, ISA, one thread, Hash128 and frozen cold fixtures plus persistent
game-prefix workloads. Pin model identities, actual work/nodes, allocations,
copy/update/refresh/output costs and king-bucket refresh incidence. Use a fixed
sample budget sufficient to choose the dominant kernel; do not turn this into
a general machine survey. The outputs are a cost breakdown and the V1/V2 order.
Lower scalar NPS is not a network-rejection result.

Reuse the private probe in
`experiments/2026-09-13-rodent-anand-artifacts/postoutput-profile/rodent_transition_profile_test.go.txt`.
Add the explicit V1.2 role through `LoadV12Default` and
`SearchEngine.SelectRodentV12DefaultEvaluator`; retain setup/search separation,
root legality and depth-zero context checks. Add a small frozen coverage set
for head thresholds and king-bucket/mirror transitions alongside the six legacy
fixtures. Do not require cross-model nodes/PVs to match.

### V1: exact 768-lane output kernel

**Completed and accepted September 19:** the exact AVX2 kernel and portable
fallback are recorded in
[the V1 result](2026-09-19-v1-rodent-v12-output/README.md). The direct kernel is
11.51x faster and the ten-block whole-search ratio is 0.611458 (38.85% less
time), with byte-identical cold/warm trajectories, exact allocation identity,
both order strata improving and 60/60 improving fixture pairs. Linux v1/v3,
Windows v3, released-network oracles, full short and race gates pass. V2 now
starts from this accepted base; do not reopen V1 or stack refresh work yet.

Scope: `rodentv12eval/evaluate.go`, new portable/dispatch/amd64-v3 kernel files
and their tests. Follow the accepted `rodenteval/output_dot*` pattern. Extract
the current arithmetic as the scalar reference before introducing assembly.
The existing V1.1 kernel's 32 blocks of 16 lanes become 48 blocks for 768; do
not copy it without reviewing widths, ABI, row addressing and head selection.

Preserve clipping to 0..255, squared activation, signed extreme weights,
modular int32 arithmetic, bias/scale division order and truncation toward zero.
The canonical V1.2 reference has four partial sums; preserve its exact result
and provide an explicit modular-arithmetic proof for any SIMD reassociation.
Evaluate only the selected material head and both correctly ordered perspectives.
Never widen through division, saturate intermediate sums or introduce floating
point to obtain plausible scores.

Gate synthetic zero/extreme/random/boundary/unaligned lanes, all eight heads,
all material head boundaries, v1 fallback and v3 SIMD, no input/model mutation,
no kernel allocations, exact raw and release-static oracle values. Run both
package and engine configured tests, plus cold and warm fixed-node identity.

### V2: exact incremental lane arithmetic

**Completed and accepted September 19:** the exact AS/ASS/ASAS AVX2 kernels and
portable fallbacks are recorded in
[the V2 result](2026-09-19-v2-rodent-v12-updates/README.md). Direct speedups are
17.58x–23.69x and the ten-block whole-search ratio against accepted V1 is
0.365683 (63.43% less time), with byte-identical cold/warm trajectories, exact
allocations, both order strata improving and 60/60 improving fixture pairs.
All oracle, fallback, full-short, race and platform-build gates pass. The next
step was the promised post-V2 profile; refresh SIMD was admitted only after that
profile showed it remained the largest flat cost.

Scope: `rodentv12eval/context.go` and narrow `apply_updates*` helpers, reusing
the V1.1 AS/ASS/ASAS patterns at 768 lanes. Preserve int16 wrap and operation
order, transition validation, two-perspective frame copy, king-bucket/mirror
refresh classification, null/pop ownership and model identity.

Tests include quiet/capture, both en-passant directions, both castling sides,
all promotions/capture promotions, king mirror/bucket boundaries for both
perspectives, rejected transitions, deep sibling reuse and forced stop. Compare
**all 1,536 lanes**, raw and static values against fresh full refresh; check
immutable weight rows, unaligned operands, no allocations, v1/v3, independent
SMP contexts and race.

For each pure kernel separately: use the existing ten-alternating-block,
six-fixture, 400,000-node, one-thread/Hash128 exact-result gate as the starting
contract. Freeze the immediate baseline and candidate, require exact cold/warm
nodes/scores/PVs and allocation equality, median elapsed candidate/base <=0.97,
every fixture and both order strata improving, plus direct-kernel improvement.
Retain shared-host telemetry instead of reviving exclusivity. A noisy or
subthreshold capped result shelves that kernel; no extension. Do not stack the
second unaccepted kernel on the first.

### V3: exact king-perspective refresh

**Completed and accepted September 19:** the post-V2 profile measured
`refreshPerspective` at 2.52 of 10.75 seconds of labeled search CPU (23.44%
flat, 24.65% cumulative), making it the largest remaining flat cost. The exact
variable-row AVX2 refresh kernel and portable fallback are recorded in
[the V3 result](2026-09-19-v3-rodent-v12-refresh/README.md). At realistic row
counts its direct speedup is 16.84x–19.79x, and the ten-block whole-search ratio
against accepted V2 is 0.784510 (21.55% less time). Cold/warm trajectories and
allocations are exact, both order strata and every fixture improve, and all
60/60 paired cells improve. The next step is a fresh post-V3 profile, followed
by G0b and the fair V1.2-versus-Anand strength gate unless profiling reveals
another major exact backend cost.

**Post-V3 decision completed September 19:** the exact accepted-V3 profile is
recorded in
[the post-V3 profile result](2026-09-19-post-v3-rodent-v12-profile/README.md).
Refresh is down to 1.58% flat (2.44% cumulative), the frame copy is 2.01%, and
no evaluator function exceeds 10% flat CPU. Remaining leading costs are spread
across generic search, TT access, output, move ordering and frame handling.
There is no fourth backend-kernel ticket before games. Advance to G0b/E0.

Scope: `rodentv12eval/context.go` and narrow `refresh*` helpers. A validated
board contributes at most 64 rows. The helper gathers pointers in the original
plane/square order; every lane begins at its bias and visits rows in that exact
order, preserving modular int16 behavior. The amd64.v3 kernel processes 48
unaligned 32-byte blocks, with compile-time guards for 768 lanes and 64 rows.
Tests cover zero through 64 rows, full-int16 wraparound, unaligned inputs, all
lane boundaries, a fully occupied valid board, immutable inputs, zero
allocations and v1/v3 dispatch. No model, feature mapping, search policy, frame
copy or output behavior changed.

The original profile could have justified update before output, but the accepted
order and bases are now explicit. No lazy evaluation, Finny, validation
removal, frame-copy fusion, search change or model-strength verdict belongs in
these kernel tickets. Major scalar costs are now addressed; the fresh post-V3
profile has stopped kernel work. If an exact kernel had
failed its cost gate, the current implementation could still have supported an
informative E0 match; kernel speed never proves or disproves the network.

### Configured test map

Set these to the already preserved WSL files after verifying their hashes:
`RODENT_V12_DEFAULT_MODEL`, `RODENT_V12_DEFAULT_ORACLE_JSON`,
`RODENT_V11_ANAND_MODEL`, `RODENT_V11_ANAND_ORACLE_JSON`.
The full Anand tagged package suite also requires
`RODENT_V11_ANAND_TRANSITION_FIXTURES`,
`RODENT_V11_ANAND_TAGGED_TRANSITION_ORACLE`, and
`RODENT_V11_ANAND_RELEASE_TRANSITION_ORACLE`. Exact retained paths and hashes
are the `model`, `release_oracle`, `transition_fixtures`,
`tagged_transition_oracle` and `release_transition_oracle` entries in
[the accepted kernel manifest](2026-09-13-rodent-anand-artifacts/update-kernel/gate/frozen-manifest.json).

V1.2's oracle is retained under
`/home/ehrli/rodent-v1.2-default-oracle-20260919/run-004`, JSON SHA-256
`a16487b7a7faf725d4c2369c91467274c10ecd04b8a4a0b8faec5c48fc08d440`.
Resolve its exact model filename from that inventory and verify the known model
hash; do not guess a path or redownload an unpinned replacement.

```sh
# Run on WSL with GOAMD64=v1, then GOAMD64=v3, and repeat relevant paths under race.
/usr/local/go/bin/go test -short -p 2 -tags=rodentoracle ./rodentv12eval ./rodenteval -count=1 -v
/usr/local/go/bin/go test -short -p 2 -tags=rodentv12contextoracle ./rodentv12eval -count=1 -v
/usr/local/go/bin/go test -short -p 2 -tags=rodentv12oracle ./engine -run 'Test(RodentV12|UCIRodentV12)' -count=1 -v
/usr/local/go/bin/go test -short -p 2 -tags=rodentoracle ./engine -run 'Test(RodentV11|UCIRodentV11)' -count=1 -v
```

These package/engine tags differ deliberately. Check the complete parent tests
execute, not merely that `go test` exits zero after a bad `-run` filter. The
V1.2 package release oracle has 104 records. Require Linux v1/v3 and Windows v3
build-only packaging where assembly changes; packaging is not installation.

## 7. G0/E0 — minimal valid harness, then a fair model decision

### G0a/G0b: narrow harness preparation, not a port of every old runner

**G0a runs before the first C1 game gate**, and is reused for H1: admit a
same-backend/model, two-immutable-binary base/candidate contract. The only engine
difference is the reviewed clock/history patch. Freeze separate role hashes and
verify the same selected evaluator on both. Do not apply E0's one-binary rule
to a source-policy experiment. Common supervision, pairing and audit preparation
belongs here; there is no need to finish the V1.2 backend adapter before C1.

**G0b runs before E0.** The shared `scripts/candidate-match/uci_preflight.py`
currently omits `rodent-v1.2-default` from its backend allowlist and diagnostic regex. Extend
those and focused tests to require the exact new diagnostic, advertised option
and selected backend. Negative tests must still reject wrong-backend output,
missing/duplicate acknowledgement, failed model load, book leakage and malformed
records. Preserve EvalFile-before-EvalBackend transactional staging.

**Completed September 19:** G0b is recorded in
[the harness admission result](2026-09-19-g0b-v12-harness/README.md). The narrow
preflight now admits and proves `rodent-v1.2-default`; all 29 WSL harness tests
pass, including mismatch and failure witnesses, and real configured V1.1/V1.2
preflights pass through one exact binary. The generic manifest remains
unchanged. This admits preparation of E0; it is not a strength verdict.

Do **not** broaden the generic HCE/NGN-v1-only manifest schema accidentally. Use
a separately frozen Rodent-specific manifest/driver descended from the accepted
Rodent comparison, with an explicit V1.2-versus-Anand role contract. Older
`driver-held-v1.py` files contain obsolete CPU exclusivity; use the documented
shared-host amendment. Retain the corrected concurrent trace-state key
`(trace thread, name)`, strict startup record counts despite interleaving, and
the independent full legal/terminal audit. Never edit a frozen tool mid-run.

Test just the relevant runner components and simulated protocol failures. The
approximately 2,000-line external-runner port and its 27 inherited failures are
separate maintenance, not a prerequisite to the next model experiment. If a
required component fails, fix and validate it separately before any verdict;
preserve failures instead of retroactively declaring their original run clean.
The existing narrow harness suite is `python3 -m unittest discover -s tests -v`
from `scripts/candidate-match` on WSL. Run it with the ticket's supervisor cap;
do not launch engines on the Mac through a test fixture.

### Default bounded strength protocol

Use this for C1, E0 and H1 unless a different protocol is explicitly frozen and
reviewed **before** results. This plan is not the released manifest.

- Same immediate accepted source/configuration except the declared candidate;
  E0 uses the same binary with the two exact backend/model choices.
- Threads1, `GOMAXPROCS=1`, Hash128, NGN Move Overhead100, OwnBook=false,
  no ponder/tablebases/score-resign-draw-maxmove adjudication.
- 10+0.1, concurrency four, recorded established CPU mask 0/2/4/6 after a
  lightweight topology/capability check. Share the hopper; keep CPU telemetry.
- Fresh A/A100 at the identical TC/concurrency/affinity/machine; require clean
  audits and the paired interval to include zero. That is a protocol check,
  not proof of fine-grained timing equivalence. Freeze which role's backend is
  used for A/A, and preflight both distinct E0 configurations.
- Candidate400 = 200 full reversed-color opening pairs. Freeze exact book bytes,
  selection/start/length and sequence. Changing a seed does not shuffle a
  sequential book. Existing default book hash is in `experiments/corpus_manifest.md`.
- Whole-opening-pair bootstrap, 100,000 resamples, seed 2026091201; publish WDL,
  pentanomial, paired estimate/interval, operational failures and disclosed load.
- ADOPT only if all frozen correctness/runtime/audit requirements pass and the
  paired lower 95% bound is strictly positive. Otherwise SHELVE at the cap.
  Positive point estimate, nominal depth, tail reduction, donor strength or
  microbenchmark wins do not override the rule. No provisional keep here.
- Estimate wall time before release; target the repository's 1–2-hour
  confirmation budget. If the declared run cannot fit, hold it or prospectively
  design a different bounded gate before seeing scores. Never shorten a live
  run into an adopted result or extend an observed near miss.

Freeze source/patch, executable/model, launchers/options, Go/build ISA, opening
selection, harness/auditor hashes, affinity/clock, budget, stop rules and output
paths. Use unique run directories and a bounded durable supervisor with an
atomic final receipt and process-survivor check. Record the exact job handle.
Use completion notification/native wait. If only polling is available, choose
roughly 10–15 minutes for an hour-scale job, silently within a suitable waiter;
do not repeatedly wake the model to narrate games. Never restart on a wait
timeout. If efficient waiting is unavailable, explain that limit once.

For E0 specifically, preserve exact release-static semantics and current search
corrections on both roles. V1.2 has not earned a default, install or release
change by passing compatibility. A valid capped loss/inconclusive result keeps
optimized Anand as reference and advances the next lane. No network training,
scale fitting or prolonged SIMD salvage campaign follows automatically.

Optional equal-node quality triage is **not required** before E0 and must not
become a tooling detour. Existing `acpl.sh` can accidentally compare default HCE,
leave OwnBook enabled and treat omitted reference moves as eighth-place scores.
Only use it after satisfying the
[quality-screen contract](2026-09-19-strategic-review-artifacts/evaluator-quality-screen-contract.md).
It cannot adopt or reject an evaluator without real-clock games.

**Completed and accepted September 19:** the separately frozen E0 package and
result are recorded in
[the V1.2-versus-Anand result](2026-09-19-e0-v12-anand-strength/README.md).
Fresh Anand/Anand A/A passed with a paired-bootstrap interval containing zero.
V1.2 then scored 145W/188D/67L in the fixed candidate400 gate, paired +68.63 Elo
with a 95% whole-pair bootstrap interval of [+45.42,+92.46]. Every legal,
terminal, operational, process and supervision audit passed. **ADOPT V1.2 as
the competitive evaluator and target for L0/H0.** This remains a relative
same-search result; default selection, packaging and installation remain under
the separate D0 release boundary.

## 8. L0/L1 — demand-driven accumulators, conditional on measured waste

Target the current accepted competitive evaluator, not two backends at once.
`engine/worker_evaluator.go`, `rodenteval/context.go` and
`rodentv12eval/context.go` are the ownership boundaries. First trace per-
perspective dirty/materialized state and transition dependencies while retaining
the eager arithmetic. On evaluation, mark the actual needed ancestor/delta
chain; on pop, classify work abandoned without any eventual consumer. Null moves
may alias immutable state. **Do not use `1 - evalCalls/pushes`: unevaluated
ancestors can still be required by evaluated descendants.**

Freeze representative warm game-prefix and cold workloads. Translate avoidable
copies/updates/refreshes into a bounded whole-search cost opportunity using that
profile, not nested percentages added together. The saved Anand `PushMove`
21.60% cumulative share is an upper bound on that entire region, not the lazy
gain. If even the optimistic avoidable share cannot clear a worthwhile ~3%
whole-search gain, shelve this prototype with a reopen condition (richer model
or a materially different measured demand profile). Otherwise admit one exact
lazy implementation.

**Completed September 19 — L1 not admitted.** The frozen perspective-aware L0
observer preserved all 12 cold/warm fixed-node trajectories exactly and closed
every accounting partition. Cold demand maps to 0.559% directly attributed
whole-search opportunity and a deliberately favorable 0.844% ceiling; the
warm-prefix extrapolation is 1.240%. An existing-profile line check found the
entire null push at only 0.14% of search CPU, raising those ceilings to 0.854%
and 1.254%. These are far below 3%, so L1 is **shelved**. Reopen only after a
material evaluator/search change produces a different measured demand profile,
or a fresh profile assigns at least 3% whole-search cost to actually avoidable
work. Exact evidence is in
[the L0 demand result](2026-09-19-l0-v12-demand/README.md). The implementation
notes below are retained only as the frozen reopen design, not as active work.

Keep validated move/king metadata per frame, materialize needed perspectives on
demand from the nearest valid ancestor or full refresh, and retain independent
worker/model ownership. Ancestor reuse must respect the perspective's king-view
metadata: crossing its own king bucket/mirror boundary requires refresh, not
blind delta replay from an incompatible accumulator. Cover alternating null/real
moves, mirrored/bucket king changes, in-check returns, TT/draw returns, siblings,
reset/reselection and every stop/unwind. Exercise backing-slice capacity growth:
null aliases/indices must not become stale or mutate siblings when storage moves.
Never reuse a cached frame from another model identity or worker.
No Finny cache, SIMD rewrite or changed score/feature policy in L1.

Gate all-lane/evaluation parity and deterministic one-thread cold **and warm**
nodes/scores/PVs. Warm workloads must retain real game history/TT/evaluator
ownership between searches and be frozen before timing. Predeclare ten
alternating paired blocks and workload weights; require aggregate elapsed ratio
<=0.97, both order strata improving, no unexplained allocations/memory growth
and no material cold-family regression (freeze the bound before testing).
Unlike the narrow SIMD gate, do not require every individual root to speed up
if the frozen representative workload wins. Report cold and warm separately.
Run multiworker ownership/race/lifecycle tests; SMP schedule node identity is
not a realistic deterministic requirement.

Finny refresh caching is a **later separate ticket only if refresh remains
material**, especially with V1.2 king buckets. Prototype worker-local,
model-owned entries keyed by `[perspective][mirror][king bucket]` (2x2x4 = 16
entries for V1.2) with exact full-refresh parity and bounded memory. Do not cache
output scores across changed material heads. Lazy demand and king-refresh reuse solve different costs; do
not add their profile ceilings or bundle them to evade independent attribution.

## 9. H0/H1 — one coherent learning-and-selection experiment

Detailed contract:
[search-history-design.md](2026-09-19-strategic-review-artifacts/search-history-design.md).
Donor comparison is pinned Counter 5.5, not a wholesale Stockfish search port.
Work in `engine/moveorder.go`, the separate root loops and `alphaBetaPV` in
`engine/search.go`; retain the immediate accepted evaluator and clock policy.

H0 uses a passive observer over frozen held-out game prefixes, preserving warm
history. Measure weighted missed quiet-PV learning events, history/rank
distributions, aliasing, shallow history-prune firing, and reduced-search
fail-high/rescue rates. Shadow calculations must not alter the baseline tree.
If the hypothesized signal has negligible update mass or produces no intended
ranking/selection mechanism, stop with a precise reopen condition. A six-root
nominal-depth/FMC comparison is not a common quality horizon across policies.

H1 is explicitly one interaction package with three fixed components:

1. Learn once from a completed node's **final quiet winner** if it raises the
   original alpha, including root/PV attempts; never every intermediate alpha
   raise and never an interrupted result. Reward that winner; penalize only
   preceding searched quiets; ignore later PV alternatives. Do not penalize the
   winner. A final noisy winner performs no quiet-history update. Retain
   killer/countermove policies.
2. Retain NGN's entry bound `H=8192`, piece/to main key and separate one-/two-ply
   context arrays. Use widened integer arithmetic
   `h += (target-h)*min(depth*depth,400)/512`, target `+H` or `-H`.
   Do not funnel this through the old 2048 bonus cap. Add a quiet-only helper:
   existing `gravityUpdate` also serves capture history and must not silently
   change capture updates. Test capture-history arithmetic remains unchanged.
3. Replace only the history term in LMR with
   `reduction -= clamp(sum/2500,-2,+2)`. Keep base table, eligibility, other
   modifiers, capture history and the existing history-prune threshold -1000.
   Its changed firing distribution is an explicit side effect to measure.

Regression matrix: quiet fail-high, in-window quiet PV winner, fail-low/no alpha
raise, noisy winner, best quiet found before later quiet alternatives, interrupted
node, aspiration fail-high/retry without double learning, root/interior parity,
int bounds/negative truncation/saturation, predecessor availability and null
contexts. Confirm F2's live ancestor contract. Do not add butterfly indexing,
extra continuation plies, a new picker or re-tuned margins to this package.
Each completed aspiration attempt may independently learn, including a completed
fail-high attempt. Forbid learning twice for the **same attempt** at cutoff and
epilogue; do not suppress all later completed retries merely because that depth
already supplied an earlier valid label.

Default: one combined `11 versus 00` full game gate under G0. A small 2x2 pilot
is optional only if it answers a declared interaction question within the budget:
`10 versus 00` and `11 versus 01` on the same 64 opening pairs (256 games), plus
an optional predeclared `11 versus 00` on 64 pairs (128 games). It is not a
substitute adoption test or a few-Elo detector. Specify the pilot-to-confirmation
decision before scores, use opening-block uncertainty, and reserve independent
confirmation openings. No posthoc best-cell selection or divisor fitting.

One package failure rejects that package, not the whole search-learning family.
The old 4x bonus and `/750` variants remain failed historical candidates; do not
revive them by renaming them. The historical T17/T8 success used different
baselines and is a motivation for interaction testing, not proof this bundle wins.

## 10. Deferred architectural branches — avoid another indefinite project

**P1, staged picker:** only after a fresh accepted-base profile still shows
material whole-list preparation cost. Use existing pseudo-legal TT validation,
then TT/capture/quiet stages with duplicate/legality coverage. Preserve ordinary
search semantics where intended, but deferred history reads can alter ordering,
reductions and pruning: this is a search-and-speed candidate requiring games,
not a mandatory node-identical refactor. Do not reinstate `6bf83e9`, the June
sentinel or rejected block selector unchanged. Predeclare prototype cost cap,
mechanism falsifier and game gate before work.

**Admitted September 19:** the fresh accepted-H1 profile measured 330 ms at the
ordinary main-search generation call and 270 ms at its whole-list scoring call,
6.91% of 8.68 seconds of labeled CPU against the prospectively frozen 3% gate.
The result and the bounded P1-R1 reference-stage contract are recorded in
[the P1 profile directory](2026-09-19-p1-staged-picker-profile/README.md). The
contract explicitly rejects the old TT-first/global-fallback shape and kills the
candidate before games if fixed-depth node inflation repeats.

**N1, one richer model:** after the V1.2 result and search/runtime findings, choose
one feasibility study, not concurrent ports. Berserk 14 is a plausible
threat-free deep-network bridge (pinned source
`8ae895a6151695be4a50d4fb65b0c131659c513a`); full model artifact/license provenance
is unresolved and must precede incorporation. The existing SF18 BIG/dual plan
is the alternative, with threat-feature maintenance, PSQT, selection/fallback
and substantially larger state. Its current SMALL implementation is an oracle,
not a ready selected-head production path.

Deliver only provenance, an independent oracle contract, legal-move-trace
feature/update cost, selected-head cost, memory/ownership estimate and a bounded
slice plan. No search integration until that feasibility decision is reviewed.
Lower NPS alone is not model rejection; donor strength alone is not NGN strength.
Owned training, new training-data generation, GPU search and language rewrites
remain out of scope.

## 11. D0/Q0 — make earned gains usable without erasing the release hold

D0 is separate from research acceptance. The recorded installations still hash
to classical `53e4d1b`; a stronger source configuration does not mean the user's
GUI is running it. Do not claim to have checked that GUI. Preserve the old
installation and rollback artifacts.

Prepare a **Rodent-specific release amendment** naming the exact accepted source,
Linux/Windows binary hashes, ISA, model hash/path, backend, score policy, startup
options and known limitations. Prefer an explicit opt-in launcher/configuration;
changing the global HCE default is a separate choice. Document README build and
launch examples accurately, including v3 and external-model requirements.

The old external V1.2 opponent emitted promotion PV token `g2g1`; its 50-game
partial result failed trace policy and remains unaccepted. That is not evidence
against NGN's imported V1.1 evaluator. The amendment must either resolve that
specific external-reporting boundary or **explicitly close its relevance to the
new scoped release**, preserving the failed record and why it does not certify
the new package. Do not silently relax the auditor, score the old failed run,
pretend all missing external cells passed, or run the stale Counter installation
runbook with its `53e4d1b` source precondition.

Only after the user/release authority approves the amended deployment scope:
build the actual accepted source, verify exact model/configuration and ISA,
perform startup/ready/selection/stop/newgame/restart checks, stage recoverably,
install only the named targets and prove rollback. Never infer installation
approval from this plan, a kernel acceptance, a strength gate or a goal resume.

Q0 is one later configuration-matched external checkpoint and, for a substantial
finalist, one held-out longer-clock confirmation. A same-exact-network native
Rodent comparison is particularly useful: it separates evaluator identity from
the combined search/runtime/clock gap. It is not pure search-only evidence.
Pin native artifact, exact model/adapter, supported options, one thread/hash,
book/TB/contempt and clocks; do not substitute a different personality/release.
Keep any native reporting fix and its protocol approval separate from NGN's gate.

Use independent openings (the recorded 16-ply Stockfish-book selection is an
available starting artifact) and prospectively choose the longer clock; the
historical 60+0.6 proposal is not a completed test or automatic launch authority.
Freeze a meaningful deficit threshold before interpreting a same-model control;
an interval containing zero does not prove a material deficit absent.

**3300 completion remains a separate evidence decision.** Before releasing the
final rating-validation manifest or observing its scores, freeze the rating
scale/pool, clock/hardware configuration, exact artifact, calibration method,
treatment of anchor uncertainty and the actual statistical completion condition.
If the user's intended definition is not settled, ask then; do not invent it
from an opponent's published number or retrofit a diagnostic Q0 result into a
goal-completion test. An official list rating requires that list's process;
local matches are not an official rating or authorization to submit externally.
Never add internal Elo wins, or add +28.73 to Counter's rating, to mark the goal
complete. A capped inconclusive candidate is shelved, not proof a whole family
has no potential; the next ticket must bring a genuinely different mechanism.

## 12. Resume prompt and mandatory handoff receipt

Suggested user prompt:

> Resume the 3300 goal with Sol. Read CLAUDE.md, the current TODO handoff and
> experiments/2026-09-19-sol-fix-plan.md. Start at S0/F1, preserve the running
> Lean hopper and all unrelated work, and work through the ranked unblocked
> tickets. Keep fixes, exact-speed changes, strength experiments and deployment
> separate. Honor the current machine-time limit; freeze each longer gate before
> launch. Commit and push verified accepted work, preserve failed evidence, and
> do not claim 3300 or install anything without their separate criteria.

At each natural stopping point record: ticket and exact accepted base; files
changed; proved/experimental classification; completed tests versus skipped/held
checks; exact remote artifact/job handle if still running; frozen verdict rule;
accepted/shelved/blocked-by-authority disposition; next specific action. A partial
implementation is not an accepted baseline. Do not restart a completed ticket
or duplicate a running job after a context/model switch.
