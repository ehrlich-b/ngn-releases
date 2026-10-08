Current binary note (NGN 0.3.0, 2026-10-07): this inventory preserves
attribution for past versions; the table below describes 0.2.0-rc.1, not 0.3.0.
The 0.3.0 executable contains NGN code, the NGN-trained embedded network
(`c6c12796…`) and the Go runtime only: no foreign evaluator or network, PeSTO
tables, Polyglot constants, Zahak-derived files or external book loading. Its
training data and alteration method are described in
[LICENSES/TRAINING-DATA.txt](../LICENSES/TRAINING-DATA.txt); the license grant is
[LICENSE-GRANT.txt](../LICENSE-GRANT.txt). Stockfish subprocess support is
tooling-only. See the top of NOTICE for the 0.3.0 summary.

# Third-party provenance for release preparation

The inventory below records the historical 0.2.0-rc.1 release. Subsequent
replacement sections record the current recovery candidate; normal builds use
locked HCE and load no owned neural model. Historical notices are retained.

NGN's randomly initialized owned network is distinct from its compatibility
backends and source dependencies. Selecting owned weights does not remove the
other backends from the executable or settle the project's source license.
NGN's original source code and the packaged NGN-owned WDL25-e10 neural network
are licensed under the GNU General Public License, version 3 only. The complete
license text is supplied in LICENSE. Existing copyright notices and third-party
licenses remain applicable to their respective components. See the approved
[license grant](../LICENSE-GRANT.txt).

| Component | Recorded origin | Preserved license evidence |
| --- | --- | --- |
| Board, move, piece, square, hash and cache code | Amanj Sherwany's [Zahak](https://github.com/amanjpro/zahak), modified in NGN | MIT copyright/permission text remains in seven engine source files. Binary bundles need the same notice. |
| Counter compatibility evaluator | Vadim Chizhov’s [CounterGo 5.5 source](https://github.com/ChizhovVadim/CounterGo/tree/63c487ca724c620f71c129d62129c6fb9109c872); the NGN package identifies its arithmetic and format as derived from that commit | Exact source LICENSE is GPLv3; SHA-256 `8ceb4b9ee5adedde47b31e975c1d90c73ad27b6b165a1dcd80c7c545eb65b903`. |
| Rodent compatibility evaluators | [Rodent V by Naman Thanki and Pawel Koziol](https://github.com/nescitus/Rodent-V/tree/b53ffaf670590932957cb63b7b6d871f6f33b7d8). `rodentv12eval` adapts the pinned V1.2 NNUE architecture, feature mapping and score arithmetic; `rodenteval` implements V1.1 Anand artifact compatibility using tagged-source mechanism evidence. Both add NGN-specific strict loaders and independently checked kernels. | Exact source LICENSE is GPLv3; SHA-256 `3972dc9744f6499f0f9b2dbf76696f2ae7ad8af9b23dde66d6af86c9dfb36986`. No borrowed model is in the owned bundle. |
| Stockfish compatibility and reference work | The Stockfish developers’ [Stockfish 18](https://github.com/official-stockfish/Stockfish/tree/sf_18); pinned source/network receipts in the experiment records | Source Copying.txt is GPLv3; same text hash as the Rodent notice above. Stockfish model licenses have their own artifact-specific provenance. No Stockfish model is in the owned bundle. |
| Owned K4 evaluator kernels | `nnue/ngnk4` internally copies and adapts kernels from NGN's Rodent V1.2 compatibility backend: the V1.2 output kernel was introduced at `9f5f4982f310283f1f69534b06b81740be2bea09`, and the explicit K4 port is `3a92418ab73b554e262975675c014aa38d8e047e`. Symbols were renamed and independent bounds/parity tests were added. | This is implementation reuse from a Rodent-derived compatibility backend. The selected owned weights/training and NGN file/scale contract are separate; this record does not claim that every custom Go assembly routine is a verbatim upstream copy. |
| Bullet training toolchain | Jamie Whiting’s [Bullet](https://github.com/jw1912/bullet/tree/629ee50000b2afb7b3337595401c830d3b1e0f42), with recorded NGN patch/harness | Exact source LICENSE is MIT; SHA-256 `57809ec584029c063fc7052102ebcf4dcc09bab5b3e5358481ac193f7b709ec8`. Training tooling is separate from the Go engine binary. |
| PeSTO-derived classical tables | Ronald Friederich’s PeSTO/RofChade evaluation tables, copied into `engine/eval_pesto.go` in `c6675ff` and subsequently tuned in NGN | The source names Ronald Friederich but its public-domain claim has not been independently established by this audit; retain the origin and resolve the precise table terms before claiming complete compliance. |
| Binpack sampler dependency | `sfbinpack` 0.6.4 from [Disservin/binpack-rust](https://github.com/Disservin/binpack-rust), pinned by `training/nnue/k4sample/Cargo.lock` | The exact crates.io version declares GPL-3.0. This is a training-data tool dependency, not a dependency of the Go engine executable. |
| Go runtime and standard library | Go 1.25.5 used for both release executables | The installed compiler's BSD-style LICENSE is preserved; SHA-256 `911f8f5782931320f5b8d1160a76365b83aea6447ee6c04fa6d5591467db9dad`. |
| T80 training archive | [official-stockfish/master-binpacks](https://huggingface.co/datasets/official-stockfish/master-binpacks/tree/1e095a758c630bc58d0b6dac4da44fcd38ac89c2), exact revision `1e095a758c630bc58d0b6dac4da44fcd38ac89c2` | The official pinned API declares `odbl`. The 10,809,713,086-byte T80 file hashes to `0d22957b8d4f0f8e6f2913be7b744b2dab5178c3563e0f916312d0d94c28b92b`. |

The owned A1/WDL25 family uses mapped search scores and game results from that
pinned ODbL archive. The corpus, transformations, quarantines, initialization,
trainer and exports are recorded. The prepared distribution includes attribution
and the exact transformation method/inputs. Sections 4.3 and 4.6 of the
[ODbL](https://opendatacommons.org/licenses/odbl/1-0/) describe produced-work notices
and access to a derivative database or its complete alteration method. This
inventory preserves the source-data terms separately from the approved GPLv3
engine/model grant. Retain the explicit distribution decision with the release
record. See the
[pinned API and notice receipt](../experiments/2026-09-29-owned-dataset-license.json).

The compact [upstream receipt](../experiments/2026-09-29-owned-release-provenance.json)
binds the exact fetched notices. Complete notice texts are archived with the
release evidence. Preserve existing copyright notices in corresponding source;
retain the approved GPL grant and corresponding source with the binary packages.

This inventory includes direct adaptation, internal reuse, trained weights and separate data/tool dependencies. “Owned” describes the initialization and training of the distributed weights; it does not claim original evaluator source. The private 2026-10-01 audit found that the published source and Windows/Linux packages omitted the names of the Rodent authors, the Counter copyright notice and the Stockfish developers copyright notice. Those omissions are corrected here before any code removal; released artifacts and historical evidence remain unchanged. See `NOTICE` for author and modification notices.

## Private replacement stage, 2026-10-01

The Rodent V1.1/V1.2 and derived K4 runtime packages/adapters, and their K4
bridge/label/pack/finalizer/sampler path, have been removed on this task branch
after adding the notices above. The inventory is retained for historical copies.
Normal builds now default to HCE and lock evaluator switching. Counter/Stockfish
reference backends still require an explicit research build; this is an HCE
fallback, not a declaration that every source component is independently written.
Piece/square/move implementations were independently regenerated from interface
contracts and verified against hidden baseline byte/score/node/perft oracles.
Four credited Zahak core files and PeSTO-derived classical values remain.
See the experiment record for completed checks and remaining provenance gates.

## Subsequent private cache replacement

The cache implementation has now also been independently regenerated from its
interface/behavior contract and passed hidden cache-record, replacement, age,
HCE node/perft, concurrency and regression/race checks. Three credited Zahak
core implementations remain: board, position and hash. PeSTO values remain.
See experiments/2026-10-01-independent-cache/report.md.

The canonical Polyglot table in engine/polyglot_random.go is from the pinned
MIT Disservin/chess-library source cited in its header and NOTICE. All 781
values and the retained LICENSES/chess-library-MIT.txt were independently
verified. Its defined values are preserved for external book interoperability;
the independent python-chess oracle is separate from this table source.

## Subsequent private hash replacement

The hash implementation has also been regenerated from its behavior contract.
All key values and the incremental/restoration proof match the immediate base;
existing independent EP/repetition/Polyglot oracles and full required regression/
race checks passed. Two credited Zahak core implementations remain: board and
position. PeSTO values remain. Evidence: experiments/2026-10-01-independent-hash/report.md.

## Subsequent private board replacement

The board container was regenerated from a behavior contract and passed hidden
occupancy/accumulator/castling/capture, hash restoration, HCE node/perft and
all required short/race checks. Position is the one remaining credited Zahak
core implementation. PeSTO-derived values remain with unresolved precise
original permission; the unsupported public-domain assertion is corrected.
Evidence: experiments/2026-10-01-independent-board/report.md.

The May 28 classical-tuning record names KierenP/ChessTrainingSets. Its pinned
mirror declares MIT, Copyright (c) 2020 Kieren Pearson, and credits Alexandru
Moșoi for quiet-labeled.epd. The full license is retained in LICENSES and NOTICE.
Exact training-file match and original contributor permission chain still need
verification; this evidence does not license the separately borrowed tables.

## Subsequent private position replacement

The position layer was independently generated from a typed behavior contract,
fresh helper implementations and synthetic callbacks/values. A controller
mistake about the legacy bare-kings draw case was corrected in the same saved
native context before integration; initial and corrected contracts are retained.
Hidden move/undo/null/EP/castling/promotion/repetition/draw/copy replay, prior
board/hash controls, fixed HCE node/perft outputs and all required short/race
checks passed. All seven identified Zahak-derived core components have been
replaced; historical notices remain. PeSTO-derived values and other documented
source/data provenance gates remain open. This is not a claim of legal
clean-room certification or complete originality.
Evidence: experiments/2026-10-01-independent-position/report.md.

## Current independent HCE release candidate

The current candidate removes the complete old HCE, PeSTO and legacy tables,
explicit Counter draw scaling, inherited magic constants and all generic,
Counter/Stockfish NNUE packages/adapters. Its new classical core has original
analytic tables and hand-chosen values, with no training corpus or network.
Fresh coordinate rays replace the old magic implementation. Old tuning/model
features and their specific tools/tests are retired; legality, key/repetition,
restoration, accumulator, ownership/cancellation and independent search-stack
controls remain. No playing-strength claim is made. The old published release
and all preceding notices stay historical evidence.

Canonical Polyglot constants remain disclosed interoperability data under the
verified pinned Disservin MIT permission; this release does not pretend to have
invented that format data or standard chess algorithms. No foreign evaluator or
NNUE package appears in the compiled main dependency closure. This documents
the identified replacements and test evidence, not blanket legal certification.
Evidence: experiments/2026-10-01-original-hce-release/report.md.

The final source-package sweep also removed the inactive training/nnue K4/Bullet
scaffold from the current tree. Historical Bullet MIT attribution is retained.
No Go runtime source changed after the verified 629bd170 engine stage.

Current binary update, 2026-10-05: external Polyglot loading and its canonical
constants are removed, with their dependent tests and fixtures. Historical
Disservin notices and licenses remain. Current sliding multipliers are generated
by NGN's cmd/magicgen from a recorded NGN seed, not from an external table.
The older candidate-state paragraphs above document stages, not current contents.
