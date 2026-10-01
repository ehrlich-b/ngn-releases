# Review before performance and playing measurements

All work runs on WSL. Deployed source 53e4d1b is preserved; current integration source is 4bd54add. Root reviewed the production source and negative fixtures below, with independent Sol cross-review of the performance harness. These are review findings, not performance or strength results.

## Performance harness

Reviewed source v2 patch c3e77bee49e1b59b113553488360f33b2b841232712ca014ba1f78930c98a93e against integration 4bd54add.

The same-v3 portable-dot/AVX2 comparison isolates the kernel. Scalar/int32 contexts and whole-search comparisons retain identical model identity, with exact outcomes required before timing. The accepted pilot network has output weights -22..24 and satisfies the bounded-output capability.

Required corrections: remove start position from external B0/HCE fixtures because B0 necessarily returns its embedded book move; equalize evaluator preparation outside the cold-search timer; validate the complete measured warm trajectory; enforce explicit child environment/cwd/CPU identity and preserve raw protocol evidence; bound blocking protocol and profiler lifetimes with the accepted process supervisor; alternate comparison order and bind external samples to the declared schedule. Root opens the timing window only after other project compute stops. Cold-search allocation and repeated warm TT/history behavior must be labeled; NPS does not establish Elo.

## Candidate runner

Reviewed source bundle v2 list 01fcd8cc81934cba48391101f1f275035ce95c2e3919633029ca90ce100fd8ca; all 20 frozen source hashes independently matched. The runner has a separate diagnostic-only manifest, same-code HCE/NGN-v1 roles, frozen binaries/net/options, one-worker CPU/environment witnesses, ready barriers, explicit OwnBook=false, full trace audit and independent game replay. The accepted B0 runner remains unchanged.

Required corrections: reject generic error/fatal log levels; serialize transcript writes and inspect output through process exit; verify advertised ranges/combo values; handle parent SIGTERM/SIGINT through supervisor cleanup and terminal receipts; include copied executable paths in failure scans; detect engines that switch to a wrong cwd after an earlier valid observation; reject boolean numeric fields and nonfinite limits. The normalized HELD review-subject digest binds experiment fields before root fills execution approval. This internal experiment review does not require another user permission request.

## Multicore

Reviewed M4c v3 patch 07394fb20bf58838c5e09202f79385dcab04df17c32204a1b85e212615b9c16a. Worker zero owns time/result/callbacks; helpers use private roots/history/evaluation state and share the synchronized TT. Live node pairs use all-atomic coherent publication; final additive counters merge only after join. One-thread and node-capped paths preserve serial behavior.

Additional review asks for deterministic cancellation/panic coverage during partial launch, cleanup ownership before the first helper launch, and simultaneous NNUE-context lifecycle checks. Per-go width diagnostics must distinguish configured/search-admission capacity from actually launched workers when setup is cancelled. No multicore playing or scaling claim is accepted yet.
