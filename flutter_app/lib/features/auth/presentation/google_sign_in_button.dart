import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../core/providers.dart';
import '../../../shared/async_action_mixin.dart';
import '../../../shared/widgets/banners.dart';
import '../application/auth_controller.dart';
import '../data/auth_models.dart';

/// "Continue with Google". Renders nothing unless the server reports Google
/// sign-in as enabled and the platform supports the native flow.
class GoogleSignInButton extends ConsumerStatefulWidget {
  const GoogleSignInButton({super.key});

  @override
  ConsumerState<GoogleSignInButton> createState() => _GoogleSignInButtonState();
}

class _GoogleSignInButtonState extends ConsumerState<GoogleSignInButton>
    with AsyncActionMixin<GoogleSignInButton> {
  Future<void> _signIn(ProvidersInfo providers) async {
    AuthResult? result;
    await runAction(() async {
      final idToken = await ref
          .read(googleSignInServiceProvider)
          .requestIdToken(serverClientId: providers.googleWebClientId);
      if (idToken == null) return; // cancelled
      result = await ref
          .read(authControllerProvider.notifier)
          .loginWithGoogle(idToken);
    });
    final r = result;
    if (r is MfaChallenge && mounted) {
      // Same second step as password login.
      context.go('/2fa', extra: r);
    }
    // On success the router redirect sends the user to /home.
  }

  @override
  Widget build(BuildContext context) {
    final service = ref.watch(googleSignInServiceProvider);
    final providers = ref.watch(signInProvidersProvider).asData?.value;
    if (providers == null || !providers.googleEnabled || !service.isSupported) {
      return const SizedBox.shrink();
    }
    return Padding(
      padding: const EdgeInsets.only(top: 12),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          if (errorText != null) ...[
            ErrorBanner(message: errorText!),
            const SizedBox(height: 12),
          ],
          OutlinedButton.icon(
            key: const Key('google_signin'),
            icon: const Icon(Icons.g_mobiledata, size: 28),
            label: const Text('Continue with Google'),
            onPressed: busy ? null : () => _signIn(providers),
          ),
        ],
      ),
    );
  }
}
