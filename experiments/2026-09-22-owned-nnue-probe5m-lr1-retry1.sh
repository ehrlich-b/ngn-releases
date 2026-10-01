#!/usr/bin/env bash
set -Eeuo pipefail
umask 022

root=/home/ehrli/nnue-owned-k4-20260920
run="$root/runs/k4-probe5m-lr1-20260922"
archive="$root/runs/k4-probe5m-lr1-20260922-attempt0-failed"
original=/mnt/c/Users/ehrli/ngn-k4-probe5m-lr1-postpanel.sh

[[ -d "$run" && ! -e "$archive" ]]
shopt -s nullglob dotglob
contents=("$run"/*)
[[ ${#contents[@]} == 1 && ${contents[0]} == "$run/STATE" ]]
[[ $(cat "$run/STATE") == FAILED ]]
[[ $(sha256sum "$original" | cut -d' ' -f1) == 5bb4044c8c5d83b2111d6cb25dfca78af0be0d568d673f098bec205420585db5 ]]
mv "$run" "$archive"
exec /bin/bash "$original"
