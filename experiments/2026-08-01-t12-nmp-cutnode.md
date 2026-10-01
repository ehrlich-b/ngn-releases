# 2026-08-01 T12 — NMP restricted to true cut nodes — BUILT, GATED, STAGED

**STATUS: built, gated, staged on the box with the on-box hash verified. NOT LAUNCHED.** Runs after T4e.
Engine tree is REVERTED and clean; nodecheck re-verified at exactly 346662/149587/765656 from a fresh build.
Change lives only in `output/t12-nmp-cutnode.patch` (sha256
`26b3d9c5d05848e6e054b610e9c94f9944101baf83e7d09d76c7b8496f75db9f`).

Supersedes the 2026-07-25 pre-verification in `experiments/2026-07-25-t12-nmp-cutnode.md`, whose mechanism
and confound findings are carried forward below and still stand.

## Premise RE-VERIFIED against HEAD (not trusted from the queue)

TODO's pointer to a shelved `output/t12` patch was **stale — no such file exists**, so the change was
re-derived from source. Both halves of the premise re-checked at HEAD:

- The NMP gate (`search.go:1482`) reads `... && canNull && !isPV && staticEval >= beta && ...` — **no
  cut-node condition**.
- `cutNode` is **already plumbed**: parameter of `alphaBetaPV` (`search.go:1177`), propagated as `!cutNode`
  at 1501/1568/2002 and `!isPV && !cutNode` at 1915, and already consumed by IIR at 1362.

So the candidate is literally `&& cutNode` inserted — a genuine one-line behavior change.

**A detail worth recording: the comment directly above that gate already says "Null-move pruning at cut nodes
only."** It has been inaccurate since the C3 fix — `!isPV` admits **both** cut nodes and all nodes. T12 makes
the code match the claim the comment has been making all along.

## Why the mechanism is sound

In PVS, a non-PV node is either a cut node (expected to fail high) or an all node (expected to fail low). NMP
assumes *"this node is probably already >= beta, so let the opponent move twice and check"*. **At an all node
that premise is contradicted by the node's own expectation**, so the null search is speculative work against
the search's own prediction. Restricting to `cutNode` removes the class of NMP attempts whose justifying
assumption does not hold.

## Gates

### Nodecheck — PASSES, and reproduces the 2026-07-25 measurement exactly

| position | base | T12 | delta |
|---|---|---|---|
| kiwipete d12 | 346662 | **451141** | **+30.1%** |
| mid d12 | 149587 | **136100** | **-9.0%** |
| end d16 | 765656 | **796906** | **+4.1%** |

**Byte-for-byte identical to the numbers recorded on 2026-07-25** (451141/136100/796906), which independently
confirms the re-derived one-liner is the same change that was pre-verified then. Measured from a fresh build
both sides. The **mixed direction is encouraging** — a pure slowdown would grow every position.

### The T18b soundness question — PASSES, in the strongest possible form

*Does the change persist speculative information?* **It does the opposite: it REMOVES a speculative prune**
from the node class where the prune's premise is known to be wrong. Nothing is cached, nothing is stored, no
heuristic guess gains durability. This is the structural inverse of T18b.

### Depth at fixed nodes — -2 net plies, NOT a rejection

| position | base d | T12 d | delta |
|---|---|---|---|
| quiet-mg-QGD | 14 | 14 | 0 |
| quiet-mg-closed | 12 | 12 | 0 |
| lateMg-ph6-11 | 13 | 12 | **-1** |
| najdorf-mg | 14 | 14 | 0 |
| rook-eg-R4P | 17 | 16 | **-1** |
| pawn-eg | 32 | 32 | 0 |

Under the **2026-08-01 narrowing** of the ebfprobe rule, a depth loss rejects only **depth-BUYING**
candidates — those that prune or reduce MORE and therefore claim depth (T6a, T20). **T12 prunes LESS by
construction**, so it claims soundness and pays in depth; the loss is its expected signature, exactly as for
corrhist (the kept T4c measures -1) and ttPv. Recorded as the size of the bill, not as a verdict: **-2 is
mild**, a third of T4e's -6, and 4 of 6 positions are flat.

### Suite

`go test -short ./engine`, `-race ./engine`, `./...` — all green with the patch applied. No new test: the
change is a one-line gate tightening with no new state, and the existing NMP tests plus nodecheck cover it.

## Mechanism evidence carried forward from 2026-07-25 (still valid — HEAD is unchanged in this region)

- **Proven to fire via a temporary direct probe: 473 NMP attempts blocked solely by the new gate at kiwipete
  d12.** Narrow (~3.1% of eligible NMP sites are all-nodes) but high-leverage (~220 nodes per blocked
  cutoff).
- **METHODOLOGY NOTE, keep this: `nmtry` is a CONFOUNDED witness for this change** — it went **UP**
  10850 -> 14796 under a strictly tighter restriction, because blocked all-nodes expand fully and their
  children are cut nodes, so the tree both grows and gets enriched in the node type where NMP still fires.
  The per-node rate rose too (3.13% -> 3.28%). **Absolute or rate counters cannot witness a restriction that
  changes tree composition — use a blocked-attempt probe.** Same trap as T20's `seeq`.

## PRE-REGISTERED EXPECTATION AND FAILURE HYPOTHESIS — written before launch, outcome unknown

**Honest expectation: +2 to +6, materially BELOW the +9.7 [S #230] prior, and a null is a live outcome.** Two
independent reasons to discount the prior, both registered here:

1. **The mechanism touches only ~3.1% of NMP sites.** A +9.7 mined from another engine's changelog was
   measured in that engine's calibration, not against NGN's already-guarded NMP (which carries the W8
   fail-low guard and the non-pawn-material zugzwang guard).
2. **kiwipete +30.1% at fixed real clock buys soundness with depth, which is exactly how T9a lost.** The
   -9.0% middlegame delta is the encouraging half; the +30.1% is the headwind.

**The base-rate warning belongs on the record too: this is a node-level pruning change, and that family is
1 keep / 11 attempts for +2.0 Elo total.** T12 is the best-motivated member left (a soundness argument rather
than a constant tweak, and it makes the code match its own comment), but the family prior is poor and this
candidate does not escape it.

**Leading failure hypothesis if T12 rejects:** the blocked NMP attempts at all nodes were *cheap and mostly
correct anyway* — an all-node that fails low costs little whether or not NMP was tried, so removing the
attempt buys back less than the +30.1% tree growth costs at real clock. **That would be evidence about NMP
node-type restriction generally, not about this constant**, since there is no constant here to blame — which
makes a null unusually informative for this lane.

**What a rejection would NOT license:** it would not reopen `DoubleExtMargin`-style constant hunts, and it
would not by itself close NMP polish (the eval-margin-scaled R variant is a separate, untested sub-item).

```yaml
id: 2026-08-01-t12-nmp-cutnode
date: 2026-08-01
change_class: search heuristic (one-line pruning-gate restriction)
hypothesis: >
  Restricting NMP to true cut nodes removes null searches whose justifying premise is contradicted by the
  node's own expectation. Expect +2 to +6, below the +9.7 prior.
base_commit_or_patch: HEAD (clean tree, nodecheck 346662/149587/765656 re-verified from a fresh build)
candidate_commit_or_patch: output/t12-nmp-cutnode.patch sha256 26b3d9c5d05848e6e054b610e9c94f9944101baf83e7d09d76c7b8496f75db9f
base_binary_sha256: 0a8f65b75e59c376052f83b14787b0af2eba65552cedfee6f70ce576891ca721   # on-box ngn_t5.exe
candidate_binary_sha256: 2081f5ac11095672c85a569bb24af407b8481076a1222e6d9716fe5fa10db4c4  # on-box ngn_t12.exe, hash verified after push
harness: sprt.exe sha256 30c33e0512725b7f552d8a1cf72ba6f1e0deb4dcb122c6b6ba8f6433c686a762 (M5-validated mill, untouched)
command: sprt.exe -new .\ngn_t12.exe -base .\ngn_t5.exe -tc 10+0.1 -concurrency 8 -lowpower=false -openings sprt_openings.txt -elo0 -3 -elo1 3 -alpha 0.05 -beta 0.05 -maxgames 8000 -mingames 300 -resignscore 900 -resignplies 5 -drawscore 10 -drawplies 10 -drawminplies 80
machine: AMD Ryzen 7 9800X3D 8c/16t, native Windows (LAN box 192.168.4.108)
tc: 10+0.1 (seconds)
concurrency: 8
openings_path: sprt_openings.txt
openings_sha256: 974e4b5ab871a9e106d0c766bfa39fc83676222337fb7702f61782e2ac5b3222
adjudication: STANDARD-ON (M5-validated flags)
decision_rule: >
  H1 accept (pLLR >= +2.94) = KEEP. H0 accept (pLLR <= -2.94) = SHELVE. At the 8000g cap a PROVISIONAL keep
  requires point estimate >= +1; below +1 shelves regardless of pLLR sign. Verdict from the harness H0/H1
  line plus a full post-mingames envelope scan, NEVER the printed final pLLR alone. HALT if flag-outs are
  non-zero on either side.
next_action: launch after T4e resolves; base stays on-box ngn_t5.exe (do NOT pool with T4e -- one behavior change at a time)
```

## Base note

T12's base is on-box `ngn_t5.exe` `0a8f65b7…`, the same base T4e uses. **If T4e KEEPS, T12 must be rebuilt on
the T4e tree before launching — do NOT pool.** If T4e shelves (the more likely outcome per its own
pre-registration), this base holds unchanged.

---

## PRE-REGISTERED TIE-BREAK — written 2026-08-02 02:25 at G5529, OUTCOME UNKNOWN

T12 is converging on the **exact boundary of its own decision rule**. Trajectory of the point estimate:
**+7.4 (G659) -> +4.8 (G1678) -> +3.0 (G2514) -> +1.6 (G3476) -> +1.3 (G3674) -> +0.9 (G4664) -> +1.3
(G5529)**, i.e. classic regression toward the mean, now oscillating either side of the `>= +1` provisional
floor with ~2500 games to run.

**This is exactly the situation where a rule gets quietly reinterpreted to suit the number that lands.**
Recording the reading in advance so it cannot be.

### The tie-break, binding

1. **The decision statistic is the PENTANOMIAL point estimate, not the trinomial.** The running log line
   shows trinomial elo; the `RESULT` block prints both and labels penta *"THE decision stat"*. They differ —
   for T4e they agreed (-20.9/-20.9), for M6-retest they agreed (-4.3/-4.3), but agreement is not guaranteed
   and the penta figure governs regardless of which is more favourable.
2. **`>= +1.0` is INCLUSIVE.** A penta point estimate of exactly `+1` is a PROVISIONAL keep. `+0.9` shelves.
   No rounding in either direction, and no appeal to the trinomial if it happens to sit on the other side.
3. **A bound crossing overrides everything.** pLLR `>= +2.94` = full KEEP; `<= -2.94` = SHELVE. The `>= +1`
   floor only applies to a run that reaches the 8000g cap undecided.
4. **The verdict comes from the harness's own `H0/H1 ACCEPTED` line plus a full post-mingames envelope scan**,
   never the printed final pLLR alone — c8-drain has now been observed **6 times**, in both directions.
5. **No "it was positive for most of the run" argument.** T12 held +2.5 to +7 for its first ~2000 games; that
   is what regression to the mean looks like and it carries no evidential weight. Equally, **no "it decayed so
   it must be a null" argument** if it lands `>= +1` — the rule is the rule in both directions.

### If it lands PROVISIONAL (penta >= +1)

It becomes **Batch 3 row 1** (Batch 3 currently has 0 rows, cert due <= 2026-08-08) and is committed to main
as a provisional keep — **the first non-negative result since T5 on 2026-07-19**. Then, and only then, the
**T12 x T19 non-additivity finding becomes live**: the composition measured -14.8/-25.1/-12.8% where each
change alone grows kiwipete, so if T19 also keeps their composition needs an explicit interaction check and
Batch 3's certification is where a non-additive loss would surface
(`experiments/2026-08-01-queue-composition-check.md`). **T19 and T15 would need rebuilding on the new tree —
already verified to apply and build clean, so this is mechanical.**

### If it lands SHELVED (penta < +1)

The miss streak goes to **10**. That is a genuinely notable number and should be recorded as such rather than
softened: at the historical ~35% hit rate, p(10 straight) = 0.65^10 ~= **0.013**. The instrument is still not
the suspect — Batch 2 positive-controlled it mid-streak at +35.1 vs +26.8 claimed — so the reading would be
about candidate quality and about the node-level family, which would go to **1 keep / 12 attempts**.

**Either way the run order is unchanged: T19 next, then T15.**

## POST-VERDICT PROCEDURE — written 04:17 at G7336, outcome still unknown

Both branches written out in advance so the verdict tick is mechanical and nothing is dropped. A keep fires
**five** bookkeeping steps at once, and the easy ones to forget are the nodecheck re-lock and the re-measure
cadence.

### If PROVISIONAL KEEP (penta >= +1.0)

1. **Commit** `output/t12-nmp-cutnode.patch` to main. Verify the rebuilt tree hashes back to the exact
   playing binary `ngn_t12.exe` `2081f5ac…` — never assume, the T8 provenance note records that a
   behaviourally-identical rebuild is not necessarily byte-identical.
2. **RE-LOCK the nodecheck baselines** — `scripts/nodecheck.sh:117-119`, currently
   `346662 / 149587 / 765656`, become **`451141 / 136100 / 796906`**. This is the step most likely to be
   missed, and a stale baseline silently mis-gates every subsequent candidate.
3. **Batch ledger** — `experiments/batch-ledger.md`, Batch 3 row 1 (currently "none yet"). Batch 3's
   certified base is `172e9e1` / `ngn_cert2.exe` `0a8f65b7…`; cert due **<= 2026-08-08**.
4. **Re-measure cadence** — ebfprobe + the FMC table after every structural keep (~4 min, zero box time;
   harness `output/fmc-table.sh`). Six weeks of drift is what let the "lane exhausted" story stand while the
   lane was delivering.
5. **Launch T19 on the NEW base**, already prepared so this is one command:
   `-new .\ngn_t19on12.exe -base .\ngn_t12.exe`, with `ngn_t19on12.exe` sha256
   **`6a560c98cee9902f0ff5ca469ffade44b7c206bbe7e608b20ff9c764be277fb3`** pushed and on-box hash verified,
   built from T12+T19 (composition verified to build with green `-short`/`-race` suites).
   **Then the T12 x T19 non-additivity finding is live** — the composition measures -14.8/-25.1/-12.8% where
   each change alone grows kiwipete, so if T19 also keeps their pairing needs an explicit interaction check
   (`experiments/2026-08-01-queue-composition-check.md`).

### If SHELVED (penta < +1.0)

1. Nothing to revert — **the engine tree never carried the change**; the patch is shelf-only.
2. **Do NOT touch the nodecheck baselines.** They stay `346662 / 149587 / 765656`.
3. Record the result here and in TODO; the miss streak becomes **10** (p ~= 0.013 at the historical rate) and
   the node-level family becomes **1 keep / 12 attempts**. State both plainly rather than softening them.
4. **Launch T19 on the UNCHANGED base**: `-new .\ngn_t19.exe -base .\ngn_t5.exe`, `ngn_t19.exe`
   `c61c91ae…` already staged and hash-verified. `ngn_t19on12.exe` becomes dead weight — harmless, leave it.
5. The leading failure hypothesis pre-registered above (blocked all-node NMP attempts were cheap and mostly
   correct anyway, so removing them buys back less than the +30.1% tree growth costs) becomes the recorded
   reading, and it is **evidence about NMP node-type restriction generally, not about a constant** — there is
   no constant here to blame, which makes this null unusually informative.

**Either branch: verify topology (1 sprt + 16 engines) and all on-box hashes after launching.**

---

# RESULT 2026-08-02 — **PROVISIONAL KEEP, penta +2.2 [-3, +8]. First non-negative since T5 (2026-07-19).**

```text
=== RESULT (8h19m21s) ===
Games: 8000   W-D-L: 2184-3683-2133   score: 50.3%
Elo(new - base): +2.2   95% CI [-5, +10]
LLR: +1.63   bounds [-2.94, 2.94]
Pentanomial [LL 235  LD 948  {LW,DD} 1598  WD 969  WW 250] over 4000 pairs
Penta Elo: +2.2   95% CI [-3, +8]   pLLR +1.83  (THE decision stat)
Flag-outs (lost on time): new 0, base 0  of 8000 games  (goal: 0)
Adjudicated early: 4303 decisive, 1809 draw  of 8000 games
Verdict: INCONCLUSIVE (ran out of games — add openings or raise -maxgames)
DONE_EXIT_0
```

- Post-mingames envelope **pLLR [-0.64, +1.83]**, elo [-4.0, +12.4]. **Zero samples at/beyond either bound**,
  so this is a clean capped-undecided run — no c8-drain ambiguity because there was no crossing to drain.
- 0/0 flag-outs over 8000 games, 962 g/hr.

## The pre-registered tie-break, applied verbatim

Written at G5529 with the outcome unknown, because T12 was oscillating around its own decision boundary:

1. **Decision statistic = PENTANOMIAL.** Penta = **+2.2** (trinomial also +2.2 — they agreed this time, but
   the rule bound the penta regardless).
2. **`>= +1.0` inclusive.** +2.2 clears it.
3. No bound crossing, so the `>= +1` floor governs — which is exactly the capped case it was written for.
4. Harness line is `INCONCLUSIVE (ran out of games)` + `DONE_EXIT_0`, i.e. reached the cap undecided.
5. **The "it decayed so it's really a null" argument was pre-barred and is not being made.** The point
   estimate did decay (+7.4 -> +1.6 -> +0.9 -> +2.2 final), which is what regression to the mean looks like;
   the rule was written to be applied in both directions and it is.

**CLAUDE.md batch-path conditions, all three met:** point estimate **+2.2 >= +1**; **LLR > 0** (pLLR +1.83);
**no regression signal** (envelope min pLLR -0.64, elo min -4.0, 0/0 flag-outs). ⇒ **PROVISIONAL KEEP.**

## Post-verdict procedure executed

1. **Committed to main** (`c010ca7`). Suites green: `-short ./engine`, `-race ./engine`, `./...`.
2. **Nodecheck baselines RE-LOCKED** in `scripts/nodecheck.sh` to **451141 / 136100 / 796906** (from
   346662 / 149587 / 765656), with the reason recorded in the file header. Verified passing after the edit.
3. **Hash-back check: BYTE-IDENTITY IS STRUCTURALLY UNACHIEVABLE — this is a real process finding, not a
   failure to verify.** The rebuild gave `5c6cb162…` against the playing binary's `2081f5ac…`. Cause
   established rather than assumed: **Go embeds VCS stamps by default**, so the commit hash is baked into the
   binary and HEAD advanced (doc commits) between build and verdict. Proven by building twice with
   `-buildvcs=false`, which is **deterministic and reproducible** (`21a18179…` both times) and differs from
   the stamped build. **Therefore the "hash-back == playing binary" step in this project's procedures can
   only ever pass when rebuilt at the identical commit and dirty state.** What is verifiable — and what was
   verified — is **behavioural identity**: the rebuilt tree reproduces T12's gate nodecheck
   (451141/136100/796906) exactly. **Future run records should claim behavioural identity via nodecheck, or
   build with `-buildvcs=false`, rather than asserting a byte match that the toolchain prevents.**
4. **Re-measure cadence run** (required after every structural keep):
   - **ebfprobe vs the pre-T12 base: -2 net plies** (0 / 0 / -1 / 0 / -1 / 0) — reproduces T12's gate
     measurement exactly, confirming the shipped tree is the gated one.
   - **FMC table on the new HEAD:** kiwipete 88.99, QGD 86.19, closed 84.66, najdorf 84.84, rook-eg 80.99
     (was 88.78 / 87.54 / 83.76 / 85.12 / 80.76) — **net -0.29 pp, flat.** Expected and reassuring: T12 is a
     pruning restriction, not an ordering change, so ordering quality should not move. **These are the new
     FMC reference numbers for future ordering work.**
5. **T19 launched on the new base** — `-new .\ngn_t19on12.exe -base .\ngn_t12.exe`, both hashes verified
   on-box, topology confirmed 1 sprt + 8 + 8. **Box idle gap 34 minutes.**

## What this result means, stated carefully

**The 9-run miss streak is broken**, and it is broken by the candidate whose pre-registered expectation was
"+2 to +6, materially BELOW the +9.7 prior" — **the outcome landed inside the predicted range**, which is the
first time this campaign a pre-registration has been calibrated rather than optimistic.

**But it is a PROVISIONAL keep at +2.2 with CI [-3, +8], not a demonstrated win.** The interval includes
zero. The batch-certification path exists precisely because single capped results at this magnitude cannot be
distinguished from noise individually; **only Batch 3's powered [0,+6] SPRT against `172e9e1` can confirm
it**, and that is where it must be judged.

**Node-level family: 2 keeps / 12 attempts.** Still a poor base rate, but no longer 1/11 — and notably the
keep is the member argued from *soundness* (removing a prune whose premise is contradicted at all-nodes)
rather than from constant-tuning, which is consistent with the campaign-wide pattern that mechanism-level
changes keep and constant tweaks do not.

**The T12 x T19 non-additivity finding is now LIVE** (`experiments/2026-08-01-queue-composition-check.md`):
T19 is running on top of T12, and their composition measured -14.8/-25.1/-12.8% where each alone grows
kiwipete. If T19 also keeps, the pair needs an explicit interaction check, and Batch 3 certification is where
a non-additive loss would surface.
