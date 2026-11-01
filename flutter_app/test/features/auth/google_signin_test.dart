import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';
import 'package:saas_app/core/network/api_client.dart';
import 'package:saas_app/core/storage/token_storage.dart';
import 'package:saas_app/features/auth/data/auth_models.dart';
import 'package:saas_app/features/auth/data/auth_repository.dart';
import 'package:saas_app/features/auth/data/google_sign_in_service.dart';
import 'package:saas_app/features/security/data/passkey_service.dart';

import '../../helpers/fake_adapter.dart';
import '../../helpers/pump_app.dart';

const _enabled = ProvidersInfo(
  googleEnabled: true,
  googleWebClientId: 'web-client.apps.googleusercontent.com',
);

AuthSuccess _success() => AuthSuccess(
  tokens: Tokens.fromJson(tokensJson()),
  user: User.fromJson(userJson()),
);

final _button = find.byKey(const Key('google_signin'));

void main() {
  setUpAll(() => registerFallbackValue(''));

  group('button visibility', () {
    testWidgets('hidden when the server has Google sign-in disabled', (
      tester,
    ) async {
      await pumpApp(tester);
      expect(_button, findsNothing);
    });

    testWidgets('hidden on platforms without native support', (tester) async {
      await pumpApp(
        tester,
        providers: _enabled,
        google: FakeGoogleSignInService(supported: false),
      );
      expect(_button, findsNothing);
    });

    testWidgets('shown on login and register when enabled', (tester) async {
      await pumpApp(tester, providers: _enabled);
      expect(_button, findsOneWidget);
      expect(find.text('Continue with Google'), findsOneWidget);

      await tester.tap(find.text("Don't have an account? Create one"));
      await tester.pumpAndSettle();
      expect(_button, findsOneWidget);
    });
  });

  group('sign-in flow', () {
    testWidgets('exchanges the ID token and lands on home', (tester) async {
      final google = FakeGoogleSignInService(token: 'tok-123');
      final repo = await pumpApp(tester, providers: _enabled, google: google);
      when(() => repo.loginWithGoogle(any()))
          .thenAnswer((_) async => _success());

      await tester.tap(_button);
      await tester.pumpAndSettle();

      verify(() => repo.loginWithGoogle('tok-123')).called(1);
      expect(google.serverClientIds, ['web-client.apps.googleusercontent.com']);
      expect(find.textContaining('Hello, Ada'), findsOneWidget);
    });

    testWidgets('does nothing when the user cancels', (tester) async {
      final repo = await pumpApp(
        tester,
        providers: _enabled,
        google: FakeGoogleSignInService(token: null),
      );
      await tester.tap(_button);
      await tester.pumpAndSettle();
      verifyNever(() => repo.loginWithGoogle(any()));
      expect(find.text('Sign in'), findsWidgets);
    });

    testWidgets('reuses the 2FA screen for accounts with TOTP', (tester) async {
      final repo = await pumpApp(tester, providers: _enabled);
      when(() => repo.loginWithGoogle(any())).thenAnswer(
        (_) async => const MfaChallenge(mfaToken: 'mfa', methods: ['totp']),
      );
      when(() => repo.loginWithMfa('mfa', '123456'))
          .thenAnswer((_) async => _success());

      await tester.tap(_button);
      await tester.pumpAndSettle();
      expect(find.text('Two-factor authentication'), findsOneWidget);

      await tester.enterText(find.byType(TextFormField), '123456');
      await tester.tap(find.widgetWithText(FilledButton, 'Verify'));
      await tester.pumpAndSettle();
      expect(find.textContaining('Hello, Ada'), findsOneWidget);
    });

    testWidgets('shows a message when Google sign-in fails', (tester) async {
      final repo = await pumpApp(
        tester,
        providers: _enabled,
        google: FakeGoogleSignInService(
          failure: const GoogleSignInFailure('Google sign-in failed.'),
        ),
      );
      await tester.tap(_button);
      await tester.pumpAndSettle();
      expect(find.text('Google sign-in failed.'), findsOneWidget);
      verifyNever(() => repo.loginWithGoogle(any()));
    });
  });

  group('AuthRepository', () {
    late InMemoryTokenStorage storage;
    late FakeAdapter adapter;
    late AuthRepository repo;

    void setup(FakeHandler handler) {
      storage = InMemoryTokenStorage();
      adapter = FakeAdapter(handler);
      final dio = Dio(BaseOptions(baseUrl: 'http://x/api/v1'))
        ..httpClientAdapter = adapter;
      repo = AuthRepository(
        api: ApiClient(dio),
        storage: storage,
        passkeys: _NoPasskeys(),
      );
    }

    test('providers parses the public configuration', () async {
      setup(
        (o) => FakeResponse(200, {
          'google': {'enabled': true, 'web_client_id': 'web-id'},
        }),
      );
      final p = await repo.providers();
      expect(p.googleEnabled, isTrue);
      expect(p.googleWebClientId, 'web-id');
      expect(adapter.requests.single.path, '/auth/providers');
    });

    test('providers falls back to disabled on errors', () async {
      setup(
        (o) => FakeResponse(500, {
          'error': {'code': 'internal_error', 'message': 'x'},
        }),
      );
      expect((await repo.providers()).googleEnabled, isFalse);
    });

    test('loginWithGoogle posts the token and persists the session', () async {
      setup((o) {
        expect(o.path, '/auth/google');
        expect(o.data, {'id_token': 'tok'});
        return FakeResponse(200, {
          'mfa_required': false,
          'tokens': tokensJson(),
          'user': {
            ...userJson(),
            'has_password': false,
            'identities': ['google'],
          },
        });
      });
      final r = await repo.loginWithGoogle('tok') as AuthSuccess;
      expect(r.user.hasPassword, isFalse);
      expect(r.user.identities, ['google']);
      expect(await storage.readRefreshToken(), 'refresh1');
    });

    test('loginWithGoogle returns the MFA challenge without tokens', () async {
      setup(
        (o) => FakeResponse(200, {
          'mfa_required': true,
          'mfa_token': 'm',
          'methods': ['totp', 'recovery_code'],
        }),
      );
      expect(await repo.loginWithGoogle('tok'), isA<MfaChallenge>());
      expect(await storage.readRefreshToken(), isNull);
    });

    test('users default to having a password when the field is absent', () {
      final u = User.fromJson(userJson());
      expect(u.hasPassword, isTrue);
      expect(u.identities, isEmpty);
    });
  });
}

class _NoPasskeys implements PasskeyService {
  @override
  Future<Map<String, dynamic>> createCredential(Map<String, dynamic> o) async =>
      {};
  @override
  Future<Map<String, dynamic>> getCredential(Map<String, dynamic> o) async =>
      {};
}
