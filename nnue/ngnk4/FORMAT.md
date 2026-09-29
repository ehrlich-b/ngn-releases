# `ngn-k4-768-v1` model format

All integers are little-endian. A file is exactly 4,744,976 bytes: a 256-byte
header followed by 4,744,720 bytes of signed 16-bit tensors. Readers reject
truncation, trailing bytes, nonzero reserved bytes, unknown identifiers, a zero
training-manifest hash, or a payload checksum mismatch.

| Offset | Bytes | Field | Required value |
| ---: | ---: | --- | --- |
| 0 | 8 | magic | `NGNK4V1\0` |
| 8 | 2 | version | 1 |
| 10 | 2 | header bytes | 256 |
| 12 | 4 | architecture code | 1 (`ngn-k4-768-v1`) |
| 16 | 4 | feature-set code | 1 |
| 20 | 4 | quantization code | 1 |
| 24 | 4 | score-policy code | 1 |
| 28 | 4 | input buckets | 4 |
| 32 | 4 | input size per bucket | 768 |
| 36 | 4 | hidden lanes | 768 |
| 40 | 4 | output buckets | 8 |
| 44 | 4 | QA | 255 |
| 48 | 4 | QB | 64 |
| 52 | 4 | score scale | 400 |
| 56 | 8 | payload bytes | 4,744,720 |
| 64 | 32 | training-manifest SHA-256 | nonzero |
| 96 | 32 | payload SHA-256 | exact |
| 128 | 128 | reserved | all zero |

The tensor payload is row-major and ordered as:

1. `input_weights[3072][768]`
2. `input_biases[768]`
3. `output_weights[8][2][768]`, with side-to-move perspective first
4. `output_biases[8]`

The loader computes the SHA-256 of the entire file. File hash, manifest hash,
payload hash, score policy and dimensions are all part of evaluator identity.

## Features and score

Planes are relative color followed by pawn, knight, bishop, rook, queen, king;
squares use A1=0. Black perspective rank-flips with `square xor 56`. A
perspective whose king is on files e-h mirrors files with `square xor 7`.
The four king-bucket rows on files a-d are `[1,1,0,0]`, `[2,2,2,2]`, then
six rows of `[3,3,3,3]`. The feature index is
`bucket*768 + relative_color*384 + piece_type*64 + oriented_square`.

Accumulators are signed int32 and output sums are signed int64. No arithmetic
wraparound is part of this format. Each activated lane is clipped to `[0,255]`
and squared. For output head `min(7,(men-2)/4)`:

```text
sum = Σ screlu(acc_stm[h]) * w[head][0][h]
    + Σ screlu(acc_ntm[h]) * w[head][1][h]
q = trunc(sum / 255) + output_bias[head]
score = trunc(q * 400 / (255 * 64))
```

The score is side-to-move NGN centipawns before rule-50 attenuation. The engine
applies that attenuation once and reserves the mate band. Training merges the
shared 768-feature factorizer into the four deployed transformer buckets before
round-half-away-from-zero i16 quantization; the factorizer is not serialized.
