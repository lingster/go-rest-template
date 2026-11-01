import 'package:flutter_secure_storage/flutter_secure_storage.dart';

/// The user's decision about biometric unlock.
enum BiometricChoice { unset, enabled, disabled }

abstract interface class BiometricPrefs {
  Future<BiometricChoice> read();
  Future<void> write(BiometricChoice choice);
}

class SecureBiometricPrefs implements BiometricPrefs {
  SecureBiometricPrefs([FlutterSecureStorage? storage])
    : _storage = storage ?? const FlutterSecureStorage();

  static const _key = 'biometric_unlock';
  final FlutterSecureStorage _storage;

  @override
  Future<BiometricChoice> read() async {
    final v = await _storage.read(key: _key);
    return BiometricChoice.values.firstWhere(
      (c) => c.name == v,
      orElse: () => BiometricChoice.unset,
    );
  }

  @override
  Future<void> write(BiometricChoice choice) =>
      _storage.write(key: _key, value: choice.name);
}

class InMemoryBiometricPrefs implements BiometricPrefs {
  InMemoryBiometricPrefs([this.value = BiometricChoice.unset]);
  BiometricChoice value;

  @override
  Future<BiometricChoice> read() async => value;

  @override
  Future<void> write(BiometricChoice choice) async => value = choice;
}
