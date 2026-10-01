#!/usr/bin/env bash
set -eu

output=/home/ehrli/repos/ngn-correction-profile-20260913/output/run-v1
monitor=/home/ehrli/nnue-public-toolchain-20260906/src/ngn-data-pilot/process_tree_monitor.py
binary=/home/ehrli/repos/ngn-correction-profile-20260913/output/correction-profile.test
model=/home/ehrli/repos/ngn-counter-pretrained-control/output/counter-pretrained-incremental-v6-attempt2/source/upstream/pkg/eval/nnue/n-30-5268.nn
prefixes=/home/ehrli/repos/ngn-counter-profile-20260913/engine/testdata/counter_profile_prefixes.json

test ! -e "$output"
mkdir -p "$output/tmp"
cd "$output"
/usr/bin/taskset -c 6 /usr/bin/python3 "$monitor" \
  --label counter-correction-profile-v1 \
  --limit-seconds 300 --term-grace-seconds 3 --sample-interval-seconds 0.25 \
  --memory-limit-kib 1048576 --cpu-list 4 \
  --stdout "$output/stdout.txt" --stderr "$output/stderr.txt" \
  --time-output "$output/time.txt" --samples "$output/samples.jsonl" --receipt "$output/supervisor-receipt.json" \
  -- /usr/bin/env -i \
  PATH=/usr/local/go/bin:/usr/bin:/bin LANG=C LC_ALL=C TZ=UTC \
  GOMAXPROCS=1 GOGC=100 GOMEMLIMIT=off GODEBUG= TMPDIR="$output/tmp" \
  NGN_COUNTER55_PROFILE_MODEL="$model" \
  NGN_COUNTER_PROFILE_PREFIXES="$prefixes" \
  NGN_COUNTER_CORRECTION_PROFILE_OUTPUT="$output/correction-profile.json" \
  "$binary" -test.run=TestCounterCorrectionHistoryProfile -test.v -test.count=1
