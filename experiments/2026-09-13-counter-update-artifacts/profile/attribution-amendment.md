# Attribution amendment

The canonical affected-path estimate remains the sampled source-line view in
`report.md`: **7.00% cold** and **5.23% persistent**.  It adds the accumulator
copy assignment line to the disjoint cumulative `applyFeatureUpdates` call.

An interim v2 archive instead headlined 6.44%/5.47%, using flat AVX2 row kernels
plus the `runtime.duffcopy` caller edge from `SearchContext.PushMove`.  That was a
useful caller-level proxy but is not accumulator-exclusive: the function also
copies boards and arguments, and sampling/inlining moves copy samples between
the source line and `duffcopy`.  Therefore v2 is superseded for attribution and
must not be treated as the canonical report.

Both views support the same bounded admission, but neither predicts realized
savings.  The one admitted copy-plus-ordered-updates fusion still has to pass the
frozen cold and persistent whole-search gate; no additional profile is warranted.
