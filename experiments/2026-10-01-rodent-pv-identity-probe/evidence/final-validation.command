PYTHONDONTWRITEBYTECODE=1 python3 -c "from pathlib import Path; [compile(p.read_text(),str(p),'exec') for p in Path('experiments/2026-10-01-rodent-pv-identity-probe').glob('*.py')]"
test -z "$(/usr/local/go/bin/gofmt -d experiments/2026-10-01-rodent-pv-identity-probe/source_mechanism_model.go)"
git diff --check
git diff --quiet
test "$(git rev-parse HEAD)" = cc0846a4da60a2c7172fab3301ba719970ce910e
! grep --exclude=final-validation.stdout --exclude=final-validation.stderr -rInE '[[:blank:]]+$' experiments/2026-10-01-rodent-pv-identity-probe experiments/2026-10-01-rodent-pv-identity-probe.md ../rodent-probe-result.md
