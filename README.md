# NGN

NGN is a UCI chess engine written in Go by Bryan Ehrlich. Version 0.3.0 plays
with an NNUE network trained by NGN's own trainer; the network is embedded in
the executable, so no extra files are needed.

## Strength

**Official rating: CCRL 40/15.** NGN 0.3.0 (1 CPU) is rated **3411 ±37** from
133 games on the list computed November 7, 2026
([engine page](https://computerchess.org.uk/ccrl/4040/cgi/engine_details.cgi?print=Details&each_game=0&eng=NGN%200.3.0%2064-bit)).
NGN is not yet on the CCRL Blitz list.

**Our estimate for the development version, early November 2026: about 3580
CCRL Blitz (8 threads).** This is our own measurement, not an official rating.
A 320-game gauntlet at 10+0.1 with 8 threads against Koivisto 6.0, akimbo 1.0.0,
Black Marlin 9.0 and Berserk 12, using their CCRL Blitz 8CPU ratings as fixed
anchors, measured **3582 [3542, 3623]**. A longer 120+1 run against three of
those anchors stopped early at 368 games, at 3629 [3593, 3665]. This version
is not released yet.

For scale: CCRL 40/15 ratings run lower than Blitz ratings (the top of each
list, Stockfish 19, is 3650 on 40/15 and 3803 on Blitz). The same kind of local
calibration put 0.3.0 at 3462 against CCRL Blitz 1CPU anchors, and CCRL 40/15
rated it 3411.

## 0.3.0 contents

- **Engine:** search, board, move generation, hashing/TT and time management
  are NGN code (the independent core first released as 0.2.0), plus the Go
  runtime and standard library. No third-party Go modules.
- **NNUE inference:** NGN code (`engine/ngnn1*.go`, `engine/ngnn3.go`), with
  NGN-written AVX2 kernels (`engine/ngnn1_kernels_amd64.s`) in GOAMD64=v3
  builds and a portable Go path otherwise.
- **Network:** NGNN2, 768 inputs → 512×2 SCReLU → 8 material output buckets,
  SHA-256 `c6c127966d1cd9848b8bbe836ae74d5672874a3e74e973a488e4d92e350416f2`.
  Trained from random initialization by `trainer/` (PyTorch); no other
  engine's network, weights or trainer were used.
- **Training data:** public Leela Chess Zero T80 (August 2022) positions with
  their search scores and game results, from
  [official-stockfish/master-binpacks](https://huggingface.co/datasets/official-stockfish/master-binpacks)
  under the Open Database License 1.0. See
  [LICENSES/TRAINING-DATA.txt](LICENSES/TRAINING-DATA.txt).
- **Classical evaluation:** NGN's hand-written HCE remains available with
  `UseNNUE=false`.

The engine uses published chess-programming techniques; acknowledgements are in
[NOTICE](NOTICE). Earlier releases (≤0.2.0-rc.1) contained third-party-derived
components; their history is kept in [NOTICE](NOTICE) and
[docs/THIRD_PARTY.md](docs/THIRD_PARTY.md).

## UCI options

| Option | Default | Notes |
|---|---|---|
| Hash | 128 | MB, 1–1024 |
| Threads | 1 | 1–64, Lazy SMP |
| Move Overhead | 100 | ms |
| OwnBook | false | small NGN-authored book |
| UseNNUE | true | false selects the HCE |
| EvalFile | `<embedded>` | path to another NGN network file |

Pondering is not supported.

## 0.3.0 local calibration

Before release, a predeclared local calibration on 2026-10-07 measured
**3462 [3430, 3493]** at 120+1 over 640 games against Counter 3.8, Counter 5.5,
Maelstrom 3.3 and Viridithas 10 using their CCRL Blitz 1CPU ratings as fixed
anchors. See
[experiments/2026-10-07-3000-claim-result.md](experiments/2026-10-07-3000-claim-result.md).

## Build

```sh
GOAMD64=v3 go build -trimpath -ldflags "-X main.releaseVersion=0.3.0" -o ngn .
GOOS=windows GOARCH=amd64 GOAMD64=v3 go build -trimpath -ldflags "-X main.releaseVersion=0.3.0" -o ngn.exe .
```

GPL-3.0-only; see [LICENSE](LICENSE) and [LICENSE-GRANT.txt](LICENSE-GRANT.txt).
