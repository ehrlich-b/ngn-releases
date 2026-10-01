# Independent position replacement — 2026-10-01

The position implementation was regenerated in a fresh personal native Luna max
context from a typed behavior contract and independently generated board,
primitives and hash helpers with synthetic callbacks/values. The preceding
position source and the controller's hidden replay oracle were not supplied.
This describes the generation process; it is not a legal clean-room certification.

The initial controller contract incorrectly said that bare kings are not a draw
under the legacy material helper. Existing NGN behavior and tests establish that
bare kings are a draw. The useful first run completed; the same saved native
context then corrected the contract, implementation and tests before integration.
Both contracts and the native evidence are retained. There was no engine policy
redesign and no change of the independent oracle to accommodate generated output.

Personal account before each turn: `ehrlich.bryan@gmail.com`, ChatGPT Pro.
Thread: `01a0f904-25dc-71c1-9f87-47f718b2dee6`.
Initial turn: 19:49:53–20:04:33 UTC; correction: 20:05:21–20:06:53 UTC.
Actual model: `gpt-5.6-luna`, reasoning max. At the correction preflight the
controller preferred user-requested `gpt-6-luna` if listed, but the complete
personal model catalog did not expose 6 Luna. The admitted useful context was
retained. Latest included weekly usage was 76%, without quota/payment denial;
no purchases, paid fallback, resets or work-account credits were used.

Independent replay includes eight FEN roots and every legal root move; full,
partial, game and null make/undo; cold/warm incremental/fresh and Polyglot keys;
castling rights, promotion and legal/raw EP; 100-ply legal game walks and undo;
repetition counts, map isolation, all 256 tag bytes and setter/turn masks;
material/clock/repetition draw controls; copy and restoration. The final ordinary
Go implementation preserves the supplied helpers unchanged and uses no unsafe
escape bypass. Real move/null mutation controls also require zero heap allocation.

Baseline and candidate produced byte-identical outputs:

- Position replay: 1,496,917 bytes, SHA-256
  `2ebb10d3fa5aeae511eca23d29e0baf13097e5ca8897e1a64c93b2bd29eefa9f`.
- Prior board replay: 2,326,306 bytes, SHA-256
  `d428770d7c354d3b9df4bd001f5a17f4b72a6260f14198a6d55b2ebe80cead8a`.
- Prior hash/EP replay: 229,574 bytes, SHA-256
  `c21d87035a2b7023b323067ac3744c5339302ab7d991f816301cbfc87f242b64`.
- Six fixed-depth HCE score/move/node/PV/perft controls: 10,932 bytes, SHA-256
  `4b9dafa94087ae370725438b14ad53bd648f4b1a942d6add7dc27f80c5fed596`.

The retained `checks.json` records baseline/candidate replay, required engine and
all-package short tests, both race variants and `git diff --check`. Every recorded
exit code must be zero before this stage can be committed. No new strength games,
Elo claims, timing benchmarks or rating-floor guarantees follow from equivalence.

All model/test work ran on the personal WSL host under the coordinated shared
CPUs 0/2 allowance: one NGN heavy unit at a time, CPUQuota=50%, Nice=10,
MemoryMax=4 GiB, GOMAXPROCS=1 and GOFLAGS=-p=1. Zahak/match/GoChessadapter,
protected benchmark CPUs, default branches and shared daemons were preserved.

This replaces the last of seven identified Zahak-derived core components in the
private candidate. Historical origins and MIT notices remain. Rodent/K4 runtime
paths were removed after attribution in earlier additive commits; PeSTO-derived
HCE values and other source/data permission questions remain. No public release,
merge, tag, CCRL resubmission or claim of complete originality/compliance is made.
