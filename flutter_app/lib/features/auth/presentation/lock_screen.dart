import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../shared/widgets/form_page.dart';
import '../../../shared/widgets/primary_button.dart';
import '../application/auth_controller.dart';

/// Shown when biometric unlock was cancelled or failed at app start.
class LockScreen extends ConsumerWidget {
  const LockScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final auth = ref.read(authControllerProvider.notifier);
    return FormPage(
      title: 'Locked',
      subtitle: 'Unlock with biometrics to continue.',
      children: [
        const Icon(Icons.lock_outline, size: 48),
        const SizedBox(height: 24),
        PrimaryButton(label: 'Try again', onPressed: auth.unlock),
        const SizedBox(height: 8),
        TextButton(
          key: const Key('use_password'),
          onPressed: auth.usePasswordInstead,
          child: const Text('Use password instead'),
        ),
      ],
    );
  }
}
