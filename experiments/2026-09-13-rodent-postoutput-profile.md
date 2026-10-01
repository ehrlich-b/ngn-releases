# Rodent post-output profile: accumulator updates dominate

One bounded profile on accepted `7623779`, Linux/amd64 v3, completed cleanly:
six existing fixtures, 400,000 nodes per search, Hash128/Threads1, 30 reported
searches plus six calibration searches. The supervisor recorded 24.39 seconds,
no timeout, memory breach, orphan or surviving process. No engine ran on the Mac.

Search-scoped samples account for 24.15 of 24.17 sampled seconds. Accumulator
`applyUpdates` consumes **58.21% flat / 61.19% cumulative** CPU. The accepted
AVX2 output kernel is now 2.69%; frame copying is 3.10%, full refresh 2.61%.
This identifies the next hotspot; comparing these profile durations is not a
controlled speed measurement or an Elo estimate.

Next candidate: exact modular int16 AVX2 add/subtract across the existing update
rows, with portable fallback. Preserve validation, feature mapping, refresh,
frame copying, null/pop and search policy. Require adversarial lane-bit parity,
existing release/transition oracles, engine tests and a predeclared repeatable
search-speed gate before acceptance. Do not combine copy fusion or heuristics.

[Results and protocol](2026-09-13-rodent-anand-artifacts/postoutput-profile/results.json)
and adjacent raw benchmark, supervisor and pprof text are archived with checksums.
Binary profiles remain in the isolated WSL output directory recorded there.
