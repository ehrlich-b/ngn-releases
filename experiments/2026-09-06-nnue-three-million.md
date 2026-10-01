# Three-million-position corpus with fixed original holdouts

The bounded expansion is independently accepted as data, not as a trained-model or strength result. It retained 3,000,048 positions after whole-chain cross-split quarantine: 2,406,580 training, 295,751 expanded validation and 297,717 expanded sealed positions. It decoded 5,091,261 source positions in 91,529 complete chains, quarantining 267 chains containing 7,099 eligible records. The first complete accepted chain overshot the 3M total target by 48; the 6M decoded cap retained 908,739 records of headroom.

The original validation BF (97,294 positions) and original sealed BF (101,571 positions) are unchanged byte-for-byte. Future training uses these original holdouts. The expanded holdout files are audit-only.

The fixed-holdout audit checked every retained new training position against all eligible model-input keys from the old validation and sealed chains, including keys from previously quarantined chains. Intersections were zero against 97,408 validation keys and 101,623 sealed keys. It checked the immutable old prefix of 30,158 chains and 1,692,787 source/conversion records and complete-chain retention. Root verified that the audit's successful independent verifier receipt names the actual expanded manifest and frozen verifier executable.

All eight supervised stages completed with exit zero and no survivors. Peak sampled process RSS was 5,816,832 KiB in verification, below the 12 GiB bound. Three jointly modified BF/sidecar semantic corruptions were rejected by independent source reconstruction. The repaired synthetic suite passed one positive case, four real rejection cases and four always-success-stub guards. The previous v1 false-pass harness remains preserved; it never ran a real decode.

Root rehashed all 3,410 final artifacts totaling 19,076,341,611 bytes, independently checked the verifier command and holdout bindings, and rehashed the original holdout bytes. The foreground wrapper actually exited zero after final hashing; the intentionally idle SSH shell is not a surviving job.

Run: /home/ehrli/nnue-public-toolchain-20260906/runs/ngn-data-3m-fixed-holdout-v2-attempt1.

- Expanded manifest SHA: 09b410afb8d443172c87861f4881a8a1158be009f6f759699b8931525bd25244.
- Training BF: 77,010,560 bytes, SHA 8dd4c839076795219998d9651ea3498446422d90540345731be00e9d576eb72d.
- Final artifact manifest SHA: b6ec2e2d181c6947d0111e0ec9784e1b7968e94b2fabeef0fa7795180d4c5923.
- Fixed-holdout audit SHA: 0aab98487cfaee6e558096a5199d06e82e5c9adaf2840262c8cd41f9e53fca22.
- Root receipt: output/nnue-data-3m-root-review-20260906/receipt.json, SHA a23f790a099c27cd1ccfba1e31b69f62484f3cb399a91dc65a972df44d118e42.

The fixed-architecture, fixed-512-update training experiment is now independently accepted. Its source delta changed only the expanded training path/count and review text; the graph, optimizer, learning rate, labels, seed, original validation, quantization gates and selector stayed fixed. It consumed exactly 512 batches of 16,384 positions (8,388,608 presentations).

Validation raw loss at candidates 0 through 8 was 0.080705193, 0.067443423, 0.060998271, 0.058495908, 0.058438328, 0.058538766, 0.058132751, 0.058232740 and 0.058279031. All nine candidates passed the predeclared quantization gates. The unchanged selector chose candidate 6 at update 384; its raw loss of 0.058132751 is below the prior 1M experiment's best 0.067229833 on the exact same original validation bytes. Selected quantization error was mean 6.050966 cp, p99 22.058792 cp and max 39.450790 cp under this pipeline's declared score scale.

All 44 supervised stages exited zero with no survivors, timeout, memory violation or monitor error. Eighteen bridge parity checks (raw and quantized for each candidate, 16 frozen FENs each) had zero integer-versus-Go difference. All nine raw/quantized NGN artifact pairs were byte-identical. Selection A and B were identical. Root independently rehashed all 11,579 final artifacts totaling 917,126,901 bytes.

Training run: /home/ehrli/nnue-public-toolchain-20260906/runs/ngn-v0-3m-fixedholdout-512updates-v1-attempt1.

- Selected network: artifacts/training/candidates/candidate-6/bridge/raw.ngn, 197,506 bytes, SHA 00d3947d9ddd3734f502bec47f6fc7c2ece641bba5f765e8129f238007aa1a98.
- Final artifact manifest: receipts/final-files.sha256, SHA 17e067796c78de7e8ee70b973dfe709af6af82d967439bf5842f8f56a33d6492.
- Selection: artifacts/selection-a.txt, SHA 81fed2d7d8118971f0ca72df8cc02fb187dcc353912b87d15069aeec5fb818dc.
- Root training receipt: output/nnue-training-3m-root-review-20260906/receipt.json, SHA c3078b7e294a1811bfd051a52a274b0287ba848391b0f508cbe7bce011287708.

The separate four-game same-code one-worker screen is now complete and independently accepted operationally. HCE won all four games by natural mate, over 438 legal plies, at 10+0.1 with Hash64, Move Overhead50, one physical CPU and OwnBook false. Every frozen runner/trace/chess/resource/survivor gate passed. Root rehashed 86 artifacts totaling 129,400,199 bytes and independently checked all 4,000 live child identity observations.

The prospective rule was frozen before games: a winless network ends count-only data/update extensions of this minimal architecture. That stop condition is met. This is a directional failure to earn a larger test, not an Elo estimate or proof that NNUE cannot help NGN. The next evaluator path is the separately reviewed pretrained Counter compatibility control. Original-game disjointness remains unproven; later release evidence must be fresh.

- Game run: /home/ehrli/repos/ngn-candidate-runner/output/candidate-smoke-3m-20260906-attempt1.
- Game final manifest SHA: c6412b07bb0457ca3f591c15f4aabfdcaf1739ab483efa4ffedacca6bb8de805.
- Root game receipt: output/candidate-smoke-3m-root-review-20260906/receipt.json, SHA 0f38118dd4ddfa1609248034497758d1b91e7070e63ae5e7f0e5c103f31b4ea2.
