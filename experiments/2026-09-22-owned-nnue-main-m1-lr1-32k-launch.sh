#!/usr/bin/env bash
set -euo pipefail
umask 022

root=/home/ehrli/nnue-owned-k4-20260920
base="$root/runs/k4-main-m1-lr1-20260922"
pilot="$root/runs/k4-production-pilot-20260921"
workspace="$root/bullet-exec-main-m1-lr1-32k-v2"
run="$root/runs/k4-main-m1-lr1-32k-20260922-retry2"
rust_stage=/mnt/c/Users/ehrli/ngn_k4_train-main-m1-lr1-32k.rs
bridge_stage=/mnt/c/Users/ehrli/ngnk4bridge-main-m1-lr1-32k-linux
trainer="$workspace/target/release/examples/ngn_k4_train"
bridge="$run/ngnk4bridge"
toolchain=/home/ehrli/nnue-public-toolchain-20260906
static="$root/bin-static-s1/ngnk4staticdump"
analysis=/mnt/c/Users/ehrli/ngnk4static-analyze-s1.py
rodent=/home/ehrli/repos/ngn-smp-depth-stagger-gate-20260918/run-001/inputs/rodent_anand_512hl.bin
manifest="$base/finalized-main/manifest.json"

[[ -d "$workspace" && ! -e "$run" ]]
[[ $(cat "$root/runs/k4-main-m1-lr1-32k-20260922-retry1/STATE") == FAILED ]]
[[ $(sha256sum "$workspace/crates/bullet_lib/examples/ngn_k4_train.rs" | cut -d' ' -f1) == 13ed246c0680299e2b81688aab4ba2e08800bcfd143875cc250671918d5f7355 ]]
[[ $(cat "$base/STATE") == CALIBRATED ]]
[[ $(cat "$base/PANEL_STATE") == PANEL_COMPLETE ]]
[[ $(sha256sum "$base/model-selected.nnue" | cut -d' ' -f1) == cebec29cbab676af6a2ade2b67da5c9a01ddb7882ed604238c0e7d67893f72d6 ]]
[[ $(sha256sum "$manifest" | cut -d' ' -f1) == ef6aca46d496be0fd876eb063cda3650a85ba7ae92c683105bf433a29565ad8e ]]
[[ $(sha256sum "$root/bullet-exec-main-m1-lr1-v1/crates/bullet_lib/examples/ngn_k4_train.rs" | cut -d' ' -f1) == 2b4d5999a7207b896fe9326906bd7933d533201db13066a54488cf64552e5e0f ]]
[[ $(sha256sum "$rust_stage" | cut -d' ' -f1) == 13ed246c0680299e2b81688aab4ba2e08800bcfd143875cc250671918d5f7355 ]]
[[ $(sha256sum "$bridge_stage" | cut -d' ' -f1) == 9d8ce15ec056a1766c40999dd4107ddd9c912c2a698695e15a6a3aa400b28e06 ]]
[[ $(systemctl --user is-active arli-hopper.service) == active ]]
[[ -z $(pgrep -x fastchess || true) ]]
[[ -z $(/usr/lib/wsl/lib/nvidia-smi --query-compute-apps=pid --format=csv,noheader) ]]
linux_free=$(df -B1 --output=avail / | tail -n 1 | tr -d ' ')
windows_free=$(/mnt/c/Windows/System32/WindowsPowerShell/v1.0/powershell.exe -NoProfile -Command '(Get-PSDrive C).Free' | tr -d '\r')
(( linux_free >= 40 * 1024 * 1024 * 1024 ))
(( windows_free >= 60 * 1024 * 1024 * 1024 ))

mkdir "$run" "$run/cargo-home"
printf 'BUILDING\n' > "$run/STATE"
trap 'printf "FAILED\n" > "$run/STATE"' ERR TERM
{
  printf 'started_utc=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  printf 'experiment=main-m1-lr1-32k\n'
  printf 'parent_manifest_sha256=%s\n' ef6aca46d496be0fd876eb063cda3650a85ba7ae92c683105bf433a29565ad8e
  printf 'rust_source_sha256=%s\n' 13ed246c0680299e2b81688aab4ba2e08800bcfd143875cc250671918d5f7355
  printf 'bridge_sha256=%s\n' 9d8ce15ec056a1766c40999dd4107ddd9c912c2a698695e15a6a3aa400b28e06
  printf 'linux_free_before_bytes=%s\nwindows_c_free_before_bytes=%s\n' "$linux_free" "$windows_free"
} > "$run/receipt.txt"

install -m 755 "$bridge_stage" "$bridge"
{
  printf 'bullet_commit=%s\n' 629ee50000b2afb7b3337595401c830d3b1e0f42
  printf 'patch_sha256=%s\n' f7f5e0dd02695fe57ec58a630b63006da96cb1ec6b8fd4769a4e6308a8121e54
  printf 'harness_sha256=%s\n' 13ed246c0680299e2b81688aab4ba2e08800bcfd143875cc250671918d5f7355
} > "$run/NGN_K4_EXECUTION_IDENTITY"

cat > "$run/cargo-home/config.toml" <<'EOF'
[source.crates-io]
replace-with = "vendored-sources"

[source.vendored-sources]
directory = "/home/ehrli/nnue-public-toolchain-20260906/vendor"

[net]
offline = true
EOF

export PATH="$toolchain/install/rust-1.88.0/bin:$PATH"
export CARGO_HOME="$run/cargo-home"
export CUDA_PATH="$toolchain/install/cuda-12.8.1"
export LD_LIBRARY_PATH="/usr/lib/wsl/lib:$CUDA_PATH/lib64"
cd "$workspace"
taskset -c 12,14 nice -n 10 timeout 30m cargo fmt -- --check > "$run/fmt.log" 2>&1
taskset -c 12,14 nice -n 10 timeout 30m cargo build --locked --offline --release --features cuda \
  -p bullet_lib --example ngn_k4_train -j2 > "$run/build.log" 2>&1
[[ -x "$trainer" ]]
ldd "$trainer" > "$run/ldd.txt"
! grep -F 'not found' "$run/ldd.txt" > /dev/null
printf 'trainer_sha256=%s\n' "$(sha256sum "$trainer" | cut -d' ' -f1)" >> "$run/receipt.txt"

[[ $(systemctl --user is-active arli-hopper.service) == active ]]
[[ -z $(pgrep -x fastchess || true) ]]
[[ -z $(/usr/lib/wsl/lib/nvidia-smi --query-compute-apps=pid --format=csv,noheader) ]]
printf 'TRAINING\n' > "$run/STATE"
taskset -c 12-15 nice -n 10 timeout 2h "$trainer" start main-m1-lr1-32k "$manifest" "$run/training" \
  > "$run/training.log" 2>&1
[[ -f "$run/training/completion.json" ]]
printf 'training_completion_sha256=%s\n' "$(sha256sum "$run/training/completion.json" | cut -d' ' -f1)" >> "$run/receipt.txt"

printf 'SELECTING\n' > "$run/STATE"
taskset -c 12-15 nice -n 10 timeout 1h "$bridge" select \
  -mode main-m1-lr1-32k -candidates "$run/training/candidates" \
  -manifest "$manifest" -validation "$pilot/finalized-pilot/validation.bf" \
  -out "$run/selection.json" > "$run/selection.stdout.txt" 2> "$run/selection.stderr.txt"
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
  -output "$run/static/predictions.csv" -receipt "$run/static/receipt.json" \
  > "$run/static/stdout.json"
taskset -c 12 nice -n 10 timeout 900 python3 "$analysis" \
  --csv "$run/static/predictions.csv" --receipt "$run/static/receipt.json" \
  --output "$run/static/analysis.json"
{
  printf 'model_sha256=%s\n' "$model_sha"
  printf 'static_analysis_sha256=%s\n' "$(sha256sum "$run/static/analysis.json" | cut -d' ' -f1)"
  printf 'ended_utc=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
} >> "$run/receipt.txt"
printf 'CALIBRATED\n' > "$run/STATE"
