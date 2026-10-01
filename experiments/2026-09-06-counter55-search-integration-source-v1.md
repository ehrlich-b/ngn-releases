# Counter 5.5 search-integration source draft v1

Status: source-only, unformatted, unbuilt, and untested. This isolated worktree is based on NGN `f7507bd025ea788492fd07847167dfb8de2e462b`. It overlays the separately source-reviewed but execution-pending Counter incremental v4 prerequisite. Nothing here is integrated or accepted.

The draft adds an exact `counter-5.5` evaluator branch through the existing concrete evaluator union. The strict loader owns immutable SHA/bound metadata and caches the maximum absolute update weight per lane. Selection rejects nil and zero-value models and binds backend, file identity, adapter revision, and HCE/PST generation.

`SearchContext` is separate from Counter's fixed 128-frame compatibility context. It starts with 128 frames, validates a prospective transition before growth, checks doubling overflow, validates a conservative root-plus-path numerical bound from cached maxima, prepares both grown slices, then publishes them. Reset performs a full root refresh plus O(512) capacity validation; it does not rescan all feature weights. Rejected transitions, roots, and growth preserve depth, state, and capacity.

The engine adapter derives Counter deltas from NGN's established semantic move bridge and obtains pre/post boards directly from maintained piece bitboards. It covers captures, en passant, promotions, castling, nulls, pop, stopped searches, and singular re-search without adding move legality to Counter. All search call sites now use a mechanical push wrapper that un-makes an already advanced engine move/null before propagating a fail-fast evaluator error.

The score adapter follows Counter 5.5's exact order: finite float32 raw value; signed truncation and +/-15000 clamp; non-pawn-material multiply/divide; optional rule-50 multiply/divide; STM sign. The explicit emergency route uses a fresh board and omits only rule-50 damping, then both routes apply NGN's non-mate static clamp.

UCI recognizes only the exact NGN-v1 or Counter 5.5 header before invoking the matching strict loader. `EvalFile` may stage either model under HCE. Active cross-format replacement is rejected until HCE is selected, while same-format replacement is transactional. `EvalBackend` advertises `hce`, `ngn-v1`, and `counter-5.5`.

Planned gates after the active match and after the standalone v4 prerequisite passes:

1. Format and verify the source-only delta is formatting-equivalent.
2. Run the accepted standalone upstream/full-refresh/incremental v4 gates first.
3. Run Counter unit tests for loader identity, dynamic growth, unsafe-context rejection, transaction preservation, and compatibility-context regression.
4. Run the build-tagged accepted-model upstream transition and score-oracle tests through the actual engine adapter.
5. Run focused worker, selection, UCI, stopped-search, singular-research, race, vet, and build gates with bounded supervision.
6. Only then prepare a separately reviewed exact binary/model preflight and prospective game screen. No game or timing evidence is part of this source draft.
