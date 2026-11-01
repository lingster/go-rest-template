class User {
  const User({
    required this.id,
    required this.email,
    required this.name,
    required this.role,
    required this.emailVerified,
    required this.isActive,
    required this.totpEnabled,
    required this.passkeyCount,
    this.hasPassword = true,
    this.identities = const [],
  });

  factory User.fromJson(Map<String, dynamic> json) => User(
    id: json['id'] as String,
    email: json['email'] as String,
    name: (json['name'] as String?) ?? '',
    role: (json['role'] as String?) ?? 'user',
    emailVerified: (json['email_verified'] as bool?) ?? false,
    isActive: (json['is_active'] as bool?) ?? true,
    totpEnabled: (json['totp_enabled'] as bool?) ?? false,
    passkeyCount: (json['passkey_count'] as num?)?.toInt() ?? 0,
    hasPassword: (json['has_password'] as bool?) ?? true,
    identities: ((json['identities'] as List?) ?? const []).cast<String>(),
  );

  final String id;
  final String email;
  final String name;
  final String role;
  final bool emailVerified;
  final bool isActive;
  final bool totpEnabled;
  final int passkeyCount;

  /// False for accounts created with Google until a password is set through
  /// the emailed reset link.
  final bool hasPassword;

  /// Linked sign-in providers, e.g. `['google']`.
  final List<String> identities;

  bool get isAdmin => role == 'admin';
}

class Tokens {
  const Tokens({
    required this.accessToken,
    required this.refreshToken,
    required this.expiresIn,
  });

  factory Tokens.fromJson(Map<String, dynamic> json) => Tokens(
    accessToken: json['access_token'] as String,
    refreshToken: json['refresh_token'] as String,
    expiresIn: (json['expires_in'] as num?)?.toInt() ?? 0,
  );

  final String accessToken;
  final String refreshToken;
  final int expiresIn;
}

/// Outcome of a login attempt: either a full session or an MFA challenge.
sealed class AuthResult {
  const AuthResult();

  factory AuthResult.fromJson(Map<String, dynamic> json) {
    if (json['mfa_required'] == true) {
      return MfaChallenge(
        mfaToken: json['mfa_token'] as String,
        methods: ((json['methods'] as List?) ?? const []).cast<String>(),
      );
    }
    return AuthSuccess(
      tokens: Tokens.fromJson(json['tokens'] as Map<String, dynamic>),
      user: User.fromJson(json['user'] as Map<String, dynamic>),
    );
  }
}

class AuthSuccess extends AuthResult {
  const AuthSuccess({required this.tokens, required this.user});
  final Tokens tokens;
  final User user;
}

class MfaChallenge extends AuthResult {
  const MfaChallenge({required this.mfaToken, required this.methods});
  final String mfaToken;
  final List<String> methods;
}

class RegisterResult {
  const RegisterResult({required this.user, required this.message});
  final User user;
  final String message;
}

/// Public sign-in provider configuration (`GET /auth/providers`).
class ProvidersInfo {
  const ProvidersInfo({
    this.googleEnabled = false,
    this.googleWebClientId = '',
  });

  factory ProvidersInfo.fromJson(Map<String, dynamic> json) {
    final google = json['google'];
    final g = google is Map ? google : const <String, dynamic>{};
    return ProvidersInfo(
      googleEnabled: (g['enabled'] as bool?) ?? false,
      googleWebClientId: (g['web_client_id'] as String?) ?? '',
    );
  }

  static const disabled = ProvidersInfo();

  final bool googleEnabled;
  final String googleWebClientId;
}
