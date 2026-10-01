# 2026-07-01 M5 — score adjudication in cmd/sprt (mill throughput)

Revamp mill-item M5 (`2026-07-01-method-revamp.md`). Before this, `cmd/sprt` played
every game to a terminal rule or the 200-ply cap; a game that was +900 for one side
ran all 200 plies and was then scored a `max-moves` DRAW, and the engines' `info …
score cp` lines were skipped entirely (`GetMove` only looked for `bestmove`). This
wastes wall-clock (the mill's bottleneck) and mis-scores decided games.

## Change

Two-sided cutechess/fastchess-style early adjudication, **default OFF**:

- `internal/uci/uci.go`: `GetMove` now parses `score cp`/`score mate` into
  `Engine.LastScoreCp`/`LastScoreValid` (mate → ±100000). `PlayGame` folds each
  searched ply's White-POV score into an `adjState`; `AdjConfig` (a zero-value =
  fully-disabled field on `GameConfig`) sets the thresholds. A win/resign verdict
  needs `|score| ≥ ResignScore` for `ResignPlies` consecutive plies with a
  consistent winner; a draw needs `|score| ≤ DrawScore` for `DrawPlies` plies past
  `DrawMinPlies`. A streak of ≥2 plies necessarily spans both engines, so every
  verdict is TWO-SIDED — a single side's mis-eval can't adjudicate alone. Reasons
  `adj-win` / `adj-draw`.
- `cmd/sprt/main.go`: flags `-resignscore` (0=off), `-resignplies` (4),
  `-drawscore` (0=off), `-drawplies` (8), `-drawminplies` (80 = move 40); header +
  end-of-run "Adjudicated early: N decisive, M draw" line for run-record
  transparency.

Backward compatible: all other `GameConfig{…}` callers (cmd/gauntlet, cmd/spsa,
cmd/smoke) use keyed literals and leave `Adj` zero-valued → adjudication off.

## Evidence

- Unit tests `internal/uci/uci_test.go` (5): score parsing incl. mate/lowerbound/
  no-score lines; two-sided win requires the full ply streak; win streak resets on
  a winner-sign flip; draw fires only past the ply floor and resets on a decisive
  ply; **zero-value `AdjConfig` never adjudicates** (the invariant every non-sprt
  caller depends on). All green.
- Full short suite green (`go test -short ./... -count=1`): engine + cmd/sprt +
  internal/uci `ok`.
- End-to-end A/A vs the real NGN binary (identical copies, `-nodes 15000`, 10
  games, c2): OFF → no adjudication, pentanomial `[LL0 LD0 {LW,DD}5 WD0 WW0]`, 5s.
  ON (`-drawscore 40 -drawplies 6 -drawminplies 20 -resignscore 600`) → "4
  decisive, 6 draw of 10", **same pentanomial `[0,0,5,0,0]`, 2s.** Same scored
  result, less wall-clock — the no-bias property on a deterministic sample. (The
  2.5× is a tiny-deterministic-sample artifact; honest estimate on real bullet
  games is +10-15%.)

## Enable gate (M6, box)

Adjudication stays OFF by default and MUST NOT be trusted for a verdict until a box
A/A at the real mill TC (10+0.1) with adjudication ON confirms base-vs-base still
centers on 0 with no W/L skew and zero spurious flips. Blocked while the LAN box is
offline. Suggested first enable values (conservative, standard): `-resignscore 900
-resignplies 5 -drawscore 10 -drawplies 10 -drawminplies 80`. Once the A/A is
clean, bake the flags into the box/cloud launch commands.
