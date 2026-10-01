#!/usr/bin/env bash
set -euo pipefail
umask 022

root=/home/ehrli/nnue-owned-k4-20260920
pilot="$root/runs/k4-production-pilot-20260921"
labels="$root/runs/k4-main20m-label-20260922"
run="$root/runs/k4-main-m1-lr1-20260922"
finalizer="$root/bin-probe5m/ngnk4finalize"
bridge="$root/bin-main-m1-lr1-v1/ngnk4bridge"
trainer="$root/bullet-exec-main-m1-lr1-v1/target/release/examples/ngn_k4_train"
static="$root/bin-static-s1/ngnk4staticdump"
analysis=/mnt/c/Users/ehrli/ngnk4static-analyze-s1.py
rodent=/home/ehrli/repos/ngn-smp-depth-stagger-gate-20260918/run-001/inputs/rodent_anand_512hl.bin

[[ -d "$labels" && ! -e "$run" ]]
[[ $(cat "$root/runs/k4-main-m1-lr1-build-20260922-retry1/STATE") == BUILT ]]
mkdir "$run"
printf 'WAITING_FOR_LABELS\n' > "$run/STATE"
trap 'printf "FAILED\n" > "$run/STATE"' ERR TERM

# Kernel notifications avoid periodic SSH or shell polling during labeling.
python3 - "$labels/STATE" <<'PY'
import ctypes
import os
from pathlib import Path
import select
import subprocess
import sys
import time

path = Path(sys.argv[1])
libc = ctypes.CDLL(None, use_errno=True)
fd = libc.inotify_init1(os.O_CLOEXEC)
if fd < 0:
    raise OSError(ctypes.get_errno(), 'inotify_init1')
wd = libc.inotify_add_watch(fd, os.fsencode(path), 0x00000008 | 0x00000400)
if wd < 0:
    raise OSError(ctypes.get_errno(), 'inotify_add_watch')
poller = select.poll()
poller.register(fd, select.POLLIN)
deadline = time.monotonic() + 25 * 3600
while True:
    state = path.read_text().strip()
    if state == 'LABELED':
        break
    if state == 'FAILED':
        active = subprocess.run(
            ['systemctl', '--user', 'is-active', 'ngn-k4-main20m-label-20260922.service'],
            capture_output=True, text=True, check=False,
        ).stdout.strip()
        if active != 'active':
            raise RuntimeError('20M labeling service failed')
    remaining_ms = int((deadline - time.monotonic()) * 1000)
    if remaining_ms <= 0 or not poller.poll(min(remaining_ms, 300_000 if state == 'FAILED' else remaining_ms)):
        if state == 'FAILED' and remaining_ms > 0:
            continue
        raise TimeoutError('20M labeling completion notification did not arrive')
    os.read(fd, 4096)
os.close(fd)
PY

[[ $(cat "$labels/STATE") == LABELED ]]
[[ $(sha256sum "$pilot/selection/manifest.json" | cut -d' ' -f1) == 53d240d4948a165a46fa42875f825edef79eb0f36a33e580a27cb562c56d4138 ]]
[[ $(sha256sum "$pilot/finalized-pilot/manifest.json" | cut -d' ' -f1) == 753e8ed1fbe09bef290fcba7209dfb77dce0170e80a5a09a22a1c3c12599755e ]]
[[ $(sha256sum "$labels/queue.tsv" | cut -d' ' -f1) == f7222c1972d0e7010756acb75100e1381c574bd0835d682d034daec51a2aa78c ]]
[[ $(sha256sum "$finalizer" | cut -d' ' -f1) == 3c22fb56f119cab21f8581e89193303f728d8b37452f9cf857ab100f19f23908 ]]
[[ $(sha256sum "$bridge" | cut -d' ' -f1) == 3c9e9c9df636968c6a3a0dd61d5f9cd4ae991f0aebc221170f24b10cbbcda906 ]]
[[ $(sha256sum "$trainer" | cut -d' ' -f1) == 06fd95b42fe85f7b7c0999cbc6089666627881d638f1270466925d715dbdf114 ]]
[[ $(sha256sum "$static" | cut -d' ' -f1) == 24cf5ed4370773a57baf34568050bc5962f4d7e821a961df85e2139220d084fb ]]
[[ $(sha256sum "$analysis" | cut -d' ' -f1) == 23498dde5a593e9237578acb33cfbcd9dc424ab2daa4e539afd9b9c96623fb02 ]]
[[ $(systemctl --user is-active arli-hopper.service) == active ]]
! pgrep -x fastchess > /dev/null
linux_free=$(df -B1 --output=avail / | tail -n 1 | tr -d ' ')
windows_free=$(/mnt/c/Windows/System32/WindowsPowerShell/v1.0/powershell.exe -NoProfile -Command '(Get-PSDrive C).Free' | tr -d '\r')
(( linux_free >= 40 * 1024 * 1024 * 1024 ))
(( windows_free >= 60 * 1024 * 1024 * 1024 ))
{
  printf 'started_utc=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  printf 'label_run_receipt_sha256=%s\n' "$(sha256sum "$labels/run-receipt.txt" | cut -d' ' -f1)"
  printf 'trainer_sha256=%s\n' 06fd95b42fe85f7b7c0999cbc6089666627881d638f1270466925d715dbdf114
  printf 'bridge_sha256=%s\n' 3c9e9c9df636968c6a3a0dd61d5f9cd4ae991f0aebc221170f24b10cbbcda906
  printf 'linux_free_before_bytes=%s\nwindows_c_free_before_bytes=%s\n' "$linux_free" "$windows_free"
} > "$run/receipt.txt"

printf 'FINALIZING\n' > "$run/STATE"
taskset -c 12 nice -n 10 timeout 2h "$finalizer" main \
  -sampler "$pilot/selection/manifest.json" \
  -pilot "$pilot/finalized-pilot/manifest.json" \
  -labels "$labels/labels" -packed "$labels/packed" \
  -output "$run/finalized-main" \
  > "$run/finalizer.stdout.json" 2> "$run/finalizer.stderr.txt"
[[ $(cat "$run/finalized-main/STATE") == COMPLETE ]]
manifest="$run/finalized-main/manifest.json"
printf 'finalized_manifest_sha256=%s\ntrain_bf_sha256=%s\n' \
  "$(sha256sum "$manifest" | cut -d' ' -f1)" \
  "$(sha256sum "$run/finalized-main/train-main.bf" | cut -d' ' -f1)" >> "$run/receipt.txt"

[[ $(systemctl --user is-active arli-hopper.service) == active ]]
[[ -z $(nvidia-smi --query-compute-apps=pid --format=csv,noheader) ]]
! pgrep -x fastchess > /dev/null
linux_free=$(df -B1 --output=avail / | tail -n 1 | tr -d ' ')
windows_free=$(/mnt/c/Windows/System32/WindowsPowerShell/v1.0/powershell.exe -NoProfile -Command '(Get-PSDrive C).Free' | tr -d '\r')
(( linux_free >= 40 * 1024 * 1024 * 1024 ))
(( windows_free >= 60 * 1024 * 1024 * 1024 ))
printf 'linux_free_before_training_bytes=%s\nwindows_c_free_before_training_bytes=%s\n' \
  "$linux_free" "$windows_free" >> "$run/receipt.txt"

printf 'TRAINING\n' > "$run/STATE"
export LD_LIBRARY_PATH=/usr/lib/wsl/lib:/home/ehrli/nnue-public-toolchain-20260906/install/cuda-12.8.1/lib64
taskset -c 12-15 nice -n 10 timeout 8h "$trainer" start main-m1-lr1 "$manifest" "$run/training" \
  > "$run/training.log" 2>&1
[[ -f "$run/training/completion.json" ]]
printf 'training_completion_sha256=%s\n' "$(sha256sum "$run/training/completion.json" | cut -d' ' -f1)" >> "$run/receipt.txt"

printf 'SELECTING\n' > "$run/STATE"
taskset -c 12-15 nice -n 10 timeout 2h "$bridge" select \
  -mode main-m1-lr1 \
  -candidates "$run/training/candidates" \
  -manifest "$manifest" \
  -validation "$pilot/finalized-pilot/validation.bf" \
  -out "$run/selection.json" \
  > "$run/selection.stdout.txt" 2> "$run/selection.stderr.txt"
update=$(jq -r .selected_update "$run/selection.json")
[[ "$update" =~ ^[0-9]+$ ]]
printf 'selection_sha256=%s\nselected_update=%s\n' \
  "$(sha256sum "$run/selection.json" | cut -d' ' -f1)" "$update" >> "$run/receipt.txt"

printf 'CALIBRATING\n' > "$run/STATE"
quant="$run/training/candidates/candidate-$update/quantised.bin"
model="$run/model-selected.nnue"
"$bridge" convert -in "$quant" -manifest "$manifest" -out "$model" > "$run/convert.json"
model_sha=$(sha256sum "$model" | cut -d' ' -f1)
mkdir "$run/static"
taskset -c 12 nice -n 10 timeout 900 "$static" \
  -manifest "$pilot/finalized-pilot/manifest.json" \
  -manifest-sha256 753e8ed1fbe09bef290fcba7209dfb77dce0170e80a5a09a22a1c3c12599755e \
  -k4-model "$model" -k4-sha256 "$model_sha" -rodent-model "$rodent" \
  -output "$run/static/predictions.csv" \
  -receipt "$run/static/receipt.json" \
  > "$run/static/stdout.json"
taskset -c 12 nice -n 10 timeout 900 python3 "$analysis" \
  --csv "$run/static/predictions.csv" \
  --receipt "$run/static/receipt.json" \
  --output "$run/static/analysis.json"
{
  printf 'model_sha256=%s\n' "$model_sha"
  printf 'static_analysis_sha256=%s\n' "$(sha256sum "$run/static/analysis.json" | cut -d' ' -f1)"
  printf 'ended_utc=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
} >> "$run/receipt.txt"
printf 'CALIBRATED\n' > "$run/STATE"
