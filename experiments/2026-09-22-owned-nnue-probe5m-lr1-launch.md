# Paired 5M lower-rate probe launch

Status at 2026-09-22 02:51 UTC: the notification-driven user service
`ngn-k4-probe5m-lr1-postpanel-20260922.service` is active with
`STATE=WAITING_FOR_ORIGINAL_PANEL`. It consumes no training data or GPU time
until the original 5M probe reaches `PANEL_STATE=PANEL_COMPLETE`. The
[prospective plan](2026-09-22-owned-nnue-probe5m-lr1-plan.md) freezes the
single learning-rate change and all selection/calibration gates.

The [exact launcher](2026-09-22-owned-nnue-probe5m-lr1-postpanel.sh) is SHA-256
`5bb4044c8c5d83b2111d6cb25dfca78af0be0d568d673f098bec205420585db5`.
It waits for a Linux inotify file-change event, then binds the finalized 5M
manifest to the original run receipt and checks the shared BF and holdouts
through the strict trainer. It also verifies GPU availability, the active Lean
hopper, disk thresholds, absence of a fastchess process, and binary identities
before a new no-clobber training attempt. On success it trains, selects on
frozen validation and calibrates the selected model on the inherited 100k set.

- Bullet commit: `629ee50000b2afb7b3337595401c830d3b1e0f42`.
- Bullet patch SHA-256:
  `f7f5e0dd02695fe57ec58a630b63006da96cb1ec6b8fd4769a4e6308a8121e54`.
- Rust harness SHA-256:
  `952d9fbacf6b9afda2f1ea8a7fd66388fa8e08c30874bb3dd6b41d642855eca9`.
- CUDA-enabled trainer SHA-256:
  `5b22d6b763ffc20586cc8d1e0669bdbe1587b5cf6990332fb7b5d0b605132afc`.
- Linux bridge SHA-256:
  `29bad4d5beb8b92140233d18fb400702cb0fd7e4f365666aeb35e703aa46218f`.

Go bridge tests and vet, shell/Python parsing, and the pinned offline CUDA
release build passed. The original 5M trainer and its service were not changed.
The authoritative eventual run is
`/home/ehrli/nnue-owned-k4-20260920/runs/k4-probe5m-lr1-20260922`.
`STATE=CALIBRATED` is the next quality evidence; `FAILED` requires inspection
of that run and its systemd journal. No game screen or 20M expansion is queued
by this service.
