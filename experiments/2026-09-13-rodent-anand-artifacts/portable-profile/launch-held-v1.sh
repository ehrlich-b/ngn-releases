#!/bin/sh
set -eu

run_dir=/home/ehrli/repos/ngn-rodent-profile-20260913/output/rodent-cold-profile-20260913/run-001
binary=/home/ehrli/repos/ngn-rodent-profile-20260913/output/rodent-cold-profile-20260913/inputs/engine-rodent-profile.test
model=/home/ehrli/rodent-v1.1-anand-sliceb-gate-20260913/run-002/inputs/network/rodent_anand_512hl.bin
supervisor=/home/ehrli/repos/ngn-rodent-profile-20260913/scripts/candidate-match/process_supervisor.py

test ! -e "$run_dir"
test -x "$binary"
test -f "$model"
test -f "$supervisor"
mkdir -p "$run_dir/tmp"
cd "$run_dir"

exec taskset -c 6 /usr/bin/python3 "$supervisor" \
  --label rodent-cold-profile-v1 \
  --limit-seconds 300 \
  --term-grace-seconds 3 \
  --sample-interval-seconds 0.25 \
  --memory-limit-kib 1048576 \
  --cpu-list 4 \
  --stdout "$run_dir/stdout" \
  --stderr "$run_dir/stderr" \
  --time-output "$run_dir/time.txt" \
  --samples "$run_dir/process-tree.jsonl" \
  --receipt "$run_dir/supervisor-receipt.json" \
  -- /usr/bin/env -i \
  PATH=/usr/local/go/bin:/usr/bin:/bin \
  LANG=C LC_ALL=C TZ=UTC \
  GOMAXPROCS=1 GOGC=100 GOMEMLIMIT=off GODEBUG= \
  TMPDIR="$run_dir/tmp" \
  RODENT_V11_ANAND_MODEL="$model" \
  "$binary" \
  -test.run=RodentProfileNoTestsSelected \
  -test.bench=BenchmarkRodentV11ColdFixedNodes \
  -test.benchtime=5x \
  -test.count=1 \
  -test.benchmem \
  -test.cpuprofile="$run_dir/cpu.pb.gz" \
  -test.memprofile="$run_dir/alloc.pb.gz" \
  -test.memprofilerate=524288
