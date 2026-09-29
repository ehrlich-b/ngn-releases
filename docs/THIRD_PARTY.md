# Third-party provenance for release preparation

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
| Counter compatibility evaluator | [CounterGo 5.5 source](https://github.com/ChizhovVadim/CounterGo/tree/63c487ca724c620f71c129d62129c6fb9109c872); the NGN package identifies its arithmetic and format as derived from that commit | Exact source LICENSE is GPLv3; SHA-256 `8ceb4b9ee5adedde47b31e975c1d90c73ad27b6b165a1dcd80c7c545eb65b903`. |
| Rodent compatibility evaluators | [Rodent V1.2 source](https://github.com/nescitus/Rodent-V/tree/b53ffaf670590932957cb63b7b6d871f6f33b7d8); strict external artifact loaders and independently checked arithmetic | Exact source LICENSE is GPLv3; SHA-256 `3972dc9744f6499f0f9b2dbf76696f2ae7ad8af9b23dde66d6af86c9dfb36986`. No borrowed model is in the owned bundle. |
| Stockfish compatibility and reference work | [Stockfish 18](https://github.com/official-stockfish/Stockfish/tree/sf_18); pinned source/network receipts in the experiment records | Source Copying.txt is GPLv3; same text hash as the Rodent notice above. Stockfish model licenses have their own artifact-specific provenance. No Stockfish model is in the owned bundle. |
| Owned K4 kernels | NGN's own Rodent-backend kernels, including commit `9f5f4982f310283f1f69534b06b81740be2bea09`, copied into `nnue/ngnk4` with renamed symbols and independent bounds/parity tests | Architecture compatibility and internal kernel reuse are disclosed. The repository record does not claim that every Go assembly routine is copied from upstream Rodent. |
| Bullet training toolchain | [Bullet](https://github.com/jw1912/bullet/tree/629ee50000b2afb7b3337595401c830d3b1e0f42), with recorded NGN patch/harness | Exact source LICENSE is MIT; SHA-256 `57809ec584029c063fc7052102ebcf4dcc09bab5b3e5358481ac193f7b709ec8`. Training tooling is separate from the Go engine binary. |
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
