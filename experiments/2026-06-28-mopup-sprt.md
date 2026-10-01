# 2026-06-28 mop-up eval — self-play SPRT (NEW vs immediate base)

First Elo lever after the re-pin. Wiring in a bare-king mop-up mating gradient
(drive the lone king to a corner + bring the winning king up). Diagnosed from the
re-pin's 400 games: 162/400 (40%) were 50-move/max-move SHUFFLE-draws (52 vs
anchors NGN outrates by 240-510), and NGN's K+R-vs-K score was a FLAT +583 (no
progress gradient) because the endgame eval was dead code, never wired into
`Evaluate`. Mechanism verified: NGN now converts K+R-vs-K (locks mate-in-9);
symmetric; gated to a bare loser vs mating material so normal eval is untouched.

```yaml
id:                 2026-06-28-mopup-sprt
date:               2026-06-28
change_class:       search/eval heuristic (correctness-adjacent: NGN could not mate K+R vs K)
hypothesis:         wiring the mop-up gradient converts won-endgame shuffle-draws into wins ⇒ NEW > BASE
base_commit:        2dadef8 (HEAD, mop-up NOT wired)
candidate:          working tree (eval.go mopUpEval + evaluateUnsafe wiring + texel_gradient Fixed lump)
base_binary_sha256: (build/win/ngn_base.exe) de0f5a180aa4f841...
cand_binary_sha256: (build/win/ngn_new.exe)  53350dc415600736...
harness:            cmd/sprt cross-compiled to sprt.exe (windows/amd64 v3)
command:            sprt.exe -new .\ngn_new.exe -base .\ngn_base.exe -nodes 128000 -concurrency 8 -lowpower=false -openings sprt_openings.txt -elo0 0 -elo1 6 -maxgames 4000 -mingames 300
machine:            LAN 9800X3D (8c/16t), native Windows
gate_type:          fixed-NODES (128k/move) — clean+deterministic for a pure-eval change (no node-spending technique to under-credit), concurrency-safe
openings:           sprt_openings (corpus_manifest.md), sha256 974e4b5...
decision_rule:      KEEP if the SPRT accepts H1 [0,6] OR finishes with NEW score >= 50% and LLR not trending to regression — the mop-up is a correctness-adjacent fix gated to won bare-king endgames, so it can only help conversion, never hurt normal play. REJECT only on a MEASURED regression (NEW significantly < 50%), which would flag an unexpected side effect.
games_or_pairs:     PENDING
result:             PENDING
verdict:            PENDING
next_action:        if KEEP, commit the mop-up + a follow-up gauntlet to re-confirm the absolute number moved off 2666; then the next conversion lever (general endgame technique beyond bare-king).
```
