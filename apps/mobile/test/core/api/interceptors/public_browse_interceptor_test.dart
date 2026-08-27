// Public browse interceptor contract tests.
//
// Verifies that _isPublicEndpoint correctly classifies the browse surface and
// that a 401 on a public endpoint does NOT trigger the session-expired signal.
//
// Tests 1-10 cover endpoint classification (GET vs POST method-awareness and
// path patterns).  Test 11 proves the 401-on-public-endpoint guard.

import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/api/interceptors/auth_interceptor.dart';

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/// An [HttpClientAdapter] that always returns the configured [statusCode].
class _FixedStatusAdapter implements HttpClientAdapter {
  _FixedStatusAdapter(this.statusCode);
  final int statusCode;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    final body = utf8.encode(jsonEncode({'ok': statusCode == 200}));
    return ResponseBody.fromBytes(
      body,
      statusCode,
      headers: {
        Headers.contentTypeHeader: ['application/json'],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

/// A [HttpClientAdapter] that captures the Authorization header from the
/// outgoing request and always returns 200.
class _CaptureAdapter implements HttpClientAdapter {
  String? capturedAuth;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    capturedAuth = options.headers['Authorization']?.toString();
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

/// Builds a [Dio] instance wired with [AuthInterceptor] using a stub token
/// fetcher so tests never touch Firebase.
Dio _buildDio(HttpClientAdapter adapter) {
  final dio = Dio()..httpClientAdapter = adapter;
  dio.options.validateStatus = (_) => true; // don't throw on 4xx/5xx
  dio.interceptors.add(
    AuthInterceptor(tokenFetcher: (_) async => 'stub-token'),
  );
  return dio;
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

void main() {
  // Reset static state before each test so tests are independent.
  setUp(() {
    AuthInterceptor.setSessionExpiredCallbackForTest(null);
  });

  // ------------------------------------------------------------------
  // 1. GET /api/v1/listings → public (no token attached to request)
  // ------------------------------------------------------------------
  test(
    '1. GET /api/v1/listings is public — no Authorization header attached',
    () async {
      final adapter = _CaptureAdapter();
      final dio = _buildDio(adapter);

      await dio.get<dynamic>('/api/v1/listings');

      expect(
        adapter.capturedAuth,
        isNull,
        reason:
            'GET /api/v1/listings is a public browse endpoint; no token should be sent',
      );
    },
  );

  // ------------------------------------------------------------------
  // 2. POST /api/v1/listings is NOT public
  // ------------------------------------------------------------------
  test(
    '2. POST /api/v1/listings is auth-required — Authorization header attached',
    () async {
      final adapter = _CaptureAdapter();
      final dio = _buildDio(adapter);

      await dio.post<dynamic>('/api/v1/listings');

      expect(
        adapter.capturedAuth,
        equals('Bearer stub-token'),
        reason:
            'POST /api/v1/listings is auth-required; token must be attached',
      );
    },
  );

  // ------------------------------------------------------------------
  // 3. GET /api/v1/listings/some-id → public
  // ------------------------------------------------------------------
  test('3. GET /api/v1/listings/some-id is public', () async {
    final adapter = _CaptureAdapter();
    final dio = _buildDio(adapter);

    await dio.get<dynamic>('/api/v1/listings/some-id');

    expect(adapter.capturedAuth, isNull);
  });

  // ------------------------------------------------------------------
  // 4. GET /api/v1/auctions → public
  // ------------------------------------------------------------------
  test('4. GET /api/v1/auctions is public', () async {
    final adapter = _CaptureAdapter();
    final dio = _buildDio(adapter);

    await dio.get<dynamic>('/api/v1/auctions');

    expect(adapter.capturedAuth, isNull);
  });

  // ------------------------------------------------------------------
  // 5. POST /api/v1/auctions/:id/bid is NOT public
  // ------------------------------------------------------------------
  test('5. POST /api/v1/auctions/id/bid is auth-required', () async {
    final adapter = _CaptureAdapter();
    final dio = _buildDio(adapter);

    await dio.post<dynamic>('/api/v1/auctions/some-auction-id/bid');

    expect(adapter.capturedAuth, equals('Bearer stub-token'));
  });

  // ------------------------------------------------------------------
  // 6. GET /api/v1/users/me is NOT public
  // ------------------------------------------------------------------
  test('6. GET /api/v1/users/me is auth-required — token attached', () async {
    final adapter = _CaptureAdapter();
    final dio = _buildDio(adapter);

    await dio.get<dynamic>('/api/v1/users/me');

    expect(
      adapter.capturedAuth,
      equals('Bearer stub-token'),
      reason: '/users/me must never be treated as a public browse route',
    );
  });

  // ------------------------------------------------------------------
  // 7. GET /api/v1/users/some-uuid → public
  // ------------------------------------------------------------------
  test(
    '7. GET /api/v1/users/some-uuid is public — no token attached',
    () async {
      final adapter = _CaptureAdapter();
      final dio = _buildDio(adapter);

      await dio.get<dynamic>(
        '/api/v1/users/550e8400-e29b-41d4-a716-446655440000',
      );

      expect(adapter.capturedAuth, isNull);
    },
  );

  // ------------------------------------------------------------------
  // 8. GET /api/v1/feed is NOT public
  // ------------------------------------------------------------------
  test('8. GET /api/v1/feed is auth-required', () async {
    final adapter = _CaptureAdapter();
    final dio = _buildDio(adapter);

    await dio.get<dynamic>('/api/v1/feed');

    expect(adapter.capturedAuth, equals('Bearer stub-token'));
  });

  // ------------------------------------------------------------------
  // 9. GET /api/v1/contents/some-id → public
  // ------------------------------------------------------------------
  test('9. GET /api/v1/contents/some-id is public', () async {
    final adapter = _CaptureAdapter();
    final dio = _buildDio(adapter);

    await dio.get<dynamic>('/api/v1/contents/some-content-id');

    expect(adapter.capturedAuth, isNull);
  });

  // ------------------------------------------------------------------
  // 10. GET /api/v1/users/check-username is NOT public (moved to auth-required v1 group)
  // ------------------------------------------------------------------
  test('10. GET /api/v1/users/check-username is auth-required', () async {
    final adapter = _CaptureAdapter();
    final dio = _buildDio(adapter);

    await dio.get<dynamic>('/api/v1/users/check-username');

    expect(
      adapter.capturedAuth,
      equals('Bearer stub-token'),
      reason:
          'check-username is now in the auth-required v1 group on the backend',
    );
  });

  // ------------------------------------------------------------------
  // 11. 401 on public browse endpoint does NOT trigger session-expired
  // ------------------------------------------------------------------
  test(
    '11. 401 on public browse endpoint does NOT trigger session-expired callback',
    () async {
      bool sessionExpiredFired = false;
      final restore = AuthInterceptor.setSessionExpiredCallbackForTest(() {
        sessionExpiredFired = true;
      });

      // Backend returns 401 (StrictBrowseAuthMiddleware rejected malformed token)
      final adapter = _FixedStatusAdapter(401);
      final dio = Dio()..httpClientAdapter = adapter;
      dio.options.validateStatus = (_) => true;
      dio.interceptors.add(
        AuthInterceptor(
          // No tokenFetcher — simulates no Firebase user (null token path)
          tokenFetcher: (_) async => null,
        ),
      );

      await dio.get<dynamic>('/api/v1/listings');

      expect(
        sessionExpiredFired,
        isFalse,
        reason:
            '401 on a public browse endpoint must NOT fire the session-expired callback',
      );

      restore();
    },
  );

  // ------------------------------------------------------------------
  // Extra: GET /api/v1/search/listings, /search/auctions, /search/content, /search/users → public
  // ------------------------------------------------------------------
  test('12. GET /api/v1/search/* browse routes are public', () async {
    final paths = [
      '/api/v1/search/listings',
      '/api/v1/search/auctions',
      '/api/v1/search/content',
      '/api/v1/search/users',
      '/api/v1/likes/stats',
    ];

    for (final path in paths) {
      final adapter = _CaptureAdapter();
      final dio = _buildDio(adapter);
      await dio.get<dynamic>(path);
      expect(
        adapter.capturedAuth,
        isNull,
        reason: 'GET $path should be public (no token)',
      );
    }
  });
}
