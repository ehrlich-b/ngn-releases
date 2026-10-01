# Qsearch capture-budget / TT contract repair

Status: **ACCEPTED September 5 under the predeclared correctness rule.** The
fixed candidate match completed; strength is inconclusive. The frozen artifacts
are now deployed and smoke-verified on WSL/Windows/Mac. The bounded A/A investigation passed its
predeclared rule; the first failed preflight is retained. Active objective
remains at least 2800 without NNUE.

## Evidence and selected repair

The [second review](2026-09-04-review-second-pass.md) reproduces an actual qsearch
lower-bound store from an insufficient capture budget. Kiwipete, absolute ply5,
window [−INFINITY,100]: qDepth5 stores score100/depth0/LowerBound at the root;
warm qDepth0 returns100 with zero nodes; cold qDepth0 returns51. An overlay
removing only the six-ply capture cap yields51 for all three calls.

The selected repair removes the arbitrary six-ply **production** capture cap.
The existing stack ceiling, legal evasions, draw/mate precedence, time/node/stop
checks and selective capture rules remain. This makes qsearch's depth0 TT entries
independent of that omitted capture budget. Classical Counter 3.8 is a source
reference for uncapped capture search; this is not a new chess technique.

No quiet promotions, TT-age fix, T17/T8 vector or new eval weights are bundled.
The conservative offline Texel filter retains its explicit unresolved-horizon
rejection: it is a different contract and does not share the production TT.

An alternative restricting q stores to full-budget roots also repairs the probe,
but is not established as cheaper. Fixed-depth diagnostics for uncapped versus
full-budget-store alternatives were +53.4/+26.8% Kiwipete nodes, +23.5/+52.5%
middlegame nodes, and ~0/−1.3% rook-ending nodes. These are cost/behavior examples,
not strength verdicts or grounds to select by favorable score. The source-based
choice here is uncapped qsearch; retained q TT stores are tested as part of it.

Terra implements/tests in `output/qcap-repair-20260904` at `4986330`. Root reviews,
controls integration, freezes artifacts, launches games and decides deployment.
Require the actual warm/cold reproduction to fail on the old runtime and pass
on the repair, both-color/state/stop/terminal coverage, full short/race and vet.

## Fresh A/A preflight: r0904qaa

The immediate accepted baseline is `ngn_20260904_castle.exe`, playing source
`70eb527`, SHA-256
`131b9bbf1cb064bd6c1edd5513db8e6c670eba154231f3aadd66cc1942a85e7d`.
The native box was verified idle after the completed castling release deployment.

Fixed **200 games / 100 opening pairs**, 10+0.1, concurrency8, native Windows
9800X3D, default affinity, lowpower=false. Both sides use that identical baseline.
Require zero operational failures/flags and a paired 95% interval containing0;
an unexpected asymmetry blocks the candidate pending investigation. No score-based
extension or early stop. This is a fresh preflight for the new baseline binary.

```text
sprt_20260904.exe -new .\ngn_20260904_castle.exe -base .\ngn_20260904_castle.exe -tc 10+0.1 -concurrency 8 -lowpower=false -openings sprt_openings.txt -elo0 -3 -elo1 3 -alpha 0.05 -beta 0.05 -maxgames 200 -mingames 201 -resignscore 900 -resignplies 5 -drawscore 10 -drawplies 10 -drawminplies 80
```

## Candidate game gate: r0904qcap

After source proof, tests, passing A/A, artifact hashes filled in and idle box:
fixed **400 games / 200 opening pairs**, otherwise the same setup as A/A.
Candidate runtime alone versus the immediate accepted castling baseline.

**Launched September 5:** the bounded A/A investigation passed. Native process
inspection confirms one versioned harness and eight engines per side, with the
scheduled task removed. Frozen candidate hash was verified immediately before
launch. No candidate result is claimed until the fixed run completes.

```text
sprt_20260904.exe -new .\ngn_20260904_qcap.exe -base .\ngn_20260904_castle.exe -tc 10+0.1 -concurrency 8 -lowpower=false -openings sprt_openings.txt -elo0 -3 -elo1 3 -alpha 0.05 -beta 0.05 -maxgames 400 -mingames 401 -resignscore 900 -resignplies 5 -drawscore 10 -drawplies 10 -drawminplies 80
```

Predeclared correctness gate: require proved budget/TT consistency and all
regressions/checks, plus zero flags, illegal/missing moves, crashes, disconnects
or watchdogs. A significantly negative paired interval rejects this implementation
pending investigation; the demonstrated bug remains open. An inconclusive
non-regression result may support the proved correction, but is not H1 or an
Elo-gain claim. No extending the cap on score sign and no adding the self-play
estimate to the last absolute2726 rating. No candidate defaults are installed
before acceptance.

Frozen harness SHA-256:
`63ef86d9b986fdb2c260782149af259e278845d89595d010df3970e6698cc9ed`.
Frozen 5,000-opening file SHA-256:
`974e4b5ab871a9e106d0c766bfa39fc83676222337fb7702f61782e2ac5b3222`.
Candidate source: `49863301ab7a37f5e06f2f9dbf0ca3571b08e206` plus the
`engine/search.go` diff saved as `output/recovery-2026-09-04/qcap-runtime.patch`,
SHA-256 `a3d84a2c3bd9a2554faad9d86e51aa61d5f72783e8ab68cc1f6d2417bd28b72d`.
The accompanying test changes do not enter the playing binary.

Frozen Go1.26.2 artifacts:

- Windows amd64/v3 `build/ngn_20260904_qcap.exe`:
  `bc84fe0d298a32ff644775b2892ee43356bd94f9e22b6a527dc99cdbb29556f2`.
- macOS arm64 `build/ngn_20260904_qcap`:
  `24d71a1ac381320d2ab790b1a88a7fdb30761bcedf573f9b98973b5a1e7eb702`.
- Linux amd64/v3 `build/ngn_20260904_qcap_linux`, built after commit `ec9d264`
  from the identical runtime, prepared locally but not installed or WSL-tested:
  `372c896b7c8ed23454d99e7315e4a960d7030131293cc3a5fe54314f75342867`.

## Implementation validation

The new warm/cold regression fails against the parent runtime (warm100/cold51)
and passes after repair for the position and its vertical/color flip, at incoming
legacy qDepth1/5/6/9. Every call checks restored board/hash/tag and all game-history
counts. A separate live capture-tree case at qDepth9 proves node-stop propagation
and state restoration. Existing terminal/clock tests remain green.

Root integration: full short suite and vet pass; independent Stockfish oracle
passes 2,293 positions / 48,926 divides / 128 raw search roots. Terra's identical
isolated implementation passed the full short/race suite and vet; those final
agent checks reported directly rather than saving standalone logs. Root logs:
`qcap-short.txt`, `qcap-vet.txt`, `qcap-oracle.txt`; parent failure:
`qcap-parent-fail.txt`, all under `output/recovery-2026-09-04/`.

Frozen Mac artifact passes the black-EP, illegal-castling and fifty-move-mate
UCI regressions and clean quit. Two fresh-process repetitions agree exactly in
nodes, score, PV and best move on all three canonical roots (d12/12/16):
463172 / 138436 / 858070 nodes. Saved in `qcap-determinism.json`. This proves
repeatability, not strength or node identity with the baseline.

## Completed A/A: preflight failed

`r0904qaa` completed exactly 200 games / 100 pairs in 12m39s, DONE_EXIT_0:
54/110/36 W/D/L; penta 2/18/49/22/9; paired estimate +31.35, 95% CI
[+0.67,+62.53] (rounded harness display [+1,+63]); pLLR +0.77. Zero flags or
operational errors. The predeclared interval must contain zero, so this is
**FAIL**, even though its SPRT footer says inconclusive. Candidate is staged
and hash-verified but has not played or replaced a default.

Raw log SHA-256:
`0a8c39e1cd75046648fc45346dba53b18ca00bafc3959804827d00e65dfabcaf`.
Audit: `output/recovery-2026-09-04/r0904qaa-audit.json`. All 200 consecutive
game counts and outcome reasons checked; independently recomputed paired mean
and interval from the reported penta. The log does not preserve per-game pair IDs,
so this is not independent reconstruction of the pair assignment.

The binary and harness hashes still match, workers are idle, and source review
finds both colors enqueued once per opening, shared pair IDs, mover-relative
result conversion and color-relative clocks. No asymmetry has been proved.
A conditional pair-label sign-flip calculation gives two-sided p=0.06277 under
the symmetry/independent-pair assumptions. That shows sensitivity of this
borderline result to the approximation; it does **not** change the failed gate
or prove the rig symmetric.

### Bounded accounting diagnostic: r0904qnodesaa

Before any additional real-clock preflight, run exactly 24 games / 12 pairs
at 10,000 nodes per move, c8, same baseline both sides, frozen harness/book and
adjudication. Require all twelve pairs to be centered (one win each or two draws),
zero failures, DONE_EXIT_0. A failure triggers state/accounting diagnosis; success
only checks deterministic symmetry and cannot certify the real-clock setup.
This is a different diagnostic, not an extension or replacement of `r0904qaa`.

```text
sprt_20260904.exe -new .\ngn_20260904_castle.exe -base .\ngn_20260904_castle.exe -nodes 10000 -concurrency 8 -lowpower=false -openings sprt_openings.txt -elo0 -3 -elo1 3 -alpha 0.05 -beta 0.05 -maxgames 24 -mingames 25 -resignscore 900 -resignplies 5 -drawscore 10 -drawplies 10 -drawminplies 80
```

Completed in 2s, DONE_EXIT_0: 10/4/10 W/D/L; penta 0/0/12/0/0. All twelve
pairs centered exactly; no operational failures. **Accounting diagnostic PASS.**
Raw log SHA-256: `67036269388c51dff66a7bebd10e1a09786aaba893d2af209ca0987c2c17ccf7`.
Together with the source/hash inspection, this finds no deterministic label or
pairing defect in the sampled games; clock-dependent asymmetry remains unresolved.

### One fixed real-clock replication: r0904qaa2

Predeclared after that diagnostic and **before launching the replication**:
exactly 400 games / 200 pairs with the original real-clock A/A setup and identical
baseline both sides. Restart the processes; retain all original 200 games.
No further automatic replication if the result fails. This is an investigation
prompted by a failed preflight, not the original run extended until it passes.

To clear the candidate hold, require zero operational failures in both runs,
the replication's paired 95% interval containing zero, and the interval computed
from **all 600 games / 300 pairs** containing zero. Report both runs and the
combined diagnostic, explicitly retaining the first failure. This conditional
investigation is not a new independently preplanned significance test. If either
interval excludes zero, keep candidate games held and investigate scheduling or
time-dependent engine state; do not keep sampling until a favorable result.

```text
sprt_20260904.exe -new .\ngn_20260904_castle.exe -base .\ngn_20260904_castle.exe -tc 10+0.1 -concurrency 8 -lowpower=false -openings sprt_openings.txt -elo0 -3 -elo1 3 -alpha 0.05 -beta 0.05 -maxgames 400 -mingames 401 -resignscore 900 -resignplies 5 -drawscore 10 -drawplies 10 -drawminplies 80
```

Launch verified: one versioned harness and sixteen expected baseline processes;
scheduled task entry deleted. At replication launch, `r0904qcap` was unlaunched. Root
source is committed at `ec9d264`; the frozen Windows/Mac binaries above retain
their original build identity. The active default remains the castling release.

### Completed investigation: PASS, original failure retained

`r0904qaa2` completed 400 games / 200 pairs in 25m28s, DONE_EXIT_0:
94/207/99 W/D/L, penta 9/48/90/45/8, paired −4.343 [−25.984,+17.264],
pLLR −0.21. Zero flags or operational errors. Combining **all** 600 games
gives penta 11/66/139/67/17, +7.529 [−10.220,+25.317]. Both required
intervals contain zero, so the predeclared investigation clears the candidate
hold. This does not erase the original failed preflight or turn the conditional
replication into an independently preplanned test. No more A/A games are needed.

Replication log SHA-256:
`c705472b6ed1abdc0d193f8832239468ee722dc70473c39acb4d5ef65f734fb4`.
Independent result audit: `output/recovery-2026-09-04/qaa-investigation-audit.json`.
The completion watcher fetched the log and verified the native workers idle.

## Completed candidate: correctness ACCEPTED, strength inconclusive

`r0904qcap` completed exactly 400 games / 200 pairs in 25m32s, DONE_EXIT_0:
106/184/110 W/D/L, penta 13/48/82/44/13; paired −3.474 [−27.360,+20.378],
pLLR −0.14. Zero flags or operational errors. Reasons: 216 adjudicated wins,
88 adjudicated draws, 48 draw-rule, 47 max-moves and one stalemate.

The mechanism proof, regression/oracle/short/race checks and frozen-artifact
tests pass; the interval is not significantly negative. This meets the stated
rule for accepting the proved correction. It is not H1, not a demonstrated
Elo gain, and does not establish equivalence within a narrow margin. No extra
games or favorable-sign extension were added. The last absolute pin stays2726.

Root independently checked all 400 consecutive counts, one-result increments,
reasons, flags, footer and paired calculation. Raw log SHA-256:
`db5202d04b04f51165e69c7242f4ce1b9bd6f99f35363449d3087b052bbc5518`.
Audit: `output/recovery-2026-09-04/r0904qcap-audit.json`. As with A/A, the log
lacks per-game pair IDs; this is not an independent pair-assignment replay.
The watcher fetched the completed result and verified native workers idle.
