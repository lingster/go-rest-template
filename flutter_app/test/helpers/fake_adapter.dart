import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';

typedef FakeHandler = FutureOr<FakeResponse> Function(RequestOptions options);

class FakeResponse {
  FakeResponse(this.status, [Object? body])
    : body = body == null ? '' : jsonEncode(body);
  final int status;
  final String body;
}

/// In-memory [HttpClientAdapter] that records requests.
class FakeAdapter implements HttpClientAdapter {
  FakeAdapter(this.handler);

  final FakeHandler handler;
  final List<RequestOptions> requests = [];

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    requests.add(options);
    final r = await handler(options);
    return ResponseBody.fromString(
      r.body,
      r.status,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

Map<String, dynamic> userJson({bool totp = false}) => {
  'id': 'u1',
  'email': 'a@b.com',
  'name': 'Ada',
  'role': 'user',
  'email_verified': true,
  'is_active': true,
  'totp_enabled': totp,
  'passkey_count': 0,
  'created_at': '2024-01-01T00:00:00Z',
  'updated_at': '2024-01-01T00:00:00Z',
};

Map<String, dynamic> tokensJson([String suffix = '1']) => {
  'access_token': 'access$suffix',
  'refresh_token': 'refresh$suffix',
  'token_type': 'Bearer',
  'expires_in': 900,
};

Map<String, dynamic> errorJson(
  String code, [
  String message = 'msg',
  Map<String, String>? details,
]) => {
  'error': {'code': code, 'message': message, 'details': ?details},
};
