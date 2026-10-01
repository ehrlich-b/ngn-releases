# 2026-08-02 T9b "true multicut" — TRIAGED: not a 3-liner, and the return-beta form is UNSOUND here

**Zero box cost. No code changed.** Assessed from source because T9b is the last untested mechanism carrying
a measured prior (+5.8 [S]) and the queue is nearly exhausted.

## What NGN already has

The singular block (`search.go:1842-1884`) already implements **multicut-LITE (W-SE2)**: when the singular
verification **fails high** — i.e. some move other than the ttMove also reached `singularBeta` — and
`ttEval >= beta`, NGN **shaves a ply** (`nextDepth--`) instead of searching full depth. The code comment
states the position explicitly:

> "this is very likely a multi-good-move cut-node. Shave a ply instead of searching full depth
> (multicut-LITE; **the return-beta multicut stays rejected, the A3 lesson**)."

So the lane is not empty. What T9b proposes is the stronger form: **return `beta` outright** rather than
shave a ply.

## Why the return-beta form is unsound AT NGN'S GEOMETRY — the arithmetic

```go
singularBeta  := int(ttEval) - (SINGULAR_MARGIN*depth)/32   // = ttEval - 2*depth at the default 64
singularDepth := (depth - 1) / 2                            // half-depth verification
```

A verification fail-high proves **`score >= singularBeta`**, and `singularBeta = ttEval - 2*depth`.

**It does not prove `score >= beta`.** At depth 12 with `ttEval = beta + 10`, `singularBeta = beta - 14`, so
the fail-high establishes only `score >= beta - 14`. **Returning `beta` there asserts a cutoff that was never
proven — unsound by up to `2*depth` centipawns**, and the error grows with depth, i.e. precisely at the
high-leverage nodes.

That is the concrete content of the pre-reset finding recorded as *"NGN's singular geometry depth-4/flat-64
differs from what the depth/2 or 3*d multicut assumes"*. It is a **soundness** mismatch, not a tuning one,
which is why the LITE form (a soundness-preserving depth reduction) is the version that survived.

**Note this is exactly the T18b failure shape.** T18b turned a *speculative* probcut guess into a durable TT
assertion and lost 22.5 Elo. Return-beta multicut would turn a bound proven at `singularBeta` into a claimed
cutoff at `beta` — again upgrading a weaker fact into a stronger assertion. **T18b's soundness question
answers this candidate before any box time: it persists/propagates an unproven bound.**

## What a SOUND multicut would actually require

Not a tweak to the existing verification. It needs its own search:

- a reduced-depth search **at `beta`** (not at `singularBeta`),
- **enumerating moves and counting fail-highs** (standard: >= 3 of the first ~6),
- returning `beta` only once that count is met.

NGN's verification is a **single null-window search that does not enumerate how many moves failed high** — it
learns only "at least one non-ttMove reached `singularBeta`". So this is a **new mechanism with real added
cost** (extra searches at every qualifying node), not the "free 3-liner inside the existing singular search"
the 2026-06-09 note called it.

## Verdict

**T9b is RECLASSIFIED, not queued as written.** The +5.8 [S] prior attaches to a mechanism NGN does not have
and cannot cheaply acquire, while the conservative form of the same idea is **already shipped** and captures
part of the value.

- **The "return beta" one-liner is DEAD** — unsound at this geometry by up to `2*depth` cp, and it fails the
  T18b soundness question. Do not run it. This closes the "free 3-liner" framing for good.
- **A properly-constructed multicut** (own search at `beta`, fail-high counting) stays open but is a
  **new mechanism**, ranked **LOW**: it adds search cost at every qualifying node, it overlaps with W-SE2
  which already fires there, and the singular family's one games-tested member this campaign (T9a double
  extension) lost 5.2.
- **Note the reopen constraint carried from T9a:** its record ranked a margin retry via SPSA over another
  hand-picked constant. Any multicut work should likewise avoid a second hand-picked count/margin.

**Queue effect: the last "cheap untested mechanism with a prior" turns out not to be cheap.** After T12/T19/
T15 the ranked queue is T8 SPSA and nothing else of substance — that is the honest position, recorded so it
is not rediscovered at 5am.

```yaml
id: 2026-08-02-t9b-multicut-triage
date: 2026-08-02
change_class: triage (no code changed)
result: >
  Return-beta multicut is unsound at NGN's singular geometry: a verification fail-high proves only
  score >= singularBeta = ttEval - 2*depth, not score >= beta, so returning beta over-claims by up to 2*depth
  cp and fails the T18b soundness question. NGN already ships the sound conservative form (W-SE2 negative
  extension). A correct multicut needs its own search at beta with fail-high counting -- a new mechanism, not
  a 3-liner.
box_cost: zero
disposition: >
  "Free 3-liner" framing CLOSED. Properly-constructed multicut stays open but ranked LOW (added search cost,
  overlaps W-SE2, and the singular family's only games-tested member this campaign lost 5.2).
```
