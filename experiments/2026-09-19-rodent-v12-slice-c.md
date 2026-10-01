# Rodent V1.2 default network: opt-in engine integration

Status: **accepted as opt-in integration** against accepted Slice B `f17114e`.
The active objective remains raise the rating to 3300. Slice C makes the exact
existing network selectable for realistic profiling; it does not establish
strength.

## Frozen scope

Add one named backend, `rodent-v1.2-default`, through the existing evaluator
selection architecture. Reuse Slice B's immutable model and worker-private
context. Add exact engine-position/transition conversion, transactional receiver
selection, per-worker reset/push/null/pop ownership, full-refresh test routing,
UCI/startup staging and diagnostics, and command-line help. Preserve HCE as the
default and preserve every existing backend's name, model, score policy and
identity.

The V1.2 backend uses its exact release-static score and the existing Rodent
non-mate clamp. Like the exact V1.2 release and accepted Slice A oracle, it does
not apply an NGN rule-50 damping layer. Failed load, selection or replacement
must leave the active evaluator, TT, history, worker and staged model unchanged.

Do not add SIMD, Finny caching, a new search policy, bundled weights, automatic
selection, deployment, profiling conclusions, matches or rating claims in this
slice.

## Acceptance

With the exact external V1.2 model on WSL, require:

1. engine mapping and packed-move transition tests for quiet, capture,
   en-passant, castle and promotion families;
2. incremental versus full-refresh search identity and exact context unwind;
3. stopped-search, explicit move/null/pop and clock-identity lifecycle parity;
4. private reset/unwind state for every configured SMP worker;
5. transactional model selection, identity-driven TT/history invalidation and
   same-model warm-state retention;
6. exact headerless UCI staging, startup defaults, diagnostic score and failed
   replacement transactionality while all old UCI options still pass;
7. configured backend package race, full short repository tests, full short
   race tests and vet; and
8. the Lean hopper remains live under the user-directed shared-host policy.

Any unexplained score, search, lifecycle, option or state mismatch rejects the
slice. Acceptance only permits portable performance profiling and a separately
frozen game gate; it is not Elo evidence or proof of 3300.

## Verification and disposition

The isolated WSL worktree
`/home/ehrli/repos/ngn-rodent-v12-slicec-20260919` was created at exact base
`f17114e2f2770510a47b232f62bac8a72c95aa59`. Key final file identities are:

- `engine/worker_evaluator.go`:
  `a9453da68ba4a2e449ae6b21c0028bb7a4c9f1d45cddb3a7bdaf00c368d1af80`;
- `engine/evaluator_selection.go`:
  `3191f13b1de401304b9446b44cbb339226d798904dc8d9dc70993491b631c6a3`;
- `engine/uci_evaluator.go`:
  `8408356482e9d91eee046f17b240f4451117bf2837919fe04eeee9904eaf98cb`;
- `engine/rodent_v12_transition.go`:
  `cd7c816991cffb4a495b3bcca4e257265510e78767b35f68a9e4e4f7c065f4d3`;
  and
- `engine/rodent_v12_evaluator_oracle_test.go`:
  `fea45290dfc5f541d577db9b9c2b12adf3d928f8e9df0d61225e236071fe759f`.

Verification used the exact external model and `GOMAXPROCS=2`:

- ordinary short engine/root tests passed in 6.711/0.009 seconds;
- eight configured V1.2 parent tests passed in 0.239 seconds, covering mapping,
  every packed-move family, incremental/full-refresh search identity, stopped
  search, explicit move/null/pop, halfmove-clock identity, three private SMP
  contexts, transactional selection/identity invalidation, exact headerless UCI
  staging/startup, live diagnostic output and corrupt replacement rejection;
- the same configured backend gate passed under race in 4.298 seconds;
- `go test -short -p 2 ./...` passed;
- `go test -short -race -p 2 ./...` passed, including engine in 39.000 seconds;
- `go vet ./...`, `go build ./...`, and final `gofmt -d` passed cleanly.

HCE remains the zero-value, command-line and UCI default. Existing NGN-v1,
Counter 5.5 and Rodent V1.1 names and score paths remain available. V1.2 is
selected only by `rodent-v1.2-default` with its exact external model; selection
changes invalidate model-owned TT/history state while reselecting the identical
model retains warm state. Every configured worker owns a distinct context and
all tested searches restore depth zero.

The shared Lean hopper was never paused or included in a test process group;
PID 3099092 remained live at the terminal check. No model bytes, SIMD, Finny
cache, search-policy change, automatic default, deployment or match was added.

**Accept Slice C as opt-in integration.** Next run a bounded portable profile
against the current optimized Rodent V1.1 reference before choosing an
optimization or freezing any game gate. Slice C is not Elo evidence, a rating
increment, or proof of 3300.
