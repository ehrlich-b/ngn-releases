#!/usr/bin/env bash
set -Eeuo pipefail
umask 022

root=/home/ehrli/nnue-owned-k4-20260920
run="$root/runs/k4-probe5m-20260921"
pilot="$root/runs/k4-production-pilot-20260921"
bin="$root/bin-probe5m"
finalizer="$bin/ngnk4finalize"
bridge="$bin/ngnk4bridge"
trainer="$root/bullet-exec-probe5m-v1/target/release/examples/ngn_k4_train"
static="$root/bin-static-s1/ngnk4staticdump"
analysis=/mnt/c/Users/ehrli/ngnk4static-analyze-s1.py
rodent=/home/ehrli/repos/ngn-smp-depth-stagger-gate-20260918/run-001/inputs/rodent_anand_512hl.bin

[[ -d "$run" && -f "$run/POSTLABEL_STATE" ]]
[[ $(cat "$run/STATE") == LABELED && $(cat "$run/POSTLABEL_STATE") == FAILED ]]
[[ ! -e "$run/POSTLABEL_STATE.attempt0-failed" ]]
[[ ! -e "$bin" && ! -e "$run/finalized-probe5m" && ! -e "$run/training-probe5m" ]]
cp -p "$run/POSTLABEL_STATE" "$run/POSTLABEL_STATE.attempt0-failed"
printf 'WAITING_FOR_LABELS\n' > "$run/POSTLABEL_STATE"
trap 'printf "FAILED\n" > "$run/POSTLABEL_STATE"' ERR TERM

# One kernel-notification wait, with a bounded deadline. No polling loop.
python3 - "$run/STATE" <<'PY'
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
deadline = time.monotonic() + 7 * 3600 + 300
while True:
    state = path.read_text().strip()
    if state == 'LABELED':
        break
    if state == 'FAILED':
        raise RuntimeError('labeling failed')
    remaining_ms = int((deadline - time.monotonic()) * 1000)
    if remaining_ms <= 0 or not poller.poll(remaining_ms):
        raise TimeoutError('labeling completion notification did not arrive')
    os.read(fd, 4096)
os.close(fd)
PY

[[ $(sha256sum /mnt/c/Users/ehrli/ngnk4finalize-probe5m | cut -d' ' -f1) == 3c22fb56f119cab21f8581e89193303f728d8b37452f9cf857ab100f19f23908 ]]
[[ $(sha256sum /mnt/c/Users/ehrli/ngnk4bridge-probe5m | cut -d' ' -f1) == 5cb49c97c133f73011b2e6917669bec833510ec4d14cb411ed1321e7ba5cd035 ]]
[[ $(sha256sum "$trainer" | cut -d' ' -f1) == 6a989de4ca6746b2f7c10938206ed3a2e89d1c50beef580ff08399cafa805f0b ]]
[[ $(sha256sum "$static" | cut -d' ' -f1) == 24cf5ed4370773a57baf34568050bc5962f4d7e821a961df85e2139220d084fb ]]
[[ $(sha256sum "$analysis" | cut -d' ' -f1) == 23498dde5a593e9237578acb33cfbcd9dc424ab2daa4e539afd9b9c96623fb02 ]]
[[ ! -e "$bin" && ! -e "$run/finalized-probe5m" && ! -e "$run/training-probe5m" ]]

accepted=$(python3 - "$run/packed" <<'PY'
import json
from pathlib import Path
import sys
files = sorted(Path(sys.argv[1]).glob('main-expansion-train-*.pack.json'))
assert len(files) == 50, f'expected 50 pack receipts, got {len(files)}'
print(sum(json.loads(p.read_text())['accepted'] for p in files))
PY
)
(( accepted >= 4000000 ))
mkdir "$bin"
install -m 755 /mnt/c/Users/ehrli/ngnk4finalize-probe5m "$finalizer"
install -m 755 /mnt/c/Users/ehrli/ngnk4bridge-probe5m "$bridge"
{
  printf 'labeling_ended_utc=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  printf 'accepted_in_first_50_shards=%s\n' "$accepted"
  printf 'finalizer_sha256=%s\n' "$(sha256sum "$finalizer" | cut -d' ' -f1)"
  printf 'bridge_sha256=%s\n' "$(sha256sum "$bridge" | cut -d' ' -f1)"
  printf 'trainer_sha256=%s\n' "$(sha256sum "$trainer" | cut -d' ' -f1)"
} > "$run/postlabel-receipt.txt"

printf 'FINALIZING\n' > "$run/POSTLABEL_STATE"
taskset -c 12 nice -n 10 timeout 1800 "$finalizer" probe5m \
  -sampler "$pilot/selection/manifest.json" \
  -pilot "$pilot/finalized-pilot/manifest.json" \
  -labels "$run/labels" -packed "$run/packed" \
  -output "$run/finalized-probe5m" \
  > "$run/finalizer.stdout.json" 2> "$run/finalizer.stderr.txt"
[[ $(cat "$run/finalized-probe5m/STATE") == COMPLETE ]]
manifest="$run/finalized-probe5m/manifest.json"
printf 'finalized_manifest_sha256=%s\n' "$(sha256sum "$manifest" | cut -d' ' -f1)" >> "$run/postlabel-receipt.txt"

printf 'TRAINING\n' > "$run/POSTLABEL_STATE"
[[ $(systemctl --user is-active arli-hopper.service) == active ]]
[[ -z $(nvidia-smi --query-compute-apps=pid --format=csv,noheader) ]]
linux_free=$(df -B1 --output=avail / | tail -n 1 | tr -d ' ')
windows_free=$(/mnt/c/Windows/System32/WindowsPowerShell/v1.0/powershell.exe -NoProfile -Command '(Get-PSDrive C).Free' | tr -d '\r')
(( linux_free >= 40 * 1024 * 1024 * 1024 ))
(( windows_free >= 60 * 1024 * 1024 * 1024 ))
printf 'linux_free_before_training_bytes=%s\nwindows_c_free_before_training_bytes=%s\n' "$linux_free" "$windows_free" >> "$run/postlabel-receipt.txt"
export LD_LIBRARY_PATH=/usr/lib/wsl/lib:/home/ehrli/nnue-public-toolchain-20260906/install/cuda-12.8.1/lib64
taskset -c 12-15 nice -n 10 timeout 1800 "$trainer" start probe5m "$manifest" "$run/training-probe5m" \
  > "$run/training.log" 2>&1
[[ -f "$run/training-probe5m/completion.json" ]]
printf 'training_completion_sha256=%s\n' "$(sha256sum "$run/training-probe5m/completion.json" | cut -d' ' -f1)" >> "$run/postlabel-receipt.txt"

printf 'SELECTING\n' > "$run/POSTLABEL_STATE"
taskset -c 12-15 nice -n 10 timeout 1800 "$bridge" select \
  -mode probe5m \
  -candidates "$run/training-probe5m/candidates" \
  -manifest "$manifest" \
  -validation "$pilot/finalized-pilot/validation.bf" \
  -out "$run/selection-probe5m.json" \
  > /dev/null 2> "$run/selection.stderr.txt"
update=$(jq -r .selected_update "$run/selection-probe5m.json")
[[ "$update" =~ ^[0-9]+$ ]]
printf 'selection_sha256=%s\nselected_update=%s\n' \
  "$(sha256sum "$run/selection-probe5m.json" | cut -d' ' -f1)" "$update" >> "$run/postlabel-receipt.txt"

printf 'CALIBRATING\n' > "$run/POSTLABEL_STATE"
quant="$run/training-probe5m/candidates/candidate-$update/quantised.bin"
model="$run/model-probe5m-selected.nnue"
"$bridge" convert -in "$quant" -manifest "$manifest" -out "$model" > "$run/convert-probe5m.json"
model_sha=$(sha256sum "$model" | cut -d' ' -f1)
mkdir "$run/static-probe5m-selected"
taskset -c 12 nice -n 10 timeout 900 "$static" \
  -manifest "$pilot/finalized-pilot/manifest.json" \
  -manifest-sha256 753e8ed1fbe09bef290fcba7209dfb77dce0170e80a5a09a22a1c3c12599755e \
  -k4-model "$model" -k4-sha256 "$model_sha" -rodent-model "$rodent" \
  -output "$run/static-probe5m-selected/predictions.csv" \
  -receipt "$run/static-probe5m-selected/receipt.json" \
  > "$run/static-probe5m-selected/stdout.json"
taskset -c 12 nice -n 10 timeout 900 python3 "$analysis" \
  --csv "$run/static-probe5m-selected/predictions.csv" \
  --receipt "$run/static-probe5m-selected/receipt.json" \
  --output "$run/static-probe5m-selected/analysis.json"
{
  printf 'model_sha256=%s\n' "$model_sha"
  printf 'static_analysis_sha256=%s\n' "$(sha256sum "$run/static-probe5m-selected/analysis.json" | cut -d' ' -f1)"
  printf 'ended_utc=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
} >> "$run/postlabel-receipt.txt"
printf 'CALIBRATED\n' > "$run/POSTLABEL_STATE"
jq '.selected_update as $u | {selected_update, early_stop, selected_integer_mse:(.candidates[] | select(.update == $u) | .integer_mean_mse)}' "$run/selection-probe5m.json"
jq '.groups.overall.all | {k4, hce, k4_minus_hce}' "$run/static-probe5m-selected/analysis.json"
