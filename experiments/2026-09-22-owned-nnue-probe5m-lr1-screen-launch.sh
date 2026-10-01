#!/usr/bin/env bash
set -Eeuo pipefail
umask 077

root=/home/ehrli/nnue-owned-k4-20260920
integration="$root/integration-v1/k4-probe5m-lr1-screen-40-v1"
held="$integration/manifest-held.json"
approved="$integration/manifest.json"
token_file="$integration/approval-token.txt"
run="$integration/run-001"
source_root="$root/integration-v1/k4-pilot-screen-40-v1/inputs/source/candidate-match"
runner="$source_root/run_candidate_match.py"
model="$root/runs/k4-probe5m-lr1-20260922/model-selected.nnue"
engine="$root/integration-v1/k4-lr1-screen-40-v1/inputs/engine/ngn-k4-lr1-screen-v1"

[[ -d "$integration" && -f "$held" ]]
[[ ! -e "$approved" && ! -e "$token_file" && ! -e "$run" ]]
[[ $(sha256sum "$held" | cut -d' ' -f1) == 9752d40ff4862b53849de299b490d111c4a95c553090e777166f4020aedde0a5 ]]
[[ $(sha256sum "$model" | cut -d' ' -f1) == 55d109e9ebc14fe5836974ed07a8d7f913c58b066408fe921462df274bd2885a ]]
[[ $(sha256sum "$engine" | cut -d' ' -f1) == 1f2502844f0eab41832cd0818f57c81d59ec66a774e1a09756e0cc72c0c6d1be ]]
[[ $(cat "$root/runs/k4-probe5m-lr1-20260922/STATE") == CALIBRATED ]]
[[ $(cat "$root/runs/k4-probe5m-lr1-20260922/PANEL_STATE") == PANEL_COMPLETE ]]
[[ -z $(pgrep -x fastchess || true) ]]

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
assert reviewed == '69d91d5314f71d613eaf114e7c3327f9fd46459c28dbc3965d586a4b7fec800c'
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
