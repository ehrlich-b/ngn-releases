# Verdict Corpus Manifest

Tracked identity of every corpus a game or eval verdict may depend on. A run
record (see `RUN_RECORD_TEMPLATE.md`) must cite the corpus it used by name; this
file pins each name to a checksum so the corpus is reproducible even when the file
itself is gitignored (too large to track). Regenerate with `shasum -a 256 <file>`
and `wc -l < <file>`; if a checksum changes, update its row in the same commit.

## Opening sets

| name | path | tracked | lines | sha256 |
|---|---|---|---|---|
| sprt_openings | `output/sprt_openings.txt` | no (gitignored, ~150 KB) | 5000 | `974e4b5ab871a9e106d0c766bfa39fc83676222337fb7702f61782e2ac5b3222` |

`sprt_openings.txt` is the default self-play SPRT opening file. Because it is
gitignored, a verdict that uses it MUST record this checksum in its run record;
regrow/extend it via `cmd/sprt -genbook` (which changes the checksum — update the
row above in that commit).

## Eval corpora

| name | path | tracked | lines | sha256 |
|---|---|---|---|---|
| acpl_corpus | `acpl_corpus.labels` | yes | 150 | `7bc44fef3566c79e0c61eeb8bd4ac9f72427e103791dfba436b44d2401bbfa1d` |

Stockfish-labeled ACPL pre-filter corpus (`make acpl` / `scripts/acpl.sh`).
Tracked, so its checksum is informational; regrow via `cmd/oracle gencorpus` + `label`.

## Anchor ratings

`opponents/ratings.json` (tracked) maps each built anchor (the Blunder ladder,
etc.) to its published CCRL rating. The anchor binaries themselves are gitignored;
rebuild with `scripts/build-anchors.sh`. A gauntlet's absolute number is only as
trustworthy as this map and the anchors' build provenance.
