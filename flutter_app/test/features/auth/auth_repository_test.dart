import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:saas_app/core/network/api_client.dart';
import 'package:saas_app/core/network/api_exception.dart';
import 'package:saas_app/core/storage/token_storage.dart';
import 'package:saas_app/features/auth/data/auth_models.dart';
import 'package:saas_app/features/auth/data/auth_repository.dart';
import 'package:saas_app/features/security/data/passkey_service.dart';

import '../../helpers/fake_adapter.dart';

class _FakePasskeys implements PasskeyService {
  Map<String, dynamic>? lastOptions;

  @override
  Future<Map<String, dynamic>> createCredential(
    Map<String, dynamic> options,
  ) async => {};

  @override
  Future<Map<String, dynamic>> getCredential(
    Map<String, dynamic> options,
  ) async {
    lastOptions = options;
    return {
      'id': 'cred',
      'rawId': 'cred',
      'type': 'public-key',
      'response': {},
    };
  }
}

void main() {
  late InMemoryTokenStorage storage;
  late FakeAdapter adapter;
  late AuthRepository repo;
  late _FakePasskeys passkeys;

  void setup(FakeHandler handler) {
    storage = InMemoryTokenStorage();
    adapter = FakeAdapter(handler);
    passkeys = _FakePasskeys();
    final dio = Dio(BaseOptions(baseUrl: 'http://x/api/v1'))
      ..httpClientAdapter = adapter;
    repo = AuthRepository(
      api: ApiClient(dio),
      storage: storage,
      passkeys: passkeys,
    );
  }

  test('login success stores tokens and returns the user', () async {
    setup((o) {
      expect(o.path, '/auth/login');
      expect(o.data, {'email': 'a@b.com', 'password': 'pw'});
      return FakeResponse(200, {
        'mfa_required': false,
        'tokens': tokensJson(),
        'user': userJson(),
      });
    });
    final r = await repo.login('a@b.com', 'pw');
    expect(r, isA<AuthSuccess>());
    expect((r as AuthSuccess).user.email, 'a@b.com');
    expect(await storage.readAccessToken(), 'access1');
    expect(await storage.readRefreshToken(), 'refresh1');
  });

  test('login returning an MFA challenge does not store tokens', () async {
    setup(
      (_) => FakeResponse(200, {
        'mfa_required': true,
        'mfa_token': 'mfa',
        'methods': ['totp', 'recovery_code'],
      }),
    );
    final r = await repo.login('a@b.com', 'pw');
    expect(r, isA<MfaChallenge>());
    expect((r as MfaChallenge).mfaToken, 'mfa');
    expect(r.methods, contains('totp'));
    expect(await storage.readRefreshToken(), isNull);
  });

  test('login maps API errors to ApiException', () async {
    setup(
      (_) => FakeResponse(403, errorJson('email_not_verified', 'verify first')),
    );
    await expectLater(
      repo.login('a@b.com', 'pw'),
      throwsA(
        isA<ApiException>().having(
          (e) => e.code,
          'code',
          ApiErrorCode.emailNotVerified,
        ),
      ),
    );
  });

  test('loginWithMfa posts mfa_token + code and stores tokens', () async {
    setup((o) {
      expect(o.path, '/auth/login/2fa');
      expect(o.data, {'mfa_token': 'mfa', 'code': '123456'});
      return FakeResponse(200, {
        'mfa_required': false,
        'tokens': tokensJson('2'),
        'user': userJson(totp: true),
      });
    });
    final r = await repo.loginWithMfa('mfa', '123456');
    expect(r.user.totpEnabled, isTrue);
    expect(await storage.readAccessToken(), 'access2');
  });

  test('passkey login runs begin -> platform -> finish', () async {
    setup((o) {
      if (o.path == '/auth/passkey/login/begin') {
        return FakeResponse(200, {
          'session_id': 's1',
          'options': {
            'publicKey': {'challenge': 'abc', 'rpId': 'localhost'},
          },
        });
      }
      expect(o.path, '/auth/passkey/login/finish');
      expect((o.data as Map)['session_id'], 's1');
      expect((o.data as Map)['credential'], containsPair('id', 'cred'));
      return FakeResponse(200, {
        'mfa_required': false,
        'tokens': tokensJson(),
        'user': userJson(),
      });
    });
    final r = await repo.loginWithPasskey();
    expect(r.user.id, 'u1');
    expect(
      passkeys.lastOptions!['publicKey'],
      containsPair('challenge', 'abc'),
    );
    expect(await storage.readRefreshToken(), 'refresh1');
  });

  test('register returns user and message', () async {
    setup(
      (_) => FakeResponse(201, {'user': userJson(), 'message': 'check email'}),
    );
    final r = await repo.register(
      email: 'a@b.com',
      password: 'password1',
      name: 'Ada',
    );
    expect(r.message, 'check email');
    expect(r.user.name, 'Ada');
  });

  test('restoreSession returns null without a stored token', () async {
    setup((_) => FakeResponse(500));
    expect(await repo.restoreSession(), isNull);
    expect(adapter.requests, isEmpty);
  });

  test('restoreSession clears tokens when the server rejects them', () async {
    setup((_) => FakeResponse(403, errorJson('account_disabled')));
    await storage.save(accessToken: 'a', refreshToken: 'r');
    expect(await repo.restoreSession(), isNull);
    expect(await storage.readRefreshToken(), isNull);
  });

  test('restoreSession keeps tokens on network failure', () async {
    setup(
      (o) => throw DioException(
        requestOptions: o,
        type: DioExceptionType.connectionError,
      ),
    );
    await storage.save(accessToken: 'a', refreshToken: 'r');
    await expectLater(
      repo.restoreSession(),
      throwsA(
        isA<ApiException>().having((e) => e.code, 'code', ApiErrorCode.network),
      ),
    );
    expect(await storage.readRefreshToken(), 'r');
  });

  test('logout clears local tokens even if the server call fails', () async {
    setup((_) => FakeResponse(500, errorJson('internal_error')));
    await storage.save(accessToken: 'a', refreshToken: 'r');
    await repo.logout();
    expect(await storage.readRefreshToken(), isNull);
    expect(adapter.requests.single.path, '/auth/logout');
    expect(adapter.requests.single.data, {'refresh_token': 'r'});
  });
}
