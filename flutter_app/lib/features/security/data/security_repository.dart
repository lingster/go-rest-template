import '../../../core/network/api_client.dart';
import 'passkey_service.dart';

class TotpSetup {
  const TotpSetup({required this.secret, required this.otpauthUrl});
  final String secret;
  final String otpauthUrl;
}

class Passkey {
  const Passkey({
    required this.id,
    required this.name,
    required this.createdAt,
    this.lastUsedAt,
  });

  factory Passkey.fromJson(Map<String, dynamic> json) => Passkey(
    id: json['id'] as String,
    name: (json['name'] as String?) ?? 'Passkey',
    createdAt: DateTime.tryParse('${json['created_at']}'),
    lastUsedAt: json['last_used_at'] == null
        ? null
        : DateTime.tryParse('${json['last_used_at']}'),
  );

  final String id;
  final String name;
  final DateTime? createdAt;
  final DateTime? lastUsedAt;
}

List<String> _recoveryCodes(Object? json) =>
    ((asMap(json)['recovery_codes'] as List?) ?? const []).cast<String>();

class SecurityRepository {
  SecurityRepository({required this.api, required this.passkeys});

  final ApiClient api;
  final PasskeyService passkeys;

  Future<TotpSetup> setupTotp() => api.post(
    '/me/2fa/totp/setup',
    decode: (j) {
      final m = asMap(j);
      return TotpSetup(
        secret: m['secret'] as String,
        otpauthUrl: m['otpauth_url'] as String,
      );
    },
  );

  Future<List<String>> enableTotp(String code) => api.post(
    '/me/2fa/totp/enable',
    body: {'code': code},
    decode: _recoveryCodes,
  );

  Future<void> disableTotp({required String password, required String code}) =>
      api.postVoid(
        '/me/2fa/totp/disable',
        body: {'password': password, 'code': code},
      );

  Future<List<String>> regenerateRecoveryCodes(String code) => api.post(
    '/me/2fa/recovery-codes',
    body: {'code': code},
    decode: _recoveryCodes,
  );

  Future<List<Passkey>> listPasskeys() => api.get(
    '/me/passkeys',
    decode: (j) => ((j as List?) ?? const [])
        .map((e) => Passkey.fromJson(asMap(e)))
        .toList(),
  );

  Future<Passkey> addPasskey(String name) async {
    final begin = await api.post('/me/passkeys/register/begin', decode: asMap);
    final credential = await passkeys.createCredential(asMap(begin['options']));
    return api.post(
      '/me/passkeys/register/finish',
      body: {
        'session_id': begin['session_id'],
        'name': name,
        'credential': credential,
      },
      decode: (j) => Passkey.fromJson(asMap(j)),
    );
  }

  Future<void> deletePasskey(String id) => api.delete('/me/passkeys/$id');
}
