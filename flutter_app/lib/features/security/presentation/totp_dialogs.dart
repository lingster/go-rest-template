import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:qr_flutter/qr_flutter.dart';

import '../../../core/network/api_exception.dart';
import '../../../shared/async_action_mixin.dart';
import '../../../shared/validators.dart';
import '../../../shared/widgets/app_text_field.dart';
import '../../../shared/widgets/banners.dart';
import '../../../shared/widgets/password_field.dart';
import '../../auth/application/auth_controller.dart';
import '../application/security_providers.dart';
import '../data/security_repository.dart';
import 'recovery_codes_view.dart';

/// Wraps dialog content in a width-constrained scroll view.
class _DialogBody extends StatelessWidget {
  const _DialogBody({required this.children});
  final List<Widget> children;

  @override
  Widget build(BuildContext context) => SizedBox(
    width: 400,
    child: SingleChildScrollView(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: children,
      ),
    ),
  );
}

/// Setup -> QR + secret -> confirm code -> recovery codes.
class TotpSetupDialog extends ConsumerStatefulWidget {
  const TotpSetupDialog({super.key});

  @override
  ConsumerState<TotpSetupDialog> createState() => _TotpSetupDialogState();
}

class _TotpSetupDialogState extends ConsumerState<TotpSetupDialog>
    with AsyncActionMixin<TotpSetupDialog> {
  final _formKey = GlobalKey<FormState>();
  final _code = TextEditingController();
  TotpSetup? _setup;
  List<String>? _recovery;
  String? _loadError;

  @override
  void initState() {
    super.initState();
    _load();
  }

  @override
  void dispose() {
    _code.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    try {
      final setup = await ref.read(securityRepositoryProvider).setupTotp();
      if (mounted) setState(() => _setup = setup);
    } catch (e) {
      if (mounted) setState(() => _loadError = errorMessage(e));
    }
  }

  Future<void> _enable() async {
    if (!_formKey.currentState!.validate()) return;
    await runAction(() async {
      final codes = await ref
          .read(securityRepositoryProvider)
          .enableTotp(_code.text.trim());
      if (mounted) setState(() => _recovery = codes);
    });
  }

  @override
  Widget build(BuildContext context) {
    if (_recovery != null) {
      return AlertDialog(
        title: const Text('Two-factor enabled'),
        content: _DialogBody(
          children: [
            RecoveryCodesView(
              codes: _recovery!,
              onDone: () => Navigator.of(context).pop(true),
            ),
          ],
        ),
      );
    }
    return AlertDialog(
      title: const Text('Enable authenticator app'),
      content: _DialogBody(
        children: [
          if (_loadError != null)
            ErrorBanner(message: _loadError!)
          else if (_setup == null)
            const Center(child: CircularProgressIndicator())
          else
            Form(
              key: _formKey,
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  const Text(
                    'Scan this QR code with your authenticator app, then '
                    'enter the 6-digit code.',
                  ),
                  const SizedBox(height: 12),
                  Center(
                    child: Container(
                      color: Colors.white,
                      padding: const EdgeInsets.all(8),
                      child: QrImageView(
                        data: _setup!.otpauthUrl,
                        size: 180,
                        semanticsLabel: 'TOTP QR code',
                      ),
                    ),
                  ),
                  const SizedBox(height: 12),
                  Row(
                    children: [
                      Expanded(
                        child: SelectableText(
                          _setup!.secret,
                          key: const Key('totp_secret'),
                          style: const TextStyle(fontFamily: 'monospace'),
                        ),
                      ),
                      IconButton(
                        tooltip: 'Copy secret',
                        icon: const Icon(Icons.copy),
                        onPressed: () => Clipboard.setData(
                          ClipboardData(text: _setup!.secret),
                        ),
                      ),
                    ],
                  ),
                  const SizedBox(height: 12),
                  if (errorText != null) ...[
                    ErrorBanner(message: errorText!),
                    const SizedBox(height: 12),
                  ],
                  AppTextField(
                    controller: _code,
                    label: 'Authenticator code',
                    keyboardType: TextInputType.number,
                    textInputAction: TextInputAction.done,
                    validator: Validators.totp,
                    onSubmitted: _enable,
                  ),
                ],
              ),
            ),
        ],
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(false),
          child: const Text('Cancel'),
        ),
        FilledButton(
          onPressed: (_setup == null || busy) ? null : _enable,
          child: const Text('Enable'),
        ),
      ],
    );
  }
}

/// Disable TOTP: needs the account password and a current code.
class DisableTotpDialog extends ConsumerStatefulWidget {
  const DisableTotpDialog({super.key});

  @override
  ConsumerState<DisableTotpDialog> createState() => _DisableTotpDialogState();
}

class _DisableTotpDialogState extends ConsumerState<DisableTotpDialog>
    with AsyncActionMixin<DisableTotpDialog> {
  /// Google-created accounts have no password: the code alone is required.
  bool get _hasPassword {
    final auth = ref.read(authControllerProvider);
    return auth is! Authenticated || auth.user.hasPassword;
  }

  final _formKey = GlobalKey<FormState>();
  final _password = TextEditingController();
  final _code = TextEditingController();

  @override
  void dispose() {
    _password.dispose();
    _code.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    if (!_formKey.currentState!.validate()) return;
    final ok = await runAction(
      () => ref
          .read(securityRepositoryProvider)
          .disableTotp(
            password: _hasPassword ? _password.text : null,
            code: _code.text.trim(),
          ),
    );
    if (ok && mounted) Navigator.of(context).pop(true);
  }

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: const Text('Disable two-factor'),
      content: _DialogBody(
        children: [
          Form(
            key: _formKey,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                if (errorText != null) ...[
                  ErrorBanner(message: errorText!),
                  const SizedBox(height: 12),
                ],
                if (_hasPassword) ...[
                  PasswordField(
                    controller: _password,
                    label: 'Password',
                    validator: (v) => Validators.required(v, 'Password'),
                  ),
                  const SizedBox(height: 12),
                ],
                AppTextField(
                  controller: _code,
                  label: 'Authenticator code',
                  keyboardType: TextInputType.number,
                  textInputAction: TextInputAction.done,
                  validator: Validators.totp,
                  onSubmitted: _submit,
                ),
              ],
            ),
          ),
        ],
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(false),
          child: const Text('Cancel'),
        ),
        FilledButton(
          onPressed: busy ? null : _submit,
          child: const Text('Disable'),
        ),
      ],
    );
  }
}

/// Regenerate recovery codes (needs a current TOTP code).
class RegenerateCodesDialog extends ConsumerStatefulWidget {
  const RegenerateCodesDialog({super.key});

  @override
  ConsumerState<RegenerateCodesDialog> createState() =>
      _RegenerateCodesDialogState();
}

class _RegenerateCodesDialogState extends ConsumerState<RegenerateCodesDialog>
    with AsyncActionMixin<RegenerateCodesDialog> {
  final _formKey = GlobalKey<FormState>();
  final _code = TextEditingController();
  List<String>? _codes;

  @override
  void dispose() {
    _code.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    if (!_formKey.currentState!.validate()) return;
    await runAction(() async {
      final codes = await ref
          .read(securityRepositoryProvider)
          .regenerateRecoveryCodes(_code.text.trim());
      if (mounted) setState(() => _codes = codes);
    });
  }

  @override
  Widget build(BuildContext context) {
    if (_codes != null) {
      return AlertDialog(
        title: const Text('New recovery codes'),
        content: _DialogBody(
          children: [
            RecoveryCodesView(
              codes: _codes!,
              onDone: () => Navigator.of(context).pop(),
            ),
          ],
        ),
      );
    }
    return AlertDialog(
      title: const Text('Regenerate recovery codes'),
      content: _DialogBody(
        children: [
          Form(
            key: _formKey,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                const Text('Old codes stop working. Enter a current code.'),
                const SizedBox(height: 12),
                if (errorText != null) ...[
                  ErrorBanner(message: errorText!),
                  const SizedBox(height: 12),
                ],
                AppTextField(
                  controller: _code,
                  label: 'Authenticator code',
                  keyboardType: TextInputType.number,
                  textInputAction: TextInputAction.done,
                  validator: Validators.totp,
                  onSubmitted: _submit,
                ),
              ],
            ),
          ),
        ],
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(),
          child: const Text('Cancel'),
        ),
        FilledButton(
          onPressed: busy ? null : _submit,
          child: const Text('Regenerate'),
        ),
      ],
    );
  }
}
