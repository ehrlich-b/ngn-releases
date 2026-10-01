# Classical review: loss recurrence, saved data and runtime

Runtime reviewed: deployed `71399c0`, then the internal-castling repair `70eb527`.
This extends the [six-angle review](2026-09-04-classical-review-prompt.md).
The native `r0904castle` game test is frozen separately; these are diagnostics,
not new playing changes or Elo estimates.

## Internal legality: recurrence beyond the first loss

[Castling repair evidence](2026-09-04-castling-repair.md) now includes a complete
200-game replay. Of 13,181 NGN-to-move positions after opening ply six, 39 in
25 games offered a pseudo-legal but illegal castle. Fresh full-history depth3
searches selected illegal internal PV heads eight times in seven games. All eight
legal emitted bestmoves match the recorded moves. There are two wins, three draws
and two losses among those games: this is recurrence and fallback-consistent
observational evidence, not seven losses explained or original timed-state replay.

The root, qsearch and legal-move availability consumers shared the same incomplete
make/final-check assumption. Both root drivers also returned negative infinity
when a nonempty pseudo-legal list contained no legal move. Both defects have
regressions that fail on the old runtime. Correctness tests, race tests and the
independent legal-move/internal-choice oracle pass on `70eb527`.

## Qsearch caches a bound from an inadequate capture budget — reproduced

The six-ply capture cap changes the work a qsearch invocation can perform, but
its TT lower-bound stores all use depth zero. A position reached after five
quiescence plies can therefore cache a bound after only one further capture,
which a new qsearch with six remaining plies treats as sufficient coverage.

A real search on the standard legal Kiwipete FEN, window `[−INFINITY,100]`,
absolute ply5, produces:

```text
qDepth5: result100; root TT move e2a6, score100, depth0, LowerBound
warm qDepth0: result100, searched nodes0, TT cutoffs1
cold qDepth0: result51
```

No fabricated TT entry is involved. Both cold searches reset TT and correction
history; the warm call differs only by the preceding real qDepth5 search. Caller
state is preserved after each call. Removing only the six-ply cap in a diagnostic
Go overlay gives **51 for all three searches**, eliminating the contradicted
root bound. This supports the capture-budget mechanism directly, beyond merely
observing a warm/cold search difference. The overlay is not a shipped fix.

Counter 3.8 has no such six-capture cap and does not store qsearch-generated
bounds (it can read main-search entries). These are two specific ways it avoids
this interaction. Retaining a cap would require a coherent remaining-depth/TT
policy; simply copying a TT-store optimization from an uncapped search does not
transfer that prerequisite. Removing the cap also requires checking work, stop
behavior and playing strength. Quiet promotions remain a separate missing move
class; this proof does not justify bundling them automatically.

Fixed-depth diagnostics illustrate why implementation choice still needs games.
Removing the cap changes canonical node counts by +53.4% (Kiwipete d12), +23.5%
(middlegame d12), and approximately zero (rook ending d16). An alternative that
retains the cap and stores q bounds only at qDepth0 also removes the reproduced
mismatch, with node changes +26.8%, +52.5%, and −1.3%. Best moves stay the same
on these three roots, but scores differ. Neither is established as cheaper or
stronger by this tiny diagnostic, and neither is in the playing release.
Raw results: `qcap-diagnostic-trees.json`; overlay control:
`qbudget-exact-fullbudget.txt`. The uncapped overlay passes the full short/race
suite; the full-budget-store overlay passes the full short suite. These checks
validate the prototypes, not their playing strength.

This follows the castling repair in priority. Raw exploratory and focused probes:
`output/recovery-2026-09-04/qbudget*`, including the current/no-cap control pair.
Both are reproducible Go overlays with production source untouched.

## Saved training data: measured limitations, not a universal failure explanation

Audited all rows in the five local `output/texel*.txt` files. For accepted files,
used the **actual** `LoadTexelSamples` and `SplitTrainTest(..., .2, 2800)` functions,
and compared the actual `TexelSample.Board` values across partitions. The loader
dedupes full position hashes, but the optimizer receives only the board: side,
castling and EP differences can survive deduplication while producing identical
model inputs. This is narrower than, and additional to, related-game leakage.

| Saved file | Rows | Full-position duplicates dropped | Held-out boards also in training | Current quiet-filter acceptance |
|---|---:|---:|---:|---:|
| texel_selfplay.txt | 30,716 | 317 | 31/6,079 (0.51%) | 1,180/2,000 (59.0%) |
| texel_selfplay_big.txt | 191,154 | 2,222 | 217/37,786 (0.57%) | 1,142/2,000 (57.1%) |
| texel_general.txt | 119,416 | 2,023 | 54/23,478 (0.23%) | 1,403/2,000 (70.2%) |
| texel_distilled.txt | 30,716 | 317 | 42/6,079 (0.69%) | 1,224/2,000 (61.2%) |
| texel_distilled_big.txt | 32,569 | n/a | Loader rejects | Not measured |

The distilled-big file ends with an incomplete FEN at line 32,569. The current
loader correctly rejects the file; this is an incomplete saved artifact, not
silent acceptance of corrupt training rows. The earlier four files load.

The filter sample is a deterministic 2,000-row reservoir per file, seed 2800.
All sampled rows have legal king-check orientation: none checks the side that
just moved, and none starts with the side to move in check. No sampled quiet
classification changes when its clock is normalized to zero, and no caller
position changes. The current filter rejects 30–43% of these old rows; changed
weights, old generation/filter policy, and conservative horizon rejection all
limit attribution. This is **not** a measured old-filter bug rate.

All rows retain fullmove number 1 and contain no game identifier. Exact original
game-disjoint validation cannot be certified from this flat schema alone.
Duplicate full-position rows also sometimes have different labels (11, 110, 3,
and 255 rows respectively); first occurrence wins under the current loader.
Different outcomes for one position are expected in game data, but input order
then chooses which observation survives. A future corpus should retain source
game/opening identity, split by those groups, and explicitly decide how to weight
repeated positions/labels. The observed model-input overlap is modest and cannot
alone establish the cause of historical Elo regressions.

These files are **not** the previously recorded million/three-million-position
external corpora. No claim about those larger corpora follows from this audit.
Raw JSON, file SHA-256s and standalone Go probe: `output/recovery-2026-09-04/corpus-*`
and `corpus_audit.go.txt`. Copy the latter to `/private/tmp/ngn-corpus-audit.go`
and run it from the repository root, passing the five files in table order.
The audit's failed initial attempt exposed the truncated fifth file; the final
report records that failure and preserves results for the accepted files.

## Another packing invariant: TT ages outgrow their stored field

`engine/cache.go` packs age with the **9-bit** `AGE_MASK`, while `AdvanceAge`
still wraps at **10 bits**. After 512 search generations, a newly stored current
entry unpacks with age zero but the cache's current age is 512. A same-generation
shallow colliding entry therefore incorrectly qualifies for stale replacement.

An executable probe allocates a 1 MB table, advances its age, stores a depth20
entry, then a colliding depth1 entry at the same generation:

```text
advances=0     deep survives=true
advances=511   deep survives=true
advances=512   deep survives=false
advances=513   deep survives=false
advances=1023  deep survives=false
advances=1024  deep survives=true
```

This is a reproduced replacement-contract defect, not a corrupt returned hash or
score. T19 reduced the packed age width for ttPv; its historical record says aging
is unchanged in practice, but the wrap constant was not adjusted. Counter's
corresponding table increments and stores the same 11-bit age width.

Priority is lower than castling: the UCI implementation recreates the TT on
`ucinewgame`, and ordinary match games do not reach 512 root searches per engine.
No incidence in the completed gauntlet is established. It is **not bundled** into
the frozen castling candidate. Probe: `output/recovery-2026-09-04/tt-age-probe.*`.

## Runtime comparison: where the sampled CPU goes

Profiled NGN `70eb527` and pinned classical Counter 3.8 (`b172b99`) locally with
Go1.26.2/Apple M4, Hash64/Threads1. Each driver performs four passes over the three
canonical FENs, requesting 2M nodes per search, resetting TT/history between
searches. NGN's process-lifetime eval/pawn caches remain warm across repeats;
Counter constructs a fresh evaluator. Startup/reset/GC are inside the profile.
Therefore these are hotspot profiles, **not an equivalent-work speed benchmark**.
The earlier interleaved fresh-process UCI throughput comparison remains separate.

- NGN: 14.7% flat CPU in `selectNextMove`, 11.8% in the TT key load, 6.0% in
  initial move scoring. Cached evaluation accounts for 20.0% cumulatively.
- Counter: 32.7% cumulative in its classical evaluator; full-list sorting plus
  move-to-top account for 13.3% flat, and TT reads 5.8% flat.
- Both have full-list move generation and direct-mapped TT. Counter even uses
  atomic entry gates for SMP. Thus a staged picker or clustered TT is not a
  demonstrated prerequisite to its strength; nor does NGN spend all its time
  doing handcrafted evaluation.

The profile suggests checking move-picking work and TT access cost before another
minor evaluation micro-optimization. Removing all sampled cost is impossible;
these percentages are not prospective NPS or Elo gains. The repeated-root setup
and cross-engine node semantics limit extrapolation. Raw profiles, summaries and
both standalone drivers are `output/recovery-2026-09-04/{castle,counter}-*` and
`{classical,counter_classical}_profile.go.txt`.

## Portable reproductions

The TT reproducer and corpus audit are committed alongside this report, so the
findings do not depend on retaining ignored output files:

```sh
python3 experiments/2026-09-04-tt-review-probes.py
cp experiments/2026-09-04-corpus-audit.go.txt /tmp/ngn-corpus-audit.go
go run /tmp/ngn-corpus-audit.go output/texel_selfplay.txt output/texel_selfplay_big.txt output/texel_general.txt output/texel_distilled.txt output/texel_distilled_big.txt
```

The first command intentionally exits nonzero on `70eb527` by reproducing both
TT defects. The corpus command requires the local input files; their original
checksums are in the saved JSON. Neither command edits production source.
