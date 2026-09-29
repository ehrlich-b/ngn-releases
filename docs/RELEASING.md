# Owned NNUE releases

The owned release profile starts with the selected `ngn.nnue` beside the executable,
backend `ngn-k4-768-v1`, scale 60, OwnBook false, Threads 1, Hash 128 MiB and
Move Overhead 100 ms. A missing or invalid model prevents startup. `-version`
works without a model. UCI Threads also controls Go's scheduler unless the caller
explicitly sets GOMAXPROCS. Keep one-worker playing claims separate from SMP smoke
checks and width-specific measurements.

Ordinary development builds retain the explicit research profile. All engine
builds, tests and execution in this project run on the authorized WSL worker.
Changing a production default or search behavior requires a new verified build;
changing a network requires export parity, the frozen playing verdict, and
executable verification with that exact network.

The release procedure is:

1. Commit the application source. Verify the full short suite, short race suite,
   owned startup tests and independent move-generation oracle on WSL. Retain their
   commands, complete output, return codes and source identity.
2. Build both executables with `scripts/build_owned_release.py`, or
   `make build-owned-release RELEASE_VERIFICATION=/path/to/report.json`.
   Specify an immutable source commit and a version; the output directory must be
   new. The build manifest records every Go/assembly source hash, compiler version,
   CGO/ISA settings, startup configuration and executable hashes. Compare all
   source hashes with the stated Git commit before packaging.
3. Run `scripts/verify_owned_release.py` with the build directory and exact selected
   net. It tests the actual Linux and native Windows executables without startup
   flags, in paths containing spaces and outside their bundle directory; checks
   missing-model failure, version output, stop/restart/newgame, evaluator scale
   changes and Threads 8; then compares 88 depth-12 searches with the gate binary.
   The independent startup tests verify scheduler behavior and explicit environment
   overrides. The lifecycle check establishes no SMP Elo gain.
4. Package each executable with the same named network, usage instructions,
   manifest, checksums, applicable notices and corresponding source. Read the
   archives back and verify their member hashes. Retain the playing records and
   rejected attempts outside Git; commit the compact receipts.
   `scripts/package_owned_release.py` also verifies and includes the complete
   training alteration-method archive and its exact filter/map inputs. Include
   the compact build, test, executable and provenance receipts in the source
   archive so its documentation links remain usable after extraction.
5. Resolve the project's license and model distribution provenance before an
   external release. Draft the exact release text and destination for Bryan's
   approval, following the external communication agreement. Do not turn a
   within-engine evaluator comparison into an absolute rating claim.

The initial `0.2.0-rc.1` application source is
`8b440bf959bc527d74bd44dc39f994718453d1b3`. Its 436 source files match the recorded
WSL build, and both actual executables pass startup/lifecycle verification.
With WDL25, all 88 search results match the accepted gate binary exactly.
The alternating three-round timing ratio is 0.978; this is a descriptive timing
check on a shared worker, not a measured speed improvement.

See the [build receipt](../experiments/2026-09-29-owned-release-build.json),
[source and upstream provenance](../experiments/2026-09-29-owned-release-provenance.json),
[test receipt](../experiments/2026-09-29-owned-release-source-tests.json), and
[actual executable verification](../experiments/2026-09-29-owned-release-executables.json).
The overall >3500/competition objective remains open until direct playing evidence
and calibration support it; a release profile or an owned model alone does not.
