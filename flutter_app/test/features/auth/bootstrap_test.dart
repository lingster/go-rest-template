import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';
import 'package:saas_app/core/providers.dart';
import 'package:saas_app/core/storage/biometric_prefs.dart';
import 'package:saas_app/features/auth/application/auth_controller.dart';
import 'package:saas_app/features/auth/data/auth_models.dart';
import 'package:saas_app/features/security/application/biometric_controller.dart';

import '../../helpers/fake_adapter.dart';
import '../../helpers/pump_app.dart';

void main() {
  late MockAuthRepository repo;
  late MockBiometricService bio;
  late InMemoryBiometricPrefs prefs;

  ProviderContainer make({
    bool hasSession = true,
    bool supported = true,
    BiometricChoice choice = BiometricChoice.enabled,
    bool authOk = true,
  }) {
    repo = MockAuthRepository();
    bio = MockBiometricService();
    prefs = InMemoryBiometricPrefs(choice);
    when(repo.hasStoredSession).thenAnswer((_) async => hasSession);
    when(repo.restoreSession)
        .thenAnswer((_) async => User.fromJson(userJson()));
    when(repo.logout).thenAnswer((_) async {});
    when(bio.isSupported).thenAnswer((_) async => supported);
    when(() => bio.authenticate(any())).thenAnswer((_) async => authOk);
    final container = ProviderContainer(
      overrides: [
        authRepositoryProvider.overrideWithValue(repo),
        biometricServiceProvider.overrideWithValue(bio),
        biometricPrefsProvider.overrideWithValue(prefs),
      ],
    );
    addTearDown(container.dispose);
    return container;
  }

  /// Reads the controller, then waits for bootstrap to leave AuthLoading.
  Future<AuthState> boot(ProviderContainer c) async {
    c.listen(authControllerProvider, (_, _) {});
    await Future<void>.delayed(Duration.zero);
    await Future<void>.delayed(Duration.zero);
    return c.read(authControllerProvider);
  }

  setUpAll(() => registerFallbackValue(''));

  test('no stored session -> unauthenticated, no prompt', () async {
    final c = make(hasSession: false);
    expect(await boot(c), isA<Unauthenticated>());
    verifyNever(() => bio.authenticate(any()));
    verifyNever(repo.restoreSession);
  });

  test('biometrics disabled -> restores session without prompting', () async {
    final c = make(choice: BiometricChoice.disabled);
    expect(await boot(c), isA<Authenticated>());
    verifyNever(() => bio.authenticate(any()));
  });

  test('unset preference is treated as disabled at bootstrap', () async {
    final c = make(choice: BiometricChoice.unset);
    expect(await boot(c), isA<Authenticated>());
    verifyNever(() => bio.authenticate(any()));
  });

  test(
    'enabled but device unsupported -> session cleared, password required',
    () async {
      final c = make(supported: false);
      expect(await boot(c), isA<Unauthenticated>());
      verifyNever(() => bio.authenticate(any()));
      verifyNever(repo.restoreSession);
      verify(repo.logout).called(1);
    },
  );

  test(
    'enabled + success: prompt happens BEFORE the session is restored',
    () async {
      final c = make();
      expect(await boot(c), isA<Authenticated>());
      verifyInOrder([() => bio.authenticate(any()), repo.restoreSession]);
    },
  );

  test('enabled + cancel -> locked, session untouched', () async {
    final c = make(authOk: false);
    expect(await boot(c), isA<AuthLocked>());
    verifyNever(repo.restoreSession);
  });

  test('locked: try again succeeds', () async {
    final c = make(authOk: false);
    await boot(c);
    when(() => bio.authenticate(any())).thenAnswer((_) async => true);
    await c.read(authControllerProvider.notifier).unlock();
    expect(c.read(authControllerProvider), isA<Authenticated>());
  });

  test('locked: use password instead clears the session', () async {
    final c = make(authOk: false);
    await boot(c);
    await c.read(authControllerProvider.notifier).usePasswordInstead();
    verify(repo.logout).called(1);
    expect(c.read(authControllerProvider), isA<Unauthenticated>());
  });

  test(
    'canUnlockWithBiometrics requires session + enabled + support',
    () async {
      var c = make(hasSession: true);
      expect(
        await c.read(authControllerProvider.notifier).canUnlockWithBiometrics(),
        isTrue,
      );
      c = make(choice: BiometricChoice.disabled);
      expect(
        await c.read(authControllerProvider.notifier).canUnlockWithBiometrics(),
        isFalse,
      );
    },
  );

  group('BiometricController', () {
    test('offer is shown only when supported and undecided', () async {
      var c = make(choice: BiometricChoice.unset);
      var s = await c.read(biometricControllerProvider.future);
      expect(s.shouldOffer, isTrue);
      c = make(choice: BiometricChoice.unset, supported: false);
      s = await c.read(biometricControllerProvider.future);
      expect(s.shouldOffer, isFalse);
    });

    test('enable requires confirmation and persists', () async {
      final c = make(choice: BiometricChoice.unset, authOk: false);
      await c.read(biometricControllerProvider.future);
      final n = c.read(biometricControllerProvider.notifier);
      expect(await n.enable(), isFalse);
      expect(prefs.value, BiometricChoice.unset);

      when(() => bio.authenticate(any())).thenAnswer((_) async => true);
      expect(await n.enable(), isTrue);
      expect(prefs.value, BiometricChoice.enabled);
      expect(c.read(biometricControllerProvider).value!.enabled, isTrue);
    });

    test('disable requires biometric confirmation', () async {
      final c = make(authOk: false);
      await c.read(biometricControllerProvider.future);
      final n = c.read(biometricControllerProvider.notifier);
      expect(await n.disable(), isFalse);
      expect(prefs.value, BiometricChoice.enabled);

      when(() => bio.authenticate(any())).thenAnswer((_) async => true);
      expect(await n.disable(), isTrue);
      expect(prefs.value, BiometricChoice.disabled);
    });

    test('decline records the decision without prompting', () async {
      final c = make(choice: BiometricChoice.unset);
      await c.read(biometricControllerProvider.future);
      await c.read(biometricControllerProvider.notifier).decline();
      expect(prefs.value, BiometricChoice.disabled);
      verifyNever(() => bio.authenticate(any()));
    });
  });
}
