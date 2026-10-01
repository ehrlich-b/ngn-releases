#!/usr/bin/env bash
set -euo pipefail
umask 022

root=/home/ehrli/nnue-owned-k4-20260920
run="$root/runs/k4-main-m1-lr1-20260922"
panel_root="$root/runs/k4-search-panel-20260921"
next="$run/search-panel"
original="$panel_root/panel.json"
runner="$panel_root/run_panel.py"
engine="$root/runs/k4-avx2-search-20260921/ngn-k4-avx2-v1"
model="$run/model-selected.nnue"
rodent=/home/ehrli/repos/ngn-smp-depth-stagger-gate-20260918/run-001/inputs/rodent_anand_512hl.bin
teacher="$root/teacher/stockfish-sf18-cb3d4ee9/src/stockfish"

[[ -d "$run" && ! -e "$run/PANEL_STATE" && ! -e "$next" ]]
[[ $(cat "$run/STATE") == CALIBRATED ]]
[[ $(sha256sum "$run/finalized-main/manifest.json" | cut -d' ' -f1) == ef6aca46d496be0fd876eb063cda3650a85ba7ae92c683105bf433a29565ad8e ]]
[[ $(sha256sum "$run/selection.json" | cut -d' ' -f1) == d2673192e31092a3bf5478541e27ae22e6ba3cb5f0ccb0500f9459b4556343b5 ]]
[[ $(sha256sum "$run/static/analysis.json" | cut -d' ' -f1) == 6727896d52162673d258c65d2fa9ea82692aca69ecd8ed1a1728cd619fb0f1b5 ]]
[[ $(sha256sum "$original" | cut -d' ' -f1) == 96de37a99f6c7649a1bf297329e49ce20490aa1c0c7c1e935620ce67ad92e50f ]]
[[ $(sha256sum "$runner" | cut -d' ' -f1) == e5a32d1b61a39d1fff67b4c994a6876aa0116757505e6cb66efbc69f70164d25 ]]
[[ $(sha256sum "$engine" | cut -d' ' -f1) == d17740084a5d7c96c3bb150ef55f8be3567779de33dddcb2faa95956458c475b ]]
[[ $(sha256sum "$rodent" | cut -d' ' -f1) == 5f7480b56538e9e64ee02fb5b4842bf0394ee44b707dae3a63ec7f902abd0afb ]]
[[ $(sha256sum "$teacher" | cut -d' ' -f1) == 174270346ae9ed600713d165fa745dfa87fc084e44907c8084767f2b905e86d3 ]]
model_sha=$(sha256sum "$model" | cut -d' ' -f1)
[[ "$model_sha" == cebec29cbab676af6a2ade2b67da5c9a01ddb7882ed604238c0e7d67893f72d6 ]]
grep -Fx "model_sha256=$model_sha" "$run/receipt.txt" > /dev/null
[[ $(systemctl --user is-active arli-hopper.service) == active ]]
! pgrep -x fastchess > /dev/null

mkdir "$next"
python3 - "$original" "$next/panel.json" "$model_sha" <<'PY'
import json
from pathlib import Path
import sys

original = json.loads(Path(sys.argv[1]).read_text())
assert original['schema'] == 'ngn-k4-search-panel-v1'
assert len(original['positions']) == 32
original['engine_sha256'] = 'd17740084a5d7c96c3bb150ef55f8be3567779de33dddcb2faa95956458c475b'
original['k4_model_sha256'] = sys.argv[3]
Path(sys.argv[2]).write_text(json.dumps(original, indent=2) + '\n')
PY

printf 'SEARCHING\n' > "$run/PANEL_STATE"
trap 'printf "FAILED\n" > "$run/PANEL_STATE"' ERR TERM
panel_sha=$(sha256sum "$next/panel.json" | cut -d' ' -f1)
taskset -c 12 nice -n 10 timeout 15m python3 "$runner" \
  --panel "$next/panel.json" --panel-sha256 "$panel_sha" \
  --engine "$engine" --k4-model "$model" --rodent-model "$rodent" \
  --teacher "$teacher" --output "$next/result.json" \
  > "$next/stdout.json" 2> "$next/stderr.txt"
[[ -s "$next/result.json" ]]
{
  printf 'candidate_model_sha256=%s\n' "$model_sha"
  printf 'candidate_panel_sha256=%s\n' "$panel_sha"
  printf 'candidate_search_result_sha256=%s\n' "$(sha256sum "$next/result.json" | cut -d' ' -f1)"
  printf 'ended_utc=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
} > "$next/receipt.txt"
printf 'PANEL_COMPLETE\n' > "$run/PANEL_STATE"
