#!/usr/bin/env bash
# Sets up an organization-level GitHub Actions runner for Android builds on an
# Ubuntu server (labels: self-hosted, Linux, X64, android). Run once with sudo:
#
#   sudo tool/ci/setup-linux-android-runner.sh
#
# - dedicated system user `ghrunner-android` (home /opt/ghrunner-android, NOT in
#   the docker group), so signing material only lives under that user
# - JDK 17, Ruby (fastlane) and build deps from apt; Android SDK under
#   /opt/android-sdk (owned by the runner user so Gradle can add components)
# - everything under /opt, keeping it off a full /home
# - asks for a registration token (hidden). Get one with:
#     gh api -X POST orgs/$ORG/actions/runners/registration-token -q .token
# Idempotent: re-running skips what is already installed/configured.
set -euo pipefail

ORG="${ORG:?set ORG=your-github-organisation}"
RUNNER_USER="${RUNNER_USER:-ghrunner-android}"
RUNNER_HOME="/opt/$RUNNER_USER"
RUNNER_DIR="$RUNNER_HOME/actions-runner"
SDK="/opt/android-sdk"
CMDLINE_TOOLS_ZIP="commandlinetools-linux-13114758_latest.zip"   # Android cmdline-tools 19.0
SDK_PACKAGES=("platform-tools" "platforms;android-36" "build-tools;36.0.0")
RUNNER_NAME="${RUNNER_NAME:-$(hostname -s)-android}"

[[ $EUID -eq 0 ]] || { echo "run with sudo" >&2; exit 1; }
as_runner() { sudo -u "$RUNNER_USER" -H env "PATH=$PATH" "$@"; }

echo "== packages"
apt-get update -qq
DEBIAN_FRONTEND=noninteractive apt-get install -y -qq \
  openjdk-17-jdk-headless ruby-full build-essential git curl unzip zip xz-utils jq libglu1-mesa >/dev/null
JAVA_HOME="$(dirname "$(dirname "$(readlink -f "$(command -v javac)")")")"
echo "JAVA_HOME=$JAVA_HOME  ($(java -version 2>&1 | head -1))  ruby $(ruby -e 'print RUBY_VERSION')"

echo "== runner user $RUNNER_USER"
if ! id "$RUNNER_USER" >/dev/null 2>&1; then
  useradd --system --create-home --home-dir "$RUNNER_HOME" --shell /bin/bash "$RUNNER_USER"
fi
if id -nG "$RUNNER_USER" | tr ' ' '\n' | grep -qx docker; then
  echo "WARNING: $RUNNER_USER is in the docker group; remove it (gpasswd -d $RUNNER_USER docker)" >&2
fi
chmod 750 "$RUNNER_HOME"

echo "== Android SDK in $SDK"
mkdir -p "$SDK"
chown "$RUNNER_USER:$RUNNER_USER" "$SDK"
if [[ ! -x "$SDK/cmdline-tools/latest/bin/sdkmanager" ]]; then
  tmp="$(mktemp -d)"
  curl -fsSL -o "$tmp/tools.zip" "https://dl.google.com/android/repository/$CMDLINE_TOOLS_ZIP"
  unzip -q "$tmp/tools.zip" -d "$tmp"
  as_runner mkdir -p "$SDK/cmdline-tools"
  mv "$tmp/cmdline-tools" "$SDK/cmdline-tools/latest"
  chown -R "$RUNNER_USER:$RUNNER_USER" "$SDK/cmdline-tools"
  rm -rf "$tmp"
fi
SDKMANAGER="$SDK/cmdline-tools/latest/bin/sdkmanager"
yes | as_runner env JAVA_HOME="$JAVA_HOME" "$SDKMANAGER" --sdk_root="$SDK" --licenses >/dev/null || true
as_runner env JAVA_HOME="$JAVA_HOME" "$SDKMANAGER" --sdk_root="$SDK" "${SDK_PACKAGES[@]}" >/dev/null
echo "installed: ${SDK_PACKAGES[*]}"

echo "== GitHub Actions runner in $RUNNER_DIR"
if [[ -f "$RUNNER_DIR/.runner" ]]; then
  echo "already configured, skipping registration"
else
  as_runner mkdir -p "$RUNNER_DIR"
  version="$(curl -fsSL https://api.github.com/repos/actions/runner/releases/latest | jq -r .tag_name | sed 's/^v//')"
  tarball="actions-runner-linux-x64-$version.tar.gz"
  as_runner curl -fsSL -o "$RUNNER_DIR/$tarball" \
    "https://github.com/actions/runner/releases/download/v$version/$tarball"
  as_runner tar -xzf "$RUNNER_DIR/$tarball" -C "$RUNNER_DIR"
  rm -f "$RUNNER_DIR/$tarball"
  read -r -s -p "Registration token: " TOKEN; echo
  as_runner bash -c "cd '$RUNNER_DIR' && ./config.sh --unattended --url 'https://github.com/$ORG' \
    --token \"\$0\" --name '$RUNNER_NAME' --labels android --runnergroup Default --work _work --replace" "$TOKEN"
  unset TOKEN
fi
# Toolchain for jobs (read by the runner on start).
as_runner bash -c "cat > '$RUNNER_DIR/.env'" <<ENV
LANG=C.UTF-8
JAVA_HOME=$JAVA_HOME
ANDROID_HOME=$SDK
ANDROID_SDK_ROOT=$SDK
ENV

echo "== service"
cd "$RUNNER_DIR"
if ! ./svc.sh status 2>/dev/null | grep -q "active (running)"; then
  ./svc.sh install "$RUNNER_USER" >/dev/null 2>&1 || true
  ./svc.sh start
fi
./svc.sh status | grep -E "Active:" || true
echo
echo "Done. Verify: gh api orgs/$ORG/actions/runners -q '.runners[] | \"\\(.name) \\(.status)\"'"
