#!/bin/sh
set -eu

worktree=/home/ehrli/repos/ngn-rodent-update-avx2-20260913
output=$worktree/output/rodent-update-avx2-20260913/packaging
mkdir -p "$output"

run_build() {
	name=$1
	goos=$2
	binary=$3
	set +e
	env CGO_ENABLED=0 GOOS="$goos" GOARCH=amd64 GOAMD64=v3 \
		GOMAXPROCS=2 GOFLAGS=-mod=readonly \
		/usr/local/go/bin/go -C "$worktree" build -p 2 -o "$binary" . \
		>"$output/$name.stdout" 2>"$output/$name.stderr"
	rc=$?
	set -e
	printf '%s\n' "$rc" >"$output/$name.exit"
	if [ "$rc" -ne 0 ]; then
		exit "$rc"
	fi
}

run_build linux-amd64-v3 linux "$output/ngn-rodent-update-linux-amd64-v3"
run_build windows-amd64-v3 windows "$output/ngn-rodent-update-windows-amd64-v3.exe"

sha256sum \
	"$output/ngn-rodent-update-linux-amd64-v3" \
	"$output/ngn-rodent-update-windows-amd64-v3.exe" \
	>"$output/binary-SHA256SUMS"
