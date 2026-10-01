# NGN

NGN 0.2.0 is an experimental Go chess engine. This release uses a new original
classical HCE with hand-chosen, analytic material/square values. No neural model,
foreign evaluator, borrowed HCE table or training corpus is part of the runtime.
Elo work is deferred; this version has no measured playing strength. The older
neural-release estimates and historical HCE matches do not rate this candidate.

The seven identified Zahak-derived core components were independently generated
from behavior contracts and verified before this stage. The old HCE/PeSTO arrays,
explicit Counter draw scaling, inherited magic constants and generic/Counter/
Stockfish NNUE packages/adapters are removed additively. Fresh coordinate-based
sliding attacks passed independent replay against every relevant blocker subset.
The engine still uses published chess techniques and the Go standard library.
Canonical Polyglot format data is retained from the precisely pinned Disservin
MIT source, with full attribution. This is not a claim that every algorithm or
format constant was invented by NGN or a blanket legal certification.

GPL-3.0-only. [NOTICE](NOTICE), [third-party history](docs/THIRD_PARTY.md) and
[recovery evidence](experiments/2026-10-01-original-hce-release/report.md) retain
historical attribution and the generation/test process. Existing public releases,
source history and experiments remain preserved. Old tuning/model features are
retired. The only playable build profile is HCE; retired owned/research profiles
fail closed. No evaluator model is loaded automatically or through UCI switching.

A separate original neural-evaluator experiment is tested but untrained and
inactive. The release includes matching source, full notices and verification evidence.

Build on the authorized WSL host: `go build -trimpath -ldflags "-X main.releaseProfile=hce -X main.releaseVersion=0.2.0" -o build/ngn .`.
Default startup is locked HCE and OwnBook=false. Matching source/binary manifests
record the exact commit, Go version, platform and verification limits.
