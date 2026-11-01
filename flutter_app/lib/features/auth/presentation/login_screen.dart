import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../core/network/api_exception.dart';
import '../../../shared/async_action_mixin.dart';
import '../../../shared/validators.dart';
import '../../../shared/widgets/app_text_field.dart';
import '../../../shared/widgets/banners.dart';
import '../../../shared/widgets/form_page.dart';
import '../../../shared/widgets/password_field.dart';
import '../../../shared/widgets/primary_button.dart';
import '../application/auth_controller.dart';
import '../data/auth_models.dart';

class LoginScreen extends ConsumerStatefulWidget {
  const LoginScreen({super.key});

  @override
  ConsumerState<LoginScreen> createState() => _LoginScreenState();
}

class _LoginScreenState extends ConsumerState<LoginScreen>
    with AsyncActionMixin<LoginScreen> {
  final _formKey = GlobalKey<FormState>();
  final _email = TextEditingController();
  final _password = TextEditingController();
  String? _info;
  bool _canUseBiometrics = false;

  @override
  void initState() {
    super.initState();
    ref.read(authControllerProvider.notifier).canUnlockWithBiometrics().then((
      v,
    ) {
      if (mounted && v) setState(() => _canUseBiometrics = true);
    });
  }

  @override
  void dispose() {
    _email.dispose();
    _password.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    if (!_formKey.currentState!.validate()) return;
    setState(() => _info = null);
    AuthResult? result;
    await runAction(() async {
      result = await ref
          .read(authControllerProvider.notifier)
          .login(_email.text.trim(), _password.text);
    });
    final r = result;
    if (r is MfaChallenge && mounted) {
      context.go('/2fa', extra: r);
    }
    // On success the router redirect sends the user to /home.
  }

  Future<void> _passkey() async {
    setState(() => _info = null);
    await runAction(
      () => ref
          .read(authControllerProvider.notifier)
          .loginWithPasskey(email: _email.text.trim()),
    );
  }

  Future<void> _biometric() async {
    await runAction(() => ref.read(authControllerProvider.notifier).unlock());
  }

  Future<void> _resend() async {
    final email = _email.text.trim();
    if (Validators.email(email) != null) return;
    final ok = await runAction(
      () => ref.read(authRepositoryProvider).resendVerification(email),
    );
    if (ok && mounted) {
      setState(() => _info = 'Verification email sent. Check your inbox.');
    }
  }

  @override
  Widget build(BuildContext context) {
    final needsVerification =
        lastApiError?.code == ApiErrorCode.emailNotVerified;
    return FormPage(
      title: 'Sign in',
      subtitle: 'Welcome back',
      children: [
        Form(
          key: _formKey,
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              if (errorText != null) ...[
                ErrorBanner(
                  message: errorText!,
                  action: needsVerification
                      ? TextButton(
                          key: const Key('resend_verification'),
                          onPressed: busy ? null : _resend,
                          child: const Text('Resend verification email'),
                        )
                      : null,
                ),
                const SizedBox(height: 16),
              ],
              if (_info != null) ...[
                InfoBanner(message: _info!),
                const SizedBox(height: 16),
              ],
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
                validator: (v) => Validators.required(v, 'Password'),
                textInputAction: TextInputAction.done,
                onSubmitted: _submit,
              ),
              Align(
                alignment: Alignment.centerRight,
                child: TextButton(
                  onPressed: () => context.go('/forgot-password'),
                  child: const Text('Forgot password?'),
                ),
              ),
              const SizedBox(height: 8),
              PrimaryButton(
                label: 'Sign in',
                loading: busy,
                onPressed: _submit,
              ),
              const SizedBox(height: 12),
              OutlinedButton.icon(
                key: const Key('passkey_signin'),
                icon: const Icon(Icons.fingerprint),
                label: const Text('Sign in with passkey'),
                onPressed: busy ? null : _passkey,
              ),
              if (_canUseBiometrics) ...[
                const SizedBox(height: 12),
                OutlinedButton.icon(
                  key: const Key('biometric_signin'),
                  icon: const Icon(Icons.face),
                  label: const Text('Sign in with biometrics'),
                  onPressed: busy ? null : _biometric,
                ),
              ],
              const SizedBox(height: 16),
              TextButton(
                onPressed: () => context.go('/register'),
                child: const Text("Don't have an account? Create one"),
              ),
            ],
          ),
        ),
      ],
    );
  }
}
