import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_web_plugins/url_strategy.dart';

import 'app.dart';

void main() {
  // Clean URLs so email links like /verify-email?token=... work on web.
  usePathUrlStrategy();
  runApp(const ProviderScope(child: App()));
}
