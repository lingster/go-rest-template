import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/network/api_exception.dart';
import '../../../shared/widgets/banners.dart';
import '../../../shared/widgets/form_page.dart';
import '../../auth/application/auth_controller.dart';
import '../application/biometric_controller.dart';
import '../application/security_providers.dart';
import '../data/security_repository.dart';
import 'totp_dialogs.dart';

class SecurityScreen extends ConsumerWidget {
  const SecurityScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return FormPage(
      title: 'Security',
      appBar: AppBar(title: const Text('Security')),
      children: const [
        _TotpSection(),
        SizedBox(height: 32),
        _PasskeySection(),
        _BiometricSection(),
      ],
    );
  }
}

class _TotpSection extends ConsumerWidget {
  const _TotpSection();

  Future<void> _refresh(WidgetRef ref) =>
      ref.read(authControllerProvider.notifier).refreshUser();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final state = ref.watch(authControllerProvider);
    final enabled = state is Authenticated && state.user.totpEnabled;
    final theme = Theme.of(context);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Text('Authenticator app (2FA)', style: theme.textTheme.titleLarge),
        const SizedBox(height: 8),
        Text(
          enabled
              ? 'Two-factor authentication is on.'
              : 'Add a second step to sign-in using a TOTP authenticator app.',
        ),
        const SizedBox(height: 12),
        if (!enabled)
          FilledButton(
            key: const Key('enable_totp'),
            onPressed: () async {
              await showDialog<bool>(
                context: context,
                barrierDismissible: false,
                builder: (_) => const TotpSetupDialog(),
              );
              await _refresh(ref);
            },
            child: const Text('Enable two-factor'),
          )
        else ...[
          OutlinedButton(
            onPressed: () => showDialog<void>(
              context: context,
              barrierDismissible: false,
              builder: (_) => const RegenerateCodesDialog(),
            ),
            child: const Text('Regenerate recovery codes'),
          ),
          const SizedBox(height: 8),
          OutlinedButton(
            key: const Key('disable_totp'),
            onPressed: () async {
              await showDialog<bool>(
                context: context,
                builder: (_) => const DisableTotpDialog(),
              );
              await _refresh(ref);
            },
            child: const Text('Disable two-factor'),
          ),
        ],
      ],
    );
  }
}

class _PasskeySection extends ConsumerStatefulWidget {
  const _PasskeySection();

  @override
  ConsumerState<_PasskeySection> createState() => _PasskeySectionState();
}

class _PasskeySectionState extends ConsumerState<_PasskeySection> {
  String? _error;
  bool _busy = false;

  Future<void> _add() async {
    final name = await showDialog<String>(
      context: context,
      builder: (_) => const _NamePasskeyDialog(),
    );
    if (name == null) return;
    await _guard(() => ref.read(securityRepositoryProvider).addPasskey(name));
  }

  Future<void> _delete(Passkey p) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Delete passkey?'),
        content: Text('"${p.name}" will no longer work for sign-in.'),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(false),
            child: const Text('Cancel'),
          ),
          FilledButton(
            onPressed: () => Navigator.of(ctx).pop(true),
            child: const Text('Delete'),
          ),
        ],
      ),
    );
    if (confirmed != true) return;
    await _guard(
      () => ref.read(securityRepositoryProvider).deletePasskey(p.id),
    );
  }

  Future<void> _guard(Future<void> Function() action) async {
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      await action();
      ref.invalidate(passkeysProvider);
      await ref.read(authControllerProvider.notifier).refreshUser();
    } catch (e) {
      if (mounted) setState(() => _error = errorMessage(e));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final passkeys = ref.watch(passkeysProvider);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Text('Passkeys', style: theme.textTheme.titleLarge),
        const SizedBox(height: 8),
        const Text('Sign in with your fingerprint, face or device PIN.'),
        const SizedBox(height: 12),
        if (_error != null) ...[
          ErrorBanner(message: _error!),
          const SizedBox(height: 12),
        ],
        passkeys.when(
          loading: () => const Center(child: CircularProgressIndicator()),
          error: (e, _) => ErrorBanner(message: errorMessage(e)),
          data: (items) => Column(
            children: [
              if (items.isEmpty)
                const Padding(
                  padding: EdgeInsets.symmetric(vertical: 8),
                  child: Text('No passkeys yet.'),
                ),
              for (final p in items)
                ListTile(
                  contentPadding: EdgeInsets.zero,
                  leading: const Icon(Icons.key),
                  title: Text(p.name),
                  subtitle: Text(_subtitle(p)),
                  trailing: IconButton(
                    tooltip: 'Delete passkey',
                    icon: const Icon(Icons.delete_outline),
                    onPressed: _busy ? null : () => _delete(p),
                  ),
                ),
            ],
          ),
        ),
        const SizedBox(height: 8),
        OutlinedButton.icon(
          key: const Key('add_passkey'),
          icon: const Icon(Icons.add),
          label: const Text('Add a passkey'),
          onPressed: _busy ? null : _add,
        ),
      ],
    );
  }

  static String _subtitle(Passkey p) {
    String fmt(DateTime d) => d.toLocal().toString().split(' ').first;
    final created = p.createdAt == null ? '' : 'Added ${fmt(p.createdAt!)}';
    final used = p.lastUsedAt == null
        ? 'never used'
        : 'last used ${fmt(p.lastUsedAt!)}';
    return created.isEmpty ? used : '$created, $used';
  }
}

class _NamePasskeyDialog extends StatefulWidget {
  const _NamePasskeyDialog();

  @override
  State<_NamePasskeyDialog> createState() => _NamePasskeyDialogState();
}

class _NamePasskeyDialogState extends State<_NamePasskeyDialog> {
  final _name = TextEditingController(text: 'My device');

  @override
  void dispose() {
    _name.dispose();
    super.dispose();
  }

  void _submit() {
    final v = _name.text.trim();
    if (v.isNotEmpty) Navigator.of(context).pop(v);
  }

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: const Text('Name this passkey'),
      content: TextField(
        controller: _name,
        autofocus: true,
        decoration: const InputDecoration(labelText: 'Name'),
        onSubmitted: (_) => _submit(),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(),
          child: const Text('Cancel'),
        ),
        FilledButton(onPressed: _submit, child: const Text('Continue')),
      ],
    );
  }
}

/// Biometric unlock toggle. Hidden when the device (or web) lacks support.
class _BiometricSection extends ConsumerWidget {
  const _BiometricSection();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final status = ref.watch(biometricControllerProvider).asData?.value;
    if (status == null || !status.supported) return const SizedBox.shrink();
    return Padding(
      padding: const EdgeInsets.only(top: 32),
      child: SwitchListTile(
        key: const Key('biometric_toggle'),
        contentPadding: EdgeInsets.zero,
        title: const Text('Biometric unlock'),
        subtitle: const Text('Require Face ID / fingerprint to open the app.'),
        value: status.enabled,
        onChanged: (on) async {
          final c = ref.read(biometricControllerProvider.notifier);
          final ok = on ? await c.enable() : await c.disable();
          if (!ok && context.mounted) {
            ScaffoldMessenger.of(context).showSnackBar(
              const SnackBar(content: Text('Biometric confirmation failed')),
            );
          }
        },
      ),
    );
  }
}
