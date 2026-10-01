# 2026-06-29 TT age-refresh on read (codex #6) — run record

## What & why

The FMC-ordering campaign falsified all four history-formula candidates (see
`2026-06-29-fmc-ordering.md`). The one unscreened, data-flagged, *non-history* ordering lever is
TT-move retention: ttlist (hash-move availability at move-loop nodes) stays ~24–26% in the
middlegame at both 400K and 1.6M nodes. NGN's `Cache.Get` (cache.go) does NOT refresh a probed
entry's age, so the collision-replacement rule in `Set` (`oldAge != age || depth >= oldDepth`)
evicts recently-USED-but-old-generation entries as readily as cold ones. This is a *missing
standard technique*, not a tuned-out local optimum — NGN's only historical Elo source has been
adding/fixing correct machinery.

**Change (cache.go `Get`):** on a TT hit, if the entry's generation != current, re-store it with the
current age. Protects hot entries from age-based eviction → higher hash-move availability across moves.

## Why the FMC proxy can't screen this (→ games)

`AdvanceAge()` runs ONCE per search (search.go:622), so `c.age` is constant within a search ⇒ the
refresh branch never fires within a single search. Proven: **nodecheck is byte-identical to HEAD**
(270446 / 81006 / 840033) — the change is provably inert within a search; its ONLY effect is
cross-move retention. The FMC proxy (one cold search) is therefore blind to it. A fixed-nodes GAME,
however, DOES exercise it (the TT persists across the ~60 per-game searches, ages 1..N), so a
fixed-nodes SPRT is a valid fast filter; real-clock short-TC is the verdict.

Correctness: `go test -short ./engine` green (incl. the back-to-back determinism test
search_test.go:274 that c4 broke); `go test -short -race ./engine` green.

## Run record

- base: HEAD `18545ce` (clean) — `build/win/ngn_base.exe` sha256 `892f5d3a86425d464fee4329c1704c1d761c7d4a3214da3c32e40fa38aa0a68c`
- cand: HEAD + cache.go age-refresh patch — `build/win/ngn_ttage.exe` sha256 `b4608abcce5cf3d4dc48188733aa3b2e79c4bf6fc779f417f87013623ba9b05c`
- toolchain: go1.26.2, GOOS=windows GOARCH=amd64 GOAMD64=v3
- machine: gaming box (AMD 9800X3D, 8c/16t V-Cache), native Windows, Session-0 SPRT
- openings: `sprt_openings.txt` sha256 `974e4b5ab871a9e106d0c766bfa39fc83676222337fb7702f61782e2ac5b3222` (canonical, 5000 lines)

### Stage 1 — fixed-nodes filter (FILTER, not a verdict)

- cmd: `sprt.exe -new .\ngn_ttage.exe -base .\ngn_base.exe -nodes 128000 -concurrency 8 -lowpower=false -openings sprt_openings.txt -elo0 -3 -elo1 3 -alpha 0.05 -beta 0.05 -maxgames 10000 -mingames 300`
- decision rule: H1 (positive side) → proceed to the real-clock verdict. H0 (negative side) /
  flat → record; reassess whether real-clock is still warranted. NOT a keep verdict on its own.

### Stage 2 — real-clock verdict (pending stage 1)

- TBD: `-tc 10+0.1` concurrency-1 (the per-change gate), launched only if stage 1 is non-negative.

## Result — SHELVED (neutral at fixed-nodes)

Stage-1 fixed-nodes filter ran to G1067 (255W 548D 264L, 51% draws): **elo −2.9 [−24,+18], LLR −0.32**
— neutral, hovering slightly negative the whole way (checked at G129 −16, G771 −5, G1067 −2.9; the CI
always spanned 0). The filter's screening question (is age-refresh promising enough to spend the
real-clock verdict on?) is answered NO. Stopped the run (its purpose served — NOT a strength verdict
on a killed SPRT) to free the box for the higher-EV IIR lever. Shelved per the KEEP rule: a
node-identical-within-search, non-correctness change with no measured improvement is not kept.

Why neutral: at 400K-equivalent fixed-nodes the per-move searches are shallow enough that the TT
isn't under heavy eviction pressure across moves, so protecting hot entries buys little. The
technique is sound and standard; it may pay at deeper real-clock TC (bigger TTs, more eviction) — but
that's unproven and not worth chasing now over IIR. cache.go change kept stashed (recorded here),
engine code reverted to clean HEAD. No box workers left (verified).
