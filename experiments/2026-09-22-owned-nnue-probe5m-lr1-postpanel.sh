#!/usr/bin/env bash
set -Eeuo pipefail
umask 022

root=/home/ehrli/nnue-owned-k4-20260920
parent="$root/runs/k4-probe5m-20260921"
pilot="$root/runs/k4-production-pilot-20260921"
run="$root/runs/k4-probe5m-lr1-20260922"
manifest="$parent/finalized-probe5m/manifest.json"
trainer="$root/bullet-exec-probe5m-lr1-v1/target/release/examples/ngn_k4_train"
bridge_stage=/mnt/c/Users/ehrli/ngnk4bridge-probe5m-lr1-v1
static="$root/bin-static-s1/ngnk4staticdump"
analysis=/mnt/c/Users/ehrli/ngnk4static-analyze-s1.py
rodent=/home/ehrli/repos/ngn-smp-depth-stagger-gate-20260918/run-001/inputs/rodent_anand_512hl.bin

[[ -d "$parent" && ! -e "$run" ]]
mkdir "$run"
printf 'WAITING_FOR_ORIGINAL_PANEL\n' > "$run/STATE"
trap 'printf "FAILED\n" > "$run/STATE"' ERR TERM

# An event-driven wait lets the original 5M training, calibration and search
# finish before this independent CUDA job uses the same host.
python3 - "$parent/PANEL_STATE" <<'PY'
import ctypes
import os
from pathlib import Path
import select
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
deadline = time.monotonic() + 9 * 3600
while True:
    state = path.read_text().strip()
    if state == 'PANEL_COMPLETE':
        break
    if state == 'FAILED':
        raise RuntimeError('original 5M panel failed')
    remaining_ms = int((deadline - time.monotonic()) * 1000)
    if remaining_ms <= 0 or not poller.poll(remaining_ms):
        raise TimeoutError('original 5M panel completion did not arrive')
    os.read(fd, 4096)
os.close(fd)
PY

[[ $(cat "$parent/finalized-probe5m/STATE") == COMPLETE ]]
manifest_sha=$(sha256sum "$manifest" | cut -d' ' -f1)
grep -Fx "finalized_manifest_sha256=$manifest_sha" "$parent/postlabel-receipt.txt" > /dev/null
[[ $(sha256sum "$trainer" | cut -d' ' -f1) == 5b22d6b763ffc20586cc8d1e0669bdbe1587b5cf6990332fb7b5d0b605132afc ]]
[[ $(sha256sum "$bridge_stage" | cut -d' ' -f1) == 29bad4d5beb8b92140233d18fb400702cb0fd7e4f365666aeb35e703aa46218f ]]
[[ $(sha256sum "$static" | cut -d' ' -f1) == 24cf5ed4370773a57baf34568050bc5962f4d7e821a961df85e2139220d084fb ]]
[[ $(sha256sum "$analysis" | cut -d' ' -f1) == 23498dde5a593e9237578acb33cfbcd9dc424ab2daa4e539afd9b9c96623fb02 ]]
[[ $(systemctl --user is-active arli-hopper.service) == active ]]
[[ -z $(nvidia-smi --query-compute-apps=pid --format=csv,noheader) ]]
! pgrep -x fastchess > /dev/null
linux_free=$(df -B1 --output=avail / | tail -n 1 | tr -d ' ')
windows_free=$(/mnt/c/Windows/System32/WindowsPowerShell/v1.0/powershell.exe -NoProfile -Command '(Get-PSDrive C).Free' | tr -d '\r')
(( linux_free >= 40 * 1024 * 1024 * 1024 ))
(( windows_free >= 60 * 1024 * 1024 * 1024 ))
install -m 755 "$bridge_stage" "$run/ngnk4bridge"
{
  printf 'started_utc=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  printf 'finalized_manifest_sha256=%s\n' "$manifest_sha"
  printf 'train_bf_sha256=%s\n' "$(sha256sum "$parent/finalized-probe5m/train-probe5m.bf" | cut -d' ' -f1)"
  printf 'trainer_sha256=%s\n' 5b22d6b763ffc20586cc8d1e0669bdbe1587b5cf6990332fb7b5d0b605132afc
  printf 'bridge_sha256=%s\n' 29bad4d5beb8b92140233d18fb400702cb0fd7e4f365666aeb35e703aa46218f
  printf 'linux_free_before_bytes=%s\nwindows_c_free_before_bytes=%s\n' "$linux_free" "$windows_free"
} > "$run/receipt.txt"

printf 'TRAINING\n' > "$run/STATE"
export LD_LIBRARY_PATH=/usr/lib/wsl/lib:/home/ehrli/nnue-public-toolchain-20260906/install/cuda-12.8.1/lib64
taskset -c 12-15 nice -n 10 timeout 1800 "$trainer" start probe5m-lr1 "$manifest" "$run/training" \
  > "$run/training.log" 2>&1
[[ -f "$run/training/completion.json" ]]
printf 'training_completion_sha256=%s\n' "$(sha256sum "$run/training/completion.json" | cut -d' ' -f1)" >> "$run/receipt.txt"

printf 'SELECTING\n' > "$run/STATE"
taskset -c 12-15 nice -n 10 timeout 1800 "$run/ngnk4bridge" select \
  -mode probe5m-lr1 \
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
"$run/ngnk4bridge" convert -in "$quant" -manifest "$manifest" -out "$model" > "$run/convert.json"
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
jq '.selected_update as $u | {selected_update, early_stop, selected_integer_mse:(.candidates[] | select(.update == $u) | .integer_mean_mse)}' "$run/selection.json"
jq '.groups.overall.all | {k4, hce, k4_minus_hce}' "$run/static/analysis.json"
