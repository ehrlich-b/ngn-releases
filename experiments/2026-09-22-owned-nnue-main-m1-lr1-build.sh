#!/usr/bin/env bash
set -euo pipefail
umask 022

root=/home/ehrli/nnue-owned-k4-20260920
source="$root/bullet-exec-main-m1-v1"
workspace="$root/bullet-exec-main-m1-lr1-v1"
run="$root/runs/k4-main-m1-lr1-build-20260922"
rust_stage=/mnt/c/Users/ehrli/ngn_k4_train-main-m1-lr1.rs
bridge_stage=/mnt/c/Users/ehrli/ngnk4bridge-main-m1-lr1-v1
bin="$root/bin-main-m1-lr1-v1"
toolchain=/home/ehrli/nnue-public-toolchain-20260906
trainer="$workspace/target/release/examples/ngn_k4_train"

[[ -d "$source" && ! -e "$workspace" && ! -e "$run" && ! -e "$bin" ]]
[[ $(sha256sum "$source/crates/bullet_lib/examples/ngn_k4_train.rs" | cut -d' ' -f1) == febb926b3926ece196975153bc64d8af3ae0531e4dfceb08a4950f6ed703971a ]]
[[ $(sha256sum "$rust_stage" | cut -d' ' -f1) == 2b4d5999a7207b896fe9326906bd7933d533201db13066a54488cf64552e5e0f ]]
[[ $(sha256sum "$bridge_stage" | cut -d' ' -f1) == 3c9e9c9df636968c6a3a0dd61d5f9cd4ae991f0aebc221170f24b10cbbcda906 ]]
grep -Fx 'bullet_commit=629ee50000b2afb7b3337595401c830d3b1e0f42' "$source/NGN_K4_EXECUTION_IDENTITY" > /dev/null
grep -Fx 'patch_sha256=f7f5e0dd02695fe57ec58a630b63006da96cb1ec6b8fd4769a4e6308a8121e54' "$source/NGN_K4_EXECUTION_IDENTITY" > /dev/null
[[ -x "$toolchain/install/rust-1.88.0/bin/cargo" ]]
[[ -d "$toolchain/pilot-cargo-home" && -d "$toolchain/pilot-vendor" ]]
[[ -d "$toolchain/install/cuda-12.8.1" ]]

mkdir "$run"
printf 'COPYING\n' > "$run/STATE"
trap 'printf "FAILED\n" > "$run/STATE"' ERR TERM
cp -a "$source" "$workspace"
install -m 644 "$rust_stage" "$workspace/crates/bullet_lib/examples/ngn_k4_train.rs"
[[ $(sha256sum "$workspace/crates/bullet_lib/examples/ngn_k4_train.rs" | cut -d' ' -f1) == 2b4d5999a7207b896fe9326906bd7933d533201db13066a54488cf64552e5e0f ]]
{
  printf 'bullet_commit=%s\n' 629ee50000b2afb7b3337595401c830d3b1e0f42
  printf 'patch_sha256=%s\n' f7f5e0dd02695fe57ec58a630b63006da96cb1ec6b8fd4769a4e6308a8121e54
  printf 'harness_sha256=%s\n' 2b4d5999a7207b896fe9326906bd7933d533201db13066a54488cf64552e5e0f
} > "$workspace/NGN_K4_EXECUTION_IDENTITY"

export PATH="$toolchain/install/rust-1.88.0/bin:$PATH"
export CARGO_HOME="$toolchain/pilot-cargo-home"
export CUDA_PATH="$toolchain/install/cuda-12.8.1"
export LD_LIBRARY_PATH="/usr/lib/wsl/lib:$CUDA_PATH/lib64"

printf 'BUILDING\n' > "$run/STATE"
cd "$workspace"
taskset -c 12,14 nice -n 10 timeout 30m cargo build \
  --locked --offline --release --features cuda \
  -p bullet_lib --example ngn_k4_train -j2 \
  > "$run/build.log" 2>&1
[[ -x "$trainer" ]]
ldd "$trainer" > "$run/ldd.txt"
! grep -F 'not found' "$run/ldd.txt" > /dev/null
grep -F 'libnvrtc' "$run/ldd.txt" > /dev/null
grep -F 'libcublas' "$run/ldd.txt" > /dev/null

mkdir "$bin"
install -m 755 "$bridge_stage" "$bin/ngnk4bridge"
{
  printf 'ended_utc=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  printf 'bullet_commit=%s\n' 629ee50000b2afb7b3337595401c830d3b1e0f42
  printf 'patch_sha256=%s\n' f7f5e0dd02695fe57ec58a630b63006da96cb1ec6b8fd4769a4e6308a8121e54
  printf 'harness_sha256=%s\n' 2b4d5999a7207b896fe9326906bd7933d533201db13066a54488cf64552e5e0f
  printf 'cuda_trainer_sha256=%s\n' "$(sha256sum "$trainer" | cut -d' ' -f1)"
  printf 'bridge_sha256=%s\n' "$(sha256sum "$bin/ngnk4bridge" | cut -d' ' -f1)"
} > "$run/receipt.txt"
printf 'BUILT\n' > "$run/STATE"
