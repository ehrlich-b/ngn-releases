# Futility controls and targeted skip traces

Completed offline on September5; accepted qcap runtime remains unchanged.
This follows the [seven-case ablation](2026-09-05-search-ablation.md).

## All remaining controls

Before searching, freeze the other16 selected positions from the same23-game
study. Compare the exact qcap and existing Futility-off binaries at400k nodes,
Hash64/Threads1, fresh game state and full history. Score distinct continuations
with SF depth16. All16 baseline final UCI records match the prior study, all
chosen moves pass the separate SF-perft audit, and all21 child scores are
usable at completed depth16. The originally written protocol said legality
would be checked first; the executed driver instead checked it in the separate
post-run audit. That audit passes, so no scored illegal continuation was used.
The committed reproducer moves this check before scoring and explicitly rejects
incomplete-depth scores; the original retained records are unchanged.

Futility-off changes five moves: differences are **+11,+35,−83,−33,−15cp**.
There are no50cp improvements and one50cp regression. The other11 choices
are unchanged. Its completed depth drops in eight cases, rises in two and stays
the same in six, illustrating the work saved by pruning. These controls were
selected by excluding the seven known large baseline disagreements; neither
stratum is a representative performance sample.

The largest regression is GAME179, where **Rf6** becomes **Qe8** (`d8e8`)
and the independently scored choice worsens83cp. These results do not reject
the entire futility lane, but do contradict treating removal as a free gain.

## Direct trace result

Before instrumented searches, freeze the five previously improved roots and
all three worsened controls. For each, watch the position after the baseline
move and eight further positions along a fresh completed-depth16 SF reply line:
72 watched states across eight roots. The instrumentation observes move
consideration, futility skips and the surviving make/search path. It checks
legality/check status on an isolated board copy, leaving the playing position
and search learning alone. All eight instrumented400k searches retain the exact
baseline final score/depth/PV/nodes/bestmove.

The trace contains29,388 records and453 futility skips in watched states,
but **none skips the corresponding watched SF continuation**. In particular,
GAME188's **Ra8+** survives futility on all five recorded attempts; GAME197's
**Ka1** does so on all41. This falsifies the narrow explanation that these
immediate reference replies were omitted by futility. It does not prove that
futility elsewhere in the tree is harmless, or that the watched line explains
the entire scored difference. No heuristic change follows automatically.

The stronger bounded conclusion is that removing futility changes how search
resources, ordering and bounds develop. A targeted pruning fix still lacks a
proved local mechanism. The next ready game candidate remains the unfinished
T17+T8 interaction, rebuilt on qcap, while the fitted evaluator completes its
separate frozen game gate. Do not bundle these diagnostic overlays into either.

## Artifacts

Raw directory `output/recovery-2026-09-04/`:

- `futility-controls-selection-2026-09-05.json`:
  `de43f763b56d3f7150572347693860bf36e81905d0e264554184a6a591b9267f`.
- `futility-controls-result-2026-09-05.json`:
  `1b54c04794642521504b944cede0636400e93e31e7eed64e6822773a5959c146`.
- `futility-controls-audit-2026-09-05.json`:
  `c0d15b1bdc8b3b7a14f3d6d4372b4129d3037c6691489fb45b3b0b0905983ca6`.
- `futility-trace-selection.json`:
  `6af1828ca131e61e4f8636ea21aaedc6c16b2bcbebe792492f7c35f58d265d09`.
- `futility-trace-result.json`:
  `458fd4424be5db7be2a3d7b21dc06059018f3cb67b8562a1b3333d806ef9809b`.

Original executed drivers, raw per-root JSONL traces, build log, source overlay
and binary are retained. Committed [control driver](2026-09-05-futility-controls.py)
and [trace driver](2026-09-05-futility-trace.py) require the named frozen inputs
and binaries. Preserve old outputs before rerunning; these drivers use the same
artifact names. The trace driver refuses Go-source differences from `1387fd6`.
