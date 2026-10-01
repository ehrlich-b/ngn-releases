# NGN published-source provenance review — 2026-10-01

Scope: public NGN 0.2.0-rc.1 source and Linux/Windows binary archives, their exact application source, introduction commits, pinned upstream sources, model/training receipts, and the isolated private replacement branch. This records evidence and unresolved licensing questions; it is not a legal certification or a finding that the whole engine is a Rodent clone.

## Exact release reviewed

The public source archive SHA-256 is `16097bb6606347ca4c873b7a5929f87bf3ec47a855a68a37379f1c96048b15a1`. Its 436 build-source files all match application commit `8b440bf959bc527d74bd44dc39f994718453d1b3`. Source publication used `7444eccfe7...`; packaging used `38883704...`. Publication was 2026-09-29 08:35:08 America/New_York.

Linux archive: `6763e81cb1a33e7b71274a3bc994096f6a6d0bc3a4ad8b5232c0de37660978b8`. Windows archive: `c5ecf446578111142a4f41d88884c6c0ea2bb1746a2d0682f9d81d87a7ba8b29`. Both public archives were read and hashed; neither engine executable was run on the Mac. The shipped network is the NGN-initialized/trained WDL25-e10 model, SHA-256 `1ec8fc1737ddfdd5f8b6ff0b4e26778085fb563f7c5e82c1fc5d3ea454b6ec29`. New weights and original evaluator implementation are separate claims.

## Where code or data came from

| Origin | Where and transformation | License/notice findings | Originality consequence |
| --- | --- | --- | --- |
| Rodent V, Naman Thanki and Pawel Koziol | `rodenteval`, `rodentv12eval`: feature mapping, buckets, tensor layout and evaluation arithmetic adapted to NGN, with strict loaders, incremental contexts and optimized Go/assembly kernels. `nnue/ngnk4` subsequently reused this feature design and internally copied Rodent-backend kernels while using newly trained weights. | GPLv3 texts and project links were distributed, but both named authors were absent from the public source and both binary bundles. The pinned Rodent V implementation is **Go**, including `nnue.go`; older Rodent projects being C++ does not change this source. | These are runtime evaluators, not only a training teacher. New weights do not make the evaluation implementation independent. No complete file equality was found in the upstream comparison; that does not establish independence. |
| CounterGo, Vadim Chizhov | `countereval` and Counter transition/score adapters port the 5.5 model layout and evaluation arithmetic, then add NGN validation, lifecycle and optimized kernels. | GPLv3 text/project link present. Upstream's explicit `Counter Copyright (c) Vadim Chizhov. All rights reserved.` notice was missing from the public source and binary notices. | A separate borrowed reference backend. HCE fallback does not select it. |
| Stockfish developers | `nnue/sf18small`, `nnue/sf18big`, feature definitions and score adapters implement Stockfish 18 formats/arithmetic in Go. This is a C++ to Go adaptation. | GPLv3-or-later upstream. Its `Copyright (C) 2004-2026 The Stockfish developers` notice was missing from public source and binary notices. No Stockfish trained network was in the owned bundle. | Translation changes the language, not the source lineage. Explicit research/reference backends remain separate from HCE. |
| Zahak, Amanj Sherwany | `engine/bitboard.go`, `cache.go`, `hash.go`, `move.go`, `piece.go`, `position.go`, `square.go` began as modified Zahak code. | MIT permission/copyright text is preserved in seven source files and the binary notice. No missing-author finding here. | These still matter to the user's independent-code goal, despite valid credit. Rewrite from behavior contracts and verify compatibility before activation. |
| PeSTO/RofChade, Ronald Friederich | `engine/eval_pesto.go` copied material/PST values, then tuning changed those values. | The file names the author and asserts public domain. The precise original table permission has not been independently established. The Chessprogramming Wiki page footer is not proof of the author's public-domain dedication. | HCE fallback still contains derived classical tables; changing an implementation around the same table does not create original table values. This remains a replacement/permission gate. |
| Bullet, Jamie Whiting | Separate MIT training toolchain, local queue/dependency patch and harness. | MIT copyright/permission notice preserved in source and binary packages. | A tool dependency, not a transplanted runtime evaluator. It need not be rewritten merely to use an original engine. Retired K4 generation must not be mistaken for a new independent training pipeline. |
| sfbinpack / Disservin binpack-rust | `sfbinpack` 0.6.4 is pinned by the Rust sampler. | Registry declares GPL-3.0; it was absent from the earlier direct inventory. 42 pinned dependency metadata records were reviewed; most declare MIT/Apache family terms, with unicode-ident also declaring Unicode-3.0. No compiled sampler was shipped in the engine bundles. | Separate data-tool dependency. Metadata alone does not certify all notices required for redistributing a compiled tool. |
| Go Authors | Go runtime and standard library, no third-party Go module dependencies. | BSD-style notice preserved. | Ordinary language/runtime dependency. |
| Stockfish master-binpacks T80 archive | Mapped search labels/game outcomes, filtered and sampled, used for randomly initialized NGN network training. | Pinned dataset revision declares ODbL 1.0. Attribution and transformation method/inputs were included separately. | Data rights are separate from GPL source/model rights; original weights do not mean independently sourced training data. |

`NOTICE` in attribution commit `d00dd2b1a878393041530a625d349b35dd94434c` supplies the omitted Rodent names, Counter copyright and Stockfish developers notice, source links, modification dates and transformation descriptions. It preserves the other notices and records the PeSTO uncertainty and sampler dependency. That is a private correction, not a retroactive alteration of the published archives.

GPLv3 is an open-source copyleft license, not a permissive license. Choosing GPLv3 for NGN does not replace component copyright notices, source requirements or database terms. License permission and the user's desire for original hot-path implementation are separate questions.

## How this accumulated

Times below use committer time in America/New_York, not author time.

| When | Commit | Event |
| --- | --- | --- |
| 2024-07-05 | `23ce9...` | Early board implementation; the project predates the later Rodent evaluator imports. |
| 2025-08-30 | `584f688...` | Search already exists before Rodent integration. This alone does not prove every search line original. |
| 2026-04-23 20:49 | `c6675ff` | PeSTO tables introduced. |
| 2026-05-28 | `bb12a48` | Classical values tuned using a Zurichess dataset; that dataset's precise origin/terms remain to be checked. |
| 2026-09-06 09:03:20 | `ed9ee07`, `e88d36c` | Counter and SF small compatibility introduced. |
| 2026-09-06 19:37:53 | `18e1c16` | SF big compatibility introduced. |
| 2026-09-13 15:38:53 | `11ca24e` | Rodent Anand runtime compatibility introduced. |
| 2026-09-19 07:25:27 | `f2df112d` | Rodent V1.2 compatibility introduced. |
| 2026-09-19 12:42:40 | `9f5f4982` | Rodent V1.2 AVX2 output kernel introduced; later accumulator updates/refresh followed that day. |
| 2026-09-20 17:00:30 | `c1fb1212` | Owned-network plan deliberately reuses Rodent architecture/kernels while initializing/training weights independently. |
| 2026-09-20 18:30:32 | `c51114b` | K4 generation/training path introduced. |
| 2026-09-21 21:59:47 | `7f6b1f4` | K4 AVX2 implementation added; not every routine's earliest origin has been proved. |
| 2026-09-27 19:34:47 | `3a92418` | Explicit Rodent V1.2 kernel port into K4. Eight of twelve normalized comparisons are byte-identical; three assembly pairs are approximately 98–99.7% similar. |
| 2026-09-29 05:36 | `8712760` | THIRD_PARTY inventory added, with project names/links and licenses but incomplete named authors. |
| 2026-09-29 05:59 | `b4896d9` | Release notices/data method packaging added. |
| 2026-09-29 08:27:17 | `dced819` | GPL source/model grant. README was shortened and its prior-art links removed, but proper named Rodent author credit was not present before that either. |
| 2026-09-29 08:35:08 | public release | Source and binaries published with those omissions. |
| 2026-09-29 20:47:13 | forum post 155487 | CCRL request submitted. |
| 2026-10-01 01:23:23 | forum post 155507 | Naman Thanki's account objects to missing named attribution and Rodent copying. No verified ban or rating decision follows from that reply alone. |
| 2026-10-01 03:18:24 | `dccbbd51` | Earlier private two-row attribution correction prepared. |
| 2026-10-01 daytime | `d00dd2b1` | Broader private source/author/modification notice correction committed before deletion. |

The evidence supports “we credited the projects and distributed GPL texts, but omitted named notices and reused runtime evaluator implementations.” It does **not** support “proper named author credit existed in the source and was accidentally lost when copied.” The September 20 plan also records intentional architecture/kernel reuse; the user's statement about intended temporary scaffolding is his own explanation, not a historical fact proved by the commits.

## Replacement work and remaining gates

The isolated branch is `task/hce-runtime-20261001` at `/home/ehrli/ngn-independent-eval-20261001/hce-candidate`; default branches and the existing benchmark owner are untouched. Attribution is committed first; subsequent deletion is additive Git history. Released assets, external reference sources, old logs and experiment records remain intact.

The private patch removes the Rodent V1.1/V1.2 and K4 runtime packages, engine adapters, backend-specific tests and the retired K4 bridge/label/pack/finalizer/sampler path. It makes the normal build HCE-only at startup, locks GUI evaluator switching, and fails closed for the old owned profile. Remaining Counter/Stockfish backends require an explicit research build. Existing unrelated match controllers and packaging records are retained; old owned packaging is not a valid path for this candidate.

A fresh personal Luna max thread created a standalone bias-free 10→8→8→1 tanh evaluator, using no existing engine source or model in its supplied context. Its tests cover deterministic initialization, perspective symmetry, integral feature restoration, finite/bounded validation, transactional training and serialization. Independent controller tests additionally compare every SGD parameter derivative against central finite differences and prove rollback after a successful private update followed by a rejected one. This is an original-context implementation experiment, not a formal legal clean-room certification or proof of playing strength. It is not yet an activated production neural evaluator; HCE is the fallback authorized by the user.

A second fresh-context Luna max worker rewrote the three piece/square/move primitives from behavior contracts. An independent review caught a Rank.Name return-type violation and the same worker corrected it. The generator did not see baseline source or the controller's compatibility oracles. All 256 int8 values, 65,536 type/color and file/rank pairs, and 100,000 opaque packed-move records now match the baseline byte-for-byte; the 2,328,430-byte proof hashes to 8339c967b681198fcff26756793c565e5e549ab95a6e03893058fe07c1b49737. Six fixed-depth HCE cases match exactly in scores, moves, nodes, PVs, root restoration and perft. Full engine/package short and race regressions passed after integration. The remaining four credited Zahak core files are board, position, hash and cache. The remaining Zahak board/position/hash/cache implementations, PeSTO values, generic NNUE provenance, magic-number origin, Polyglot constant-source notices, and Zurichess data provenance still need concrete resolution. Shared techniques such as alpha-beta, null move, LMR, SEE and bitboards are not by themselves evidence of copied expression; search source comparisons must distinguish algorithms from actual transfers.

No new strength games, remote push, tag, release, deployment or code resubmission is authorized by this record. HCE's historical 2884 ±50-style calibration is not a current official rating or a verified strength floor for a new network. A production neural switch or changed HCE values needs a resource-coordinated predeclared strength gate before making a 2800-floor claim.

## Evidence retained locally

- `public-source-build-binding.json`: all 436 build-source matches.
- `public-binary-package-audit.json`: public archive members, hashes and notice scans.
- `published-source-name-scan.json`: named-author absence in the release source.
- `file-introduction-history.json`: introduction/commit dates.
- `upstream/pinned-code-fetch-receipts.json`: 133 Git blob verified pinned source files, no failed fetches.
- `sampler-dependency-license-inventory.json`: 42 exact registry metadata records.
- `attribution-commit.json`: attribution-first private commit receipt.
- Native session/events, source snapshots, task prompts and validation logs: original-context generation and actual test results.

Primary origins: [Rodent V pinned source](https://github.com/nescitus/Rodent-V/tree/b53ffaf670590932957cb63b7b6d871f6f33b7d8), [Counter pinned source](https://github.com/ChizhovVadim/CounterGo/tree/63c487ca724c620f71c129d62129c6fb9109c872), [Stockfish 18](https://github.com/official-stockfish/Stockfish/tree/sf_18), [Zahak](https://github.com/amanjpro/zahak), [Bullet](https://github.com/jw1912/bullet/tree/629ee50000b2afb7b3337595401c830d3b1e0f42), [PeSTO origin page](https://www.chessprogramming.org/PeSTO%27s_Evaluation_Function), [GPLv3 text](https://www.gnu.org/licenses/gpl-3.0.html), [GNU translation FAQ](https://www.gnu.org/licenses/gpl-faq.html#TranslateCode), [ODbL text](https://opendatacommons.org/licenses/odbl/1-0/).
