#!/usr/bin/env bash
set -Eeuo pipefail
umask 022

root=/home/ehrli/nnue-owned-k4-20260920
pilot="$root/runs/k4-production-pilot-20260921"
run="$root/runs/k4-pilot-lr1-20260922-retry1"
manifest="$pilot/finalized-pilot/manifest.json"
static="$root/bin-static-s1/ngnk4staticdump"
analysis=/mnt/c/Users/ehrli/ngnk4static-analyze-s1.py
rodent=/home/ehrli/repos/ngn-smp-depth-stagger-gate-20260918/run-001/inputs/rodent_anand_512hl.bin

[[ $(cat "$run/STATE") == SELECTED ]]
[[ $(sha256sum "$run/selection.json" | cut -d' ' -f1) == 4ad114f4f6ea91d46f63097e50be23f61e7e10e14cbfdf16bb5e934b3fb5d4ae ]]
[[ $(sha256sum "$run/ngnk4bridge" | cut -d' ' -f1) == 37da957bd6c3214357d9ea511a023503aa05c2e59664da21d75f20cda679119c ]]
[[ $(sha256sum "$static" | cut -d' ' -f1) == 24cf5ed4370773a57baf34568050bc5962f4d7e821a961df85e2139220d084fb ]]
[[ $(sha256sum "$analysis" | cut -d' ' -f1) == 23498dde5a593e9237578acb33cfbcd9dc424ab2daa4e539afd9b9c96623fb02 ]]
[[ $(sha256sum "$manifest" | cut -d' ' -f1) == 753e8ed1fbe09bef290fcba7209dfb77dce0170e80a5a09a22a1c3c12599755e ]]
[[ ! -e "$run/model-selected.nnue" && ! -e "$run/static" ]]
update=$(jq -r .selected_update "$run/selection.json")
[[ "$update" == 1024 ]]

printf 'CALIBRATING\n' > "$run/STATE"
trap 'printf "FAILED_CALIBRATION\n" > "$run/STATE"' ERR TERM
quant="$run/training/candidates/candidate-$update/quantised.bin"
model="$run/model-selected.nnue"
"$run/ngnk4bridge" convert -in "$quant" -manifest "$manifest" -out "$model" > "$run/convert.json"
model_sha=$(sha256sum "$model" | cut -d' ' -f1)
mkdir "$run/static"
taskset -c 12 nice -n 10 timeout 900 "$static" \
  -manifest "$manifest" \
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
  printf 'selected_update=%s\n' "$update"
  printf 'model_sha256=%s\n' "$model_sha"
  printf 'predictions_sha256=%s\n' "$(sha256sum "$run/static/predictions.csv" | cut -d' ' -f1)"
  printf 'static_receipt_sha256=%s\n' "$(sha256sum "$run/static/receipt.json" | cut -d' ' -f1)"
  printf 'analysis_sha256=%s\n' "$(sha256sum "$run/static/analysis.json" | cut -d' ' -f1)"
  printf 'ended_utc=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
} > "$run/calibration-receipt.txt"
printf 'CALIBRATED\n' > "$run/STATE"
jq '.groups.overall.all | {k4, hce, k4_minus_hce}' "$run/static/analysis.json"
