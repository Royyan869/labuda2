import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/core/api/interceptors/auth_interceptor.dart';

class _CaptureAdapter implements HttpClientAdapter {
  String? lastAuthorizationHeader;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    lastAuthorizationHeader = options.headers['Authorization']?.toString();
    final body = utf8.encode(jsonEncode({'ok': true}));
    return ResponseBody.fromBytes(
      body,
      200,
      headers: {
        Headers.contentTypeHeader: ['application/json'],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

void main() {
  // PUBLIC BROWSE — VIEWER-IDENTITY CONTRACT (convergence):
  // /api/v1/users/:id is a public browse endpoint (unauthenticated users may
  // read public profiles), and the backend group is optional-auth: the HiShumi
  // token travels whenever it exists so viewer-scoped facts resolve for a
  // logged-in viewer. The intercept policy for /users/* paths is:
  //   - /api/v1/users/me               → auth-required  (own profile)
  //   - /api/v1/users/check-username   → auth-required  (now in v1 auth group)
  //   - /api/v1/users/<any-other-id>   → browse (token attached when present,
  //                                       anonymous when no credential)
  test(
    '/api/v1/users/:id browse paths attach the token (viewer identity)',
    () async {
      final adapter = _CaptureAdapter();
      final dio = Dio()..httpClientAdapter = adapter;
      dio.interceptors.add(
        AuthInterceptor(hishumiTokenFetcher: () async => 'fresh-token'),
      );

      // Any user-ID path (including former "trending") → identity travels
      await dio.get<dynamic>('/api/v1/users/trending');
      expect(
        adapter.lastAuthorizationHeader,
        equals('Bearer fresh-token'),
        reason:
            'browse GET must carry the viewer identity so viewer-scoped facts resolve',
      );

      await dio.get<dynamic>('/api/v1/users/some-user-uuid');
      expect(adapter.lastAuthorizationHeader, equals('Bearer fresh-token'));
    },
  );

  test(
    '/api/v1/users/check-username is now auth-required (moved to v1 group)',
    () async {
      final adapter = _CaptureAdapter();
      final dio = Dio()..httpClientAdapter = adapter;
      dio.interceptors.add(
        AuthInterceptor(hishumiTokenFetcher: () async => 'fresh-token'),
      );

    // check-username is explicitly auth-required (v1 auth group)
    await dio.get<dynamic>('/api/v1/users/check-username');
      expect(
        adapter.lastAuthorizationHeader,
        equals('Bearer fresh-token'),
        reason:
            'check-username is now in the auth-required v1 group on the backend',
      );
    },
  );

  test('/api/v1/users/me remains auth-required', () async {
    final adapter = _CaptureAdapter();
    final dio = Dio()..httpClientAdapter = adapter;
    dio.interceptors.add(
      AuthInterceptor(hishumiTokenFetcher: () async => 'fresh-token'),
    );

    await dio.get<dynamic>('/api/v1/users/me');
    expect(
      adapter.lastAuthorizationHeader,
      equals('Bearer fresh-token'),
      reason: '/api/v1/users/me must never be treated as a public browse route',
    );
  });
}
