import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../shared/async_action_mixin.dart';
import '../../../shared/validators.dart';
import '../../../shared/widgets/app_text_field.dart';
import '../../../shared/widgets/banners.dart';
import '../../../shared/widgets/form_page.dart';
import '../../../shared/widgets/password_field.dart';
import '../../../shared/widgets/primary_button.dart';
import '../../auth/application/auth_controller.dart';
import '../application/profile_providers.dart';

class ProfileScreen extends ConsumerWidget {
  const ProfileScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final state = ref.watch(authControllerProvider);
    final email = state is Authenticated ? state.user.email : '';
    return FormPage(
      title: 'Profile',
      appBar: AppBar(title: const Text('Profile')),
      children: [
        Text(email, style: Theme.of(context).textTheme.titleMedium),
        const SizedBox(height: 24),
        const _NameCard(),
        const SizedBox(height: 24),
        const _PasswordCard(),
        const SizedBox(height: 24),
        OutlinedButton.icon(
          key: const Key('logout'),
          icon: const Icon(Icons.logout),
          label: const Text('Log out'),
          onPressed: () => ref.read(authControllerProvider.notifier).logout(),
        ),
      ],
    );
  }
}

class _NameCard extends ConsumerStatefulWidget {
  const _NameCard();

  @override
  ConsumerState<_NameCard> createState() => _NameCardState();
}

class _NameCardState extends ConsumerState<_NameCard>
    with AsyncActionMixin<_NameCard> {
  final _formKey = GlobalKey<FormState>();
  late final TextEditingController _name;
  bool _saved = false;

  @override
  void initState() {
    super.initState();
    final s = ref.read(authControllerProvider);
    _name = TextEditingController(text: s is Authenticated ? s.user.name : '');
  }

  @override
  void dispose() {
    _name.dispose();
    super.dispose();
  }

  Future<void> _save() async {
    if (!_formKey.currentState!.validate()) return;
    setState(() => _saved = false);
    final ok = await runAction(() async {
      final user = await ref
          .read(profileRepositoryProvider)
          .updateName(_name.text.trim());
      ref.read(authControllerProvider.notifier).setUser(user);
    });
    if (ok && mounted) setState(() => _saved = true);
  }

  @override
  Widget build(BuildContext context) {
    return Form(
      key: _formKey,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Text('Name', style: Theme.of(context).textTheme.titleMedium),
          const SizedBox(height: 12),
          if (errorText != null) ...[
            ErrorBanner(message: errorText!),
            const SizedBox(height: 12),
          ],
          if (_saved) ...[
            const InfoBanner(message: 'Profile updated.'),
            const SizedBox(height: 12),
          ],
          AppTextField(
            controller: _name,
            label: 'Display name',
            textInputAction: TextInputAction.done,
            onSubmitted: _save,
            validator: (v) => Validators.required(v, 'Name'),
          ),
          const SizedBox(height: 12),
          PrimaryButton(label: 'Save', loading: busy, onPressed: _save),
        ],
      ),
    );
  }
}

class _PasswordCard extends ConsumerStatefulWidget {
  const _PasswordCard();

  @override
  ConsumerState<_PasswordCard> createState() => _PasswordCardState();
}

class _PasswordCardState extends ConsumerState<_PasswordCard>
    with AsyncActionMixin<_PasswordCard> {
  final _formKey = GlobalKey<FormState>();
  final _current = TextEditingController();
  final _new = TextEditingController();
  final _confirm = TextEditingController();
  bool _changed = false;

  @override
  void dispose() {
    _current.dispose();
    _new.dispose();
    _confirm.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    if (!_formKey.currentState!.validate()) return;
    setState(() => _changed = false);
    final ok = await runAction(
      () => ref
          .read(profileRepositoryProvider)
          .changePassword(
            currentPassword: _current.text,
            newPassword: _new.text,
          ),
    );
    if (ok && mounted) {
      _formKey.currentState!.reset();
      _current.clear();
      _new.clear();
      _confirm.clear();
      setState(() => _changed = true);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Form(
      key: _formKey,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Text(
            'Change password',
            style: Theme.of(context).textTheme.titleMedium,
          ),
          const SizedBox(height: 12),
          if (errorText != null) ...[
            ErrorBanner(message: errorText!),
            const SizedBox(height: 12),
          ],
          if (_changed) ...[
            const InfoBanner(message: 'Password changed.'),
            const SizedBox(height: 12),
          ],
          PasswordField(
            controller: _current,
            label: 'Current password',
            validator: (v) => Validators.required(v, 'Current password'),
          ),
          const SizedBox(height: 12),
          PasswordField(
            controller: _new,
            label: 'New password',
            newPassword: true,
            validator: Validators.newPassword,
          ),
          const SizedBox(height: 12),
          PasswordField(
            controller: _confirm,
            label: 'Confirm new password',
            newPassword: true,
            textInputAction: TextInputAction.done,
            onSubmitted: _submit,
            validator: Validators.matches(() => _new.text),
          ),
          const SizedBox(height: 12),
          PrimaryButton(
            label: 'Change password',
            loading: busy,
            onPressed: _submit,
          ),
        ],
      ),
    );
  }
}
