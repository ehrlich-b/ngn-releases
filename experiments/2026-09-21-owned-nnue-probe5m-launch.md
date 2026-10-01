# Owned K4 5M data probe launch

Status at 2026-09-22 01:44 UTC: the frozen main-expansion label job and its
notification-driven post-label continuation are running on WSL. This is a
diagnostic run; no network was promoted and no playing-strength claim follows
from launch.

The authoritative run directory is
`/home/ehrli/nnue-owned-k4-20260920/runs/k4-probe5m-20260921`.
The sampler manifest SHA-256 is
`53d240d4948a165a46fa42875f825edef79eb0f36a33e580a27cb562c56d4138`.
[The exact label script](2026-09-21-owned-nnue-probe5m-label.sh) has SHA-256
`3afafce1081f1ecd58cbb49c88cf4eedd32a2921129622d46c2554969d74b851`.
It froze the first 50 ordered, disjoint main-expansion shards (5M candidate
positions) in `queue.tsv`, SHA-256
`1bbd0437ba86c1dd01778e91a9ad32e99a786700b733978cd4a268d479fe996a`.
The preceding pilot used 1.2M candidates for 1,038,134 accepted positions, so
50 shards are expected to supply more than the required 4M accepted records.
The exact count is checked from all 50 pack receipts before finalization.

The label service is `ngn-k4-probe5m-label-20260921.service`. It uses the pinned
SF18 executable SHA-256
`174270346ae9ed600713d165fa745dfa87fc084e44907c8084767f2b905e86d3`,
the existing labeler SHA-256
`cfd238f70fcb9ebec7e26415ff8b3cf09e62230cc840f068ea2b0d2916ee9db7`,
and packer SHA-256
`cd1cf0a4f2676f219cc2ea398b97753afa2516d626007048cf9e5a925bb13e25`.
Two workers are confined to CPU cores 10 and 11 at nice 10 while the Lean
hopper stays active. The preflight found 898.6 GB free in WSL and 286.5 GB
free on physical Windows C:, above the frozen 40/60 GiB gates.

[The post-label continuation](2026-09-21-owned-nnue-probe5m-postlabel.sh),
SHA-256 `f25c4222b8e20e218dafe8cdc193d562f5e959e07b1d8eca6e82c7102a2c0bbf`,
runs as `ngn-k4-probe5m-postlabel-20260921.service`. It waits on the label
`STATE` file using Linux inotify, without a polling loop. After `LABELED`, it
checks the 4M accepted threshold, verifies the staged finalizer, bridge and
trainer binaries, then automatically:

1. Finalizes an exact 5M BF with the original 1M pilot as its byte-identical
   prefix, inheriting the original validation, calibration and reserved-test
   corpora.
2. Trains the unchanged K4 graph for 32,768 updates at batch size 16,384,
   retaining checkpoints every 1,024 updates.
3. Selects on validation integer MSE under the predeclared 8,192-update
   minimum and eight-check patience rule.
4. Converts the selected checkpoint and scores the frozen 100k calibration set.

The finalizer, bridge and trainer executable SHA-256 values are respectively
`3c22fb56f119cab21f8581e89193303f728d8b37452f9cf857ab100f19f23908`,
`5cb49c97c133f73011b2e6917669bec833510ec4d14cb411ed1321e7ba5cd035`,
and `6a989de4ca6746b2f7c10938206ed3a2e89d1c50beef580ff08399cafa805f0b`.
The finalizer's small fixture test proved the 5M mode is the exact prefix of
the main-mode accepted stream and inherits all three fixed holdouts. The
pinned offline CUDA build and focused Go tests/vet passed. The post-label
service refuses another GPU compute job at the training gate, enforces the
same disk thresholds, and stops on a failed receipt or command.

Completion is signaled by `POSTLABEL_STATE=CALIBRATED`; `FAILED` names a stopped
continuation whose last stage and stderr log should be inspected. Neither
service needs status polling while the label job is active. The next read is
due near the expected multi-hour completion or after a failure notification.

At 2026-09-22 02:03 UTC, a third user service,
`ngn-k4-probe5m-panel-20260921.service`, began waiting on the post-label state
file via inotify. [Its pinned script](2026-09-21-owned-nnue-probe5m-panel.sh)
is SHA-256 `b2ffd37a948ff99d661dc1f0ac9a161ae403c5c7f379bc7b5ae8f1f34e0abf24`.
After `CALIBRATED`, it will run the existing 32-position HCE/K4/Rodent/SF18
search diagnostic using the selected 5M model and the verified AVX2 engine.
The positions, budgets, teacher, Rodent net and original UCI runner are pinned;
the derived panel records the selected model and optimized engine SHA-256.
`PANEL_STATE=PANEL_COMPLETE` will indicate a finished search receipt under
`search-probe5m`. This diagnostic does not start a game screen or the 20M
expansion; those decisions depend on the resulting quality evidence.
