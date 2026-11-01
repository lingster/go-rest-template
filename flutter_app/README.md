# flutter_app

Flutter client for the SaaS starter API (see `../docs/API_CONTRACT.md`).
Targets iOS, Android and web. Material 3, light/dark, Riverpod + go_router + dio.

## Run

```sh
cd flutter_app
flutter pub get
flutter run                       # API at http://localhost:8080
flutter run -d chrome --web-port 5173
```

### Configuration (dart-define)

| Define | Default | Notes |
|---|---|---|
| `API_BASE_URL` | `http://localhost:8080` | Server root without `/api/v1` (added automatically). |

Config is read with `String.fromEnvironment`, so it is baked in at build time
(also used by release CI):

```sh
flutter run --dart-define=API_BASE_URL=http://10.0.2.2:8080        # Android emulator -> host machine
flutter build web --release --dart-define=API_BASE_URL=https://api.example.com
flutter build apk --release --dart-define=API_BASE_URL=https://api.example.com
```

`localhost` inside the Android emulator is the emulator itself; use `10.0.2.2`.
iOS simulator can use `localhost`. The backend `CORS_ALLOWED_ORIGINS` must
include the web origin when running on web.

## Checks

```sh
flutter analyze
flutter test
flutter build web
flutter build apk --debug
```

## Architecture

Feature-first, with `data` (API/models) -> `application` (Riverpod notifiers) -> `presentation` (widgets).

```
lib/
  main.dart, app.dart, router.dart, core/theme.dart
  core/
    config/app_config.dart        API_BASE_URL
    network/api_client.dart       dio wrapper -> ApiException
    network/api_exception.dart    contract error body -> typed ApiErrorCode
    network/auth_interceptor.dart bearer header + single-flight refresh on 401 + retry
    storage/token_storage.dart    TokenStorage interface (secure storage / in-memory)
    storage/biometric_prefs.dart  unset | enabled | disabled
    providers.dart                DI wiring (storage, api client, platform services)
  features/
    auth/      models (User, Tokens, sealed AuthResult), AuthRepository,
               AuthController (AuthLoading | Unauthenticated | AuthLocked | Authenticated),
               screens: splash, login, register, forgot/reset password, verify email, 2FA, lock
    profile/   edit name, change password, logout
    security/  TOTP 2FA (QR, recovery codes), passkeys, biometric toggle,
               PasskeyService / BiometricService interfaces
    home/      placeholder dashboard
  shared/      validators, async_action_mixin, widgets (form page, text/password fields,
               primary button, banners)
```

Key behaviours:

- **Refresh**: on 401 (non `/auth/*` calls) the interceptor calls `/auth/refresh` once
  (concurrent failures share the same future), stores the rotated pair and retries.
  If refresh fails, tokens are cleared and `AuthController.sessionExpired()` sends the user to login.
- **Router guards** (`authRedirect` in `router.dart`): loading -> `/splash`; locked -> `/lock`;
  signed out -> `/login` except public routes; signed in -> away from auth screens.
  `/verify-email?token=` and `/reset-password?token=` are reachable in any state.
  Web uses path URLs (`usePathUrlStrategy`) so email links work; configure your host with SPA fallback to `index.html`.
- **MFA**: login returns `MfaChallenge`; the challenge is passed in memory to `/2fa`.
- **email_not_verified** on login shows a "Resend verification email" action.

### Add a feature

1. `lib/features/<name>/data/<name>_repository.dart` - takes `ApiClient`, returns models; throws `ApiException`.
2. `lib/features/<name>/application/<name>_providers.dart` - `Provider` for the repository and
   a `Notifier`/`AsyncNotifier`/`FutureProvider` for state.
3. `lib/features/<name>/presentation/<name>_screen.dart` - use `FormPage`, `AppTextField`,
   `PrimaryButton`, `ErrorBanner` and `AsyncActionMixin` for loading/error handling.
4. Add a `GoRoute` in `router.dart` (protected by default; add to `_guestOnly`/`_always` if public).
5. Test the repository with `FakeAdapter` (`test/helpers/fake_adapter.dart`) and screens with `pumpApp`.

## Biometric unlock (iOS / Android)

Uses `local_auth` behind `BiometricService` (mockable; always unsupported on web, where all
biometric UI is hidden).

- After a successful login, if the device has enrolled biometrics and the user has not decided, Home offers
  "Enable biometric unlock?". The choice is stored in secure storage.
- On app start with a stored refresh token and biometrics enabled, the biometric prompt is shown BEFORE the
  refresh token is used. Success restores the session; cancel/failure shows the lock screen
  ("Try again" / "Use password instead", which logs out and clears the session).
- Login shows "Sign in with biometrics" when a stored session and biometric unlock exist.
- Security screen has a toggle; enabling and disabling both require a biometric confirmation.

Platform setup (already applied in this template):

- iOS `ios/Runner/Info.plist`: `NSFaceIDUsageDescription`.
- Android `AndroidManifest.xml`: `<uses-permission android:name="android.permission.USE_BIOMETRIC"/>`;
  `MainActivity` extends `FlutterFragmentActivity` (required by local_auth).
  The device needs an enrolled fingerprint/face.

## Passkeys

`PasskeyService` wraps the [`passkeys`](https://pub.dev/packages/passkeys) plugin. Server `options.publicKey`
(go-webauthn) is converted to the plugin request and the result back to standard WebAuthn JSON
(`id, rawId, type, response{clientDataJSON, attestationObject | authenticatorData, signature, userHandle}`,
base64url). Note go-webauthn omits empty `transports`; `publicKeyOf` normalises this for the plugin.

Required platform setup:

- **RP ID must match the domain** that serves the association files, and match the backend
  `WEBAUTHN_RP_ID` / `WEBAUTHN_RP_ORIGINS`. `localhost` works for web only; native apps need a real HTTPS domain.
- **iOS** (13+, real passkeys need iOS 16+): add the *Associated Domains* capability with
  `webcredentials:your-domain.com` in Xcode (Runner -> Signing & Capabilities) and serve
  `https://your-domain.com/.well-known/apple-app-site-association`:
  ```json
  {"webcredentials": {"apps": ["TEAMID.com.example.saasApp"]}}
  ```
- **Android** (minSdk 24 set in `android/app/build.gradle.kts`; passkeys needs 23+, local_auth 24+; Android 9+ devices with Google Play services recommended):
  serve `https://your-domain.com/.well-known/assetlinks.json`:
  ```json
  [{"relation": ["delegate_permission/common.handle_all_urls",
                 "delegate_permission/common.get_login_creds"],
    "target": {"namespace": "android_app", "package_name": "com.example.saas_app",
               "sha256_cert_fingerprints": ["<SHA-256 of your signing cert>"]}}]
  ```
  Native passkey origin is `android:apk-key-hash:<base64url sha256>`; add it to `WEBAUTHN_RP_ORIGINS`.
- **Web**: `web/index.html` loads the plugin's JS bundle from `web/passkeys_bundle.js` (a local copy of
  corbado/flutter-passkeys 2.4.0 `bundle.js`). WebAuthn requires HTTPS (or localhost).

## Rename the package / bundle id for a new project

1. `pubspec.yaml`: change `name:` and replace `package:saas_app/` imports
   (`grep -rl saas_app lib test | xargs sed -i '' 's/package:saas_app/package:NEW/g'` on macOS).
2. Android: `namespace` and `applicationId` in `android/app/build.gradle.kts`; move
   `android/app/src/main/kotlin/com/example/saas_app/MainActivity.kt` to the new package path and update its `package` line;
   update `android:label` in the manifest.
3. iOS: `PRODUCT_BUNDLE_IDENTIFIER` in `ios/Runner.xcodeproj/project.pbxproj` and display name in `Info.plist`;
   update the apple-app-site-association app id.
4. Web: `web/index.html` title and `web/manifest.json`.
5. Update `AppConfig.appName`, then `flutter clean && flutter pub get`.
   (Alternative: `dart pub global activate rename` then `rename setBundleId` / `setAppName`.)

## Known limitations

- Access tokens are refreshed only reactively (on 401), not proactively before expiry.
- Passkey flows cannot be exercised in unit tests (platform plugin); conversion logic and the repository flow are tested with a fake `PasskeyService`.
- On web, `flutter_secure_storage` uses encrypted localStorage (weaker than native keystores).
