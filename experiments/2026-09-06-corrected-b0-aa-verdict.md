# Corrected B0 A/A control accepted

The exclusive WSL control completed on September 6 at 05:03:19 UTC. It played 200 games / 100 reversed-color opening pairs at 30+0.3 with concurrency 8 on distinct physical cores, fixed Hash64 and Threads1, strict fastchess parsing, no recovery, and natural game endings.

Both sides used the exact corrected B0 binary from b134e21. B0PV_A scored 98.5/200 (49.25%), inside the prospectively declared 40–60% runner-sanity range. Pentanomial pair counts were [3,24,48,23,2]. This is an identical-engine control and establishes no playing-strength gain or new rating.

Independent pinned Stockfish replay validated every one of 31,750 played plies and all 200 terminal outcomes: 97 checkmates, 83 threefold repetitions, 14 insufficient-material draws, and 6 fifty-move draws. Both runner and audit exited zero with empty stderr. No warning/error/fatal log lines or live chess processes remained. Root rehashed all 39 artifacts in the final manifest and reconciled game, pair, score, and ply totals.

Frozen run: /home/ehrli/repos/ngn-next/output/nnue-smp-b0-pv-corrected-20260905/aa-200-tc30

- Final manifest SHA256: f01b3c14d4cc8cbbc035fb27d75151c5567ebcff2d519af4258a7300c0f21edc
- Audit SHA256: 828cb50b8bbb8a0240b9ff4c2f5dc527a7c0b647270764c357e0c7b0e2fb7f19
- PGN SHA256: fbf9d49a31acfea62a2a7022be25e1eac05ba0a3a44675c4212bb982fb48e9c3
- Root review SHA256: d65bf596671da6559b99cf47bbf02b3279a92790ef4a7f09a22eaaccf36d0685

The full window from supervisor admission at 04:20:46 to terminal audit state was approximately 42.5 minutes. This is measured control throughput, not a promised candidate duration or an Elo-power estimate.

[The overhead source audit](2026-09-06-move-overhead-option-audit.md) establishes that the recorded 100 ms option was sent but ignored by this baseline; its effective compiled overhead was 50 ms on both sides. Frozen inputs remain unchanged.

The earlier pre-repair failed control and the initial approval-field launch rejection remain separate retained artifacts. Neither contributes games to this accepted run.

The exclusive match window is closed. Root released bounded correctness work: M3 on CPUs0,2; N3/N4 on4,6; private Rust/data-tool provisioning and synthetic tests on12,14. No real T80 processing or next timed match is admitted by this control verdict alone.
