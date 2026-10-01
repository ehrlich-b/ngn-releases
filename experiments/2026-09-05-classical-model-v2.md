# Classical model v2: implementation and first grouped corpus

Status: offline implementation integrated; no evaluation weights changed and no
new playing default installed. This addresses the [classical comparison's first
target](2026-09-04-classical-evaluation-gap.md), not a claim of Elo improvement.
The active goal remains at least 2800 without NNUE; the latest absolute pin is
2726 [2668,2784]. The separate frozen qsearch repair is in its own game gate.

## Model and fidelity

`ngn-texel-v2-936` preserves the 798-entry legacy prefix and appends 66 MG
mobility cells, 66 EG cells, and six threat coefficients. Two legacy mobility
scalars remain explicitly inactive reservations: 936 stored, 934 active.
No weights are transplanted from another engine. The existing live evaluator
`engine/eval.go` and its coefficients are byte-identical to the pre-expansion
source; extraction helpers live in offline tuning files.

The JSON export includes version, exact ordered names, integer values, stored
and active counts, and inactive reservations. `scripts/apply_tuned.py` validates
all metadata and all 936 entries before changing either source file. Go-source
input remains available for legacy coordinate descent. Source writes are staged
but two file replacements are sequential, not a two-file atomic transaction.
`ApplyTexelModel` clears derived caches and requires fresh/rebuilt board
accumulators; it is an offline/startup API, never safe during an active search.

Proofs completed:

- Every appended coefficient changes the actual live evaluator and matches a
  previously captured feature trace under perturbation: 138/138, no zero skips.
  Mobility-bin coverage uses constructed evaluator boards (including pawn
  blockers on back ranks), explicitly not a reachable-game training corpus.
- Existing diverse legal walks test frozen-trace parity under changes to the
  complete parameter vector; numerical gradients cover the new coordinates.
- `scripts/verify_texel_model.py` copies sources to a temporary directory,
  changes every one of 936 values, imports, rebuilds the actual engine and tuner,
  and requires the compiled dump to equal the full expected vector. Invalid
  versions/names/counts/reservations, missing values, booleans, NaN, out-of-range
  integers, extra and duplicate JSON fields reject before source changes.
- Root-built initial model preserves nodes, score, complete PV and best move on
  the three canonical searches versus frozen qcap: 463172 / 138436 / 858070.
  Evidence: `output/recovery-2026-09-04/texel-v2-identity.json` and UCI logs.
- An independent read-only agent review found no model-order/activation/import
  blocker; it noted the sequential source-file replacement limitation above.

## Grouped data contract

`cmd/texelcorpus` records full startpos UCI move lists, canonical opening IDs
(placement/turn/rights/EP), absolute opening indices, nodes/maxplies/seed,
generator executable SHA/source, filter ID/source, terminal reasons and White
outcomes. Only legally completed games carry labels; max-ply truncations remain
unresolved and contribute no rows. Mate takes precedence over clock draws.
Sampling uses a seed derived from the configured seed and canonical opening,
so sharding and duplicate book indices cannot change a played game's samples.

The loader replays every game, checks samples against exact replayed FENs, rejects
terminal continuations, false/truncated max-ply records and terminal samples,
deduplicates identical games, and partitions whole opening groups 80/10/10 by
fixed hash. It keeps at most 32 board-distinct samples per game. Distinct games'
observations remain within a partition; cross-partition board collisions are
kept only in test, then validation, then train. File order cannot choose outcomes
or row order. Reports retain input checksums, exclusions, completed groups,
partition row W/D/L and training phase/new-feature support.

The loader deliberately does not rerun weight-dependent quiet classification:
the generator's exact source/filter identity is its provenance. It validates
legal replay and labels, not whether a newer evaluator would make the same
quietness decision. Existing anonymous flat datasets cannot prove game-disjoint
validation; the old loader also now rejects nonfinite/out-of-range labels.

`texel -mode audit -corpus ... -report ...` measures corpus structure and training
feature support without fitting. `-mode gradient -corpus ...` calibrates K on
train only, uses validation for checkpoint selection, and measures baseline and
fitted integer models on the test partition only after selection ends. A
regression changes test labels and proves the fitted model, K and validation
loss stay identical. Test-set results may reject this one fit, never select
hyperparameters or another checkpoint from it.

## Predeclared first corpus and fit

Before generation: reserve original opening lines 0–999 for confirmation games.
Canonical audit finds 1000 unique reserved groups and zero overlap with lines
1000–4999. Remove four duplicate training groups, leaving **3996** unique
training openings in original order. Frozen training book:
`output/recovery-2026-09-04/classical-v2-training-openings.txt`, SHA-256
`4f7e93fb77514d27c4e565ac1ab4d54fb37511aa54700473a91d7f50688e1c05`.
Original book SHA-256:
`974e4b5ab871a9e106d0c766bfa39fc83676222337fb7702f61782e2ac5b3222`.
Original-index mapping and exclusions: `classical-v2-book-audit.json` in that
output directory. No new opening source or opening cycling is used.

1. Freeze a committed generator source and binary hash. Smoke exactly 32 games,
   first 32 filtered opening indices, 8000 nodes/move, max400 plies, seed20260905,
   GOMAXPROCS=1 on the Mac. Require no operational failure, valid full replay,
   exact recorded settings and nonzero completed games/samples. Exclude this
   smoke file from the final corpus; the full run regenerates these openings.
2. If smoke passes, generate the entire 3996-opening range once: four independent
   Mac processes, GOMAXPROCS=1 each, ranges [0,999), [999,1998), [1998,2997),
   [2997,3996), same frozen binary/settings. No Windows/WSL compute contention
   with timed games. Every process ends after its fixed range, no recycling.
3. Load all four shards with split seed `ngn-classical-v2`. Require all replay
   checks, at least 60% legally completed games, at least 1500 train and 100 each
   validation/test completed opening groups, and 20000 train rows. Each split
   must contain all three outcomes, with no outcome exceeding 90% of its rows.
   Report unsupported mobility/threat coordinates rather than asserting full
   empirical support; unsupported weights stay at their starting values.
   Failure stops this fit and prompts diagnosis, not favorable-seed resampling.
4. After the qsearch repair verdict and corpus acceptance, run one joint fit:
   300 maximum epochs, LR1, patience30, L2=1e-7, baseline-calibrated fixed K.
   Freeze the selected rounded model and compare both integer validation and
   final-test MSE with their baselines at the same K. Both must improve to
   advance; a lower float training loss is insufficient. Do not refit based on
   the final-test result. Record exact model/report/binary hashes.
5. A model that passes offline gates still needs a separately predeclared paired
   real-clock comparison against the then-accepted immediate base, capped at
   1–2 confirmation hours. No default changes or speculative-weight commits
   before that game verdict. No predicted or added-up Elo gains.

The generator includes the committed qsearch correction currently under game
test. Those are legally played outcomes regardless of that implementation's
eventual strength verdict. Fitting/playing integration waits for that verdict;
generation does not establish or substitute for its acceptance.

## Frozen implementation and generation artifacts

Committed source `9add92c0324f3fabb3999fb22ab251461a0d152c`, clean tracked
checkout at build time; Go1.26.2 darwin/arm64:

- `build/texelcorpus_20260905`: SHA-256
  `b832e5cd83896c2da537d8277acf922b1afcc9dbca7a251910f9c744eb024db3`.
- `build/texel_20260905`: SHA-256
  `edf7d2777ff12a9848244c723975b30591108966edf1fb1853a3dc3777ad623f`.

Root integrated full short/race/vet suites and source-roundtrip verification
all pass. Logs: `classical-v2-full-short.txt`, `classical-v2-full-race.txt`,
`classical-v2-full-vet.txt`, `classical-v2-source-roundtrip.txt` under
`output/recovery-2026-09-04/`.

Smoke command (the final corpus will not load this file):

```sh
GOMAXPROCS=1 build/texelcorpus_20260905 -openings output/recovery-2026-09-04/classical-v2-training-openings.txt -opening-start 0 -games 32 -nodes 8000 -maxplies 400 -seed 20260905 -source 9add92c0324f3fabb3999fb22ab251461a0d152c -out output/recovery-2026-09-04/classical-v2-smoke.jsonl
build/texel_20260905 -mode audit -corpus output/recovery-2026-09-04/classical-v2-smoke.jsonl -report output/recovery-2026-09-04/classical-v2-smoke-audit.json
```

Smoke PASS: 32 attempted, 32 legally completed, zero unresolved/errors; 978
generated sample rows, 950 after 28 within-game board repeats are removed.
Loader replay and exact generator/settings/index checks pass. Smoke SHA-256:
`8175dbbddf678f42c52b9d7da120a0af7d58b57e44f3f8e940d704651a0601e0`.
This small smoke does not satisfy or replace the full corpus acceptance rule.

Full generation uses the same command/settings, changing only `-opening-start`
to 0/999/1998/2997, `-games 999`, and `-out` to
`output/recovery-2026-09-04/classical-v2-shard-{0,1,2,3}.jsonl`. Four Mac
processes are launched together, each with GOMAXPROCS=1. Per-process logs and
the final combined audit remain in that output directory.

### Generator predecessor correction before accepting data

The first full generation was stopped without a corpus verdict: root review
found `SetLastMovePlayed` was reset to empty but never updated after real moves.
Unlike UCI's position replay, this disabled predecessor-dependent root ordering
(`engine/moveorder.go:677`). Internal search sets/restores its own predecessors,
so legal replay alone did not catch the missing handoff at successive roots.
The 32-game smoke above verified its stated format/replay checks, but that
generator is superseded and none of its data will enter the fit.

All four generation workers terminated (exit −15), before publishing the full
shards. Root moves the state reset before opening replay and sets the predecessor
after every actual opening/searched move. `TestGeneratorHandsPlayedMoveToNextRootSearch`
fails before correction (empty predecessor after e2e4) and covers both opening
and searched moves. Parent failure log: `corpus-predecessor-parent-fail.txt`.
This is a data-generation defect; no new playing-engine runtime is bundled.
The fixed ranges/settings/acceptance rule remain; restart with a new frozen
generator hash and a fresh 32-game smoke. Do not pool partial earlier data.

Corrected source `227c4d34ef3418a662f78527486db4dc32763aa2`; frozen Mac generator
`build/texelcorpus_20260905_r2`, SHA-256
`99dfa1a2a9aa4d12c0c98c9aa49f86e2e86271ef0f54399af57e96d84a8d301c`.
Full short/race/vet pass (`corpus-predecessor-{short,race,vet}.txt`). A second
read-only state audit found no further missing handoff. Generation intentionally
calls Search with a fresh default-size TT, bypassing UCI's optional external
book/tablebase probes; the fixed opening prefix supplies opening diversity.
Restart artifacts use `classical-v2-r2-*` names. The validated tuner executable
from `9add92c` remains valid: this correction changes only data generation.

The added independent Stockfish smoke exposed a serialization limitation:
`GenerateFEN` always emits fullmove1 because Position does not store that counter.
Corpus samples now derive it from their startpos replay ply, and the loader
requires the same exact FEN. This metadata correction does not change search,
quiet filtering, labels or model inputs. The old r2 smoke's format check passed,
but the independent check failed at game0/ply21 (reported move1, actual move11).
The new fullmove regression fails before correction. Both generator and loader
must now be rebuilt; r2 data is not pooled into the final corpus. Reproducer:
`experiments/2026-09-05-corpus-smoke-oracle.py` checks every move against SF legal
moves, sample states and final outcomes, tolerating only uncapturable EP-target
normalization between FEN serializers. New artifacts use `classical-v2-r3-*`.

Final corrected source `f556aef700bc0d07303c548f202117b0328cfcf9`, Go1.26.2
darwin/arm64, clean tracked checkout at build:

- Generator `build/texelcorpus_20260905_r3` SHA-256
  `fe1eb13edc5c3611e197892c1d81d4276306048d743a4b7235dce3d17c741800`.
- Tuner/loader `build/texel_20260905_r3` SHA-256
  `046bf1dfce08fd8f462677467bdc44fbc62801f34a7c9d48c18923c206b18a26`.

Full short, focused pipeline race and full vet pass after the metadata correction
(`corpus-fullmove-{short,race,vet}.txt`); full-project race passed immediately
before it. Smoke and full generation retain all predeclared settings/ranges,
using these binaries/source identity and r3 output paths. Before full generation,
also require the 32-game Stockfish smoke oracle and exact generated moves/results
to match r2 (the fullmove correction is serialization-only).

R3 smoke PASS: 32 completed games, 1016 generated samples, 1000 retained after
16 within-game board repeats. Independent Stockfish audit passes every one of
5229 plies, all 1016 sample states and every terminal outcome. Moves/results
are exactly identical to r2; only sample fullmove metadata differs. SHA-256:
`6c301cd6b4389ac28ece4f92cb93b48a5c753c728414ea298cef99d6bc55b983`.
Evidence: `classical-v2-r3-smoke-{audit,oracle,identity}.json` in the output
directory. Full generation is cleared and starts with the frozen r3 artifacts,
same four 999-opening ranges and settings. No earlier data is included.

A bounded local controller (`run_classical_v2_fit_when_ready.py` in the output
directory) waits for all four shards and the completed replay audit, verifies
every record's frozen source/binary/settings/index, enforces every predeclared
corpus acceptance threshold, and runs the single fit only on success. It records
acceptance, baseline model, fit/model reports and hashes; it never edits source
weights or launches games. The qsearch repair verdict is already accepted.
Root review of the completed fit precedes any [paired model games](2026-09-05-classical-confirmation.md).

## Completed corpus and single fit

Generation and full loader replay completed in 321s: 3996 attempted games,
3989 legally completed, seven unresolved excluded. After 1782 within-game
board-repeat exclusions and120 cross-partition exclusions, 121818 rows remain:
98191 train /12458 validation /11169 test, from3214 /404 /371 completed
opening groups. All three outcome classes are represented in each split with
no class near the90% limit. Every appended parameter has nonzero training
support, although some queen bins have only116–328 observations; those counts
are not independent games or a guarantee of stable estimates.

The predeclared corpus gate passes. Exact input SHA-256s, row W/D/L and phase/bin
counts are in `classical-v2-r3-corpus-{audit,acceptance}.json`. No failed earlier
generation, smoke file, flat legacy corpus or confirmation opening is included.

The single predeclared fit stopped early, using train-calibrated K=
0.9330378336706635. Integer model MSE:

| Partition | Initial model | Fitted model |
|---|---:|---:|
| Train | 0.081149765455 | 0.079638310773 |
| Validation (checkpoint selection) | 0.084579501529 | 0.084065047164 |
| Final test (measured after selection) | 0.077396602676 | 0.077069219580 |

Both required held-out comparisons improve: ~0.61% validation and~0.42% final
test relative MSE. This is modest offline evidence and cannot predict an Elo
gain. 543 stored coefficients change, including96 appended coefficients. The
largest appended mobility change is8cp and threat change4cp; the larger material/
scalar changes are EG pawn94→116 and EG passed-pawn coefficient44→21. Neither
test labels nor this review select another checkpoint or refit.

Frozen model SHA-256:
`6057944aea194e815a64985727a27785c46db03194c04f18fc93007884fe35a2`.
Reports: `classical-v2-r3-{fit-report,fit-result}.json`, `classical-v2-r3-fit.log`.
Root independently checked the artifacts/counts and a second agent reviewed
calibration, partitioning, checkpoint selection, parameter mapping and bounds;
no blocker found. All936 entries match the separately compiled candidate dump.
Weights exist only in `output/classical-v2-candidate-20260905`; main/defaults
retain the accepted untuned model. Real-clock games remain required.
