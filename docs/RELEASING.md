# NGN 0.3.0 release

0.3.0 embeds the NGN-trained network `engine/nets/ngn-h512-c6c12796.nnue` and
starts with UseNNUE=true. The HCE remains selectable with UseNNUE=false.
The strength record is the 2026-10-07 claim run, which used the same engine
source and network loaded through EvalFile.

All engine builds, tests and execution run on the authorized WSL host. The Mac
is a text-review/publication terminal.

1. Verify a clean immutable source head and the short/race/vet checks.
2. Prove search identity: at fixed depth/nodes, the default embedded startup
   must match `EvalFile=<same net>` + `UseNNUE=true` on the claim binary
   (bestmove and node counts), and the embedded bytes must hash to
   `DefaultNetSHA256`.
3. Build Linux and Windows amd64 with Go 1.25.5, CGO_ENABLED=0, GOAMD64=v3,
   `-trimpath -ldflags '-X main.releaseVersion=0.3.0'`. Version/UCI/legal-move
   smoke must pass; run the Windows executable natively before publishing.
4. Package LICENSE, LICENSE-GRANT.txt, NOTICE, LICENSES/, docs/THIRD_PARTY.md
   and README.md with each binary, plus the exact source archive and
   `ngn-0.3.0-training-method.tar.gz` (ODbL alteration method and inputs).
5. Preserve old public releases and history. Publish only with the user's
   release authority and exact approved forum text.

The 0.2.0 HCE procedure is preserved in git history; the rc.1 procedure is in
[the historical rc.1 document](historical/RELEASING-owned-0.2.0-rc.1.md).
