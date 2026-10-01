# NGN-owned NNUE: Sol implementation and WSL run plan

Status: planning handoff, not a launch receipt. Prepared against NGN `5910fae`
on `research/n1-sf18-feasibility-20260920`. No training, labeling, installation,
match, source implementation, or hopper mutation was performed for this plan.
The user will resume with Sol. This document supersedes the earlier decision to
defer owned-network development; historical experiment receipts remain intact.

## 1. Decision and finish line

Train a randomly initialized, king-bucketed NGN network using the established
`(768 × 4 mirrored -> 768) × 2 -> 8 material heads` shape. Reuse the verified
Rodent V1.2 feature/update/kernel work through an explicitly separate owned-model
contract, not by replacing its pinned external weights or disabling its checks.
Use the already installed Bullet/CUDA toolchain. The first serious run targets
20 million distinct training inputs with newly produced, source-pinned Stockfish
18 search labels. A 1M-data pilot precedes that expenditure. Neither is a claim
that 20M positions will suffice to lead the Go field.

“Own” here means no imported trained tensor or optimizer state: NGN selects the
recipe and trains the weights from initialization, retaining reproducible source,
data, checkpoints and export provenance. Architecture reuse, public positions and
an external labeling teacher are disclosed. This is **teacher-supervised NGN
training**, not a claim of wholly self-generated data or teacher-free learning.
If the user wants those stronger restrictions, revise the data lane before launch.

The next implementation goal is one correctly integrated, audited owned candidate
and an honest playing verdict, not a promised rating. It must first demonstrate
an improvement over same-code HCE, then challenge same-code borrowed evaluators
and actual Counter/Rodent engines. A loss is a valid experiment result, not a
reason to rename or weaken the gate. Keep HCE and borrowed backends available.

Why this choice:

| Route | Useful property | Decision |
| --- | --- | --- |
| Repeat Chess768->128 with more updates | Existing pipeline | Rejected: its count-only stop rule already fired. |
| Single-bucket 512-neuron network | Simpler; NGN has related backends | Diagnostic fallback only; omits useful king dependence. |
| Four king buckets, 768 neurons, eight heads | Existing optimized inference and matching Bullet primitives | Selected: substantial capacity/representation improvement with bounded integration. |
| Train the entire SF18 BIG/threat architecture | Demonstrated strong borrowed reference | Later option; different, much larger training graph and feature pipeline. |

Rodent's release reports this selected shape and 2.1B training positions. That is
evidence for a credible architecture and the scale of the eventual challenge,
not proof of its recipe, our achievable Elo, or a requirement to start with 2.1B.
[Rodent V1.2 release](https://github.com/nescitus/Rodent-V/releases/tag/Rodent_v_1.2).

## 2. What the first experiments actually taught us

The following combines checked repository reports with fresh reads of WSL's
3M training receipt, source, selector, consumption ledger, selected model hash,
and completed four-game audit. It does not repeat the old full artifact rehash.

| Experiment | Distinct training positions | Presentations | Result |
| --- | ---: | ---: | --- |
| 100k-total pilot | 80,311 | 2,097,152 | Validation best early, then overfit; failed HCE screen. |
| 1M-total pilot | 801,150 | 2,097,152 | Validation still improving; selected candidate scored 0/4 against HCE. |
| Same 1M, longer schedule | 801,150 | 8,388,608 | Best at update 128; all later validation checkpoints worse. |
| 3M-total expansion | 2,406,580 | 8,388,608 | Better same-holdout loss, but selected candidate again scored 0/4. |

The 3M run selected update 384: raw validation loss `0.058132751389`, versus
`0.067229833` for the longer 1M run on exactly the same validation bytes.
Its selected 197,506-byte network hashes to
`00d3947d9ddd3734f502bec47f6fc7c2ece641bba5f765e8129f238007aa1a98`.
Every training record was presented three or four times. The training stage
completed in 6.42 seconds, including stage overhead; that is not a sustained
throughput forecast for the new architecture. It was a small controlled pilot,
not an exhausted overnight GPU training effort.

The four 3M games were complete and legal: 438 plies, all HCE wins by natural
mate, no operational failure. They used only two reversed-color opening pairs
and the old `4bd54add` search, not today's search. They reject that candidate as
a replacement; they do not locate its Elo or isolate one causal defect.
The PGN also shows lower NNUE search throughput than HCE in those games.
Treat speed/search interaction as a possible contributor, not a measured cause.

What is proved:

- Training, representative gradients, strict exports and independent integer/Go
  parity worked for the old graph. The failure was not simply a broken GPU run.
- Longer training on insufficient data overfit. Distinct coverage helped loss.
- Better old validation loss did not establish better chess or earn promotion.
- The historical labels used 75% game result and 25% `sigmoid(raw_score/400)`.
  The exact archive producer/score transform was never established. That divisor
  was an explicit plumbing assumption, not verified modern SF centipawns.
- Whole encoded-chain and exact model-input separation was checked, but original
  game disjointness was not proved. Inspected “sealed” data are no longer fresh.

Plausible explanations, deliberately not presented as proven causes:

- Limited capacity and no king-dependent feature buckets.
- Narrow prefix coverage, sparse king/endgame situations, correlated game outcomes.
- Poor target calibration and a result-heavy blend for a small corpus.
- Score scale/search pruning interaction; loss is not a move-quality objective.
- Quantization or inference cost contributing to losses, despite exact integer
  implementation parity. Float-to-integer error and integer-to-Go parity differ.

Do not pay for a new full replication of every September 6 gate. Retain their
evidence and retest what changes. Before the pilot, make one bounded diagnostic
on at most 10,000 frozen positions: old selected net, HCE, imported SF18, and fresh
teacher labels. Report error/sign/rank/score distributions by material, king
bucket, side, tactical/quiet class and game phase. Include the four old games as
failure-analysis fixtures, never as a checkpoint-selection or release-test set.
This can falsify obvious sign/scale defects; it cannot establish causality alone.

Historical evidence: [training pilot](2026-09-06-nnue-training-pilot.md),
[1M follow-up](2026-09-06-nnue-one-million.md),
[3M and games](2026-09-06-nnue-three-million.md),
[score provenance](2026-09-06-nnue-score-provenance-followup.md).

## 3. Architecture and file/score contract — implement before real training

Suggested public backend/format identifier: `ngn-k4-768-v1`. Keep `ngn-v1`,
`rodent-v1.2-default`, Counter and SF18 identities and decoding behavior intact.
Suggested implementation locations are new work, not commands already available:
`nnue/ngnk4/`, a K4 bridge alongside `cmd/nnuebridge`, and explicit engine/UCI
selection. Prefer extracting a shared private arithmetic core if this preserves
all locked-backend behavior; do not undertake an unrelated evaluator refactor.

Freeze these semantics in a format specification and executable fixtures:

- Piece planes: relative color then pawn/knight/bishop/rook/queen/king; A1=0.
  For Black's perspective, swap color and rank-flip squares (`xor 56`). Mirror
  files (`xor 7`) when that perspective's king is on files e–h.
- Use the existing four-bucket map. Its half-board rows a–d are
  `[1,1,0,0]`, `[2,2,2,2]`, then six rows `[3,3,3,3]`.
  Feature index is `bucket*768 + relative_color*384 + piece_type*64 + square`.
- Shared transformer, 768 hidden units per perspective, SCReLU, STM/NTM
  concatenation. Output bucket is `min(7, (piece_count-2)/4)` on valid boards.
- Training-only shared 768-feature factorizer, merged into all four transformer
  buckets before quantization; zero-initialized factorizer and randomized main
  trainable tensors are allowed. No pretrained initialization.
- Deployed parameter count: 2,372,360; i16 tensor payload: 4,744,720 bytes.
  The factorizer adds 589,824 training parameters, not deployed parameters.
- `QA=255`, `QB=64`; feature weights/bias quantized by QA, output weights by QB,
  output bias by QA*QB. Specify f32->f64 scaling and half-away-from-zero rounding.
  Freeze output-head transpose/order with asymmetric fixtures.
- Float network output is `z`; undamped NGN score is `400*z`. Integer contract:
  `q = trunc(sum(clamp(acc,0,255)^2 * w_out)/255) + b_out`, then
  `score = trunc(q*400/(255*64))`. Use wide arithmetic for the reference and
  post-dot multiplication. No Rodent `206` multiplier or material factor.
- At NGN's engine boundary, use the established `nnueSearchScore` rule-50
  attenuation exactly once and reserve the mate band. No learned mate encoding.
- Versioned, bounded header: magic/version, architecture/feature/quantization/
  score-policy identifiers, dimensions, payload length and checksum. Model SHA,
  training-manifest SHA and actual score policy participate in evaluator identity.
  Reject wrong dimensions, ranges, corrupt checksum, truncation and trailing data;
  an invalid new selection must leave the old model, TT and histories untouched.

**New-weights arithmetic hazard:** `rodentv12eval` currently accepts one SHA and
uses i16 transformer arithmetic and modular i32 output sums. Exact agreement with
that one network does not authorize arbitrary learned weights. For new models:

1. Use an i32/i64 independent reference and prove no i16 transformer overflow
   under a conservative legal-board bound, including promotions (at most 32 men).
2. Admit the existing i32 dot fast path only when, for each head,
   `255^2 * sum(abs(output_weights)) <= MaxInt32`. Bound every intermediate
   operation, including partial update sequences, not only final scores.
3. If a legitimate candidate fails a fast-path bound, use a tested wide path and
   measure it, or revise the numerical contract before a new run. Never silently
   wrap, saturate, discard weights, or relax a test to admit a candidate.

Reference code: `rodentv12eval/{model,evaluate,load,context,output_dot}.go`,
`engine/{rodent_v12_transition,worker_evaluator,evaluator_selection}.go`,
`nnue/quantize.go`, and `cmd/nnuebridge/bridge.go`.
The installed pinned Bullet example already demonstrates king buckets, an
export-merged factorizer and output heads:
[Bullet bucket example](https://github.com/jw1912/bullet/blob/629ee50000b2afb7b3337595401c830d3b1e0f42/examples/progression/3_input_buckets.rs).

Required changed-graph tests: all 64 king squares for both colors, all eight heads,
asymmetric STM/NTM outputs, mirror-boundary and bucket-crossing king moves,
captures, castling, en passant, promotions, null moves and full unwind. Exact
integer parity across independent exporter, Go full refresh, incremental, v1 and
v3 kernels; at least 10,000 seeded legal transitions in addition to edge fixtures.
Representative finite-difference derivatives must cover transformer, factorizer
and multiple heads. Test resume preserves optimizer, LR step and data cursor.
Do not impose global color/STM score negation where two-perspective tempo terms
make it mathematically invalid. Test symmetries on the actual feature contract.

## 4. Data and targets — remove the unknown historical score scale

### 4.1 Source positions, not inherited labels

Reuse the existing 10.81GB archive, not another download or a copy per attempt:

`/home/ehrli/nnue-public-toolchain-20260906/data/t80-2022-08/test80-2022-08-aug-16tb7p.v6-dd.min.binpack`

SHA-256: `0d22957b8d4f0f8e6f2913be7b744b2dab5178c3563e0f916312d0d94c28b92b`.
Publisher revision: `1e095a758c630bc58d0b6dac4da44fcd38ac89c2` in
`official-stockfish/master-binpacks`. The recorded dataset license is ODbL, not
CC0. Preserve provenance/attribution; publication of derived datasets or weights
needs its own license review. Retraining does not automatically erase obligations.
[Dataset entry](2026-09-05-nnue-data-entry.md).

Decode original FEN/state before flattening. Do not relabel the existing BF alone:
that representation omits information such as castling rights, EP and clocks.
The current converter/source verifier is reusable, but its giant per-position
JSON sidecars and repeated build snapshots must not be scaled linearly.
[Bullet data format caveats](https://github.com/jw1912/bullet/blob/629ee50000b2afb7b3337595401c830d3b1e0f42/docs/3-data.md).

Perform a bounded-memory full archive scan and deterministic, hash-priority
sampling across complete encoded chains, rather than consuming another early
prefix. Use compressed shard metadata with original source/chain/entry IDs and
literal reconstructible state. Cap scan at one complete input file and four
hours; if insufficient eligible data exist, record counts and revise, not recycle
duplicates to report a larger distinct corpus. Use restartable chunk manifests.

Default eligibility: standard chess, structurally valid/reachable decoded state,
at least ply 8, not in check, legal source move, and quiet non-promotion source
move. Verify EP captures explicitly. Do not filter using unknown historical score
units. After labeling, require a legal non-capture/non-promotion teacher best move
and a finite ordinary score. Record every rejection category and original counts.
Preserve some won/lost and low-material positions; do not select only balanced
middlegames. Report natural phase/score/king/head/side distributions before any
decision to stratify. No Chess960 augmentation in this first standard-chess run.

### 4.2 Splits and sampling

- Use a new split domain/seed (`26092001`) and deterministic 97/1/1/1 chain
  assignment: training, validation, calibration/diagnostics, reserved test.
- Exclude all historical validation/sealed input keys from new training, including
  the mirrored K4-equivalent keys. Freeze new holdouts before the first update.
- Quarantine whole chains when any K4 input crosses partitions. Compute keys from
  both perspective feature lists and output head, not merely FEN text or the old
  Chess768 key. Check all eligible members of chosen holdout chains, not only the
  capped retained subset. Exclude exact and architecture-equivalent duplicates.
- Target training sizes are 1M accepted unique inputs for pilot and 20M for main;
  take 100k validation, 100k calibration, 100k reserved test if available. “20M”
  always means training inputs, not total across splits or presentation count.
- The pilot is a deterministic subset of the planned main training pool. Holdouts
  stay byte-identical between these two stages. Never resize them using results.
- Deterministic training permutation/shards and recorded cursor; two loader
  threads, bounded queue eight. Actual batch IDs/counts must witness coverage;
  model RNG seed alone does not establish data-order reproducibility.
- These remain encoded-chain and input-disjoint partitions, **not proven
  original-game-disjoint partitions**. Their loss is developmental evidence.
  Fresh completed-game tests on unseen opening pairs are the strength evidence.
  For later predictive generalization claims, generate game-ID-preserving data
  and new holdouts; do not relabel an old sealed set “unseen.”

### 4.3 New labels and exact score convention

Default teacher: pinned official SF18 source
`cb3d4ee9b47d0c5aae855b12379378ea1439675c`, exact BIG/SMALL model hashes,
single thread, MultiPV=1, Hash=16MiB, no tablebases or book, 5,000 nodes per root.
Use the existing teacher binary only after matching source/build/net provenance;
otherwise build a new immutable teacher from that pin. Existing binary location
and SHA are in the evidence file beside this plan.

Reset the root's halfmove clock to zero for labeling, while retaining original
state in provenance; keep legal castling/EP state. This prevents the learner from
trying to infer an invisible clock or learning attenuation that NGN applies again.
Root history is absent by contract. Use `ucinewgame` plus `isready` before each
root for reproducibility; measure the cost of clearing Hash rather than omitting
it silently. At most two one-thread teacher processes, on two admitted CPUs.

Wait for `bestmove`; pair it with the last completed exact MultiPV=1 score from
the same request, requiring depth >=4. Reject bound-only, mate/TB sentinel,
missing-score, illegal-PV and failed/timeout records with explicit receipts.
Use a two-second per-root deadline; investigate if more than 5% of throughput-pilot
records lack exact/depth-qualified scores.
Normal quiet/mate/tail filtering is counted separately and included in accepted
throughput, not treated as a protocol error. Abort on process/protocol failure
rather than silently skipping it. Shard/checkpoint progress by stable record IDs.

For the first candidate, train on **100% new teacher score**, 0% archived game
result. This is a deliberate distillation baseline, not a claim that result
blending is inferior. A later result blend is a separate controlled experiment.
Retain original result fields only as provenance/diagnostics.

Define targets in NGN pawn units rather than labeling SF's normalized UCI number
as its raw value. SF18 uses `uci_cp = round(100 * internal_value / a(material))`.
Implement the pinned material polynomial independently, then define:

```text
m = clamp(pawns + 3*knights + 3*bishops + 5*rooks + 9*queens, 17, 78) / 58
a = ((-72.32565836*m + 185.93832038)*m - 144.58862193)*m + 416.44950446
c = round(uci_cp * a / 208)              # STM, approximate internal_value*100/208
target = 1 / (1 + exp(-c / 400))         # natural exponential, not base 10
prediction = sigmoid(z)
loss = mean((prediction - target)^2)
engine_undamped_score = 400*z
```

UCI rounding makes `c` an approximation; verify and record the bound
`a/(2*208)+0.5` NGN score units against a small instrumented pinned-source oracle.
Alternatively store the exact internal value from that oracle if extending it
into the labeler, but freeze one choice before data generation; do not mix label
contracts. Source of polynomial and UCI normalization:
[pinned SF18 UCI](https://github.com/official-stockfish/Stockfish/blob/cb3d4ee9b47d0c5aae855b12379378ea1439675c/src/uci.cpp).
The `208` mapping agrees with NGN's existing `sf18BigCentipawns` unit conversion;
the teacher still uses SF's search, not an identical NGN evaluation policy.

This sigmoid is a chosen bounded training transform, **not calibrated actual
win probability**. Store finite `c` in i16 only after range checks; reject
`abs(c)>10,000` as saturated tail data, counting it separately. The existing
Bullet target mapper can consume these scores with `eval_scale=400` and
`ConstantWDL=0`. Preserve STM->White->Bullet orientation tests. Example witnesses:
`c=0 -> .5`, `c=400 -> .73105858`, `c=-400 -> .26894142`.
`cmd/oracle/pipeline.go` instead uses base-10 sigmoid and White-relative Texel
output: it is not a drop-in producer for this contract.

### 4.4 Cost admission, not a guessed overnight promise

Before bulk labeling, run a <=120-second benchmark over a frozen stratified pool,
including decoding, teacher reset, filtering, writing and hashing. Report accepted
positions/second, not just NPS. Follow with a <=30-minute 50k accepted-label shard
to validate steady-state rate/restart and compare 1,000 calibration positions at
5k versus 20k nodes. A mean absolute target difference >0.05 requires a label
quality review before bulk work; do not silently raise nodes or change targets.

Compute `estimated_seconds = accepted_target / measured_accepted_rate`, with
30% slack. Pilot labeling wall cap: four hours. Main incremental labeling cap:
24 hours. A 20M corpus at 250 accepted/s is about 22.2 hours **before** slack;
at 1,000/s it is 5.6 hours. These are examples, not measured rates. Budget from
remaining accepted records, including holdouts, not the already completed pilot.
If the forecast exceeds the cap, stop for an explicit revised data/resource
decision. Do not substitute weak static labels, lower nodes, extra CPUs, old
uncalibrated labels, or repeated samples to make the run look complete.

Longer-term, 100M+ varied inputs and eventually game-ID-preserving self-play are
the expansion path. After a useful 20M result, choose between broader trusted
public data, more fresh teacher labels, and NGN-generated positions with known
game IDs. Freeze another recipe before expanding. No billion-position download
or multi-day automatic sweep is part of this first run.

## 5. Training schedule and checkpoint selection

Use Bullet `629ee50000b2afb7b3337595401c830d3b1e0f42`, installed Rust 1.88.0,
CUDA toolkit 12.8.1 (observed nvcc 12.8.93), sm_120, FP32. Reuse locked/offline
dependencies and checked error-propagating save/queue fixes. Do not upgrade the
stack for novelty. Copy only the small changed harness; reference immutable
source/vendor/data identities instead of duplicating caches into every attempt.

| Setting | Pilot | Main run |
| --- | ---: | ---: |
| Unique training inputs | 1,000,000 | 20,000,000 |
| Batch size | 16,384 | 16,384 |
| Maximum completed updates | 1,024 | 16,384 |
| Maximum presentations | 16,777,216 | 268,435,456 |
| Validation/export interval | 128 updates | 512 updates |
| GPU training wall cap | 1 hour | 4 hours |
| Initialization seed | 26092001 | 26092001, fresh cold initialization |

AdamW: beta1=.9, beta2=.999, decay=.01; preserve the pinned implementation's
epsilon and update semantics and serialize them in the manifest. Initial LR=.001;
cosine decay to .00005 over the predeclared maximum schedule. This is a starting
recipe, not a tuned optimum. Transformer main/factorizer clipping ±.99 each,
other tensors ±1.98 as in the pinned default; quantization/range gates still apply.
No architecture/seed/LR grid in this first experiment.

Stop early after four successive validation checks without an eligible integer
loss improvement >=1e-5, but not before 25% of scheduled updates. Save/validate
every scheduled checkpoint up to that point. Immediate stop on NaN, invalid
gradient/optimizer, range failure, lost supervisor or resource violation.
Checkpoint 0 is diagnostic only. Select the lowest **integer inference** MSE
among eligible trained checkpoints, tie to earliest update; report raw loss too.
Keep the old runs' raw-loss rule unchanged in their historical records.

Eligibility: complete checkpoint, all weights/optimizer finite, exact strict
raw->merged->quantized conversion, safe arithmetic, independent Go parity, and
float/integer score error on validation mean <=8, p99<=32, max<=64 NGN units.
These are conservative prospective engineering gates, not Elo thresholds.
If they fail, investigate export versus quantization; do not increase tolerances
after seeing games. Report integer-vs-float MSE change and saturation by head.

Before either full schedule, measure 256 warm updates plus one strict export and
full validation pass, with a ten-minute cap, using a disposable smoke attempt.
Measure GPU/RSS and end-to-end presentations/second for this actual graph; forecast
the full stage including validation/export overhead. A non-fitting batch is a
manifest revision before a cold run, not an unrecorded mid-run batch-size change.
Do not count smoke updates toward the separately initialized pilot or main run.

Keep one immutable initial checkpoint; all raw/quantized candidate tensors and
metric receipts; resumable optimizer state at current and best checkpoints.
Avoid retaining duplicate automatic saves. An optimizer checkpoint is not a
resume receipt without RNG, step, permutation/cursor, architecture and input hashes.
Crash/resume writes a new attempt receipt referencing the parent; never overwrites
a failed attempt as if it had succeeded. GPU reruns need not be bit-identical:
report measured repeatability rather than promising determinism CUDA cannot prove.

Validation and reserved data must not enter the loader. A negative-control test
that removes/mutates reserved labels must leave candidate selection unchanged.
Read the reserved labels once after the main candidate and scale are frozen;
do not use their result to switch checkpoints. The 1M pilot does not open them.

## 6. Pilot, integration and strength gates

1. **Changed-pipeline gate:** decoder/labels/feature map/gradient/export/range/
   transition/resume tests above. Semantic negative controls must reject sign,
   perspective, king bucket, output transpose, factorizer merge, source hash and
   duplicate-split corruption; changing checksums alone must not hide bad data.
2. **1M pilot:** meet resource and coverage bounds, lower validation integer MSE
   than initialization, non-degenerate material/score distributions, and improve
   fresh-teacher agreement over the old 128-net on the diagnostic partition.
   If not, investigate before labeling 20M. Low loss alone is insufficient.
3. **Same-code integration:** UCI owns an explicit model identity; reload failure
   is transactional; successful reload invalidates TT/history appropriately;
   worker contexts remain private. Run focused race tests, `go vet ./...`, short
   suite, and portable/v3 builds using normal bounded gates. Sample full-refresh
   versus incremental complete searches, including cancellation and SMP unwind.
4. **Cost and playing screen:** record 32 fixed-node positions, evaluator calls,
   ns/node and time-to-depth against same-code HCE and borrowed Rodent/SF18.
   Run 40 fresh paired-opening games against HCE at 10+0.1, Threads1, Hash128,
   overhead100, concurrency1. Fixed-node tests diagnose cost; real-clock games
   determine practical strength. There is no fixed-node Elo claim.
5. **Pilot decision:** 0 points/40 or any operational error stops expansion for
   diagnosis. Score <35% is a hold for a written diagnosis, not automatic more
   data. Score >=35% plus the data/learning gates permits the 20M run, explicitly
   without claiming the pilot beats HCE. Use a disjoint game set for the main net.
6. **20M candidate:** same tests and 40-game screen. If score >=50%, run a frozen
   400-game HCE confirmation at 30+0.3. If 35–50%, inspect evidence before paying
   for more games/data. Below35% reject the recipe for promotion. Small-screen
   cutoffs only allocate resources, not quantify strength.
7. **Owned-network milestone:** all audits pass and the 400-game paired-bootstrap
   95% lower confidence bound on score exceeds50%. Otherwise report inconclusive
   or loss. Freeze candidate before games; no picking a runner-up after a loss.
8. **Next goal, not automatic victory:** compare that frozen owned candidate with
   same-code SF18/Rodent and actual Counter5.5/Rodent1.2 under identical conditions,
   then a longer-TC confirmation. Report the borrowed-to-owned strength gap even
   if owned beats HCE. A new absolute rating requires appropriate external evidence.

Use complete legal games, reversed colors, opening hashes, identical search policy,
no hidden book, no resignation/adjudication shortcut, no recovery hiding crashes,
and the existing independent PGN/terminal/process audit. No max-ply draws as a
substitute for completing games. Freeze pair-bootstrap method/seed before play.
Timing-sensitive games must not overlap our own training/labeling/build jobs.
Keep the hopper running; use an admitted quiet CPU, record co-tenancy, and call
results shared-host evidence rather than pretending full-machine exclusivity.
If interference prevents interpretable timing, defer the match, not the hopper.

Saved Counter anchor is useful context but is still **uncertified**: 100 games
finished 49W/46D/5L, while the runner failed its trace audit on blank footer lines.
Before using that anchor as accepted evidence, make an additive parser recovery
with strict regression tests, finish independent chess audit and hashes, and
preserve the original FAILED receipt. This is a small parallel prerequisite for
comparison reports, not a reason to rerun 100 games or postpone owned-net design.

Failure diagnosis order: wrong identity/data/targets -> independent integer
semantics -> feature/phase coverage -> scale and pruning interactions -> inference
cost -> remaining model/data capacity. For search coupling, use a research-only
small fixed-node conservative-search comparison, not a production weakening.
Make one evidence-backed change at a time after this first integrated recipe;
do not claim a bundled architecture/data/target change isolates the earlier cause.

## 7. WSL resources, disk blocker and execution discipline

Read-only observation on September 20 at 20:45 UTC: RTX 5080, 16,303 MiB total VRAM,
1,219 MiB in use; hopper PID 968 live. Linux root reports 615 GiB available, but
Windows C: has only 5.8 GiB and the WSL distro BasePath is on C:. **Bulk work must
not launch from this snapshot.** Free ext4 blocks are not a guarantee that a
dynamically expanding VHDX can obtain host disk space.

Before execution, Sol must obtain a fresh availability snapshot and satisfy:

- Host C: free >=60 GiB, Linux free >=40 GiB; projected new allocation <=20 GiB,
  with at least 25 GiB host reserve after that allocation. Revise estimates using
  the actual small shard. No deleting user files, old evidence, caches or VHDX
  compaction/migration without appropriate authority. Earlier permission to delete
  named videos is not general cleanup permission.
- Stop our job before further bulk writes when host free <25 GiB, Linux free
  <20 GiB, or the job's new storage exceeds 20 GiB. Include logs, source/build
  artifacts, checkpoint states,
  shuffle temp, sidecars, pagefile/VHD growth and failed-attempt files in accounting.
- One GPU trainer; two loader threads, queue 8; aggregate NGN CPU allowance two
  physical cores (historically 12,14, recheck topology/availability). Builds `-j2`,
  Go `GOMAXPROCS=2`, teacher two processes maximum. Do not reserve all CPUs.
- RSS <=12 GiB for the NGN job tree. Total observed GPU use <=12 GiB; require >=4 GiB
  free immediately before launch and stop our job if the shared-device ceiling is
  exceeded. Device-wide VRAM samples are not attribution or a hard allocation cap.
- Use the existing bounded process supervisor, fail-closed monitor, monotonic
  deadlines, stage return codes and survivor checks. Prefer cgroup limits if
  available without host reconfiguration; describe sampled limits honestly.
- Log resource samples locally at sensible intervals (e.g. 5–10 s resources,
  60 s host disk); control log volume. Check descendants by actual process identity,
  not only a stale PID file. Restart only after confirmed terminal state.
- No GPU driver installation, hopper stop/pause, remote cleanup or unlimited job.
  If children need the PC, checkpoint/stop only our work and respect user steering.

Suggested new root: `/home/ehrli/nnue-owned-k4-20260920` with immutable manifests,
content-addressed source/data references, independent attempts and additive
receipts. Do not edit `/home/ehrli/nnue-public-toolchain-20260906/runs/*` in place.
Keep large data/model artifacts outside Git; commit source, compact manifests,
model cards and reviewable summaries at coherent completed gates.

While a job runs, announce its handle and expected completion once; use native
completion notification or one bounded waiter. If polling is unavoidable, use
10–15 minutes for hour-scale labeling, back off on no change, and batch status.
No per-game narration, duplicate monitoring agents or artificial side tasks.
The earlier two-minute machine-check restriction applied to planning, not Sol's
normal bounded implementation/validation runs.

## 8. Sol's execution order and handoff deliverables

Start in `/private/tmp/ngn-n1-sf18-worktree`; main working tree has unrelated
untracked `SHA256SUMS`. Preserve it. Use this plan's eventual commit or a branch
from it, not the older main-worktree checkout. Recheck worktree state once.

1. Read this plan and `owned-nnue-plan/evidence.json`; keep the hopper unchanged.
   Resolve disk headroom before remote build/data/GPU writes. Source-only local
   implementation can proceed if the disk gate is held.
2. Commit the format/score contract, new backend/bridge and changed-graph tests.
   Prove imported backend regression parity; preserve opt-in/default behavior.
3. Commit streaming sampler/split verifier, deterministic teacher labeler, strict
   exporter/selector and resumable bounded runner. Reuse existing supervisors;
   fix exactly the relevant hazards rather than building a second orchestration
   platform. Put future maintained training sources in this repository, not only
   an untracked WSL directory.
4. Freeze `run-manifest.json`: source commits/diffs, binaries/net hashes, compiler/
   CUDA versions, architecture, score transform, source archive/license, split
   algorithm/keys, teacher policy, exact shard order, optimizer/schedule, resource
   ceilings, selector, match openings and stop rules. Unknown pins must be filled
   and verified before execution, not represented by “latest.”
5. Run the bounded label/graph/resume probes, publish measured disk/time forecast,
   then 1M pilot. Proceed to 20M only through the explicit gates above. Every stage
   writes PASS/FAIL/HELD plus its evidence; a launch script cannot run past HELD.
6. Retain selected model SHA, initial-state proof, data/target receipts, coverage
   counts, curves, every candidate's eligibility, paired match results, operating
   costs, and a model card disclosing public data and teacher. Record losses too.
7. Stop at the agreed run's verdict. Do not automatically expand to 100M, change
   search defaults, publish a model, announce strongest-Go status or start a sweep.

The runner/CLI for this new architecture does **not** exist yet. This is a
complete implementation/run specification, not a fictitious ready-to-execute
training command. Sol's normal implementation work must close the listed gates.
Planning is finished when this handoff is committed and internally checked;
training remains deliberately unstarted.

## Planning completion check

- Failure investigation: checked saved trainer/selector/coverage/terminal/game
  receipts and selected model hash; separated established results from hypotheses.
- Source cross-check: read installed pinned Bullet graph/buckets/target/optimizer
  code, NGN evaluator/loader/arithmetic/score adapters, and pinned upstream sources.
- Next-run design: selected one architecture, exact target/score contract, data
  route, pilot/main schedule, numerical and playing gates, budget and stop rules.
- WSL preparation: located reusable tools/data/teacher and identified the actual
  C:-backed disk constraint; no launch, installation, deletion or hopper mutation.
- Internal checks: evidence JSON parses; parameter/payload/presentation/coverage
  arithmetic matches; score inversion bound checked over all clamped integer
  material values and sampled signed scores (maximum bound <1.434 NGN units).
- Publication: GitHub repository privacy/destination verification did not succeed
  (`gh repo view ehrlich-b/ngn` could not resolve the repository). Commit locally;
  do not infer public/private status or push without successful verification.
