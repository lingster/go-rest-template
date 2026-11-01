import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../features/auth/application/auth_controller.dart';
import '../features/auth/data/google_sign_in_service.dart';
import '../features/security/data/biometric_service.dart';
import '../features/security/data/passkey_service.dart';
import 'config/app_config.dart';
import 'network/api_client.dart';
import 'network/auth_interceptor.dart';
import 'network/dio_factory.dart';
import 'storage/biometric_prefs.dart';
import 'storage/token_storage.dart';

final tokenStorageProvider = Provider<TokenStorage>(
  (ref) => SecureTokenStorage(),
);

final biometricServiceProvider = Provider<BiometricService>(
  (ref) => LocalAuthBiometricService(),
);

final biometricPrefsProvider = Provider<BiometricPrefs>(
  (ref) => SecureBiometricPrefs(),
);

final googleSignInServiceProvider = Provider<GoogleSignInService>(
  (ref) => PluginGoogleSignInService(),
);

final passkeyServiceProvider = Provider<PasskeyService>(
  (ref) => PluginPasskeyService(),
);

/// Authenticated API client (bearer token + single-flight refresh on 401).
final apiClientProvider = Provider<ApiClient>((ref) {
  final dio = createBaseDio(AppConfig.apiRoot);
  final bareDio = createBaseDio(AppConfig.apiRoot);
  dio.interceptors.add(
    AuthInterceptor(
      storage: ref.watch(tokenStorageProvider),
      dio: dio,
      refresh: createRefreshCallback(bareDio),
      onSessionExpired: () =>
          ref.read(authControllerProvider.notifier).sessionExpired(),
    ),
  );
  return ApiClient(dio);
});
