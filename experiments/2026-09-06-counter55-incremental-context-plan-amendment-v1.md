# Incremental-context plan amendment v1

This additive amendment preserves
`experiments/2026-09-06-counter55-incremental-context-plan.md`.

1. Arithmetic error bookkeeping is test-only. Production `Context` stores
   only the model, board frames, float32 accumulator frames, and current
   frame. It contains no operation counts, float64 bounds, or exact-rational
   state. The independent oracle test reconstructs path updates and computes
   all drift bounds outside production.
2. `Board` is a public overlap-free 12-plane representation and may contain
   up to 64 occupied squares. The earlier 667-operation worst case applies
   only to a separately validated chess root with at most 32 pieces and 127
   ordinary five-row pushes. Public-API tests and receipts use the actual root
   occupancy and actual update count. General bounds are calculated from
   `popcount(root)+sum(updateRowsOnLivePath)`; no 32-piece assumption is made
   by `countereval`.
3. The fixed 128-frame context is a Counter-compatibility target for this
   slice. It is not evidence that NGN search recursion is bounded safely.
   Before integration, the adapter must either prove its reachable recursion
   bound is at most 127 pushes or move to checked dynamic growth. Search must
   never depend on an unchecked Counter array overflow.
4. `MoveDelta` validation is semantic rather than a second chess legality
   engine. It checks plane/square ranges, occupancy, mover/capture colors and
   types, promotion/castling field consistency, and exact derived-post-board
   equality. Legal move generation and checks, pins, castling rights/path,
   promotion rank, and king safety remain responsibilities of the upstream
   oracle and later NGN engine adapter.

5. `NewContext` performs one cold conservative safety check before publishing
   a context. For each hidden lane it bounds the root accumulator plus four
   maximum-absolute feature-row updates for each of the remaining 127 frames,
   then bounds every output product and their cumulative sum under the existing
   `MaxFloat32/4` margin. This uses transient float64 locals only. It stores no
   bound state in `Context`. A format-valid model that passes the loader's
   full-refresh bound can therefore still be rejected for unsafe incremental
   capacity; `EvaluateRaw` never becomes the first place such a failure is
   observable.
