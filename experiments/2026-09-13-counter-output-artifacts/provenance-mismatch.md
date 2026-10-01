# Counter output candidate provenance correction

The first frozen `candidate.patch` had SHA-256
`a8dd686809bcbf2d4a8ee59568c26334fa09a8a646f108b629f0c265f76ce9d4`.
Its newly copied Counter source and test files accidentally had mode `100755`.
The first `receipt.json` bound that hash and had SHA-256
`9d9247a5b15f21df6f4d743548ce6dcb156c06b17c055a978fc40be19856e337`.

Final review normalized only these eight file modes from `100755` to `100644`:

- `countereval/output_dot.go`
- `countereval/output_dot_fallback.go`
- `countereval/output_dot_amd64_v3.go`
- `countereval/output_dot_amd64_v3.s`
- `countereval/output_dot_test.go`
- `countereval/output_dot_fallback_test.go`
- `countereval/output_dot_amd64_v3_test.go`
- `countereval/output_dot_benchmark_test.go`

No source content changed in that correction. Exact rebuilds produced the same
binary hashes. Regenerating `candidate.patch` changed its SHA-256 to
`8966b491a3c8e57588b94009e44fe8e27983ccace2a867e9f3558dc5ac732786`.
There was a brief mismatch while the unversioned patch path held these new bytes
and the old receipt still named the old hash. The corrected versioned receipt
binds the new patch hash.

The old patch bytes were overwritten and are not claimed as preserved. The old
receipt is reconstructed as `receipt-v1.json` because its only byte difference
from the corrected receipt was the embedded patch hash; its expected SHA-256 is
`9d9247a5b15f21df6f4d743548ce6dcb156c06b17c055a978fc40be19856e337`.
