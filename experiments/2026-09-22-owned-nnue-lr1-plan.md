# K4-LR1 learning-rate isolation probe

Status: frozen before launch. The 5M distinct-data probe remains the primary
next quality gate; LR1 is a cheap independent diagnostic on the existing pilot
BF. It does not start or justify the 20M expansion.

K4-S1's 16,384-update run worsened validation from its first checkpoint. S1
changed both the cosine horizon and how long the original high learning rate
persisted, so its result did not isolate coverage from learning-rate behavior.
LR1 repeats S1 with only the cosine endpoints reduced fivefold, from
`0.001 -> 0.00005` to `0.0002 -> 0.00001`. It cold-starts from seed 26092001
on the exact pilot 1M training BF, uses batch size 16,384, the unchanged
K4 graph, score-only SF18 target, 16,384 updates and 512-update checkpoints.
The strict `pilot-lr1` receipts keep it separate from S1 and from the 5M run.

Use the inherited 100k validation BF and the S1 selector's integer-MSE rank,
4,096-update earliest stop point, eight-check patience and `1e-5` improvement
gate. Train through the full horizon to retain the diagnostic curve. Compare
the selected checkpoint with the original pilot's 0.011393 integer MSE and
S1's 0.012274. Only if LR1 improves on the original pilot validation score,
convert it and score the frozen 100k calibration set; a real-time game screen
still requires static and fixed-node evidence. Selection and calibration use
no 5M or reserved-test observations.

Before training, verify the exact pilot manifest and BF hashes, pinned Bullet
commit/patch, idle GPU, active Lean hopper and available disk. Run on the same
low-priority WSL CPU lane, without modifying either active 5M service or its
trainer binary. Stop the probe on any receipt or resource-gate failure.
