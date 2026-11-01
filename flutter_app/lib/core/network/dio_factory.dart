import 'package:dio/dio.dart';

import 'auth_interceptor.dart';

/// Plain JSON client without auth handling.
Dio createBaseDio(String baseUrl) => Dio(
  BaseOptions(
    baseUrl: baseUrl,
    connectTimeout: const Duration(seconds: 15),
    receiveTimeout: const Duration(seconds: 30),
    headers: {'Accept': 'application/json'},
    contentType: Headers.jsonContentType,
  ),
);

/// Builds the `/auth/refresh` call on top of an interceptor-free [Dio].
RefreshCallback createRefreshCallback(Dio bareDio) => (refreshToken) async {
  final response = await bareDio.post<Map<String, dynamic>>(
    '/auth/refresh',
    data: {'refresh_token': refreshToken},
  );
  final data = response.data!;
  return (
    accessToken: data['access_token'] as String,
    refreshToken: data['refresh_token'] as String,
  );
};
