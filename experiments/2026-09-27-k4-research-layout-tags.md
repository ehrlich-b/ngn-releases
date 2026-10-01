# Research build tags for K4 layout experiments (K8 buckets, 1024 width)

Status 2026-09-27: infrastructure only; default build proved behavior- and
speed-identical. No strength claim.

- `nnue/ngnk4`: `InputBuckets`, `kingBucketTable` and a bucket label move to
  `layout_k4.go` (`!ngnk8`, frozen default) / `layout_k8.go` (`ngnk8`);
  `HiddenSize` moves to `width_768.go` (`!ngnw1024`) / `width_1024.go`
  (`ngnw1024`). `PayloadSize`, `ArchitectureID` (`ngn-<k4|k8>-<768|1024>-v1`)
  and `FeatureSetID` derive from them; default IDs are unchanged. The three
  AVX2 kernels read their 16-lane loop count from `go_asm.h`
  (`const_k4LaneBlocks`) instead of a literal 48.
- K8 layout (trainer `K8_BUCKET_LAYOUT`, files e–h mirrored): rank 1
  a/b/c/d = 0/1/2/3, rank 2 a–b = 4 and c–d = 5, ranks 3–4 = 6, ranks 5–8 = 7.
- `cmd/ngnk4bridge`: its independent reference bucket table follows the same
  tag; raw checkpoint size derives from `ngnk4.PayloadSize`.
- Trainer (`ngn_k4_train.rs`): `NGN_K8` / `NGN_H1024` set at compile time
  select the layout/width; unset reproduces the frozen harness values
  (`DEPLOYED_VALUES` is now computed and equals 2,372,360 by default).
- Two frozen-contract tests skip under non-default tags.

Evidence (WSL, GOAMD64=v3): `nnue/ngnk4` and `cmd/ngnk4bridge` tests pass under
all four tag combinations, and `nnue/ngnk4` also under GOAMD64=v1 with both
tags. Default-build identity against the S1 binary
([receipt](2026-09-27-k4-layout-tags-identity.json)): same 88 positions,
depth 12, two alternating rounds, all identical; median time ratio 0.994
(noise).
