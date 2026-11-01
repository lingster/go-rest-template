import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:saas_app/core/network/api_exception.dart';

void main() {
  test('parses contract error body', () {
    final e = ApiException.fromResponseBody({
      'error': {
        'code': 'validation_error',
        'message': 'Invalid input',
        'details': {'email': 'invalid'},
      },
    }, statusCode: 422);
    expect(e.code, ApiErrorCode.validationError);
    expect(e.statusCode, 422);
    expect(e.details['email'], 'invalid');
    expect(e.displayMessage, contains('email: invalid'));
  });

  test('maps every contract code', () {
    for (final c in ApiErrorCode.values) {
      expect(ApiErrorCode.parse(c.wire), c);
    }
    expect(
      ApiErrorCode.parse('email_not_verified'),
      ApiErrorCode.emailNotVerified,
    );
    expect(ApiErrorCode.parse('nope'), ApiErrorCode.unknown);
    expect(ApiErrorCode.parse(null), ApiErrorCode.unknown);
  });

  test('tolerates malformed bodies', () {
    final e = ApiException.fromResponseBody('<html>', statusCode: 502);
    expect(e.code, ApiErrorCode.unknown);
    expect(e.statusCode, 502);
  });

  test('fromDio without response is a network error', () {
    final e = ApiException.fromDio(
      DioException(
        requestOptions: RequestOptions(path: '/x'),
        type: DioExceptionType.connectionError,
      ),
    );
    expect(e.code, ApiErrorCode.network);
  });

  test('fromDio reads the response body', () {
    final ro = RequestOptions(path: '/x');
    final e = ApiException.fromDio(
      DioException(
        requestOptions: ro,
        response: Response(
          requestOptions: ro,
          statusCode: 401,
          data: {
            'error': {'code': 'invalid_credentials', 'message': 'bad'},
          },
        ),
      ),
    );
    expect(e.code, ApiErrorCode.invalidCredentials);
    expect(e.message, 'bad');
  });
}
