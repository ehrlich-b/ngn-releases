# Counter 5.5 pretrained evaluator control

## Why this is the next practical imported-network candidate

The minimal NGN-v1 training/inference pipeline is validated, but its first selected models have not earned replacement of HCE. A separately verified pretrained evaluator can help distinguish training/model quality from integration into NGN search.

Counter 5.5 supplies a simpler general-use pretrained network than the first Stockfish-small compatibility milestone. Its exact release source and model are available at commit 63c487ca724c620f71c129d62129c6fb9109c872. This does not imply that importing the evaluator imports Counter's playing strength, nor that it will beat NGN.

Primary source: https://github.com/ChizhovVadim/CounterGo/tree/63c487ca724c620f71c129d62129c6fb9109c872/pkg/eval/nnue . The official v1.55.0 release states that it fixed a training bug, switched to 768x512x1, and trained a new network. Repository metadata records GPL-3.0; retain exact source/model provenance and attribution in any implementation.

## Observed exact contract

- Model n-30-5268.nn: 1,576,988 bytes; SHA256 3488baed71f4d432d028e05b65e2ff13f8c9afe7f91974d1a3c34a0691d6670c. Root verified the downloaded model and seven source files against the pinned Git tree's blob identities.
- The legacy prefix is exactly24 bytes: hex425a02000100000000030000010000000100000000020000, or LE u32 values154178,1,768,1,1,512. The old reader ignores these fields; our named compatibility loader must validate the supported prefix and exact EOF rather than reproduce that permissiveness. Do not invent unverified meanings for its fields.
- Payload is394,241 little-endian float32 values: input-major 768x512 FT weights,512 biases,512 output weights,one output bias. All pinned values are finite, ranging from-40.7199631 to43.4409142.
- One absolute white-perspective accumulator. Features use piece type/color/square, with black pieces offset by6 and no perspective flip. Verify piece enum and A1 indexing against the pinned common package before freezing fixtures.
- Full refresh initializes the512 biases then adds occupied-square feature rows in ascending square order. Output is a float32 ReLU dot plus bias. The portable release implementation's arithmetic order is the first oracle.
- Counter's score adapter truncates raw white score to int, clips to15000, applies a non-pawn-material factor, applies its200-based rule50 factor, then changes sign for black to move. This differs from NGN-v1's score contract and cannot be silently replaced by the NGN-v1 adapter.

## First Sol implementation slice

Create an isolated package and worktree. Implement the strict named legacy loader and full-refresh portable reference only. Preserve NGN-v1's existing fixed format and HCE behavior; no search wiring in this slice.

Build a test-only oracle from the pinned Counter release with its portable evaluator and a pinned Go/compiler target. Expose exact active feature indices, accumulator lanes, raw output, and the upstream adapted score separately. Compare independently mapped NGN positions against the actual Counter parser/evaluator. Include both sides to move, asymmetric material, kings on different files/ranks, promotions, castling and en-passant positions.

Require checked lengths/header/tensor counts/EOF, rejection of nonfinite tensors and malformed inputs, and bounded arithmetic for accepted models. Separate the canonical portable float32 contract from future AVX/FMA reduction order. Do not claim incremental-to-fresh bit equality for floating accumulators without evidence.

The next slice adds private incremental contexts and exact transition order, tested against the upstream incremental oracle and full refresh with explicit floating-error criteria. Only after those pass should the backend receive transactional selection and a separately registered score adapter. Search tests must cover model identity/history/TT invalidation and every static/emergency evaluation route.

Then profile and optimize, run an operational game screen, and test playing strength prospectively inside NGN's own search. The full Stockfish18 small/big compatibility ladder remains a separate deliverable; this simpler control does not replace it.

Evidence cache: output/counter-pretrained-control-research-20260906. No Counter-based NGN evaluator has been implemented or run yet.
