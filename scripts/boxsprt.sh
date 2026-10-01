#!/usr/bin/env bash
# boxsprt.sh (M3) — drive the free SPRT mill on the LAN gaming PC (9800X3D, native
# Windows) from the Mac over SSH. Wraps the run_*.bat + schtasks (far-future /sd +
# delete-after-run) launch + poll/fetch/kill flow so the box mill is one command
# instead of hand-assembled ssh/powershell each time.
#
#   boxsprt.sh push   <local-path> [remote-name]   scp a file into the box sprt dir
#   boxsprt.sh launch <name> [--] <sprt args...>   write run_<name>.bat, fire it detached (schtasks
#                                                  create/run/delete), output <name>_out.txt
#   boxsprt.sh ps                                  list live sprt/engine processes on the box
#   boxsprt.sh tail   <name> [N]                   tail <name>_out.txt (default 20 lines)
#   boxsprt.sh fetch  <name> [dest-dir]            scp <name>_out.txt to the Mac (default cwd)
#   boxsprt.sh hash   <remote-name>                sha256 of a file in the box sprt dir
#   boxsprt.sh kill                                Stop-Process sprt FIRST (it respawns engines), then engines
#
# Notes / gotchas baked in (see experiments/2026-06-28-night-runbook.md, memory project_gaming_pc_worker):
#   - Engine paths in the sprt args MUST carry a separator to dodge Go ErrDot, and the backslash
#     must survive bash: quote them, e.g. -new '.\ngn_t1a.exe' -base '.\ngn_base2.exe'.
#   - Pass -lowpower=false (the macOS taskpolicy path does not exist on Windows).
#   - schtasks uses a far-future /sd so a leftover task can never re-fire at date rollover; the task
#     is deleted right after /run (the started process is detached in Session 0 and survives).
#   - ALWAYS end a session with `boxsprt.sh ps` empty (no leaked workers).
set -euo pipefail

BOX="${BOXSPRT_HOST:-ehrli@192.168.4.108}"
KEY="${BOXSPRT_KEY:-$HOME/.ssh/id_ed25519}"
REMOTE_DIR='C:\Users\ehrli\ngn\sprt'   # Windows path (schtasks/powershell)
REMOTE_REL='ngn/sprt'                   # forward-slash path for scp (relative to home)
# Binary `launch` runs. Defaults to sprt.exe so every existing SPRT call site is
# byte-identical; T8 sets BOXSPRT_BIN=spsa.exe to drive the tuner on the same rig.
BIN="${BOXSPRT_BIN:-sprt.exe}"
SSH=(ssh -i "$KEY" -o IdentitiesOnly=yes -o ConnectTimeout=20)
SCP=(scp -i "$KEY" -o IdentitiesOnly=yes -o ConnectTimeout=20)

usage() { sed -n '2,29p' "$0"; exit "${1:-0}"; }

cmd="${1:-}"; shift || true
case "$cmd" in
  push)
    src="${1:?push needs a local path}"; name="${2:-$(basename "$src")}"
    "${SCP[@]}" "$src" "$BOX:$REMOTE_REL/$name"
    echo "pushed $src -> $REMOTE_DIR\\$name"
    ;;

  launch)
    name="${1:?launch needs a run name}"; shift
    [ "${1:-}" = "--" ] && shift || true
    [ "$#" -gt 0 ] || { echo "launch: no sprt args given" >&2; exit 2; }
    args="$*"
    task="ngn_$name"
    bat="$REMOTE_DIR\\run_$name.bat"
    # Build the CRLF batch file locally, then scp it (avoids remote quoting hell).
    tmp="$(mktemp)"
    printf '@echo off\r\ncd /d "%%USERPROFILE%%\\ngn\\sprt"\r\n%s %s > %s_out.txt 2>&1\r\necho DONE_EXIT_%%ERRORLEVEL%%>> %s_out.txt\r\n' "$BIN" "$args" "$name" "$name" > "$tmp"
    "${SCP[@]}" "$tmp" "$BOX:$REMOTE_REL/run_$name.bat"
    rm -f "$tmp"
    # schtasks create (far-future /sd so it never auto-fires) -> run now -> delete the task entry.
    "${SSH[@]}" "$BOX" "schtasks /create /tn $task /tr \"cmd /c $bat\" /sc once /sd 01/01/2099 /st 00:00 /f"
    "${SSH[@]}" "$BOX" "schtasks /run /tn $task"
    "${SSH[@]}" "$BOX" "schtasks /delete /tn $task /f" || true
    echo "launched run_$name.bat -> ${name}_out.txt   (poll: $0 tail $name ; $0 ps)"
    ;;

  ps)
    # exit 0 even when idle: no matching process is the normal good state (Get-Process -Name
    # sets a nonzero exit on no-match, which would trip set -e in callers).
    "${SSH[@]}" "$BOX" 'powershell -NoProfile -Command "Get-Process -Name sprt*,ngn*,spsa* -ErrorAction SilentlyContinue | Select-Object Id,ProcessName,StartTime,CPU | Format-Table -AutoSize; exit 0"'
    ;;

  tail)
    name="${1:?tail needs a run name}"; n="${2:-20}"
    "${SSH[@]}" "$BOX" "powershell -NoProfile -Command \"Get-Content $REMOTE_DIR\\${name}_out.txt -Tail $n\""
    ;;

  fetch)
    name="${1:?fetch needs a run name}"; dest="${2:-.}"
    "${SCP[@]}" "$BOX:$REMOTE_REL/${name}_out.txt" "$dest/"
    echo "fetched ${name}_out.txt -> $dest/"
    ;;

  hash)
    name="${1:?hash needs a remote file name}"
    "${SSH[@]}" "$BOX" "powershell -NoProfile -Command \"(Get-FileHash $REMOTE_DIR\\$name).Hash\""
    ;;

  kill)
    # sprt respawns engines, so stop the harness FIRST, then any leftover engines.
    "${SSH[@]}" "$BOX" 'powershell -NoProfile -Command "Stop-Process -Name sprt*,spsa* -Force -ErrorAction SilentlyContinue; Start-Sleep -Milliseconds 700; Stop-Process -Name ngn_* -Force -ErrorAction SilentlyContinue; Write-Output killed-harnesses-then-engines-verify-with-ps; exit 0"'
    ;;

  ""|-h|--help|help) usage 0 ;;
  *) echo "unknown subcommand: $cmd" >&2; usage 1 ;;
esac
