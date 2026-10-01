# Owned K4 5M paired learning-rate result

Status at 2026-09-22 06:35 UTC: the lower-rate candidate passed its static,
32-position search and audited 40-game HCE expenditure gates. The 20M
expansion is now permitted, subject to its resource and corpus checks. The
screen does not establish a 3k rating.

## Identical data and isolated schedules

The [label receipt](2026-09-22-owned-nnue-probe5m-label-receipt.txt) closes all
50 frozen main-expansion shards, with 4,327,066 accepted positions. The
[post-label receipt](2026-09-22-owned-nnue-probe5m-postlabel-receipt.txt)
binds the exact 5M finalized corpus manifest SHA-256
`c3bd3b204d1211c95e65e76217dd995e767077a851411a50cf1293f88cf844ab`.
Its BF SHA-256 is
`c41c72f3411faf92bb1b548d1300442578c3b0fd8d4dde8fb0859f7823ffb90e`:
the original 1M pilot prefix plus four million accepted expansion inputs.
Both 5M runs used that same BF and the same frozen validation and calibration
sets, K4 graph, seed, teacher target, batch size, 32,768-update horizon and
1,024-update checkpoint interval. Only cosine learning-rate endpoints changed,
from `0.001 -> 0.00005` to `0.0002 -> 0.00001`.

The original-rate [selection](2026-09-22-owned-nnue-probe5m-selection.json)
is SHA-256 `584d20922a037b0b8104308bc697adfa3c3fcf3b57b5a2a1c170e0f8cd7e6916`.
It selected update **1,024**, integer validation MSE **0.008889**, and
retrospectively stopped at update 9,216. Its model is SHA-256
`d5262a2157c74f13463bbb35fbf81341c154f96fd30867dbfe0b22d9ee6b9459`.
The paired lower-rate [receipt](2026-09-22-owned-nnue-probe5m-lr1-receipt.txt)
and [selection](2026-09-22-owned-nnue-probe5m-lr1-selection.json), the latter
SHA-256 `525516a6da0a432c891c24208caaee66043c18cc005b8194d178747db8e0f3bb`,
selected update **3,072**, integer MSE **0.008464**, and retrospective stop
11,264. Its model is SHA-256
`55d109e9ebc14fe5836974ed07a8d7f913c58b066408fe921462df274bd2885a`.
Both runs completed their full training horizons for the diagnostic curve;
later checkpoints had higher validation loss. The lower-rate update 1,024
failed the frozen quantization eligibility rule, so it was not selected.

## Frozen 100k static calibration

The original-rate [analysis](2026-09-22-owned-nnue-probe5m-static-analysis.json)
is SHA-256 `629020eada4e5991e04a934fc35687c3be7443fa74d4331b28c58a529ff40278`.
The lower-rate [analysis](2026-09-22-owned-nnue-probe5m-lr1-static-analysis.json)
is SHA-256 `828836d44109e47e40b0d9b79d5c3117b3ff4940f262fa78cc171e0fa6bb15c2`.
Their [prediction receipt](2026-09-22-owned-nnue-probe5m-static-receipt.json)
and [paired prediction receipt](2026-09-22-owned-nnue-probe5m-lr1-static-receipt.json)
bind the exact models and prediction CSVs retained on WSL.

| Evaluator | MAE (cp) | WDL MSE | Spearman | Sign accuracy, teacher magnitude ≥50 |
| --- | ---: | ---: | ---: | ---: |
| HCE | 152.70 | 0.011296 | 0.6845 | 80.83% |
| 1M LR1 | 150.55 | 0.010957 | 0.6848 | 81.05% |
| 5M original rate | 142.98 | 0.008930 | 0.7500 | 84.99% |
| 5M lower rate | **135.78** | **0.008511** | **0.7669** | **86.00%** |
| Borrowed Rodent Anand | 133.13 | 0.008559 | 0.8352 | 89.30% |

In the pre-existing teacher-score bands from -100 through +99 cp (43,524
positions), 5M lower-rate MAE is **84.28 cp**, versus 86.06 for 5M original
rate, 94.12 for 1M LR1, and 68.74 for HCE. All output heads except the rare
head 0 now beat HCE's MAE; head 0 contains only 237 calibration positions and
remains 260.2 versus HCE's 249.7 cp. Position-level static improvement is not
playing-strength evidence by itself.

## Frozen 32-position search panel

The original-rate [panel](2026-09-22-owned-nnue-probe5m-search-panel-result.json)
is SHA-256 `eca320dc77d34aa39840bc9dc44e4542318b41338c2fd6cc614ab1cd333f5fa4`.
The lower-rate [panel](2026-09-22-owned-nnue-probe5m-lr1-search-panel-result.json)
is SHA-256 `cb57ed14b850e496ed80df510c7b750ecac3f5b716f95181d409ac2dd7264075`.
Both used the same 32 frozen FENs, SF18 reference, HCE and Rodent controls,
and AVX2 engine. The lower-rate [launcher](2026-09-22-owned-nnue-probe5m-lr1-panel.sh)
is SHA-256 `9ec83f7551bd1b73faa43c68a396654b92372114eda7f24bc1186a567cc88c1f`.

| Same 32 positions | HCE | 5M original rate | 5M lower rate |
| --- | ---: | ---: | ---: |
| Equal-node teacher move agreement | 15/32 | 13/32 | **17/32** |
| Equal-time teacher move agreement | 17/32 | 14/32 | **19/32** |
| Equal-time mean reported nodes | 222,163 | 127,508 | 126,043 |

In the lower-rate equal-node panel, K4 alone matched the teacher on seven
positions and HCE alone on five. Equal-time results have the same seven/five
split. Thirty-two positions are a small diagnostic, and K4 still searches
substantially fewer nodes at equal time. The paired static and panel gains
justify the already specified 40-game screen, not promotion.

## Real-clock gate

The [HELD manifest](2026-09-22-owned-nnue-probe5m-lr1-screen-held.json) is
SHA-256 `9752d40ff4862b53849de299b490d111c4a95c553090e777166f4020aedde0a5`.
Its normalized review subject is
`69d91d5314f71d613eaf114e7c3327f9fd46459c28dbc3965d586a4b7fec800c`.
The [launch script](2026-09-22-owned-nnue-probe5m-lr1-screen-launch.sh),
SHA-256 `1456afe967e469d31bdd0d315ca84c7623168f638e06e6c0775453c66ed37df7`,
approved only that reviewed subject and kept the approval token out of Git.
The match uses the same clean-source AVX2 engine for both roles, 20 reused
opening pairs, 10+0.1, one worker/thread, Hash 128, no book, tablebase,
adjudication or recovery, and independent game and process audits. Reused
openings aid direct comparison but do not create independent rating evidence.
The predeclared cutoff is **35%**: below it holds the recipe; at least 35%
permits further expenditure, but does not establish a 3k rating.

The [terminal receipt](2026-09-22-owned-nnue-probe5m-lr1-screen-terminal.json),
SHA-256 `fec31b043fb72d9a2758fde1ecd036a06428678b20be0ced6bb7ff2f399395c9`,
records 40 complete games, 20 pairs, and both independent audits passing at
06:34:55 UTC. The [chess audit](2026-09-22-owned-nnue-probe5m-lr1-screen-audit.json),
SHA-256 `13f652f15a93231a04cb887c684165bad393c675331829d444a1aa71f7d2e95a`,
checked 6,322 legal plies, all natural terminal states and zero probable
embedded-book signature plies. Its PGN SHA-256 is
`aee4983e7ea1db48fc59da9f310df08533744dd6bb5dbe60f1e9be61c066932a`.
The [operational audit](2026-09-22-owned-nnue-probe5m-lr1-screen-audit-operational.json),
SHA-256 `591685764ac6b600f1cddc53cd24acb10bda8ad188eabc9b3ce2e21b48535c4a`,
passed; the [final run state](2026-09-22-owned-nnue-probe5m-lr1-screen-state.json)
binds the output-file digest.

The owned 5M lower-rate model scored **26.5/40 (66.25%)** against HCE:
**22 wins, 9 draws and 9 losses**. It scored 11/20 as White and 15.5/20 as
Black. This is above the predeclared 35% threshold and sharply better than
the 1M LR1 screen's 8/40, but 40 reused-opening games cannot provide a
precise strength or absolute-rating estimate. The result authorizes the
20M expansion; subsequent 20M play must use a disjoint game set. The owned
NNUE still needs the frozen 400-game confirmation and external comparison
specified in the original plan before any 3k claim.
