#!/bin/sh
set -eu

repo=/home/ehrli/repos/ngn-counter-fused-20260913
out=$repo/output/counter-fused-20260913
logs=$out/logs
receipts=$out/receipts
binary=$out/binaries/engine-fusion-candidate.test
go=/usr/local/go/bin/go
oracle_root=/home/ehrli/repos/ngn-counter-pretrained-control/output/counter-pretrained-incremental-v6-attempt2

run() {
	name=$1
	shift
	printf 'START %s\n' "$name"
	if "$@" >"$logs/$name.log" 2>&1; then
		echo 0 >"$logs/$name.exit"
		printf 'PASS %s\n' "$name"
	else
		status=$?
		echo "$status" >"$logs/$name.exit"
		printf 'FAIL %s exit=%s\n' "$name" "$status"
		exit "$status"
	fi
}

run fullshort env GOMAXPROCS=2 GOAMD64=v3 GOFLAGS=-mod=readonly \
	"$go" -C "$repo" test -p 2 -short -count=1 ./...

run counteroracle-v1 env GOMAXPROCS=2 GOTOOLCHAIN=local GOFLAGS=-mod=readonly CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOAMD64=v1 \
	COUNTER_MODEL="$oracle_root/source/upstream/pkg/eval/nnue/n-30-5268.nn" \
	COUNTER_FENS="$oracle_root/inputs/fixtures.tsv" \
	COUNTER_ORACLE_JSON="$oracle_root/artifacts/oracle.json" \
	COUNTER_TRANSITIONS="$oracle_root/inputs/transition_fixtures.tsv" \
	COUNTER_TRANSITION_ORACLE_JSON="$oracle_root/artifacts/transition-oracle.json" \
	"$go" test -C "$repo" -v -count=1 -p=2 -tags counteroracle ./countereval

run counteroracle-v3 env GOMAXPROCS=2 GOTOOLCHAIN=local GOFLAGS=-mod=readonly CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOAMD64=v3 \
	COUNTER_MODEL="$oracle_root/source/upstream/pkg/eval/nnue/n-30-5268.nn" \
	COUNTER_FENS="$oracle_root/inputs/fixtures.tsv" \
	COUNTER_ORACLE_JSON="$oracle_root/artifacts/oracle.json" \
	COUNTER_TRANSITIONS="$oracle_root/inputs/transition_fixtures.tsv" \
	COUNTER_TRANSITION_ORACLE_JSON="$oracle_root/artifacts/transition-oracle.json" \
	"$go" test -C "$repo" -v -count=1 -p=2 -tags counteroracle ./countereval

run frozen-snapshots env GOMAXPROCS=1 \
	NGN_COUNTER55_PROFILE_MODEL="$oracle_root/source/upstream/pkg/eval/nnue/n-30-5268.nn" \
	NGN_COUNTER55_PROFILE_OUTPUT="$receipts/candidate-cold-snapshot.json" \
	NGN_COUNTER_PROFILE_PREFIXES="$receipts/probe-inputs/counter_profile_prefixes.json" \
	NGN_COUNTER_PROFILE_OUTPUT="$receipts/candidate-persistent-snapshot.json" \
	"$binary" \
	-test.run '^(TestCounterTransitionFixedNodeSnapshot|TestCounterProfilePersistentSnapshot)$' \
	-test.count=1 -test.v
