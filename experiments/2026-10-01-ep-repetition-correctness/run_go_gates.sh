#!/usr/bin/env bash
set -euo pipefail

ROOT=$(git rev-parse --show-toplevel)
ART="$ROOT/experiments/2026-10-01-ep-repetition-correctness"
MARKER="$ROOT/../cpu-coordinated.json"
GO=/usr/local/go/bin/go

python3 - "$MARKER" <<'PY'
import json, pathlib, sys
path = pathlib.Path(sys.argv[1])
if not path.is_file():
    raise SystemExit(f"CPU coordination marker absent: {path}")
data = json.loads(path.read_text())
if data.get("approved") is not True:
    raise SystemExit(f"CPU coordination is not approved: {data!r}")
PY

export GOMAXPROCS=1
export GOFLAGS=-p=1

{
    date -u --iso-8601=seconds
    printf 'head=%s\n' "$(git rev-parse HEAD)"
    printf 'branch=%s\n' "$(git branch --show-current)"
    printf 'go=%s\n' "$($GO version)"
    printf 'GOMAXPROCS=%s\nGOFLAGS=%s\n' "$GOMAXPROCS" "$GOFLAGS"
    printf '%s\n' '--- cpu coordination marker ---'
    cat "$MARKER"
    printf '%s\n' '--- process controls ---'
    ps -o pid,ppid,ni,psr,comm -p $$ -p $PPID
    grep -E '^(Cpus_allowed_list|Mems_allowed_list):' /proc/self/status
    cat /proc/self/cgroup
    printf '%s\n' '--- cgroup limits ---'
    CGROUP=$(awk -F: '$1=="0" {print $3}' /proc/self/cgroup)
    for item in cpu.max memory.max; do
        path="/sys/fs/cgroup${CGROUP}/$item"
        if test -r "$path"; then printf '%s=' "$item"; cat "$path"; fi
    done
} > "$ART/go-environment.txt"

run() {
    name=$1
    shift
    commandline=$(printf '%q ' "$@")
    printf '%s\n' "${commandline% }" > "$ART/$name.command"
    set +e
    "$@" > "$ART/$name.stdout" 2> "$ART/$name.stderr"
    code=$?
    set -e
    printf '%s\n' "$code" > "$ART/$name.exit"
    if test "$code" -ne 0; then
        echo "$name failed with exit $code" >&2
        exit "$code"
    fi
}

HEAD=$(git rev-parse HEAD)
run ngn-baseline-probe "$GO" run "$ART/ngn_baseline_probe.go" "$HEAD"
run go-test-short-engine "$GO" test -short ./engine -count=1
run go-test-short-race-engine "$GO" test -short -race ./engine -count=1
run go-test-short-all "$GO" test -short ./... -count=1

date -u --iso-8601=seconds > "$ART/go-gates-finished.txt"
