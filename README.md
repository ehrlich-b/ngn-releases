# NGN

NGN is a UCI chess engine written in Go by Bryan Ehrlich. Version 0.4.0 plays
with an NGN-trained network embedded in the executable; no sidecar is needed.

## Strength

**Official rating: CCRL 40/15.** NGN 0.3.0 (1 CPU) is rated **3411 ±37** from
133 games on the list computed November 7, 2026
([engine page](https://computerchess.org.uk/ccrl/4040/cgi/engine_details.cgi?print=Details&each_game=0&eng=NGN%200.3.0%2064-bit)).
NGN is not yet on the CCRL Blitz list. 0.4.0 has been submitted for testing.

**0.4.0 against earlier NGN (our own testing, 10+0.1, one thread):** +222
[+196, +248] Elo against 0.3.0 over 1,000 games, and +104 [+68, +140] against
the October 19 development build over 385 games. Self-play gains usually shrink
on CCRL; until 0.4.0 is rated, our guess for it on 40/15 is roughly 3450-3550.

**Earlier Blitz estimate, October 19 development build: about 3580 CCRL Blitz
(8 threads).** This is our own measurement, not an official rating. A 320-game
gauntlet at 10+0.1 with 8 threads against Koivisto 6.0, akimbo 1.0.0, Black
Marlin 9.0 and Berserk 12, using their CCRL Blitz 8CPU ratings as fixed anchors,
measured **3582 [3542, 3623]**. A longer 120+1 run against three of those
anchors stopped early at 368 games, at 3629 [3593, 3665].

For scale: CCRL 40/15 ratings run lower than Blitz ratings (the top of each
list, Stockfish 19, is 3650 on 40/15 and 3803 on Blitz). The same kind of local
calibration put 0.3.0 at 3462 against CCRL Blitz 1CPU anchors, and CCRL 40/15
rated it 3411.

## 0.4.0 contents

- **Engine:** NGN search, board, move generation, hashing, time management and
  NNUE inference, with the Go runtime and standard library. No external Go modules.
- **Network:** NGNN3, 8 mirrored king-context buckets × 768 piece-square inputs,
  768 hidden units per perspective (768×2 SCReLU), 8 material output buckets.
  SHA-256 `2dcb6d936c0275927cb9e881f42e22558e618ad7e6fd998df6c4e5046d42c443`
  (9,463,428 bytes). NGN's PyTorch trainer trains NGN-owned weights;
  no other engine's trained network is included.
- **Training data:** public Leela Chess Zero training positions, search scores and
  game results, distributed by
  [official-stockfish/master-binpacks](https://huggingface.co/datasets/official-stockfish/master-binpacks)
  including the ODbL T80 archive. The H768 manifest also names T76 and T60/T70
  corpora. Verified fresh H512 initialization, H768 widening and corpus content
  pins are documented in [data provenance and method](docs/TRAINING-METHOD.md)
  and [data notice](LICENSES/TRAINING-DATA.txt).
- **Evaluation:** the embedded NGNN3 network is the default. `UseNNUE=false`
  selects NGN's classical evaluation. `EvalFile` can select another NGN network.
- **Source:** engine and trainer, small owned test fixtures, license texts,
  and the PGO build profile. Research operations and opponent engines are excluded.

The engine uses published chess-programming techniques. [NOTICE](NOTICE) and
[docs/THIRD_PARTY.md](docs/THIRD_PARTY.md) preserve dated origins and named credits
for earlier releases. [Release notes](RELEASE-NOTES-v0.4.0.md) describe changes.

## UCI options

The following is the complete option list printed by the 0.4.0 engine at startup.
Tuning values are integer parameters; change them only for controlled experiments.

```text
option name Hash type spin default 128 min 1 max 1024
option name Threads type spin default 1 min 1 max 64
option name Move Overhead type spin default 100 min 0 max 5000
option name W2ScoreTime type spin default 0 min 0 max 1
option name OwnBook type check default false
option name EvalFile type string default <embedded>
option name UseNNUE type check default true
option name T1104ContPlies type spin default 0 min 0 max 1
option name W2QCapture type spin default 0 min 0 max 1
option name T1104Cuckoo type spin default 0 min 0 max 1
option name T1104CorrMore type spin default 0 min 0 max 1
option name W2NullVerify type spin default 0 min 0 max 1
option name W2NullMargin type spin default 0 min 0 max 1
option name W2CorrMargin type spin default 0 min 0 max 1
option name W2QChecks type spin default 0 min 0 max 1
option name T1104LowPly type spin default 0 min 0 max 1
option name T1104PriorBonus type spin default 0 min 0 max 1
option name NullMoveR type spin default 4 min 2 max 5
option name NullMoveMinDepth type spin default 2 min 1 max 6
option name LMRFullDepth type spin default 3 min 1 max 6
option name T1104LMRDeeper type spin default 0 min 0 max 1
option name LMRMoveThreshold type spin default 2 min 1 max 8
option name T1104LMRCaptures type spin default 0 min 0 max 1
option name T1104LMRFractional type spin default 0 min 0 max 1
option name FutilityMargin type spin default 101 min 50 max 160
option name FutilityMaxDepth type spin default 8 min 1 max 8
option name ReverseFutilityMargin type spin default 69 min 40 max 300
option name DeltaMargin type spin default 261 min 0 max 400
option name SingularDepth type spin default 8 min 4 max 12
option name SingularMargin type spin default 67 min 16 max 256
option name T1104SingularDouble type spin default 0 min 0 max 1
option name ExtensionBudget type spin default 29 min 4 max 64
option name AspInit type spin default 18 min 4 max 40
option name AspMult type spin default 171 min 110 max 250
option name LMPBase type spin default 3 min 2 max 16
option name LMRBase type spin default 0 min 0 max 300
option name LMRDivisor type spin default 200 min 100 max 600
option name HistoryLMRDivisor type spin default 2500 min 500 max 6000
option name HistoryLMRCap type spin default 2 min 1 max 5
option name RFPMaxDepth type spin default 7 min 1 max 12
option name RFPImproving type spin default 1 min 0 max 3
option name FutilityImproving type spin default 1 min 0 max 3
option name LMPQuadratic type spin default 1 min 1 max 4
option name LMPNonImprovingDivisor type spin default 2 min 1 max 5
option name LMPMaxDepth type spin default 9 min 2 max 14
option name SEECaptureDepth type spin default 4 min 1 max 8
option name SEECaptureThreshold type spin default 100 min 0 max 400
option name SEEQuietDepth type spin default 4 min 1 max 8
option name SEEQuietSlope type spin default 80 min 0 max 300
option name HistoryPruneDepth type spin default 3 min 1 max 8
option name HistoryPruneThreshold type spin default 1000 min 100 max 4000
option name HistoryPruneMoves type spin default 3 min 0 max 10
option name NullMoveDepthDivisor type spin default 600 min 100 max 1200
option name NullMoveOffset type spin default 1 min 0 max 4
option name IIRDepth type spin default 4 min 2 max 10
option name IIRPhase type spin default 8 min 0 max 24
option name IIDDepth type spin default 4 min 2 max 10
option name IIDReduction type spin default 2 min 1 max 4
option name SingularMarginDivisor type spin default 32 min 8 max 96
option name SingularTTDepthOffset type spin default 3 min 0 max 8
option name HistoryRateCap type spin default 400 min 64 max 512
option name HistoryRateScale type spin default 1 min 1 max 4
option name HistoryBonusCap type spin default 2048 min 512 max 8192
option name PawnCorrectionWeight type spin default 3 min 0 max 10
option name NonPawnCorrectionWeight type spin default 3 min 0 max 10
option name MinorCorrectionWeight type spin default 3 min 0 max 10
option name SoftIncrementPercent type spin default 80 min 0 max 150
option name HardSoftMultiplier type spin default 4 min 1 max 8
option name HardBankPercent type spin default 30 min 1 max 100
```

Pondering is not supported.

## Build

Go 1.21.2 or newer is required. The commands below target x86-64-v3 (AVX2).

```sh
GOAMD64=v3 go build -trimpath -ldflags "-X main.releaseVersion=0.4.0" -o ngn .
GOOS=windows GOARCH=amd64 GOAMD64=v3 go build -trimpath -ldflags "-X main.releaseVersion=0.4.0" -o ngn.exe .
```

For byte-identical builds from checkouts with different publication histories,
set `GOFLAGS=-buildvcs=false`; source archives contain no VCS metadata.
Use the same Go version, CGO setting and bundled `default.pgo` for comparisons.
The 0.4.0 release binaries were built with Go 1.27.3 and `CGO_ENABLED=0`; with
those settings and `-buildvcs=false`, this source rebuilds them byte for byte.

GPL-3.0-only; see [LICENSE](LICENSE) and [LICENSE-GRANT.txt](LICENSE-GRANT.txt).
