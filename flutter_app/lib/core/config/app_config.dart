/// Compile-time configuration, supplied via `--dart-define`.
///
/// Example: `flutter run --dart-define=API_BASE_URL=http://10.0.2.2:8080`
/// (10.0.2.2 is the host machine as seen from the Android emulator).
abstract final class AppConfig {
  static const String apiBaseUrl = String.fromEnvironment(
    'API_BASE_URL',
    defaultValue: 'http://localhost:8080',
  );

  static const String apiPrefix = '/api/v1';

  /// Base URL including the `/api/v1` prefix, without a trailing slash.
  static String get apiRoot =>
      '${apiBaseUrl.replaceFirst(RegExp(r'/+$'), '')}$apiPrefix';

  static const String appName = 'SaaS App';
}
