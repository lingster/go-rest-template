#!/usr/bin/env bash
# One-shot setup of the GitHub Actions secrets for the store pipelines
# (docs/STORES.md), plus the first signed Android App Bundle for the manual
# first upload to Play. Run it yourself on a Mac with gh logged in:
#
#   tool/ci/setup-store-secrets.sh            # secrets only
#   tool/ci/setup-store-secrets.sh --build-aab  # secrets + signed AAB for the first Play upload
#
# Values are read from files or hidden prompts and piped straight into
# `gh secret set`; nothing is echoed, written to shell history or passed as
# a command-line argument. Every step can be skipped by leaving its prompt empty.
set -euo pipefail

REPO="${REPO:-$(gh repo view --json nameWithOwner -q .nameWithOwner 2>/dev/null || true)}"
[[ -n "$REPO" ]] || { echo "set REPO=owner/name (or run inside the cloned repo with gh logged in)" >&2; exit 1; }
SLUG="${REPO##*/}"
SECURE_DIR="${SECURE_DIR:-$HOME/secure}"
KEYSTORE="$SECURE_DIR/$SLUG-upload.jks"
KEYCHAIN_ITEM="$SLUG-upload-keystore"   # macOS login keychain item holding the keystore password
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
BUILD_AAB=0
[[ "${1:-}" == "--build-aab" ]] && BUILD_AAB=1

say() { printf '\n== %s\n' "$*"; }
secret_from_stdin() { gh secret set "$1" --repo "$REPO" >/dev/null && echo "set secret $1"; }
ask_hidden() { local v; read -r -s -p "$1: " v; echo >&2; printf '%s' "$v"; }
ask() { local v; read -r -p "$1: " v; printf '%s' "$v"; }

command -v gh >/dev/null || { echo "gh CLI is required" >&2; exit 1; }
gh auth status >/dev/null 2>&1 || { echo "run 'gh auth login' first" >&2; exit 1; }

# --- Android upload key -------------------------------------------------------
say "Android upload keystore ($KEYSTORE)"
if [[ ! -f "$KEYSTORE" ]]; then
  mkdir -p "$SECURE_DIR" && chmod 700 "$SECURE_DIR"
  KS_PASS="$(openssl rand -base64 33 | tr -d '/+=\n' | cut -c1-32)"
  export KS_PASS
  keytool -genkeypair -noprompt -keystore "$KEYSTORE" -storetype PKCS12 \
    -alias upload -keyalg RSA -keysize 4096 -validity 10000 \
    -dname "CN=$SLUG, O=${KEY_ORG:-$SLUG}, C=${KEY_COUNTRY:-GB}" \
    -storepass:env KS_PASS -keypass:env KS_PASS
  chmod 600 "$KEYSTORE"
  security add-generic-password -U -a "$USER" -s "$KEYCHAIN_ITEM" -w "$KS_PASS"
  echo "created keystore; password saved in your login keychain as '$KEYCHAIN_ITEM'"
  echo "BACK UP $KEYSTORE (e.g. in your password manager): Play needs this upload key forever."
else
  KS_PASS="$(security find-generic-password -a "$USER" -s "$KEYCHAIN_ITEM" -w 2>/dev/null || true)"
  [[ -n "$KS_PASS" ]] || KS_PASS="$(ask_hidden "Existing keystore password")"
  export KS_PASS
  echo "using existing keystore"
fi
base64 -i "$KEYSTORE" | tr -d '\n' | secret_from_stdin ANDROID_KEYSTORE_BASE64
printf '%s' "$KS_PASS" | secret_from_stdin ANDROID_KEYSTORE_PASSWORD
printf '%s' "$KS_PASS" | secret_from_stdin ANDROID_KEY_PASSWORD
printf 'upload' | secret_from_stdin ANDROID_KEY_ALIAS

# --- Google Play service account ----------------------------------------------
say "Google Play service account JSON"
PLAY_JSON="$(ask "Path to the Play service account JSON (empty = skip)")"
if [[ -n "$PLAY_JSON" ]]; then
  [[ -f "$PLAY_JSON" ]] || { echo "no such file: $PLAY_JSON" >&2; exit 1; }
  secret_from_stdin PLAY_SERVICE_ACCOUNT_JSON < "$PLAY_JSON"
fi

# --- App Store Connect API key + CI keychain ----------------------------------
say "App Store Connect API key"
shopt -s nullglob
P8S=("$HOME"/.appstoreconnect/private_keys/AuthKey_*.p8)
shopt -u nullglob
P8="${P8S[0]:-}"
if [[ ${#P8S[@]} -ne 1 ]]; then P8="$(ask "Path to AuthKey_XXXXXXXXXX.p8 (empty = skip)")"; fi
if [[ -n "$P8" ]]; then
  KEY_ID="$(basename "$P8" .p8)"; KEY_ID="${KEY_ID#AuthKey_}"
  echo "key file: $P8 (key id $KEY_ID)"
  printf '%s' "$KEY_ID" | secret_from_stdin ASC_KEY_ID
  secret_from_stdin ASC_KEY_P8 < "$P8"
  ISSUER="$(ask "Issuer ID (App Store Connect > Users and Access > Integrations)")"
  [[ -n "$ISSUER" ]] && printf '%s' "$ISSUER" | secret_from_stdin ASC_ISSUER_ID
fi
echo "CI_KEYCHAIN_PATH variable: $(gh variable get CI_KEYCHAIN_PATH --repo "$REPO" 2>/dev/null || echo '(unset: a new $SLUG-ci keychain will be created on a self-hosted Mac; hosted macOS runners need no keychain secret)')"
KC_PASS="$(ask_hidden "Password of that CI keychain on the Mac mini (empty = skip)")"
[[ -n "$KC_PASS" ]] && printf '%s' "$KC_PASS" | secret_from_stdin CI_KEYCHAIN_PASSWORD

say "Secrets now on $REPO"
gh secret list --repo "$REPO"

# --- First signed App Bundle for the manual Play upload ------------------------
if [[ $BUILD_AAB -eq 1 ]]; then
  say "Building signed App Bundle (first manual upload to Play)"
  API_BASE_URL="$(gh variable get API_BASE_URL --repo "$REPO" 2>/dev/null || echo http://localhost:8080)"
  cd "$ROOT/flutter_app"
  APP_KEYSTORE_PATH="$KEYSTORE" APP_KEYSTORE_PASSWORD="$KS_PASS" \
  APP_KEY_ALIAS=upload APP_KEY_PASSWORD="$KS_PASS" \
    flutter build appbundle --release --target-platform android-arm,android-arm64 \
      --build-name=0.1.0 --build-number=1 \
      --dart-define=API_BASE_URL="$API_BASE_URL" --dart-define=APP_VERSION=0.1.0
  AAB="$ROOT/flutter_app/build/app/outputs/bundle/release/app-release.aab"
  jarsigner -verify "$AAB" >/dev/null && echo "signed: $AAB"
  echo "Upload it in Play Console > your app > Testing > Internal testing > Create new release."
fi
