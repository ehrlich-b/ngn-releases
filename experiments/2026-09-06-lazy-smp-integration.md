# Real Lazy SMP integration

Integrated as 168385f on WSL. Deployed /home/ehrli/repos/ngn remains accepted 53e4d1b. This is a correctness acceptance, not a strength or scaling result.

Iterative time/depth searches now run the configured private workers. Worker zero owns the TimeManager, result and completed-depth callbacks; helpers share only the synchronized transposition table. One Hash allocation remains the configured total. Fixed-node searches explicitly run one worker. Helper setup copies and resets private roots/evaluators under one session/model lease, checks cancellation between phases, and advances TT age once per iterative search.

The cleanup owner is installed before the first helper launch. Primary/callback/setup panic or helper panic cancels and joins every launched helper before the session lease is released. Normal helper completion does not stop the primary. The original primary panic takes priority; helper panic records retain the original helper stack. Per-go diagnostics report the actual admitted prefix after launch, including effective one when setup is cancelled. Only additive counters merge after all helpers join; result/depth/PV/clock ownership remains primary.

Reviewed production patch v5: 5d42f127bf7ff5f4c8535985c026834fc283b62fcee2ecd0b06bd477495ae96f. Evidence SHA: 13ecc94fc80a9ec66fb9f89586816c6c0b27e48f35fdbbf2791c90eafcb5c35f. All ten gates passed: focused race, full short, full short race, vet, build-all, snapshot build/record/compare, engine build and UCI handshake. Corrected-B0 one-thread snapshots differ only in source/build metadata. Root independently verified all 15 evidence artifacts.

Test-only v6 amendment d3b27c6e35f113d56384f061d16ca455953257773cb335ca2175ee6008d37c0a strengthens the NNUE concurrency witness: all three workers block inside their first pushed frames, distinct nonnil Int32Context pointers are checked, then authoritative stop/release/join must restore depth zero and full-root score parity. NewGame retains private balanced contexts; a model switch replaces them. Its focused race test passed; production v5 bytes stayed unchanged. Evidence SHA: 123c341c6bb8497da99f16e3ad33b4f008b21f1d4049ab36ca6171b45a4d06ed.

Root applied v5 plus v6 to the integration tree and verified byte equality for every changed source file against the validated combined worktree. Root source receipt: output/m4c-root-integration-20260906/source-identity.json, SHA789e2cec50df9c60c8cb6b0c5e9a610f64cfbc854258bc71dd628726ec33d7ad. There was no reason to repeat unchanged broad tests during the isolated performance window.

Next: controlled 1/2/4/8-thread operational/scaling measurements, then predeclared paired real-clock tests. Helpers currently use the same search policy; useful speedup or Elo improvement is not assumed.
