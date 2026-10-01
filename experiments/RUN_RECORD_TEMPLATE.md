# Run Record Template

Every game or eval verdict gets one tracked record in this directory, named
`YYYY-MM-DD-<slug>.md`. Copy the YAML block below into that file and fill every
field BEFORE drawing a conclusion; prose narrative goes in the body after the
block. The block is the data — keep it parseable.

**A/A preflight is required** before any real-clock concurrent self-play verdict:
run the same binary against itself at the exact TC / concurrency / CPU affinity /
power profile / machine you will use for the candidate, and confirm the null is
centered (no score or draw-rate skew introduced by the harness). Record its result
in `aa_preflight`. A candidate verdict on an un-preflighted concurrent config is
not valid (this is the failure mode the 2026-06-28 reset exists to prevent).

```yaml
id:                        # YYYY-MM-DD-<slug>
date:                      # absolute
change_class:              # correctness | node-identical | speed | search/eval heuristic | tune | lane-closure
hypothesis:                # the single question this run decides
base_commit:               # commit or patch hash of the BASE
candidate_commit:          # commit or patch hash of the CANDIDATE
base_binary_sha256:
candidate_binary_sha256:
harness_commit:            # scripts/ commit driving the run
command:                   # exact command line
machine:                   # CPU model, core type (P/E), OS
go_version:
goarch_goamd64:
tc:                        # e.g. 10+0.1  (SECONDS, not minutes — all our games are bullet)
concurrency:
openings:                  # corpus NAME from corpus_manifest.md
openings_sha256:
aa_preflight:              # result of the same-binary A/A (REQUIRED for real-clock concurrent runs)
decision_rule:             # predeclared bounds / cap / penta rule, written BEFORE the run
games_or_pairs:
result:                    # W/D/L or pentanomial buckets
flags_errors:              # flag-outs, crashes, illegal moves, no-move results
verdict:                   # keep | shelve | reject | inconclusive, judged against decision_rule
next_action:
```

Existing records (`2026-06-22-damping-bvsa.md`, `2026-06-23-corrhist-cvsb.md`,
`2026-06-23-rfp-damp-dvsb.md`) predate this template and are kept as historical
evidence; new runs use the block above.
