# 09 — Invariants, Joints & Finding Intake

*Anchored to the whole search @ `87714a1`. This is the doc the rest of the spec points into.*

This doc does three jobs:
1. **Consolidates the invariants** from docs 01-08 into one status table — the fix queue at a glance.
2. **Catalogues the joints** — the cross-technique interactions where bugs actually live in this engine.
3. **Is the intake framework** for bug-hunt findings: every reported bug maps to an invariant or a
   joint and gets a status. The four audits from 2026-05-31 are recorded here as worked examples.

---

## Why "joints"

The bugs that have actually hurt this engine were not "technique X is wrong." They were *technique X
composes wrongly with technique Y* — each locally correct, broken only at the seam:

- NMP that **silently never fired** because PVS collapsed its window to a null window (the C3 bug). NMP
  was correct; *PVS × the NMP gate* was the bug.
- IID/singular re-search that **returned a bogus draw** because the node's own hash was still on the
  repetition stack. The re-search was correct; *re-entrancy × the RepStack* was the bug.
- Extensions that are **computed but never applied** off the PV because the LMR reduced-scout searches
  from the wrong base depth. Extensions are correct; LMR is correct; *extensions × LMR* is the bug.

A single-technique audit will give all three a clean bill. **You only catch these by auditing the
seam.** That is what the joint catalogue is for.

---

## How to receive a finding (the triage protocol)

When an audit/agent reports a bug, do **not** apply the fix on faith and do **not** drop it on the floor.
Run it through this protocol (the codification of the "verify-agent-claims" and "keep-correctness-fixes-
on-correctness-grounds" rules):

1. **Verify the mechanism against live code.** Open the cited `file:line`, confirm the code does what the
   finding says. An agent claim is a *hypothesis* until the lead reads the lines. (Two of this run's findings
   were corroborated by two independent agents — that raises confidence but does not replace reading.)
2. **Map it to an invariant or joint.** Which numbered `[INV-*]` / `[J*]` does it concern? If none exists,
   the spec is incomplete — add one. The map tells you the *blast radius* and the other techniques in the seam.
3. **Set a status** — `[HOLDS]` (false alarm / already guarded), `[VIOLATED]` (real, with severity), or
   `[ACCEPTED]` (real but deliberate/inherent — e.g. GHI). Record the finding ref + date.
4. **Decide keep / fix / accept** by the keep-correctness-fixes rule:
   - A **provable correctness fix** (mechanism-verified + no measurable regression in a short run) is *kept on
     correctness grounds*. Do **not** gate it on a +10-ELO swing you can't resolve.
   - **Only reject** a change that is *both* (a) non-correctness (speculative/tuning) *and* (b) shows a concrete
     cost. (Example: C5 was reverted correctly — not a correctness fix, 2.1× node cost.)
5. **Apply one at a time, gated.** Per fix: depth-10 Kiwipete node/seldepth vs a freshly-built clean HEAD +
   `make smoke-tactical` + `go test ./engine/`. Never batch correctness fixes (you lose attribution).

It is fine to **record a finding and not fix it this session** — the durable artifact is this ledger, not a
same-day patch. A finding parked here with a status is *not* dropped on the floor.

---

## Master invariant table

| ID | Invariant | Status | Where |
|---|---|---|---|
| INV-A1 | Negamax sign discipline (every child negated) | `[HOLDS]` | 01 |
| INV-A2 | Make/unmake + lastMovePlayed balanced on every path | `[HOLDS]` | 01 |
| INV-A3 | RepStack balanced per node (push guarded, pop unconditional) | `[HOLDS]` latent | 01, J3 |
| INV-A4 | Clock-interrupted iteration discarded entirely | `[HOLDS]` | 01, J13 |
| INV-A5/EXT3 | Every budget-using entry point sets `RootDepth` | **`[VIOLATED]`** ML | 01, 05, J8 |
| INV-A6 | `ply < MaximumDepth` always (array-overflow guard) | `[HOLDS]` | 01 |
| INV-A7 | Single-threaded heuristic state (no race) | `[HOLDS]` | 01 |
| INV-TT1 | Non-exact bound never returned as exact | `[HOLDS]` | 02 |
| INV-TT2 | TT cutoffs only off-PV, only at sufficient depth | `[HOLDS]` | 02 |
| INV-TT3/M2 | Mate scores round-trip through ply adjustment | `[HOLDS]` | 02, 07, J5 |
| INV-TT4 | TT move never trusted blindly (legality-filtered) | `[HOLDS]` | 02, J6 |
| INV-TT5 | Singular verification root neither cuts nor stores | `[HOLDS]` | 02, 05 |
| INV-MO1..5 | Ordering: TT-first, non-overlapping bands, quiet-only killers, bounds-safe, pure | `[HOLDS]` | 03 |
| INV-NMP1 | NMP fires at every cut node (C3 fix) | `[HOLDS]` | 04 |
| INV-NMP2 | NMP zugzwang-guarded (non-pawn material) | `[HOLDS]` | 04 |
| INV-NMP3 | No two null moves in a row | `[HOLDS]` | 04 |
| INV-LMR1 | Reduced depth stays ≥ 1 (never falls into qsearch) | `[HOLDS]` | 04 |
| INV-LMR2 | Captures/checking moves not reduced | `[HOLDS]` | 04, J1 |
| INV-SEE1 | SEE-prune sign + king/promo exclusions correct | `[HOLDS]` | 04 |
| INV-PC1 | Probcut sign + return-bound correct | `[HOLDS]` | 04 |
| INV-EXT1 | Per-node net extension ≤ +1 | `[HOLDS]` | 05 |
| INV-EXT2 | Cumulative path extension ≤ 24 (ID path) | `[HOLDS]` | 05 |
| INV-EXT4 | Singular verification doesn't corrupt state | `[HOLDS]` latent | 05, J3 |
| INV-Q1 | No stand-pat while in check | `[HOLDS]` | 06, J11 |
| INV-Q2 | Delta/SEE pruning sign-correct + promo-aware | `[HOLDS]` | 06 |
| INV-Q5 | qsearch TT stores can't poison main search | `[HOLDS]` | 06 |
| INV-M1/M3 | Mate distance + mate-distance pruning use `ply` | `[HOLDS]` | 07 |
| INV-D1 | Draw scores never stored in the TT | `[HOLDS]` | 07, J4 |
| INV-D2 | Stalemate scores 0 (main search) | `[HOLDS]` | 07 |
| INV-D3 | Game map and search RepStack disjoint | `[HOLDS]` | 07 |
| INV-T1 | Hard ceiling + emergency reserve inviolable | `[HOLDS]` | 08 |
| INV-T2 | Iteration 1 always completes | `[HOLDS]` | 08 |
| INV-T3 | Stability can't falsely fire on move 1/forced | `[HOLDS]` | 08 |
| **the LMR/ext seam** | Extended depth is the depth searched (off-PV, non-first moves) | **`[VIOLATED]`** HIGH | 04, 05, J1 |
| DIV-FUT1 | Futility excludes promotions | **`[VIOLATED]`** MED | 04, J10 |
| DIV-LMP1 | LMP/history-prune exclude promotions | **`[VIOLATED]`** LOW | 04, J10 |
| DIV-RFP1 | RFP doesn't fire on a width-1 PV node | **`[VIOLATED]`** LOW | 04, J9 |
| DIV-Q1 | Stalemate scores 0 in qsearch (all qDepth) | **`[VIOLATED]`** LOW-MED | 06, J12 |
| DIV-Q2 | seldepth not double-counted | **`[VIOLATED]`** cosmetic | 06 |

---

## Joint catalogue

Each joint = the seam between ≥2 techniques, the invariant that must hold across it, and its status.

### J1 — Extensions × LMR reduced-scout base `[VIOLATED]` HIGH
The reduced scout searches from `depth-1` (`reducedDepth := depth-1-reduction`, search.go:1560), not the
extension-extended `nextDepth`. LMR excludes captures/checks (`reduction==0`), and the re-searches at :1575/
:1580 only fire for `reduction>0` or PV nodes — so **check/recapture extensions are computed, counted, and
never applied off-PV for non-first moves.** Full treatment + fix (`reducedDepth := nextDepth - reduction`) in
**doc 05**. *Confirmed by two independent audits + lead read.* **The worked example of "it's both together."**

### J2 — TT-refined static eval ("S7") × the NMP/RFP/futility gates `[HOLDS]` (by design)
`staticEval` is overwritten with the TT score when the stored bound brackets it (search.go:1121), and that
value then decides whether NMP/RFP/futility fire. So a *shallower* TT search can flip a pruning decision. This
is intentional (a search-refined estimate is sharper than a fresh eval) and guarded (skips mate scores and the
singular root). Watch it when changing the eval or the TT — it is a real coupling, not a bug.

### J3 — Re-entrancy (IID, singular) × the RepStack `[HOLDS]`, latent panic-unsafety
IID (:1064) and singular verification (:1438) re-enter `alphaBetaPV` on the *same* position. The node already
pushed its hash, so without hiding it the re-entry self-matches and returns a bogus draw (0) — the historical
bug. Fixed by `RepStackLen--/++` around both. **Latent:** the straddle is panic-unsafe (if the inner call
panics, the `++` is skipped) and the conditional push vs unconditional pop (INV-A3) is asymmetric — both are
*contained* (a corrupted `info` dies with the search; the 512 cap is unreachable under the ply-100 guard).
Defensive fix: convert both to `defer`-balanced form. *(TT/ordering/state audit.)*

### J4 — TT × path-dependent draws (graph-history interaction) `[ACCEPTED]`
A repetition draw is path-relative; the TT is position-keyed. NGN never stores the draw at the repetition node
(INV-D1), but a parent's score can still be path-tainted. Inherent; documented in **doc 07**. Not a fresh bug.

### J5 — Mate scores × TT ply-adjust × aspiration windows `[HOLDS]`
Mate scores are stored node-relative and converted on store/read (INV-TT3/M2). Mate-distance pruning collapses
the window near mate; aspiration widening handles the resulting fail-high/low. The one rough edge is J9 (RFP's
width-1 proxy). Otherwise sound.

### J6 — TT move × move-generation legality (collision safety) `[HOLDS]`
A retrieved `ttMove` is only ever an ordering hint; it is played only if it matches a generated pseudo-legal
move and passes the legality filter (INV-TT4). A hash collision can't corrupt the board. *(TT/ordering/state
audit.)*

### J7 — `lastMovePlayed` global × null move × recursion `[HOLDS]`
The counter-move key is a *global* threaded imperatively (`SetLastMovePlayed`/`savedLast`) through every
recursion and **zeroed across a null move** (so children don't key counters on our own prior move). Saved/
restored on every branch incl. null/probcut/singular (INV-A2). The one imperatively-threaded global — fragile,
but correct, and the blast radius is one search (fresh `info` per search). Would become a data race under
multi-threading (INV-A7).

### J8 — Extension budget × `RootDepth` entry-point setup `[VIOLATED]` MEDIUM-LOW
The budget clamp reads `info.RootDepth`. The ID entry sets it; `searchFixedUnsafe`/`alphaBeta` don't, leaving it
0 → fixed-depth searches past ~depth 24 are silently truncated. The clamp math itself is *provably correct on the
ID path* (INV-EXT2). The seam is between the recursion's assumption and the entry point's setup. Fix in **doc 05
INV-EXT3**. *Confirmed by two audits.*

### J9 — RFP × mate-distance window collapse `[VIOLATED]` LOW
Mate-distance pruning can collapse a genuine PV node's window to width 1, and RFP's `beta-alpha==1` proxy then
fires on the PV. Low reachability. Fix: gate RFP on `!isPV` (doc 04 DIV-RFP1).

### J10 — Forward pruning × promotions `[VIOLATED]` (futility MED, LMP/hist LOW)
Futility (:1301), LMP (:1309), history-prune (:1348) skip `!IsCapture()` quiets without excluding
`PromoType()==NoType`, so a quiet promotion (a ~+800 swing the static eval can't see) can be pruned. Futility is
the real hole (prunes from move 1); LMP/hist are near-unreachable (promos sort first). Fix: add the promo guard
(doc 04 DIV-FUT1/LMP1).

### J11 — qsearch stand-pat × in-check `[HOLDS]`
You can't pass out of check, so the in-check qsearch branch never stands pat and searches all evasions (INV-Q1).
The eval × check-detection × qsearch seam, handled correctly.

### J12 — qsearch terminal scoring × qDepth `[VIOLATED]` LOW-MED
The not-in-check stalemate check runs only at `qDepth==0`; deeper, a stalemate returns the material eval, not 0
(doc 06 DIV-Q1). The seam is between stalemate detection and the qDepth recursion. Also note the benign half:
the qsearch *leaf eval* feeds the parent's `improving`/S7 reuse — correct, just worth knowing.

### J13 — Time manager × clock-interrupt discard `[HOLDS]`
The time manager decides *when* to stop (doc 08); the search decides *what to keep* when stopped (INV-A4 —
discard the interrupted iteration). **Neither alone prevents the catastrophic time-pressure blunder; together
they do.** This pairing *was* the fix. The canonical example of a joint that is a feature, not a bug — and of
why you must not "simplify" either half in isolation.

### J14 — Stability signal × aspiration windows `[HOLDS]`
A window break (`attempts>1`) is read as volatility and resets `stableIters` (doc 08). The aspiration re-search
loop and the time manager share the `attempts` counter — change one and re-check the other.

### J15 — `improving` heuristic × in-check × StaticEvalStack `[HOLDS]`
`staticEval` is undefined in check, so it's inherited from `StaticEvalStack[ply-2]` (search.go:1109), and
`improving` compares against `[ply-2]` (:1133). This keeps the improving chain sane across checking plies and
feeds LMR (`!improving ⇒ reduce more`) and futility. A subtle three-way seam (eval × check × the ply-2 stack);
verified consistent, no off-by-one.

---

## Finding ledger — 2026-05-31 background audits

Four background agents audited the search the day this spec was written. All findings recorded; verified ones
mapped to the catalogue. **Per the keep-correctness-fixes rule, these are parked with status — fixing is a
separate, gated step, and it is acceptable that not all are fixed the same session.**

### Agent: pruning & reductions (`af2f023f`)
| # | Finding | Maps to | Status |
|---|---|---|---|
| F1 | Extensions discarded off-PV via `depth-1` scout base | J1 | `[VIOLATED]` HIGH — lead-verified |
| F2 | Futility prunes quiet promotions (no PromoType guard) | J10/DIV-FUT1 | `[VIOLATED]` MED — verified |
| F3 | LMP/history-prune also lack the promo guard | J10/DIV-LMP1 | `[VIOLATED]` LOW — verified (near-unreachable) |
| F4 | RFP gated on `beta-alpha==1` not `!isPV` | J9/DIV-RFP1 | `[VIOLATED]` LOW — verified (low trigger) |
| — | Clean bills: NMP gate (C3), LMR formula/cap, probcut, singular, SEE signs, aspiration classify | — | corroborates INV-NMP*, LMR1, PC1, SEE1 |

### Agent: extensions, qsearch, mate/draw (`a4f0b130`)
| # | Finding | Maps to | Status |
|---|---|---|---|
| F1 | Check/recapture extensions dead off-PV (non-first moves) | J1 | `[VIOLATED]` — **independent confirm of F1 above** |
| F2 | `SearchFixed`/`alphaBeta` leave `RootDepth=0` → depth cap | J8/INV-EXT3 | `[VIOLATED]` ML — **independent confirm** |
| F3 | qsearch stalemate scored as eval below `qDepth==0` | J12/DIV-Q1 | `[VIOLATED]` LOW-MED — verified |
| F4 | seldepth double-counted (`ply+qDepth`) | DIV-Q2 | `[VIOLATED]` cosmetic — verified |
| — | Clean bills: mate encoding, mate-distance, main-search mate/stalemate, budget math (ID), per-node guardrail, singular, IID, qsearch in-check, delta, SEE, qsearch TT, draw detection | — | corroborates INV-M*, EXT1/2, Q1/2/5, D1 |

### Agent: TT, ordering, make/unmake state (`a186f283`)
| # | Finding | Maps to | Status |
|---|---|---|---|
| L1 | RepStack conditional push vs unconditional pop | INV-A3 | `[HOLDS]` latent — defensive only (unreachable) |
| L2 | IID/singular `RepStackLen` straddle panic-unsafe | J3 | `[HOLDS]` latent — defensive only (contained) |
| L3 | Singular doesn't exclude mate-scored TT entries | J5 | efficiency — add `abs(ttEval)<MATE_IN_MAX` |
| L4 | Root ordering looks up killers at wrong depth slot | DIV-MO2 | efficiency only |
| — | **No live state-corruption bug.** Clean bills: TT bounds, mate encoding, TT-move legality, make/unmake balance, repetition split, ordering bands, IID/singular re-entrancy | — | corroborates INV-TT*, MO*, A2, D3 |

### Agent: review & harden this session's fixes (`af64cc38`)
| Fix | Verdict | Maps to |
|---|---|---|
| Extension budget (cbe05db) | ROCK SOLID on ID path; **latent `SearchFixed` RootDepth bug** | INV-EXT2 holds / INV-EXT3 violated |
| C4 LMR re-search (c136b63) | ROCK SOLID — matches canonical PVS+LMR in all 9 cases | doc 01 PVS contract |
| Stability-scaled soft time (3e88982) | ROCK SOLID — hard cap untouched | INV-T1/T3 |
| Increment-aware alloc (fdbf565) | ROCK SOLID for real clocks; **comment overstates the 0.3 cap** | INV-T1 / DIV-T1 |
| C5 revert (6bcee9d) | CORRECT — dropped nothing correctness-relevant | keep/drop rule |
| — | **Biggest gap: no regression test for the extension budget.** | test-gap below |

---

## Fix queue (VIOLATED, by severity — apply one at a time, gated)

1. **[ATTEMPTED → REVERTED 2026-06-01] J1 — extension × LMR base.** Applied `reducedDepth := nextDepth - reduction`
   (search.go:1615) + dropped the tautological clamp (keeping it would re-clamp a check back to depth-1, defeating
   the fix — the doc's old "keep the dead branch" was wrong). Gated: `go test`/`smoke-tactical` clean, node check
   CONFIRMED the extension now fires (kiwipete d12 +51%, mid −21%, **end +83%**). But the fixed-time SPRT (800ms,
   [-5,0]) leaned **−59 ELO [−144,+25] at G65** and the +83% endgame-tree bloat amplifies NGN's #1 weakness →
   **reverted, not kept** (stopped early for the headline gauntlet; not a formal verdict). The long-dead off-PV
   extension is better left dead — the engine had implicitly tuned around it. Status stays `[VIOLATED]` but is an
   ACCEPTED non-fix now; re-run to a formal bound before any retry. (One-liner above is the change if revisited.)
2. **[MEDIUM] J10/FUT1 — futility prunes promotions.** Add `&& move.PromoType()==NoType` at :1301. Correctness
   fix, low-risk; keep on correctness grounds (no ELO gate). Gate: smoke-tactical + go test.
3. **[MEDIUM-LOW] J8 — `SearchFixed` RootDepth.** `info.RootDepth = depth` in `searchFixedUnsafe` (and the
   ply-0 equivalent in `alphaBeta`). Non-shipping but a real landmine for `cmd/trace_move`/deep tests. One-liner.
4. **[LOW-MED] J12/Q1 — qsearch stalemate below qDepth==0.** Return 0 on a not-in-check qnode with no legal
   move, or document as an accepted horizon limit.
5. **[LOW] J10/LMP1 — LMP/history-prune promo guard.** Add the promo guard for consistency.
6. **[LOW] J9/RFP1 — RFP gate.** `!isPV` instead of `beta-alpha==1`.
7. **[cosmetic] DIV-Q2 — seldepth.** `if ply > info.SelDepth` in qsearch.
8. **[defensive] INV-A3 / J3.** Guard the RepStack pop / make the IID+singular straddles `defer`-balanced.
9. **[comment] DIV-T1.** Credit the emergency guard, not the 0.3 cap.

### Test gaps to close
- **Extension budget:** a fixed-depth search on a forcing-check position asserting `SelDepth <= RootDepth +
  EXTENSION_BUDGET`; and a node-count identity for `SearchFixed` at depth 30 clamped-vs-disabled (also catches
  the RootDepth=0 bug, #3).
- **J1 regression:** after the fix, a position where the decisive line is a non-first checking/recapture move
  needing one extra ply, asserting it's now found (and a node/seldepth delta concentrated on checking lines).

---

## Maintenance

When you fix a `[VIOLATED]` item: flip its status here and in the source doc, add the fixing commit, and (if it
moved strength) the measurement. When you find a new joint: add a `J*` entry and link it from the technique docs.
Keep this ledger as the single place where "what's known broken and why we believe it" lives.
