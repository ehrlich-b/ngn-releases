# Rodent V1.2 default network: incremental context slice

Status: **accepted as inactive compatibility** against accepted Slice A
`f2df112`. The active objective remains raise the rating to 3300. Slice B admits
exact incremental state for later search integration; it does not establish
playing strength.

## Authority and boundary

Retain Slice A's exact external network, strict loader, scalar full refresh,
arithmetic, output buckets, and release adapter. The transition authority is the
pinned Rodent V1.2 source at tag commit
`b53ffaf670590932957cb63b7b6d871f6f33b7d8`, especially `nnue.go` SHA-256
`de1cd6f23fa6a7fe501bdcb3cf9531681d290c57ba7129b9a8764204ebeb85ac`.

Add only one worker-private `SearchContext` and its tests. Frames own a Position
and both 768-lane perspectives. Pushes derive and validate the complete semantic
transition before publishing a child; null pushes copy exact accumulator bits;
pop restores an ancestor without inverse arithmetic; storage begins at 128
frames and doubles explicitly.

All ordinary moves apply modular int16 add/subtract deltas to both perspectives.
An own-king move refreshes only that king's perspective when its oriented
four-bucket index changes or when it crosses the D/E horizontal-mirror boundary.
The other perspective remains incremental. Output-bucket selection always uses
the child board's current total piece count.

Do not add SIMD, Finny caching, engine/UCI selection, a backend adapter, search
policy, bundled weights, deployment, matches, or rating claims in Slice B.

## Predeclared verification

Synthetic deterministic parameters must prove all 768 lanes against independent
full refresh for quiet moves, captures, both en-passant colors, all castles, all
promotion piece types with and without capture, output-bucket threshold changes,
and int16 wrapping. White and black king tests must cover every input-bucket
boundary, both mirror directions, and a same-view move that must remain
incremental. Lifecycle tests must cover nulls, growth, reset, multi-ply unwind,
sibling frame reuse, independent contexts, and transactional rejection.

A build-tagged exact-model transition battery must replay multi-ply paths through
the same public context API. At every checkpoint, all 1,536 accumulator lanes,
raw output, release-static output, board, side, and depth must match a fresh full
refresh using the strict-loaded external model. The exact model remains external.

Accept as inactive compatibility only if focused tests, the required exact-model
battery, package race with the battery enabled, full short repository tests, and
full short race tests all pass on WSL. The Lean hopper must remain live under the
user-directed shared-host policy. Any unexplained lane or score mismatch rejects
the slice rather than changing Slice A arithmetic to fit it.

## Verification and disposition

The accepted implementation adds three files, with these SHA-256 identities:

- `context.go`:
  `9762576f2796da7c873236f6453382e477ae12ffd889758084abb9218d67d6e2`;
- `context_test.go`:
  `6271f3f0db97d95480a92582173dedc15b99d64883eb921857ae44cea62c6f06`;
  and
- `context_exact_test.go`:
  `f22596642492e8d5bf449fe0d8dcd2cd4f822b1427a69b820492ee2e265163eb`.

The isolated WSL worktree
`/home/ehrli/repos/ngn-rodent-v12-sliceb-20260919` was created at exact base
`f2df112dd51b33242d06a8fe12d533d856f407f2`. Verification completed with
`GOMAXPROCS=2` and at most two package workers:

- focused synthetic tests passed in 0.034 seconds after the predeclared bucket
  boundary matrix was completed;
- the required external-model battery passed both multi-ply paths in 0.011
  seconds, including bucket/mirror refreshes, en passant, promotion-capture,
  both castles, null/pop restoration and sibling reuse;
- package race with both Slice A's 104-record exact-release oracle and Slice B's
  exact-model battery enabled passed in 1.360 seconds;
- `go test -short -p 2 ./...` passed, including engine in 6.832 seconds;
- `go test -short -race -p 2 ./...` passed, including engine in 39.465 seconds;
  and
- `go vet ./rodentv12eval` and final `gofmt -d` passed cleanly.

Every external-model checkpoint matched fresh full refresh in all 1,536 int16
lanes, raw output, release-static output, board, side and depth. Synthetic tests
also cover every promotion type, both en-passant colors, all four castles, an
output-bucket threshold capture, every white/black king input-bucket boundary,
both mirror directions, same-view incremental king moves, int16 wrap, growth,
reset, independent contexts, multi-ply unwind and transactional failures.

The shared Lean hopper was never paused or included in a test process group;
PID 3099092 remained live at the terminal check. No model bytes, SIMD, Finny
cache, engine/UCI adapter, search policy, deployment, match, or default change
was added.

**Accept Slice B as inactive compatibility.** The next bounded step is named
opt-in engine/UCI integration with worker/search lifecycle parity, followed by
realistic profiling and a separately frozen strength gate. Slice B is not Elo
evidence, a rating increment, or proof of 3300.
