import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/providers.dart';
import '../../../core/storage/biometric_prefs.dart';

class BiometricStatus {
  const BiometricStatus({required this.supported, required this.choice});
  final bool supported;
  final BiometricChoice choice;

  bool get enabled => supported && choice == BiometricChoice.enabled;

  /// Show the post-login "Enable biometric unlock?" offer.
  bool get shouldOffer => supported && choice == BiometricChoice.unset;
}

final biometricControllerProvider =
    AsyncNotifierProvider<BiometricController, BiometricStatus>(
      BiometricController.new,
    );

/// Biometric unlock preference. Changing it requires a biometric confirmation.
class BiometricController extends AsyncNotifier<BiometricStatus> {
  @override
  Future<BiometricStatus> build() async => BiometricStatus(
    supported: await ref.read(biometricServiceProvider).isSupported(),
    choice: await ref.read(biometricPrefsProvider).read(),
  );

  /// Returns false if the user cancelled the biometric confirmation.
  Future<bool> enable() => _change(BiometricChoice.enabled);

  Future<bool> disable() => _change(BiometricChoice.disabled);

  /// "Not now" on the offer dialog: remembers the decision, no prompt.
  Future<void> decline() async {
    await ref.read(biometricPrefsProvider).write(BiometricChoice.disabled);
    ref.invalidateSelf();
  }

  Future<bool> _change(BiometricChoice choice) async {
    final ok = await ref
        .read(biometricServiceProvider)
        .authenticate(
          choice == BiometricChoice.enabled
              ? 'Confirm to enable biometric unlock'
              : 'Confirm to disable biometric unlock',
        );
    if (!ok) return false;
    await ref.read(biometricPrefsProvider).write(choice);
    ref.invalidateSelf();
    await future;
    return true;
  }
}
