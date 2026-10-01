# NNUE optimization slice after runtime parity

This is a proposed bounded implementation, not measured speed or playing strength. Keep the accepted int64 scalar evaluator/context as the numerical reference. Finish the production move/null/undo adapter and score policy first. Target the pinned Go 1.25.5 Linux amd64 v3 toolchain with Go assembly and a portable reference fallback; do not make compiler migration a prerequisite.

## Safe narrower arithmetic for the current architecture

The strict NGN v1 model has int16 feature weights and biases, 128 hidden units, two perspectives, and at most 64 occupied squares in its public Position contract. Therefore every refreshed accumulator lies within [-2129920,2129855], including bias. Checked delta updates remove before adding and preserve at most 64 pieces, so int32 accumulators are sufficient without overflow or saturation. A separately implemented int32 context could halve accumulator copy volume. It must match every int64 accumulator lane, not only clipped scores, after all legal transitions and undo.

SCReLU clamps each activation to [0,255]. For a model whose 256 output weights each lie in [-128,128], activation times output weight fits signed int16: magnitude at most 32640. Multiplying that result by activation and summing all 256 terms fits signed int32: magnitude at most 256*255*255*128 = 2130739200, below 2147483647. This gives a useful exact packed multiply route for the current Bullet exporter, whose accepted clipping at +/-1.98 and output quantizer64 produces output weights within +/-127.

This condition is a checked runtime capability of an immutable model, never an assumption about all valid NGN files. The container permits full int16 output weights. Those models must keep a wider dot-product path. Do not clamp their weights or change their numerical result to obtain the fast path.

After reducing the dot product, widen to int64 and retain the exact existing evaluation order: dot/255 with signed truncation; add output bias; multiply by400; divide by255*64 with signed truncation. Collapsing the two divisions or multiplying the scaled value in int32 changes semantics or can overflow.

## Implementation and acceptance order

1. Add a portable int32 accumulator implementation behind an explicit mode with checked model capability metadata. Preserve the int64 implementation as an independently runnable oracle.
2. Compare full lanes and raw scores on dense signed tensors, boundary output weights128/-128 and neighboring unsupported129/-129, extreme full-int16 fallback models, every special move/null/undo, stack limits, rejected transitions, and independent contexts under race.
3. Add one amd64 assembly kernel for the bounded output path and one add/sub feature-row kernel. Keep feature mapping, validated deltas, score policy and model loading outside assembly. Use appropriate CPU-feature dispatch; GOAMD64=v3 is the initial benchmark artifact, not a claim that every machine supports the kernel.
4. Check assembly versus scalar on every previously frozen oracle case plus randomized accumulator inputs within proven bounds. Test signed truncation and the side-to-move half order explicitly. No per-node allocation or cgo transition.
5. Profile actual NNUE search before and after on WSL and run the predeclared real-clock candidate test. Kernel throughput or NPS alone does not establish strength.

Do not bundle TT layout, Go compiler/PGO, search margins, network width or training changes into this optimization verdict. A future network architecture supplies its own numerical bounds and inference implementation.
