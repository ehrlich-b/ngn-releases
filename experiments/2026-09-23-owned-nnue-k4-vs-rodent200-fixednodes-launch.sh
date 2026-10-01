#!/usr/bin/env bash
set -euo pipefail
umask 077

root=/home/ehrli/nnue-owned-k4-20260920
integration="$root/integration-v1/k4-main20m-vs-rodent-fixednodes-200-v1"
held="$integration/manifest-held.json"
approved="$integration/manifest.json"
token_file="$integration/approval-token.txt"
run="$integration/run-001"
source_root="$root/integration-v1/k4-fixednodes-source-20260923"
runner="$source_root/run_candidate_match.py"
model="$root/runs/k4-main-m1-lr1-20260922/model-selected.nnue"
rodent=/home/ehrli/repos/ngn-smp-depth-stagger-gate-20260918/run-001/inputs/rodent_anand_512hl.bin

[[ -d "$integration" && -f "$held" ]]
[[ ! -e "$approved" && ! -e "$token_file" && ! -e "$run" ]]
[[ $(sha256sum "$held" | cut -d' ' -f1) == c8ebf895a25b144813b977ab9523f46113e3ec4f5ac0ae6eb4bc624511c328ce ]]
[[ $(sha256sum "$source_root/manifest.py" | cut -d' ' -f1) == a7ce8f4a6141e963688eefac273a9b9822f87d57674ce7191f288701af492b3b ]]
[[ $(sha256sum "$source_root/manifest.schema.json" | cut -d' ' -f1) == 9d5abd371d338e724bf2da3cb520069293f9472b9e3f6827c1d21ce722155419 ]]
[[ $(sha256sum "$runner" | cut -d' ' -f1) == 9ba8aa75a917923b01a456d30e513f7793ebc585a75522765275f6db2ef89a6a ]]
[[ $(sha256sum "$model" | cut -d' ' -f1) == cebec29cbab676af6a2ade2b67da5c9a01ddb7882ed604238c0e7d67893f72d6 ]]
[[ $(sha256sum "$rodent" | cut -d' ' -f1) == 5f7480b56538e9e64ee02fb5b4842bf0394ee44b707dae3a63ec7f902abd0afb ]]
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
assert reviewed == '0b602e6b7026e1477b9da2beba405b3d7fa92d6f6cbeaa0a276b8a209fd144a2'
assert value['schema'] == 'ngn-candidate-match-borrowed-fixed-nodes-v1'
assert [role['backend'] for role in value['inputs']['roles']] == ['ngn-k4-768-v1', 'rodent-v1.1-anand']
assert value['match']['games'] == 200 and value['match']['pairs'] == 100
assert value['match']['node_limit'] == 160000 and 'time_control' not in value['match']
assert value['match']['concurrency'] == 2
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
