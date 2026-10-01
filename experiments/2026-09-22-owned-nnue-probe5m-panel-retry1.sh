#!/usr/bin/env bash
set -Eeuo pipefail
umask 022

root=/home/ehrli/nnue-owned-k4-20260920
run="$root/runs/k4-probe5m-20260921"
original=/mnt/c/Users/ehrli/ngn-k4-probe5m-panel.sh
state="$run/PANEL_STATE"

[[ -d "$run" && -f "$state" && $(cat "$state") == FAILED ]]
[[ ! -e "$state.attempt0-failed" && ! -e "$run/search-probe5m" ]]
[[ $(sha256sum "$original" | cut -d' ' -f1) == b2ffd37a948ff99d661dc1f0ac9a161ae403c5c7f379bc7b5ae8f1f34e0abf24 ]]
mv "$state" "$state.attempt0-failed"
exec /bin/bash "$original"
