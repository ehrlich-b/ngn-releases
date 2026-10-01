# Owned K4 teacher-quality gate

Status: **PASS_BOUND for the predeclared 0.05 mean target-difference hold.**
The 20k search yielded ordinary exact CP comparisons on 945/1,000 positions;
45 had a last-completed-PV versus final-best-move mismatch and 10 yielded mate
scores. The bounded result below keeps these 55 cases in the denominator. It
does not turn mate scores into ordinary training labels or relax the production
labeler's 5k acceptance rule.

## Frozen inputs and execution

- Finalized pilot manifest SHA-256:
  `753e8ed1fbe09bef290fcba7209dfb77dce0170e80a5a09a22a1c3c12599755e`.
- Calibration BF: 100,000 accepted positions, SHA-256
  `98de4eb3382c6face5cd92d9194d3271ec0fb6aff1ee30405f94846ddfaceb60`.
- Sample: the first 1,000 records by
  `SHA256("ngn-k4-teacher-quality-sample-v1\0" || accepted_label_id)` across
  the entire frozen calibration accepted set. The sampled tuple stream SHA-256
  is `c752df471d55ff0fd9ccc729ef759c56e25aa62d6a2f7cdddcd0b5053c811bbe`.
  The hash rule ignores score, position content, model prediction and game
  outcome; it avoids taking adjacent records from the first encoded chains.
- Teacher: pinned official SF18 source and networks from the existing 5k label
  header. Executable SHA-256:
  `174270346ae9ed600713d165fa745dfa87fc084e44907c8084767f2b905e86d3`.
  The 20k diagnostic retained Threads 1, Hash 16 MiB, MultiPV 1, no tablebases,
  `ucinewgame`/`isready` before each root and the exact score conversion. It
  changed only the node budget to 20,000 and per-root timeout to eight seconds.
- One low-priority WSL CPU process ran on CPU 12, with no GPU work. Windows C:
  had 289,237,807,104 free bytes before this bounded run. The Lean hopper was
  still running. The final audit completed in about 17 seconds.

The authoritative receipt is
`/home/ehrli/nnue-owned-k4-20260920/runs/k4-teacher-quality-hash-bound-20260921/audit.json`,
SHA-256 `452f84bc8be81e5ab681c736241d7f9d1d7b54a3830622b0897c7d9ffb4b93b8`.
The Linux diagnostic binary SHA-256 was
`53ae679bdca5d8a7564cb625cb3d61591cc5a1a97125089387f427d98ea365dc`.
The earlier first-1,000-in-stream audit and initial hash-sample audit remain
separate, unmodified diagnostic receipts. The final hash sample was identical
between its two runs and gave the same 945 exact scores and summary metrics.

## Result

| Measure | Result |
| --- | ---: |
| Ordinary exact CP comparisons | 945 / 1,000 |
| Mean absolute WDL-target difference on those 945 | 0.01767346 |
| Median / p90 absolute difference | 0.01092892 / 0.04300482 |
| Spearman score correlation on those 945 | 0.97993388 |
| Sign disagreement, 5k score magnitude at least 50 | 13 / 686 |
| Sign disagreement, 5k score magnitude at least 100 | 3 / 509 |
| PV/final-best-move mismatch at 20k | 45 |
| Mate score at 20k | 10, all same sign as the 5k score |
| Conservative full-sample mean upper bound | **0.04306377** |
| Predeclared hold threshold | 0.05 |

For the 45 PV mismatches, the upper bound allows *any* target in `[0,1]`, so it
does not assume that the last exact completed-depth CP is the final 20k label.
For the 10 mate scores, it maps a positive forced mate to target 1 and a
negative forced mate to target 0. All 10 matched the 5k score sign, and their
5k targets were already close to those boundaries. Under this explicit mate
convention, even the worst possible PV-mismatch values keep the full-sample
mean below 0.05. Without a mate-target convention, 20k mate scores have no
finite CP target, so a literal 1,000-pair ordinary-CP mean is undefined.

The 5k labels are therefore adequate for the *specified mean-difference gate*.
This does not establish that the teacher is optimal, that sparse buckets are
well learned, or that the failed K4 candidate is strong. The next discriminant
is the frozen 100k static calibration comparison against the selected K4 model,
HCE and borrowed Rodent, followed by equal-node/equal-time search checks.

## Shared-host correction

The prior explanation for the host running hot was wrong. The user found a
pinned `sshd` process using the CPU and killed it. Our Windows SSH endpoint
initially timed out; after the service was restored, SSH and WSL responded.
The earlier heat must not be attributed to K4, the Lean hopper or another job
without process evidence.
