import 'package:flutter_test/flutter_test.dart';
import 'package:passkeys/types.dart';
import 'package:saas_app/features/auth/application/auth_controller.dart';
import 'package:saas_app/features/auth/data/auth_models.dart';
import 'package:saas_app/features/security/data/passkey_service.dart';
import 'package:saas_app/router.dart';

import '../helpers/fake_adapter.dart';

void main() {
  group('authRedirect', () {
    final authed = Authenticated(User.fromJson(userJson()));

    test('loading always goes to splash', () {
      expect(authRedirect(const AuthLoading(), '/home'), '/splash');
      expect(authRedirect(const AuthLoading(), '/splash'), isNull);
    });

    test('signed out is sent to login except public routes', () {
      expect(authRedirect(const Unauthenticated(), '/home'), '/login');
      expect(authRedirect(const Unauthenticated(), '/security'), '/login');
      expect(authRedirect(const Unauthenticated(), '/register'), isNull);
      expect(authRedirect(const Unauthenticated(), '/reset-password'), isNull);
      expect(authRedirect(const Unauthenticated(), '/verify-email'), isNull);
    });

    test('locked is confined to the lock screen', () {
      expect(authRedirect(const AuthLocked(), '/home'), '/lock');
      expect(authRedirect(const AuthLocked(), '/lock'), isNull);
      expect(authRedirect(const Unauthenticated(), '/lock'), '/login');
      expect(authRedirect(authed, '/lock'), '/home');
    });

    test('signed in is bounced away from guest routes', () {
      expect(authRedirect(authed, '/login'), '/home');
      expect(authRedirect(authed, '/splash'), '/home');
      expect(authRedirect(authed, '/profile'), isNull);
      expect(authRedirect(authed, '/verify-email'), isNull);
    });
  });

  group('passkey JSON conversion', () {
    test('publicKeyOf unwraps the server options', () {
      expect(
        publicKeyOf({
          'publicKey': {'challenge': 'c'},
        }),
        {'challenge': 'c'},
      );
      expect(publicKeyOf({'challenge': 'c'}), {'challenge': 'c'});
    });

    test('server creation options parse into the plugin request', () {
      final req = RegisterRequestType.fromJson(
        publicKeyOf({
          'publicKey': {
            'challenge': 'Y2hhbGxlbmdl',
            'rp': {'id': 'localhost', 'name': 'App'},
            'user': {'id': 'dXNlcg', 'name': 'a@b.com', 'displayName': 'Ada'},
            'pubKeyCredParams': [
              {'type': 'public-key', 'alg': -7},
            ],
            'authenticatorSelection': {
              'residentKey': 'preferred',
              'userVerification': 'preferred',
            },
            'excludeCredentials': [
              {'type': 'public-key', 'id': 'Y3JlZA'},
            ],
            'timeout': 60000,
          },
        }),
      );
      expect(req.relyingParty.id, 'localhost');
      expect(req.challenge, 'Y2hhbGxlbmdl');
      expect(req.excludeCredentials.single.id, 'Y3JlZA');
    });

    test('registration result becomes standard WebAuthn JSON', () {
      final json = registrationToJson(
        const RegisterResponseType(
          id: 'id1',
          rawId: 'raw1==',
          clientDataJSON: 'cd',
          attestationObject: 'att',
          transports: ['internal'],
        ),
      );
      expect(json['type'], 'public-key');
      expect(json['rawId'], 'raw1');
      final resp = json['response'] as Map;
      expect(resp['clientDataJSON'], 'cd');
      expect(resp['attestationObject'], 'att');
      expect(resp['transports'], ['internal']);
    });

    test('authentication result becomes standard WebAuthn JSON', () {
      final json = authenticationToJson(
        const AuthenticateResponseType(
          id: 'id1',
          rawId: 'raw1',
          clientDataJSON: 'cd',
          authenticatorData: 'ad',
          signature: 'sig',
          userHandle: '',
        ),
      );
      final resp = json['response'] as Map;
      expect(resp['authenticatorData'], 'ad');
      expect(resp['signature'], 'sig');
      expect(resp.containsKey('userHandle'), isFalse);
    });
  });
}
