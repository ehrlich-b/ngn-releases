# Pinned Go opponent provisioning on WSL

No head-to-head strength result is claimed. These are frozen public-release artifacts prepared for subsequent matched-condition testing. Every build and executable ran on WSL; the deployed NGN tree remained unchanged.

| Opponent | Source revision | Selected binary SHA-256 | Protocol state |
|---|---|---|---|
| Counter 5.5 | 63c487ca724c620f71c129d62129c6fb9109c872 | 6c48fb52934d49d3774633e32f0b4fb4796b1e2c63925f24167ef0f0c0d761c8 | UCI/ready/quit pass |
| Zahak 10.0 | 72ff8066e2d8348d8d3847fab62c4a6b8344240d | fdb23a317860bf8ec847d4818c62557a40601c7ee61a176acb764c17104beac0 | UCI/ready/quit pass |
| Blunder 8.5.5 AVX2 | 89230a74a966b3400610271eca49c4be241e94cd | 87e219dca06b24212a762d1ef5c220d761c3ffef99e1ec67e7d2700ea9677a3e | Sequential UCI/ready/quit pass |
| Chess-3 v4.0 PGO | a33531629cbe82eef6810f982ec814e79d52a3b3 | 28335b31fd2e818dc8db6e6188a76744fee8d9223762fe3e59046d22798db3ec | Final PGO UCI/ready/quit pass |
| Rodent V1.1 Anand/testers | 5689d0babebe95d87592eaaaee73ea555ef9345c | 9c68d7b39dc933eff5fd2da88bd4be1b0d009f6d2add7485c121cfc0d1308531 | Public artifact pinned; runtime admission pending |
| Rodent V1.2 non-Tal testers | b53ffaf670590932957cb63b7b6d871f6f33b7d8 | 9cfb8195207ee5695c1973a89664ab73b34b5bcbc10ad3bc0f0afe28b9713cbc | Latest stable artifact pinned; runtime admission pending |

Evidence root: /home/ehrli/repos/ngn-next/output/go-opponent-preflight-20260906. Counter/Zahak are recorded in handshake.json. Blunder's selected archive member and hash are recorded in blunder-binary-identity.json; its successful protocol receipt is blunder-handshake-attempt3/receipt.json. Two earlier batched-input attempts are preserved: the console-to-UCI reader handoff lost pre-buffered commands. Sequential ready barriers solve the handshake without source changes.

Chess-3 was built from the exact release tag using Go 1.25.5, Linux amd64.v3, readonly modules and the release Makefile's prescribed PGO workflow: unprofiled build, built-in 50-position benchmark profile, final PGO build. The bounded profile stage completed successfully with no surviving descendants. Profile SHA-256: 498c324ed40c73a996aebc3d7febe9848792c0653460332bc630bd42fbe34154. Final build and protocol receipts are chess3-pgo-build-v1/receipt.json and chess3-pgo-handshake-v1/receipt.json. Profiling the prescribed workload is provisioning evidence, not a speed or strength comparison.

The retained Counter, Zahak and Chess-3 handshakes each ran under a one-CPU affinity mask and advertised Threads maximum 1 in that admission context; this is not a global engine capability claim. Counter's pinned source implements multicore search; its standalone runtime width still needs confirmation under the actual match mask. Every external match must repeat the handshake under its actual CPU mask and bind the advertised option domain before setting Threads. Blunder advertises no Threads option. Counter has specific benign version/network startup stderr; Zahak prints a startup banner. The NGN diagnostic runner's exact no-stderr profile therefore cannot be silently reused for external opponents.

Rodent V1.1 is the September 5 publicly rated anchor; V1.2 is the current stable release and must be admitted separately using its non-Tal testers artifact. Legal-search smoke and full audited matches remain pending. Recheck the public release inventory before a leadership claim; do not substitute a development branch or assume published ratings share these settings.
