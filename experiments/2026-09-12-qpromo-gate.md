# Direct quiet promotions on the Counter baseline

```yaml
id: 2026-09-12-qpromo-gate
date: 2026-09-12
change_class: search/eval heuristic
hypothesis: Including direct quiet promotions in qsearch improves the accepted Counter-backed engine at real clocks.
base_commit: b7ffdc886a56516af673243cae2dbd7597115cc0
candidate_commit: uncommitted base plus production patch 171cfea254797ec0f756ed3503ea295ec4a5175bf88bd486a0cbb2009870bbb7
base_binary_sha256: 86ef7473fa5f53e4fdd800f550f4fe3f182db09056afacbad98cf174d88554bc
candidate_binary_sha256: 3a0300324e3f1727f62d6d64b88e382075a7b6f33381ec76f4bb42468aca55a2
harness_commit: frozen Fastchess and audit sources listed by the run manifest
command: python3 /home/ehrli/repos/ngn-qpromo-gate-20260912/qpromo_gate_driver_v2.py /home/ehrli/repos/ngn-qpromo-gate-20260912/qpromo_gate_manifest_v2_approved.json
machine: Ryzen 7 9800X3D, Windows WSL Ubuntu, physical-core representatives 0/2/4/6
go_version: go1.25.5
goarch_goamd64: linux/amd64/v3, CGO_ENABLED=0, GOMAXPROCS=1 per engine
tc: 10+0.1
concurrency: 4
openings: existing audited 200 unique six-ply development opening prefixes, sequential and reversed colors
openings_sha256: 2257ecfb9af03f1ee0bff68f71ec97dbe363cdb9f535fab1a0ebc5af1abaaf33
aa_preflight: PASS100 games, 50 pairs, identical base binary and match configuration; candidate admitted
decision_rule: complete AA100 with paired95 interval including0; then complete candidate400; adopt only if candidate paired-bootstrap lower95 Elo is greater than0 and every integrity gate passes; otherwise shelve; no score-conditioned extension or rerun
games_or_pairs: AA100/50 then candidate400/200
result: candidate118W/178D/104L; penta[9,46,80,52,13]; paired-bootstrap+12.17[-11.30,+34.86]Elo
flags_errors: zero; full legal/terminal and strict operational audits pass
verdict: SHELVE at the fixed400-game cap; positive point estimate does not qualify
next_action: retain the accepted engine unchanged and test the exact-order Counter output-kernel speed hypothesis
```

The same behavior source is now accompanied by rating-documentation commit
`6b72088`. No rating games are added in this experiment, and its relative result
will not be added arithmetically to the historical classical rating.

## Scope and prior proof

The [earlier witnesses](2026-09-05-quiet-promotion-witness.md) expose a qsearch
move-class omission, including mirrored quiet-promotion mates. This experiment
tests the playing value of the small direct-generator patch on current source.
It appends Q/R/B/N quiet promotions to the existing captures and widens the
local move/score arrays from 64 to 96. Existing delta/SEE pruning and evaluator
transition policy remain intact. A witness is not itself an Elo verdict, so
this run uses the stricter positive-gain acceptance rule above.

Sol implemented the rebase in `/home/ehrli/repos/ngn-qpromo-candidate-20260912`.
Root reviewed the production diff and regression coverage. Tests include both
colors, capture/quiet promotion mixtures, blocked and diagonally pinned pawns,
all 32 possible quiet-promotion variants, a deterministic legal walk, exact
mate scores, and search-stack restoration. The Counter regression uses active
nonzero pawn and promoted-piece feature weights, checks full-refresh parity on
committed frames, and verifies exact restoration of the starting raw value.

Final source passed short engine tests (7.481 s), engine race tests (39.759 s)
and the full short suite. Both binaries use the same build flags and toolchain.
Root read the final logs. The immutable build/test receipt is:

`/home/ehrli/repos/ngn-qpromo-candidate-20260912/output/qpromo-receipt/qpromo-build-test-receipt-20260912.json`

SHA-256 `aa709d5e9e6e63a1e27bcc7a3fae66e8143ec25f114010cab36f9cdca5141d5a`.
It binds the production patch, test patch, source files, commands and logs.

## Frozen protocol

Run root: `/home/ehrli/repos/ngn-qpromo-gate-20260912/run-001`.

- Driver SHA-256: `917b8550307fc213ce1997ad91d264bca2ca064b090bb4fda2f06d4fd8fc7eb0`.
- Held manifest SHA-256: `a6e5807a1df671ec52e3ee36515e7c589c6a294de9cf1b9629727ed819570fa6`.
- Released manifest SHA-256: `7f74b75abec5a0f3adc18ad0e1c42b1ce9b7a21d403c15a0170600a13296fd17`.
- Canonically copied run manifest SHA-256: `89e3523f49f8237d7bdc021e131263bc1fa52be89333ba789d7fae99d40f976c`.
- Fastchess SHA-256: `e61220f6651599b2a40a7bd1d7535cf59be8e3eaa1f74dd2f6d5dc2fb1c0bc00`.
- Counter model SHA-256: `3488baed71f4d432d028e05b65e2ff13f8c9afe7f91974d1a3c34a0691d6670c`.
- Opening PGN SHA-256: `1ae6a65a0224d7a4ff893a3c999542cd3d05e9a7f0e989fe2deee1cff7b8bf5a`.
- Hash 128 MiB, Threads 1, OwnBook false, Move Overhead 100 ms per engine.
- Match seed 20260912; paired percentile bootstrap seed 2026091201,
  100,000 resamples of complete opening pairs.
- A/A uses the first 50 openings; candidate uses all 200. A/A is a control,
  not candidate selection or tuning, so this overlap is intentional.
- Candidate is engine A in the independent audit; base is engine B.
- No adjudication. Every played move, natural terminal outcome, opening pair,
  protocol trace and process identity is independently checked.
- A 10-second prelaunch CPU sample must show at most one non-owned core of
  activity. During each match, 12 consecutive five-second samples above that
  threshold reject timing evidence. Existing unrelated jobs are preserved.
- Base and candidate binaries are copied and hash-checked before A/A starts.
  Source patches, build receipt and audit tools are frozen alongside them.
- Failed or incomplete runs have no strength verdict. The fixed schedule is
  not extended after its score is observed.

Root and an independent Sol reviewer corrected the initial scoring orientation,
checked the frozen module interfaces and provenance, and approved this exact
driver/held-manifest pair before launch. Both real UCI preflights passed. Root
authorized changing only the manifest's release state and launching once.

The original exact driver and run manifest remain in run 001. The output directory is immutable and
the script refuses to rerun into it. A detached launch was rejected before it
started. After fresh 96–100% idle CPU observations and explicit task/resource
review, the foreground WSL launch was approved and started once. The driver
then passed its own prelaunch CPU check and began A/A. No timing games were
started by the rejected attempt.

## Run 001: operational failure before candidate admission

The supervisor terminated A/A after 77.297 seconds when sampled process-tree
RSS reached 4,196,856 KiB, exceeding its 4,194,304-KiB limit. Seven control games
had finished; no candidate game started. The receipt records
`sampled-process-tree-memory-limit`, no timeout or monitor error, and zero
surviving descendants. These incomplete control games have no verdict and are
not reused.

Eight NGN processes accounted for 4,155,648 KiB at the peak, versus 29,944 KiB
for the Python stage and 9,856 KiB for Fastchess. The cap was undersized for the
resident engine processes; `Hash=128` limits the transposition table, not the
whole Go process. The preserved earlier accepted Counter/HCE run reached
751,108 KiB in an individual engine. Eight such processes alone project to
5.73 GiB, leaving little overhead under a 6-GiB cap. The current host had
13.10 GiB available after cleanup. Non-owned CPU activity stayed below 0.13 core
in all recorded samples.

Root authorized a **memory-only operational amendment to 8,388,608 KiB** and a
fresh full A/A followed by the original fixed candidate schedule. Engine bytes,
model, options, opening schedule, clocks, CPU affinity, seeds and decision rule
remain unchanged. Run 001 and its exact source/manifest remain preserved.
The amendment is based on resource receipts, not the incomplete control score.

## Run 002: memory-only amendment

The [tracked driver](2026-09-12-qpromo-gate.py) now matches v2 exactly; its only
code difference is the memory limit. The [tracked manifest](2026-09-12-qpromo-gate-manifest.json)
is the canonically frozen run-002 copy. Original v1 files remain unchanged on
WSL. The reviewed amendment receipt binds run 001's supervisor, memory samples,
CPU observations and prior accepted-engine memory evidence.

- Driver SHA-256: `9c35fe89ef99a854deba2919e230af4decf6d7f93e55cb10c96444e8c51b2455`.
- Held v2 manifest SHA-256: `68070eb184132e0818a98aa1dac3951c4b67405d7f16fb7874f40d8c95dd0c27`.
- Approved v2 manifest SHA-256: `12fa10771a3b101f7b1613f613eeb230114f417928f9c684ed7dc9594c1e7839`.
- Canonically frozen v2 manifest SHA-256: `0fde74caace183f2c95c2b1c160833b7083c7c5e16c82a3fe4a9619edba09dae`.
- Resource amendment SHA-256: `a672be9ff284171c5dcd79ee69f770fab438d26635bfde029220d3ba20e404c8`.

Root reviewed the one-line driver diff and launched v2 once in the foreground.
It uses a new immutable `/home/ehrli/repos/ngn-qpromo-gate-20260912/run-002`
directory and starts the full A/A schedule again. The engine comparison and
its fixed statistical gate are unchanged.

### Completed control; candidate admitted

Run 002 completed all 100 control games / 50 pairs in 835.755 seconds:
22 wins, 52 draws, 26 losses for base A; penta `[1,18,18,10,3]`.
The predeclared paired-bootstrap interval includes zero. Independent legal
replay passed all 15,196 plies and all natural terminal outcomes; strict
operational audit passed 490,480 trace lines. The supervisor returned zero,
with no termination, timeout, monitor error or surviving descendants. Peak
sampled process-tree RSS was 5,259,180 KiB, below the amended 8-GiB cap.
The frozen runner admitted the original 400-game candidate schedule. Its
result was pending at this checkpoint; the control is not strength evidence for
the patch. Its final paired-bootstrap interval is −13.90 [−59.64,+31.35] Elo.

### Completed candidate: shelve

The fixed 400-game / 200-pair comparison completed in 3,355.578 seconds with
118 wins, 178 draws and 104 losses for candidate A (51.75% score). Penta is
`[9,46,80,52,13]`. The predeclared 100,000-resample paired bootstrap gives
**+12.17 [−11.30,+34.86] Elo**. The lower bound is not positive, so the exact
rule requires **SHELVE**. There is no score-conditioned extension, provisional
keep or claim that this proves the change is harmful or worthless.

Independent Stockfish replay passed all 62,848 played plies and all 400 natural
terminal results. The strict operational audit passed 2,018,027 trace lines;
there were no flags, crashes, illegal moves or no-move results. The supervisor
returned zero with no termination, timeout, monitor error or survivors. Peak
sampled process-tree RSS was 5,637,896 KiB. The foreground SSH command also
returned zero. The complete decision and hash inventory are preserved in the
[artifact directory](2026-09-12-qpromo-artifacts/).

Only documentation, measurement code and the intentionally archived experimental
patches enter Git. The production and regression diffs are saved as `.patch`
files, not installed engine source. The accepted playing engine and deployment
remain unchanged. The isolated WSL candidate worktree and both raw run roots
are preserved; no unrelated jobs or user files were removed.

### Independent terminal review and preservation repair

Sol's independent checker reconstructed each engine-A score from the game
headers and result, mapped games back to their reversed-color opening pairs,
and reproduced both 100,000-resample bootstrap intervals exactly. It verified
the complete 92-file inventory and terminal/decision digest chain, build and
input identities, legal/terminal, operational and process receipts. Neither
stage had even one sampled interval above one non-owned CPU core (168 A/A and
671 candidate samples); prelaunch activity was 0.03699 core. A separate live
process scan found no remaining run-owned processes. Root reviewed the checker,
its final diff and complete result, and verified the archived file identities.

The first two executed review versions failed on a preservation assumption:
the driver hash-checked all manifest inputs before launch but did not copy the
two memory-amendment provenance receipts into run 002. The original external
`resource_amendment_receipt` and `run001_supervisor_receipt` both still matched
their exact prelaunch hashes. Final v4 explicitly permits only those two named,
path-and-hash-bound external records and reports their origin. Both receipts
and failed reviews are archived alongside the result. No frozen run file,
statistical rule, game, or original receipt changed; no games were rerun.

- [Final review](2026-09-12-qpromo-artifacts/independent-review.json), SHA-256
  `9a358146b1b4301545c475306bf2f2f4471cdc2a5c2a5619c90002fc8f39be78`.
- [Exact v4 verifier](2026-09-12-qpromo-artifacts/verify-terminal.py), SHA-256
  `05ebd5a82e6aac7a1df4ea6bbbb47b954b0a637c5de8de197da3f96b275dba70`.
- Terminal SHA-256: `8f4781b0fb57dbd05ac846338bb9b0522b8e90c5bf9fc8cb6d49a331987e46b0`.
- Decision SHA-256: `c81ec9b28a0ed25d1cf87f6aa295373eb597c8fba8080d50391b74a3b4b0e66c`.
- Inventory SHA-256: `e8d4159bedc519321611d7077f3f3b6a43dae0eb80da6d16c1b047542efa1c04`.

To repeat the read-only verification on WSL, invoke the archived verifier with
`--run-root /home/ehrli/repos/ngn-qpromo-gate-20260912/run-002` and a fresh
`--output` path outside that frozen run. It refuses execution off WSL or to
overwrite an existing result.

## Next experiment selected before this verdict

A bounded read-only Sol assessment identified the Counter output loop in
`countereval/context.go` (`evaluateAccumulator`) as the next focused target.
The [September 6 profile](2026-09-06-current-status.md#numerical-and-performance-evidence)
attributed 33.56% flat CPU to scalar evaluation before the accepted feature-row
AVX2 optimization; the output loop itself remains scalar. That is evidence for
a promising target, not a current measured speedup or an Elo prediction.

Try an **exact-order AVX2 output kernel**: vectorize positive-lane comparison
and separately rounded float32 multiplication, then accumulate active products
in their original ascending order with scalar additions. Keep the scalar
implementation as the oracle. Horizontal reassociation and FMA are forbidden;
the earlier real FMA witness changed expected `+0` to `-2^-46`.

Before any acceptance, require exact raw bits on official full/incremental and
adversarial finite inputs, signed-zero/subnormal/rounding cases, all eight-lane
activity masks, zero kernel allocations, fixed-search node/score/PV identity,
and GNU disassembly confirming vector multiplication without fused operations.
Then use reciprocal-order direct-kernel timing and the existing six-fixture
fixed-node search protocol. Shelve on any parity failure, direct-kernel speedup
below 1.5x, whole-search equal-weight median gain below 3%, or any fixture median
slower than base. A repeatably faster, behavior-identical implementation uses
the repository's pure-speed gate; it does not need an Elo claim to justify
integration. No implementation or benchmark for this second candidate began
while the quiet-promotion verdict was pending.
