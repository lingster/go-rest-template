import 'dart:async';

import 'package:dio/dio.dart';

import '../storage/token_storage.dart';

/// Result of a successful refresh call.
typedef TokenPair = ({String accessToken, String refreshToken});

/// Performs the actual `/auth/refresh` call. Must NOT use a [Dio] that has an
/// [AuthInterceptor] attached (to avoid recursion).
typedef RefreshCallback = Future<TokenPair> Function(String refreshToken);

/// Attaches the bearer token and, on a 401, performs ONE refresh (shared by
/// all concurrent failing requests) and retries the original request.
/// If the refresh fails the stored tokens are cleared and [onSessionExpired]
/// fires.
class AuthInterceptor extends Interceptor {
  AuthInterceptor({
    required this.storage,
    required this.dio,
    required this.refresh,
    required this.onSessionExpired,
  });

  final TokenStorage storage;

  /// The client used to replay the failed request.
  final Dio dio;
  final RefreshCallback refresh;
  final void Function() onSessionExpired;

  static const _retriedKey = 'auth_retried';
  Future<String?>? _inFlight;

  bool _isAuthEndpoint(String path) => path.contains('/auth/');

  @override
  Future<void> onRequest(
    RequestOptions options,
    RequestInterceptorHandler handler,
  ) async {
    if (!_isAuthEndpoint(options.path)) {
      final token = await storage.readAccessToken();
      if (token != null) {
        options.headers['Authorization'] = 'Bearer $token';
      }
    }
    handler.next(options);
  }

  @override
  Future<void> onError(
    DioException err,
    ErrorInterceptorHandler handler,
  ) async {
    final request = err.requestOptions;
    final eligible =
        err.response?.statusCode == 401 &&
        !_isAuthEndpoint(request.path) &&
        request.extra[_retriedKey] != true;
    if (!eligible) return handler.next(err);

    final newToken = await _refreshOnce();
    if (newToken == null) return handler.next(err);

    try {
      request.extra[_retriedKey] = true;
      request.headers['Authorization'] = 'Bearer $newToken';
      handler.resolve(await dio.fetch<dynamic>(request));
    } on DioException catch (e) {
      handler.next(e);
    }
  }

  /// Single-flight: concurrent callers await the same future.
  Future<String?> _refreshOnce() {
    return _inFlight ??= _doRefresh().whenComplete(() => _inFlight = null);
  }

  Future<String?> _doRefresh() async {
    final refreshToken = await storage.readRefreshToken();
    if (refreshToken == null) {
      await _expire();
      return null;
    }
    try {
      final pair = await refresh(refreshToken);
      await storage.save(
        accessToken: pair.accessToken,
        refreshToken: pair.refreshToken,
      );
      return pair.accessToken;
    } catch (_) {
      await _expire();
      return null;
    }
  }

  Future<void> _expire() async {
    await storage.clear();
    onSessionExpired();
  }
}
