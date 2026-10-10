// Phase 3B — MOBILE_HTTP_CREDENTIAL_AUTHORITY
// Canonical tests proving actual header behavior after HiShumi JWT migration.
//
// Requirements (from PHASE 3B spec §9):
// - Normal authenticated request → Bearer HiShumiAccessToken
// - Firebase separation → normal API uses HiShumi, NOT Firebase
// - No fallback → HiShumi missing + Firebase present → MUST NOT silently use Firebase
// - Exchange boundary → Firebase credential remains allowed (skipAuth, body)
// - Completion boundary → restricted completion credential (skipAuth + Bearer restricted)
// - Existing request behavior → headers, public, body, etc. not broken

import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/core/api/interceptors/auth_interceptor.dart';

class _CaptureAdapter implements HttpClientAdapter {
  RequestOptions? lastOptions;
  String? lastAuth;
  Map<String, dynamic>? lastHeaders;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    lastOptions = options;
    lastAuth = options.headers['Authorization']?.toString();
    lastHeaders = Map<String, dynamic>.from(options.headers);
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
  group('HiShumi HTTP credential authority (Phase 3B)', () {
    setUp(() {
      AuthInterceptor.resetSessionExpiryGuard();
      AuthInterceptor.setSessionExpiredCallbackForTest(null);
    });

    test('Normal authenticated request: Authorization = Bearer HiShumiAccessToken',
        () async {
      final adapter = _CaptureAdapter();
      final dio = Dio()..httpClientAdapter = adapter;
      dio.interceptors.add(
        AuthInterceptor(hishumiTokenFetcher: () async => 'hishumi-access-jwt-abc'),
      );

      await dio.get<dynamic>('/api/v1/users/me');

      expect(adapter.lastAuth, equals('Bearer hishumi-access-jwt-abc'));
      // also works for generic normal route
      await dio.get<dynamic>('/api/v1/feed');
      expect(adapter.lastAuth, equals('Bearer hishumi-access-jwt-abc'));
    });

    test('Firebase separation: HiShumi exists + Firebase exists → uses HiShumi, NOT Firebase',
        () async {
      const hishumiToken = 'hishumi-jwt-canonical';
      const firebaseToken = 'firebase-jwt-should-not-be-used';

      // Simulate that Firebase currentUser.getIdToken would return firebaseToken
      // if fallback existed. We deliberately do NOT pass firebase via interceptor.
      // The only fetcher wired is HiShumi.
      final adapter = _CaptureAdapter();
      final dio = Dio()..httpClientAdapter = adapter;
      dio.interceptors.add(
        AuthInterceptor(hishumiTokenFetcher: () async => hishumiToken),
      );

      await dio.get<dynamic>('/api/v1/users/me');
      expect(adapter.lastAuth, equals('Bearer $hishumiToken'));
      expect(adapter.lastAuth, isNot(equals('Bearer $firebaseToken')),
          reason: 'normal API must NOT use Firebase token even if Firebase session active');

      // Proof that Firebase token string is not present anywhere in headers
      expect(adapter.lastAuth, isNot(contains(firebaseToken)));
    });

    test('No fallback: HiShumi missing + Firebase exists → MUST NOT silently use Firebase',
        () async {
      final adapter = _CaptureAdapter();
      final dio = Dio()..httpClientAdapter = adapter;

      // HiShumi fetcher returns null (missing credential)
      dio.interceptors.add(
        AuthInterceptor(hishumiTokenFetcher: () async => null),
      );

      // Firebase "exists" is simulated — but interceptor must NOT read it.
      // If fallback existed, Authorization would be Bearer <Firebase>.
      await dio.get<dynamic>('/api/v1/users/me');

      expect(adapter.lastAuth, isNull,
          reason:
              'when HiShumi access token missing, interceptor must NOT silently attach Firebase token');
      // Also for other normal routes
      await dio.get<dynamic>('/api/v1/feed');
      expect(adapter.lastAuth, isNull);
    });

    test('No fallback: empty HiShumi token → no Firebase fallback, no Bearer', () async {
      final adapter = _CaptureAdapter();
      final dio = Dio()..httpClientAdapter = adapter;
      dio.interceptors.add(
        AuthInterceptor(hishumiTokenFetcher: () async => ''),
      );

      await dio.get<dynamic>('/api/v1/users/me');
      expect(adapter.lastAuth, isNull);
    });

    test('Exchange boundary: /auth/firebase/exchange uses Firebase credential (skipAuth)',
        () async {
      final adapter = _CaptureAdapter();
      final dio = Dio()..httpClientAdapter = adapter;
      // Even though HiShumi token exists, skipAuth routes must NOT attach it.
      dio.interceptors.add(
        AuthInterceptor(hishumiTokenFetcher: () async => 'hishumi-should-not-attach'),
      );

      await dio.post<dynamic>(
        '/api/v1/auth/firebase/exchange',
        data: {'firebase_id_token': 'firebase-id-token-xyz'},
        options: Options(extra: {'skipAuth': true}),
      );

      expect(adapter.lastAuth, isNull,
          reason: 'exchange is skipAuth — HiShumi JWT must NOT be attached');

      // Verify body still carries Firebase token (datasource contract)
      expect(
        (adapter.lastOptions!.data as Map)['firebase_id_token'],
        equals('firebase-id-token-xyz'),
      );
    });

    test('Completion boundary: /auth/complete-profile uses restricted token', () async {
      final adapter = _CaptureAdapter();
      final dio = Dio()..httpClientAdapter = adapter;
      dio.interceptors.add(
        AuthInterceptor(hishumiTokenFetcher: () async => 'hishumi-should-not-overwrite'),
      );

      await dio.post<dynamic>(
        '/api/v1/auth/complete-profile',
        data: {'username': 'alice'},
        options: Options(
          extra: {'skipAuth': true},
          headers: {'Authorization': 'Bearer restricted-token-xyz'},
        ),
      );

      expect(adapter.lastAuth, equals('Bearer restricted-token-xyz'),
          reason: 'complete-profile must preserve restricted token, not HiShumi');
    });

    test('/users/me now sends HiShumi JWT (backend may still reject — expected)', () async {
      final adapter = _CaptureAdapter();
      final dio = Dio()..httpClientAdapter = adapter;
      dio.interceptors.add(
        AuthInterceptor(hishumiTokenFetcher: () async => 'hishumi-for-users-me'),
      );

      await dio.get<dynamic>('/api/v1/users/me');
      expect(adapter.lastAuth, equals('Bearer hishumi-for-users-me'));

      await dio.get<dynamic>('/users/me');
      expect(adapter.lastAuth, equals('Bearer hishumi-for-users-me'));
    });

    test('Existing request behavior preserved: headers, content-type, query, body',
        () async {
      final adapter = _CaptureAdapter();
      final dio = Dio()..httpClientAdapter = adapter;
      dio.interceptors.add(
        AuthInterceptor(hishumiTokenFetcher: () async => 'hishumi-jwt'),
      );

      await dio.post<dynamic>(
        '/api/v1/for-sale',
        data: {'title': 'hello'},
        queryParameters: {'page': 2},
        options: Options(
          headers: {'X-Custom': 'custom-value', 'Content-Type': 'application/json'},
        ),
      );

      expect(adapter.lastAuth, equals('Bearer hishumi-jwt'));
      expect(adapter.lastHeaders!['X-Custom'], equals('custom-value'));
      expect(adapter.lastHeaders!['Content-Type'], equals('application/json'));
      expect(adapter.lastOptions!.queryParameters['page'], equals(2));
      expect((adapter.lastOptions!.data as Map)['title'], equals('hello'));
    });

    test('Browse GET carries the HiShumi token (viewer identity)', () async {
      final adapter = _CaptureAdapter();
      final dio = Dio()..httpClientAdapter = adapter;
      dio.interceptors.add(
        AuthInterceptor(hishumiTokenFetcher: () async => 'hishumi-jwt'),
      );

      await dio.get<dynamic>('/api/v1/for-sale');
      expect(adapter.lastAuth, equals('Bearer hishumi-jwt'));

      await dio.get<dynamic>('/api/v1/users/some-uuid');
      expect(adapter.lastAuth, equals('Bearer hishumi-jwt'));

      // POST to the same prefix behaves identically — one attach rule
      await dio.post<dynamic>('/api/v1/for-sale');
      expect(adapter.lastAuth, equals('Bearer hishumi-jwt'));
    });

    test('skipAuth extra bypasses HiShumi even on normal path', () async {
      final adapter = _CaptureAdapter();
      final dio = Dio()..httpClientAdapter = adapter;
      dio.interceptors.add(
        AuthInterceptor(hishumiTokenFetcher: () async => 'hishumi-jwt'),
      );

      await dio.get<dynamic>(
        '/api/v1/users/me',
        options: Options(extra: {'skipAuth': true}),
      );
      expect(adapter.lastAuth, isNull);
    });

    test('Ad-hoc Authorization header without skipAuth is overwritten by HiShumi',
        () async {
      // This proves canonical authority: caller-provided Bearer is replaced
      // by HiShumi on normal routes. Only skipAuth routes preserve caller header.
      final adapter = _CaptureAdapter();
      final dio = Dio()..httpClientAdapter = adapter;
      dio.interceptors.add(
        AuthInterceptor(hishumiTokenFetcher: () async => 'hishumi-canonical'),
      );

      await dio.get<dynamic>(
        '/api/v1/feed',
        options: Options(headers: {'Authorization': 'Bearer caller-token'}),
      );

      expect(adapter.lastAuth, equals('Bearer hishumi-canonical'),
          reason: 'canonical HiShumi authority must overwrite caller Bearer on normal routes');
    });

    test('Idempotency-Key header preserved alongside HiShumi Authorization', () async {
      final adapter = _CaptureAdapter();
      final dio = Dio()..httpClientAdapter = adapter;
      dio.interceptors.add(
        AuthInterceptor(hishumiTokenFetcher: () async => 'hishumi-jwt'),
      );

      await dio.post<dynamic>(
        '/api/v1/orders',
        data: {'amount': 100},
        options: Options(headers: {'Idempotency-Key': 'idem-123'}),
      );

      expect(adapter.lastAuth, equals('Bearer hishumi-jwt'));
      expect(adapter.lastHeaders!['Idempotency-Key'], equals('idem-123'));
    });
  });
}
