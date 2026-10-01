#!/bin/sh
set -eu

repo=/home/ehrli/repos/ngn-counter-fused-20260913
out=$repo/output/counter-fused-20260913
logs=$out/logs
receipts=$out/receipts
binary=$out/binaries/engine-fusion-candidate.test

cd "$repo"
git diff --binary -- \
	countereval/search_context.go \
	countereval/feature_updates_fallback.go \
	countereval/feature_updates_amd64_v3.go \
	countereval/feature_updates_amd64_v3.s >"$receipts/production.patch"
git diff --binary -- \
	countereval/feature_updates_amd64_v3_test.go \
	countereval/feature_updates_fallback_test.go \
	countereval/feature_updates_test.go \
	countereval/search_context_test.go >"$receipts/tests.patch"
git diff --binary >"$receipts/candidate.patch"

: >"$logs/fused-kernels-disassembly.log"
for symbol in copyAndUpdate2AVX2 copyAndUpdate3AVX2 copyAndUpdate4AVX2; do
	objdump -d -M intel \
		--disassemble="github.com/ehrlich-b/ngn/countereval.$symbol.abi0" \
		"$binary" >>"$logs/fused-kernels-disassembly.log"
done

sha256sum \
	countereval/search_context.go \
	countereval/feature_updates_fallback.go \
	countereval/feature_updates_amd64_v3.go \
	countereval/feature_updates_amd64_v3.s \
	countereval/feature_updates_amd64_v3_test.go \
	countereval/feature_updates_fallback_test.go \
	countereval/feature_updates_test.go \
	countereval/search_context_test.go >"$receipts/source-files.sha256"

sha256sum \
	"$receipts/production.patch" \
	"$receipts/tests.patch" \
	"$receipts/candidate.patch" \
	"$binary" \
	"$logs/fused-kernels-disassembly.log" \
	"$receipts/probe-inputs/counter_transition_profile_test.go.txt" \
	"$receipts/probe-inputs/counter_post_output_profile_test.go.txt" \
	"$receipts/probe-inputs/counter_profile_prefixes.json" \
	"$receipts/candidate-cold-snapshot.json" \
	"$receipts/candidate-persistent-snapshot.json" >"$receipts/frozen-artifacts.sha256"

sha256sum \
	"$logs/focused-v3.log" \
	"$logs/focused-v1.log" \
	"$logs/shortengine.log" \
	"$logs/raceengine.log" \
	"$logs/fullshort.log" \
	"$logs/counteroracle-v1.log" \
	"$logs/counteroracle-v3.log" \
	"$logs/frozen-snapshots.log" \
	"$logs/fullshort-evidence-layout-failure.log" >"$receipts/test-logs.sha256"

git rev-parse HEAD >"$receipts/base-head.txt"
git status --short >"$receipts/worktree-status.txt"
/usr/local/go/bin/go version >"$receipts/go-version.txt"
