#!/usr/bin/env bash
set -Eeuo pipefail
umask 022

root=/home/ehrli/nnue-owned-k4-20260920
pilot="$root/runs/k4-production-pilot-20260921"
manifest="$pilot/finalized-pilot/manifest.json"
run="$root/runs/k4-pilot-lr1-20260922-retry1"
trainer="$root/bullet-exec-pilot-lr1-v1/target/release/examples/ngn_k4_train"
bridge_stage=/mnt/c/Users/ehrli/ngnk4bridge-pilot-lr1-v1

[[ ! -e "$run" ]]
[[ $(sha256sum "$manifest" | cut -d' ' -f1) == 753e8ed1fbe09bef290fcba7209dfb77dce0170e80a5a09a22a1c3c12599755e ]]
[[ $(sha256sum "$pilot/finalized-pilot/train-pilot.bf" | cut -d' ' -f1) == 467d3876c6d670f66d6ac5986c00e3e1c7f34843ae87653c6c578bf1c06228e8 ]]
[[ $(sha256sum "$trainer" | cut -d' ' -f1) == 34d49b9dd175a15e622aa6ac8781817606914899f8b609976806424a0f14e187 ]]
[[ $(sha256sum "$bridge_stage" | cut -d' ' -f1) == 37da957bd6c3214357d9ea511a023503aa05c2e59664da21d75f20cda679119c ]]
[[ $(systemctl --user is-active arli-hopper.service) == active ]]
[[ -z $(nvidia-smi --query-compute-apps=pid --format=csv,noheader) ]]
linux_free=$(df -B1 --output=avail / | tail -n 1 | tr -d ' ')
windows_free=$(/mnt/c/Windows/System32/WindowsPowerShell/v1.0/powershell.exe -NoProfile -Command '(Get-PSDrive C).Free' | tr -d '\r')
(( linux_free >= 40 * 1024 * 1024 * 1024 ))
(( windows_free >= 60 * 1024 * 1024 * 1024 ))

mkdir "$run"
printf 'TRAINING\n' > "$run/STATE"
trap 'printf "FAILED\n" > "$run/STATE"' ERR TERM
install -m 755 "$bridge_stage" "$run/ngnk4bridge"
{
  printf 'started_utc=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  printf 'manifest_sha256=%s\n' 753e8ed1fbe09bef290fcba7209dfb77dce0170e80a5a09a22a1c3c12599755e
  printf 'train_bf_sha256=%s\n' 467d3876c6d670f66d6ac5986c00e3e1c7f34843ae87653c6c578bf1c06228e8
  printf 'trainer_sha256=%s\n' 34d49b9dd175a15e622aa6ac8781817606914899f8b609976806424a0f14e187
  printf 'bridge_sha256=%s\n' 37da957bd6c3214357d9ea511a023503aa05c2e59664da21d75f20cda679119c
  printf 'linux_free_before_bytes=%s\nwindows_c_free_before_bytes=%s\n' "$linux_free" "$windows_free"
} > "$run/receipt.txt"

export LD_LIBRARY_PATH=/usr/lib/wsl/lib:/home/ehrli/nnue-public-toolchain-20260906/install/cuda-12.8.1/lib64
taskset -c 12-15 nice -n 10 timeout 1800 "$trainer" start pilot-lr1 "$manifest" "$run/training" \
  > "$run/training.log" 2>&1
[[ -f "$run/training/completion.json" ]]
printf 'training_completion_sha256=%s\n' "$(sha256sum "$run/training/completion.json" | cut -d' ' -f1)" >> "$run/receipt.txt"

printf 'SELECTING\n' > "$run/STATE"
taskset -c 12-15 nice -n 10 timeout 1800 "$run/ngnk4bridge" select \
  -mode pilot-lr1 \
  -candidates "$run/training/candidates" \
  -manifest "$manifest" \
  -validation "$pilot/finalized-pilot/validation.bf" \
  -out "$run/selection.json" \
  > "$run/selection.stdout.txt" 2> "$run/selection.stderr.txt"
printf 'selection_sha256=%s\n' "$(sha256sum "$run/selection.json" | cut -d' ' -f1)" >> "$run/receipt.txt"
printf 'ended_utc=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" >> "$run/receipt.txt"
printf 'SELECTED\n' > "$run/STATE"
jq '.selected_update as $u | {selected_update, early_stop, selected_integer_mse:(.candidates[] | select(.update == $u) | .integer_mean_mse)}' "$run/selection.json"
