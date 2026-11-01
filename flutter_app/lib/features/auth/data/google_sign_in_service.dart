import 'package:flutter/foundation.dart';
import 'package:google_sign_in/google_sign_in.dart';

import '../../../core/config/app_config.dart';
import '../../../core/network/api_exception.dart';

/// Obtains a Google ID token from the platform. The token is sent to the
/// server (`POST /auth/google`), which verifies it; nothing here is trusted.
abstract interface class GoogleSignInService {
  /// True on platforms where the native Google sign-in flow is available
  /// (iOS and Android). The button is hidden elsewhere.
  bool get isSupported;

  /// Shows the Google account picker and returns the ID token, or `null` when
  /// the user cancelled. [serverClientId] is the *web* OAuth client ID (the ID
  /// token audience); it overrides the `GOOGLE_SERVER_CLIENT_ID` define.
  Future<String?> requestIdToken({String? serverClientId});
}

/// A failed (not cancelled) Google sign-in, e.g. missing configuration.
class GoogleSignInFailure implements UserMessageException {
  const GoogleSignInFailure(this.userMessage);

  @override
  final String userMessage;
}

/// Production implementation on `google_sign_in` v7 (singleton, one-time
/// `initialize`, `authenticate()` returning the account).
class PluginGoogleSignInService implements GoogleSignInService {
  PluginGoogleSignInService([GoogleSignIn? plugin])
    : _plugin = plugin ?? GoogleSignIn.instance;

  final GoogleSignIn _plugin;
  Future<void>? _initialized;

  @override
  bool get isSupported =>
      !kIsWeb &&
      (defaultTargetPlatform == TargetPlatform.iOS ||
          defaultTargetPlatform == TargetPlatform.android);

  Future<void> _initialize(String? serverClientId) {
    final server = AppConfig.googleServerClientId.isNotEmpty
        ? AppConfig.googleServerClientId
        : serverClientId;
    final isApple = defaultTargetPlatform == TargetPlatform.iOS;
    return _initialized ??= _plugin.initialize(
      // iOS client ID (Android resolves its client from the package + SHA-1).
      clientId: isApple && AppConfig.googleIosClientId.isNotEmpty
          ? AppConfig.googleIosClientId
          : null,
      // Without the web client ID the Android SDK returns no idToken.
      serverClientId: (server == null || server.isEmpty) ? null : server,
    );
  }

  @override
  Future<String?> requestIdToken({String? serverClientId}) async {
    try {
      await _initialize(serverClientId);
      final account = await _plugin.authenticate();
      final idToken = account.authentication.idToken;
      // Don't keep a Google session around: the app has its own.
      await _plugin.signOut().catchError((_) {});
      if (idToken == null || idToken.isEmpty) {
        throw const GoogleSignInFailure(
          'Google did not return an ID token. Check the server client ID '
          'configuration.',
        );
      }
      return idToken;
    } on GoogleSignInException catch (e) {
      if (e.code == GoogleSignInExceptionCode.canceled) return null;
      _initialized = null; // allow a retry after configuration errors
      throw GoogleSignInFailure(
        e.code == GoogleSignInExceptionCode.clientConfigurationError ||
                e.code == GoogleSignInExceptionCode.providerConfigurationError
            ? 'Google sign-in is not configured correctly for this app.'
            : 'Google sign-in failed. Please try again.',
      );
    }
  }
}
