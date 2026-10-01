# NGN morning assessment — September 29, 2026

As of 15:58 UTC (11:58 a.m. EDT), the bounded continuation is finished. The
strongest selected configuration is still WDL25 at K4EvalScale 60. We now have
a tested public release and direct longer-clock evidence supporting the 3500
ballpark. A confirmed greater-than-3500 floor and strongest-Go-engine status
remain unproven.

## Delivery

The explicitly approved GPL-3.0-only [v0.2.0-rc.1 public prerelease](https://github.com/ehrlich-b/ngn-releases/releases/tag/v0.2.0-rc.1)
contains native Windows and Linux x86-64 AVX2 packages, corresponding engine
source, the selected network's original float/optimizer checkpoint, complete
training alteration method, playing evidence and preserved dependency notices.
All eight remote asset hashes match. An anonymous Windows download matches
the actually tested executable and network. Existing desktop/WSL installations
are still the historical classical build; use the new package for this configuration.

The release starts with the adjacent owned network, scale 60, OwnBook false,
Threads 1 and Hash 128. It fails when the model is missing. The scheduler
follows accepted UCI Threads unless an explicit GOMAXPROCS override is supplied.
Scale is isolated per evaluator and participates in TT identity. Required WSL
short/race/oracle tests and real Windows/Linux startup and width-eight lifecycle
checks pass. All 88 depth-12 fixtures across three alternating rounds match
the gate binary's nodes, score, depth and best move. These lifecycle checks
do not establish a multicore Elo gain.

This is a concrete distributable build to offer CCRL testers. No CCRL outreach
has been sent, no list entry exists, and tester admission remains external.
The public text claims the measured within-NGN evaluator advantage rather than
an official absolute rating. See the [publication receipt](2026-09-29-owned-release-publication.json).

## Completed playing evidence

All intervals below are paired 95% intervals from the completed fixed gates.
The first row compares evaluators inside NGN, not native Rodent engines.

| Candidate / opponent | Games | Clock | Relative Elo | Decision |
| --- | ---: | --- | --- | --- |
| WDL25 / borrowed Rodent V1.2 in NGN | 800 | 10+0.1 | +27.85 [12.60,43.22] | Independently confirms selected owned evaluator |
| WDL25 / exact native Counter 5.5 | 400 | 60+0.6 | +167.31 [144.10,192.01] | Accepted external anchor |
| WDL40 / WDL25 | 1,600 | 10+0.1 | −8.69 [−18.69,+1.30] | Shelve |
| Recovered pure-score e20 / WDL25 | 800 | 10+0.1 | −10.86 [−24.80,+3.04] | Shelve |
| WDL15 / WDL25 | 800 | 10+0.1 | +8.69 [−3.91,+21.31] | Shelve under fixed promotion rule |

WDL15's fresh 128-game A/A passed. Its scored gate is 170 wins, 480 draws,
150 losses, 400 reversed-color pairs and pentanomial `[10,67,220,99,4]`.
All 119,280 plies and operational/process audits pass. Independent off-worker
PGN recount confirms results, pairings, openings, plies and hashes. The original
float/optimizer checkpoint and network are already copied and hash-verified.
Its rule required a positive lower bound at exactly 800 games; no extension
or pooling was allowed. A small gain is unresolved, so the frozen public
WDL25 package remains selected. This comparison reused 176 of 400 opening
prefixes from earlier development and is not untouched blind confirmation.
See the [recount and exact verdict](2026-09-29-owned-wdl15-recount.json).

The independent Counter recount is 214 wins, 151 draws and 35 losses over
200 reversed-color pairs. All 69,062 plies and final operational/process
audits pass. The former rejected 400-game run remains separate and unscored.
The narrow, fingerprinted Counter post-draw-PV checker has 546 positive
diagnostics and eleven negative controls; it is not an illegal-move allowance.
Native Rodent remains held because the recorded malformed-promotion PV
mechanism is unproved. See the [Counter recount](2026-09-29-owned-counter-fresh-recount.json).

## Absolute strength and remaining work

A working judgment for the selected single-thread engine is about **3500**,
broadly **3400–3600** on a CCRL-like scale. This range is judgment, not an
absolute confidence interval. Counter's author-published 3333 CCRL 40/15 anchor
plus the measured +167.31 gives 3500.31, with sampling-only arithmetic
endpoints 3477.10–3525.01. Those endpoints omit opponent-rating, clock and
rating-pool uncertainty. They do not establish a greater-than-3500 floor.
Source: [Counter's published strength](https://github.com/ChizhovVadim/CounterGo#strength).

The continuation confirmed the existing winner rather than producing a new
stronger network. Randomly initialized project-trained weights, the owned
export/evaluator pipeline and reproducible distribution are real advances;
capacity/recipe variants have not earned promotion on the current data.

Next useful strength work is owned multicore measurement and a broader fixed
opponent suite with target-pool/clock calibration. Bigger-data training remains
the principal GPU lane once capacity is available. Search-margin retuning can
follow; more small recipe sweeps on the same opening blocks have lower priority.

## Host and durable state

No continuation training or chess job remains active, and the final host check
finds zero NGN/fastchess/Counter processes. Docker, both current Arli hopper
workers and the relay are running. C: has **30.01 GiB free**. The maintenance
window reclaimed 49.38 GiB from the VHDX and required no reboot; another
compaction is not currently needed for the completed small-write work.
The approximately 350GB data-expansion lane still lacks capacity.

Automatic compaction was not enabled. Safe offline compaction must coordinate
the keepalive and live services; experimental sparse-VHD reclamation was
refused rather than forced. No training/checkpoint data were deleted in this
continuation. Finished research and release state are committed and pushed to
the verified private branches; the curated source and approved release are public.
