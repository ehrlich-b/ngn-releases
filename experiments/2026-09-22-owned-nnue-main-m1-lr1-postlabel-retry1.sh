#!/usr/bin/env bash
set -euo pipefail
umask 022

root=/home/ehrli/nnue-owned-k4-20260920
run="$root/runs/k4-main-m1-lr1-20260922"
archive="$root/runs/k4-main-m1-lr1-20260922-attempt0-failed"
original=/mnt/c/Users/ehrli/ngn-k4-main-m1-lr1-postlabel.sh

[[ $(systemctl --user is-active ngn-k4-main-m1-lr1-postlabel-20260922.service) == failed ]]
[[ -d "$run" && ! -e "$archive" ]]
[[ $(cat "$run/STATE") == FAILED ]]
shopt -s nullglob dotglob
contents=("$run"/*)
[[ ${#contents[@]} == 1 && ${contents[0]} == "$run/STATE" ]]
[[ $(sha256sum "$original" | cut -d' ' -f1) == 9d14bd27b8390019b67f50b09d41a540a6cf3d1059b4afc5322bbb52bd84715d ]]

mv "$run" "$archive"
exec /bin/bash "$original"
