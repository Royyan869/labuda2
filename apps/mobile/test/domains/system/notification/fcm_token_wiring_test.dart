// Runtime Honesty Tier 3 — FCM token registration / deregistration wiring.
//
// Verifies that the Tier-3 contract holds at the unit level:
//   1. `NotificationRemoteDatasource.saveUserToken` reaches the
//      canonical `NotificationApiDatasource.registerFCMToken`
//      (POST /notifications/fcm-token) with the FCM token also acting
//      as the stable `deviceId`.
//   2. `NotificationRemoteDatasource.deleteUserToken` reaches the
//      canonical `NotificationApiDatasource.removeFCMToken`
//      (DELETE /notifications/fcm-token?device_id=...) with the same
//      FCM token as the precise key.
//   3. `FCMTokenManager.initializeToken` retries exactly once on a
//      transient `getToken()` failure — never loops indefinitely.
//   4. `FCMTokenManager.deleteToken` completes successfully even when
//      backend deregistration throws, so logout always completes.
//
// To avoid depending on Firebase test infrastructure, the tests:
//   * Subclass `NotificationApiDatasource` with `noSuchMethod` so we
//     never call its real (Dio-driven) implementation. Only
//     `registerFCMToken` / `removeFCMToken` are overridden.
//   * Inject the `tokenFetcher` / `tokenDeleter` / `tokenRefreshStreamProvider`
//     seams on `FCMTokenManager` to drive FirebaseMessaging-shaped
//     calls without touching the real FCM SDK.

import 'dart:async';

import 'package:dio/dio.dart' show Response, RequestOptions, DioException;
import 'package:flutter_test/flutter_test.dart';

import 'package:labuda/core/api/api_client.dart';
import 'package:labuda/domains/system/notification/data/datasources/notification_api_datasource.dart';
import 'package:labuda/domains/system/notification/data/datasources/notification_remote_datasource.dart';
import 'package:labuda/domains/system/notification/data/models/api/notification_api_models.dart';
import 'package:labuda/domains/system/notification/services/fcm_token_manager.dart';

// ---------------------------------------------------------------------------
// Test doubles
// ---------------------------------------------------------------------------

/// `implements ApiClient` with `noSuchMethod` so subclassing
/// [NotificationApiDatasource] doesn't blow up booting Firebase via
/// the real ApiClient constructor. None of the recorded methods touch
/// the underlying ApiClient — they are overridden below.
class _NoopApiClient implements ApiClient {
  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

/// Records the canonical-API calls made by `NotificationRemoteDatasource`.
class _RecordingApiDatasource extends NotificationApiDatasource {
  _RecordingApiDatasource() : super(_NoopApiClient());

  final List<RegisterFCMTokenRequest> registerCalls = [];
  final List<String> removeCalls = [];

  /// If non-null, [registerFCMToken] throws this error.
  Object? registerThrows;

  /// If non-null, [removeFCMToken] throws this error.
  Object? removeThrows;

  @override
  Future<FCMTokenResponse> registerFCMToken(
    RegisterFCMTokenRequest request,
  ) async {
    registerCalls.add(request);
    if (registerThrows != null) {
      throw registerThrows!;
    }
    return FCMTokenResponse(
      id: 'fcm_id_${registerCalls.length}',
      userId: 'unused',
      token: request.token,
      platform: request.platform,
      deviceId: request.deviceId,
      isActive: true,
      createdAt: DateTime.utc(2026, 5, 16),
      updatedAt: DateTime.utc(2026, 5, 16),
    );
  }

  @override
  Future<void> removeFCMToken(String deviceId) async {
    removeCalls.add(deviceId);
    if (removeThrows != null) {
      throw removeThrows!;
    }
  }
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

void main() {
  group('NotificationRemoteDatasource — canonical wiring', () {
    test('saveUserToken reaches NotificationApiDatasource.registerFCMToken '
        'with the FCM token reused as deviceId', () async {
      final api = _RecordingApiDatasource();
      final datasource = NotificationRemoteDatasource(apiDatasource: api);

      await datasource.saveUserToken(userId: 'user_42', token: 'fcm_token_abc');

      expect(api.registerCalls, hasLength(1));
      final call = api.registerCalls.single;
      expect(call.token, equals('fcm_token_abc'));
      expect(
        call.deviceId,
        equals('fcm_token_abc'),
        reason:
            'FCM token must double as the stable deviceId so a later '
            'removeFCMToken can target the exact same record',
      );
      expect(
        call.platform,
        anyOf(equals('ios'), equals('android'), equals('web')),
        reason: 'platform must resolve via platformDetector',
      );
    });

    test('deleteUserToken reaches NotificationApiDatasource.removeFCMToken '
        'with the FCM token as the deviceId key', () async {
      final api = _RecordingApiDatasource();
      final datasource = NotificationRemoteDatasource(apiDatasource: api);

      await datasource.deleteUserToken(
        userId: 'user_42',
        token: 'fcm_token_abc',
      );

      expect(api.removeCalls, equals(['fcm_token_abc']));
    });

    test('saveUserToken rethrows backend failure so caller can apply '
        'retry/backoff policy', () async {
      final api = _RecordingApiDatasource()
        ..registerThrows = DioException(
          requestOptions: RequestOptions(path: '/notifications/fcm-token'),
          response: Response<dynamic>(
            requestOptions: RequestOptions(path: '/notifications/fcm-token'),
            statusCode: 500,
          ),
        );
      final datasource = NotificationRemoteDatasource(apiDatasource: api);

      Object? thrown;
      try {
        await datasource.saveUserToken(userId: 'u', token: 't');
      } catch (e) {
        thrown = e;
      }

      expect(thrown, isA<DioException>());
      expect(api.registerCalls, hasLength(1));
    });

    test('saveUserToken/deleteUserToken degrade gracefully (no throw) when '
        'no apiDatasource is wired — legacy bootstrap path', () async {
      // No apiDatasource — emulates the pre-Tier-3 wiring.
      final datasource = NotificationRemoteDatasource();

      // Must NOT throw — we want a graceful warning + no-op, not a crash.
      await datasource.saveUserToken(userId: 'u', token: 't');
      await datasource.deleteUserToken(userId: 'u', token: 't');
    });
  });

  group('FCMTokenManager — bounded retry on getToken', () {
    test('getToken returns null twice → returns null after exactly 2 attempts '
        '(no infinite loop)', () async {
      final api = _RecordingApiDatasource();
      final datasource = NotificationRemoteDatasource(apiDatasource: api);

      var getTokenCalls = 0;
      final manager = FCMTokenManager(
        datasource: datasource,
        tokenFetcher: () async {
          getTokenCalls++;
          return null; // Always null → simulates persistent failure.
        },
        tokenRefreshStreamProvider: () => const Stream<String>.empty(),
      );

      final result = await manager.initializeToken('user_1');

      expect(result, isNull);
      expect(
        getTokenCalls,
        equals(2),
        reason:
            'getToken should be attempted exactly 2 times (initial + 1 retry)',
      );
      expect(
        api.registerCalls,
        isEmpty,
        reason: 'no backend registration when device-side token is null',
      );
    });

    test('getToken throws on first call, succeeds on second → token is '
        'registered exactly once (the retry succeeded)', () async {
      final api = _RecordingApiDatasource();
      final datasource = NotificationRemoteDatasource(apiDatasource: api);

      var getTokenCalls = 0;
      final manager = FCMTokenManager(
        datasource: datasource,
        tokenFetcher: () async {
          getTokenCalls++;
          if (getTokenCalls == 1) {
            throw StateError('simulated transient FCM failure');
          }
          return 'recovered_token';
        },
        tokenRefreshStreamProvider: () => const Stream<String>.empty(),
      );

      final result = await manager.initializeToken('user_1');

      expect(result, equals('recovered_token'));
      expect(getTokenCalls, equals(2));
      expect(api.registerCalls, hasLength(1));
      expect(api.registerCalls.single.token, equals('recovered_token'));
    });

    test(
      'getToken succeeds on first call → no retry, single backend call',
      () async {
        final api = _RecordingApiDatasource();
        final datasource = NotificationRemoteDatasource(apiDatasource: api);

        var getTokenCalls = 0;
        final manager = FCMTokenManager(
          datasource: datasource,
          tokenFetcher: () async {
            getTokenCalls++;
            return 'fresh_token';
          },
          tokenRefreshStreamProvider: () => const Stream<String>.empty(),
        );

        final result = await manager.initializeToken('user_1');

        expect(result, equals('fresh_token'));
        expect(getTokenCalls, equals(1));
        expect(api.registerCalls, hasLength(1));
      },
    );
  });

  group('FCMTokenManager — logout tolerance', () {
    test('deleteToken completes even when backend deregistration throws '
        '(logout must always succeed)', () async {
      final api = _RecordingApiDatasource();
      final datasource = NotificationRemoteDatasource(apiDatasource: api);

      final manager = FCMTokenManager(
        datasource: datasource,
        tokenFetcher: () async => 'registered_token',
        tokenDeleter: () async {}, // device-side delete succeeds
        tokenRefreshStreamProvider: () => const Stream<String>.empty(),
      );

      // Bring the manager into a state where it knows the active token.
      await manager.initializeToken('user_1');
      expect(manager.fcmToken, equals('registered_token'));

      // Now backend deregister will throw — logout must still complete.
      api.removeThrows = DioException(
        requestOptions: RequestOptions(path: '/notifications/fcm-token'),
        response: Response<dynamic>(
          requestOptions: RequestOptions(path: '/notifications/fcm-token'),
          statusCode: 500,
        ),
      );

      // No throw expected.
      await manager.deleteToken('user_1');

      expect(api.removeCalls, hasLength(1));
      expect(
        manager.fcmToken,
        isNull,
        reason: 'local token state must be cleared even on backend failure',
      );
    });

    test('deleteToken completes even when device-side delete throws', () async {
      final api = _RecordingApiDatasource();
      final datasource = NotificationRemoteDatasource(apiDatasource: api);

      final manager = FCMTokenManager(
        datasource: datasource,
        tokenFetcher: () async => 'registered_token',
        tokenDeleter: () async {
          throw StateError('simulated platform FCM delete failure');
        },
        tokenRefreshStreamProvider: () => const Stream<String>.empty(),
      );

      await manager.initializeToken('user_1');

      // No throw expected — device-side failure logged & swallowed,
      // backend deregister still attempted.
      await manager.deleteToken('user_1');

      expect(api.removeCalls, hasLength(1));
      expect(manager.fcmToken, isNull);
    });

    test('deleteToken with both device-side AND backend failures still '
        'returns normally and clears local token state', () async {
      final api = _RecordingApiDatasource()
        ..removeThrows = StateError('backend down');
      final datasource = NotificationRemoteDatasource(apiDatasource: api);

      final manager = FCMTokenManager(
        datasource: datasource,
        tokenFetcher: () async => 'registered_token',
        tokenDeleter: () async {
          throw StateError('platform delete failed');
        },
        tokenRefreshStreamProvider: () => const Stream<String>.empty(),
      );

      await manager.initializeToken('user_1');

      // No throw — logout must complete.
      await manager.deleteToken('user_1');

      expect(manager.fcmToken, isNull);
    });
  });

  group('FCMTokenManager — concurrent-register mutex', () {
    test('a token-refresh event firing while initializeToken is registering '
        'is single-flighted: backend sees exactly one register for the '
        'same (user, token) tuple', () async {
      final api = _RecordingApiDatasource();

      // Slow saveUserToken so the refresh event has time to race in.
      final slowDatasource = _SlowDatasource(
        apiDatasource: api,
        delay: const Duration(milliseconds: 50),
      );

      // Controllable token-refresh stream.
      final refreshController = StreamController<String>.broadcast();

      final manager = FCMTokenManager(
        datasource: slowDatasource,
        tokenFetcher: () async => 'token_v1',
        tokenRefreshStreamProvider: () => refreshController.stream,
      );

      // Kick off initializeToken — first register is in flight.
      final initFuture = manager.initializeToken('user_1');

      // Wait briefly so the in-flight register has started.
      await Future<void>.delayed(const Duration(milliseconds: 5));

      // Fire a refresh for the SAME token (this can happen on iOS
      // immediately after a fresh token issuance).
      refreshController.add('token_v1');

      // Let everything settle.
      await initFuture;
      await Future<void>.delayed(const Duration(milliseconds: 200));
      await refreshController.close();

      // The mutex should have de-duplicated the second register
      // attempt against the in-flight first one. Expect EXACTLY 1
      // call for token_v1.
      final tokenV1Registrations = api.registerCalls
          .where((c) => c.token == 'token_v1')
          .length;
      expect(
        tokenV1Registrations,
        equals(1),
        reason:
            'mutex must single-flight registration of the same token across '
            'initializeToken + onTokenRefresh races',
      );
    });
  });
}

// ---------------------------------------------------------------------------
// Helpers that don't touch the FCM SDK
// ---------------------------------------------------------------------------
//
// Note: FCMTokenManager now accepts a nullable `messaging:` so unit
// tests that supply the three seams (`tokenFetcher`, `tokenDeleter`,
// `tokenRefreshStreamProvider`) can omit FirebaseMessaging entirely.

/// Wraps a [NotificationApiDatasource] but adds an artificial delay so
/// the mutex race can be observed deterministically.
class _SlowDatasource extends NotificationRemoteDatasource {
  final Duration _delay;
  _SlowDatasource({
    required NotificationApiDatasource apiDatasource,
    required Duration delay,
  }) : _delay = delay,
       super(apiDatasource: apiDatasource);

  @override
  Future<void> saveUserToken({
    required String userId,
    required String token,
  }) async {
    await Future<void>.delayed(_delay);
    await super.saveUserToken(userId: userId, token: token);
  }
}
