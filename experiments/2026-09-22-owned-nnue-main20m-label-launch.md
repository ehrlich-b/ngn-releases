# Owned K4 20M label continuation

Status at 2026-09-22 06:43 UTC: the label service is active; no 20M BF or
trained model exists yet. The [5M lower-rate game screen](2026-09-22-owned-nnue-probe5m-result.md)
scored 26.5/40 (66.25%) against HCE with both audits passing, above the
predeclared 35% expenditure gate. This authorizes the 20M data expansion,
not an absolute rating claim.

The [exact script](2026-09-22-owned-nnue-main20m-label.sh) is SHA-256
`0e03e8f518d47e1e5c18fe26a5085dc2cecf064e1749f7d2a01f6343fa172ead`.
The sampler manifest remains SHA-256
`53d240d4948a165a46fa42875f825edef79eb0f36a33e580a27cb562c56d4138`.
It lists 238 ordered main-expansion training shards with 23,750,028
candidate positions. The first 50 shards were already labeled and packed in
the 5M probe. This continuation links their completed artifacts into a new
run directory and begins at shard 50, with no relabeling or overwrite.

The authoritative WSL run is
`/home/ehrli/nnue-owned-k4-20260920/runs/k4-main20m-label-20260922`.
Its frozen `queue.tsv` is SHA-256
`f7222c1972d0e7010756acb75100e1381c574bd0835d682d034daec51a2aa78c`
and contains the remaining 188 shards. The initial 50 contain 4,327,066
accepted expansion records. After each two-shard batch, the script records
the accepted count and stops as soon as at least 19,000,000 expansion records
are available. The strict finalizer will use exactly the first 19M, yielding
the pilot's byte-identical 1M prefix plus 19M ordered expansion records.

The job uses the same pinned Stockfish 18 labeler and independent BF packer
as the 5M run, one process each on cores 10 and 11 at nice 10. It checks the
active Lean hopper and at least 40 GiB free in WSL and 60 GiB physically free
on Windows C: before every batch. Each label subprocess has a 45-minute
deadline, each packer a 10-minute deadline, and the systemd user unit has a
24-hour runtime cap. No timed match overlaps this label job. `STATE=LABELED`
is the completion signal; `FAILED` requires inspection of the live service
handle and the retained per-shard artifacts before any resume decision.

The service `ngn-k4-main20m-label-20260922.service` entered `LABELING` at
06:42 UTC with process ID 322192. The initial run receipt binds the sampler,
5M source receipt, queue, and accepted target. Exact finalization, training,
selection, calibration, disjoint game screening, 400-game confirmation and
external rating evidence remain required for the 3k objective.
