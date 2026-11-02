# Releasing

Mobile store builds happen only when a GitHub release is published (or a workflow is run by hand
with a tag). Pushes to `main` and pull requests run `ci.yml` checks only; nothing is uploaded.
Without the store secrets the store workflows skip with a notice.

One-time setup (accounts, keys, secrets, runners): [STORES.md](STORES.md).

## Cut a release

```sh
# from an up-to-date main (or any commit you want to ship)
gh release create v0.1.0 --target main --generate-notes
# a pre-release (still goes to the internal tracks)
gh release create v0.2.0-rc.1 --target main --generate-notes --prerelease
```

- Tags must be semver with a `v`: `vMAJOR.MINOR.PATCH`, optionally `-pre.release`
  (e.g. `v1.2.3-rc.1`). Anything else fails the workflows before they build.
- **No version bump in `flutter_app/pubspec.yaml`**: the app version comes from the tag (`v1.2.3` and
  `v1.2.3-rc.1` become `1.2.3`, passed as `--build-name`). `version:` in pubspec is only used for
  local builds.
- Build numbers are computed: each store build gets `max(highest number on App Store Connect / Play
  + 1, GITHUB_RUN_NUMBER)` (`tool/fastlane/build_info.rb`), so re-runs and re-releases never collide.
- Two releases with the same `X.Y.Z` (e.g. `v1.2.3-rc.1` then `v1.2.3`) both upload: same version
  name, higher build number.

### A GitHub release also deploys the webapp

`release.yml` runs on the same event: Go binaries, the Flutter web zip and APK, and the **Cloudflare
deploy of the webapp** (production). So do not publish a GitHub release just to get a test build from a
feature branch. For an internal test build of a branch, tag the commit and dispatch the store
workflows by hand:

```sh
git tag v0.3.0-rc.1 <branch-or-commit> && git push origin v0.3.0-rc.1   # a tag, not a GitHub release
gh workflow run ios-internal.yml     -f tag=v0.3.0-rc.1
gh workflow run android-internal.yml -f tag=v0.3.0-rc.1
```

## What runs on a release

| Workflow | What it does | Runner (default) |
|---|---|---|
| `release.yml` | GoReleaser binaries, Flutter web zip, APK, webapp zip + Cloudflare deploy | `ubuntu-latest` |
| `ios-internal.yml` | gate, `flutter-checks.yml`, then signed IPA uploaded to TestFlight | `macos-latest` (`IOS_RUNNER`) |
| `android-internal.yml` | gate, `flutter-checks.yml`, then signed AAB (arm/arm64) to the Play **internal** track | `ubuntu-latest` (`ANDROID_RUNNER`) |
| `flutter-checks.yml` | Reusable: `flutter analyze`, `flutter test` and the Ruby tooling tests (incl. the listing checker) at the release tag | `ubuntu-latest` (`ANDROID_RUNNER`) |
| `store-gate.yml` | Reusable: skips a store workflow with a `::notice::` when its secrets are missing | `ubuntu-latest` |

Both store workflows use a concurrency group with `cancel-in-progress: false`: an upload in progress
always finishes; newer ones queue. Checks must pass before anything is uploaded.

### Build-time configuration of store builds

| `--dart-define` | Source |
|---|---|
| `API_BASE_URL` | repo variable `API_BASE_URL`; unset falls back to the app default (`localhost`), so set it |
| `GOOGLE_SERVER_CLIENT_ID`, `GOOGLE_IOS_CLIENT_ID` | optional repo variables |
| `APP_VERSION` | the release tag without `v` |

`config/dev.json` is never used in store builds. The list is built in one place,
`AppConfig.dart_defines` in `tool/fastlane/app_config.rb`; add new store-build settings there.

## Re-running

- A failed job: *Actions, the run, Re-run failed jobs* (build numbers still go up).
- A fresh run for an existing tag (e.g. after fixing a secret):
  ```sh
  gh workflow run ios-internal.yml     -f tag=v0.1.0
  gh workflow run android-internal.yml -f tag=v0.1.0
  ```
  Manual runs build exactly `refs/tags/<tag>`.
- Something wrong in the release: fix on `main` and cut a new tag (`v0.1.1`); do not move tags.

## Draft Play app

While the Play app has no rolled-out release yet (a "draft app"), Play rejects releases that are not
drafts. Set the repo variable `PLAY_RELEASE_STATUS=draft`: builds then land as drafts which you roll
out by hand in Play Console. Delete the variable once the app has a first rollout.

## Store listing

How to write and where to edit the listing: [README, "Customise your store listing"](../README.md#customise-your-store-listing-aso).
The upload flows:

**Google Play**: *Actions, Play listing, Run workflow* (optional `ref`, default `main`), or locally with
`PLAY_SERVICE_ACCOUNT_JSON` set:

```sh
cd flutter_app/android && bundle exec fastlane listing
```

It uploads text, icon, feature graphic and screenshots only (no build, no changelogs) and compares
sha256, so unchanged images are not re-uploaded. It attaches to the newest internal version code, so
one internal build must exist first (supply quirk, see STORES.md). The `release` lane also uploads
`images/icon.png` to every existing listing language after the AAB; a failure only warns (Play takes
its hi-res icon from the listing, never from the AAB).

**App Store**: *Actions, iOS metadata, Run workflow*. `tag` is the App Store version to attach the
listing to (created or edited, e.g. `v0.3.1`); `ref` (default `main`) is where the listing files come
from, since older tags may lack the lane. Or locally with the `ASC_*` variables set:

```sh
cd flutter_app/ios && RELEASE_TAG=v0.3.1 bundle exec fastlane metadata
```

The lane uploads text and screenshots only (no build, no submission, no automatic release) and
replaces existing screenshots. Gotchas:

- If the version has no *App Review Information* yet, `deliver` crashes with "No data"; the lane
  creates an empty review detail and retries. Fill in contact and demo-account details in App Store Connect.
- App Store precheck rejects the word "Google" or "Android" in iOS text (also in the description of an
  app that offers Google sign-in: say "other providers"), and any URL that returns 404 on a HEAD request
  (privacy, support, marketing URLs must exist before you upload).

## Local store builds

```sh
bundle install                                   # fastlane, pinned by Gemfile.lock
cd flutter_app/ios && bundle exec fastlane internal      # ASC_* + APPLE_TEAM_ID + CI_KEYCHAIN_PASSWORD env
cd flutter_app/android && bundle exec fastlane internal  # PLAY_SERVICE_ACCOUNT_JSON + APP_KEYSTORE_* env
RELEASE_TAG=v0.1.0 bundle exec fastlane internal         # version from a tag instead of pubspec
```

Ruby tests (no gems needed): `for t in tool/fastlane/*_test.rb; do ruby $t; done`.
