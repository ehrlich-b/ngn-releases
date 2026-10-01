#!/usr/bin/env bash
set -euo pipefail
umask 022

root=/home/ehrli/nnue-owned-k4-20260920
run="$root/runs/k4-main20m-label-20260922"
labels="$run/labels"
packed="$run/packed"
labeler="$root/bin-76fd484/ngnk4label"
packer="$root/bin-76fd484/ngnk4pack"
teacher="$root/teacher/stockfish-sf18-cb3d4ee9/src/stockfish"

windows_free() {
  /mnt/c/Windows/System32/WindowsPowerShell/v1.0/powershell.exe -NoProfile \
    -Command '(Get-PSDrive C).Free' < /dev/null | tr -d '\r'
}

check_resources() {
  [[ $(systemctl --user is-active arli-hopper.service) == active ]]
  local linux_bytes windows_bytes
  linux_bytes=$(df -B1 --output=avail / | tail -n 1 | tr -d ' ')
  windows_bytes=$(windows_free)
  [[ "$linux_bytes" =~ ^[0-9]+$ && "$windows_bytes" =~ ^[0-9]+$ ]]
  (( linux_bytes >= 40 * 1024 * 1024 * 1024 ))
  (( windows_bytes >= 60 * 1024 * 1024 * 1024 ))
}

[[ $(systemctl --user is-active ngn-k4-main20m-label-20260922.service) == failed ]]
[[ -d "$run" && $(cat "$run/STATE") == FAILED ]]
[[ $(sha256sum "$run/queue.tsv" | cut -d' ' -f1) == f7222c1972d0e7010756acb75100e1381c574bd0835d682d034daec51a2aa78c ]]
[[ $(sha256sum "$labeler" | cut -d' ' -f1) == cfd238f70fcb9ebec7e26415ff8b3cf09e62230cc840f068ea2b0d2916ee9db7 ]]
[[ $(sha256sum "$packer" | cut -d' ' -f1) == cd1cf0a4f2676f219cc2ea398b97753afa2516d626007048cf9e5a925bb13e25 ]]
[[ $(sha256sum "$teacher" | cut -d' ' -f1) == 174270346ae9ed600713d165fa745dfa87fc084e44907c8084767f2b905e86d3 ]]
[[ $(wc -l < "$run/progress.tsv") == 1 ]]
[[ $(tail -n 1 "$run/progress.tsv" | cut -f2,3) == $'1\t4414183' ]]
! pgrep -x ngnk4label > /dev/null
! pgrep -x ngnk4pack > /dev/null
! pgrep -x fastchess > /dev/null
check_resources

# The first new shard finished cleanly. Verify its label and BF bytes and
# cross-linked receipts before continuing with the next frozen queue entry.
python3 - "$run" <<'PY'
import hashlib
import json
from pathlib import Path
import sys

run = Path(sys.argv[1])
sid = 'main-expansion-train-000050'
label = run / 'labels' / f'{sid}.labels.jsonl'
bf = run / 'packed' / f'{sid}.bf'
l = json.loads((run / 'labels' / f'{sid}.receipt.json').read_text())
p = json.loads((run / 'packed' / f'{sid}.pack.json').read_text())
queue = (run / 'queue.tsv').read_text().splitlines()
assert len(queue) == 188 and queue[0].split('\t')[0] == sid
assert queue[1].split('\t')[0] == 'main-expansion-train-000051'
assert l['input_sha256'] == queue[0].split('\t')[3]
assert p['input_shard_sha256'] == l['input_sha256']
assert l['accepted'] == p['accepted'] == 87117
for path, size, digest in ((label, l['output_bytes'], l['output_sha256']),
                           (bf, p['output_bytes'], p['output_sha256'])):
    data = path.read_bytes()
    assert len(data) == size and hashlib.sha256(data).hexdigest() == digest, path
assert p['input_sha256'] == l['output_sha256']
PY

# mapfile owns the complete frozen queue in Bash memory. Child processes no
# longer share a queue file descriptor, which ended the first attempt after
# the first shard despite 187 entries remaining.
mapfile -t queue < "$run/queue.tsv"
[[ ${#queue[@]} == 188 ]]
accepted_total=4414183
completed_new=1
printf 'resume1_started_utc=%s\nresume1_initial_expansion_accepted=%s\n' \
  "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$accepted_total" >> "$run/run-receipt.txt"
printf 'LABELING\n' > "$run/STATE"
trap 'printf "FAILED\n" > "$run/STATE"' ERR TERM

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
    < /dev/null > "$labels/$shard_id.stdout.json" 2> "$labels/$shard_id.stderr.txt"
  taskset -c "$cpu" nice -n 10 timeout --signal=TERM --kill-after=10s 10m \
    "$packer" -input "$output" -output "$bf" -receipt "$pack_receipt" \
    < /dev/null > "$packed/$shard_id.stdout.json" 2> "$packed/$shard_id.stderr.txt"
}

for ((index = 1; index < ${#queue[@]}; index += 2)); do
  check_resources
  IFS=$'\t' read -r first_id first_input first_bytes first_sha <<< "${queue[index]}"
  label_one "$first_id" "$first_input" "$first_bytes" "$first_sha" 10 &
  first=$!
  second_id=
  if (( index + 1 < ${#queue[@]} )); then
    IFS=$'\t' read -r second_id second_input second_bytes second_sha <<< "${queue[index+1]}"
    label_one "$second_id" "$second_input" "$second_bytes" "$second_sha" 11 &
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
done

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
