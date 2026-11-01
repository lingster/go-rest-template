import '../../../core/network/api_client.dart';
import '../../../core/network/api_exception.dart';
import '../../../core/storage/token_storage.dart';
import '../../security/data/passkey_service.dart';
import 'auth_models.dart';

/// All unauthenticated + session-lifecycle API calls. Persists tokens on
/// successful login so the rest of the app only deals with [User].
class AuthRepository {
  AuthRepository({
    required this.api,
    required this.storage,
    required this.passkeys,
  });

  final ApiClient api;
  final TokenStorage storage;
  final PasskeyService passkeys;

  Future<AuthResult> login(String email, String password) async {
    final result = await api.post(
      '/auth/login',
      body: {'email': email, 'password': password},
      decode: (j) => AuthResult.fromJson(asMap(j)),
    );
    return _persist(result);
  }

  Future<AuthSuccess> loginWithMfa(String mfaToken, String code) async {
    final result = await api.post(
      '/auth/login/2fa',
      body: {'mfa_token': mfaToken, 'code': code},
      decode: (j) => AuthResult.fromJson(asMap(j)),
    );
    return await _persist(result) as AuthSuccess;
  }

  /// Passkey sign-in. [email] may be omitted for discoverable credentials.
  Future<AuthSuccess> loginWithPasskey({String? email}) async {
    final begin = await api.post(
      '/auth/passkey/login/begin',
      body: {if (email != null && email.isNotEmpty) 'email': email},
      decode: asMap,
    );
    final credential = await passkeys.getCredential(asMap(begin['options']));
    final result = await api.post(
      '/auth/passkey/login/finish',
      body: {'session_id': begin['session_id'], 'credential': credential},
      decode: (j) => AuthResult.fromJson(asMap(j)),
    );
    return await _persist(result) as AuthSuccess;
  }

  /// Sign-in providers the server has enabled. Never throws: a failure just
  /// means "no external providers".
  Future<ProvidersInfo> providers() async {
    try {
      return await api.get(
        '/auth/providers',
        decode: (j) => ProvidersInfo.fromJson(asMap(j)),
      );
    } on ApiException {
      return ProvidersInfo.disabled;
    }
  }

  /// Exchanges a Google ID token for a session, or an MFA challenge when the
  /// account has 2FA enabled.
  Future<AuthResult> loginWithGoogle(String idToken) async {
    final result = await api.post(
      '/auth/google',
      body: {'id_token': idToken},
      decode: (j) => AuthResult.fromJson(asMap(j)),
    );
    return _persist(result);
  }

  Future<RegisterResult> register({
    required String email,
    required String password,
    required String name,
  }) => api.post(
    '/auth/register',
    body: {'email': email, 'password': password, 'name': name},
    decode: (j) {
      final m = asMap(j);
      return RegisterResult(
        user: User.fromJson(asMap(m['user'])),
        message: (m['message'] as String?) ?? '',
      );
    },
  );

  Future<String> verifyEmail(String token) => api.post(
    '/auth/verify-email',
    body: {'token': token},
    decode: decodeMessage,
  );

  Future<String> resendVerification(String email) => api.post(
    '/auth/resend-verification',
    body: {'email': email},
    decode: decodeMessage,
  );

  Future<String> forgotPassword(String email) => api.post(
    '/auth/forgot-password',
    body: {'email': email},
    decode: decodeMessage,
  );

  Future<String> resetPassword(String token, String password) => api.post(
    '/auth/reset-password',
    body: {'token': token, 'password': password},
    decode: decodeMessage,
  );

  Future<User> fetchCurrentUser() =>
      api.get('/me', decode: (j) => User.fromJson(asMap(j)));

  Future<bool> hasStoredSession() async =>
      await storage.readRefreshToken() != null;

  /// Returns the stored session's user, or null if there is no valid session.
  /// Network failures propagate so the caller can decide.
  Future<User?> restoreSession() async {
    if (await storage.readRefreshToken() == null) return null;
    try {
      return await fetchCurrentUser();
    } on ApiException catch (e) {
      if (e.code == ApiErrorCode.network) rethrow;
      await storage.clear();
      return null;
    }
  }

  /// Best-effort server logout, always clears local tokens.
  Future<void> logout() async {
    final refreshToken = await storage.readRefreshToken();
    try {
      if (refreshToken != null) {
        await api.postVoid(
          '/auth/logout',
          body: {'refresh_token': refreshToken},
        );
      }
    } on ApiException {
      // Ignore: local logout must always succeed.
    } finally {
      await storage.clear();
    }
  }

  Future<AuthResult> _persist(AuthResult result) async {
    if (result is AuthSuccess) {
      await storage.save(
        accessToken: result.tokens.accessToken,
        refreshToken: result.tokens.refreshToken,
      );
    }
    return result;
  }
}
