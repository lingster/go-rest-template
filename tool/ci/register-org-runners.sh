#!/usr/bin/env bash
# Registers two organization-level GitHub Actions runners (labels "ios" and
# "android") for your organisation on THIS Mac mini. Run it on the Mac mini as the user
# that owns the existing runners. Idempotent: a runner directory that is already
# configured is skipped.
#
#   gh api -X POST orgs/$ORG/actions/runners/registration-token -q .token
#   tool/ci/register-org-runners.sh        # paste the token when prompted
#
# A registration token lives 1 hour and can be reused within that time; if the
# script runs past it, get a fresh one and re-run (finished runners are skipped).
set -euo pipefail

ORG="${ORG:?set ORG=your-github-organisation}"
RUNNER_GROUP="Default"
BASE="$HOME"
HOST="$(hostname -s)"

latest_release() {
  curl -fsSL https://api.github.com/repos/actions/runner/releases/latest
}

# Prints "<url> <sha256>" of the osx-x64 tarball.
resolve_download() {
  local json version url sha
  json="$(latest_release)"
  version="$(printf '%s' "$json" | sed -n 's/.*"tag_name": *"v\([^"]*\)".*/\1/p' | head -n1)"
  [ -n "$version" ] || { echo "cannot determine latest runner version" >&2; exit 1; }
  url="https://github.com/actions/runner/releases/download/v${version}/actions-runner-osx-x64-${version}.tar.gz"
  # The release notes list "<name> <sha256>" lines in an HTML comment/table.
  sha="$(printf '%s' "$json" | grep -o "actions-runner-osx-x64-${version}.tar.gz[^a-f0-9]*[a-f0-9]\{64\}" \
         | grep -o '[a-f0-9]\{64\}' | head -n1 || true)"
  echo "$url $sha"
}

read -r DL_URL DL_SHA <<<"$(resolve_download)"
TARBALL="$(mktemp -t actions-runner).tar.gz"
trap 'rm -f "$TARBALL"' EXIT
echo "Downloading $DL_URL"
curl -fsSL -o "$TARBALL" "$DL_URL"
if [ -n "$DL_SHA" ]; then
  echo "$DL_SHA  $TARBALL" | shasum -a 256 -c - >/dev/null \
    || { echo "sha256 mismatch for runner download" >&2; exit 1; }
  echo "sha256 verified"
else
  echo "WARNING: no sha256 found in release metadata; download not verified" >&2
fi

# Reads JAVA_HOME / ANDROID_HOME from an existing runner's .env, else prompts.
env_value() {
  local key="$1" val=""
  for f in "$HOME"/actions-runner*/.env; do
    [ -f "$f" ] || continue
    val="$(sed -n "s/^${key}=//p" "$f" | head -n1)"
    [ -n "$val" ] && break
  done
  if [ -z "$val" ]; then
    read -r -p "$key (not found in existing runners' .env): " val
  fi
  printf '%s' "$val"
}

register() {
  local kind="$1" dir="$BASE/actions-runner-ci-$1"
  if [ -f "$dir/.runner" ]; then
    echo "[$kind] already configured in $dir, skipping"
    return
  fi
  mkdir -p "$dir"
  tar -xzf "$TARBALL" -C "$dir"
  if [ -z "${TOKEN:-}" ]; then
    read -r -s -p "Registration token: " TOKEN
    echo
  fi
  (
    cd "$dir"
    ./config.sh --unattended --url "https://github.com/$ORG" --token "$TOKEN" \
      --name "$HOST-$kind" --labels "$kind" --runnergroup "$RUNNER_GROUP" \
      --work _work --replace
    if [ "$kind" = android ]; then
      {
        echo "JAVA_HOME=$(env_value JAVA_HOME)"
        echo "ANDROID_HOME=$(env_value ANDROID_HOME)"
      } >> .env
    fi
    ./svc.sh install && ./svc.sh start
  )
  echo "[$kind] registered as $HOST-$kind"
}

TOKEN=""
register ios
register android

echo
echo "Verify (should list $HOST-ios and $HOST-android as online):"
echo "  gh api orgs/$ORG/actions/runners --jq '.runners[] | [.name, .status, ([.labels[].name] | join(\",\"))] | @tsv'"
