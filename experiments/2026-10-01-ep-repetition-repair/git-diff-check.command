git diff --check
for f in engine/enpassant_repetition_correctness_test.go experiments/2026-10-01-ep-repetition-repair.md; do git diff --no-index --check -- /dev/null "$f"; done  # exit 1 means content differs; any output means whitespace error
