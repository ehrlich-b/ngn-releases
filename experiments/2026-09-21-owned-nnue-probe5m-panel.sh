#!/usr/bin/env bash
set -Eeuo pipefail
umask 022

root=/home/ehrli/nnue-owned-k4-20260920
run="$root/runs/k4-probe5m-20260921"
panel_root="$root/runs/k4-search-panel-20260921"
next="$run/search-probe5m"
original="$panel_root/panel.json"
runner="$panel_root/run_panel.py"
engine="$root/runs/k4-avx2-search-20260921/ngn-k4-avx2-v1"
model="$run/model-probe5m-selected.nnue"
rodent=/home/ehrli/repos/ngn-smp-depth-stagger-gate-20260918/run-001/inputs/rodent_anand_512hl.bin
teacher="$root/teacher/stockfish-sf18-cb3d4ee9/src/stockfish"

[[ -d "$run" && ! -e "$run/PANEL_STATE" ]]
printf 'WAITING_FOR_CALIBRATION\n' > "$run/PANEL_STATE"
trap 'printf "FAILED\n" > "$run/PANEL_STATE"' ERR TERM

# The post-label service writes this existing file and closes it at each stage.
# Wait for the kernel event, not a timer-driven status check.
python3 - "$run/POSTLABEL_STATE" <<'PY'
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
    if state == 'CALIBRATED':
        break
    if state == 'FAILED':
        raise RuntimeError('post-label continuation failed')
    remaining_ms = int((deadline - time.monotonic()) * 1000)
    if remaining_ms <= 0 or not poller.poll(remaining_ms):
        raise TimeoutError('calibration completion notification did not arrive')
    os.read(fd, 4096)
os.close(fd)
PY

[[ ! -e "$next" ]]
[[ $(sha256sum "$original" | cut -d' ' -f1) == 96de37a99f6c7649a1bf297329e49ce20490aa1c0c7c1e935620ce67ad92e50f ]]
[[ $(sha256sum "$runner" | cut -d' ' -f1) == e5a32d1b61a39d1fff67b4c994a6876aa0116757505e6cb66efbc69f70164d25 ]]
[[ $(sha256sum "$engine" | cut -d' ' -f1) == d17740084a5d7c96c3bb150ef55f8be3567779de33dddcb2faa95956458c475b ]]
[[ $(sha256sum "$rodent" | cut -d' ' -f1) == 5f7480b56538e9e64ee02fb5b4842bf0394ee44b707dae3a63ec7f902abd0afb ]]
[[ $(sha256sum "$teacher" | cut -d' ' -f1) == 174270346ae9ed600713d165fa745dfa87fc084e44907c8084767f2b905e86d3 ]]
model_sha=$(sha256sum "$model" | cut -d' ' -f1)
grep -Fx "model_sha256=$model_sha" "$run/postlabel-receipt.txt" > /dev/null
mkdir "$next"
python3 - "$original" "$next/panel.json" "$model_sha" <<'PY'
import json
from pathlib import Path
import sys

original = json.loads(Path(sys.argv[1]).read_text())
assert original['schema'] == 'ngn-k4-search-panel-v1'
assert len(original['positions']) == 32
original['engine_sha256'] = 'd17740084a5d7c96c3bb150ef55f8be3567779de33dddcb2faa95956458c475b'
original['k4_model_sha256'] = sys.argv[3]
Path(sys.argv[2]).write_text(json.dumps(original, indent=2) + '\n')
PY

printf 'SEARCHING\n' > "$run/PANEL_STATE"
panel_sha=$(sha256sum "$next/panel.json" | cut -d' ' -f1)
taskset -c 12 nice -n 10 timeout 15m python3 "$runner" \
  --panel "$next/panel.json" --panel-sha256 "$panel_sha" \
  --engine "$engine" --k4-model "$model" --rodent-model "$rodent" \
  --teacher "$teacher" --output "$next/result.json" \
  > "$next/stdout.json" 2> "$next/stderr.txt"
[[ -s "$next/result.json" ]]
{
  printf 'candidate_model_sha256=%s\n' "$model_sha"
  printf 'candidate_panel_sha256=%s\n' "$panel_sha"
  printf 'candidate_search_result_sha256=%s\n' "$(sha256sum "$next/result.json" | cut -d' ' -f1)"
  printf 'ended_utc=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
} > "$next/receipt.txt"
printf 'PANEL_COMPLETE\n' > "$run/PANEL_STATE"
