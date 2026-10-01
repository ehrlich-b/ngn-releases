# 2026-08-03 T22 — probcut restricted to true cut nodes — BUILT, GATED, STAGED

**Final status, recovered 2026-09-04: SHELVED.** The run completed August 4:
8000 games, W/D/L 2108/3768/2124, penta -0.7 [-6,+4], final pLLR -0.62,
post-minimum pLLR envelope [-1.50,+1.58], zero flags/errors, `DONE_EXIT_0`.
The predeclared +1 floor was not met. Patch remains unapplied.
Log: `output/recovery-2026-09-04/t22_out.txt`. The preparation status below is historical.

**STATUS: built, gated, staged, on-box hash verified. NOT LAUNCHED** — T8 owns the box until ~2026-08-04
13:00. Engine tree reverted; nodecheck re-verified at 295507 / 112109 / 667703. Patch
`output/t22-probcut-cutnode.patch`.

## Premise verified against HEAD

Probcut gates at `search.go` on `!isPV && !inCheck && depth >= 5 && abs(beta) < MATE_VALUE-100` — **no
cut-node condition**, exactly as NMP was before T12. `cutNode` is already a parameter of `alphaBetaPV` and in
scope. The candidate is `&& cutNode` inserted: one line.

## HOW THIS CANDIDATE IS JUSTIFIED — and a correction to my own first framing

**My initial framing was "T12's argument applied to probcut." On inspection that parallel is WEAKER than it
looked, and the honest framing is different.**

- **T12's argument was about a contradicted premise.** NMP assumes *"this node is already >= beta"*; at an
  all node the search's own expectation contradicts that, and NMP is unsound in principle (zugzwang), so the
  attempt is speculative work against the node's own prediction.
- **Probcut's speculation is about DEPTH, not node type.** A successful probcut has *proved* `score >=
  probcutBeta > beta` via a real depth-4 verification search. That proof is equally valid at an all node —
  it is speculative only because the verification is shallow. **So restricting probcut removes some CORRECT
  prunes, not merely wrongly-premised ones.** The T12 parallel does not carry.

**The honest mechanism is T18a's, at 60-90x the magnitude.** T18a (probcut TT short-circuit) was killed at
the gate on 2026-07-25 because it saved only **0.1-0.15%** of nodes and was result-identical — re-classed as
a pure speed change that never warranted a games gate. **T22 is the same idea — skip probcut attempts that
will mostly fail — but it saves 9-11%**, because at an all node the node itself is expected to fail low, so
the verification searches there mostly come back negative and the work is wasted.

**Crucially, unlike T18a, T22 is NOT behaviour-identical** (nodecheck moves, and it loses the all-node
probcuts that would have succeeded), **so it does require a games gate.** The bet is that the wasted
verification searches outweigh the correct prunes lost.

## Gates

### Nodecheck — PASSES

| position | base (T12+T19) | T22 | delta |
|---|---|---|---|
| kiwipete d12 | 295507 | 295995 | +0.2% |
| mid d12 | 112109 | 99829 | **-11.0%** |
| end d16 | 667703 | 607707 | **-9.0%** |

kiwipete is below the 1% bar; mid and end clear it decisively, so the gate passes. **The tree SHRINKS**,
consistent with removing wasted verification searches rather than removing prunes.

### Fire rate — measured AT GATE TIME (the rule T19b earned)

| position | probcut-eligible sites | blocked by `cutNode` | share |
|---|---|---|---|
| kiwipete d12 | 11508 | 2538 | **22.1%** |
| mid d12 | 5170 | 1241 | **24.0%** |
| end d16 | 16732 | 6381 | **38.1%** |

**22-38% of all probcut attempts happen at all nodes** — a far larger share than T12's ~3.1% of NMP sites.
In absolute terms it is still a small fraction of the tree (0.15-0.43% of nodes), because probcut is
comparatively rare.

**No amplification ratio is quoted.** That diagnostic was **retracted the same day** as ill-defined — its
value for this very candidate spans 150x (71x vs 0.46x) depending on an unstated denominator
(`experiments/2026-08-03-amplification-ratio.md`).

### The T18b soundness question — PASSES in the strongest form

*Does it persist speculative information?* **No — it removes a speculative prune.** Nothing cached, nothing
asserted, no heuristic gains durability. Same answer as T12's, the strongest available.

### Depth at fixed nodes — -1 net ply, NOT a rejection

0 / +1 / -1 / 0 / -1 / 0 across the six probe positions. **-1 net, exactly matching the kept-T4c reference.**
T22 prunes *less*, so it is accuracy-buying and a depth cost is its expected signature under the 2026-08-01
narrowing. Not a rejection signal; also, per T20, a depth *gain* could not have promoted it either.

### Suite

`go test -short ./engine`, `-race ./engine`, `./...` — all green. Edit region gofmt-clean (`search.go` has
pre-existing struct-alignment diffs unrelated to this change).

### Binary

`ngn_t22.exe` **`28fb2c1fb491d75550b45bd3dfe2c92205fda265fd800625d0276f48f9e4186d`**, pushed, on-box hash
verified. **Independently re-verified after a process incident**: rebuilding from a freshly-restored tree
reproduced that hash exactly, confirming the staged binary is clean T22 with no debug instrumentation.

## PRE-REGISTERED EXPECTATION AND FAILURE HYPOTHESIS — before launch, outcome unknown

**Honest expectation: +1 to +4**, i.e. at or below T12's +2.2 despite blocking a much larger *share* of its
mechanism's sites. Reason: **probcut is rare** (0.7% of nodes are eligible) where NMP is common, so T22's
absolute footprint is smaller even at 22-38% blocked.

**Leading failure hypothesis: the all-node probcuts that succeed are worth more than the failed attempts
cost.** A successful probcut at an all node is a genuine proved bound and prunes a whole subtree; a failed
attempt costs one shallow verification search. If the hit rate at all nodes is even moderate, the arithmetic
inverts and T22 loses. **This is the specific way the weakened T12-parallel could bite**, and it is why the
expectation is set at or below T12 rather than above it.

**What a rejection would NOT license:** it would not weaken T12 (a separate mechanism with a genuinely
contradicted premise, kept and certified), and it would not reopen the probcut *margin* (`probcutBeta =
beta + 200`, flat where SF scales it improving-aware) — that remains a separate, deliberately-low-ranked
item from T18b's record.

```yaml
id: 2026-08-03-t22-probcut-cutnode
date: 2026-08-03
change_class: search heuristic (one-line pruning-gate restriction)
hypothesis: >
  Restricting probcut to true cut nodes removes verification searches that mostly fail, because an all node
  is expected to fail low. Saves 9-11% of the tree. Expect +1 to +4.
base_commit_or_patch: HEAD = T12+T19 (nodecheck 295507/112109/667703, re-verified from a fresh build)
candidate_commit_or_patch: output/t22-probcut-cutnode.patch
base_binary_sha256: 6a560c98cee9902f0ff5ca469ffade44b7c206bbe7e608b20ff9c764be277fb3   # on-box ngn_t19on12.exe / ngn_cert3.exe
candidate_binary_sha256: 28fb2c1fb491d75550b45bd3dfe2c92205fda265fd800625d0276f48f9e4186d  # on-box ngn_t22.exe, hash verified
harness: sprt.exe sha256 30c33e0512725b7f552d8a1cf72ba6f1e0deb4dcb122c6b6ba8f6433c686a762
command: sprt.exe -new .\ngn_t22.exe -base .\ngn_t19on12.exe -tc 10+0.1 -concurrency 8 -lowpower=false -openings sprt_openings.txt -elo0 -3 -elo1 3 -alpha 0.05 -beta 0.05 -maxgames 8000 -mingames 300 -resignscore 900 -resignplies 5 -drawscore 10 -drawplies 10 -drawminplies 80
machine: AMD Ryzen 7 9800X3D 8c/16t, native Windows (LAN box 192.168.4.108)
tc: 10+0.1 (seconds)
concurrency: 8
openings_sha256: 974e4b5ab871a9e106d0c766bfa39fc83676222337fb7702f61782e2ac5b3222
decision_rule: >
  H1 accept (pLLR >= +2.94) = KEEP. H0 accept (pLLR <= -2.94) = SHELVE. At the 8000g cap a PROVISIONAL keep
  requires PENTA point estimate >= +1.0 inclusive; below +1 shelves regardless of pLLR sign. Verdict from the
  harness H0/H1 line plus a full post-mingames envelope scan, never the printed final pLLR. HALT on any
  flag-out.
next_action: >
  Launch when T8 resolves (~2026-08-04 13:00) OR sooner if T8 is surrendered. Batch 4 row 1 candidate. If any
  keep lands first, rebuild on the new tree -- do NOT pool.
```
