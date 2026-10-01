set -euo pipefail
git diff --check
tmpdir=$(mktemp -d /tmp/ngn-ep-polyglot-review.XXXXXX)
index="$tmpdir/index"
names="$tmpdir/names"
GIT_INDEX_FILE="$index" git read-tree HEAD
GIT_INDEX_FILE="$index" git add -A -f -- \
    LICENSES/chess-library-MIT.txt \
    engine/book_test.go engine/enpassant_repetition_correctness_test.go \
    engine/polyglot.go engine/polyglot_random.go \
    engine/polyglot_external_contract_test.go \
    engine/polyglot_book_move_contract_test.go \
    engine/testdata/polyglot_book_move_oracle.json \
    engine/testdata/polyglot_book_move_oracle.provenance.md \
    experiments/2026-10-01-ep-polyglot-integration-review.md \
    experiments/2026-10-01-ep-polyglot-integration-review
GIT_INDEX_FILE="$index" git diff --cached --check
GIT_INDEX_FILE="$index" git diff --cached --name-only > "$names"
python3 - "$names" <<'PY'
from pathlib import Path
import sys

names = Path(sys.argv[1]).read_text().splitlines()
exact = {
    "LICENSES/chess-library-MIT.txt",
    "engine/book_test.go",
    "engine/enpassant_repetition_correctness_test.go",
    "engine/polyglot.go",
    "engine/polyglot_random.go",
    "engine/polyglot_external_contract_test.go",
    "engine/polyglot_book_move_contract_test.go",
    "engine/testdata/polyglot_book_move_oracle.json",
    "engine/testdata/polyglot_book_move_oracle.provenance.md",
    "experiments/2026-10-01-ep-polyglot-integration-review.md",
}
prefix = "experiments/2026-10-01-ep-polyglot-integration-review/"
unexpected = [name for name in names if name not in exact and not name.startswith(prefix)]
assert not unexpected, unexpected
print(f"scoped_candidate_files={len(names)}")
PY
test -z "$(/usr/local/go/bin/gofmt -d \
    engine/book_test.go engine/enpassant_repetition_correctness_test.go \
    engine/polyglot.go engine/polyglot_random.go \
    engine/polyglot_external_contract_test.go engine/polyglot_book_move_contract_test.go)"
python3 - <<'PY'
import ast
from pathlib import Path

root = Path("experiments/2026-10-01-ep-polyglot-integration-review")
for name in ("verify_key_transitions.py", "verify_integration.py"):
    ast.parse((root / name).read_text(), filename=name)
print("python_ast=ok files=2")
PY
python3 -B experiments/2026-10-01-ep-polyglot-integration-review/verify_key_transitions.py
python3 -B experiments/2026-10-01-ep-polyglot-integration-review/verify_integration.py
cmp -s experiments/2026-10-01-ep-polyglot-integration-review/evidence/integration-independent-key-transition.json ../integration-independent-key-transition.json
cmp -s experiments/2026-10-01-ep-polyglot-integration-review/evidence/integration-materialization.json ../integration-materialization.json
git diff --quiet 28d66322debef23d01645a1bc73d7e942ada9813 -- engine/hash.go engine/position.go
test -z "$(git diff --name-only HEAD -- engine/search.go engine/evaluation.go engine/uci.go scripts/ownedbenchmark.py scripts/ownedmatch_gochess.py)"
printf 'frozen_inputs=byte-identical\nreviewed_components=byte-identical\nforbidden_production_paths=unchanged\ngit_diff_check=ok\ngofmt=ok\n'
