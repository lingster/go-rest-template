#!/usr/bin/env bash
# Shared helpers for the host-wide CI job lock (sourced by job-started.sh and job-completed.sh).
# The lock is a directory taken with an atomic `mkdir` (no flock on macOS) holding:
#   pid    PID of the job's Runner.Worker process
#   comm   that process's name, to tell it from an unrelated process that reused the PID
#   owner  runner/repo/run/job that holds it
#   since  when it was taken (for the log line only)
# Works on macOS bash 3.2 and Linux bash. Settings (environment):
#   CI_HOST_LOCK_DIR       lock directory            (default $HOME/.ci-host-job.lock)
#   CI_HOST_LOCK_POLL      seconds between attempts  (default 15)
#   CI_HOST_LOCK_WAIT_MAX  give up after N seconds   (default 21600 = 6 h)
#   CI_HOST_LOCK_WORKER_PID  use this PID instead of finding Runner.Worker (tests)

hl_dir() { printf '%s' "${CI_HOST_LOCK_DIR:-$HOME/.ci-host-job.lock}"; }
hl_log() { printf '[host-lock] %s\n' "$*"; }

# runner/repo/run.attempt/job: identifies this job so only it can release its lock.
hl_owner() {
  printf '%s/%s/run-%s.%s/%s' "${RUNNER_NAME:-unknown-runner}" "${GITHUB_REPOSITORY:-unknown-repo}" \
    "${GITHUB_RUN_ID:-0}" "${GITHUB_RUN_ATTEMPT:-1}" "${GITHUB_JOB:-unknown-job}"
}

# hl_proc PID: sets HL_PPID and HL_PCOMM (executable name without its directory; macOS prints the full path)
# from one `ps` call. Fails if the PID is gone or a zombie.
hl_proc() {
  local out
  HL_PPID="" HL_PCOMM=""
  out=$(ps -o ppid=,stat=,comm= -p "$1" 2>/dev/null) || return 1
  [ -n "$out" ] || return 1
  # shellcheck disable=SC2034 # HL_PSTAT is only read by the case below
  read -r HL_PPID HL_PSTAT HL_PCOMM <<<"$out"
  case $HL_PSTAT in *Z*) return 1 ;; esac
  [ -n "$HL_PCOMM" ] || return 1
  HL_PCOMM=${HL_PCOMM##*/}
}

# Sets HL_PID and HL_COMM to the job's Runner.Worker: the nearest ancestor of this shell with that name.
# $PPID alone is wrong: the runner may start the hook through a short-lived shell. If no ancestor is named
# Runner.Worker, falls back to the oldest ancestor below PID 1 (the runner service, alive for as long as
# any job) and sets HL_FALLBACK=1.
hl_find_worker() {
  local pid=$$ top=$$ topcomm=""
  HL_FALLBACK=0
  if [ -n "${CI_HOST_LOCK_WORKER_PID:-}" ]; then
    HL_PID=$CI_HOST_LOCK_WORKER_PID
    hl_proc "$HL_PID" && HL_COMM=$HL_PCOMM || HL_COMM=""
    return 0
  fi
  while [ -n "$pid" ] && [ "$pid" -gt 1 ] 2>/dev/null; do
    hl_proc "$pid" || break
    if [ "$HL_PCOMM" = "Runner.Worker" ]; then
      HL_PID=$pid HL_COMM=$HL_PCOMM
      return 0
    fi
    top=$pid topcomm=$HL_PCOMM
    pid=$HL_PPID
  done
  HL_FALLBACK=1
  HL_PID=$top HL_COMM=$topcomm
}

hl_read() { cat "$1/$2" 2>/dev/null || true; }

# Writes a file inside the lock directory atomically (readers never see a half-written value).
hl_put() { printf '%s\n' "$3" >"$1/.$2.tmp.$$" && mv -f "$1/.$2.tmp.$$" "$1/$2"; }

# True if a path exists and was last modified more than a minute ago.
hl_older_than_minute() { [ -n "$(find "$1" -maxdepth 0 -mmin +1 2>/dev/null)" ]; }

# True if the lock directory exists but its holder is gone: the recorded PID is not alive (or is now another
# program), or the directory has had no PID for over a minute (its creator died between mkdir and writing it).
# A directory that is younger than that and has no PID is a holder still writing it: not stale.
hl_stale() {
  local dir=$1 pid comm
  [ -d "$dir" ] || return 1
  pid=$(hl_read "$dir" pid)
  if [ -z "$pid" ]; then
    hl_older_than_minute "$dir"
    return
  fi
  case $pid in *[!0-9]*) return 0 ;; esac
  hl_proc "$pid" || return 0
  comm=$(hl_read "$dir" comm)
  [ -z "$comm" ] || [ "$comm" = "$HL_PCOMM" ] || return 0
  return 1
}

# Removes a stale lock. Only one waiter at a time may do this (the .steal guard, itself a mkdir lock) and it
# re-checks staleness while holding the guard, so two waiters can never remove a lock a third just took.
hl_steal() {
  local dir=$1 guard="$1.steal"
  if ! mkdir "$guard" 2>/dev/null; then
    # A stealer that died while holding the guard must not block everyone forever.
    ! hl_older_than_minute "$guard" || rmdir "$guard" 2>/dev/null || true
    return 1
  fi
  if hl_stale "$dir"; then
    hl_log "removing stale lock $dir (holder: $(hl_read "$dir" owner), pid $(hl_read "$dir" pid) is gone)"
    rm -rf "$dir"
  fi
  rmdir "$guard" 2>/dev/null || true
}

# One attempt to take the lock for this job. Needs HL_PID/HL_COMM from hl_find_worker.
hl_try_acquire() {
  local dir=$1
  mkdir "$dir" 2>/dev/null || return 1
  hl_put "$dir" pid "$HL_PID"
  hl_put "$dir" comm "$HL_COMM"
  hl_put "$dir" since "$(date '+%Y-%m-%d %H:%M:%S')"
  hl_put "$dir" owner "$(hl_owner)"
}

# Blocks until this job holds the lock. Returns 1 (after an ::error::) when CI_HOST_LOCK_WAIT_MAX passes.
hl_acquire() {
  local dir poll max started waited
  dir=$(hl_dir) poll=${CI_HOST_LOCK_POLL:-15} max=${CI_HOST_LOCK_WAIT_MAX:-21600}
  mkdir -p "$(dirname "$dir")"
  hl_find_worker
  [ "$HL_FALLBACK" = 0 ] || hl_log "no Runner.Worker among my ancestors; tracking process $HL_PID ($HL_COMM) instead"
  started=$SECONDS
  while :; do
    if hl_try_acquire "$dir"; then
      hl_log "acquired $dir for $(hl_owner) (pid $HL_PID, waited $((SECONDS - started))s)"
      return 0
    fi
    # Retry at once after removing a stale lock; if another waiter holds the steal guard, wait as usual.
    if hl_stale "$dir" && hl_steal "$dir"; then
      continue
    fi
    waited=$((SECONDS - started))
    if [ "$waited" -ge "$max" ]; then
      echo "::error::Gave up after ${waited}s waiting for the host job lock $dir held by $(hl_read "$dir" owner) (pid $(hl_read "$dir" pid), since $(hl_read "$dir" since)). If that job is gone, remove the directory."
      return 1
    fi
    hl_log "waiting for $dir held by $(hl_read "$dir" owner) (pid $(hl_read "$dir" pid), since $(hl_read "$dir" since)); waited ${waited}s, next check in ${poll}s"
    sleep "$poll"
  done
}

# Releases the lock only if this job owns it. Always returns 0.
hl_release() {
  local dir held
  dir=$(hl_dir)
  if [ ! -d "$dir" ]; then
    hl_log "no lock at $dir to release"
    return 0
  fi
  held=$(hl_read "$dir" owner)
  if [ "$held" = "$(hl_owner)" ]; then
    rm -rf "$dir"
    hl_log "released $dir"
  else
    hl_log "not releasing $dir: held by '${held:-nobody known}', this job is $(hl_owner)"
  fi
  return 0
}
