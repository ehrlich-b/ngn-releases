# NGN 0.2.0 HCE release

0.2.0 uses the independent HCE-only runtime. Owned/research neural profiles and
external evaluator selection are retired. No network or opening book is bundled.
Prior NNUE estimates and historical HCE matches do not rate this version.

All engine builds, tests and execution run on the authorized personal WSL host,
under the documented shared CPU0/2 50% NGN quota, nice10 and serial Go limits.
The Mac is a text-review/publication terminal. Preserve the existing benchmark
owner, protected CPUs and shared services.

1. Verify a clean immutable source head and the recorded short/race/vet checks.
   Source/document-only changes may reuse the identical tested engine tree;
   record this binding instead of claiming tests ran on a different tree.
2. Retain LICENSE, NOTICE, complete LICENSES and docs/THIRD_PARTY.md in binary
   packages and the exact matching source archive. Check every referenced
   component notice is present and bind all archive hashes to the source head.
3. Build Linux and Windows amd64-v1 with Go 1.25.5, CGO_ENABLED=0, GOAMD64=v1
   and -ldflags '-X main.releaseProfile=hce -X main.releaseVersion=0.2.0'.
   Version/UCI/model-backend rejection and legal-move smoke must pass.
   Record Windows as cross-compiled unless native execution is verified.
4. Preserve the old public source history. Add corrected author attribution
   first, then replace the current tree; never rewrite/delete old releases.
5. Publish only with the user's release authority and exact approved forum text.
   Do not attach an Elo estimate or promise competition acceptance.

The earlier owned-network procedure is preserved in
[the historical rc.1 document](historical/RELEASING-owned-0.2.0-rc.1.md).
Current correctness/generation records are in
[the independent HCE experiment](../experiments/2026-10-01-original-hce-release/report.md).
