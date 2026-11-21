#!/usr/bin/env bash
# Runner hook ACTIONS_RUNNER_HOOK_JOB_STARTED: waits until no other job runs on this host, then takes the
# host lock (see host-lock.sh) so only one CI job runs at a time across all runners on the machine.
# Install with tool/ci/install-runner-hooks.sh. A non-zero exit fails the job, which is the intent when
# the wait exceeds CI_HOST_LOCK_WAIT_MAX.
set -u
# shellcheck source=host-lock.sh
. "$(dirname "${BASH_SOURCE[0]}")/host-lock.sh"
hl_acquire
