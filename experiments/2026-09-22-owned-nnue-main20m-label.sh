#!/usr/bin/env bash
set -euo pipefail
umask 022

root=/home/ehrli/nnue-owned-k4-20260920
pilot="$root/runs/k4-production-pilot-20260921"
sampler="$pilot/selection/manifest.json"
source="$root/runs/k4-probe5m-20260921"
run="$root/runs/k4-main20m-label-20260922"
labels="$run/labels"
packed="$run/packed"
labeler="$root/bin-76fd484/ngnk4label"
packer="$root/bin-76fd484/ngnk4pack"
teacher="$root/teacher/stockfish-sf18-cb3d4ee9/src/stockfish"

windows_free() {
  /mnt/c/Windows/System32/WindowsPowerShell/v1.0/powershell.exe -NoProfile \
    -Command '(Get-PSDrive C).Free' | tr -d '\r'
}

check_resources() {
  [[ $(systemctl --user is-active arli-hopper.service) == active ]]
  local linux_bytes windows_bytes
  linux_bytes=$(df -B1 --output=avail / | tail -n 1 | tr -d ' ')
  windows_bytes=$(windows_free)
  [[ "$linux_bytes" =~ ^[0-9]+$ && "$windows_bytes" =~ ^[0-9]+$ ]]
  (( linux_bytes >= 40 * 1024 * 1024 * 1024 ))
  # Checking before every two-shard batch preserves ample margin above the
  # frozen physical-volume stop threshold of 25 GiB.
  (( windows_bytes >= 60 * 1024 * 1024 * 1024 ))
}

[[ ! -e "$run" ]]
[[ $(cat "$source/STATE") == LABELED ]]
[[ $(cat "$source/POSTLABEL_STATE") == CALIBRATED ]]
[[ $(cat "$source/PANEL_STATE") == PANEL_COMPLETE ]]
[[ $(sha256sum "$source/run-receipt.txt" | cut -d' ' -f1) == 995f643947688cd7a8b25ecab06944f3df05ac8cc74bb508568c1233b1d55edb ]]
[[ $(sha256sum "$sampler" | cut -d' ' -f1) == 53d240d4948a165a46fa42875f825edef79eb0f36a33e580a27cb562c56d4138 ]]
[[ $(sha256sum "$labeler" | cut -d' ' -f1) == cfd238f70fcb9ebec7e26415ff8b3cf09e62230cc840f068ea2b0d2916ee9db7 ]]
[[ $(sha256sum "$packer" | cut -d' ' -f1) == cd1cf0a4f2676f219cc2ea398b97753afa2516d626007048cf9e5a925bb13e25 ]]
[[ $(sha256sum "$teacher" | cut -d' ' -f1) == 174270346ae9ed600713d165fa745dfa87fc084e44907c8084767f2b905e86d3 ]]
! pgrep -x fastchess > /dev/null
check_resources

mkdir "$run" "$labels" "$packed"
printf 'LABELING\n' > "$run/STATE"
trap 'printf "FAILED\n" > "$run/STATE"' ERR TERM

initial_accepted=$(python3 - "$sampler" "$source" "$labels" "$packed" "$run/queue.tsv" <<'PY'
import json
from pathlib import Path
import sys

sampler, source, labels, packed, queue = map(Path, sys.argv[1:])
manifest = json.loads(sampler.read_text())
assert manifest['schema'] == 'ngn-k4-sampler-output-v2'
assert manifest['state'] == 'COMPLETE'
shards = [s for s in manifest['shards'] if s['stage'] == 'main-expansion' and s['split'] == 'train']
assert len(shards) == 238
assert sum(s['records'] for s in shards) == 23_750_028
assert len(shards[:50]) == 50 and sum(s['records'] for s in shards[:50]) == 5_000_000
accepted = 0
for shard in shards[:50]:
    sid = shard['shard_id']
    files = (
        (source / 'labels' / (sid + '.labels.jsonl'), labels / (sid + '.labels.jsonl')),
        (source / 'labels' / (sid + '.receipt.json'), labels / (sid + '.receipt.json')),
        (source / 'packed' / (sid + '.bf'), packed / (sid + '.bf')),
        (source / 'packed' / (sid + '.pack.json'), packed / (sid + '.pack.json')),
    )
    for old, new in files:
        assert old.is_file() and not new.exists(), str(old)
        new.symlink_to(old)
    accepted += json.loads(files[3][0].read_text())['accepted']
assert accepted == 4_327_066, accepted
with queue.open('x') as out:
    for shard in shards[50:]:
        item = shard['file']
        print(shard['shard_id'], item['path'], item['bytes'], item['sha256'], sep='\t', file=out)
assert len(shards[50:]) == 188
print(accepted)
PY
)
[[ "$initial_accepted" =~ ^[0-9]+$ ]]
accepted_total=$initial_accepted
completed_new=0
{
  printf 'started_utc=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  printf 'sampler_sha256=%s\n' 53d240d4948a165a46fa42875f825edef79eb0f36a33e580a27cb562c56d4138
  printf 'source_5m_run_receipt_sha256=%s\n' 995f643947688cd7a8b25ecab06944f3df05ac8cc74bb508568c1233b1d55edb
  printf 'queue_sha256=%s\n' "$(sha256sum "$run/queue.tsv" | cut -d' ' -f1)"
  printf 'initial_expansion_accepted=%s\n' "$initial_accepted"
  printf 'target_expansion_accepted=19000000\n'
} > "$run/run-receipt.txt"

label_one() {
  local shard_id=$1 input=$2 expected_bytes=$3 expected_sha=$4 cpu=$5
  local output="$labels/$shard_id.labels.jsonl"
  local label_receipt="$labels/$shard_id.receipt.json"
  local bf="$packed/$shard_id.bf"
  local pack_receipt="$packed/$shard_id.pack.json"
  [[ ! -e "$output" && ! -e "$label_receipt" && ! -e "$bf" && ! -e "$pack_receipt" ]]
  [[ $(stat -c %s "$input") == "$expected_bytes" ]]
  [[ $(sha256sum "$input" | cut -d' ' -f1) == "$expected_sha" ]]
  taskset -c "$cpu" nice -n 10 /usr/bin/time \
    -f 'elapsed_seconds=%e\nuser_seconds=%U\nsystem_seconds=%S\nmax_rss_kib=%M\nexit_code=%x' \
    -o "$labels/$shard_id.time.txt" \
    timeout --signal=TERM --kill-after=10s 45m \
    "$labeler" -input "$input" -output "$output" -receipt "$label_receipt" \
      -teacher "$teacher" \
      -teacher-source-commit cb3d4ee9b47d0c5aae855b12379378ea1439675c \
      -teacher-big-net-sha256 c288c895ea924429ea9092e3f36b2b3c1f00f2a3a4c759ff7e57e79e3b43e4a7 \
      -teacher-small-net-sha256 37f18f62d772f3107e1d6aaca3898c130c3c86f2ab63e6555fbbca20635a899d \
    > "$labels/$shard_id.stdout.json" 2> "$labels/$shard_id.stderr.txt"
  taskset -c "$cpu" nice -n 10 timeout --signal=TERM --kill-after=10s 10m \
    "$packer" -input "$output" -output "$bf" -receipt "$pack_receipt" \
    > "$packed/$shard_id.stdout.json" 2> "$packed/$shard_id.stderr.txt"
}

while IFS=$'\t' read -r shard_id input expected_bytes expected_sha; do
  check_resources
  first_id=$shard_id
  label_one "$shard_id" "$input" "$expected_bytes" "$expected_sha" 10 &
  first=$!
  second_id=
  if IFS=$'\t' read -r shard_id input expected_bytes expected_sha; then
    second_id=$shard_id
    label_one "$shard_id" "$input" "$expected_bytes" "$expected_sha" 11 &
    second=$!
  fi
  status=0
  wait "$first" || status=1
  if [[ -n "$second_id" ]]; then
    wait "$second" || status=1
  fi
  (( status == 0 ))
  for sid in "$first_id" "$second_id"; do
    if [[ -n "$sid" ]]; then
      count=$(jq -er '.accepted' "$packed/$sid.pack.json")
      [[ "$count" =~ ^[0-9]+$ ]]
      accepted_total=$((accepted_total + count))
      completed_new=$((completed_new + 1))
    fi
  done
  printf '%s\t%s\t%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$completed_new" "$accepted_total" >> "$run/progress.tsv"
  if (( accepted_total >= 19000000 )); then
    break
  fi
done < "$run/queue.tsv"

(( accepted_total >= 19000000 ))
{
  printf 'ended_utc=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  printf 'new_shards_packed=%s\n' "$completed_new"
  printf 'expansion_accepted_available=%s\n' "$accepted_total"
  printf 'label_receipts=%s\n' "$(find "$labels" -maxdepth 1 -name '*.receipt.json' | wc -l)"
  printf 'pack_receipts=%s\n' "$(find "$packed" -maxdepth 1 -name '*.pack.json' | wc -l)"
  printf 'linux_free_after_bytes=%s\n' "$(df -B1 --output=avail / | tail -n 1 | tr -d ' ')"
  printf 'windows_c_free_after_bytes=%s\n' "$(windows_free)"
} >> "$run/run-receipt.txt"
printf 'LABELED\n' > "$run/STATE"
