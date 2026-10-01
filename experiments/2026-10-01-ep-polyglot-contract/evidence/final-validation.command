PYTHONDONTWRITEBYTECODE=1 python3 -c "from pathlib import Path; [compile(p.read_text(), str(p), 'exec') for p in Path('experiments/2026-10-01-ep-polyglot-contract').glob('*.py')]"
test -z "$(/usr/local/go/bin/gofmt -d engine/polyglot_external_contract_test.go)"
cmp -s ../polyglot-independent-oracle.json engine/testdata/polyglot_special_move_oracle.json
cmp -s engine/polyglot_external_contract_test.go ../polyglot-baseline-proof8b/engine/polyglot_external_contract_test.go
cmp -s engine/testdata/polyglot_special_move_oracle.json ../polyglot-baseline-proof8b/engine/testdata/polyglot_special_move_oracle.json
git diff --check
git diff --quiet
git -C ../polyglot-baseline-proof8b diff --check
git -C ../polyglot-baseline-proof8b diff --quiet
