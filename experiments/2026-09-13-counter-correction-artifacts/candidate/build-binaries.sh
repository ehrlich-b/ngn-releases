#!/usr/bin/env bash
set -euo pipefail

base=/home/ehrli/repos/ngn-counter-minor-off-base-20260913
candidate=/home/ehrli/repos/ngn-counter-minor-off-20260913
out="$candidate/output/counter-minor-off-20260913"

env CGO_ENABLED=0 GOAMD64=v3 GOMAXPROCS=2 GOTOOLCHAIN=local GOFLAGS=-mod=readonly \
  /usr/local/go/bin/go build -C "$base" -p 2 \
  -o "$out/binaries/ngn-counter-minor-base-270c735" . 2>&1 | tee "$out/logs/build-base-v3.log"
env CGO_ENABLED=0 GOAMD64=v3 GOMAXPROCS=2 GOTOOLCHAIN=local GOFLAGS=-mod=readonly \
  /usr/local/go/bin/go build -C "$candidate" -p 2 \
  -o "$out/binaries/ngn-counter-minor-off-270c735" . 2>&1 | tee "$out/logs/build-candidate-v3.log"

/usr/local/go/bin/go version | tee "$out/logs/go-version.log"
sha256sum "$out/binaries/ngn-counter-minor-base-270c735" "$out/binaries/ngn-counter-minor-off-270c735" | tee "$out/receipts/binary-sha256.txt"
stat -c '%n %s bytes mode=%a' "$out/binaries/ngn-counter-minor-base-270c735" "$out/binaries/ngn-counter-minor-off-270c735" | tee "$out/receipts/binary-stat.txt"
