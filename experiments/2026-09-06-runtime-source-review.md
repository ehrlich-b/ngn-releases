# Runtime and data source review — September 6

Historical source-review record, supplemented by [the accepted TT/NNUE foundation integration](2026-09-06-tt-nnue-foundation-integration.md). The corrected B0 A/A is now accepted and bounded correctness work is released. Statements below about pending M3/N3/N4 execution describe the earlier review boundary. All implementation and source inspection described here occurred on WSL; the Mac only transports commands and text. Accepted playing deployment at /home/ehrli/repos/ngn remains 53e4d1b.

## Frozen candidates awaiting execution

- M3 owned/coherent TT: incremental patch against bf750a4, SHA256 4d6cd446ff243f28838b5b87f472839df110e395473438b3ad3639102875ed84. Root reviewed production ownership, generation invalidation, callback snapshots, migrated callers, and final overlap/coherent-hit tests. Early checks preceded final test-source corrections; run final gates before integration.
- P0 explicit removal of the implicit 50M node ceiling: commit f4e2d38; patch SHA256 5e1a6f8133e5b86f959d580da29c5ec7820f6c01a1b8e83a9babfc10f0bd72e0. The test seeds the counter above the old ceiling and takes the existing depth safety return, avoiding a 50M-node test. Explicit node-budget cancellation remains independently asserted.
- L0 completion sessions: commit e629ee6; patch SHA256 376a4532c28b03ea2003607a8486303ab9b7d0aa9d4bec70aaaf85660d2e2ffa. Root reviewed prepared/running/stopping/idle publication, single cancellation, completion after final writes, no lifecycle mutex across output/join, and deterministic prepared-stop/duplicate-go/two-waiter/blocked-output controls.
- N3a semantic engine-to-NNUE transitions: patch SHA256 1b2c8a0c2a87f52f398bb4c4dd4c4c2e1340411dab5e97f9a33123c6beaf8b31.
- N3b concrete per-worker evaluator: patch SHA256 019ab8e5cef612127cd7e16eb4df5fce7cdd797b32c5cff21489ec3a349afe92.

N3a/b are frozen in /home/ehrli/repos/ngn-nnue-runtime-n3a/output/nnue-runtime-n3-20260906, based on 441de61. They do not yet change search, UCI selection, control, or TT. Root read their production implementation and tests. They need formatting, focused/full/race checks, vet/build, and the corrected deterministic HCE reference before integration.

## Concrete review findings addressed

The compact real-move adapter originally predicted After from Before and submitted that prediction without observing the actual engine mutation. It now compares actual post-MakeMove facts and side to move before committing. Occupancy/pawns/kings alone cannot distinguish a queen promotion accidentally made as a rook; affected mailbox additions and removed-but-not-readded empty squares are checked too. Null transitions capture all twelve piece bitboards so a same-occupancy non-pawn type/color change cannot masquerade as a null move. En-passant capture codes are range checked before piece accessors. Independent full lane reconstruction covers special moves; full-position Push/PushNull success coverage is retained alongside compact API coverage.

N3b's immutable identity includes backend, adapter revision, full NNUE metadata/digests/contracts, and HCE/PST generation. Workers share only the immutable model. Ordinary NNUE scores remain int64 through exact overflow-safe rule-50 attenuation, are clamped to +/-25000, then converted to int. Emergency returns perform fresh model evaluation with no attenuation and the same clamp. No HCE tempo, perspective flip, phase blend, or full-evaluation cache enters this route. Three bounded correction terms add at most 144, leaving the result below the 29000 mate band. Raw package inference parity remains unchanged.

The data verifier originally did not independently link its reconstructed features/labels to the binary records read by training. It now reconstructs the canonical 32-byte Bullet record from literal FEN and source STM labels, compares both sidecar board and actual BF bytes, and rejects jointly corrupted BF/sidecar records even when their receipts are regenerated. Conversion and first-threshold reconstruction maintain incremental split counts; chain-local key dedup and one-time historical quarantine avoid repeated full-prefix scans. Synthetic fixtures and the durable bounded wrapper remain source work; no T80 decoding or real-data training has run.

## L1 review constraints

Move timing begins at go receipt, before logging/parsing/setup. Coordinator setup owns a root copy, private TimeManager, cancellation/completion and output policy; cancellation and hard-deadline checks surround setup. Board fallback/make-unmake also reads hidden HCE PSTs and must be covered by a read lease before the inner prepared-search admission.

Preserve existing allocation order: SetTimeControl currently computes tournament limits using the previous manager's game-phase/move-count state, then UpdateGameState records current state. A fresh default manager would silently reset that policy input. Snapshot the existing policy state and preserve the order while moving clock origin. Correcting the stale-state order, if desired, is a separate policy experiment. A two-search fixture must demonstrate preserved budgets.

Explicit stop publishes the active result once. Replacement/EOF/quit suppress stale results and join. All failure paths complete, retain an error, and publish a legal fallback where publication is appropriate. No helper search is authorized by L1 alone.

## Post-control execution order

After all 200 control games, independent audit, final artifacts and empty terminal-process receipt pass, release bounded correctness work. Validate/integrate final M3 first; then validate N3a/b and P0/L0 independently. N3 search wiring must cover iterative and fixed roots, null, probcut, main PVS, qsearch evasions/captures, and singular parent pop/re-push. Reporting PV walks and board-only legality probes do not advance NNUE.

N4 portable int32 contexts are a separate source slice with unchanged independent int64 lane oracle and unchanged scalar/full evaluation; assembly follows a separate review and parity gate. Real data conversion follows private locked dependency provisioning and passing synthetic converter/verifier controls. No timed match overlaps builds, tests, training or data processing.
