import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../auth/application/auth_controller.dart';
import '../../security/application/biometric_controller.dart';

/// Placeholder dashboard: replace with your product's main screen.
class HomeScreen extends ConsumerStatefulWidget {
  const HomeScreen({super.key});

  @override
  ConsumerState<HomeScreen> createState() => _HomeScreenState();
}

class _HomeScreenState extends ConsumerState<HomeScreen> {
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _offerBiometrics());
  }

  /// After login: offer biometric unlock once, if supported and undecided.
  Future<void> _offerBiometrics() async {
    final status = await ref.read(biometricControllerProvider.future);
    if (!mounted || !status.shouldOffer) return;
    final controller = ref.read(biometricControllerProvider.notifier);
    final accept = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Enable biometric unlock?'),
        content: const Text(
          'Use Face ID or your fingerprint to unlock the app next time.',
        ),
        actions: [
          TextButton(
            key: const Key('biometric_decline'),
            onPressed: () => Navigator.of(ctx).pop(false),
            child: const Text('Not now'),
          ),
          FilledButton(
            key: const Key('biometric_accept'),
            onPressed: () => Navigator.of(ctx).pop(true),
            child: const Text('Enable'),
          ),
        ],
      ),
    );
    if (accept == true) {
      // If the confirmation is cancelled the choice stays undecided.
      await controller.enable();
    } else if (accept == false) {
      await controller.decline();
    }
  }

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(authControllerProvider);
    final user = state is Authenticated ? state.user : null;
    final theme = Theme.of(context);
    return Scaffold(
      appBar: AppBar(
        title: const Text('Home'),
        actions: [
          IconButton(
            tooltip: 'Security',
            icon: const Icon(Icons.shield_outlined),
            onPressed: () => context.push('/security'),
          ),
          IconButton(
            tooltip: 'Profile',
            icon: const Icon(Icons.person_outline),
            onPressed: () => context.push('/profile'),
          ),
        ],
      ),
      body: Center(
        child: Padding(
          padding: const EdgeInsets.all(24),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Text(
                'Hello, ${user?.name.isNotEmpty == true ? user!.name : user?.email ?? ''}!',
                style: theme.textTheme.headlineMedium,
                textAlign: TextAlign.center,
              ),
              const SizedBox(height: 8),
              Text(
                'This is your dashboard placeholder.',
                style: theme.textTheme.bodyLarge,
              ),
            ],
          ),
        ),
      ),
    );
  }
}
