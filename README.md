# NGN

NGN is a UCI chess engine written in Go. This repository provides frozen release source and downloadable Windows and Linux packages.

Download an owned NNUE package from [Releases](https://github.com/ehrlich-b/ngn-releases/releases), extract it, and register `ngn.exe` or `ngn` with your chess GUI. Keep `ngn.nnue` beside the executable.

The owned profile selects the packaged network automatically. Defaults are Threads 1, Hash 128 MiB, K4EvalScale 60, OwnBook false and Move Overhead 100 ms. The amd64-v3 packages require a compatible x86-64 CPU.

NGN has no official CCRL rating. Playing comparisons and their limits are recorded in each release. See `docs/RELEASING.md` for the build and verification procedure, and `docs/THIRD_PARTY.md` for provenance.

NGN's original source and the packaged owned network are licensed under GPL-3.0-only. Preserve the existing third-party notices. Training data and altered-database inputs retain their ODbL-1.0 terms. The complete alteration method accompanies each network release.
