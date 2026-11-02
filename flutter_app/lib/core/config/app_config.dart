/// Compile-time configuration, supplied via `--dart-define`.
///
/// Example: `flutter run --dart-define=API_BASE_URL=http://10.0.2.2:8080`
/// (10.0.2.2 is the host machine as seen from the Android emulator).
abstract final class AppConfig {
  static const String apiBaseUrl = String.fromEnvironment(
    'API_BASE_URL',
    defaultValue: 'http://localhost:8080',
  );

  /// Web OAuth client ID (the ID token audience). Optional: when empty the app
  /// uses `web_client_id` from `GET /auth/providers`. Required by the Android
  /// SDK to receive an ID token at all.
  static const String googleServerClientId = String.fromEnvironment(
    'GOOGLE_SERVER_CLIENT_ID',
  );

  /// iOS OAuth client ID (also set `GOOGLE_IOS_CLIENT_ID` in
  /// `ios/Flutter/Google.local.xcconfig`, see the README).
  static const String googleIosClientId = String.fromEnvironment(
    'GOOGLE_IOS_CLIENT_ID',
  );

  static const String apiPrefix = '/api/v1';

  /// Base URL including the `/api/v1` prefix, without a trailing slash.
  static String get apiRoot =>
      '${apiBaseUrl.replaceFirst(RegExp(r'/+$'), '')}$apiPrefix';

  static const String appName = 'SaaS Starter';
}
