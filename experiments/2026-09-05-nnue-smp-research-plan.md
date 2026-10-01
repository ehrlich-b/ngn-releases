# Research plan: pluggable NNUE and multicore NGN

Audience: Bryan and the Sol implementation agents. Date:2026-09-05.
Decision: choose a minimal verifiable NNUE pipeline and an extensible route to external networks, especially named Stockfish architectures; define staged multicore ownership and synchronization before production changes.
Assumptions: Go remains the runtime engine; CPU inference; GPU training on the authorized WSL RTX5080; portable scalar reference; architecture-specific Go SIMD/assembly proposed. No native-Windows or Mac project computation.
Source classes: exact upstream source commits/tags, official trainer and Go/CUDA documentation, published test records; secondary discussions only for discovery.

## Steps
1. COMPLETE — Discovery: latest NNUE feature/architecture/serialization state; Stockfish release versus development; actual NGN global state/lifecycle; trainer/GPU/data readiness.
2. COMPLETE — Follow-up: reconcile compatibility, per-worker evaluator API, shared-TT publication, quantization/score conventions, data provenance and meaningful test power. Independently verify consequential claims.
3. COMPLETE — Synthesis: minimal v0, extensible interfaces, Stockfish compatibility tiers, staged SMP plan, explicit failure tests and acceptance gates.
4. COMPLETE — Verification/delivery: read back cited design artifacts, audit unsupported claims and omissions, publish a concise decision summary; then assign bounded Sol implementation.

## Distinct research lanes
- Sol nnue_architecture_research: architecture and network loading, reference inference and feature/update compatibility.
- Sol nnue_pipeline_research: proven trainer, RTX5080 stack, data, quantization/export and independent pipeline validation.
- Sol worker_state: NGN ownership inventory, Go memory-model-safe multicore, UCI/control lifecycle and scaling tests.
- Root: coordinate interfaces, review evidence, investigate measurement/acceptance, independently spot-check high-impact claims and write final synthesis.

## Gap matrix
| Claim or decision | Current evidence | Confidence / gap | Next check |
|---|---|---|---|
| Stockfish18 is not a universal NNUE file contract | Official sf_18 release saysSFNNv10 threat inputs; latest docs list later versions | High for version specificity; exact layouts pending | Pinned release/master source and serializer |
| Minimal NNUE must validate every representation boundary | Prior NGN trace/filter/checkpoint defects reproduced | High | Independent export/scalar/incremental/SIMD fixtures and mutations |
| Engine state currently blocks safe SMP | Source documents single-thread TT and ignored Threads option | High; full mutable-state graph pending | Sol ownership/lifecycle audit |
| Atomic key and payload independently do not establish a coherent TT entry | Go memory model defines synchronization per operation, not a two-word transaction | High | Choose proven publication protocol and concurrent adversarial test |
| GPU is usable for training | nvidia-smi seesRTX5080;torch/nvcc/cargo absent | Visibility only | Verified Blackwell-compatible runtime and tiny GPU workload later |
| Useful SMP gain requires real clocks and fair resource allocation | Goal explicitly requires1/2/4/8 and8vs1 improvement | High; prospective sample power pending | Two clocks, fixed total TT, no oversubscription, paired test plan |

The update_plan tool is unavailable in this session; this file records the required research plan and phases. Root completed the researched design and reconciled all three Sol reviews before assigning feature implementation. The existing user authorization covers the staged work; this research gate does not create another user approval requirement.

Discovery result: root independently verified Stockfish18/master architecture differences and search-level score adaptation, NGN cached/raw evaluator entry points, Go memory-model/race-detector requirements, and latestGo1.27 experimentalSIMD. The initial roadmap referencedGo1.26 only; implementation toolchain selection must account for1.27 and remain separately pinned. Fastchess latest release is namedv1.8.2-alpha (July26); pin and test a release rather than describing it generically as stable. Sol lanes are resolving the remaining compatibility/data/ownership details.

Final synthesis: [canonical NNUE and multicore plan](2026-09-05-nnue-smp/report-source.md), with [claim ledger](2026-09-05-nnue-smp/claim-ledger.json). Root independently spot-checked Stockfish/Bullet/Counter/Plenty/Reckless source, network archive provenance and full network SHA256; fixed source permalinks; verified report/roadmap links and git diff whitespace. Sol reviews tightened numeric bounds, validation isolation, score/model contracts, cancellable setup, direct-TT exclusivity and node-limited worker policy. No builds/tests/engines/training were run in this research phase. All four research steps are complete; the broader NNUE/SMP goal remains active.
