# Owned K4 NNUE teacher-label throughput gate

Date: 2026-09-20

Status: **PASS for labeling throughput; bulk run HELD on disk and final sampler audit**

## Scope

This gate exercised the newly source-built Stockfish 18 teacher, two-process
label supervisor, resumable label journal, v2 K4 architecture identity, and
independent JSONL-to-Bullet packer on 55,000 deterministic positions, continuing
until more than 50,000 labels were accepted. The
positions came from the prior audited prefix pilot solely to measure teacher
throughput. They are not the new full-archive 97/1/1/1 selection and are not
eligible as pilot training data.

Remote root:

`/home/ehrli/nnue-owned-k4-20260920/runs/label-throughput-v2`

Teacher build receipt:

`/home/ehrli/nnue-owned-k4-20260920/teacher/sf18-build-receipt.json`

Receipt SHA-256: `79438117517ce044dd69478133fe53a30698ef86ef2965befa67a8e28a76619b`.

Teacher binary SHA-256:
`174270346ae9ed600713d165fa745dfa87fc084e44907c8084767f2b905e86d3`.
The clean official `sf_18` checkout resolved to frozen commit
`cb3d4ee9b47d0c5aae855b12379378ea1439675c`; BIG and SMALL networks matched
the frozen full hashes.

## Discovered and corrected protocol defect

The first evidence-only v1 pass completed in 132.23 seconds but accepted only
17,971/50,000. It rejected 31,397 records as `node-budget-short`. That test was
invalid: after `go nodes 5000`, Stockfish can reach the commanded root budget
between completed-depth `info` reports, so the last exact CP line can legally
show fewer than 5,000 cumulative nodes before `bestmove`.

The contract was bumped to v2. It still sends exactly `go nodes 5000`, waits for
`bestmove`, requires depth at least four, and retains the exact reported node
count, but does not reinterpret a completed-depth line as the final root-node
counter. A fixture with a 4,999-node last line now covers the observed protocol
shape.

The same change also closes a split-isolation gap: sampler, labeler and packer
now use `ngn-k4-input-v1`, hashing both sorted king-bucketed/mirrored
perspective feature lists plus the material output head. The labeler and packer
independently recompute this identity; forged old Chess768 keys are rejected.

## v2 results

Two one-thread Stockfish processes ran concurrently at nice 10 on CPUs 8 and 9.
The Lean hopper remained live throughout.

| Shard | Inputs | Accepted | Rejected | Wall seconds | Max RSS KiB |
| --- | ---: | ---: | ---: | ---: | ---: |
| 000 | 25,000 | 22,872 | 2,128 | 131.11 | 384,384 |
| 001 | 25,000 | 22,851 | 2,149 | 132.50 | 384,384 |
| 002 continuation | 5,000 | 4,590 | 410 | 26.59 | 384,384 |
| Total | 55,000 | 50,313 | 4,687 | 159.09 staged wall | 768,768 maximum concurrent summed peaks |

Acceptance was 91.478%. Rejections were explicit: 25 depth-below-minimum, 24
mate-score, 1,870 nonquiet-bestmove, and 2,768 PV/bestmove mismatches. All three
label processes and all three independent pack operations exited zero. No NGN
labeler or Stockfish process survived either supervisor. The first two shards
ran concurrently; the small continuation deliberately used one process only.

Measured accepted rate:

`45,723 / 132.50 = 345.079 accepted positions/second`.

Frozen forecast with 30% slack:

- 1.3M pilot plus holdouts: `1,300,000 / 345.079 * 1.30 = 4,897 s = 81.6 min`, below the four-hour cap.
- 19M incremental main training positions after the pilot: `19,000,000 / 345.079 * 1.30 = 71,578 s = 19.88 h`, below the 24-hour cap.
- Even 20.3M from zero forecasts to about 21.24 hours with slack.

This is a capacity forecast, not authorization to launch. It assumes the final
full-archive sampler produces a similar eligible distribution. The sampler now
freezes 1.25M pilot candidates, 25M total train candidates, and 125k candidates
per holdout, enough to tolerate 20% rejection. Labeling stops only after the
exact accepted target is available; the forecast above is therefore correctly
based on accepted throughput rather than pre-label candidate count. Even if the
entire 23.75M main-expansion candidate pool were consumed, the observed input
rate with 30% slack forecasts about 22.7 hours, still inside its 24-hour gate.

## Bound artifacts

Input shard hashes:

- 000: `3161b4c2efdc0674d32ff68067ab0f3bfcb8e844f896a63baf1ae7bf5c51870b`
- 001: `7fff0803c1de81f6dc9b645a3937846ac1c91dffe3d9b4982191f46f14050b44`
- 002: `42b2762fde6a5d74d5400359089cb2703e45ea4b7469bb20ae60a24c1b9786fb`

Label output hashes:

- 000: `0a27d8f4e483dbd9c654358ecf4edf5284f5332e877fbd695b2b6ba4520ae52e`
- 001: `6d2375ba81c87e63f105faae3385c9e1b0d55e340643d5fb0240cda3c4be3325`
- 002: `bd2113d7e82bdb121d848bf39d5b7f5911a4605bb0c5ac4811c856de0b1fd91e`

Packed BF hashes and accepted counts:

- 000: `88b5cb4c1a336c9f54fa8b50d625d16bb4f7c76b606b2e3ec165b721d02ca469`, 22,872 records.
- 001: `296df08e70569a44ae27d948606f27744d26475eb0763df3bf1790578cda23fc`, 22,851 records.
- 002: `c5e6c2958554e2105f96cd222acec47011b44e67fb3893339f453f98c625657b`, 4,590 records.

The packer receipts bind label/input hashes, exact teacher source/net/binary
identities, result constant, score perspective, K4-key contract, record order,
and accepted-stream digest.

## Remaining hold

Windows C: had 5,154,881,536 bytes free after the two-shard gate. The plan requires at
least 60 GiB before pilot launch and stops below 25 GiB. Therefore no 1M pilot,
20M main run, or large sampler spill is authorized yet. Source-side sampler and
split-verifier implementation can continue without weakening that gate.
