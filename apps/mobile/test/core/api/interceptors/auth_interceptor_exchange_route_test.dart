import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/api/interceptors/auth_interceptor.dart';

class _CaptureAdapter implements HttpClientAdapter {
  String? lastAuth;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    lastAuth = options.headers['Authorization']?.toString();
    return ResponseBody.fromBytes(
      utf8.encode(jsonEncode({'ok': true})),
      200,
      headers: {Headers.contentTypeHeader: ['application/json']},
    );
  }

  @override
  void close({bool force = false}) {}
}

class _OkAdapter implements HttpClientAdapter {
  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    return ResponseBody.fromBytes(
      utf8.encode(jsonEncode({'ok': true})),
      200,
      headers: {Headers.contentTypeHeader: ['application/json']},
    );
  }

  @override
  void close({bool force = false}) {}
}

void main() {
  // Phase 3B: Firebase exchange & complete-profile are skipAuth.
  // No Labuda token should be attached even if a Labuda token exists.
  test('/auth/firebase/exchange with skipAuth:true must NOT attach Labuda token',
      () async {
    final adapter = _CaptureAdapter();
    final dio = Dio()..httpClientAdapter = adapter;
    dio.interceptors.add(
      AuthInterceptor(labudaTokenFetcher: () async => 'labuda-jwt'),
    );

    await dio.post<dynamic>(
      '/api/v1/auth/firebase/exchange',
      data: {'firebase_id_token': 'firebase-token'},
      options: Options(extra: {'skipAuth': true}),
    );

    expect(
      adapter.lastAuth,
      isNull,
      reason:
          'Firebase exchange is skipAuth — AuthInterceptor must not attach Labuda JWT',
    );

    await dio.post<dynamic>(
      '/api/v1/auth/complete-profile',
      data: {'username': 'alice'},
      options: Options(
        extra: {'skipAuth': true},
        headers: {'Authorization': 'Bearer restricted-token'},
      ),
    );

    // The second request also skipAuth, so interceptor should not overwrite
    // the explicitly set restricted token nor add Labuda.
    // Capture adapter shows the header as set by the caller (if any).
    // Since we passed it via Options.headers, it should be preserved.
    // But interceptor skipAuth path does not touch headers, so it stays.
    expect(
      adapter.lastAuth,
      equals('Bearer restricted-token'),
      reason:
          'complete-profile is skipAuth with restricted token — Labuda must NOT overwrite',
    );
  });

  test('normal authenticated request attaches Labuda token, not Firebase', () async {
    final adapter = _CaptureAdapter();
    final dio = Dio()..httpClientAdapter = adapter;
    dio.interceptors.add(
      AuthInterceptor(labudaTokenFetcher: () async => 'labuda-jwt-xyz'),
    );

    await dio.get<dynamic>('/api/v1/users/me');

    expect(adapter.lastAuth, equals('Bearer labuda-jwt-xyz'));
  });

  // Legacy forceRefresh semantics are obsolete: skipAuth routes never
  // trigger Labuda fetcher.
  test('skipAuth routes never invoke Labuda fetcher', () async {
    var fetchCount = 0;
    final dio = Dio()..httpClientAdapter = _OkAdapter();
    dio.interceptors.add(
      AuthInterceptor(labudaTokenFetcher: () async {
        fetchCount++;
        return 'labuda';
      }),
    );

    await dio.post<dynamic>(
      '/api/v1/auth/firebase/exchange',
      options: Options(extra: {'skipAuth': true}),
    );
    await dio.post<dynamic>(
      '/auth/complete-profile',
      options: Options(extra: {'skipAuth': true}),
    );

    expect(fetchCount, equals(0),
        reason: 'skipAuth must bypass Labuda token read entirely');
  });
}
