import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/providers.dart';
import '../data/security_repository.dart';

final securityRepositoryProvider = Provider<SecurityRepository>(
  (ref) => SecurityRepository(
    api: ref.watch(apiClientProvider),
    passkeys: ref.watch(passkeyServiceProvider),
  ),
);

/// The user's registered passkeys. Invalidate to reload.
final passkeysProvider = FutureProvider.autoDispose<List<Passkey>>(
  (ref) => ref.watch(securityRepositoryProvider).listPasskeys(),
);
