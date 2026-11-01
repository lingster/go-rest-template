import 'package:dio/dio.dart';

import 'api_exception.dart';

/// Thin wrapper over [Dio] that decodes JSON and maps every failure to
/// [ApiException]. All repositories talk to the API through this class.
class ApiClient {
  ApiClient(this._dio);

  final Dio _dio;

  Future<T> send<T>(
    String method,
    String path, {
    Object? body,
    Map<String, dynamic>? query,
    required T Function(Object? json) decode,
  }) async {
    try {
      final response = await _dio.request<Object?>(
        path,
        data: body,
        queryParameters: query,
        options: Options(method: method),
      );
      return decode(response.data);
    } on DioException catch (e) {
      throw ApiException.fromDio(e);
    }
  }

  Future<T> get<T>(String path, {required T Function(Object? json) decode}) =>
      send('GET', path, decode: decode);

  Future<T> post<T>(
    String path, {
    Object? body,
    required T Function(Object? json) decode,
  }) => send('POST', path, body: body, decode: decode);

  Future<T> patch<T>(
    String path, {
    Object? body,
    required T Function(Object? json) decode,
  }) => send('PATCH', path, body: body, decode: decode);

  Future<void> postVoid(String path, {Object? body}) =>
      send<void>('POST', path, body: body, decode: (_) {});

  Future<void> delete(String path) =>
      send<void>('DELETE', path, decode: (_) {});
}

/// Decodes `{"message": "..."}` responses.
String decodeMessage(Object? json) =>
    json is Map ? (json['message'] as String? ?? '') : '';

/// Casts a decoded JSON value to an object map.
Map<String, dynamic> asMap(Object? json) =>
    Map<String, dynamic>.from(json as Map);
