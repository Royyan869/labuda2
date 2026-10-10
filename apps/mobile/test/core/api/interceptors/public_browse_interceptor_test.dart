// Public browse interceptor contract tests.
//
// CANONICAL VIEWER-IDENTITY CONTRACT: the HiShumi access token is attached to
// every request, including the public browse GETs. The backend browse group
// is optional-auth (no header → anonymous, valid header → authenticated
// viewer with full context), and viewer-scoped blocks (viewer_capabilities)
// only resolve when the identity travels. A guest has no token, so it stays
// anonymous.
//
// Tests 1-10 prove token attachment per method; test 11 proves a 401 without
// a credential never signals session-expired; test 12 covers the search
// surface.

import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/core/api/interceptors/auth_interceptor.dart';

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
    AuthInterceptor(hishumiTokenFetcher: () async => 'stub-token'),
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
  // 1. GET /api/v1/for-sale → viewer-scoped browse (token attached)
  // ------------------------------------------------------------------
  test(
    '1. GET /api/v1/for-sale attaches the HiShumi token (viewer identity)',
    () async {
      final adapter = _CaptureAdapter();
      final dio = _buildDio(adapter);

      await dio.get<dynamic>('/api/v1/for-sale');

      expect(
        adapter.capturedAuth,
        equals('Bearer stub-token'),
        reason:
            'browse GET must carry the viewer identity so viewer_capabilities resolves for a logged-in viewer',
      );
    },
  );

  // ------------------------------------------------------------------
  // 2. POST /api/v1/for-sale remains auth-required
  // ------------------------------------------------------------------
  test(
    '2. POST /api/v1/for-sale is auth-required — Authorization header attached',
    () async {
      final adapter = _CaptureAdapter();
      final dio = _buildDio(adapter);

      await dio.post<dynamic>('/api/v1/for-sale');

      expect(
        adapter.capturedAuth,
        equals('Bearer stub-token'),
        reason:
            'POST /api/v1/for-sale is auth-required; token must be attached',
      );
    },
  );

  // ------------------------------------------------------------------
  // 3. GET /api/v1/for-sale/some-id → viewer-scoped browse
  // ------------------------------------------------------------------
  test('3. GET /api/v1/for-sale/some-id attaches the token', () async {
    final adapter = _CaptureAdapter();
    final dio = _buildDio(adapter);

    await dio.get<dynamic>('/api/v1/for-sale/some-id');

    expect(adapter.capturedAuth, equals('Bearer stub-token'));
  });

  // ------------------------------------------------------------------
  // 4. GET /api/v1/auctions → viewer-scoped browse
  // ------------------------------------------------------------------
  test('4. GET /api/v1/auctions attaches the token', () async {
    final adapter = _CaptureAdapter();
    final dio = _buildDio(adapter);

    await dio.get<dynamic>('/api/v1/auctions');

    expect(adapter.capturedAuth, equals('Bearer stub-token'));
  });

  // ------------------------------------------------------------------
  // 5. POST /api/v1/auctions/:id/bid remains auth-required
  // ------------------------------------------------------------------
  test('5. POST /api/v1/auctions/id/bid is auth-required', () async {
    final adapter = _CaptureAdapter();
    final dio = _buildDio(adapter);

    await dio.post<dynamic>('/api/v1/auctions/some-auction-id/bid');

    expect(adapter.capturedAuth, equals('Bearer stub-token'));
  });

  // ------------------------------------------------------------------
  // 6. GET /api/v1/users/me remains auth-required
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
  // 7. GET /api/v1/users/some-uuid → viewer-scoped browse
  // ------------------------------------------------------------------
  test(
    '7. GET /api/v1/users/some-uuid attaches the token',
    () async {
      final adapter = _CaptureAdapter();
      final dio = _buildDio(adapter);

      await dio.get<dynamic>(
        '/api/v1/users/550e8400-e29b-41d4-a716-446655440000',
      );

      expect(adapter.capturedAuth, equals('Bearer stub-token'));
    },
  );

  // ------------------------------------------------------------------
  // 8. GET /api/v1/feed remains auth-required
  // ------------------------------------------------------------------
  test('8. GET /api/v1/feed is auth-required', () async {
    final adapter = _CaptureAdapter();
    final dio = _buildDio(adapter);

    await dio.get<dynamic>('/api/v1/feed');

    expect(adapter.capturedAuth, equals('Bearer stub-token'));
  });

  // ------------------------------------------------------------------
  // 9. GET /api/v1/contents/some-id → viewer-scoped browse
  // ------------------------------------------------------------------
  test('9. GET /api/v1/contents/some-id attaches the token', () async {
    final adapter = _CaptureAdapter();
    final dio = _buildDio(adapter);

    await dio.get<dynamic>('/api/v1/contents/some-content-id');

    expect(adapter.capturedAuth, equals('Bearer stub-token'));
  });

  // ------------------------------------------------------------------
  // 10. GET /api/v1/users/check-username remains auth-required
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
  // 11. 401 without a credential does NOT trigger session-expired
  // ------------------------------------------------------------------
  test(
    '11. 401 with no credential does NOT trigger session-expired callback',
    () async {
      bool sessionExpiredFired = false;
      final restore = AuthInterceptor.setSessionExpiredCallbackForTest(() {
        sessionExpiredFired = true;
      });

      // Defensive simulation: 401 although no token was attached. An
      // anonymous browse request never receives a 401 from the backend
      // (StrictBrowse: no header → anonymous, pass through).
      final adapter = _FixedStatusAdapter(401);
      final dio = Dio()..httpClientAdapter = adapter;
      dio.options.validateStatus = (_) => true;
      dio.interceptors.add(
        AuthInterceptor(
          // No hishumiTokenFetcher — simulates no HiShumi session (null token path)
          hishumiTokenFetcher: () async => null,
        ),
      );

      await dio.get<dynamic>('/api/v1/for-sale');

      expect(
        sessionExpiredFired,
        isFalse,
        reason:
            '401 without a credential must NOT fire the session-expired callback',
      );

      restore();
    },
  );

  // ------------------------------------------------------------------
  // 12. Search surface carries the viewer identity too
  // ------------------------------------------------------------------
  test('12. GET /api/v1/search/* browse routes attach the token', () async {
    final paths = [
      '/api/v1/search/for-sale',
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
        equals('Bearer stub-token'),
        reason: 'GET $path must carry the viewer identity',
      );
    }
  });

  // ------------------------------------------------------------------
  // 13. GUEST NEGATIVE PROOF: no token → no Authorization header at all
  // ------------------------------------------------------------------
  test('13. guest (no credential) sends NO Authorization header', () async {
    final adapter = _CaptureAdapter();
    final dio = Dio()..httpClientAdapter = adapter;
    dio.options.validateStatus = (_) => true;
    dio.interceptors.add(
      AuthInterceptor(hishumiTokenFetcher: () async => null),
    );

    await dio.get<dynamic>('/api/v1/for-sale/84b4b44d-be1b-4443-9c65-10947f86aa26');

    expect(
      adapter.capturedAuth,
      isNull,
      reason: 'a guest has no credential and must stay anonymous',
    );
  });
}
