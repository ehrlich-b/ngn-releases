# Counter 5.5 search-integration boundary

Root source review, 2026-09-06. Design for the slice following accepted incremental compatibility. This document does not release builds or games during the exclusive multicore match window.

## Objective and scope

Make the verified pretrained Counter model selectable in NGN search through the existing worker-evaluator boundary. Preserve HCE and NGN-v1 behavior. Keep Counter's score adaptation explicit and independently tested; do not treat matching raw network output as matching search evaluation.

The current Counter loader/full-refresh package is accepted. The incremental context is being implemented separately. This integration should use those concrete types rather than add a general plugin framework or interface calls inside hidden-unit loops.

## Existing code seams

- `engine/worker_evaluator.go`: concrete tagged evaluator model, model identity, per-worker context, Reset/PrepareMove/PushMove/PrepareNull/PushNull/Pop, SearchSTM and emergency LegacyUndampedSTM.
- `engine/evaluator_selection.go`: transactional receiver-local selection, root refresh, history identity, and lazy TT invalidation.
- `engine/uci_evaluator.go`: staged file/model and active backend; UCI joins searches before selection or replacement.
- `engine/search.go`: search already routes ordinary, quiescence, null, singular re-search and emergency evaluation through the worker boundary. Reuse those calls; do not duplicate search logic.

## Model selection and identity

Add a named exact backend `counter-5.5`. Keep an immutable Counter model pointer and immutable load identity with the model itself. The current loader returns metadata separately, so a small prerequisite change must provide loader-owned metadata access. Selection must reject a zero-value/unvalidated model; caller-supplied digest strings are not validation.

Composite identity must include backend, exact file digest, score-adapter revision and the existing HCE/PST generation. Any score-changing selection invalidates TT/history before use. All worker contexts remain private; only validated model weights are shared.

Staging must continue to work in the existing EvalFile-then-EvalBackend order. Dispatch using an exact recognized format identifier followed by its strict loader; do not silently accept a different architecture after failure. Staging a valid inactive backend is allowed under HCE. Replacing an active network with a different backend's file must fail transactionally or require explicit selection of HCE first. Failed loads/selections preserve the usable active configuration. Clearing EvalFile while a neural backend is active remains rejected. Do not advertise SF18 as selectable until it can evaluate.

## State transition contract

Translate the actual pre-move position into Counter's semantic delta, then let NGN make and legality-check the move before pushing the verified post-board. A failed evaluator push must not leave the engine board advanced. Null transitions verify the engine's side/facts change while Counter's feature board stays identical. Pop restores an earlier frame; never inverse-add weights.

The compatibility context's 128 frames are not evidence of an NGN recursion bound. Integration must provide safe capacity for actual nested search and singular re-search, with dynamic growth or a proved bound and tested handling. No silent switch to fresh evaluation on overflow. Reset every admitted search root and each worker root.

Keep numerical error-bound accounting in tests, outside production frames. Preserve exact portable upstream update order. Verify the final target GOAMD64=v3 arithmetic against the portable canonical oracle before claiming the existing v1 parity carries over.

## Score adapter

Use the historical Counter order, with explicit signed truncation:

1. Convert finite raw White-perspective output toward zero, with safe pre-conversion handling of values outside the intended range.
2. Clamp to +/-15000.
3. Multiply by (160 + non-pawn material) and divide by 160.
4. Multiply by (200 - rule50) and divide by 200.
5. Negate for Black to move.

Non-pawn material is 4 per knight/bishop, 6 per rook and 12 per queen, counting both sides. Preserve separate divisions. Reuse the accepted actual-upstream adapter vectors, including rounding witnesses.

NGN must additionally keep static evaluation outside the mate band for malformed/extreme positions; use explicit final bounds and report the difference from historical Counter where that safety boundary applies. Normal legal fixtures should remain exact. Do not apply NGN-v1 rule50 attenuation or HCE tempo on top. Existing search correction history remains NGN search policy, separated from the base evaluator oracle.

Emergency evaluation deliberately uses a fresh board and an explicitly named undamped adapter (omit only the rule50 step), matching the existing NGN emergency route. Test this policy separately; it is not an upstream Counter search claim.

## Required evidence, without a new framework

Reuse existing evaluator-selection and worker-transition test seams. Demonstrate legal ordinary/special moves, null, branch pop, singular re-search, emergency fresh evaluation, failed selection, model replacement and worker isolation. Every observed incremental frame must agree with the actual upstream path oracle. Full refresh is a board-state reference with a bounded floating-point difference, not an assumed bit-identical alternate search result.

Run focused correctness and race gates when compute is released, then the required combined engine checks once. Build one exact candidate with both HCE and Counter modes. Update the existing match capability description and trace checks narrowly for the new backend; preflight must confirm model digest, backend, options and actual search. First use one worker and the same binary on both sides.

A short prospective paired-clock screen decides whether to spend on a larger strength test. Freeze its rule before games. A winless screen would stop promotion and trigger comparison of the known network inside Counter versus NGN, including score calibration/search interaction, rather than more unstructured training. Four games cannot estimate Elo or demonstrate a release improvement.
