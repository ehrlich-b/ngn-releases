# Search-family follow-up on seven frozen disagreements

This is an offline diagnostic, not a search candidate or Elo verdict. All
artifacts were generated locally while the native classical fit match remained
frozen. No production source, playing defaults or fitted weights changed.

## Protocol and integrity

Before variant searches, freeze **every** case from the completed
[safe-check study](2026-09-05-safe-check-study.md) where the current400k-node
NGN choice is at least50cp below its separately scored SF alternative. This
selects seven of23 cases: capture lines105,148,188,197,194,140,195. They include
two draws, four losses and one win. Selection explicitly favors finding changes
that improve a poor baseline choice; it is unsuitable for estimating overall
playing benefit or comparing whole-engine loss rates.

Use accepted qcap400k as baseline, each of eight single-family `SearchToggles`
overlays at400k, accepted qcap4M, and the exact frozen fitted evaluator at400k.
IID and Singular stay enabled in every overlay. UCI processes are reused;
each search receives `ucinewgame`, Hash64/Threads1 setup and the complete game
prefix. Every distinct chosen continuation is scored by fresh-game SF depth16.
No bounded/incomplete score is treated as a numerical result.

All seven baseline final reported depth/score/PV/nodes/bestmove records match
the prior study. SF perft independently validates all20 distinct selected moves
at their full prefixes. All20 child scores complete depth16 and are usable.
Root checked all eight overlay texts: each changes exactly its declared toggle,
with frozen source and executable hashes matching the result. Local run29seconds;
the predeclared15-minute cap was not approached. Raw result and the subsequent
audit were frozen separately.

## Results

The table is calculated from the raw JSON. “Improved” means at least50cp versus
the baseline's scored choice; “worse” means any negative delta. Neither label
establishes causality or Elo, and zero worse choices in this selected sample
does not establish non-regression.

| Intervention | Improved /7 | Worse /7 |
|---|---:|---:|
| Baseline,10x nodes | 5 | 0 |
| Disable futility | 5 | 0 |
| Disable null move | 3 | 1 |
| Disable reverse futility | 3 | 0 |
| Disable SEE pruning | 3 | 0 |
| Disable LMR | 3 | 0 |
| Disable Probcut | 2 | 0 |
| Disable LMP | 2 | 1 |
| Disable history pruning | 0 | 0 |
| Frozen fitted evaluator | 2 | 0 |

Disabling futility at400k produces **the same choices on all seven roots** as
the baseline at4M. The five improvements are59,53,200,63 and62cp. This makes
futility the strongest next diagnostic lead, but does not justify disabling it
globally. Several other removals change the same decisions; tree/order changes
and shallow reference scoring still limit attribution.

The back-rank witness from GAME188 changes from Rd7 to Qd7 under either more
nodes or disabled futility, improving the reference score53cp. Thus the current
evaluator can avoid that immediate safe-check exposure with different search.
This weakens an evaluation-only explanation. The fitted evaluator retains Rd7
there, while improving two other selected positions59 and92cp. Its native game
gate remains the sole decision about retaining the fitted vector.

Next diagnostic: trace the actual futility skips on these roots and compare
against the remaining16 controls from the fixed23-position sample. Require a
specific mechanism and inspect counterexamples before choosing a smaller policy
change or a bounded game test. Do not infer a gain from the seven selected cases
or close other search lanes from this sample. The already completed T8 vector
still awaits its separate confirmation on the accepted base.

## Artifacts and reproduction

Raw directory: `output/recovery-2026-09-04/`.

- `search-ablation-result-2026-09-05.json` SHA-256
  `81566bf7c47817a06bbfacca8f559d6ddca06e677db6e73bc3aa23bbb8d01290`.
- `search-ablation-postrun-audit-2026-09-05.json` SHA-256
  `18fa122da3ad6eedc057e15af64618cf4d1045920be37b5511afd5ff760aa22c`.
- `search-ablation-selection-2026-09-05.json`, original protocol/runner,
  overlay sources, build logs and executable hashes are retained separately.
- Committed [reproducer](2026-09-05-search-ablation.py) preserves the experiment
  and adds input/source guards. It requires the exact prior study and binaries;
  it refuses Go-source differences from `e549f3c`. It overwrites its named raw
  outputs when rerun, so preserve the original artifacts first. Full baseline
  and move-legality audit remains a separate required review step.

## Separate speed investigation: shelved

A Terra implementation restricted the `selectNextMove` score slice once to
help eliminate loop bounds checks. A seeded differential test covered all
lengths1..256, duplicate scores and every selection prefix; order and swap
semantics matched. Root inspected all60 interleaved timing searches across the
three canonical roots: exact nodes/scores/PVs/bestmoves agree, but mean times
were effectively flat (Kiwipete .21969→.21946s, mid .06645→.06669s, ending
.22538→.22539s). No repeatable speed benefit: discarded, no source integration.

Exact diff/test and raw identity/timing records are `picker-speed-*` in the same
directory. Reopen with a different concrete mechanism and repeatable timing
improvement while retaining order and search identity. The profile hotspot alone
does not prove any particular optimization will help.
