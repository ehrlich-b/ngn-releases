# Owned K4-S1 schedule probe

Status: **failed the validation and static calibration gates.** The frozen
20M expansion remains held. The next discriminator is the predeclared 5M
unique-data probe on the same architecture and SF18 target.

## Frozen input and run

K4-S1 cold-started from seed 26092001 and used the exact pilot training BF
(`467d3876c6d670f66d6ac5986c00e3e1c7f34843ae87653c6c578bf1c06228e8`)
and finalizer manifest
(`753e8ed1fbe09bef290fcba7209dfb77dce0170e80a5a09a22a1c3c12599755e`).
It retained the model, score target, batch size 16,384, AdamW settings,
quantization and sequential data order. Its cosine horizon was extended from
1,024 to 16,384 updates, with checkpoints every 512 rather than 128 updates.
The trainer binary SHA-256 was
`e0ad175d2ce7c12ce1c92560821ff05271116f956a865680047e553633634550`.

The WSL run at
`/home/ehrli/nnue-owned-k4-20260920/runs/k4-s1-20260921` completed all
16,384 updates and 268,435,456 presentations in 61 seconds, with all 33
checkpoint receipts. Its completion receipt SHA-256 is
`41e9ec3372e5008486b6d91a3e223d837906977bcfbaa56d50bb09eabe6a670b`.
The Lean hopper remained active; CPU affinity was 12–15 at nice 10. Windows C:
free was 288.2 GB before and 287.3 GB after the run. No competing GPU compute
process was present at launch.

## Validation-only selection

[The full selector receipt](2026-09-21-owned-nnue-s1-selection.json) has
SHA-256 `cdbd62d7519a97edb79979c01f2c42087b5ed403de2408981cb95e07657a0ce1`.
All 32 trained checkpoints passed eligibility, but validation integer MSE was
already worse at the first checkpoint and increased across the longer run:

| Checkpoint | Pilot integer MSE | K4-S1 integer MSE |
| ---: | ---: | ---: |
| 0 | 0.026957 | 0.026957 |
| 512 | 0.011531 | 0.012274 |
| 1,024 | 0.011393 | 0.013631 |
| 4,608 | — | 0.015997 |
| 16,384 | — | 0.017226 |

The prospectively declared S1 rule required at least 4,096 updates and eight
checks without a `1e-5` integer-MSE improvement. Its retrospective stop point
was update 4,608, and it selected update 512. Training ran to the full horizon
to preserve all diagnostic checkpoints; this stop rule only governs selection.

## Held-out static calibration

The selected update-512 model SHA-256 is
`d8595e0f4c945b4dedffe42d9ab6c5ac73a8af41ef21c89314c62a75c3eff0aa`.
The scorer verified the exact 100,000-position calibration BF and label shards
used in the prior comparison. Its predictions CSV SHA-256 is
`1e05a37fd6c830069788ae036eeee5f62f2e4cb90992d227c184629170dd3aba`;
its receipt SHA-256 is
`38cd44ce7c2dddbe5c637a01288755b480dc5f8d65224dd042091145ceebe084`.
[The analysis](2026-09-21-owned-nnue-s1-static-analysis.json) has SHA-256
`d9b5c074ebeda95a4731e2470e45e140f2f6797e96df50865f1686746c29b8b6`.

| Evaluator | MAE (cp) | WDL MSE | Spearman | Sign accuracy at teacher magnitude ≥50 |
| --- | ---: | ---: | ---: | ---: |
| HCE | 152.70 | 0.011296 | 0.6845 | 80.83% |
| Original K4 pilot | 157.92 | 0.011457 | 0.6691 | 80.13% |
| K4-S1 selected | 164.05 | 0.012426 | 0.6348 | 78.43% |
| Borrowed Rodent Anand | 133.13 | 0.008559 | 0.8352 | 89.30% |

S1's low-material head 0 remains weak: 422.8 cp MAE on only 237 examples,
versus HCE's 249.7. It also worsened the global calibration error by 6.1 cp
relative to the original pilot. Calibration positions can share source chains,
so these 100,000 rows are not 100,000 independent games. The selected S1 net
was not sent to game screening because both prior quality gates failed.

## Interpretation and next run

This experiment rejects the longer cosine schedule as specified. It does not
isolate unique-data starvation from the high learning rate persisting longer:
those are coupled when the cosine horizon changes. The broader 5M data probe
tests whether more distinct positions help under the unchanged target and
architecture. Its first 50 frozen main-expansion shards, totaling 5M candidate
positions, began labeling on the two low-priority CPU lanes at
2026-09-22T01:34:10Z. The authoritative WSL run is
`/home/ehrli/nnue-owned-k4-20260920/runs/k4-probe5m-20260921`, with queue
SHA-256 `1bbd0437ba86c1dd01778e91a9ad32e99a786700b733978cd4a268d479fe996a`.
Finalization will use exactly the first 4M accepted expansion positions after
the byte-identical 1M pilot prefix and inherit the original three holdouts.
