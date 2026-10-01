#!/usr/bin/env bash
set -euo pipefail
umask 077

root=/home/ehrli/nnue-owned-k4-20260920
integration="$root/integration-v1/k4-main20m-lr1-confirm-400-v1"
held="$integration/manifest-held.json"
approved="$integration/manifest.json"
token_file="$integration/approval-token.txt"
run="$integration/run-001"
source_root="$root/integration-v1/k4-pilot-screen-40-v1/inputs/source/candidate-match"
runner="$source_root/run_candidate_match.py"
model="$root/runs/k4-main-m1-lr1-20260922/model-selected.nnue"

[[ -d "$integration" && -f "$held" ]]
[[ ! -e "$approved" && ! -e "$token_file" && ! -e "$run" ]]
[[ $(sha256sum "$held" | cut -d' ' -f1) == 8054dac3d04f0d71f621bc18792a2167d9d1d6830159372f915e262c565b4536 ]]
[[ $(sha256sum "$model" | cut -d' ' -f1) == cebec29cbab676af6a2ade2b67da5c9a01ddb7882ed604238c0e7d67893f72d6 ]]
[[ $(sha256sum "$integration/inputs/openings/opening-prefixes-4041-4240.txt" | cut -d' ' -f1) == 645894923fa6541a687eac7b65c5d3f09c094c3b503f94ccd26f18c5c695148e ]]
[[ $(sha256sum "$integration/inputs/openings/openings-4041-4240.pgn" | cut -d' ' -f1) == ed36907ffd9711ee917f8c547b6d351a3d52b7ddbb7ec7518bd3b4e54f5fc497 ]]
[[ $(cat "$root/runs/k4-main-m1-lr1-20260922/STATE") == CALIBRATED ]]
[[ $(cat "$root/runs/k4-main-m1-lr1-20260922/PANEL_STATE") == PANEL_COMPLETE ]]
[[ $(systemctl --user is-active arli-hopper.service) == active ]]
[[ -z $(pgrep -x fastchess || true) ]]
[[ -z $(pgrep -x ngn_k4_train || true) ]]

python3 - "$held" "$approved" "$token_file" "$source_root" <<'PY'
import json
import os
from pathlib import Path
import secrets
import sys

held, approved, token_file, source_root = map(Path, sys.argv[1:])
sys.path.insert(0, str(source_root))
from manifest import review_subject_sha256, validate_manifest

value = json.loads(held.read_text())
validate_manifest(value)
assert value['status'] == 'HELD'
reviewed = review_subject_sha256(value)
assert reviewed == '59e35609c7b3d1ef57d49c8fcdb6d9bc42069ef44c476f005745ba617fc4df7f'
assert value['match']['games'] == 400 and value['match']['pairs'] == 200
assert value['match']['time_control'] == '30+0.3' and value['match']['concurrency'] == 2
value['status'] = 'APPROVED_TO_RUN'
value['approval'] = {
    'authority': 'root',
    'token': secrets.token_urlsafe(32),
    'exclusive_match_window': True,
    'reviewed_manifest_sha256': reviewed,
}
validate_manifest(value)
with approved.open('x') as out:
    json.dump(value, out, indent=2)
    out.write('\n')
os.chmod(approved, 0o600)
with token_file.open('x') as out:
    out.write(value['approval']['token'] + '\n')
os.chmod(token_file, 0o600)
PY

exec nice -n 10 taskset -c 12,14 python3 "$runner" \
  --manifest "$approved" --output "$run" \
  --approval-token "$(cat "$token_file")"
