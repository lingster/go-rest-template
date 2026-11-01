import 'package:flutter/material.dart';

/// Maximum width for form content on wide screens.
const double kFormMaxWidth = 420;

/// Scrollable, centered column constrained to [kFormMaxWidth]. Used by every
/// auth screen and by in-app form pages.
class FormPage extends StatelessWidget {
  const FormPage({
    super.key,
    required this.title,
    required this.children,
    this.subtitle,
    this.appBar,
  });

  final String title;
  final String? subtitle;
  final List<Widget> children;

  /// When given, replaces the default (title shown in body, no app bar).
  final PreferredSizeWidget? appBar;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Scaffold(
      appBar: appBar,
      body: SafeArea(
        child: Center(
          child: SingleChildScrollView(
            padding: const EdgeInsets.all(24),
            child: ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: kFormMaxWidth),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  if (appBar == null) ...[
                    Text(
                      title,
                      style: theme.textTheme.headlineMedium,
                      textAlign: TextAlign.center,
                    ),
                    if (subtitle != null) ...[
                      const SizedBox(height: 8),
                      Text(
                        subtitle!,
                        style: theme.textTheme.bodyMedium,
                        textAlign: TextAlign.center,
                      ),
                    ],
                    const SizedBox(height: 24),
                  ],
                  ...children,
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}
