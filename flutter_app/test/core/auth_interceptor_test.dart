import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:saas_app/core/network/auth_interceptor.dart';
import 'package:saas_app/core/storage/token_storage.dart';

import '../helpers/fake_adapter.dart';

void main() {
  late InMemoryTokenStorage storage;
  late Dio dio;
  late FakeAdapter adapter;
  late int refreshCalls;
  late int expiredCalls;
  late bool refreshFails;

  setUp(() async {
    storage = InMemoryTokenStorage();
    await storage.save(accessToken: 'old', refreshToken: 'r0');
    refreshCalls = 0;
    expiredCalls = 0;
    refreshFails = false;
    adapter = FakeAdapter((o) {
      final auth = o.headers['Authorization'];
      if (auth == 'Bearer new') return FakeResponse(200, {'ok': true});
      return FakeResponse(401, errorJson('token_expired'));
    });
    dio = Dio(BaseOptions(baseUrl: 'http://x/api/v1'))
      ..httpClientAdapter = adapter;
    dio.interceptors.add(
      AuthInterceptor(
        storage: storage,
        dio: dio,
        refresh: (rt) async {
          refreshCalls++;
          await Future<void>.delayed(const Duration(milliseconds: 20));
          if (refreshFails) throw Exception('refresh failed');
          expect(rt, 'r0');
          return (accessToken: 'new', refreshToken: 'r1');
        },
        onSessionExpired: () => expiredCalls++,
      ),
    );
  });

  test('attaches bearer token', () async {
    await storage.save(accessToken: 'new', refreshToken: 'r0');
    final res = await dio.get<dynamic>('/me');
    expect(res.statusCode, 200);
    expect(adapter.requests.single.headers['Authorization'], 'Bearer new');
    expect(refreshCalls, 0);
  });

  test(
    'concurrent 401s trigger a single refresh and are all retried',
    () async {
      final results = await Future.wait([
        dio.get<dynamic>('/me'),
        dio.get<dynamic>('/me/passkeys'),
        dio.get<dynamic>('/me'),
      ]);
      expect(results.map((r) => r.statusCode), everyElement(200));
      expect(refreshCalls, 1);
      expect(expiredCalls, 0);
      expect(await storage.readRefreshToken(), 'r1');
      expect(await storage.readAccessToken(), 'new');
      // 3 initial attempts + 3 retries
      expect(adapter.requests.length, 6);
    },
  );

  test(
    'refresh failure clears session, signals expiry once, surfaces 401',
    () async {
      refreshFails = true;
      final futures = [dio.get<dynamic>('/me'), dio.get<dynamic>('/me')];
      for (final f in futures) {
        await expectLater(
          f,
          throwsA(
            isA<DioException>().having(
              (e) => e.response?.statusCode,
              'status',
              401,
            ),
          ),
        );
      }
      expect(refreshCalls, 1);
      expect(expiredCalls, 1);
      expect(await storage.readRefreshToken(), isNull);
    },
  );

  test('does not loop when the retried request is still 401', () async {
    adapter = FakeAdapter((_) => FakeResponse(401, errorJson('unauthorized')));
    dio.httpClientAdapter = adapter;
    await expectLater(dio.get<dynamic>('/me'), throwsA(isA<DioException>()));
    expect(refreshCalls, 1);
    expect(adapter.requests.length, 2);
  });

  test('auth endpoints are not refreshed or given a token', () async {
    adapter = FakeAdapter(
      (_) => FakeResponse(401, errorJson('invalid_credentials')),
    );
    dio.httpClientAdapter = adapter;
    await expectLater(
      dio.post<dynamic>('/auth/login'),
      throwsA(isA<DioException>()),
    );
    expect(refreshCalls, 0);
    expect(
      adapter.requests.single.headers.containsKey('Authorization'),
      isFalse,
    );
  });
}
