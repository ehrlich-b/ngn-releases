# K4 20M labeling recovery and lower-rate training launch

At 06:50:02 UTC the first 20M label service exited after completing only
`main-expansion-train-000050`: its progress receipt recorded one new shard and
4,414,183 accepted expansion records, while the frozen queue still had 187
entries. The label, teacher and BF pack receipts for shard 50 agree on 87,117
accepted records. Its label and BF bytes were rehashed against those receipts.
No shard 51 output existed at recovery. A queue consumed through the shell's
shared stdin was vulnerable to a child process reading ahead. The recovery
loads the frozen queue into Bash memory and sends label/packer stdin to
`/dev/null`.

The [resume script](2026-09-22-owned-nnue-main20m-label-resume1.sh) is SHA-256
`c7204ca348f243dad78fb23dc64e16662ac4e4713f60426fc2463b0292cb9298`.
It requires the previous service to be terminal, preserves shard 50, starts at
frozen shard 51, retains the original resource and teacher gates, and stops
after at least 19M accepted expansion records. The user service
`ngn-k4-main20m-label-resume1-20260922.service` was active at 07:02 UTC with
two pinned label workers processing shards 51 and 52. The run's `STATE` was
`LABELING`; its original queue SHA-256 remained
`f7222c1972d0e7010756acb75100e1381c574bd0835d682d034daec51a2aa78c`.

The prospective [20M lower-rate plan](2026-09-22-owned-nnue-main-m1-lr1-plan.md)
selects `main-m1-lr1` before the corpus or games exist. An isolated, offline,
CUDA-enabled trainer build passed on the pinned Bullet/CUDA toolchain. The first
build attempt stopped before compilation because its pilot-only Cargo vendor
lacked `montyformat`; the [successful retry](2026-09-22-owned-nnue-main-m1-lr1-build-retry1.sh)
used the complete pinned vendor. Its [receipt](2026-09-22-owned-nnue-main-m1-lr1-build-receipt.txt)
records trainer SHA-256
`06fd95b42fe85f7b7c0999cbc6089666627881d638f1270466925d715dbdf114`
and bridge SHA-256
`3c9e9c9df636968c6a3a0dd61d5f9cd4ae991f0aebc221170f24b10cbbcda906`.
The finalizer and static evaluator hashes were independently checked on WSL.

The [postlabel script](2026-09-22-owned-nnue-main-m1-lr1-postlabel.sh) is
SHA-256 `9d14bd27b8390019b67f50b09d41a540a6cf3d1059b4afc5322bbb52bd84715d`.
The first watcher exited immediately when it saw the original label failure;
it created only a `FAILED` state file and performed no finalization or GPU
work. The [rearm wrapper](2026-09-22-owned-nnue-main-m1-lr1-postlabel-retry1.sh)
archived that empty attempt and started
`ngn-k4-main-m1-lr1-postlabel-retry1-20260922.service`. At launch, its new
run was `WAITING_FOR_LABELS`. The watcher uses inotify, then checks pinned
identities, hopper/GPU/disk gates, finalizes the exact 20M BF, trains on CUDA,
selects from frozen validation, and scores the inherited 100k calibration set.
It does not start a game screen. That requires a separate static/search review
and fresh disjoint openings after one candidate is frozen.

This is an active run, not a result or a 3k rating claim. Its completion
signals are `LABELED` for the label run and `CALIBRATED` for the dependent
training run; `FAILED` requires inspection of the actual service and retained
artifacts before any new attempt.
