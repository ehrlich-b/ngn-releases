#!/bin/sh
set -eu

worktree=/home/ehrli/repos/ngn-rodent-update-avx2-20260913
raw=$worktree/output/rodent-update-avx2-20260913/validation
inputs=/home/ehrli/repos/ngn-rodent-output-gate-20260913/run-001/inputs
mkdir -p "$raw"

run_stage() {
	name=$1
	shift
	set +e
	"$@" >"$raw/$name.stdout" 2>"$raw/$name.stderr"
	rc=$?
	set -e
	printf '%s\n' "$rc" >"$raw/$name.exit"
	if [ "$rc" -ne 0 ]; then
		exit "$rc"
	fi
}

run_stage focused-v1 \
	env CGO_ENABLED=0 GOAMD64=v1 GOMAXPROCS=2 GOFLAGS=-mod=readonly \
	/usr/local/go/bin/go -C "$worktree" test -p 2 ./rodenteval \
	-run '^TestRodentApplyUpdates' -count=1 -v

run_stage focused-v3 \
	env CGO_ENABLED=0 GOAMD64=v3 GOMAXPROCS=2 GOFLAGS=-mod=readonly \
	/usr/local/go/bin/go -C "$worktree" test -p 2 ./rodenteval \
	-run '^TestRodentApplyUpdates' -count=1 -v

run_stage oracle-v1 \
	env CGO_ENABLED=0 GOAMD64=v1 GOMAXPROCS=2 GOFLAGS=-mod=readonly \
	RODENT_V11_ANAND_MODEL="$inputs/model.bin" \
	RODENT_V11_ANAND_ORACLE_JSON="$inputs/release-oracle.json" \
	RODENT_V11_ANAND_TRANSITION_FIXTURES="$inputs/transition-fixtures.json" \
	RODENT_V11_ANAND_TAGGED_TRANSITION_ORACLE="$inputs/tagged-transition-oracle.json" \
	RODENT_V11_ANAND_RELEASE_TRANSITION_ORACLE="$inputs/release-transition-oracle.json" \
	/usr/local/go/bin/go -C "$worktree" test -p 2 -tags rodentoracle \
	./rodenteval -count=1 -v

run_stage oracle-v3 \
	env CGO_ENABLED=0 GOAMD64=v3 GOMAXPROCS=2 GOFLAGS=-mod=readonly \
	RODENT_V11_ANAND_MODEL="$inputs/model.bin" \
	RODENT_V11_ANAND_ORACLE_JSON="$inputs/release-oracle.json" \
	RODENT_V11_ANAND_TRANSITION_FIXTURES="$inputs/transition-fixtures.json" \
	RODENT_V11_ANAND_TAGGED_TRANSITION_ORACLE="$inputs/tagged-transition-oracle.json" \
	RODENT_V11_ANAND_RELEASE_TRANSITION_ORACLE="$inputs/release-transition-oracle.json" \
	/usr/local/go/bin/go -C "$worktree" test -p 2 -tags rodentoracle \
	./rodenteval -count=1 -v

run_stage engine-rodent-v3 \
	env CGO_ENABLED=0 GOAMD64=v3 GOMAXPROCS=2 GOFLAGS=-mod=readonly \
	RODENT_V11_ANAND_MODEL="$inputs/model.bin" \
	/usr/local/go/bin/go -C "$worktree" test -p 2 -tags rodentoracle \
	./engine -run Rodent -count=1 -v

run_stage full-short-v3 \
	env CGO_ENABLED=0 GOAMD64=v3 GOMAXPROCS=2 GOFLAGS=-mod=readonly \
	/usr/local/go/bin/go -C "$worktree" test -short -p 2 ./... -count=1

run_stage full-short-race-v3 \
	env GOAMD64=v3 GOMAXPROCS=2 GOFLAGS=-mod=readonly \
	/usr/local/go/bin/go -C "$worktree" test -short -race -p 2 ./... -count=1
