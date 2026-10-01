#!/usr/bin/env bash
set -eu

root=/home/ehrli/rodent-v1.1-anand-oracle-20260913
source_dir="$root/source-v3"
run="$root/run-003"
driver="$source_dir/rodent_oracle_driver_v3.py"
monitor=/home/ehrli/nnue-public-toolchain-20260906/src/ngn-data-pilot/process_tree_monitor.py

test -f "$driver"
test ! -e "$run"
mkdir -p "$run/tmp"

/usr/bin/taskset -c 6 /usr/bin/python3 "$monitor" \
  --label rodent-v1.1-anand-raw-oracle-v1 \
  --limit-seconds 60 --term-grace-seconds 3 --sample-interval-seconds 0.05 \
  --memory-limit-kib 1048576 --cpu-list 4 \
  --stdout "$run/driver.stdout" --stderr "$run/driver.stderr" \
  --time-output "$run/time.txt" --samples "$run/process-tree.jsonl" \
  --receipt "$run/supervisor-receipt.json" \
  -- /usr/bin/taskset -c 4 /usr/bin/env -i \
  PATH=/usr/local/go/bin:/usr/bin:/bin LANG=C LC_ALL=C TZ=UTC \
  GOMAXPROCS=1 TMPDIR="$run/tmp" \
  /usr/bin/python3 "$driver" --output "$run"

find "$source_dir" "$run" -type f ! -name FINAL_SHA256SUMS -print0 \
  | sort -z | xargs -0 sha256sum > "$run/FINAL_SHA256SUMS"
sha256sum "$run/FINAL_SHA256SUMS"
