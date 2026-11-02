# Store setup (one-time, manual)

The app ships to **TestFlight** (internal testing) and the **Google Play internal testing
track** from GitHub Actions. How releases are cut: [RELEASING.md](RELEASING.md). This file lists
what CI cannot do for you. Until the secrets below exist, every store workflow skips itself with
a `::notice::` (a fresh project never fails because of them).

Throughout, `REPO=<owner>/<repo>` is your GitHub repository.

## What lives where

| Thing | Where |
|---|---|
| iOS bundle id / Android applicationId | the `bundle_id` you chose at init; both are written once to `tool/fastlane/app_config.rb` |
| Apple team id | repo variable `APPLE_TEAM_ID`, or `--apple-team-id` / copier `apple_team_id` at init (baked into `app_config.rb`) |
| App Store Connect app id, team name | optional repo variables `APPLE_APP_ID`, `APPLE_TEAM_NAME` |
| Store listing text and images | [README](../README.md#customise-your-store-listing-aso) |

`tool/fastlane/app_config.rb` is the single place for app-specific values of the release tooling;
the Fastfiles, workflows and docs never repeat them.

## Secrets and variables

Set under **Settings, Secrets and variables, Actions** (or with `gh`, see below).

| Name | Kind | Needed by | Notes |
|---|---|---|---|
| `ASC_KEY_ID`, `ASC_ISSUER_ID`, `ASC_KEY_P8` | secrets | iOS workflows | App Store Connect API key (role Admin) |
| `CI_KEYCHAIN_PASSWORD` | secret | self-hosted Mac only | hosted macOS runners generate a throwaway keychain, no secret needed |
| `PLAY_SERVICE_ACCOUNT_JSON` | secret | Play workflows | service account JSON (section 4) |
| `ANDROID_KEYSTORE_BASE64`, `ANDROID_KEYSTORE_PASSWORD`, `ANDROID_KEY_ALIAS`, `ANDROID_KEY_PASSWORD` | secrets | `android-internal.yml` | upload keystore (section 3) |
| `APPLE_TEAM_ID` | variable | `ios-internal.yml` | 10 characters; optional if baked in at init |
| `APPLE_APP_ID` | variable | optional | numeric "Apple ID" of the App Store Connect app; skips a lookup by bundle id |
| `APPLE_TEAM_NAME` | variable | optional | legal team name; when set, a provisioning profile of another team is rejected |
| `CI_KEYCHAIN_PATH` | variable | optional | reuse an existing keychain on a self-hosted Mac (see the certificate limit below) |
| `API_BASE_URL` | variable | store builds | production API URL; unset falls back to `localhost`, so set it |
| `GOOGLE_SERVER_CLIENT_ID`, `GOOGLE_IOS_CLIENT_ID` | variables | optional | Sign in with Google, see the README |
| `PLAY_RELEASE_STATUS` | variable | optional | `draft` while Play still treats the app as a draft app (section 4) |
| `IOS_RUNNER`, `ANDROID_RUNNER` | variables | optional | self-hosted runner labels as a JSON array, see "Runners" |

A workflow skips with a notice unless **all** secrets it needs exist (`store-gate.yml`):
Play: service account plus the four keystore secrets. iOS: the three `ASC_*` secrets.
Listing workflows need only the service account / the `ASC_*` secrets.

## Runners

Default: GitHub-hosted. `macos-latest` for iOS, `ubuntu-latest` for Android, the checks and the
Play listing. Flutter, JDK 17 and Ruby are installed by the actions in `.github/actions/`
(`subosito/flutter-action`, `actions/setup-java`, `ruby/setup-ruby` with a bundler cache); the
Android SDK and Xcode are preinstalled on the hosted images.

To use your own machines, set a repo variable holding a JSON array of labels:

```sh
gh variable set IOS_RUNNER     --repo $REPO --body '["self-hosted","macOS","ios"]'
gh variable set ANDROID_RUNNER --repo $REPO --body '["self-hosted","Linux","android"]'
```

Self-hosted requirements: the setup actions detect `runner.environment == 'self-hosted'` and then
use what is installed. Mac: Xcode, CocoaPods, Homebrew Ruby at `/usr/local/opt/ruby/bin` (Flutter is
still fetched by `flutter-action`). Linux: Ruby with bundler, `JAVA_HOME` and `ANDROID_HOME` in the
runner's `.env`. `tool/ci/setup-linux-android-runner.sh` and `tool/ci/register-org-runners.sh`
(`ORG=<your-org>`) set such runners up. Keep the repo **private** when it uses self-hosted runners:
they execute workflow code, so fork pull requests must never reach them. The iOS runner needs a
logged-in GUI session so `codesign` can use the keychain.

## Quick setup script

`tool/ci/setup-store-secrets.sh --build-aab` does the secret steps below from a Mac with `gh`
logged in: creates the Android upload keystore in `~/secure/<repo>-upload.jks` (password kept in the
login keychain; back the keystore up), sets the four `ANDROID_*` secrets, `PLAY_SERVICE_ACCOUNT_JSON`,
the `ASC_*` secrets and `CI_KEYCHAIN_PASSWORD` from hidden prompts, then builds the signed AAB for the
first manual Play upload. Values are never echoed or passed as arguments. Variables are set by hand.

## 1. Apple (App Store Connect) setup

- Enrol in the Apple Developer Program. The **Account Holder** must accept the current Program
  License Agreement or uploads fail with `FORBIDDEN.REQUIRED_AGREEMENTS_MISSING_OR_EXPIRED`.
- Create the app by hand in App Store Connect (the API key cannot create apps) with your bundle id,
  plus a TestFlight internal group. Add internal testers under *Users and Access*, then to the group
  (enable automatic distribution).
- Team id: developer.apple.com, *Membership details*. Set it as the variable `APPLE_TEAM_ID`.
- `ITSAppUsesNonExemptEncryption=false` is already set in `Info.plist` (export compliance: the app
  only uses HTTPS). Change it if you add your own cryptography.
- The 1024px icon (`flutter_app/assets/icon`) must be valid; App Store Connect rejects a build without it.

### Secrets

Create an API key in *Users and Access, Integrations* (Admin or App Manager):

```sh
gh secret set ASC_KEY_ID    --repo $REPO --body XXXXXXXXXX
gh secret set ASC_ISSUER_ID --repo $REPO --body xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
gh secret set ASC_KEY_P8    --repo $REPO < ~/.appstoreconnect/private_keys/AuthKey_XXXXXXXXXX.p8
gh variable set APPLE_TEAM_ID --repo $REPO --body ABCDE12345
# self-hosted Mac only:
openssl rand -base64 24 | tr -d '\n' | gh secret set CI_KEYCHAIN_PASSWORD --repo $REPO
```

**Signing** is fully automatic: the lane creates an *Apple Distribution* certificate and an App
Store profile through the API and signs with them. **Certificate limit**: Apple allows only a few
distribution certificates per team, and the first run on a new keychain creates one. A hosted runner
starts with an empty keychain every run, so each iOS release creates (and, if the limit is hit,
fails to create) a certificate; revoke stale ones at developer.apple.com, or use a self-hosted Mac
whose keychain persists, or point `CI_KEYCHAIN_PATH` at an existing keychain holding your
certificate and set `CI_KEYCHAIN_PASSWORD` to its password. The lane only unlocks and reuses it.

## 2. Android upload keystore

Make a new upload key for this app. Keep the `.jks` and passwords in a password manager: losing them
means a Play support request to reset the upload key.

```sh
mkdir -p ~/keys && cd ~/keys
keytool -genkeypair -v -keystore upload.jks -storetype PKCS12 \
  -alias upload -keyalg RSA -keysize 4096 -validity 10000 \
  -dname "CN=Your Name, O=Your Company, C=GB"
# PKCS12 uses one password for store and key: enter the same one twice if asked.
```

```sh
base64 -w0 ~/keys/upload.jks | gh secret set ANDROID_KEYSTORE_BASE64 --repo $REPO   # macOS: base64 -i
gh secret set ANDROID_KEYSTORE_PASSWORD --repo $REPO     # paste the keystore password
gh secret set ANDROID_KEY_ALIAS         --repo $REPO --body upload
gh secret set ANDROID_KEY_PASSWORD      --repo $REPO     # same password (PKCS12)
```

`flutter_app/android/app/build.gradle.kts` signs the release build from `APP_KEYSTORE_PATH`,
`APP_KEYSTORE_PASSWORD`, `APP_KEY_ALIAS`, `APP_KEY_PASSWORD` (set by `android-internal.yml` from the
secrets above), or from the git-ignored `flutter_app/android/key.properties` (`storeFile`,
`storePassword`, `keyAlias`, `keyPassword`) locally. With neither it falls back to the debug key
with a warning (never distributable), so a plain `flutter build apk --release` still works.

## 3. Google Play Console

1. **Service account**: in Google Cloud create one (IAM, Service accounts, Keys, add key, JSON) and
   enable the *Google Play Android Developer API*. Then:
   ```sh
   gh secret set PLAY_SERVICE_ACCOUNT_JSON --repo $REPO < ~/keys/play-service-account.json
   ```
2. **Grant it access**: Play Console, *Users and permissions*, invite the service account email, then
   *App permissions, Add app*, choose your app, with at least *View app information* and *Release apps
   to testing tracks*. Changes can take a few minutes (rarely 24 hours) to reach the API.
3. **The first upload must be manual.** Google requires the first App Bundle of a new app to be
   uploaded in the console. Build it locally, signed with the same upload key CI will use:
   ```sh
   cd flutter_app
   export APP_KEYSTORE_PATH=~/keys/upload.jks
   export APP_KEYSTORE_PASSWORD='...' APP_KEY_ALIAS=upload APP_KEY_PASSWORD='...'
   flutter build appbundle --release --target-platform android-arm,android-arm64 \
     --build-name=0.1.0 --build-number=1 --dart-define=API_BASE_URL=https://api.example.com
   # -> build/app/outputs/bundle/release/app-release.aab
   ```
   Play Console, *Test and release, Testing, Internal testing, Create new release*, keep **Play App
   Signing** (Google holds the app signing key; yours is only the upload key), upload the `.aab`,
   *Save*, *Review release*, *Start rollout to Internal testing*. CI picks version codes above
   anything already on Play, so starting at 1 is fine.
4. **Testers**: *Internal testing, Testers*: create an email list, tick it, and send testers the
   opt-in link (they must accept it before the Play Store shows the app).
5. If *Start rollout* is blocked, complete the *Set up your app* items it flags (see
   [STORE_SUBMISSION.md](STORE_SUBMISSION.md)).

### Draft app caveat

Until the app has had a release rolled out, the Play API only accepts `draft` releases ("Only
releases with status draft may be created on draft app"). Until then set the repo variable, and roll
each draft out by hand in Play Console:

```sh
gh variable set PLAY_RELEASE_STATUS --repo $REPO --body draft
```

Once the app is no longer a draft app, delete it (`gh variable delete PLAY_RELEASE_STATUS --repo
$REPO`); unset means `completed`.

### Listing upload needs a build

The `play-listing.yml` workflow (supply) attaches the listing to the newest **internal** version
code, so at least one internal build must exist (step 3 or a release). Without one the lane stops
with "No internal build on Play yet".

## Checklist (order)

1. Create the store apps (App Store Connect app + group; Play app) and fill the listing placeholders
   ([README](../README.md#customise-your-store-listing-aso)).
2. Set `APPLE_TEAM_ID`, `API_BASE_URL` and the Apple secrets (section 1).
3. Create the keystore and Android secrets (section 2).
4. Set up the Play service account and permissions; upload the first AAB by hand; set
   `PLAY_RELEASE_STATUS=draft` if the app is still a draft (section 3).
5. Cut the first release from a tag, see [RELEASING.md](RELEASING.md).
