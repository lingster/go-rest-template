import 'package:flutter/widgets.dart';

import '../core/network/api_exception.dart';

/// Shared loading/error handling for stateful form screens.
mixin AsyncActionMixin<T extends StatefulWidget> on State<T> {
  bool busy = false;
  String? errorText;

  /// The last [ApiException], so screens can react to specific codes.
  ApiException? lastApiError;

  /// Runs [action], toggling [busy] and capturing errors. Returns true on
  /// success.
  Future<bool> runAction(Future<void> Function() action) async {
    setState(() {
      busy = true;
      errorText = null;
      lastApiError = null;
    });
    try {
      await action();
      if (mounted) setState(() => busy = false);
      return true;
    } on ApiException catch (e) {
      if (mounted) {
        setState(() {
          busy = false;
          lastApiError = e;
          errorText = e.displayMessage;
        });
      }
    } catch (e) {
      if (mounted) {
        setState(() {
          busy = false;
          errorText = errorMessage(e);
        });
      }
    }
    return false;
  }
}
