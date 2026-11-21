#!/usr/bin/env bash
# Installs the host-wide one-job-at-a-time lock (tool/ci/runner-hooks/) for the self-hosted runners on THIS
# machine: copies the hooks to a stable directory and points every runner's .env at them
# (ACTIONS_RUNNER_HOOK_JOB_STARTED / ACTIONS_RUNNER_HOOK_JOB_COMPLETED). Idempotent. Run as the user that
# owns the runners. Prints, but does not run, the commands that restart the runners.
#
#   tool/ci/install-runner-hooks.sh [--dry-run] [--hooks-dir DIR] [RUNNER_DIR...]
#
# RUNNER_DIR defaults to every $HOME/actions-runner* directory. A runner whose .env already sets a
# different hook is reported and left alone. CI_HOOKS_DIR also sets the hooks directory (default $HOME/ci-hooks).
# Works with macOS bash 3.2.
set -euo pipefail

SRC="$(cd "$(dirname "${BASH_SOURCE[0]}")/runner-hooks" && pwd)"
HOOK_FILES="host-lock.sh job-started.sh job-completed.sh"
HOOKS_DIR="${CI_HOOKS_DIR:-$HOME/ci-hooks}"
DRY_RUN=0
RUNNERS=""

usage() { sed -n '2,11p' "$0" | sed 's/^# \{0,1\}//'; }

while [ $# -gt 0 ]; do
  case "$1" in
    --dry-run) DRY_RUN=1 ;;
    --hooks-dir) HOOKS_DIR="${2:?--hooks-dir needs a directory}"; shift ;;
    -h | --help) usage; exit 0 ;;
    -*) echo "unknown option: $1" >&2; usage >&2; exit 2 ;;
    *) RUNNERS="$RUNNERS
$1" ;;
  esac
  shift
done

if [ -z "$RUNNERS" ]; then
  for d in "$HOME"/actions-runner*; do
    [ -d "$d" ] && RUNNERS="$RUNNERS
$d"
  done
fi
[ -n "$RUNNERS" ] || { echo "no runner directories found (looked for $HOME/actions-runner*); pass them as arguments" >&2; exit 1; }

run() { # run <description> <command...>: does it, or just says so with --dry-run
  local what=$1
  shift
  if [ "$DRY_RUN" = 1 ]; then echo "[dry-run] $what"; else "$@"; fi
}

# env_value FILE KEY: the value of KEY= in an env file (last one wins), empty if unset.
env_value() { { sed -n "s/^$2=//p" "$1" 2>/dev/null || true; } | tail -n 1; }

# set_env_line FILE KEY VALUE: appends KEY=VALUE, adding the newline the last line may lack.
set_env_line() {
  [ ! -s "$1" ] || [ -z "$(tail -c 1 "$1")" ] || printf '\n' >>"$1"
  printf '%s=%s\n' "$2" "$3" >>"$1"
}

# restart_hint DIR: the command that restarts that runner's service.
restart_hint() {
  local dir=$1 svc="" unit
  [ -f "$dir/.service" ] && svc=$(cat "$dir/.service")
  case "$(uname -s)" in
    Darwin)
      if [ -n "$svc" ]; then
        unit=$(basename "$svc" .plist)
        echo "launchctl kickstart -k gui/\$(id -u)/$unit"
      else
        echo "(cd '$dir' && ./svc.sh stop && ./svc.sh start)"
      fi
      ;;
    *)
      if [ -n "$svc" ]; then
        echo "systemctl --user restart '$svc'   # or, for a system-level unit: (cd '$dir' && sudo ./svc.sh stop && sudo ./svc.sh start)"
      else
        echo "(cd '$dir' && sudo ./svc.sh stop && sudo ./svc.sh start)   # or: systemctl --user restart <unit>"
      fi
      ;;
  esac
}

install_hooks() {
  local f
  run "mkdir -p $HOOKS_DIR" mkdir -p "$HOOKS_DIR"
  for f in $HOOK_FILES; do
    run "install $SRC/$f -> $HOOKS_DIR/$f" install -m 755 "$SRC/$f" "$HOOKS_DIR/$f"
  done
}

STARTED_KEY=ACTIONS_RUNNER_HOOK_JOB_STARTED
COMPLETED_KEY=ACTIONS_RUNNER_HOOK_JOB_COMPLETED
STARTED_VAL="$HOOKS_DIR/job-started.sh"
COMPLETED_VAL="$HOOKS_DIR/job-completed.sh"
CHANGED=""
SKIPPED=""

# configure_runner DIR: wires one runner; records it in CHANGED (needs a restart) or SKIPPED (conflict).
configure_runner() {
  local dir=$1 env_file="$1/.env" cur_s cur_c
  if [ ! -d "$dir" ]; then
    echo "skip $dir: not a directory"
    SKIPPED="$SKIPPED
$dir (not a directory)"
    return 0
  fi
  cur_s=$(env_value "$env_file" "$STARTED_KEY")
  cur_c=$(env_value "$env_file" "$COMPLETED_KEY")
  if { [ -n "$cur_s" ] && [ "$cur_s" != "$STARTED_VAL" ]; } || { [ -n "$cur_c" ] && [ "$cur_c" != "$COMPLETED_VAL" ]; }; then
    echo "skip $dir: .env already sets a different hook (started='${cur_s:-}' completed='${cur_c:-}'); merge by hand"
    SKIPPED="$SKIPPED
$dir (different hook already set)"
    return 0
  fi
  if [ "$cur_s" = "$STARTED_VAL" ] && [ "$cur_c" = "$COMPLETED_VAL" ]; then
    echo "ok   $dir: already configured"
    return 0
  fi
  [ -n "$cur_s" ] || run "$dir/.env: add $STARTED_KEY=$STARTED_VAL" set_env_line "$env_file" "$STARTED_KEY" "$STARTED_VAL"
  [ -n "$cur_c" ] || run "$dir/.env: add $COMPLETED_KEY=$COMPLETED_VAL" set_env_line "$env_file" "$COMPLETED_KEY" "$COMPLETED_VAL"
  if [ "$DRY_RUN" = 1 ]; then echo "would configure $dir"; else echo "set  $dir: hooks configured in .env"; fi
  CHANGED="$CHANGED
$dir"
}

install_hooks
while IFS= read -r runner; do
  [ -z "$runner" ] || configure_runner "$runner"
done <<<"$RUNNERS"

echo
if [ -n "$CHANGED" ]; then
  echo "Restart these runners so they read the new .env. Do it only when they are IDLE:"
  echo "restarting a runner kills the job it is running."
  while IFS= read -r runner; do
    [ -z "$runner" ] || { echo "  # $runner"; echo "  $(restart_hint "$runner")"; }
  done <<<"$CHANGED"
else
  echo "Nothing changed; no restart needed."
fi
if [ -n "$SKIPPED" ]; then
  echo
  echo "Skipped (not configured):$SKIPPED"
fi
[ "$DRY_RUN" = 0 ] || echo "(dry run: nothing was written)"
