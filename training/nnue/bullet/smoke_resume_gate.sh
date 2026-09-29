#!/usr/bin/env bash
set -euo pipefail

readonly FINALIZER_SCHEMA="ngn-k4-finalized-corpus-v1"
readonly FINALIZER_CONTRACT="sf18-5kn-material-inversion-v2"
readonly IDENTITY_CONTRACT='sha256(ngn-k4-final-segments-v1\0 || segment_name || count:u64le || sha256(ngn-k4-final-accepted-v1\0 || id:32 || k4_key:32 || ordinal:u64le) for each ordered segment)'
readonly TRAIN_RECORDS=1000000
readonly HOLDOUT_RECORDS=100000
readonly RECORD_BYTES=32

usage() {
  echo "usage: smoke_resume_gate.sh TRAINER SMOKE_BF SAMPLER_RECEIPT NEW_OUTPUT_ROOT" >&2
  exit 2
}

[[ $# -eq 4 ]] || usage
trainer="$(realpath -- "$1")"
smoke_bf="$(realpath -- "$2")"
sampler="$(realpath -- "$3")"
output_root="$4"

[[ -x "$trainer" ]] || { echo "trainer is not executable: $trainer" >&2; exit 1; }
[[ -f "$smoke_bf" ]] || { echo "smoke BF is not a regular file: $smoke_bf" >&2; exit 1; }
[[ -f "$sampler" ]] || { echo "sampler receipt is not a regular file: $sampler" >&2; exit 1; }
[[ ! -e "$output_root" ]] || { echo "refusing existing output root: $output_root" >&2; exit 1; }
[[ "$(stat -c %s -- "$smoke_bf")" -eq $((TRAIN_RECORDS * RECORD_BYTES)) ]] || {
  echo "smoke BF must contain exactly $TRAIN_RECORDS records" >&2
  exit 1
}

fixture="$output_root/fixture"
mkdir -p -- "$fixture"
for split in validation calibration reserved-test; do
  dd if="$smoke_bf" of="$fixture/$split.bf" bs="$RECORD_BYTES" count="$HOLDOUT_RECORDS" status=none
done

manifest="$fixture/finalized-smoke-fixture.json"
sampler_bytes="$(stat -c %s -- "$sampler")"
sampler_sha="$(sha256sum -- "$sampler" | awk '{print $1}')"
train_bytes="$(stat -c %s -- "$smoke_bf")"
train_sha="$(sha256sum -- "$smoke_bf" | awk '{print $1}')"
validation="$(realpath -- "$fixture/validation.bf")"
calibration="$(realpath -- "$fixture/calibration.bf")"
reserved_test="$(realpath -- "$fixture/reserved-test.bf")"
holdout_bytes=$((HOLDOUT_RECORDS * RECORD_BYTES))
validation_sha="$(sha256sum -- "$validation" | awk '{print $1}')"
calibration_sha="$(sha256sum -- "$calibration" | awk '{print $1}')"
reserved_test_sha="$(sha256sum -- "$reserved_test" | awk '{print $1}')"

jq -n \
  --arg schema "$FINALIZER_SCHEMA" \
  --arg contract "$FINALIZER_CONTRACT" \
  --arg identity_contract "$IDENTITY_CONTRACT" \
  --arg sampler "$sampler" --argjson sampler_bytes "$sampler_bytes" --arg sampler_sha "$sampler_sha" \
  --arg train "$smoke_bf" --argjson train_bytes "$train_bytes" --arg train_sha "$train_sha" \
  --arg validation "$validation" --arg validation_sha "$validation_sha" \
  --arg calibration "$calibration" --arg calibration_sha "$calibration_sha" \
  --arg reserved_test "$reserved_test" --arg reserved_test_sha "$reserved_test_sha" \
  --argjson train_records "$TRAIN_RECORDS" --argjson holdout_records "$HOLDOUT_RECORDS" \
  --argjson holdout_bytes "$holdout_bytes" --argjson record_bytes "$RECORD_BYTES" '
    def receipt($path; $bytes; $sha): {path: $path, bytes: $bytes, sha256: $sha};
    def corpus($name; $split; $records; $path; $bytes; $sha; $segment): {
      name: $name,
      split: $split,
      records: $records,
      record_bytes: $record_bytes,
      file: receipt($path; $bytes; $sha),
      segments: [{name: $segment, records: $records, accepted_identity_sha256: $sha}],
      accepted_identity_sha256: $sha,
      identity_contract: $identity_contract
    };
    def shard($stage; $split; $id; $records; $path; $bytes; $sha): {
      stage: $stage,
      split: $split,
      shard_id: $id,
      candidate_records: $records,
      accepted_available: $records,
      accepted_used: $records,
      sampler_input: receipt($sampler; $sampler_bytes; $sampler_sha),
      labels: receipt($sampler; $sampler_bytes; $sampler_sha),
      pack_receipt: receipt($sampler; $sampler_bytes; $sampler_sha),
      packed: receipt($path; $bytes; $sha),
      packer_accepted_stream_sha256: $sha
    };
    {
      schema: $schema,
      contract_version: $contract,
      state: "COMPLETE",
      mode: "pilot",
      command: ["synthetic-smoke-resume-fixture", "not-owned-production-data"],
      sampler: receipt($sampler; $sampler_bytes; $sampler_sha),
      corpora: [
        corpus("train-pilot"; "train"; $train_records; $train; $train_bytes; $train_sha; "pilot/train"),
        corpus("validation"; "validation"; $holdout_records; $validation; $holdout_bytes; $validation_sha; "fixed/validation"),
        corpus("calibration"; "calibration"; $holdout_records; $calibration; $holdout_bytes; $calibration_sha; "fixed/calibration"),
        corpus("reserved-test"; "reserved-test"; $holdout_records; $reserved_test; $holdout_bytes; $reserved_test_sha; "fixed/reserved-test")
      ],
      used_shards: [
        shard("pilot"; "train"; "synthetic-smoke-train"; $train_records; $train; $train_bytes; $train_sha),
        shard("fixed"; "validation"; "synthetic-smoke-validation"; $holdout_records; $validation; $holdout_bytes; $validation_sha),
        shard("fixed"; "calibration"; "synthetic-smoke-calibration"; $holdout_records; $calibration; $holdout_bytes; $calibration_sha),
        shard("fixed"; "reserved-test"; "synthetic-smoke-reserved-test"; $holdout_records; $reserved_test; $holdout_bytes; $reserved_test_sha)
      ]
    }
  ' > "$manifest"
sync "$fixture"

cuda_lib="${NGN_CUDA_LIB:-/home/ehrli/nnue-public-toolchain-20260906/install/cuda-12.8.1/lib64}"
export LD_LIBRARY_PATH="$cuda_lib:/usr/lib/wsl/lib${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"

full="$output_root/full"
resumed="$output_root/resumed-from-128"
"$trainer" start smoke "$manifest" "$full"
parent="$full/candidates/candidate-128/receipt.json"
"$trainer" resume smoke "$manifest" "$parent" "$resumed"

full_final="$full/candidates/candidate-256/receipt.json"
resumed_final="$resumed/candidates/candidate-256/receipt.json"
"$trainer" compare "$(dirname -- "$full_final")" "$(dirname -- "$resumed_final")"
[[ "$(jq -c '.reader' "$full_final")" == "$(jq -c '.reader' "$resumed_final")" ]] || {
  echo "resume reader witness mismatch" >&2
  exit 1
}

echo "NGN_K4_SMOKE_RESUME_PASS full=$full_final resumed=$resumed_final"
