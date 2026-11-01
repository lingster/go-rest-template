/// Form field validators (return an error string or null).
abstract final class Validators {
  static final _email = RegExp(r'^[^@\s]+@[^@\s]+\.[^@\s]+$');

  static String? required(String? v, [String label = 'This field']) =>
      (v == null || v.trim().isEmpty) ? '$label is required' : null;

  static String? email(String? v) {
    final r = required(v, 'Email');
    if (r != null) return r;
    return _email.hasMatch(v!.trim()) ? null : 'Enter a valid email address';
  }

  /// Password policy from the API contract: 8-128 characters.
  static String? newPassword(String? v) {
    if (v == null || v.isEmpty) return 'Password is required';
    if (v.length < 8) return 'Use at least 8 characters';
    if (v.length > 128) return 'Use at most 128 characters';
    return null;
  }

  static String? Function(String?) matches(
    String Function() other, [
    String message = 'Passwords do not match',
  ]) =>
      (v) => v == other() ? null : message;

  static String? totpOrRecovery(String? v) => required(v, 'Code');

  static String? totp(String? v) {
    final r = required(v, 'Code');
    if (r != null) return r;
    return RegExp(r'^\d{6}$').hasMatch(v!.trim())
        ? null
        : 'Enter the 6-digit code';
  }
}
