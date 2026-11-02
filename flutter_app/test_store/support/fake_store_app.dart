// Fake providers for the store screenshots: the real app and screens, no
// network, no platform plugins. Edit the data here to change what the
// screenshots show.
import 'package:flutter_riverpod/misc.dart' show Override;
import 'package:saas_app/core/config/app_config.dart';
import 'package:saas_app/core/network/api_client.dart';
import 'package:saas_app/core/network/dio_factory.dart';
import 'package:saas_app/core/providers.dart';
import 'package:saas_app/core/storage/biometric_prefs.dart';
import 'package:saas_app/core/storage/token_storage.dart';
import 'package:saas_app/features/auth/application/auth_controller.dart';
import 'package:saas_app/features/auth/data/auth_models.dart';
import 'package:saas_app/features/auth/data/google_sign_in_service.dart';
import 'package:saas_app/features/security/data/biometric_service.dart';

import '../../test/helpers/fake_adapter.dart';

/// The signed-in user of the screenshots.
Map<String, dynamic> storeUserJson() => {
  ...userJson(totp: true),
  'name': 'Alex Morgan',
  'email': 'alex@example.com',
  'passkey_count': 1,
};

/// Passkeys listed on the security screen.
const storePasskeys = <Map<String, dynamic>>[
  {
    'id': 'p1',
    'name': 'iPhone',
    'created_at': '2026-09-02T09:00:00Z',
    'last_used_at': '2026-10-30T08:12:00Z',
  },
];

class _StoreSession extends AuthController {
  _StoreSession(this._state);
  final AuthState _state;

  @override
  AuthState build() => _state;
}

/// Pretends the device has biometrics, so that row shows on the security screen.
class _StoreBiometrics implements BiometricService {
  @override
  Future<bool> isSupported() async => true;

  @override
  Future<bool> authenticate(String reason) async => true;
}

/// Pretends the platform supports Google sign-in, so its button shows.
class _StoreGoogle implements GoogleSignInService {
  @override
  bool get isSupported => true;

  @override
  Future<String?> requestIdToken({String? serverClientId}) async => null;
}

/// An API that answers the few reads the screens make.
ApiClient _storeApi() {
  final dio = createBaseDio(AppConfig.apiRoot)
    ..httpClientAdapter = FakeAdapter((options) {
      return switch (options.path) {
        '/me/passkeys' => FakeResponse(200, storePasskeys),
        '/me' => FakeResponse(200, storeUserJson()),
        _ => FakeResponse(404, errorJson('not_found')),
      };
    });
  return ApiClient(dio);
}

/// Provider overrides for a store screenshot, signed in or on the sign-in page.
List<Override> storeOverrides({required bool signedIn}) {
  return [
    apiClientProvider.overrideWithValue(_storeApi()),
    tokenStorageProvider.overrideWithValue(InMemoryTokenStorage()),
    authControllerProvider.overrideWith(
      () => _StoreSession(
        signedIn
            ? Authenticated(User.fromJson(storeUserJson()))
            : const Unauthenticated(),
      ),
    ),
    signInProvidersProvider.overrideWith(
      (ref) async => const ProvidersInfo(
        googleEnabled: true,
        googleWebClientId: 'store.apps.googleusercontent.com',
      ),
    ),
    biometricServiceProvider.overrideWithValue(_StoreBiometrics()),
    googleSignInServiceProvider.overrideWithValue(_StoreGoogle()),
    // "enabled" so the home screen does not open the unlock offer dialog.
    biometricPrefsProvider.overrideWithValue(
      InMemoryBiometricPrefs(BiometricChoice.enabled),
    ),
  ];
}
