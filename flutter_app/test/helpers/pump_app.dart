import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';
import 'package:saas_app/app.dart';
import 'package:saas_app/core/providers.dart';
import 'package:saas_app/core/storage/biometric_prefs.dart';
import 'package:saas_app/features/security/data/biometric_service.dart';
import 'package:saas_app/features/auth/application/auth_controller.dart';
import 'package:saas_app/features/auth/data/auth_repository.dart';

class MockAuthRepository extends Mock implements AuthRepository {}

class MockBiometricService extends Mock implements BiometricService {}

/// Boots the full app with a mocked [AuthRepository] and no stored session,
/// so it settles on the login screen.
Future<MockAuthRepository> pumpApp(WidgetTester tester) async {
  tester.view.physicalSize = const Size(900, 1600);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);

  final repo = MockAuthRepository();
  final bio = MockBiometricService();
  when(bio.isSupported).thenAnswer((_) async => false);
  when(repo.hasStoredSession).thenAnswer((_) async => false);
  when(repo.restoreSession).thenAnswer((_) async => null);
  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        authRepositoryProvider.overrideWithValue(repo),
        biometricServiceProvider.overrideWithValue(bio),
        biometricPrefsProvider.overrideWithValue(InMemoryBiometricPrefs()),
      ],
      child: const App(),
    ),
  );
  await tester.pumpAndSettle();
  return repo;
}
