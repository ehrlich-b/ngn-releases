#!/usr/bin/env bash
set -Eeuo pipefail
umask 022

root=/home/ehrli/nnue-owned-k4-20260920
pilot="$root/runs/k4-production-pilot-20260921"
sampler="$pilot/selection/manifest.json"
run="$root/runs/k4-probe5m-20260921"
labels="$run/labels"
packed="$run/packed"
labeler="$root/bin-76fd484/ngnk4label"
packer="$root/bin-76fd484/ngnk4pack"
teacher="$root/teacher/stockfish-sf18-cb3d4ee9/src/stockfish"

[[ ! -e "$run" ]]
[[ $(sha256sum "$sampler" | cut -d' ' -f1) == 53d240d4948a165a46fa42875f825edef79eb0f36a33e580a27cb562c56d4138 ]]
[[ $(sha256sum "$labeler" | cut -d' ' -f1) == cfd238f70fcb9ebec7e26415ff8b3cf09e62230cc840f068ea2b0d2916ee9db7 ]]
[[ $(sha256sum "$packer" | cut -d' ' -f1) == cd1cf0a4f2676f219cc2ea398b97753afa2516d626007048cf9e5a925bb13e25 ]]
[[ $(sha256sum "$teacher" | cut -d' ' -f1) == 174270346ae9ed600713d165fa745dfa87fc084e44907c8084767f2b905e86d3 ]]
[[ $(systemctl --user is-active arli-hopper.service) == active ]]
linux_free=$(df -B1 --output=avail / | tail -n 1 | tr -d ' ')
windows_free=$(/mnt/c/Windows/System32/WindowsPowerShell/v1.0/powershell.exe -NoProfile -Command '(Get-PSDrive C).Free' | tr -d '\r')
(( linux_free >= 40 * 1024 * 1024 * 1024 ))
(( windows_free >= 60 * 1024 * 1024 * 1024 ))

mkdir "$run" "$labels" "$packed"
printf 'LABELING\n' > "$run/STATE"
trap 'printf "FAILED\n" > "$run/STATE"' ERR TERM
python3 - "$sampler" "$run/queue.tsv" <<'PY'
import json, sys
manifest = json.load(open(sys.argv[1]))
assert manifest['schema'] == 'ngn-k4-sampler-output-v2'
assert manifest['state'] == 'COMPLETE'
shards = [s for s in manifest['shards'] if s['stage'] == 'main-expansion' and s['split'] == 'train']
assert len(shards) >= 50
assert sum(s['records'] for s in shards[:50]) == 5_000_000
with open(sys.argv[2], 'x') as out:
    for shard in shards[:50]:
        item = shard['file']
        assert shard['records'] == 100_000
        print(shard['shard_id'], item['path'], item['bytes'], item['sha256'], sep='\t', file=out)
PY
{
  printf 'started_utc=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  printf 'sampler_sha256=%s\n' 53d240d4948a165a46fa42875f825edef79eb0f36a33e580a27cb562c56d4138
  printf 'queue_sha256=%s\n' "$(sha256sum "$run/queue.tsv" | cut -d' ' -f1)"
  printf 'linux_free_before_bytes=%s\n' "$linux_free"
  printf 'windows_c_free_before_bytes=%s\n' "$windows_free"
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
    "$labeler" -input "$input" -output "$output" -receipt "$label_receipt" \
      -teacher "$teacher" \
      -teacher-source-commit cb3d4ee9b47d0c5aae855b12379378ea1439675c \
      -teacher-big-net-sha256 c288c895ea924429ea9092e3f36b2b3c1f00f2a3a4c759ff7e57e79e3b43e4a7 \
      -teacher-small-net-sha256 37f18f62d772f3107e1d6aaca3898c130c3c86f2ab63e6555fbbca20635a899d \
    > "$labels/$shard_id.stdout.json" 2> "$labels/$shard_id.stderr.txt"
  taskset -c "$cpu" nice -n 10 "$packer" \
    -input "$output" -output "$bf" -receipt "$pack_receipt" \
    > "$packed/$shard_id.stdout.json" 2> "$packed/$shard_id.stderr.txt"
}

index=0
while IFS=$'\t' read -r shard_id input expected_bytes expected_sha; do
  label_one "$shard_id" "$input" "$expected_bytes" "$expected_sha" 10 &
  first=$!
  if IFS=$'\t' read -r shard_id input expected_bytes expected_sha; then
    label_one "$shard_id" "$input" "$expected_bytes" "$expected_sha" 11 &
    second=$!
  else
    second=
  fi
  status=0
  wait "$first" || status=1
  if [[ -n "$second" ]]; then
    wait "$second" || status=1
  fi
  (( status == 0 ))
  index=$((index + 2))
done < "$run/queue.tsv"
[[ "$index" == 50 ]]
printf 'LABELED\n' > "$run/STATE"
printf 'ended_utc=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" >> "$run/run-receipt.txt"
printf 'label_receipts=%s\n' "$(find "$labels" -maxdepth 1 -name '*.receipt.json' | wc -l)" >> "$run/run-receipt.txt"
printf 'pack_receipts=%s\n' "$(find "$packed" -maxdepth 1 -name '*.pack.json' | wc -l)" >> "$run/run-receipt.txt"
printf 'linux_free_after_bytes=%s\n' "$(df -B1 --output=avail / | tail -n 1 | tr -d ' ')" >> "$run/run-receipt.txt"
printf 'windows_c_free_after_bytes=%s\n' "$(/mnt/c/Windows/System32/WindowsPowerShell/v1.0/powershell.exe -NoProfile -Command '(Get-PSDrive C).Free' | tr -d '\r')" >> "$run/run-receipt.txt"
