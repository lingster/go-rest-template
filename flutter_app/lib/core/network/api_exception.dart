import 'package:dio/dio.dart';

/// Error codes defined by the API contract, plus client-side extras.
enum ApiErrorCode {
  validationError('validation_error'),
  invalidCredentials('invalid_credentials'),
  unauthorized('unauthorized'),
  invalidToken('invalid_token'),
  tokenExpired('token_expired'),
  emailNotVerified('email_not_verified'),
  accountDisabled('account_disabled'),
  forbidden('forbidden'),
  invalidMfaCode('invalid_mfa_code'),
  notFound('not_found'),
  conflict('conflict'),
  rateLimited('rate_limited'),
  internalError('internal_error'),

  /// Client-side: no response (offline, DNS, timeout, CORS ...).
  network('network_error'),

  /// Anything the contract does not list.
  unknown('unknown');

  const ApiErrorCode(this.wire);
  final String wire;

  static ApiErrorCode parse(String? value) => ApiErrorCode.values.firstWhere(
    (c) => c.wire == value,
    orElse: () => ApiErrorCode.unknown,
  );
}

/// Typed representation of the contract's error body:
/// `{"error": {"code": "...", "message": "...", "details": {...}}}`.
class ApiException implements Exception {
  const ApiException({
    required this.code,
    required this.message,
    this.statusCode,
    this.details = const {},
  });

  final ApiErrorCode code;
  final String message;
  final int? statusCode;
  final Map<String, String> details;

  /// Parses a decoded response body. Tolerates malformed bodies.
  factory ApiException.fromResponseBody(Object? body, {int? statusCode}) {
    if (body is Map && body['error'] is Map) {
      final error = body['error'] as Map;
      final rawDetails = error['details'];
      final details = <String, String>{};
      if (rawDetails is Map) {
        rawDetails.forEach((k, v) => details['$k'] = '$v');
      }
      return ApiException(
        code: ApiErrorCode.parse(error['code'] as String?),
        message: (error['message'] as String?) ?? 'Request failed',
        statusCode: statusCode,
        details: details,
      );
    }
    final suffix = statusCode != null ? ' ($statusCode)' : '';
    return ApiException(
      code: ApiErrorCode.unknown,
      message: 'Unexpected server response$suffix',
      statusCode: statusCode,
    );
  }

  factory ApiException.fromDio(DioException e) {
    final inner = e.error;
    if (inner is ApiException) return inner;
    final response = e.response;
    if (response != null) {
      return ApiException.fromResponseBody(
        response.data,
        statusCode: response.statusCode,
      );
    }
    return const ApiException(
      code: ApiErrorCode.network,
      message: 'Cannot reach the server. Check your connection.',
    );
  }

  /// Message plus per-field details, for display.
  String get displayMessage {
    if (details.isEmpty) return message;
    final lines = details.entries.map((e) => '${e.key}: ${e.value}');
    return '$message\n${lines.join('\n')}';
  }

  @override
  String toString() => 'ApiException(${code.wire}, $statusCode, $message)';
}

/// Implemented by non-API errors that carry a user-presentable message.
abstract interface class UserMessageException implements Exception {
  String get userMessage;
}

/// Turns any thrown object into a user-presentable message.
String errorMessage(Object error) => switch (error) {
  ApiException e => e.displayMessage,
  UserMessageException e => e.userMessage,
  _ => 'Something went wrong.',
};
