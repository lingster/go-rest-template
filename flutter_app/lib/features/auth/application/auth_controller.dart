import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/providers.dart';
import '../../../core/storage/biometric_prefs.dart';
import '../data/auth_models.dart';
import '../data/auth_repository.dart';

final authRepositoryProvider = Provider<AuthRepository>(
  (ref) => AuthRepository(
    api: ref.watch(apiClientProvider),
    storage: ref.watch(tokenStorageProvider),
    passkeys: ref.watch(passkeyServiceProvider),
  ),
);

sealed class AuthState {
  const AuthState();
}

/// Session restore in progress (app start).
class AuthLoading extends AuthState {
  const AuthLoading();
}

class Unauthenticated extends AuthState {
  const Unauthenticated();
}

/// A stored session exists but biometric unlock has not succeeded.
class AuthLocked extends AuthState {
  const AuthLocked();
}

class Authenticated extends AuthState {
  const Authenticated(this.user);
  final User user;
}

final authControllerProvider = NotifierProvider<AuthController, AuthState>(
  AuthController.new,
);

/// Owns the session. Screens call its methods; the router listens to [state].
class AuthController extends Notifier<AuthState> {
  AuthRepository get _repo => ref.read(authRepositoryProvider);

  @override
  AuthState build() {
    Future.microtask(_bootstrap);
    return const AuthLoading();
  }

  /// Bootstrap is async; ignore results that arrive after disposal.
  void _set(AuthState next) {
    if (ref.mounted) state = next;
  }

  Future<void> _bootstrap() async {
    try {
      if (!await _repo.hasStoredSession()) {
        _set(const Unauthenticated());
        return;
      }
      final optedIn =
          await ref.read(biometricPrefsProvider).read() ==
          BiometricChoice.enabled;
      if (!optedIn) {
        await _restore();
      } else if (await ref.read(biometricServiceProvider).isSupported()) {
        await unlock();
      } else {
        // Opted in, but biometrics are no longer available (e.g. enrolment
        // removed): never restore silently, require the password instead.
        await usePasswordInstead();
      }
    } catch (_) {
      _set(const Unauthenticated());
    }
  }

  Future<void> _restore() async {
    try {
      final user = await _repo.restoreSession();
      _set(user == null ? const Unauthenticated() : Authenticated(user));
    } catch (_) {
      _set(const Unauthenticated());
    }
  }

  /// True when the user opted in and the device still supports biometrics.
  Future<bool> biometricUnlockEnabled() async =>
      await ref.read(biometricPrefsProvider).read() ==
          BiometricChoice.enabled &&
      await ref.read(biometricServiceProvider).isSupported();

  /// True if a stored session can be unlocked with biometrics.
  Future<bool> canUnlockWithBiometrics() async =>
      await _repo.hasStoredSession() && await biometricUnlockEnabled();

  /// Prompts for biometrics BEFORE the refresh token is used. On cancel or
  /// failure the app is [AuthLocked].
  Future<void> unlock() async {
    final ok = await ref
        .read(biometricServiceProvider)
        .authenticate('Unlock your account');
    if (!ok) {
      _set(const AuthLocked());
      return;
    }
    await _restore();
  }

  /// "Use password instead": drops the stored session.
  Future<void> usePasswordInstead() async {
    await _repo.logout();
    _set(const Unauthenticated());
  }

  /// Returns [MfaChallenge] when a second factor is required.
  Future<AuthResult> login(String email, String password) async {
    final result = await _repo.login(email, password);
    _apply(result);
    return result;
  }

  Future<void> verifyMfa(String mfaToken, String code) async {
    _apply(await _repo.loginWithMfa(mfaToken, code));
  }

  Future<void> loginWithPasskey({String? email}) async {
    _apply(await _repo.loginWithPasskey(email: email));
  }

  Future<void> logout() async {
    await _repo.logout();
    state = const Unauthenticated();
  }

  /// Called by the network layer when refreshing the session failed.
  void sessionExpired() => state = const Unauthenticated();

  void setUser(User user) => state = Authenticated(user);

  Future<void> refreshUser() async => setUser(await _repo.fetchCurrentUser());

  void _apply(AuthResult result) {
    if (result is AuthSuccess) state = Authenticated(result.user);
  }
}
