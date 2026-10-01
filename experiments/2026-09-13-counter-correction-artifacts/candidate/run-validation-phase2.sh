#!/usr/bin/env bash
set -euo pipefail

repo=/home/ehrli/repos/ngn-counter-minor-off-20260913
out="$repo/output/counter-minor-off-20260913"
export GOMAXPROCS=2
export GOTOOLCHAIN=local
export GOFLAGS=-mod=readonly

run_logged() {
  local name=$1
  shift
  "$@" 2>&1 | tee "$out/logs/$name.log"
}

run_logged race-engine-v3 env CGO_ENABLED=1 GOAMD64=v3 /usr/local/go/bin/go test -C "$repo" -short -race -count=1 -p=2 ./engine
run_logged short-all-v3 env CGO_ENABLED=0 GOAMD64=v3 /usr/local/go/bin/go test -C "$repo" -short -count=1 -p=2 ./...

oracle=/home/ehrli/repos/ngn-counter-pretrained-control/output/counter-pretrained-incremental-v6-attempt2
for amd in v1 v3; do
  run_logged "counter-oracle-$amd" env \
    COUNTER_MODEL="$oracle/source/upstream/pkg/eval/nnue/n-30-5268.nn" \
    COUNTER_FENS="$oracle/inputs/fixtures.tsv" \
    COUNTER_ORACLE_JSON="$oracle/artifacts/oracle.json" \
    COUNTER_TRANSITIONS="$oracle/inputs/transition_fixtures.tsv" \
    COUNTER_TRANSITION_ORACLE_JSON="$oracle/artifacts/transition-oracle.json" \
    CGO_ENABLED=0 GOAMD64="$amd" /usr/local/go/bin/go test -C "$repo" -v -count=1 -p=2 -tags counteroracle ./countereval
done

printf 'VALIDATION_COMPLETE\n' | tee "$out/receipts/validation-complete.txt"
