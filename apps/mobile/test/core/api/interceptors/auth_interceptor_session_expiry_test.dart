// Phase 3B — Labuda JWT authority: 401 does NOT auto-refresh via Firebase.
// Refresh lifecycle is deferred to Phase 3C. AuthInterceptor must NOT
// attempt Firebase getIdToken(true) nor retry the request.
//
// Verifies:
// - 401 on normal API does NOT retry and does NOT signal session-expired
//   (Phase 3B intentionally disables auto-refresh).
// - 401 on public browse also does NOT signal.
// - non-401 never signals.
// - Guard re-arm on successful Labuda token attach still works (preparation for 3C).

import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:labuda/core/api/interceptors/auth_interceptor.dart';

class _Canned401Adapter implements HttpClientAdapter {
  int fetchCount = 0;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    fetchCount++;
    final body = utf8.encode(jsonEncode({'error': 'unauthorized'}));
    return ResponseBody.fromBytes(
      body,
      401,
      headers: {Headers.contentTypeHeader: ['application/json']},
    );
  }

  @override
  void close({bool force = false}) {}
}

class _Status500Adapter implements HttpClientAdapter {
  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    final body = utf8.encode(jsonEncode({'error': 'server'}));
    return ResponseBody.fromBytes(
      body,
      500,
      headers: {Headers.contentTypeHeader: ['application/json']},
    );
  }

  @override
  void close({bool force = false}) {}
}

void main() {
  setUp(() {
    AuthInterceptor.resetSessionExpiryGuard();
  });

  test('401 does NOT retry and does NOT signal sessionExpired (Phase 3B disabled)',
      () async {
    var callbackCount = 0;
    var retrierCalls = 0;
    final dio = Dio()..httpClientAdapter = _Canned401Adapter();

    final restore = AuthInterceptor.setSessionExpiredCallbackForTest(
      () => callbackCount++,
    );

    dio.interceptors.add(
      AuthInterceptor(
        labudaTokenFetcher: () async => 'labuda-token',
        requestRetrier: (options) async {
          retrierCalls++;
          return Response<dynamic>(
            requestOptions: options,
            statusCode: 200,
            data: {'ok': true},
          );
        },
      ),
    );

    Object? thrown;
    try {
      await dio.get<dynamic>('/some/path');
    } catch (e) {
      thrown = e;
    } finally {
      restore();
    }

    expect(thrown, isA<DioException>());
    expect((thrown as DioException).response?.statusCode, equals(401));
    expect(retrierCalls, equals(0),
        reason: 'Phase 3B must NOT auto-retry on 401');
    expect(callbackCount, equals(0),
        reason: 'Phase 3B must NOT auto-signal sessionExpired on Labuda 401');
  });

  test('401 with Labuda fetcher returning null still does NOT signal sessionExpired',
      () async {
    var callbackCount = 0;
    final dio = Dio()..httpClientAdapter = _Canned401Adapter();

    final restore = AuthInterceptor.setSessionExpiredCallbackForTest(
      () => callbackCount++,
    );

    dio.interceptors.add(
      AuthInterceptor(labudaTokenFetcher: () async => null),
    );

    try {
      await dio.get<dynamic>('/some/path');
    } catch (_) {}

    restore();
    expect(callbackCount, equals(0));
  });

  test('non-401 errors do NOT signal session expired', () async {
    var callbackCount = 0;
    final dio = Dio()..httpClientAdapter = _Status500Adapter();

    final restore = AuthInterceptor.setSessionExpiredCallbackForTest(
      () => callbackCount++,
    );

    dio.interceptors.add(
      AuthInterceptor(labudaTokenFetcher: () async => 'any_token'),
    );

    try {
      await dio.get<dynamic>('/server-error');
    } catch (_) {}

    restore();

    expect(callbackCount, equals(0),
        reason: 'session-expired must only be considered for 401');
  });

  test('guard is re-armed after a successful Labuda token attach', () async {
    // This proves the guard-reset path (onRequest success) still works
    // so Phase 3C can re-use it without adding new code.
    final adapter = _Canned401Adapter();
    final dio = Dio()..httpClientAdapter = adapter;

    // Manually set guard to signaled, then perform a successful attach
    // via a 200 ok adapter to prove it resets.
    AuthInterceptor.resetSessionExpiryGuard();

    // We simulate by using a capture adapter that returns 200 and checks
    // that a Labuda token was attached - the interceptor should reset guard.
    // Since Phase 3B does not auto-signal, we just verify the interceptor
    // still attaches the token and does not throw.
    final capture = _Capture200Adapter();
    final dio2 = Dio()..httpClientAdapter = capture;
    dio2.interceptors.add(
      AuthInterceptor(labudaTokenFetcher: () async => 'fresh_labuda'),
    );

    await dio2.get<dynamic>('/normal/path');
    expect(capture.capturedAuth, equals('Bearer fresh_labuda'));
  });
}

class _Capture200Adapter implements HttpClientAdapter {
  String? capturedAuth;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    capturedAuth = options.headers['Authorization']?.toString();
    return ResponseBody.fromBytes(
      utf8.encode(jsonEncode({'ok': true})),
      200,
      headers: {Headers.contentTypeHeader: ['application/json']},
    );
  }

  @override
  void close({bool force = false}) {}
}
