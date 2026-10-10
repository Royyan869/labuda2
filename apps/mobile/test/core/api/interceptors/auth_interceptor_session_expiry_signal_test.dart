// SESSION HONESTY (Tier 2) — AuthInterceptor session-expired signal contract.
//
// The signal producer was dead code: `AuthInterceptor._signalSessionExpiredOnce`
// was defined and `AuthController.handleSessionExpired` was wired as
// `onSessionExpired`, but nothing ever fired the signal — a 401 whose refresh
// failed left the user in an authenticated shell where every API call
// 401-looped silently (stale authenticated state).
//
// Canonical contract proven here:
//   - 401 + an EXISTING refresh session that cannot be refreshed → the
//     session-expired signal fires exactly once (the controller then runs the
//     canonical signOut → Unauthenticated).
//   - 401 + successful refresh → NO signal (session healthy).
//   - 401 with NO stored refresh token → NO signal (guest/anonymous request;
//     nothing to expire — the Phase 3B no-credential contract stays intact).
//   - non-401 → NO signal.

import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:hishumi/core/api/interceptors/auth_interceptor.dart';
import 'package:hishumi/core/common/result.dart';
import 'package:hishumi/core/src/interfaces/services/i_local_storage_service.dart';

class _FakeStorage implements ILocalStorageService {
  _FakeStorage({this.access, this.refresh});
  String? access;
  String? refresh;
  int saveCount = 0;

  @override
  Future<Result<void>> saveHiShumiCredential(String a, String r) async {
    if (a.isEmpty || r.isEmpty) return Result.error('empty');
    access = a;
    refresh = r;
    saveCount++;
    return Result.success(null);
  }

  @override
  Future<Result<String?>> readHiShumiAccessToken() async =>
      Result.success(access);

  @override
  Future<Result<String?>> readHiShumiRefreshToken() async =>
      Result.success(refresh);

  @override
  dynamic noSuchMethod(Invocation inv) => super.noSuchMethod(inv);
}

/// Adapter: original endpoint always 401; /auth/refresh scripted to
/// succeed or fail.
class _RefreshScriptAdapter implements HttpClientAdapter {
  _RefreshScriptAdapter({required this.refreshShouldFail});
  final bool refreshShouldFail;
  int refreshCalls = 0;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    if (options.path.contains('/auth/refresh')) {
      refreshCalls++;
      if (refreshShouldFail) {
        return ResponseBody.fromBytes(
          utf8.encode(
            jsonEncode({
              'success': false,
              'error': {'message': 'invalid refresh'},
            }),
          ),
          401,
          headers: {Headers.contentTypeHeader: ['application/json']},
        );
      }
      return ResponseBody.fromBytes(
        utf8.encode(
          jsonEncode({
            'success': true,
            'data': {
              'access_token': 'new-access',
              'refresh_token': 'new-refresh',
              'expires_at': DateTime.now().add(const Duration(hours: 1)).toIso8601String(),
              'refresh_expires_at':
                  DateTime.now().add(const Duration(days: 30)).toIso8601String(),
            },
          }),
        ),
        200,
        headers: {Headers.contentTypeHeader: ['application/json']},
      );
    }
    if (options.extra['hishumi_retry'] == true) {
      return ResponseBody.fromBytes(
        utf8.encode(jsonEncode({'success': true, 'data': {'ok': true}})),
        200,
        headers: {Headers.contentTypeHeader: ['application/json']},
      );
    }
    return ResponseBody.fromBytes(
      utf8.encode(jsonEncode({'success': false, 'error': {'message': 'unauthorized'}})),
      401,
      headers: {Headers.contentTypeHeader: ['application/json']},
    );
  }

  @override
  void close({bool force = false}) {}
}

Dio _dioWith(
  _FakeStorage storage,
  _RefreshScriptAdapter adapter,
) {
  final dio = Dio(BaseOptions(baseUrl: 'https://example.com'))
    ..httpClientAdapter = adapter
    ..options.validateStatus = (s) => s != null && s < 500 && s != 401;
  final interceptor = AuthInterceptor(localStorage: storage);
  interceptor.attachDio(dio);
  dio.interceptors.add(interceptor);
  return dio;
}

Future<void> _flush([int n = 4]) async {
  for (var i = 0; i < n; i++) {
    await Future<void>.delayed(Duration.zero);
  }
}

void main() {
  setUp(() {
    AuthInterceptor.resetSessionExpiryGuard();
    AuthInterceptor.setSessionExpiredCallbackForTest(null);
  });

  test(
    '401 + existing refresh session + failed refresh → session-expired '
    'signal fires exactly once',
    () async {
      var signalCount = 0;
      final restore = AuthInterceptor.setSessionExpiredCallbackForTest(
        () => signalCount++,
      );
      final storage = _FakeStorage(access: 'old-a', refresh: 'old-r');
      final adapter = _RefreshScriptAdapter(refreshShouldFail: true);
      final dio = _dioWith(storage, adapter);

      try {
        await dio.get<dynamic>('/api/v1/users/me');
        fail('should propagate the 401');
      } on DioException catch (e) {
        expect(e.response?.statusCode, 401);
      }
      await _flush();
      restore();

      expect(adapter.refreshCalls, 1);
      expect(signalCount, 1,
          reason: 'a dead session must drive the canonical signOut via '
              'onSessionExpired — not a silent authenticated shell');
    },
  );

  test('401 + successful refresh → no signal (session healthy)', () async {
    var signalCount = 0;
    final restore = AuthInterceptor.setSessionExpiredCallbackForTest(
      () => signalCount++,
    );
    final storage = _FakeStorage(access: 'old-a', refresh: 'old-r');
    final adapter = _RefreshScriptAdapter(refreshShouldFail: false);
    final dio = _dioWith(storage, adapter);

    final resp = await dio.get<dynamic>('/api/v1/users/me');
    await _flush();
    restore();

    expect(resp.statusCode, 200);
    expect(adapter.refreshCalls, 1);
    expect(signalCount, 0);
  });

  test(
    '401 with NO stored refresh token → no signal (guest contract, '
    'Phase 3B preserved)',
    () async {
      var signalCount = 0;
      final restore = AuthInterceptor.setSessionExpiredCallbackForTest(
        () => signalCount++,
      );
      final storage = _FakeStorage();
      final adapter = _RefreshScriptAdapter(refreshShouldFail: true);
      final dio = _dioWith(storage, adapter);

      try {
        await dio.get<dynamic>('/api/v1/users/me');
      } on DioException catch (_) {}
      await _flush();
      restore();

      expect(adapter.refreshCalls, 0,
          reason: 'no session → no refresh attempt');
      expect(signalCount, 0,
          reason: 'an anonymous 401 has nothing to expire');
    },
  );

  test('non-401 errors never signal', () async {
    var signalCount = 0;
    final restore = AuthInterceptor.setSessionExpiredCallbackForTest(
      () => signalCount++,
    );
    final storage = _FakeStorage(access: 'a', refresh: 'r');
    final dio = Dio(BaseOptions(baseUrl: 'https://example.com'))
      // 500 is an error for Dio but not a 401 → no auth handling.
      ..httpClientAdapter = _Always500Adapter()
      ..options.validateStatus = (s) => s != null && s < 400;
    dio.interceptors.add(AuthInterceptor(localStorage: storage));

    try {
      await dio.get<dynamic>('/api/v1/feed');
    } on DioException catch (_) {}
    await _flush();
    restore();

    expect(signalCount, 0);
  });
}

class _Always500Adapter implements HttpClientAdapter {
  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async => ResponseBody.fromBytes(
        utf8.encode(jsonEncode({'error': 'server'})),
        500,
        headers: {Headers.contentTypeHeader: ['application/json']},
      );

  @override
  void close({bool force = false}) {}
}
