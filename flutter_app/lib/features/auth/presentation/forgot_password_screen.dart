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

class ForgotPasswordScreen extends ConsumerStatefulWidget {
  const ForgotPasswordScreen({super.key});

  @override
  ConsumerState<ForgotPasswordScreen> createState() =>
      _ForgotPasswordScreenState();
}

class _ForgotPasswordScreenState extends ConsumerState<ForgotPasswordScreen>
    with AsyncActionMixin<ForgotPasswordScreen> {
  final _formKey = GlobalKey<FormState>();
  final _email = TextEditingController();
  String? _message;

  @override
  void dispose() {
    _email.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    if (!_formKey.currentState!.validate()) return;
    String? message;
    final ok = await runAction(() async {
      message = await ref
          .read(authRepositoryProvider)
          .forgotPassword(_email.text.trim());
    });
    if (ok && mounted) {
      setState(
        () => _message = (message == null || message!.isEmpty)
            ? 'If that account exists, a reset link has been sent.'
            : message,
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    return FormPage(
      title: 'Forgot password',
      subtitle: "Enter your email and we'll send you a reset link.",
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
              if (_message != null) ...[
                InfoBanner(message: _message!),
                const SizedBox(height: 16),
              ],
              AppTextField(
                controller: _email,
                label: 'Email',
                keyboardType: TextInputType.emailAddress,
                textInputAction: TextInputAction.done,
                autofillHints: const [AutofillHints.email],
                validator: Validators.email,
                onSubmitted: _submit,
              ),
              const SizedBox(height: 24),
              PrimaryButton(
                label: 'Send reset link',
                loading: busy,
                onPressed: _submit,
              ),
              const SizedBox(height: 16),
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
