# Rodent V1.2 default network: strict full-refresh slice

Status: **accepted as inactive compatibility** against base `5f13aae`. The
active objective remains raise the rating to 3300. This slice establishes exact
full-refresh compatibility with the existing network; it does not establish
playing strength by itself.

## Admission and provenance

Use only the official Rodent V1.2 non-Tal default model already pinned on WSL:

- release tag/commit: `Rodent_v_1.2` /
  `b53ffaf670590932957cb63b7b6d871f6f33b7d8`;
- current release asset: `release-1-2.zip`, SHA-256
  `81c46c45d3f26393b375f6be03d60f4207bb79f4f924156e53fee6ac75a7be6d`;
- exact testers binary: 7,311,522 bytes, SHA-256
  `9cfb8195207ee5695c1973a89664ab73b34b5bcbc10ad3bc0f0afe28b9713cbc`;
- exact external model: `rodent_4kb_768hl_8ob_v2.bin`, 4,744,768
  bytes, SHA-256
  `c35a1abc1b8c1cb1d5f4221454d494c1a6da1ed9088fd51ab27038bfa74b5053`;
  the source and release copies are byte-identical; and
- pinned source `nnue.go` SHA-256
  `de1cd6f23fa6a7fe501bdcb3cf9531681d290c57ba7129b9a8764204ebeb85ac`
  plus `nnue_scalar.go` SHA-256
  `bb9f440d15953eba2dcb51b662af687b301e3c2b83f0f43bd9d1e8ca4f3fda50`.

The release configuration is pure NNUE, horizontally mirrored, four king-input
buckets, 3,072 input rows, 768 hidden lanes, eight material-count output
buckets, scale 206, SCReLU 255/64, and the symmetric release material adapter.
The model remains an external file. Do not embed or redistribute it.

## Slice boundary

Add a standalone `rodentv12eval` package only:

- strict exact-artifact loader with bounded reads, exact size/SHA/trailer and
  transactional failure;
- immutable parameter storage;
- portable two-perspective full refresh with exact king orientation, king-bucket
  table and horizontal mirroring;
- exact output-bucket selection, four-part int32 modular SCReLU accumulation,
  truncating scale arithmetic, and release material scaling; and
- synthetic malformed-model, board, mapping, bucket, wrap and truncation tests.

Do not add an incremental context, AVX2, Finny cache, engine/UCI selection,
search policy, default change, match, deployment or rating claim in Slice A.

## Result-unseen oracle contract

Before evaluating NGN output, run the exact testers executable twice over a
frozen 104-record corpus. It contains 40 score-unselected layouts, both sides
to move, plus clocks 0/50/99 on six established layouts. It retains the V1.1
compatibility roots and special positions, and adds exact piece-count witnesses
for all eight output buckets plus explicit white/black king witnesses for all
four oriented input buckets. Require unique names/FENs, deterministic passes,
the exact UCI identity/options/network record, empty stderr, clean exit and no
survivors. The command is `position fen <FEN>; nnue; isready`; its integer is
the full-refresh raw score before the material adapter.

The initial driver SHA-256 before execution was
`62a2dc0c5f3fd482a1d0bd8aae4a50b639e72e542f69172210b79673716b7823`.
No scale, mapping, fixture, command, or expected score was changed after the
release was queried.

## Oracle execution

All attempts are preserved under
`/home/ehrli/rodent-v1.2-default-oracle-20260919`:

- `run-001` failed before launching Rodent because the driver's side-to-move
  assertion expected 36 rather than the actual 52 records per side;
- `run-002` fixed only that assertion, completed both release query passes, and
  then failed while requesting the stale V1.1-only `main.init.3` symbol;
- `run-003` removed only that symbol request, completed both query passes, and
  proved that the exact V1.2 release has no symbol section at all; and
- `run-004` replaced impossible disassembly requests with an explicit preserved
  `go tool nm` “no symbol section” witness. Its final driver SHA-256 is
  `d8db860fc98c3b7133727f7487fb050acff3383cb708fecf4f13cdd343a26284`.

The raw values from failed runs were not inspected before those driver-only
corrections. Run 004 completed 104 records twice with identical results. The
exact binary returned cleanly in 0.016 seconds, emitted empty stderr, launched
no search or benchmark, and left no survivor. Its independently derived static
scores use the pinned tagged-source formula. Principal evidence:

- oracle JSON SHA-256:
  `a16487b7a7faf725d4c2369c91467274c10ecd04b8a4a0b8faec5c48fc08d440`;
- output inventory SHA-256:
  `c348f3d6120ab815371501ee81a52b87ab72e1f9762a72716a942de2eabe2a9b`;
- driver receipt SHA-256:
  `e102659d04703b889c6d53a53e844dea4b55d55cdbf54ce8482865349ed330b5`;
  and
- supervisor receipt SHA-256:
  `3441b0e59f24cda9609134fc4089dc7e9880e2d69d429cb85d734fc4f3a0c7a2`.

The outer supervisor completed in 6.37 seconds on CPU 0, sampled a 251,944 KiB
peak process tree, stayed below its 1 GiB/60-second bounds, and reported no
timeout, monitor error, orphan, signal, or survivor. The shared Lean hopper was
not paused or placed in that process group; PID 3099092 remained live throughout.

## Acceptance

Accept as inactive compatibility only if:

1. the exact-release oracle completes as frozen and independently derived
   release-static values use the pinned symmetric material formula;
2. every NGN raw and release-static value matches all 104 records exactly;
3. strict-loader and synthetic arithmetic/mapping/bucket tests pass;
4. package race, full short repository and full short race suites pass on WSL;
5. the source diff remains confined to the package, tests and this evidence;
   and
6. the Lean hopper remains live under the user-directed shared-host policy.

## Verification and disposition

The new `rodentv12eval` package is standalone and the external model remains
untracked. On the isolated WSL worktree at exact base `5f13aae`:

- focused short synthetic tests passed;
- the build-tagged exact loader matched all 104 oracle raw values and all 104
  independently scaled release-static values;
- package race with the oracle enabled passed in 1.165 seconds;
- `go test -short -p 2 ./...` passed, including engine in 8.610 seconds; and
- `go test -short -race -p 2 ./...` passed, including engine in 41.523 seconds.

Every locally retained Go file is byte-identical to its WSL-tested counterpart.
The accepted source adds no incremental state, SIMD, engine/UCI selection,
search policy, embedded weights, deployment, match, or default change.

**Accept Slice A as inactive compatibility.** It admits a separately bounded
incremental implementation as a later slice. It is not Elo evidence, a rating
increment, or proof of 3300.
