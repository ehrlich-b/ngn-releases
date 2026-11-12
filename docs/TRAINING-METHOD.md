# NGN 0.4.0 data and network method

The public source contains the current trainer, direct record reader, objective,
exporter and integer reference. It does not contain the corpora or checkpoints.
NGN 0.4.0 embeds NGNN3 H768/KB8/NB8, SHA-256
`2dcb6d936c0275927cb9e881f42e22558e618ad7e6fd998df6c4e5046d42c443`.
The verified lineage starts with NGN trainer revision `7ef9821`, a fresh
H512 initialization (resume null, seed 20261006), and 115 epochs. Its export
SHA-256 is `812152293dad04184599b2f9ab82779168e6e134b221de136d8f8daae86d9f22`;
its checkpoint SHA-256 is
`eb774fa78a6c30def4a336e5f1f3c46258a89c63774065161dc8e885f4c36c39`.
H768 widens that checkpoint and trains for 30 further epochs. The historical
checkpoints and corpora are not distributed in this source archive.
See `trainer/README.md` for complete NGNN1/2/3 layouts and reader behavior.

## Historical T80 transformation (2026-09-24)

Source: official-stockfish/master-binpacks revision
`1e095a758c630bc58d0b6dac4da44fcd38ac89c2`, file
`test80-2022-08-aug-16tb7p.v6-dd.min.binpack`, 10,809,713,086 bytes,
SHA-256 `0d22957b8d4f0f8e6f2913be7b744b2dab5178c3563e0f916312d0d94c28b92b`.
ODbL 1.0 applies independently of the engine/model GPL grant.

The September decoder used sfbinpack 0.6.4. It kept quiet, not-in-check
train-split positions, rejected conflicting network-input keys using the Rodent
V1.2 layout (Naman Thanki and Pawel Koziol), dropped VALUE_NONE=32002, and
applied a monotone 64-knot score mapping calibrated on Stockfish 18 positions.
Scatter split seed: 26092001; bucket seed: 2026092401; 256 buckets;
gather seed: 2026092402. Output: 2,374,290,033 STM-normalized 32-byte boards,
SHA-256 `270f65bdf9f5c948160cae5330030a60f99e94383de2a290b565ae7aad7cde26`.
The current direct reader interprets this published layout without importing
the decoder. This data lineage is separate from engine implementation lineage.

The original conflict-key input (210,844,800 bytes, SHA-256
`bff4bd784b3431559c843ce7762700e49ad02fd1930f772d7f42c015fccdf857`),
exact older decoder and historical supplemental method archive are not included.
The retained score map is `data-method/score-map.json`, byte-identical to the
recorded source. This document is a method description, not a claim that the
historical altered database can be reproduced from this archive alone.

## H768 training record

The preserved manifest names T80 corpus-a1, T76 and T60/T70 inputs in that order.
The corpus content identities are pinned below; the complete historical corpus
build inputs are not distributed. Third-party-evaluated positions are permitted
training inputs. The manifest records archive holdout modulus
1024 and split seed 20261006; H=768, KB=8, NB=8; batch 16384; learning rate
0.0002; 30 epochs × 256,000,000 positions; weight decay 0.01; WDL lambda 0.75;
K=400; cosine decay to 0.1; BF16; seed 20261006; keep-best-only. The recorded
entry point loads `widened.pt`; it is not the generic trainer's fresh-start CLI.
Current `trainer/train.py` and `trainer/model.py` show the training objective;
`trainer/archive.py` defines content-hash holdout and bounded block reads.
The export record reports byte-identical reexport and 20 integer parity cases.
Sanitized semantic records and those parity cases are in `data-method/`.
No new training, corpus fetch or strength experiment was run for this source.

## Verified weight lineage and corpus pins

The widening script checks the H512 checkpoint hash above, copies all 512 old
hidden weights and biases, copies both perspectives of the output weights and
the output biases, and preserves the king map. It seeds the added hidden weights
with 20261104 and initializes their output weights and biases to zero. Comparison
of the two integer exports confirms every retained value and every zero output.
The training entry point loads `widened.pt` strictly into NGN's model before
training. The final export matches the embedded network byte for byte.
The H768 config's resume-null field alone does not describe that injected load.
This verifies NGN weight lineage; it does not claim that the full historical
training run can be reproduced from the public archive alone.

Paths below are portable record identifiers; byte counts and hashes identify
corpus content. H512 uses T80, T76 and T74full; H768 uses T80, T76 and T60/T70.
No third-party trained network or optimizer is an input to this chain.

| Corpus path | Bytes | SHA-256 |
| --- | ---: | --- |
| `corpus-a1/train-archive.bf` | 75977281056 | `270f65bdf9f5c948160cae5330030a60f99e94383de2a290b565ae7aad7cde26` |
| `session19-data/t76.bf` | 55627628064 | `a9ad07afcfeefca0d3080a2ac06657e334bf47645448ca183b51202f4412f8b3` |
| `session57-data-step2/t74full.bf` | 183371624416 | `dffe7f87cfc40b7e46bf2cf1fb0b1b203f6cc2eb63d94267d223f45bb6c2533d` |
| `session138-net/t60t70.bf` | 10334481152 | `421e8044023205f234586545a90bcdbd550ee2cdd941fc85c64bd055dfa33364` |
