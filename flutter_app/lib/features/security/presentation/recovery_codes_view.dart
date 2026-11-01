import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

/// Shows recovery codes (only visible once) with a copy action.
class RecoveryCodesView extends StatelessWidget {
  const RecoveryCodesView({
    super.key,
    required this.codes,
    required this.onDone,
  });

  final List<String> codes;
  final VoidCallback onDone;

  @override
  Widget build(BuildContext context) {
    return Column(
      mainAxisSize: MainAxisSize.min,
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        const Text(
          'Save these recovery codes somewhere safe. Each can be used once '
          'if you lose your authenticator. They will not be shown again.',
        ),
        const SizedBox(height: 12),
        Container(
          padding: const EdgeInsets.all(12),
          decoration: BoxDecoration(
            color: Theme.of(context).colorScheme.surfaceContainerHighest,
            borderRadius: BorderRadius.circular(8),
          ),
          child: SelectableText(
            codes.join('\n'),
            key: const Key('recovery_codes'),
            style: const TextStyle(fontFamily: 'monospace', height: 1.6),
          ),
        ),
        const SizedBox(height: 12),
        OutlinedButton.icon(
          icon: const Icon(Icons.copy),
          label: const Text('Copy codes'),
          onPressed: () async {
            await Clipboard.setData(ClipboardData(text: codes.join('\n')));
            if (context.mounted) {
              ScaffoldMessenger.of(context).showSnackBar(
                const SnackBar(content: Text('Recovery codes copied')),
              );
            }
          },
        ),
        const SizedBox(height: 8),
        FilledButton(onPressed: onDone, child: const Text("I've saved them")),
      ],
    );
  }
}
