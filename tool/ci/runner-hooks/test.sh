#!/usr/bin/env bash
# Self-contained test of the host job lock hooks (no bats). Run: bash tool/ci/runner-hooks/test.sh
# Jobs are simulated by running the hooks with the GitHub env vars set and a fake worker PID
# (a background `sleep` we can kill).
set -u
HERE=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
TMP=$(mktemp -d)
export CI_HOST_LOCK_DIR="$TMP/lock"
export CI_HOST_LOCK_POLL=0.2
FAKE_PIDS=""
PASS=0
FAIL=0

cleanup() {
  [ -z "$FAKE_PIDS" ] || kill $FAKE_PIDS 2>/dev/null
  rm -rf "$TMP"
}
trap cleanup EXIT

check() { # check <description> <command...>
  local what=$1
  shift
  if "$@"; then
    PASS=$((PASS + 1))
    echo "ok   - $what"
  else
    FAIL=$((FAIL + 1))
    echo "FAIL - $what"
  fi
}

fake_worker() { # fake_worker VAR: sets VAR to the PID of a long-lived fake worker
  sleep 300 &
  FAKE_PIDS="$FAKE_PIDS $!"
  eval "$1=$!"
}

kill_worker() {
  kill "$1" 2>/dev/null
  wait "$1" 2>/dev/null
}

# job <hook> <runner> <worker pid>: runs a hook as that runner's job, output to $LOG (default: discarded).
job() {
  RUNNER_NAME=$2 GITHUB_REPOSITORY=acme/$2-repo GITHUB_RUN_ID=100 GITHUB_JOB=build \
    CI_HOST_LOCK_WORKER_PID=$3 bash "$HERE/$1.sh" >"${LOG:-/dev/null}" 2>&1
}

owner() { cat "$CI_HOST_LOCK_DIR/owner" 2>/dev/null; }
has_owner() { owner | grep -q "^$1/"; }
lock_exists() { [ -d "$CI_HOST_LOCK_DIR" ]; }
contains() { grep -q -- "$2" "$1"; }

# wait_until <seconds> <command...>
wait_until() {
  local limit=$1 i=0
  shift
  while [ "$i" -lt $((limit * 10)) ]; do
    "$@" && return 0
    sleep 0.1
    i=$((i + 1))
  done
  return 1
}
alive() { kill -0 "$1" 2>/dev/null; }
dead() { ! alive "$1"; }

echo "== two jobs: the second waits until the first completes =="
fake_worker W1
fake_worker W2
LOG="$TMP/a.log" check "job A acquires the free lock" job job-started runnerA "$W1"
check "lock records A as owner" has_owner runnerA
check "lock records A's worker PID" test "$(cat "$CI_HOST_LOCK_DIR/pid")" = "$W1"
LOG="$TMP/b.log" job job-started runnerB "$W2" &
B=$!
sleep 1
check "job B is still waiting" alive "$B"
check "B's log names the holder" contains "$TMP/b.log" "waiting for .* held by runnerA/"
check "the lock still belongs to A" has_owner runnerA
check "A completes" job job-completed runnerA "$W1"
check "B then acquires" wait_until 5 dead "$B"
wait "$B"
check "B exited 0" test $? -eq 0
check "lock now belongs to B" has_owner runnerB

echo "== release by a non-owner is ignored =="
LOG="$TMP/c.log" job job-completed runnerA "$W1"
check "completed hook exits 0 for a non-owner" test $? -eq 0
check "non-owner left B's lock alone" has_owner runnerB
check "non-owner log says not releasing" contains "$TMP/c.log" "not releasing"
check "owner releases" job job-completed runnerB "$W2"
check "lock is gone" test ! -d "$CI_HOST_LOCK_DIR"
check "completed with no lock exits 0" job job-completed runnerB "$W2"

echo "== a stale lock (holder PID dead) is recovered =="
fake_worker W3
job job-started runnerC "$W3"
kill_worker "$W3"
fake_worker W4
LOG="$TMP/d.log" check "new job takes over from the dead holder" job job-started runnerD "$W4"
check "lock now belongs to D" has_owner runnerD
check "log says the stale lock was removed" contains "$TMP/d.log" "removing stale lock"
job job-completed runnerD "$W4"

echo "== a recycled PID (different program) counts as stale =="
fake_worker W5
job job-started runnerE "$W5"
echo "not-$(cat "$CI_HOST_LOCK_DIR/comm")" >"$CI_HOST_LOCK_DIR/comm"
fake_worker W6
check "job takes over when the PID now runs another program" job job-started runnerF "$W6"
check "lock now belongs to F" has_owner runnerF
job job-completed runnerF "$W6"

echo "== two waiters race for the same stale lock: exactly one wins =="
fake_worker W7
job job-started runnerG "$W7"
kill_worker "$W7"
fake_worker W8
fake_worker W9
CI_HOST_LOCK_WAIT_MAX=3 LOG="$TMP/h.log" job job-started runnerH "$W8" &
H=$!
CI_HOST_LOCK_WAIT_MAX=3 LOG="$TMP/i.log" job job-started runnerI "$W9" &
I=$!
wait "$H"
HS=$?
wait "$I"
IS=$?
check "one job got the lock and the other timed out" test $((HS + IS)) -ne 0 -a "$HS" -ne "$IS"
check "the lock is held by exactly the winner" test "$(owner | cut -d/ -f1)" = "$([ "$HS" -eq 0 ] && echo runnerH || echo runnerI)"
rm -rf "$CI_HOST_LOCK_DIR"

echo "== a lock with no PID is a holder still writing it; stale only after a minute =="
mkdir "$CI_HOST_LOCK_DIR"
fake_worker W10
check "young PID-less lock is respected (job times out)" \
  test "$(CI_HOST_LOCK_WAIT_MAX=1 job job-started runnerJ "$W10" >/dev/null 2>&1; echo $?)" -eq 1
touch -t 200001010000 "$CI_HOST_LOCK_DIR"
check "old PID-less lock is recovered" job job-started runnerJ "$W10"
check "lock now belongs to J" has_owner runnerJ
job job-completed runnerJ "$W10"

echo "== WAIT_MAX timeout =="
fake_worker W11
fake_worker W12
job job-started runnerK "$W11"
start=$SECONDS
CI_HOST_LOCK_WAIT_MAX=1 LOG="$TMP/l.log" job job-started runnerL "$W12"
status=$?
check "waiter exits non-zero" test "$status" -ne 0
check "waiter emitted ::error::" contains "$TMP/l.log" "^::error::"
check "it gave up after about WAIT_MAX seconds" test $((SECONDS - start)) -le 4
check "the holder's lock is untouched" has_owner runnerK
job job-completed runnerK "$W11"

echo "== the worker is found by walking the ancestors, not \$PPID =="
FAKE_BASH="$TMP/Runner.Worker"
if cp "$(command -v bash)" "$FAKE_BASH" 2>/dev/null && [ "$("$FAKE_BASH" -c 'ps -o comm= -p $$; :' | sed 's|.*/||')" = "Runner.Worker" ]; then
  # Runner.Worker -> short-lived bash -> hook: $PPID is the short-lived shell.
  "$FAKE_BASH" -c "echo \$\$ >'$TMP/worker.pid'; RUNNER_NAME=runnerM GITHUB_REPOSITORY=acme/m bash -c 'bash $HERE/job-started.sh; :' >'$TMP/m.log' 2>&1; :"
  check "lock records the Runner.Worker PID" test "$(cat "$CI_HOST_LOCK_DIR/pid" 2>/dev/null)" = "$(cat "$TMP/worker.pid")"
  check "no fallback warning" bash -c "! grep -q 'no Runner.Worker' '$TMP/m.log'"
  rm -rf "$CI_HOST_LOCK_DIR"
else
  echo "skip - cannot fake a process named Runner.Worker here"
fi

echo
echo "passed $PASS, failed $FAIL"
[ "$FAIL" -eq 0 ]
