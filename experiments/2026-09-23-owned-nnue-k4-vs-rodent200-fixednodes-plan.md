# Owned K4 versus Rodent fixed-node diagnostic

Status: HELD before launch. This is a counterpart to the completed 200-game
10+0.1 [same-code comparison](2026-09-23-owned-nnue-k4-vs-rodent200-result.md).
No model promotion or absolute-rating claim will be made from this run.

Use the exact same optimized NGN executable for both roles, the same frozen
owned K4 and borrowed Rodent V1.1 Anand networks, the same 100 openings
4241–4340 with reversed colors, two concurrent single-worker games on CPUs
12 and 14, Hash 128 MiB, no embedded or external engine book, no tablebases,
no adjudication, and independent chess and operational audits. The one changed
axis is the search budget: **160,000 nodes per move for both roles** through
Fastchess `-each nodes=160000`, with no clock budget. That is the 160,854
median search-node count rounded down from the audited 10+0.1 PGN. Reusing the
same openings makes the two conditions easier to inspect but does not create
new independent opening evidence. The frozen
[HELD manifest](2026-09-23-owned-nnue-k4-vs-rodent200-fixednodes-held.json)
is SHA-256 `c8ebf895a25b144813b977ab9523f46113e3ec4f5ac0ae6eb4bc624511c328ce`;
review subject SHA-256 is
`0b602e6b7026e1477b9da2beba405b3d7fa92d6f6cbeaa0a276b8a209fd144a2`.

All 34 candidate-match WSL unit tests passed after adding the narrow
fixed-node manifest contract, and the changed runner, manifest and schema
hashes on WSL matched the local files. The Python validator requires
`node_limit` and forbids `time_control` for the new schema; the existing clock
schemas retain the previous budget behavior. The separate
[setup script](2026-09-23-owned-nnue-k4-vs-rodent200-fixednodes-setup.py)
reconstructs this HELD manifest from the earlier one and hashes the exact
source files.

At completion, require 200 legal games, 100 complete reversed-color pairs,
zero unexpected operational errors or survivors and the independent audit's
no-embedded-book check. Report K4 points, W/D/L and a 100,000-resample
opening-pair bootstrap 95% interval with seed 2026092302. Also compute the
fixed-node minus real-clock score difference over the same 100 six-ply opening
prefixes and give a 100,000-resample paired-bootstrap interval with seed
2026092303, using the already frozen 10+0.1 audit. A smaller
loss at equal nodes supports a throughput contribution; a similar large loss
keeps model/search-fit quality as the primary concern. The conditions differ
in effective depth and time management, so subtracting their Elo estimates
cannot establish an exact speed-caused Elo fraction.
