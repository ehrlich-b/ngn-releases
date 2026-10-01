#!/usr/bin/env bash
set -euo pipefail
umask 077

root=/home/ehrli/nnue-owned-k4-20260920
integration="$root/integration-v1/k4-main20m-vs-rodent-200-v1"
held="$integration/manifest-held.json"
approved="$integration/manifest.json"
token_file="$integration/approval-token.txt"
run="$integration/run-001"
source_root="$root/integration-v1/k4-owned-vs-rodent-v1-source"
runner="$source_root/run_candidate_match.py"
model="$root/runs/k4-main-m1-lr1-20260922/model-selected.nnue"
rodent=/home/ehrli/repos/ngn-smp-depth-stagger-gate-20260918/run-001/inputs/rodent_anand_512hl.bin

[[ -d "$integration" && -f "$held" ]]
[[ ! -e "$approved" && ! -e "$token_file" && ! -e "$run" ]]
[[ $(sha256sum "$held" | cut -d' ' -f1) == 60e7df9d8c4eefefaa2fb15d28a718803430c02ec4a78ed1129d0ceacb60f919 ]]
[[ $(sha256sum "$source_root/manifest.py" | cut -d' ' -f1) == d1ad1c5e1bb70b2f2532eb09ba20f6235d2e8a92de586179017f8a1e5d50cb84 ]]
[[ $(sha256sum "$source_root/manifest.schema.json" | cut -d' ' -f1) == 8911954c0a0f031f04e56513fc072bff3e71b6605d68b297292e064882fccac4 ]]
[[ $(sha256sum "$model" | cut -d' ' -f1) == cebec29cbab676af6a2ade2b67da5c9a01ddb7882ed604238c0e7d67893f72d6 ]]
[[ $(sha256sum "$rodent" | cut -d' ' -f1) == 5f7480b56538e9e64ee02fb5b4842bf0394ee44b707dae3a63ec7f902abd0afb ]]
[[ $(sha256sum "$integration/inputs/openings/opening-prefixes-4241-4340.txt" | cut -d' ' -f1) == 1031bb8040cce617a115fba0f42545041a189260197527af6ab92ce9b568eb45 ]]
[[ $(sha256sum "$integration/inputs/openings/openings-4241-4340.pgn" | cut -d' ' -f1) == 7862f4938d57884a462e2012c43a48fadbb9bec2f5edf1a11ea72bd85085da0b ]]
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
assert reviewed == '988c40972ea671e1dc3baaf42a366633d3732a53845fdf5f3fb3c1a4d5b39724'
assert value['schema'] == 'ngn-candidate-match-borrowed-v1'
assert [role['backend'] for role in value['inputs']['roles']] == ['ngn-k4-768-v1', 'rodent-v1.1-anand']
assert value['match']['games'] == 200 and value['match']['pairs'] == 100
assert value['match']['time_control'] == '10+0.1' and value['match']['concurrency'] == 2
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
