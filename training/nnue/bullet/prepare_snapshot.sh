#!/usr/bin/env bash
set -euo pipefail

readonly BULLET_COMMIT="629ee50000b2afb7b3337595401c830d3b1e0f42"
readonly RUN_RS_SHA="3513776fe95252124de9f525957cf2a4924e5dc172bb64675ccf74b6a3245ae3"
readonly VALUE_RS_SHA="f3e36f52c91c6b604846a83ed1b7241926557ce1ea4539389258c67b3c746fe6"
readonly CARGO_TOML_SHA="5f2b3c29a72600624684d5e2a7ef0f70434f588958ede06362ae4f825cf9e7e6"
readonly CARGO_LOCK_SHA="dbf60b75be6aedd74064b198c2793f733fa4c02fd679c36cebea72eb0e20b546"

usage() {
  echo "usage: prepare_snapshot.sh PRISTINE_BULLET_ROOT NEW_EXECUTION_ROOT" >&2
  exit 2
}

[[ $# -eq 2 ]] || usage
source_root="$1"
execution_root="$2"
script_root="$(cd -- "$(dirname -- "$0")" && pwd)"

[[ -d "$source_root/.git" ]] || {
  echo "pristine Bullet root is not a git worktree" >&2
  exit 1
}
[[ ! -e "$execution_root" ]] || {
  echo "refusing existing execution root: $execution_root" >&2
  exit 1
}
[[ "$(git -C "$source_root" rev-parse HEAD)" == "$BULLET_COMMIT" ]] || {
  echo "Bullet commit mismatch" >&2
  exit 1
}
[[ -z "$(git -C "$source_root" status --porcelain)" ]] || {
  echo "pristine Bullet root is dirty" >&2
  exit 1
}

printf '%s  %s\n' "$RUN_RS_SHA" "$source_root/crates/trainer/src/run.rs" | sha256sum --check --status
printf '%s  %s\n' "$VALUE_RS_SHA" "$source_root/crates/bullet_lib/src/value.rs" | sha256sum --check --status
printf '%s  %s\n' "$CARGO_TOML_SHA" "$source_root/crates/bullet_lib/Cargo.toml" | sha256sum --check --status
printf '%s  %s\n' "$CARGO_LOCK_SHA" "$source_root/Cargo.lock" | sha256sum --check --status

cp -a --reflink=auto "$source_root" "$execution_root"
patch -d "$execution_root" -p1 --forward --batch < "$script_root/ngn-k4-bullet.patch"
mkdir -p "$execution_root/crates/bullet_lib/examples"
cp "$script_root/ngn_k4_train.rs" "$execution_root/crates/bullet_lib/examples/ngn_k4_train.rs"

printf 'bullet_commit=%s\npatch_sha256=%s\nharness_sha256=%s\n' \
  "$BULLET_COMMIT" \
  "$(sha256sum "$script_root/ngn-k4-bullet.patch" | awk '{print $1}')" \
  "$(sha256sum "$script_root/ngn_k4_train.rs" | awk '{print $1}')" \
  > "$execution_root/NGN_K4_EXECUTION_IDENTITY"
