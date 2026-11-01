import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../core/network/api_exception.dart';
import '../../../shared/widgets/banners.dart';
import '../../../shared/widgets/form_page.dart';
import '../../../shared/widgets/primary_button.dart';
import '../application/auth_controller.dart';

/// Route: `/verify-email?token=...`. Calls the API on open.
class VerifyEmailScreen extends ConsumerStatefulWidget {
  const VerifyEmailScreen({super.key, required this.token});

  final String? token;

  @override
  ConsumerState<VerifyEmailScreen> createState() => _VerifyEmailScreenState();
}

class _VerifyEmailScreenState extends ConsumerState<VerifyEmailScreen> {
  bool _loading = true;
  String? _message;
  String? _error;

  @override
  void initState() {
    super.initState();
    _verify();
  }

  Future<void> _verify() async {
    final token = widget.token;
    if (token == null || token.isEmpty) {
      _loading = false;
      _error = 'This verification link is invalid or incomplete.';
      return;
    }
    try {
      final message = await ref.read(authRepositoryProvider).verifyEmail(token);
      if (!mounted) return;
      setState(() {
        _loading = false;
        _message = message.isEmpty ? 'Your email has been verified.' : message;
      });
      // Refresh the cached user so "verified" shows up if already signed in.
      if (ref.read(authControllerProvider) is Authenticated) {
        await ref.read(authControllerProvider.notifier).refreshUser();
      }
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _loading = false;
        _error = errorMessage(e);
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final signedIn = ref.watch(authControllerProvider) is Authenticated;
    return FormPage(
      title: 'Verify email',
      children: [
        if (_loading)
          const Center(child: CircularProgressIndicator())
        else ...[
          if (_error != null) ErrorBanner(message: _error!),
          if (_message != null) InfoBanner(message: _message!),
          const SizedBox(height: 24),
          PrimaryButton(
            label: signedIn ? 'Continue' : 'Go to sign in',
            onPressed: () => context.go(signedIn ? '/home' : '/login'),
          ),
        ],
      ],
    );
  }
}
