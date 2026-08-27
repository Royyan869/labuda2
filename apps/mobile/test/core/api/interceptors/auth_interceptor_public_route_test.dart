import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/api/interceptors/auth_interceptor.dart';

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
  // PUBLIC BROWSE OPTION C — contract update (2026-07-01):
  // /api/v1/users/:id is now a public browse endpoint so that unauthenticated
  // users can view public profiles. The intercept policy for /users/* paths is:
  //   - /api/v1/users/me               → auth-required  (own profile)
  //   - /api/v1/users/check-username   → auth-required  (now in v1 auth group)
  //   - /api/v1/users/<any-other-id>   → public browse  (no token sent)
  test(
    '/api/v1/users/:id paths are now public browse (no token attached)',
    () async {
      final adapter = _CaptureAdapter();
      final dio = Dio()..httpClientAdapter = adapter;
      dio.interceptors.add(
        AuthInterceptor(tokenFetcher: (_) async => 'fresh-token'),
      );

      // Any user-ID path (including former "trending") → public, no token
      await dio.get<dynamic>('/api/v1/users/trending');
      expect(
        adapter.lastAuthorizationHeader,
        isNull,
        reason:
            '/api/v1/users/:id is a public browse endpoint; no token should be sent',
      );

      await dio.get<dynamic>('/api/v1/users/some-user-uuid');
      expect(adapter.lastAuthorizationHeader, isNull);
    },
  );

  test(
    '/api/v1/users/check-username is now auth-required (moved to v1 group)',
    () async {
      final adapter = _CaptureAdapter();
      final dio = Dio()..httpClientAdapter = adapter;
      dio.interceptors.add(
        AuthInterceptor(tokenFetcher: (_) async => 'fresh-token'),
      );

      // check-username is explicitly excluded from the public browse list
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
      AuthInterceptor(tokenFetcher: (_) async => 'fresh-token'),
    );

    await dio.get<dynamic>('/api/v1/users/me');
    expect(
      adapter.lastAuthorizationHeader,
      equals('Bearer fresh-token'),
      reason: '/api/v1/users/me must never be treated as a public browse route',
    );
  });
}
