# Search lifecycle integration — September 6

Accepted for correctness on WSL through87f521b6c572d44f7fb65f1bfc6ee1b51552222a. Sol implemented separate P0/L0/L1/P1 changes; root reviewed each source slice and verified55 lifecycle and53 overhead gate artifacts before integration.

- P0 f5df380 removes the implicit50-million-node ceiling while preserving explicit go nodes limits.
- L0 8bfc645 replaces launch sleeps and polling with prepared/running/stopping/idle sessions, cancellation and a completion channel closed after final output.
- L1 41267de charges setup from go receipt, snapshots the root and existing time-policy state, makes setup cancellable, suppresses stale replacement output, and joins on stop/reconfiguration/EOF/quit. Failure publishes an explicit error with a legal root fallback and records the original stack.
- Test compatibility6bdbf5f retains the declared Go1.21 language version.
- P1 87f521b implements the advertised Move Overhead option, checked0..5000ms, retained through newgame. UCI default is100ms; standalone TimeManager default remains50ms.

L1 deliberately preserves the existing SetTimeControl-before-UpdateGameState policy order; changing that allocation policy remains separate. Explicit stop retains its one-bestmove obligation. Replacement, EOF and quit suppress future output and join without holding the lifecycle mutex across I/O or waiting.

Combined full short/race/vet/build and corrected deterministic search comparison all pass. Evidence: output/lifecycle-integration-20260906/evidence.json, SHA2569d05ac737dd2ae650696002e479016d1dc2d4d681716339a3a39e5df8fa2a99a. Root verified every listed artifact. The snapshot differs only in requested-source/compiler metadata; timing behavior requires its own real-clock verdict. No multicore workers are introduced by these slices.

The frozen B0 control sent100ms but its old option handler ignored that value and used50ms. Future matched tests must record effective settings; comparisons against B0 cannot pretend both engines implemented the same100ms reserve. New NNUE and HCE configurations can share the implemented policy.

Future acceptance audits must reject explicit engine error/crash diagnostics even if the legal fallback permits a game to finish. A completed legal game alone does not establish operational success. Deployment remains53e4d1b.
