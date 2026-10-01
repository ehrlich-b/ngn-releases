#!/usr/bin/env bash
set -euo pipefail
umask 077

root=/home/ehrli/nnue-owned-k4-20260920
integration="$root/integration-v1/k4-main20m-lr1-screen-40-v1"
held="$integration/manifest-held.json"
approved="$integration/manifest.json"
token_file="$integration/approval-token.txt"
run="$integration/run-001"
source_root="$root/integration-v1/k4-pilot-screen-40-v1/inputs/source/candidate-match"
runner="$source_root/run_candidate_match.py"
model="$root/runs/k4-main-m1-lr1-20260922/model-selected.nnue"

[[ -d "$integration" && -f "$held" ]]
[[ ! -e "$approved" && ! -e "$token_file" && ! -e "$run" ]]
[[ $(sha256sum "$held" | cut -d' ' -f1) == 4aed850b0b0c8b916513811b7f8fea03a5409d9fd7d267341cff04bce689ec15 ]]
[[ $(sha256sum "$model" | cut -d' ' -f1) == cebec29cbab676af6a2ade2b67da5c9a01ddb7882ed604238c0e7d67893f72d6 ]]
[[ $(cat "$root/runs/k4-main20m-label-20260922/STATE") == LABELED ]]
[[ $(cat "$root/runs/k4-main-m1-lr1-20260922/STATE") == CALIBRATED ]]
[[ $(cat "$root/runs/k4-main-m1-lr1-20260922/PANEL_STATE") == PANEL_COMPLETE ]]
[[ $(sha256sum "$integration/inputs/openings/opening-prefixes-4021-4040.txt" | cut -d' ' -f1) == 5da172ae1d740d788d7e149ea48847e9b9e340accad3ef0c99440ffe3c315e66 ]]
[[ $(sha256sum "$integration/inputs/openings/openings-4021-4040.pgn" | cut -d' ' -f1) == 45bcb60d261f7c384f271281797655342f5c74e6ae6ac6d8dd91a4f0430d1d5b ]]
[[ $(systemctl --user is-active arli-hopper.service) == active ]]
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
assert reviewed == '4bed01e100021ab3fb4e6d7d26d31f9dec79e6386ebb386b3518304cdbc67788'
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
