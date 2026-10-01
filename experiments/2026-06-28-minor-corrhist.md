# 2026-06-28 Minor-piece correction history — real-clock self-play SPRT (STAGED, CONTINGENT)

Third correction history (after pawn + non-pawn). Keyed on each side's minor-piece
(knight+bishop) placement with a distinct constant pair, summed alongside the pawn and
non-pawn terms at the static-eval site. A FINER key than non-pawn (which lumps every
non-pawn into one slot): it sharpens corrections for recurring minor-piece configuration
errors (outposts, bad bishops, N-vs-B imbalances). Exact mechanical mirror of the validated
non-pawn code — same EMA/gating/weight — so low bug surface.

```yaml
id:            2026-06-28-minor-corrhist
date:          2026-06-28
change_class:  search/eval heuristic (eval correction) — gate = real-clock self-play SPRT
hypothesis:    a finer minor-piece corrector adds sharper leaf-eval corrections ⇒ NEW >= BASE
base:          ngn_corr.exe (HEAD engine + non-pawn corrector) — the IMMEDIATE base, so this isolates ONLY the minor term
candidate:     ngn_corr2.exe (HEAD engine + non-pawn + minor) — SHA256 aa3ba2ec96e37e245fbf7d98684d2777dd892aac69b44d832da70e6253bba829, staged on box ~\ngn\sprt\
command:       sprt.exe -new .\ngn_corr2.exe -base .\ngn_corr.exe -tc 10+0.1 -concurrency 8 -lowpower=false -openings sprt_openings.txt -elo0 0 -elo1 6 -maxgames 3000 -mingames 400
local_checks:  go build OK; go test -short ./engine GREEN; depth-14 search sane (cp 17-32, sensible PVs, 2.3M nps).
contingency:   CONTINGENT on the non-pawn run keeping. If non-pawn REGRESSES, do NOT run this (stacking a finer corrector on a base that already over-corrects compounds the problem) — sort the non-pawn base first (halve its weight), then re-stage minor on the kept base.
decision_rule: KEEP if real-clock self-play shows NEW >= BASE non-regressing (KEEP rule: tiny non-regressing gain is a keep). If it REGRESSES, suspect summed-magnitude over-correction (pawn+nonpawn+minor each ±49cp ⇒ ±147 worst case) — reduce the per-corrector weight before shelving.
status:        STAGED — behind the running non-pawn SPRT. Run only if non-pawn keeps.
result:        PENDING
verdict:       PENDING
```
