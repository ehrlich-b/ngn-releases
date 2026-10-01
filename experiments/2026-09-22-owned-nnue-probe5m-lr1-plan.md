# K4 5M lower-rate paired probe

Status: frozen before training. The first 5M probe still labels and trains at
the original `0.001 -> 0.00005` cosine learning rate. Its selected model and
search panel will complete before this follow-up starts.

The 1M LR1 experiment improved both frozen validation integer MSE and 100k
static calibration relative to the original pilot, while its 40-game HCE
screen is underway. A separate `probe5m-lr1` run will test whether that
learning-rate effect persists with more distinct positions. It will cold-start
from the same seed 26092001 on the byte-identical finalized 5M BF used by
`probe5m`: the exact pilot 1M prefix plus four million accepted expansion
positions, with the same validation, calibration and reserved-test corpora.

All other settings match `probe5m`: unchanged K4 graph and score-only SF18
target, batch size 16,384, 32,768 updates, 1,024-update checkpoints, two loader
threads and no result blend. Only the cosine endpoints become
`0.0002 -> 0.00001`. A separate trainer binary and `probe5m-lr1` checkpoint
receipts prevent mixing with the original 5M run. The selector retains the
8,192-update minimum early-stop point, eight-check patience, `1e-5` improvement
gate, strict quantization eligibility and lowest validation integer MSE rule.

The run waits for `PANEL_STATE=PANEL_COMPLETE` from the original 5M probe. It
then verifies the finalized manifest, exact train BF, binary SHA-256s, idle GPU,
active Lean hopper and disk gates before training. It scores the selected model
on the inherited frozen 100k calibration set. Compare paired validation and
static results first; run an equal-node panel only for a candidate with
positive quality evidence. Neither this probe nor the LR1 1M screen starts
the held 20M label expansion automatically.
