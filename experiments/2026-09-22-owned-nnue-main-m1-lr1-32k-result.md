# K4 20M short-schedule result

Status at 2026-09-23 03:13 UTC: training, selection, static calibration and the
fixed 32-position search panel completed. The new model remains diagnostic;
no game-strength promotion is claimed. The original 20M model's separate
400-game HCE confirmation was launched afterward.

The [frozen plan](2026-09-22-owned-nnue-main-m1-lr1-32k-plan.md) changed only
the cosine horizon from 131,072 to 32,768 updates, keeping the 20M BF,
seed, architecture, score target, batch size, LR endpoints and holdouts. The
first launcher stopped before training on Rust formatting; retry1 stopped
before training because it selected the incomplete `pilot-vendor`. Retry2
used the pinned full offline vendor and completed. The
[receipt](2026-09-22-owned-nnue-main-m1-lr1-32k-run-receipt.txt) and
[completion](2026-09-22-owned-nnue-main-m1-lr1-32k-training-completion.json)
bind its executable, corpus and 32,768 updates.

The frozen [selection](2026-09-22-owned-nnue-main-m1-lr1-32k-selection.json)
chose update **28,672** with validation integer MSE **0.007016925086052009**,
versus **0.007169997081706572** at the original selected update 20,480. All
eight trained checkpoints were measured; update 32,768 rose to 0.00702735.
The selected `.nnue` is SHA-256
`6f9bd70e786f3ecb93c48e702404b00cfc1463417c7998478964035b7f0b6332`.

The inherited 100k [calibration](2026-09-22-owned-nnue-main-m1-lr1-32k-static-analysis.json)
improved overall K4 MAE from **130.18 to 127.78 cp**. On the 237-position
output head 0 it improved from **345.37 to 282.40 cp**. In the teacher
`-50:49` cp band it improved from **78.67 to 77.06 cp**, while HCE remained
66.60 cp and borrowed Rodent 40.43 cp in that band.

The fixed [search panel](2026-09-22-owned-nnue-main-m1-lr1-32k-search-panel-result.json)
gave the new model SF18 move agreement of **15/32 at equal nodes** and
**14/32 at equal time**, versus the original model's **17/32** and **21/32**.
The equal-time regression is a warning, though 32 positions cannot settle
playing strength. The loss and static gains are real on their measured sets;
they have not yet shown a game-strength gain. Keep the original model frozen
for its predeclared 400-game confirmation. A later direct same-code match is
needed to decide whether the short schedule is a playing improvement.

Both selected `.nnue` files and both complete selected checkpoint directories
were copied off WSL to
`/Users/ehrlich/repos/ngn/output/nnue-owned-k4-backup-20260922/` (ignored
by Git). Original model SHA-256 is
`cebec29cbab676af6a2ade2b67da5c9a01ddb7882ed604238c0e7d67893f72d6`;
new model SHA-256 is above. Original checkpoint TAR SHA-256 is
`1059e83b37fc78eaf1c5aabbc80bd96557a53825fa1c3818ba089d8d20c15f88`;
new checkpoint TAR SHA-256 is
`b798ab5bf42e4e57179c446b86edc41b77b634efb7fa282e7c0899b83852efdb`.
Both TARs matched their source hashes and every contained file matched its
checkpoint receipt. The full 20M corpus and all 131k-run checkpoints remain
on WSL; the selected models and resumable selected checkpoints have the
verified off-host copy.
