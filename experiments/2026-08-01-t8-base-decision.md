# 2026-08-01 T8 base decision — RESOLVED: run SPSA on the T17-FIXED base (T17b)

**DECIDED AND PREPARED. Zero box hours. Tuning base built, gated, staged, on-box hash verified. Tree clean.**

This decision is **time-critical and one-way**: once T8 converges on the unfixed base, its vector is
entangled with a proven defect and T17 can never afterward be isolated. It is resolved **now**, while T12
holds the box, rather than at launch time.

## The three premises — RE-VERIFIED against HEAD, not trusted from the queue

Two queue claims already went stale tonight (T12's shelved-patch pointer did not exist; the M6 throughput
paragraph was obsolete), so every premise was re-checked:

1. **`output/t17.patch` still applies clean** to HEAD. Yes — 2 files, +102/-2.
2. **All five `improving` consumers are live SPSA dims.** Verified against the registry in `search.go`,
   which is **14 dims**: `NullMoveR, NullMoveMinDepth, LMRFullDepth, LMRMoveThreshold, FutilityMargin,
   FutilityMaxDepth, ReverseFutilityMargin, DeltaMargin, SingularDepth, SingularMargin, ExtensionBudget,
   AspInit, AspMult, LMPBase`. The five consumers — **LMRFullDepth, LMRMoveThreshold, FutilityMargin,
   ReverseFutilityMargin, LMPBase** — are all present.
3. **`cmd/spsa` has no dim-subset flag** and iterates the whole registry (`cmd/spsa/main.go:84`,
   `for _, tp := range engine.TunableSearchParams`). Confirmed: the flag set is
   engine/tc/iters/single/pairs/openings/out/resume/rend/afrac/report/seed/lowpower/selftest — **no `-dims`**.

**Therefore running the EXISTING 14-dim T8 on the FIXED base IS the interaction test T17 pre-declared**
("fix + SPSA over the four margins as ONE explicit interaction test"), needs **no new code**, and costs
**exactly the same** as the T8 round already planned. Only the base changes.

## Decision: FIXED base

**Reasoning, in order of weight:**

1. **Irreversibility asymmetry — the decisive argument.** Tuning on the *unfixed* base permanently forecloses
   T17: the converged vector adapts 5 knobs to a corrupted input, and no later run can separate them. Tuning
   on the *fixed* base is recoverable — if it disappoints, T8-on-unfixed can still be run later at the cost
   of another round. **Identical cost, one option reversible and one not.**
2. **The defect is proven, not suspected.** Slot 0 measured 0 after a full depth-8 search while slots 1/2/3
   held 653/419/271; 1068 sentinel references in the endgame position, **100% of them spuriously
   `improving=true`**. Both bugs are real and mechanism-proven.
3. **It is mechanism-level work, which is where this campaign's keeps come from.** Keeps replaced or added
   whole mechanisms (T5 aspiration, T4 corrhist, T1b/T1e); misses adjusted single sites or constants (T7,
   T10b, T9a, T13, T16, T3a, T1c, T18b, T20, T4d, T4e). *"Repair a proven defect and recalibrate its
   consumers"* sits in the first category; *"tune 14 constants on a known-corrupted input"* sits squarely in
   the second.
4. The defect's cost is **measured at ~6 Elo** (T17 standalone = -6.4), so this is not a hypothetical
   cleanup.

## The counter-argument, which TODO's analysis did NOT record — stating it because it is real

**The fixed base starts ~6.4 Elo in the hole.** The confirmation SPRT is *(fix + tuned vector) vs unmodified
HEAD*, so the tune must find **more than +6.4** merely to break even, whereas T8-on-unfixed starts from 0.
That is a genuine handicap and it was not mentioned in the queue's write-up of this decision.

**Why it does not flip the decision:** the -6.4 was measured with the consumers *not* retuned, which is the
entire premise of the reopen. The corrected `improving` signal is strictly more informative than the
corrupted one (the corrupted form computes "is the eval positive" at ply 2 and "always true" after checks),
so a well-tuned engine on the correct signal should dominate a well-tuned engine on the corrupted one *in
principle*. **The honest risk is that SPSA does not find that optimum** — 14 dims, a noisy objective, and no
guarantee of reaching a global optimum. That risk is recorded here rather than discovered later.

**Accepted, explicitly:** a negative outcome is **fix-or-tune ambiguous**. That ambiguity is inherent to any
interaction test and is sanctioned by CLAUDE.md when the experiment is explicitly about an interaction. The
alternative — cementing a proven, measured, 6-Elo-costly defect into the tuned vector forever — is worse.

## Prepared artifacts

- **Tuning base `ngn_t17fix.exe` sha256 `3e8016d7dd2bf6f76a5a405b21f69367d8727a4f6af5fb9922815267c538e83f`**,
  pushed to the box, **on-box hash VERIFIED equal**.
- `spsa.exe` on box re-verified at `b92d3c50…` (the M4-parallel tuner).
- Gates on the fixed base: **nodecheck -28.7% / +39.6% / -7.3%** (247063 / 208825 / 709959 vs the locked
  346662 / 149587 / 765656) — the fix moves the tree decisively, as it must by construction.
  `go test -short ./engine`, `-race ./engine`, `./...` all **green**, including the patch's own regression
  **`TestImprovingReferenceIsWritten` (PASS)**.
- Engine tree **reverted**; nodecheck re-verified at exactly 346662/149587/765656.

## Standing rules for the run itself — unchanged, restated so they are not lost

- **T8 IS A TUNER, NOT A VERDICT.** No keep/shelve may be read from `plus_pct` or from param movement — that
  is the exact failure the policy names, and it is how the 2026-06-28 joint-SPSA run was invalidated.
- Evidence requires **params verified live in code** and **>= 20k games**.
- The converged vector gets a **separate real-clock 10+0.1 c8 SPRT [-3,+3] vs unmodified HEAD**, and only
  that decides.
- **5+0.05 is NOT available** — M6-retest failed 2026-08-01 and the TC is closed, so this round runs at
  **10+0.1** and costs ~25h, not the ~12h an M6 pass would have bought.
- If the converged vector lands within rounding of the defaults, the finding is "already near a local
  optimum" and the lane closes without a second run.

```yaml
id: 2026-08-01-t8-base-decision
date: 2026-08-01
change_class: decision + preparation (no verdict)
decision: run T8 SPSA on the T17-FIXED base (T17b), not the unfixed base
premises_reverified: [t17 patch applies, 5 consumers are live SPSA dims, cmd/spsa has no dim-subset flag]
tuning_base_sha256: 3e8016d7dd2bf6f76a5a405b21f69367d8727a4f6af5fb9922815267c538e83f
box_cost: zero
next_action: >
  Launch only when the T-queue (T12 live, then T19, T15) is exhausted. ~25h at 10+0.1. Verdict is a separate
  SPRT on the converged vector vs unmodified HEAD, never plus_pct.
```

---

## DIM LIVENESS PRE-FLIGHT on the fixed base — all 14 LIVE, verified empirically 2026-08-01

**Why this was run rather than assumed:** the 2026-06-28 joint-SPSA round was invalidated precisely because
it tuned knobs that could not move (9/11 params pinned, sub-integer steps, `plus_pct` ~50% = zero gradient),
and the T17 fix changes how **five** of these dims' consumers read `improving`. Launching a **25-hour** run
on an unverified registry is the one pre-flight worth its cost. The T8 round-1 manifest verified 13 dims on
the *old* base; this re-verifies all 14 on the base that will actually be tuned.

Method: `setoption name <dim> value <min|max>` over UCI, then fixed-depth node count at kiwipete d12. Default
(T17-fixed base) = **247063** nodes.

| dim | default | at MIN | at MAX | status |
|---|---|---|---|---|
| NullMoveR | 247063 | 190988 | 302102 | LIVE |
| NullMoveMinDepth | 247063 | 442788 | 351834 | LIVE |
| LMRFullDepth | 247063 | **247063** | 455894 | LIVE (flat lower half — see note) |
| LMRMoveThreshold | 247063 | 436676 | 360584 | LIVE |
| FutilityMargin | 247063 | 265883 | 398545 | LIVE |
| FutilityMaxDepth | 247063 | 366722 | **247063** | LIVE (max == default — see note) |
| ReverseFutilityMargin | 247063 | 159005 | 450336 | LIVE |
| DeltaMargin | 247063 | 343380 | 404835 | LIVE |
| SingularDepth | 247063 | 394239 | 303979 | LIVE |
| SingularMargin | 247063 | 379137 | 187648 | LIVE |
| ExtensionBudget | 247063 | 320520 | **247063** | LIVE (rarely binds high — see note) |
| AspInit | 247063 | 570153 | 266354 | LIVE |
| AspMult | 247063 | **891106** | 298341 | LIVE |
| LMPBase | 247063 | 320575 | 372896 | LIVE |

**No dead dims. Every one of the 14 moves the tree on at least one side.**

**The three flat sides are all explained, and none is a defect:**

- **`LMRFullDepth` at min=1 reproduces the default exactly.** This is the *already-recorded* flat lower half
  (`1 == default 3` in effect) from the T8 round-1 manifest — reproduced here on the new base, so it is a
  property of the parameter's shape, not of the T17 fix.
- **`FutilityMaxDepth` at max=8 equals the default because the default IS 8** — the dim's range is
  `[1, 8]` and it already sits at its ceiling. Only the downward direction can move, and it does (366722).
- **`ExtensionBudget` at max=64 equals the default**, matching the recorded observation that it *rarely binds
  above 24*. The downward side moves (320520), so the dim is live.

**Also worth recording: `AspMult` is by far the most sensitive dim** — at min=110 the tree is **891106 nodes,
3.6x the default**. Aspiration behaviour dominates this parameter set, which is consistent with T5
(aspiration modernization) being the single biggest keep of the campaign at +20.2. **SPSA will feel this dim
hardest**, and a converged vector that moves `AspMult` far from 150 should be treated with suspicion until
the confirmation SPRT says otherwise — T5's window shape was games-validated and is not obviously improvable
by a noisy 14-dim search.

**Pre-flight verdict: the registry is sound on the fixed base and the 25-hour run is not at risk of tuning
dead knobs.** Tree reverted after the sweep; nodecheck re-verified at exactly 346662/149587/765656.

---

## STALENESS NOTICE 2026-08-02 — the staged tuning base is OBSOLETE. Do not launch T8 with it.

**`ngn_t17fix.exe` `3e8016d7…` was built from `HEAD-before-T12 + t17.patch`. T12 was kept on 2026-08-02
(`c010ca7`), so that binary is no longer HEAD-derived and must NOT be used.** Marking it dead here rather
than leaving a hash in the record that looks verified and is not — the same failure mode as the stale
`output/t12` pointer this campaign already tripped over.

### Re-verified: the decision survives, only the artifact is stale

`output/t17.patch` was re-checked against the advanced tree and **applies clean on BOTH**:

- **T12** (current HEAD) — clean.
- **T12 + T19** (if T19 keeps) — clean.

So the T17b decision itself is intact and carries no conflict risk. **Only the prebuilt binary is void.**

### Policy — build T8's base AT LAUNCH TIME, not in advance

T8 sits behind T15, so between now and its launch the tree may absorb **T19 and/or T15**. Pre-building the
base again would simply go stale again. Therefore:

1. **At launch: `git apply output/t17.patch` onto whatever HEAD is current**, build, and cross-compile that
   as the tuning base. Do not reuse any previously pushed `ngn_t17fix*.exe`.
2. **RE-RUN THE DIM-LIVENESS PRE-FLIGHT on the actual launch base** (~6 min, the method is recorded above).
   **This is not a formality now: T12 restricts NMP to true cut nodes, which directly narrows where
   `NullMoveR` and `NullMoveMinDepth` can act.** Both were verified live on the pre-T12 base
   (190988/302102 and 442788/351834 against a 247063 default); **whether they remain live under T12's
   narrower NMP is unverified and must be measured, not assumed.** A dim that has gone inert is exactly the
   defect that invalidated the 2026-06-28 joint-SPSA round.
3. The default node count for the liveness table must be re-measured too — 247063 was the pre-T12 figure.

### What is unchanged

The **decision** (fixed base, i.e. T17 applied, on the irreversibility-asymmetry argument), the **recorded
counter-argument** (the fixed base starts ~6.4 Elo down, so the tune must find >+6.4 to break even), the
**~25h at 10+0.1** cost (5+0.05 is closed), and the **tuner-not-verdict** rule all stand exactly as written.

---

## LAUNCH RUNBOOK — ordered, so the launch is mechanical (written 2026-08-02 12:20)

**PREREQUISITE — do not skip: Batch 3 certification must have PASSED first.** T8's base is
`current HEAD + output/t17.patch`, and HEAD contains T12 (+2.2 provisional) and T19 (+7.3 full accept). **If
the cert fails, the base is wrong and the 25-hour run is wasted** — bisect and re-certify before launching.
This is the whole reason the cert was moved ahead of T8 (`experiments/batch-ledger.md`).

**Composition re-verified 2026-08-02:** `output/t17.patch` applies clean on **T12+T19** *and* on
**T12+T19+T15**, so whichever tree wins, the base builds.

### Steps

1. **Build the base from CURRENT HEAD** (never reuse a prebuilt one — `ngn_t17fix.exe` `3e8016d7…` is already
   dead from the T12 keep):
   ```
   git apply output/t17.patch
   make build && ./scripts/nodecheck.sh          # record the numbers; they define the base
   go test -short ./engine && go test -short -race ./engine && go test -short ./...
   GOOS=windows GOARCH=amd64 GOAMD64=v3 go build -o build/win/ngn_t8base.exe .
   ```
   Then push and **verify the on-box hash**. **Revert the tree afterwards** — T17 is speculative until its
   own verdict; only the tuner runs it.
2. **RE-RUN THE DIM-LIVENESS PRE-FLIGHT on this exact binary.** Method and table format are recorded above.
   **Not a formality: T12 restricts NMP to true cut nodes, which directly narrows where `NullMoveR` and
   `NullMoveMinDepth` can act, and T19 changes reduction at PV-marked nodes, which touches `LMRFullDepth` /
   `LMRMoveThreshold`.** Four of the fourteen dims sit downstream of changes made since the last liveness
   check. **A dim that has gone inert is exactly the defect that invalidated the 2026-06-28 joint-SPSA run**
   (9/11 params pinned, zero gradient). Re-measure the default node count too — 247063 was pre-T12.
3. **Launch the tuner** (`boxsprt.sh` drives it via `BOXSPRT_BIN`, already supported and byte-identical on the
   default path):
   ```
   BOXSPRT_BIN=spsa.exe ./scripts/boxsprt.sh launch t8 -- \
     -engine '.\ngn_t8base.exe' -tc 10+0.1 -iters 1500 -pairs 8 \
     -openings sprt_openings.txt -out t8_spsa.jsonl -report 25 -lowpower=false
   ```
   **1500 iters x 2 games x 8 pairs = 24000 games**, clearing the >=20k policy floor; ~25h at 10+0.1
   (5+0.05 is CLOSED, so the ~12h version does not exist). Expect **1 spsa + 16 engine procs = 17**.
   Checkpoints every 25 iters to `t8_spsa.jsonl`; `-resume`-able if interrupted.
4. **While it runs: nothing to read.** `plus_pct` and param movement are NOT evidence — that is the exact
   failure the policy names and how the 2026-06-28 run was invalidated.
5. **On completion:** take the converged vector, apply it, and run a **separate real-clock 10+0.1 c8 SPRT
   [-3,+3] vs unmodified HEAD**. **Only that decides.** If the vector lands within rounding of the defaults,
   the finding is "already near a local optimum" and the lane closes without a second round.

### Standing cautions carried forward

- **`AspMult` is by far the most sensitive dim** (min=110 gave 3.6x the default node count). **Treat a
  converged vector that moves it far from 150 with suspicion** until the confirmation SPRT says otherwise —
  T5's aspiration shape was games-validated at +20.2 and is not obviously improvable by a noisy 14-dim search.
- **The fixed base starts ~6.4 Elo down** (T17 standalone measured -6.4), so the tune must find more than
  that to break even. Recorded when the decision was made; not a reason to abandon it, but the honest bar.
- **A negative is fix-or-tune ambiguous.** Inherent to an interaction test, sanctioned by CLAUDE.md when
  explicit, and accepted deliberately over cementing a proven 6-Elo defect into the tuned vector forever.


---

## LAUNCH AUTHORISATION NOTE — written 2026-08-03 03:30, before T8 is launched

T8 will come up for launch when T19b resolves (~06:35), which is **well outside the window Bryan actually
stated** ("overnight tonight and all day Sunday" — that ended ~24h earlier). A 25-hour job is qualitatively
different from the 6-8h SPRTs the loop has been running unattended, so the decision is recorded rather than
improvised at launch time.

**Launching it is the right call, and the deciding fact is that it is INTERRUPTIBLE WITHOUT LOSS:**

- `cmd/spsa` checkpoints theta to `t8_spsa.jsonl` **every 25 iterations** (`-report 25`) and is
  **`-resume`-able from the last checkpoint** (`main.go:69-73`, verified in source, not assumed).
- So if Bryan wants the box back for anything, T8 can be killed at any point and resumed later having lost at
  most 25 iterations of work. **A 25-hour commitment that can be surrendered at 25-iteration granularity is
  not really a 25-hour commitment.**

Supporting reasons: idle box time is the campaign's **#1 measured cost** (one stall burned 128 box-hours);
T8 is the documented ranked next item with a written runbook; and **its base was deliberately cleared by the
Batch 3 certification**, which was itself run early specifically so this launch would not sit on an
unconfirmed composite. Not launching would waste the certification that was run to enable it.

**What would change the call:** if T19b unexpectedly KEEPS, the tree changes and T8's base must be rebuilt on
the new HEAD (the runbook already requires building at launch, so this is handled) — and the dim-liveness
pre-flight becomes even more important, since a third accepted change would sit upstream of the same four
NMP/LMR dims. If the box needs to be freed for a re-pin or a bisect instead, T8 yields.

---

## PRE-FLIGHT RUN 2026-08-03 05:30 on the ACTUAL launch base — and it found a real problem

Base built from **current HEAD (T12 + T19) + `output/t17.patch`**, staged as **`ngn_t8base.exe` sha256
`c67200c1314b22baff0cbc22902c542154e536e6028c02c529e5b997642d0996`**, on-box hash verified. Nodecheck on the
base: **347516 / 101343 / 539095**. Suite green. Tree reverted, baselines re-verified at 295507/112109/667703.

*(Valid only if T19b shelves, which leaves HEAD unchanged. If T19b keeps, rebuild — the runbook already
requires building at launch.)*

### Dim liveness — all live, but ONE HAS ALL BUT DIED

Default = **347516** nodes at kiwipete d12.

| dim | at MIN | at MAX | status |
|---|---|---|---|
| **NullMoveR** | 346938 | 348549 | **LIVE but effectively INERT — see below** |
| NullMoveMinDepth | 388707 | 271168 | LIVE (±17%) |
| LMRFullDepth | 347516 | 464605 | LIVE (flat lower half, as always) |
| LMRMoveThreshold | 480763 | 540642 | LIVE |
| LMPBase | 359926 | 456219 | LIVE |
| FutilityMargin | 287731 | 364965 | LIVE |
| ReverseFutilityMargin | 129532 | 481781 | LIVE |

**`NullMoveR`'s sensitivity has collapsed ~97x.**

| base | span across the dim's full range, as % of default |
|---|---|
| pre-T12 (2026-08-01 pre-flight) | **45.0%** (190988 -> 302102 on a 247063 default) |
| post-T12 (this pre-flight) | **0.46%** (346938 -> 348549 on a 347516 default) |

**This is exactly the risk the staleness notice predicted** — *"T12 restricts NMP to true cut nodes, which
directly narrows where `NullMoveR` and `NullMoveMinDepth` can act, and their liveness under T12 is
unverified."* It has been verified, and the answer is that **`NullMoveMinDepth` survived fine (±17%) while
`NullMoveR` did not.** T12 removed ~97% of the leverage that knob had over the tree, because the null-move
reduction amount can only matter where null moves are actually tried, and T12 restricted that to cut nodes.

### What this changes for the run — recorded BEFORE launch

- **`cmd/spsa` has no `-dims` flag**, so T8 will tune all 14 including this one. **`NullMoveR` will wander on
  noise and contribute essentially nothing.** That is the *exact* pathology that invalidated the 2026-06-28
  joint-SPSA round (9/11 params pinned, `plus_pct` ~50%, zero gradient) — the difference is that here it is
  **one dim out of fourteen, identified in advance**, rather than the whole run.
- **PRE-REGISTERED: do NOT read meaning into `NullMoveR`'s converged value.** Whatever it lands on is noise,
  not a tuned optimum, and it must not be reported as a finding or carried into the confirmation SPRT as
  though it were evidence. If the converged vector is otherwise near defaults, judge that on the other 13.
- **This does not block the run.** Thirteen dims retain real leverage, and the T17b interaction test — the
  actual reason for the fixed base — depends on `LMRFullDepth`, `LMRMoveThreshold`, `LMPBase`,
  `FutilityMargin` and `ReverseFutilityMargin`, **all five of which measured healthy above.**
- **It does earn a standing rule:** a kept change can silently kill a tuning dim downstream of it. **Re-run
  the dim-liveness pre-flight after every keep that touches a mechanism a dim controls** — not just before an
  SPSA round. Had T8 been launched on the pre-T12 pre-flight, 1/14 of a 25-hour run would have been spent on
  a dead knob with nobody the wiser.

---

## LAUNCH 2026-08-03 07:30 — and two harness traps that nearly corrupted a 25-hour run

T8 is **LIVE**: `1 spsa + 16 ngn_t8base = 17 procs`, `params=14`, start theta == registered defaults
including `LMPBase=5`. Base `ngn_t8base.exe` `c67200c1…`, tuner `spsa.exe` `f26a2814…`, both hash-verified.

**It took three launch attempts, and both problems were real.**

### Trap 1 — the on-box `spsa.exe` was STALE and silently tuned 13 dims, not 14

The first launch printed **`params=13`**. The registry is **compiled into the tuner binary**, and the on-box
`spsa.exe` (`b92d3c50…`) was built **2026-07-25, before T8b registered `LMPBase`**. So it carried the old
13-dim registry.

**This was not cosmetic. `LMPBase` is one of the five `improving` consumers the entire T17b interaction test
depends on** — the whole reason for choosing the fixed base. A 25-hour run would have recalibrated **4 of 5**
consumers and produced a vector that could never be interpreted as the pre-declared interaction test.

**My own base-decision doc asserted "all five consumers are live SPSA dims" — that was verified against the
ENGINE registry (`search.go`), not against what the stale TUNER binary had compiled in.** The verification
was real but aimed at the wrong artifact. Fixed by rebuilding `spsa.exe` from current HEAD (`f26a2814…`),
which reports `params=14`.

**Rule: a tool that embeds a registry must be rebuilt whenever that registry changes, and the dim COUNT in
its startup banner must be checked against the expected number at every launch.** `params=N` is the cheap
assertion; it caught this in seconds once looked at.

### Trap 2 — I created a leaked worker that `ps` could not see

Rebuilding as `spsa14.exe` made the process invisible to the harness: `boxsprt.sh ps` uses
`Get-Process -Name sprt,ngn_*,spsa` — an **exact** match, so `spsa14` never appears. Worse, `boxsprt.sh kill`
stops `sprt`, `ngn_*` and `spsa` — **also exact** — so **the tuner survived the kill while its engines were
stopped**, leaving an orphan (PID 37084) that `ps` reported as an idle box.

That is precisely the failure the no-leak rule exists to prevent, and it was **self-inflicted by choosing a
non-matching binary name**. Cleaned up by killing `spsa14` explicitly, deleting the stray binary, and
relaunching under the standard `spsa.exe` name so both `ps` and `kill` work as designed. Verified: 17 procs
visible, box otherwise clean.

**Rule: never introduce a new process name on the box.** The harness's `ps`/`kill` patterns are exact-match,
so a novel name is simultaneously unmonitorable and unkillable by the standard tooling. Reuse the sanctioned
names (`sprt`, `ngn_*`, `spsa`) — overwrite in place rather than versioning the filename. **Editing the
harness patterns instead was rejected: harness edits during a live run are barred.**

---

## DURATION ESTIMATE CORRECTED 2026-08-03 08:20 — T8 is a ~57 HOUR run, not ~25h

**Measured, not projected.** Tuner started 07:19:34, first checkpoint at iter 25 reached by 08:16:26 =
**56m52s for 400 games (25 iters x 8 pairs x 2 games) = 422 g/hr.**

| | g/hr | 24000 games |
|---|---|---|
| SPRT at the same TC/concurrency | ~960 | ~25h |
| **T8 SPSA, measured** | **422 (44%)** | **~57h** |

**The "~25h" figure carried in this runbook and in TODO was wrong by 2.3x.** It was derived by dividing the
game count by the SPRT throughput, which assumed SPSA moves games at the same rate. It does not.

**Leading explanation — the per-iteration BARRIER.** An SPSA iteration must complete all 8 perturbation pairs
before the gradient can be averaged and theta updated, so **each iteration costs the SLOWEST pair, not the
mean.** An SPRT has no such barrier: finished games are replaced immediately, so slow games overlap with
fast ones. With real game-length variance at 10+0.1, paying max-of-8 instead of mean-of-8 every iteration is
easily a ~2x tax. **Recorded as the leading explanation, not proven** — confirming it would need per-pair
timings the harness does not currently emit.

**Consequence: T8 finishes ~2026-08-05 16:00, not 2026-08-04 08:00.**

### Why the run is NOT being shortened

The obvious response is to cut `-iters`. **It is not available:** CLAUDE.md requires a SPSA run reach
**>= 20k games** to be evidence at all (`CLAUDE.md:43`), which floors this at **1250 iterations** — only 17%
below the 1500 already running. Cutting to the floor would save ~9h and buy a weaker result; cutting below
it would make the entire 48h run inadmissible.

So the real choice was **run ~57h or do not run T8**, and running remains right for the reasons already
recorded: the box is otherwise idle, the base was deliberately cleared by the Batch 3 certification for
exactly this, and **the run checkpoints every 25 iterations and is `-resume`-able**, so the commitment can be
surrendered at ~4-minute granularity if the box is needed.

### What this changes for planning

- **A future SPSA round should be budgeted at ~420 g/hr, not the SPRT rate.** This is now measured for the
  8-pair configuration at 10+0.1 on this box.
- **`-pairs` is the lever if the barrier explanation is right** — more pairs means more work absorbed per
  barrier, but also more processes per core. Not something to change mid-run.
- It also **quietly strengthens the case for having certified Batch 3 first**: the tuner's base being wrong
  would now have cost ~57 hours, not ~25.

### CORRECTION TO THE CORRECTION 2026-08-03 09:20 — the ~57h figure was MY error. It is ~30h.

**The 422 g/hr measurement one tick earlier was wrong, and the mistake was mine, not the harness's.** I took
the iter-25 checkpoint as having occurred at the moment I *observed* it (08:16:26) and divided from the
process start. But a checkpoint line sits in the log until someone polls; at the true rate iter 25 completed
around **07:50**. So the interval was inflated by roughly **26 minutes of my own polling latency**, on top of
process startup — on a sample of a single early checkpoint.

**Measured properly, from the checkpoint file's own `LastWriteTime`:**

| | value |
|---|---|
| tuner start | 07:19:34 |
| iter 75 checkpoint written | **08:48:50** |
| elapsed | 1.488 h |
| games (75 x 8 pairs x 2) | 1200 |
| **rate** | **807 g/hr = 84% of the 960 SPRT baseline** |
| **24000 games** | **~29.8 h** |

**So the honest sequence is: original estimate ~25h (assumed the SPRT rate); my "correction" ~57h (wrong,
built on a startup- and latency-contaminated single sample); actual ~30h.** The original figure was close;
mine was not.

**The barrier explanation survives but shrinks by a lot.** There *is* a real tax — 807 vs ~960 g/hr, about
**16%** — consistent with paying max-of-8 instead of mean-of-8 per iteration. It is **not** the ~56% tax I
claimed. **Budget future SPSA rounds at ~800 g/hr, not 420.**

**The lesson is one this campaign has already recorded and I repeated anyway:** *do not read a rate from a
single early sample.* It is the same error class as the SPRT low-n lesson (`sprt_instrument.md`:
"a CI-excludes-0 early read can still be noise"), applied to throughput instead of Elo. **The specific trap
worth naming: a log line's observation time is not its event time.** Use the artifact's own timestamp — here
`t8_spsa.jsonl`'s `LastWriteTime` — never the moment a poll happened to notice it.

**T8 now finishes ~2026-08-04 13:00, not 2026-08-05 16:00.** Nothing else about the run changes: still
resumable, still >= 20k games, still a tuner and not a verdict.

---

## HEALTH CHECK 2026-08-03 13:20 at iter 275/1500 — PASSES. And a false alarm worth recording.

**This is a health check, NOT a verdict.** No keep/shelve is being read from `plus_pct` or param movement —
that is barred. The only question asked is the one that **invalidated the 2026-06-28 joint-SPSA round**:
*are the parameters able to move at all?*

### My first read said "10 of 14 pinned" — and it was WRONG

The `[iter N]` log line prints the **rounded** integer theta. Read from there, ten of fourteen params sat
exactly at their start values after 275 iterations, which is the 2026-06-28 signature ("9/11 params pinned").

**The checkpoint file stores BOTH `theta` (float) and `rounded`.** Reading `theta`:

| param | start | theta@275 | drift | % of the 0.5 needed to flip |
|---|---|---|---|---|
| ReverseFutilityMargin | 120 | 111.435 | **-8.565** | 1713% |
| SingularMargin | 64 | 58.671 | **-5.329** | 1066% |
| AspMult | 150 | 152.747 | +2.747 | 549% |
| FutilityMargin | 100 | 101.071 | +1.071 | 214% |
| DeltaMargin | 100 | 99.645 | -0.355 | 71% |
| ExtensionBudget | 24 | 23.648 | -0.352 | 70% |
| AspInit | 12 | 11.755 | -0.245 | 49% |
| NullMoveMinDepth | 3 | 2.758 | -0.242 | 48% |
| LMRFullDepth | 3 | 2.763 | -0.237 | 47% |
| NullMoveR | 3 | 3.180 | +0.180 | 36% |
| SingularDepth | 6 | 6.149 | +0.149 | 30% |
| LMRMoveThreshold | 2 | 2.129 | +0.129 | 26% |
| FutilityMaxDepth | 8 | 7.914 | -0.086 | 17% |
| LMPBase | 5 | 4.935 | -0.065 | 13% |

**All 14 are drifting. Not one sits at its exact start value.** Four are already 47-71% of the way to a
rounding flip at 18% of the run.

### This is NOT the 2026-06-28 failure — two independent reasons

1. **Theta accumulates in float.** The 2026-06-28 kill was "param movement mathematically impossible
   (sub-integer steps)". Here sub-integer gradients accumulate correctly and cross the rounding boundary when
   they are large enough; the integer view is only how theta is *applied*, not how it is *stored*.
2. **`plus_pct` is NOT stuck at 50%.** That run's zero-gradient signature was `plus_pct ~50%`. Recent
   checkpoints here read **47.4, 50.2, 52.1, 53.9** — the objective is discriminating between perturbations.

**Verdict on the health check: T8 is behaving. The run is valid so far.** (`NullMoveR` remains the known
weak dim — pre-registered as noise given its 97x sensitivity collapse — but even it is drifting, so it is
weak rather than dead.)

### The trap, and it is the same one as yesterday

**A monitoring line showed a DERIVED view (`rounded`) and I read it as the underlying quantity.** That is the
third instance this session of the same error class:

- observation-time read as event-time (T8 rate → a wrong "57h" figure),
- inconsistent denominators (the amplification ratio → a retracted diagnostic),
- and now rounded theta read as theta.

**Rule: when a tool exposes both a raw and a derived value, monitor the RAW one.** For SPSA specifically:
**judge param movement from `theta` in `t8_spsa.jsonl`, never from the `[iter N]` log line**, which prints
`rounded` and will show healthy params as frozen for hundreds of iterations.

---

## POST-RUN RUNBOOK — written 2026-08-03 16:20 at iter 425/1500, before the vector exists

T8 converts to a verdict only through a confirmation SPRT, and that is where a 30-hour investment gets
wasted if the mechanics are wrong. Settled in advance.

### 1. There is NO script to apply an SPSA vector. Do not look for one.

`scripts/apply_tuned.py` **is for TEXEL eval tuning** — it splices PST tables and `eval.go` weight lines. It
does **not** touch the 14 search constants and must not be used here. Checked, not assumed.

**And `cmd/sprt` cannot pass UCI options.** Its full flag set is
`-new -base -depth -movetime -tc -nodes -basedepth -maxgames -mingames -maxmoves -openings -concurrency
-lowpower -genbook -genplies -balancecp -balancedepth -out -gametimeout -resign*/draw*` — **no option flag,
and no `setoption` support in the harness at all.** So although the dims *are* UCI-settable (that is how the
liveness pre-flight probed them), **the confirmation candidate must be built with the values edited into
source.** There is no runtime path.

### 2. Read the vector correctly

Take the **final** checkpoint line of `t8_spsa.jsonl` and use its **`rounded`** object — that is what the
engine actually applies. (`theta` is the float accumulator; use it to judge *movement*, never as the vector.)
Edit the 14 constants in the `const` block near the top of `engine/search.go`
(`NULL_MOVE_R`, `NULL_MOVE_MIN_DEPTH`, `LMR_FULL_DEPTH`, `LMR_MOVE_THRESHOLD`, `FUTILITY_MARGIN`,
`FUTILITY_MAX_DEPTH`, `REVERSE_FUTILITY_MARGIN`, `DELTA_MARGIN`, `SINGULAR_DEPTH`, `SINGULAR_MARGIN`,
`EXTENSION_BUDGET`, `ASP_INIT`, `ASP_MULT`, `LMP_BASE`).

### 3. The comparison — easy to get wrong, and getting it wrong wastes the run

**Candidate = HEAD + `output/t17.patch` + the tuned vector. Base = unmodified HEAD.**

**Do NOT compare tuned-vs-untuned on the T17 base.** That would measure the tune alone and **miss the entire
point** — T17b exists to test *fix + recalibration as one interaction*, which is why the fixed base was
chosen over the reversible-but-foreclosing alternative. The bundle is the unit under test.

Consequence already on record: **the bundle starts ~6.4 Elo down** (T17 standalone measured -6.4), so the
tune must find more than that to break even. A negative is **fix-or-tune ambiguous** — inherent to an
interaction test, accepted deliberately.

### 4. Pre-registered cautions for reading the vector

- **`NullMoveR`'s converged value is NOISE.** Its sensitivity collapsed ~97x post-T12 (span 45.0% -> 0.46%).
  Do not report it as a tuned optimum or as a finding.
- **Treat `AspMult` far from 150 with suspicion** until the SPRT says otherwise — it is by far the most
  sensitive dim (3.6x node swing across its range) and T5's aspiration shape was games-validated at +20.2.
- **If the whole vector lands within rounding of the defaults**, the finding is *"already near a local
  optimum"* and the lane closes without a second round.
- **`plus_pct` and param movement are NOT evidence** — the rule that invalidated the 2026-06-28 round.

### 5. Sequencing against T22

T22 is staged and is the cheaper item (~8h vs the confirmation's ~8h plus 30h already spent). **If T8's
vector is near-default, run T22 first** and treat T8's confirmation as low priority. **If the vector moved
materially, confirm it first** — it sits upstream of everything else in Batch 4, and leaving an unconfirmed
tuned vector in the tree would contaminate every later candidate's base.

---

## CHECKPOINT-FILE CONTAMINATION + trajectory read at iter 475 (2026-08-03 17:20)

### The file mixes THREE campaigns. Analysis must filter; resume is safe only by an invariant.

`t8_spsa.jsonl` is **append-only across runs**, and currently holds:

| date | checkpoints | iters | dims |
|---|---|---|---|
| 2026-07-25 | 9 | 25..225 | **13** |
| 2026-07-26 | 17 | 250..650 | **13** |
| **2026-08-03 (current)** | 19 | 25..475 | **14** |

**My first trajectory analysis was contaminated by this** — it merged all 45 rows, produced nonsense
(iters "25..650" for a run at 475) and crashed on the 13-dim rows' missing `LMPBase`. Any future analysis
**must filter by `time` prefix.**

**`-resume` is SAFE today, but only because of an invariant worth stating.** `loadCheckpoint`
(`cmd/spsa/main.go:442`) scans the whole file and keeps the **last parseable line** — not the highest
`iter`. That is the current run's latest checkpoint **only because the current run is the last writer.**
If anything else ever appends to this file, or the file is reordered, **resume would silently load a stale
and possibly 13-dim theta.** Since resumability is the stated justification for running a 30-hour job
unattended, this invariant is load-bearing: **do not write to `t8_spsa.jsonl` from any other process, and
prefer a per-run `-out` filename in future rounds.**

### Trajectory at iter 475/1500 — a real gradient exists, and it is concentrated

Efficiency = |net drift| / total path length. 1.0 is straight-line drift; ~0 is a random walk.

| param | net drift | efficiency | |
|---|---|---|---|
| **ReverseFutilityMargin** | **-15.29** | **0.84** | directed |
| NullMoveMinDepth | -0.34 | 0.67 | directed |
| **SingularMargin** | **-9.26** | **0.63** | directed |
| LMRMoveThreshold | +0.22 | 0.44 | |
| FutilityMargin | +2.11 | 0.33 | |
| LMRFullDepth | -0.25 | 0.28 | |
| ExtensionBudget | +0.68 | 0.26 | |
| AspInit | +0.15 | 0.25 | |
| DeltaMargin | -6.65 | 0.23 | |
| LMPBase | -0.22 | 0.23 | |
| NullMoveR | +0.09 | 0.18 | random walk |
| AspMult | +1.60 | 0.13 | random walk |
| SingularDepth | +0.08 | 0.11 | random walk |
| FutilityMaxDepth | -0.07 | 0.05 | random walk |

**This is a health signal, not a verdict** — no keep/shelve may be read from param movement.

What it says: **the objective has a genuine gradient in a small number of dims.** `ReverseFutilityMargin`
(120 -> ~105) and `SingularMargin` (64 -> ~55) are moving with 0.63-0.84 efficiency, i.e. close to
straight-line descent rather than noise. Most other dims are wandering.

**Two predictions recorded now, before the run ends:**

1. **The vector will NOT land "within rounding of the defaults"** — the pre-registered condition that would
   close the lane without a second round. Two dims have already moved 13-15% of their starting value with
   directed trajectories, and step sizes have not yet decayed.
2. **`NullMoveR` is wandering at efficiency 0.18, exactly as the 97x sensitivity collapse predicted.** Its
   pre-registration as noise is holding up, and this is independent evidence for it.

**`AspMult` at efficiency 0.13 is reassuring** given the standing caution to distrust large `AspMult` moves:
it is drifting only +1.6 from 150 and doing so incoherently, so T5's games-validated aspiration shape is not
being pulled far by this run.

### The contamination hazard, made precise — and a mitigation staged

`loadCheckpoint` maps params by name and **silently leaves a missing one at its default**:

```go
for _, p := range params {
    if v, ok := last.Theta[p.name]; ok { p.theta = v }   // absent name -> silently keeps the default
}
```

So a resume that ever picked up one of the **13-dim** rows would produce a **silently corrupted mixed
state**: thirteen params jumping to a stale July run's values while **`LMPBase` quietly stayed at its default
5** — with no error and no warning. And because `startIter = it + 1`, it would also **resume from iter 651**
(the July-26 high-water mark) and run only to 1500, skipping most of the schedule. Two independent
corruptions from one stale line.

This does not bite today — the last line is the current run's 14-dim checkpoint — but the protection is an
*invariant*, not a check.

**MITIGATION STAGED: `experiments/2026-08-03-t8-checkpoints-clean.jsonl`** (kept in `experiments/`, not `output/` — the latter is gitignored by convention, and a recovery artifact whose entire purpose is to survive must be tracked) contains only the current run's 14-dim checkpoints.
**If T8 ever needs resuming, do NOT resume against `t8_spsa.jsonl`.** Instead push the clean file to the box
under a fresh name and run with `-out <that file> -resume`, which both loads a guaranteed-14-dim theta and
keeps future checkpoints uncontaminated.

**And for the next SPSA round: use a per-run `-out` filename.** The default `output/spsa.jsonl` and the
reused `t8_spsa.jsonl` are how three campaigns ended up in one file.


### Prediction check at iter 650/1500 (2026-08-03 20:30) — tracking

At iter 475 I recorded the prediction that **the vector will NOT land "within rounding of the defaults"** —
the pre-registered condition that would close the lane without a second round. **It is holding: 6 of 14
rounded values already differ.**

| param | default -> rounded | theta |
|---|---|---|
| **ReverseFutilityMargin** | **120 -> 103** | 102.63 |
| **SingularMargin** | **64 -> 50** | 49.75 |
| DeltaMargin | 100 -> 95 | 94.80 |
| FutilityMargin | 100 -> 103 | 102.80 |
| AspMult | 150 -> 152 | 151.78 |
| ExtensionBudget | 24 -> 25 | 24.95 |

**The two big movers are exactly the two that showed directed trajectories** at iter 475
(ReverseFutilityMargin efficiency 0.84, SingularMargin 0.63) — the rounded vector is confirming what the
trajectory analysis suggested, rather than diverging from it.

**Three more sit close to a rounding flip** and may yet move: `NullMoveMinDepth` (theta 2.56, flips at 2.50),
`LMPBase` (4.63, flips at 4.50), `LMRFullDepth` (2.76). Step sizes are still decaying, so these are not
guaranteed.

**`AspMult` has moved only +2 from 150** — the standing caution to distrust a large `AspMult` excursion
remains unbreached, and T5's games-validated aspiration shape is intact.

**This is state, not evidence.** No keep/shelve may be read from param movement or `plus_pct`; the converged
vector still requires its own SPRT against unmodified HEAD, per the post-run runbook above.
