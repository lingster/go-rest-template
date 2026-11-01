import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../shared/async_action_mixin.dart';
import '../../../shared/validators.dart';
import '../../../shared/widgets/banners.dart';
import '../../../shared/widgets/form_page.dart';
import '../../../shared/widgets/password_field.dart';
import '../../../shared/widgets/primary_button.dart';
import '../application/auth_controller.dart';

/// Route: `/reset-password?token=...`
class ResetPasswordScreen extends ConsumerStatefulWidget {
  const ResetPasswordScreen({super.key, required this.token});

  final String? token;

  @override
  ConsumerState<ResetPasswordScreen> createState() =>
      _ResetPasswordScreenState();
}

class _ResetPasswordScreenState extends ConsumerState<ResetPasswordScreen>
    with AsyncActionMixin<ResetPasswordScreen> {
  final _formKey = GlobalKey<FormState>();
  final _password = TextEditingController();
  final _confirm = TextEditingController();
  bool _done = false;

  @override
  void dispose() {
    _password.dispose();
    _confirm.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    if (!_formKey.currentState!.validate()) return;
    final ok = await runAction(() async {
      await ref
          .read(authRepositoryProvider)
          .resetPassword(widget.token!, _password.text);
    });
    if (ok && mounted) setState(() => _done = true);
  }

  @override
  Widget build(BuildContext context) {
    final token = widget.token;
    if (token == null || token.isEmpty) {
      return FormPage(
        title: 'Reset password',
        children: [
          const ErrorBanner(
            message: 'This reset link is invalid or incomplete.',
          ),
          const SizedBox(height: 16),
          TextButton(
            onPressed: () => context.go('/forgot-password'),
            child: const Text('Request a new link'),
          ),
        ],
      );
    }
    if (_done) {
      return FormPage(
        title: 'Password updated',
        children: [
          const InfoBanner(message: 'Your password has been reset.'),
          const SizedBox(height: 24),
          PrimaryButton(
            label: 'Sign in',
            onPressed: () => context.go('/login'),
          ),
        ],
      );
    }
    return FormPage(
      title: 'Reset password',
      subtitle: 'Choose a new password.',
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
              PasswordField(
                controller: _password,
                label: 'New password',
                newPassword: true,
                validator: Validators.newPassword,
              ),
              const SizedBox(height: 16),
              PasswordField(
                controller: _confirm,
                label: 'Confirm new password',
                newPassword: true,
                textInputAction: TextInputAction.done,
                onSubmitted: _submit,
                validator: Validators.matches(() => _password.text),
              ),
              const SizedBox(height: 24),
              PrimaryButton(
                label: 'Reset password',
                loading: busy,
                onPressed: _submit,
              ),
            ],
          ),
        ),
      ],
    );
  }
}
