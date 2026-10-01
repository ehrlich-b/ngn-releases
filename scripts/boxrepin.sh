#!/usr/bin/env bash
# Drive NGN gauntlet re-pin runs on the LAN Windows box from the Mac.
#
# This exists so Codex/user approvals can be scoped to one narrow script prefix
# (`scripts/boxrepin.sh`) instead of approving ad hoc ssh/scp commands. The
# script intentionally exposes named workflow operations rather than arbitrary
# remote shell passthrough.
#
#   boxrepin.sh push <local-path> [remote-name]        copy a file into %USERPROFILE%\ngn\repin
#   boxrepin.sh stage-ngn <sprt-file> [remote-name]    copy an engine from %USERPROFILE%\ngn\sprt
#   boxrepin.sh hash <remote-file>...                  SHA-256 files in the repin dir
#   boxrepin.sh probe [ngn.exe] [only-list]            gauntlet -probe for the standard TC
#   boxrepin.sh launch <run> [ngn.exe] [games] [only]  detached standard 120+1 gauntlet
#   BOXREPIN_BIN selects a versioned harness (default: gauntlet.exe).
#   boxrepin.sh tail <run> [N]                         tail <run>_out.txt
#   boxrepin.sh fetch <run> [dest-dir]                 fetch out/tally/pgn if present
#   boxrepin.sh ps                                     list live gauntlet/engine processes
#   boxrepin.sh kill                                   stop gauntlet and child engines
set -euo pipefail

BOX="${BOXREPIN_HOST:-ehrli@192.168.4.108}"
KEY="${BOXREPIN_KEY:-$HOME/.ssh/id_ed25519}"
REMOTE_DIR='C:\Users\ehrli\ngn\repin'
REMOTE_REL='ngn/repin'
SPRT_DIR='C:\Users\ehrli\ngn\sprt'
ONLY_DEFAULT='v6.1.0,v7.2.0,v7.4.0,v8.0.0,counter-3.8'
BIN="${BOXREPIN_BIN:-gauntlet.exe}"

SSH=(ssh -i "$KEY" -o IdentitiesOnly=yes -o BatchMode=yes -o ConnectTimeout=20)
SCP=(scp -i "$KEY" -o IdentitiesOnly=yes -o BatchMode=yes -o ConnectTimeout=20)

usage() { sed -n '2,18p' "$0"; exit "${1:-0}"; }

safe_name() {
  case "${1:-}" in
    *[!A-Za-z0-9_.-]*|"") echo "unsafe file/run name: ${1:-<empty>}" >&2; exit 2 ;;
  esac
}

safe_only() {
  case "${1:-}" in
    *[!A-Za-z0-9_.,-]*|"") echo "unsafe -only list: ${1:-<empty>}" >&2; exit 2 ;;
  esac
}

safe_name "$BIN"
cmd="${1:-}"; shift || true
case "$cmd" in
  push)
    src="${1:?push needs a local path}"; name="${2:-$(basename "$src")}"
    safe_name "$name"
    "${SCP[@]}" "$src" "$BOX:$REMOTE_REL/$name"
    echo "pushed $src -> $REMOTE_DIR\\$name"
    ;;

  stage-ngn)
    src="${1:?stage-ngn needs a source file in the box sprt dir}"
    dst="${2:-$src}"
    safe_name "$src"; safe_name "$dst"
    "${SSH[@]}" "$BOX" "powershell -NoProfile -Command \"Copy-Item -Force '$SPRT_DIR\\$src' '$REMOTE_DIR\\$dst'\""
    echo "staged $SPRT_DIR\\$src -> $REMOTE_DIR\\$dst"
    ;;

  hash)
    [ "$#" -gt 0 ] || { echo "hash needs at least one remote file" >&2; exit 2; }
    ps_files=()
    for name in "$@"; do
      safe_name "$name"
      ps_files+=("'$REMOTE_DIR\\$name'")
    done
    joined="$(IFS=,; echo "${ps_files[*]}")"
    "${SSH[@]}" "$BOX" "powershell -NoProfile -Command \"Get-FileHash -Algorithm SHA256 $joined | Select-Object Path,Hash | Format-Table -AutoSize\""
    ;;

  probe)
    ngn="${1:-ngn.exe}"
    only="${2:-$ONLY_DEFAULT}"
    safe_name "$ngn"; safe_only "$only"
    "${SSH[@]}" "$BOX" "powershell -NoProfile -Command \"Set-Location '$REMOTE_DIR'; .\\$BIN -ngn '.\\$ngn' -anchors ratings.json -tc 120+1 -games 80 -concurrency 8 -lowpower=false -openings sprt_openings.txt -only '$only' -probe\""
    ;;

  launch)
    run="${1:?launch needs a run name}"
    ngn="${2:-ngn.exe}"
    games="${3:-80}"
    only="${4:-$ONLY_DEFAULT}"
    safe_name "$run"; safe_name "$ngn"; safe_only "$only"
    case "$games" in *[!0-9]*|"") echo "games must be an integer: $games" >&2; exit 2 ;; esac

    task="ngn_repin_$run"
    bat="$REMOTE_DIR\\run_$run.bat"
    tmp="$(mktemp)"
    printf '@echo off\r\ncd /d "%%USERPROFILE%%\\ngn\\repin"\r\n%s -ngn .\\%s -anchors ratings.json -tc 120+1 -games %s -concurrency 8 -lowpower=false -openings sprt_openings.txt -only %s -tally-out %s_tally.json -pgn %s.pgn -no-record > %s_out.txt 2>&1\r\necho DONE_EXIT_%%ERRORLEVEL%%>> %s_out.txt\r\n' \
      "$BIN" "$ngn" "$games" "$only" "$run" "$run" "$run" "$run" > "$tmp"
    "${SCP[@]}" "$tmp" "$BOX:$REMOTE_REL/run_$run.bat"
    rm -f "$tmp"

    "${SSH[@]}" "$BOX" "schtasks /create /tn $task /tr \"cmd /c $bat\" /sc once /sd 01/01/2099 /st 00:00 /f"
    "${SSH[@]}" "$BOX" "schtasks /run /tn $task"
    "${SSH[@]}" "$BOX" "schtasks /delete /tn $task /f" || true
    echo "launched run_$run.bat -> ${run}_out.txt   (poll: $0 tail $run ; $0 ps)"
    ;;

  tail)
    run="${1:?tail needs a run name}"; n="${2:-20}"
    safe_name "$run"
    case "$n" in *[!0-9]*|"") echo "tail line count must be an integer: $n" >&2; exit 2 ;; esac
    "${SSH[@]}" "$BOX" "powershell -NoProfile -Command \"Get-Content '$REMOTE_DIR\\${run}_out.txt' -Tail $n\""
    ;;

  fetch)
    run="${1:?fetch needs a run name}"; dest="${2:-.}"
    safe_name "$run"
    mkdir -p "$dest"
    for suffix in out.txt tally.json pgn; do
      "${SCP[@]}" "$BOX:$REMOTE_REL/${run}_$suffix" "$dest/" 2>/dev/null || true
    done
    "${SCP[@]}" "$BOX:$REMOTE_REL/${run}.pgn" "$dest/" 2>/dev/null || true
    echo "fetched available $run artifacts -> $dest/"
    ;;

  ps)
    "${SSH[@]}" "$BOX" 'powershell -NoProfile -Command "Get-Process -Name gauntlet*,ngn*,blunder*,counter_38 -ErrorAction SilentlyContinue | Select-Object Id,ProcessName,StartTime,CPU | Format-Table -AutoSize; exit 0"'
    ;;

  kill)
    "${SSH[@]}" "$BOX" 'powershell -NoProfile -Command "Stop-Process -Name gauntlet* -Force -ErrorAction SilentlyContinue; Start-Sleep -Milliseconds 700; Stop-Process -Name ngn*,blunder*,counter_38 -Force -ErrorAction SilentlyContinue; Write-Output killed-gauntlet-then-engines-verify-with-ps; exit 0"'
    ;;

  ""|-h|--help|help) usage 0 ;;
  *) echo "unknown subcommand: $cmd" >&2; usage 1 ;;
esac
