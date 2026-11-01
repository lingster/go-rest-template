import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../shared/async_action_mixin.dart';
import '../../../shared/validators.dart';
import '../../../shared/widgets/app_text_field.dart';
import '../../../shared/widgets/banners.dart';
import '../../../shared/widgets/form_page.dart';
import '../../../shared/widgets/primary_button.dart';
import '../application/auth_controller.dart';
import '../data/auth_models.dart';

/// Second step of login: TOTP code or recovery code.
class MfaScreen extends ConsumerStatefulWidget {
  const MfaScreen({super.key, required this.challenge});

  final MfaChallenge challenge;

  @override
  ConsumerState<MfaScreen> createState() => _MfaScreenState();
}

class _MfaScreenState extends ConsumerState<MfaScreen>
    with AsyncActionMixin<MfaScreen> {
  final _formKey = GlobalKey<FormState>();
  final _code = TextEditingController();
  bool _useRecovery = false;

  @override
  void dispose() {
    _code.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    if (!_formKey.currentState!.validate()) return;
    await runAction(
      () => ref
          .read(authControllerProvider.notifier)
          .verifyMfa(widget.challenge.mfaToken, _code.text.trim()),
    );
    // On success the router redirect sends the user to /home.
  }

  @override
  Widget build(BuildContext context) {
    return FormPage(
      title: 'Two-factor authentication',
      subtitle: _useRecovery
          ? 'Enter one of your recovery codes.'
          : 'Enter the 6-digit code from your authenticator app.',
      children: [
        Form(
          key: _formKey,
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              if (errorText != null) ...[
                ErrorBanner(message: errorText!),
                const SizedBox(height: 16),
              ],
              AppTextField(
                key: ValueKey(_useRecovery),
                controller: _code,
                label: _useRecovery ? 'Recovery code' : 'Authenticator code',
                autofocus: true,
                keyboardType: _useRecovery
                    ? TextInputType.text
                    : TextInputType.number,
                textInputAction: TextInputAction.done,
                autofillHints: const [AutofillHints.oneTimeCode],
                validator: _useRecovery
                    ? Validators.totpOrRecovery
                    : Validators.totp,
                onSubmitted: _submit,
              ),
              const SizedBox(height: 24),
              PrimaryButton(label: 'Verify', loading: busy, onPressed: _submit),
              const SizedBox(height: 8),
              TextButton(
                key: const Key('toggle_recovery'),
                onPressed: () => setState(() {
                  _useRecovery = !_useRecovery;
                  _code.clear();
                  errorText = null;
                }),
                child: Text(
                  _useRecovery
                      ? 'Use authenticator code instead'
                      : 'Use a recovery code instead',
                ),
              ),
              TextButton(
                onPressed: () => context.go('/login'),
                child: const Text('Back to sign in'),
              ),
            ],
          ),
        ),
      ],
    );
  }
}
