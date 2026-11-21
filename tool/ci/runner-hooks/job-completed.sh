#!/usr/bin/env bash
# Runner hook ACTIONS_RUNNER_HOOK_JOB_COMPLETED: releases the host lock if this job holds it.
# Never fails the job. If the runner is killed before this runs, the next waiter sees the dead PID
# and removes the lock.
set -u
# shellcheck source=host-lock.sh
. "$(dirname "${BASH_SOURCE[0]}")/host-lock.sh"
hl_release || true
exit 0
