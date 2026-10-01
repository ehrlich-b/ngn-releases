# 08 — Time Management

*Anchored to `engine/time.go` and the root-loop hooks in `engine/search.go` @ `87714a1`.*

---

## Purpose

Decide how long to think. Two failure modes to avoid: **flagging** (losing on time — catastrophic, an
automatic loss regardless of position) and **under-thinking** (banking time you never spend). The design
is asymmetric: flagging must be *impossible*; under-thinking is merely suboptimal. So a hard ceiling and
an emergency reserve are inviolable, while the soft target flexes.

---

## Canonical

- **Soft limit** — the target before *starting* a new iterative-deepening iteration. Don't begin an
  iteration you can't expect to finish within the soft budget. Spending ~the increment each move keeps the
  bank flat.
- **Hard limit** — never exceeded mid-iteration; the anti-flag ceiling.
- **Stability** — when the best move has been stable for several iterations, the decision is settled: cut
  the soft budget and bank time for genuinely hard moves. When the best move is *changing* or the aspiration
  window keeps breaking, the position is sharp — do **not** cut the search short.
- **The move must always be real** — finish at least one full iteration so you never play a depth-0 blunder,
  and never commit a half-searched move from an interrupted iteration (that coupling lives in the search, doc
  01 INV-A4).

---

## NGN

**Modes** (`TimeControl`, time.go:12): `FixedDepth`, `FixedTime`, `TimePerMove`, `Tournament`, `Infinite`.
Only `Tournament` has the soft/hard machinery; the simpler modes use a single capped `allocatedTime`.

**Tournament budgets** (`computeTournamentLimits`, :149):
```
usable = baseTime − emergencyTime(100ms) − moveOverhead(50ms)
soft   = usable/movesToGo + 0.8·increment
hard   = min(soft·4, usable·0.3)            // with: if hard < soft { hard = soft }
```
`movesToGo` falls back to `estimateMovesRemaining` (clamped [10,40], :176). This replaced an older
allocation that stacked phase × complexity × score multipliers on an even share and could plan several× a
sustainable slice — bleeding the bank faster than the increment refilled.

**Stop decision** (`shouldStopTournamentSearch`, :296), checked in order:
1. `elapsed >= hardTime` → stop (the ceiling).
2. `baseTime − elapsed <= emergencyTime` → stop (emergency reserve).
3. `depth < 1` → don't stop (always finish iteration 1).
4. Soft check with stability shrink and next-iteration projection:
   ```
   soft' = soft/2   if stableIters>=6
           soft·0.7 if stableIters>=3
           soft     otherwise
   stop if  elapsed + 2·lastIterationTime > soft'      // (or elapsed>=soft' before any timing exists)
   ```

**Rate limiting** (`ShouldStopSearch`, :265): the expensive `time.Now()` runs only every 1024 calls (or once
`shouldStop` latches), so the search can poll it per node cheaply. Fine at NGN's NPS.

**Stability signal** (the root feeds it):
- `NewIteration()` (:240) at the top of each ID depth records `lastIterationTime`.
- `ReportCompletedIteration(bestMoveChanged, windowHeldFirstTry)` (:257): resets `stableIters` to 0 on a
  best-move change or a window break (a fail-high/low re-search, `attempts>1`), else increments. Called by the
  root only on a fully-completed iteration (:738) — `bestMoveChanged` computed *before* `info.BestMove` is
  overwritten, `windowHeldFirstTry == (attempts==1)`.

---

## Invariants

- **`[INV-T1]` The hard ceiling and emergency reserve are inviolable `[HOLDS]`** *(audited 2026-05-31, 87714a1)* —
  the hard-time and emergency checks (:299, :303) run *first* and read only fixed budgets; the stability shrink
  touches **only** `soft` (:317-323). So stability scaling can only ever play *faster*, never later — it cannot
  increase flag risk. Verified by `TestStabilityScaledSoftStop` and the fix-review audit.
- **`[INV-T2]` Iteration 1 always completes `[HOLDS]`** — the `depth<1 ⇒ don't stop` floor (:309) guarantees a
  real move even under extreme time pressure; combined with INV-A4 (discard interrupted iterations) the engine
  never plays a depth-0 or half-searched move.
- **`[INV-T3]` Stability cannot falsely fire on move 1 / forced moves `[HOLDS]`** *(audited 2026-05-31)* — the
  shrink needs `stableIters>=3`, i.e. ≥3 completed stable iterations (depth ≥4); on iteration 1
  `info.BestMove==EmptyMove ⇒ bestMoveChanged=true ⇒ stableIters stays 0`. A forced move self-accelerates
  (intended: it's fully decided). A mid-flight stop doesn't call `ReportCompletedIteration`, so `stableIters` is
  unchanged (no false information). Verified by the fix-review audit.

---

## Divergences & status

- **`[DIV-T1]` The "0.3 cap mathematically prevents flagging" comment is overstated `[ACCEPTED]` (comment-only)** —
  when the increment is large relative to the bank (e.g. 1s bank + 5s inc), the `if hard < soft { hard = soft }`
  override (:166) lets `hard` exceed `0.3·usable`, so the 0.3 cap is *not* the binding anti-flag guarantee in
  that regime. **The real backstop is the per-node emergency guard** (:303), which caps real spend at `bank −
  100ms` regardless of `hard`. This regime never occurs in real chess TCs (verified empirically: +168 ELO and 0
  flag-outs at 2+0; 0 flag-outs in the 15+0.15 CCRL run). **Action:** fix the comment to credit the emergency
  guard, not the 0.3 cap. *(finding: fix-review agent af64cc38, 2026-05-31.)*

---

## The real-clock unlock (context)

Tournament `-tc` mode (sends `wtime`/`btime`, never an external `stop`) is what makes the full Blunder anchor
ladder viable for absolute CCRL calibration, because movetime-mode `stop`-noncompliance doesn't apply. This is
why time-management correctness is on the critical path: finishing it lets the gauntlet drop its
"+30-80 optimistic, movetime proxy" bias caveat (see `CLAUDE.md` / `TODO.md`).

---

## Joints this opens (doc 09)

- **J13** — time management × the clock-interrupt discard (INV-A4): the time manager decides *when* to stop;
  the search decides *what to keep* when stopped. Both are required for the anti-blunder guarantee; neither
  suffices alone. This pairing was the fix for the catastrophic time-pressure blunders.
- **J14** — stability signal × aspiration windows: a window break (`attempts>1`) is read as volatility and
  resets stability. The aspiration re-search and the time manager share the `attempts` counter.
