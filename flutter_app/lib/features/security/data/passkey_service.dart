import 'package:passkeys/authenticator.dart';
import 'package:passkeys/types.dart';

import '../../../core/network/api_exception.dart';

/// Platform passkey operations. Inputs are the server's `options` object
/// (`{"publicKey": {...}}` from go-webauthn); outputs are standard WebAuthn
/// JSON (`PublicKeyCredential.toJSON()` shape, base64url fields).
abstract interface class PasskeyService {
  Future<Map<String, dynamic>> createCredential(Map<String, dynamic> options);
  Future<Map<String, dynamic>> getCredential(Map<String, dynamic> options);
}

/// Any passkey failure (cancelled, unsupported, domain not associated ...).
class PasskeyException implements UserMessageException {
  const PasskeyException(this.userMessage);

  factory PasskeyException.from(AuthenticatorException e) => PasskeyException(
    e is PasskeyAuthCancelledException
        ? 'Passkey request was cancelled.'
        : 'Passkey failed: ${e.runtimeType}. Check device and domain setup.',
  );

  @override
  final String userMessage;
}

/// Extracts `publicKey` from the server options (tolerates an unwrapped map)
/// and normalises it for the plugin: go-webauthn omits empty `transports`
/// on credential descriptors, which the plugin's parser requires.
Map<String, dynamic> publicKeyOf(Map<String, dynamic> options) {
  final pk = options['publicKey'];
  final result = Map<String, dynamic>.from(pk is Map ? pk : options);
  for (final key in const ['allowCredentials', 'excludeCredentials']) {
    final list = result[key];
    if (list is List) {
      result[key] = [
        for (final c in list)
          if (c is Map)
            <String, dynamic>{
              for (final e in c.entries) '${e.key}': e.value,
              'transports': c['transports'] ?? <String>[],
            },
      ];
    }
  }
  return result;
}

String _b64urlNoPad(String s) => s.replaceAll('=', '');

/// Plugin registration response -> WebAuthn JSON.
Map<String, dynamic> registrationToJson(RegisterResponseType r) => {
  'id': r.id,
  'rawId': _b64urlNoPad(r.rawId),
  'type': 'public-key',
  'response': {
    'clientDataJSON': _b64urlNoPad(r.clientDataJSON),
    'attestationObject': _b64urlNoPad(r.attestationObject),
    if (r.transports.isNotEmpty)
      'transports': r.transports.whereType<String>().toList(),
  },
  'clientExtensionResults': <String, dynamic>{},
};

/// Plugin authentication response -> WebAuthn JSON.
Map<String, dynamic> authenticationToJson(AuthenticateResponseType r) => {
  'id': r.id,
  'rawId': _b64urlNoPad(r.rawId),
  'type': 'public-key',
  'response': {
    'clientDataJSON': _b64urlNoPad(r.clientDataJSON),
    'authenticatorData': _b64urlNoPad(r.authenticatorData),
    'signature': _b64urlNoPad(r.signature),
    if (r.userHandle.isNotEmpty) 'userHandle': _b64urlNoPad(r.userHandle),
  },
  'clientExtensionResults': <String, dynamic>{},
};

/// Production implementation backed by the `passkeys` plugin
/// (iOS/macOS, Android Credential Manager, web WebAuthn).
class PluginPasskeyService implements PasskeyService {
  PluginPasskeyService([PasskeyAuthenticator? authenticator])
    : _authenticator = authenticator ?? PasskeyAuthenticator();

  final PasskeyAuthenticator _authenticator;

  @override
  Future<Map<String, dynamic>> createCredential(
    Map<String, dynamic> options,
  ) async {
    try {
      final request = RegisterRequestType.fromJson(publicKeyOf(options));
      return registrationToJson(await _authenticator.register(request));
    } on AuthenticatorException catch (e) {
      throw PasskeyException.from(e);
    }
  }

  @override
  Future<Map<String, dynamic>> getCredential(
    Map<String, dynamic> options,
  ) async {
    try {
      final request = AuthenticateRequestType.fromJson(
        publicKeyOf(options),
        mediation: MediationType.Optional,
        preferImmediatelyAvailableCredentials: false,
      );
      return authenticationToJson(await _authenticator.authenticate(request));
    } on AuthenticatorException catch (e) {
      throw PasskeyException.from(e);
    }
  }
}
