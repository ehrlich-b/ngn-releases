#!/bin/sh
set -eu

output=/home/ehrli/repos/ngn-rodent-postoutput-profile-20260913/output/rodent-postoutput-profile-20260913
binary=$output/inputs/engine-rodent-postoutput-profile.test
mkdir -p "$output/inputs"

set +e
env CGO_ENABLED=0 GOAMD64=v3 GOMAXPROCS=2 GOFLAGS=-mod=readonly \
    /usr/local/go/bin/go -C /home/ehrli/repos/ngn-rodent-postoutput-profile-20260913 \
    test -p 2 -c -o "$binary" ./engine \
    >"$output/build.stdout" 2>"$output/build.stderr"
rc=$?
set -e
printf '%s\n' "$rc" >"$output/build.exit"
exit "$rc"
