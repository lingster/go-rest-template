import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../shared/async_action_mixin.dart';
import '../../../shared/validators.dart';
import '../../../shared/widgets/app_text_field.dart';
import '../../../shared/widgets/banners.dart';
import '../../../shared/widgets/form_page.dart';
import '../../../shared/widgets/password_field.dart';
import '../../../shared/widgets/primary_button.dart';
import '../application/auth_controller.dart';
import 'google_sign_in_button.dart';

class RegisterScreen extends ConsumerStatefulWidget {
  const RegisterScreen({super.key});

  @override
  ConsumerState<RegisterScreen> createState() => _RegisterScreenState();
}

class _RegisterScreenState extends ConsumerState<RegisterScreen>
    with AsyncActionMixin<RegisterScreen> {
  final _formKey = GlobalKey<FormState>();
  final _name = TextEditingController();
  final _email = TextEditingController();
  final _password = TextEditingController();
  final _confirm = TextEditingController();
  String? _successMessage;

  @override
  void dispose() {
    _name.dispose();
    _email.dispose();
    _password.dispose();
    _confirm.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    if (!_formKey.currentState!.validate()) return;
    String? message;
    final ok = await runAction(() async {
      final result = await ref
          .read(authRepositoryProvider)
          .register(
            email: _email.text.trim(),
            password: _password.text,
            name: _name.text.trim(),
          );
      message = result.message;
    });
    if (ok && mounted) {
      setState(() {
        _successMessage = (message == null || message!.isEmpty)
            ? 'Account created. Check your email to verify your address.'
            : message;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    if (_successMessage != null) {
      return FormPage(
        title: 'Check your email',
        children: [
          InfoBanner(message: _successMessage!),
          const SizedBox(height: 24),
          PrimaryButton(
            label: 'Back to sign in',
            onPressed: () => context.go('/login'),
          ),
        ],
      );
    }
    return FormPage(
      title: 'Create account',
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
                controller: _name,
                label: 'Name',
                autofillHints: const [AutofillHints.name],
                validator: (v) => Validators.required(v, 'Name'),
              ),
              const SizedBox(height: 16),
              AppTextField(
                controller: _email,
                label: 'Email',
                keyboardType: TextInputType.emailAddress,
                autofillHints: const [AutofillHints.email],
                validator: Validators.email,
              ),
              const SizedBox(height: 16),
              PasswordField(
                controller: _password,
                newPassword: true,
                validator: Validators.newPassword,
              ),
              const SizedBox(height: 16),
              PasswordField(
                controller: _confirm,
                label: 'Confirm password',
                newPassword: true,
                textInputAction: TextInputAction.done,
                onSubmitted: _submit,
                validator: Validators.matches(() => _password.text),
              ),
              const SizedBox(height: 24),
              PrimaryButton(
                label: 'Create account',
                loading: busy,
                onPressed: _submit,
              ),
              const GoogleSignInButton(),
              const SizedBox(height: 16),
              TextButton(
                onPressed: () => context.go('/login'),
                child: const Text('Already have an account? Sign in'),
              ),
            ],
          ),
        ),
      ],
    );
  }
}
