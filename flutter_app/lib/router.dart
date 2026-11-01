import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'features/auth/application/auth_controller.dart';
import 'features/auth/data/auth_models.dart';
import 'features/auth/presentation/forgot_password_screen.dart';
import 'features/auth/presentation/lock_screen.dart';
import 'features/auth/presentation/login_screen.dart';
import 'features/auth/presentation/mfa_screen.dart';
import 'features/auth/presentation/register_screen.dart';
import 'features/auth/presentation/reset_password_screen.dart';
import 'features/auth/presentation/splash_screen.dart';
import 'features/auth/presentation/verify_email_screen.dart';
import 'features/home/presentation/home_screen.dart';
import 'features/profile/presentation/profile_screen.dart';
import 'features/security/presentation/security_screen.dart';

/// Routes only reachable while signed out (signed-in users are bounced home).
const _guestOnly = {'/login', '/register', '/forgot-password', '/2fa'};

/// Reachable regardless of auth state (email links).
const _always = {'/verify-email', '/reset-password'};

/// Pure redirect logic, extracted for testing.
String? authRedirect(AuthState auth, String path) {
  if (auth is AuthLoading) return path == '/splash' ? null : '/splash';
  if (_always.contains(path)) return null;
  if (auth is AuthLocked) return path == '/lock' ? null : '/lock';
  if (auth is Authenticated) {
    return (path == '/splash' || path == '/lock' || _guestOnly.contains(path))
        ? '/home'
        : null;
  }
  // Unauthenticated
  return _guestOnly.contains(path) ? null : '/login';
}

final routerProvider = Provider<GoRouter>((ref) {
  final refresh = ValueNotifier<int>(0);
  ref.listen<AuthState>(authControllerProvider, (_, _) => refresh.value++);
  ref.onDispose(refresh.dispose);

  return GoRouter(
    initialLocation: '/splash',
    refreshListenable: refresh,
    redirect: (context, state) =>
        authRedirect(ref.read(authControllerProvider), state.uri.path),
    routes: [
      GoRoute(path: '/splash', builder: (_, _) => const SplashScreen()),
      GoRoute(path: '/lock', builder: (_, _) => const LockScreen()),
      GoRoute(path: '/login', builder: (_, _) => const LoginScreen()),
      GoRoute(path: '/register', builder: (_, _) => const RegisterScreen()),
      GoRoute(
        path: '/forgot-password',
        builder: (_, _) => const ForgotPasswordScreen(),
      ),
      GoRoute(
        path: '/reset-password',
        builder: (_, state) =>
            ResetPasswordScreen(token: state.uri.queryParameters['token']),
      ),
      GoRoute(
        path: '/verify-email',
        builder: (_, state) =>
            VerifyEmailScreen(token: state.uri.queryParameters['token']),
      ),
      GoRoute(
        path: '/2fa',
        // The challenge only lives in memory; without it start over.
        redirect: (_, state) => state.extra is MfaChallenge ? null : '/login',
        builder: (_, state) =>
            MfaScreen(challenge: state.extra! as MfaChallenge),
      ),
      GoRoute(path: '/home', builder: (_, _) => const HomeScreen()),
      GoRoute(path: '/profile', builder: (_, _) => const ProfileScreen()),
      GoRoute(path: '/security', builder: (_, _) => const SecurityScreen()),
    ],
  );
});
