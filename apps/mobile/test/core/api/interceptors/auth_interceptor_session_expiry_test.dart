// Runtime Honesty Tier 2 — AuthInterceptor session-expiry signalling.
//
// Verifies the canonical 401 -> token-refresh outcomes:
//   1. 401 + refresh returns a fresh token         → retry attempted once
//                                                     with new Bearer; no
//                                                     session-expired signal.
//   2. 401 + refresh returns null/empty token      → onSessionExpired fires
//                                                     exactly once; request
//                                                     not retried; original
//                                                     401 propagates.
//   3. 401 + refresh throws                        → onSessionExpired fires
//                                                     exactly once.
//   4. Two back-to-back 401-refresh-failures       → onSessionExpired fires
//                                                     exactly once across
//                                                     the burst.
//   5. Successful onRequest token attach after a
//      prior signal                                → guard re-armed; the
//                                                     next 401-refresh-fail
//                                                     fires onSessionExpired
//                                                     again.
//
// Test seams used (added to AuthInterceptor):
//   * `tokenFetcher`  : avoids hitting FirebaseAuth.instance — production
//                       wiring would call `currentUser.getIdToken(force)`.
//   * `requestRetrier`: avoids real HTTP I/O for the retry path —
//                       production wiring uses `Dio().fetch(...)`.
//   * `setSessionExpiredCallbackForTest`: installs and later restores the
//                       process-wide session-expired listener.
//
// We use a tiny `HttpClientAdapter` that always returns 401 so Dio
// raises a `DioException` (default `validateStatus`) and the
// interceptor's `onError` is invoked exactly as in production.

import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:labuda/core/api/interceptors/auth_interceptor.dart';

class _Canned401Adapter implements HttpClientAdapter {
  int fetchCount = 0;
  String? lastAuthorizationHeader;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    fetchCount++;
    lastAuthorizationHeader = options.headers['Authorization']?.toString();
    final body = utf8.encode(jsonEncode({'error': 'unauthorized'}));
    return ResponseBody.fromBytes(
      body,
      401,
      headers: {
        Headers.contentTypeHeader: ['application/json'],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

void main() {
  // The test cases below configure the interceptor's `tokenFetcher` to
  // bypass Firebase. With the fetcher injected, the interceptor's
  // currentUser-null pre-check is skipped (see auth_interceptor.dart
  // `if (_tokenFetcher == null && _firebaseAuth.currentUser == null)`)
  // so we never touch `FirebaseAuth.instance`.

  setUp(() {
    // Reset the static guard before every test.
    AuthInterceptor.resetSessionExpiryGuard();
  });

  test(
    '401 + refresh success: retry is attempted exactly once with the new Bearer',
    () async {
      String? observedRetryAuth;
      var retrierCalls = 0;
      final dio = Dio()..httpClientAdapter = _Canned401Adapter();

      final restore = AuthInterceptor.setSessionExpiredCallbackForTest(() {
        fail('onSessionExpired must NOT fire on a successful token refresh');
      });

      dio.interceptors.add(
        AuthInterceptor(
          tokenFetcher: (forceRefresh) async {
            // The first onRequest pass calls with forceRefresh=false; the
            // 401-retry path calls with forceRefresh=true.
            return forceRefresh ? 'fresh_token_xyz' : 'stale_token';
          },
          requestRetrier: (options) async {
            retrierCalls++;
            observedRetryAuth = options.headers['Authorization']?.toString();
            return Response<dynamic>(
              requestOptions: options,
              statusCode: 200,
              data: {'ok': true},
            );
          },
        ),
      );

      late Response<dynamic> response;
      try {
        response = await dio.get<dynamic>('/some/path');
      } finally {
        restore();
      }

      expect(retrierCalls, equals(1), reason: 'retry should fire exactly once');
      expect(
        observedRetryAuth,
        equals('Bearer fresh_token_xyz'),
        reason: 'retry must carry the FRESHLY refreshed token',
      );
      expect(response.statusCode, equals(200));
      expect((response.data as Map)['ok'], equals(true));
    },
  );

  test(
    '401 + refresh returns null token: onSessionExpired fires once, request NOT retried',
    () async {
      var callbackCount = 0;
      var retrierCalls = 0;
      final dio = Dio()..httpClientAdapter = _Canned401Adapter();

      final restore = AuthInterceptor.setSessionExpiredCallbackForTest(
        () => callbackCount++,
      );

      dio.interceptors.add(
        AuthInterceptor(
          tokenFetcher: (forceRefresh) async {
            // First request: hand back a token so the request fires.
            // On 401-retry (forceRefresh=true): simulate "Firebase says
            // your user has no valid token anymore" by returning null.
            return forceRefresh ? null : 'stale_token';
          },
          requestRetrier: (_) async {
            retrierCalls++;
            return Response<dynamic>(
              requestOptions: RequestOptions(path: '/dummy'),
              statusCode: 200,
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
      expect(callbackCount, equals(1));
      expect(
        retrierCalls,
        equals(0),
        reason: 'request must NOT be retried when refresh returns null',
      );
    },
  );

  test('401 + refresh throws: onSessionExpired fires once', () async {
    var callbackCount = 0;
    final dio = Dio()..httpClientAdapter = _Canned401Adapter();

    final restore = AuthInterceptor.setSessionExpiredCallbackForTest(
      () => callbackCount++,
    );

    dio.interceptors.add(
      AuthInterceptor(
        tokenFetcher: (forceRefresh) async {
          if (forceRefresh) {
            throw StateError('simulated: getIdToken failed');
          }
          return 'stale_token';
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
    expect(callbackCount, equals(1));
  });

  test(
    'two consecutive 401 + refresh failures: callback only fires once (burst guard)',
    () async {
      var callbackCount = 0;
      final dio = Dio()..httpClientAdapter = _Canned401Adapter();

      final restore = AuthInterceptor.setSessionExpiredCallbackForTest(
        () => callbackCount++,
      );

      dio.interceptors.add(
        AuthInterceptor(
          // ALL refreshes fail (return null). No `requestRetrier` so the
          // happy-retry path is never reached.
          tokenFetcher: (_) async => null,
        ),
      );

      // Two parallel requests, each receives 401, each attempts refresh,
      // each refresh returns null → session-expired signal should
      // de-duplicate to a single callback invocation.
      final futures = <Future<void>>[
        dio.get<dynamic>('/path/one').catchError((Object _) {
          return Response<dynamic>(
            requestOptions: RequestOptions(path: '/path/one'),
          );
        }),
        dio.get<dynamic>('/path/two').catchError((Object _) {
          return Response<dynamic>(
            requestOptions: RequestOptions(path: '/path/two'),
          );
        }),
      ];
      await Future.wait(futures);

      restore();

      expect(
        callbackCount,
        equals(1),
        reason:
            'a single session-expired signal must cover the entire 401 burst',
      );
    },
  );

  test('guard is re-armed after a successful onRequest token attach', () async {
    var callbackCount = 0;
    var nextRequestTokenIsFresh = false;
    final dio = Dio()..httpClientAdapter = _Canned401Adapter();

    final restore = AuthInterceptor.setSessionExpiredCallbackForTest(
      () => callbackCount++,
    );

    dio.interceptors.add(
      AuthInterceptor(
        tokenFetcher: (forceRefresh) async {
          if (forceRefresh) {
            // Refresh always fails in this test (returns null); the
            // adapter always returns 401, so the only knob being
            // varied is whether onRequest gets a fresh token, which
            // resets the guard.
            return null;
          }
          // Drives the onRequest token attach. Returning a non-empty
          // token resets the session-expiry guard (the production
          // intent: a fresh successful auth session re-arms the
          // detector).
          return nextRequestTokenIsFresh ? 'fresh_attach_token' : null;
        },
      ),
    );

    // 1) First 401 burst — refresh fails → callback fires once.
    Object? thrown;
    try {
      await dio.get<dynamic>('/one');
    } catch (e) {
      thrown = e;
    }
    expect(thrown, isA<DioException>());
    expect(callbackCount, equals(1));

    // 2) Simulate that a re-login succeeded: the next outgoing request
    // gets a fresh token attached in onRequest, which must reset the
    // guard.
    nextRequestTokenIsFresh = true;
    try {
      await dio.get<dynamic>('/two');
    } catch (_) {
      // Adapter still returns 401, refresh still fails → callback
      // should fire AGAIN because the guard was reset by the
      // successful onRequest attach.
    }

    restore();

    expect(
      callbackCount,
      equals(2),
      reason:
          'callback must re-fire after a fresh successful onRequest '
          'attach has reset the guard',
    );
  });

  test('non-401 errors do NOT signal session expired', () async {
    var callbackCount = 0;
    final dio = Dio()..httpClientAdapter = _Status500Adapter();

    final restore = AuthInterceptor.setSessionExpiredCallbackForTest(
      () => callbackCount++,
    );

    dio.interceptors.add(
      AuthInterceptor(tokenFetcher: (_) async => 'any_token'),
    );

    try {
      await dio.get<dynamic>('/server-error');
    } catch (_) {
      // expected
    }

    restore();

    expect(
      callbackCount,
      equals(0),
      reason: 'session-expired must only be triggered by 401, not by 5xx',
    );
  });
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
      headers: {
        Headers.contentTypeHeader: ['application/json'],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}
