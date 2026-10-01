# K4 20M lower-rate training plan

Status: completed through the fixed 400-game confirmation, including independent
chess and operational audits. The result and resume point are recorded in
[the 20M result](2026-09-22-owned-nnue-main-m1-lr1-result.md).
The primary candidate is `main-m1-lr1`.

The paired 5M comparison used the same exact BF and changed only cosine
learning-rate endpoints. The lower-rate run selected validation integer MSE
0.008464 versus 0.008889, static MAE 135.78 versus 142.98 cp, and matched
the SF18 reference on 17 versus 13 of the 32 equal-node panel positions. Its
audited 40-game HCE score was 26.5/40 (66.25%), above the predeclared 35%
expenditure gate. The original-rate 5M model failed to improve equal-node
agreement. These observations choose the lower-rate recipe prospectively for
the expensive 20M corpus; the 40 games do not establish an absolute rating.

`main-m1-lr1` consumes the exact finalized 20M `main` BF: the frozen pilot 1M
prefix plus the first 19M accepted expansion records, with the original
validation, calibration and reserved-test holdouts inherited by hash. It
retains the K4 architecture, score-only SF18 target, model seed 26092001,
batch size 16,384, AdamW, loader setup, full 131,072-update M1 horizon,
4,096-update checkpoints, and strict quantization/export gates. Its cosine
endpoints remain those of the successful 5M lower-rate run:
`0.0002 -> 0.00001`. This yields 2,147,483,648 presentations, or 107.4 per
unique 20M input. The selector cannot retrospectively stop before update
32,768 and uses six non-improving checks with the unchanged `1e-5` integer-MSE
improvement gate. It ranks eligible checkpoints by the same frozen validation
integer MSE; no checkpoint will be chosen after inspecting games.

After the exact BF passes finalization and resource checks, train in a new
no-clobber WSL workspace with the CUDA-enabled binary. Require its complete
checkpoint receipts, select one eligible model, verify Bullet/export/integer
parity, and score the inherited 100k calibration positions. Compare the
pre-existing score bands and output heads, then run the same 32-position
equal-node/equal-time panel. Hold for diagnosis if semantic, quantization,
static or panel quality collapses. If it remains viable, freeze one model
before a 40-game HCE screen on openings disjoint from the 5M pilot screen.
Only a 20M screen score at least 50% proceeds to the specified 400-game
30+0.3 confirmation. The owned-network milestone needs the 400-game paired
bootstrap 95% lower bound above 50%, then same-code borrowed evaluator and
actual-engine comparisons. An absolute 3k rating requires appropriate
external calibration beyond HCE-only match scores.

The previously implemented original-rate `main-m1` mode remains a distinct
historical control. It is not a fallback candidate to select after seeing
the lower-rate model's games.
