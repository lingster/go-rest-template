import 'package:flutter/foundation.dart';
import 'package:local_auth/local_auth.dart';

/// Device biometric prompt. Mockable; unsupported on web.
abstract interface class BiometricService {
  /// True if the device has enrolled biometrics (and we are not on web).
  Future<bool> isSupported();

  /// Shows the system prompt. Returns false on cancel/failure (never throws).
  Future<bool> authenticate(String reason);
}

class LocalAuthBiometricService implements BiometricService {
  LocalAuthBiometricService([LocalAuthentication? auth])
    : _auth = auth ?? LocalAuthentication();

  final LocalAuthentication _auth;

  @override
  Future<bool> isSupported() async {
    if (kIsWeb) return false;
    try {
      if (!await _auth.isDeviceSupported()) return false;
      return (await _auth.getAvailableBiometrics()).isNotEmpty;
    } catch (_) {
      return false;
    }
  }

  @override
  Future<bool> authenticate(String reason) async {
    if (kIsWeb) return false;
    try {
      return await _auth.authenticate(
        localizedReason: reason,
        biometricOnly: true,
        persistAcrossBackgrounding: true,
      );
    } catch (_) {
      return false;
    }
  }
}
