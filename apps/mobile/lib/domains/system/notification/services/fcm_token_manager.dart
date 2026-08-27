/// FCM Token Manager
///
/// Handles Firebase Cloud Messaging token operations:
/// - Token retrieval (with bounded retry)
/// - Token refresh (with concurrent-register mutex)
/// - Token persistence via the canonical backend datasource
/// - Token cleanup (logout-tolerant — backend failure does NOT block)
///
/// Extracted from fcm_service.dart for better modularity.
///
/// Tier 3 (Runtime Honesty) hardening:
/// - `getToken()` failures are no longer silently swallowed — they are
///   logged and retried exactly once before giving up
/// - `onTokenRefresh` events route through a mutex so a refresh that
///   fires while `initializeToken` is still running cannot cause
///   duplicate concurrent backend registrations
/// - `deleteToken()` no longer rethrows on backend deregistration
///   failure — logout must always complete; the worst case is an
///   orphan token record server-side (logged explicitly)
library;

// Dart
import 'dart:async';

import 'package:firebase_messaging/firebase_messaging.dart';
import 'package:labuda/core/src/interfaces/services/i_logger_service.dart';
import 'package:labuda/domains/system/notification/data/datasources/notification_remote_datasource.dart';

/// Test seam — returns the current FCM device token. Production wraps
/// `FirebaseMessaging.getToken()`.
typedef FcmTokenFetcher = Future<String?> Function();

/// Test seam — deletes the device-side FCM token. Production wraps
/// `FirebaseMessaging.deleteToken()`.
typedef FcmTokenDeleter = Future<void> Function();

/// Test seam — yields the `onTokenRefresh` stream. Production wraps
/// `FirebaseMessaging.onTokenRefresh`.
typedef FcmTokenRefreshStreamProvider = Stream<String> Function();

class FCMTokenManager {
  /// Maximum total attempts (1 initial + 1 retry) for getToken.
  static const int _getTokenMaxAttempts = 2;

  /// Bound on logout-time backend deregistration — logout must not
  /// hang on a flaky network.
  static const Duration _logoutBackendTimeout = Duration(seconds: 5);

  // Nullable so unit tests that supply the seams below do not need a
  // real FirebaseMessaging instance (which would require booting the
  // Firebase platform channel). Production always supplies a non-null
  // value via the FcmService constructor.
  final FirebaseMessaging? _messaging;
  final NotificationRemoteDatasource _datasource;
  final ILoggerService? _logger;

  // Test seams. When null, production paths fall through to the
  // FirebaseMessaging singleton.
  final FcmTokenFetcher? _tokenFetcher;
  final FcmTokenDeleter? _tokenDeleter;
  final FcmTokenRefreshStreamProvider? _tokenRefreshStreamProvider;

  String? _fcmToken;
  String? get fcmToken => _fcmToken;

  StreamSubscription<String>? _tokenRefreshSubscription;

  /// Concurrent-register guard. Holds the future of the in-flight
  /// backend registration call so a coincident token-refresh event
  /// does NOT fire a second `registerFCMToken` against the same
  /// (token, user) tuple. Cleared in `finally`.
  Future<void>? _ongoingRegistration;

  FCMTokenManager({
    FirebaseMessaging? messaging,
    required NotificationRemoteDatasource datasource,
    ILoggerService? logger,
    FcmTokenFetcher? tokenFetcher,
    FcmTokenDeleter? tokenDeleter,
    FcmTokenRefreshStreamProvider? tokenRefreshStreamProvider,
  }) : // Production always supplies [messaging]. Unit tests typically
       // supply only the seams they exercise — the operation-specific
       // fallback (`_messaging!.xxx()`) trips with a clear NPE if a
       // test inadvertently drives an un-stubbed FirebaseMessaging
       // path, which is the desired failure mode.
       _messaging = messaging,
       _datasource = datasource,
       _logger = logger,
       _tokenFetcher = tokenFetcher,
       _tokenDeleter = tokenDeleter,
       _tokenRefreshStreamProvider = tokenRefreshStreamProvider;

  Future<String?> _getDeviceToken() async {
    final fetcher = _tokenFetcher;
    if (fetcher != null) return fetcher();
    // Production path: _messaging is guaranteed non-null when no seam
    // is supplied (see constructor assert).
    return _messaging!.getToken();
  }

  Future<void> _deleteDeviceToken() async {
    final deleter = _tokenDeleter;
    if (deleter != null) return deleter();
    return _messaging!.deleteToken();
  }

  Stream<String> _onTokenRefresh() {
    final factory = _tokenRefreshStreamProvider;
    if (factory != null) return factory();
    return _messaging!.onTokenRefresh;
  }

  /// Initialize token: fetch from FCM, register with backend, set up
  /// refresh listener. Bounded retry (1 retry max) on transient
  /// device-side failures (e.g. iOS pre-permission moment, transient
  /// FCM connectivity). Returns null on permanent failure and never
  /// loops further.
  Future<String?> initializeToken(String userId) async {
    Object? lastError;
    for (var attempt = 1; attempt <= _getTokenMaxAttempts; attempt++) {
      try {
        final token = await _getDeviceToken();
        if (token == null || token.isEmpty) {
          _logger?.warning(
            'FCM initializeToken attempt $attempt: getToken returned '
            'null/empty (transient — likely pre-permission on iOS or '
            'network unavailable)',
          );
          if (attempt < _getTokenMaxAttempts) continue;
          return null;
        }

        _fcmToken = token;

        // Backend registration. Wrapped in its own try/catch so an
        // initial-registration failure is logged AND retried as part
        // of this loop (instead of falling through silently).
        try {
          await _registerWithBackend(userId, token);
        } catch (e, stackTrace) {
          _logger?.error(
            'FCM initializeToken attempt $attempt: backend registration '
            'failed (will retry if attempts remain)',
            extra: {'error': e.toString(), 'attempt': '$attempt'},
            stackTrace: stackTrace,
          );
          if (attempt < _getTokenMaxAttempts) {
            lastError = e;
            continue;
          }
          // Backend failed but device-side token was obtained — keep
          // _fcmToken so the refresh listener can later re-register.
          // Set up the refresh listener regardless and return the
          // device token to the caller so the FCM stream still flows.
          _setupTokenRefresh(userId);
          return token;
        }

        _setupTokenRefresh(userId);
        return token;
      } catch (e, stackTrace) {
        lastError = e;
        _logger?.error(
          'FCM initializeToken attempt $attempt: getToken threw',
          extra: {'error': e.toString(), 'attempt': '$attempt'},
          stackTrace: stackTrace,
        );
        if (attempt >= _getTokenMaxAttempts) break;
      }
    }
    _logger?.error(
      'FCM initializeToken: gave up after $_getTokenMaxAttempts attempts',
      extra: {'lastError': lastError?.toString() ?? 'unknown'},
    );
    return null;
  }

  /// Single-flight backend registration. If a registration is already
  /// in flight, returns that future instead of issuing a second
  /// concurrent POST. Cleared in finally so subsequent registrations
  /// (e.g. a later token refresh) are not permanently blocked.
  Future<void> _registerWithBackend(String userId, String token) async {
    final inFlight = _ongoingRegistration;
    if (inFlight != null) {
      _logger?.debug(
        'FCM _registerWithBackend: registration already in flight — '
        'awaiting existing call to prevent storm',
      );
      // Don't rethrow if the awaited registration failed — that error
      // belongs to the original caller; we just observed it.
      try {
        await inFlight;
      } catch (_) {
        // Best-effort observation only.
      }
      return;
    }
    final future = _datasource.saveUserToken(userId: userId, token: token);
    _ongoingRegistration = future;
    try {
      await future;
    } finally {
      _ongoingRegistration = null;
    }
  }

  /// Setup token refresh listener
  void _setupTokenRefresh(String userId) {
    // Cancel existing subscription if any
    _tokenRefreshSubscription?.cancel();

    // Listen for token refresh
    _tokenRefreshSubscription = _onTokenRefresh().listen(
      (newToken) async {
        final oldToken = _fcmToken;
        _fcmToken = newToken;
        _logger?.info(
          'FCM onTokenRefresh fired '
          '(oldTokenLen=${oldToken?.length ?? 0}, '
          'newTokenLen=${newToken.length})',
        );
        try {
          // Register NEW token before removing OLD token so there is
          // never a gap during which no token is registered for this
          // user. The mutex makes this safe against concurrent
          // initializeToken calls.
          await _registerWithBackend(userId, newToken);

          if (oldToken != null && oldToken != newToken) {
            try {
              await _datasource.deleteUserToken(
                userId: userId,
                token: oldToken,
              );
            } catch (e) {
              // An orphan old token on the backend is the least-bad
              // failure mode; the new token is what matters for
              // current routing. Log and continue.
              _logger?.warning(
                'FCM onTokenRefresh: old-token cleanup failed — '
                'backend may retain orphan record: $e',
              );
            }
          }
        } catch (e, stackTrace) {
          _logger?.error(
            'FCM onTokenRefresh: backend re-registration failed — '
            'push notifications may not arrive until next app start',
            extra: {'error': e.toString(), 'userId': userId},
            stackTrace: stackTrace,
          );
        }
      },
      onError: (Object error, StackTrace stackTrace) {
        _logger?.error(
          'FCM onTokenRefresh stream error — refresh listener may have died',
          extra: {'error': error.toString()},
          stackTrace: stackTrace,
        );
      },
    );
  }

  /// Delete token from device and server.
  ///
  /// Tolerant of partial failure: device-side and backend-side delete
  /// are attempted independently, each with a timeout, so a slow or
  /// failed backend deregistration does NOT block logout. The worst
  /// case is an orphan token record server-side (logged explicitly so
  /// it is visible in telemetry).
  Future<void> deleteToken(String userId) async {
    final tokenSnapshot = _fcmToken;

    // Device-side delete. Bounded by timeout so a hung FCM SDK call
    // cannot stall logout indefinitely.
    try {
      await _deleteDeviceToken().timeout(_logoutBackendTimeout);
    } catch (e, stackTrace) {
      _logger?.warning(
        'FCM deleteToken: device-side delete failed/timeout — '
        'logout still continues',
        extra: {'error': e.toString(), 'userId': userId},
      );
      // Don't print stackTrace at warning level — debug only.
      _logger?.debug(
        'FCM deleteToken device-side stack',
        extra: {'stack': stackTrace.toString()},
      );
    }

    // Backend-side delete. Same timeout discipline.
    if (tokenSnapshot != null) {
      try {
        await _datasource
            .deleteUserToken(userId: userId, token: tokenSnapshot)
            .timeout(_logoutBackendTimeout);
      } catch (e, stackTrace) {
        _logger?.error(
          'FCM deleteToken: backend deregistration failed — '
          'orphan token may remain server-side, cross-account push risk '
          'for this device until backend GC',
          extra: {'error': e.toString(), 'userId': userId},
          stackTrace: stackTrace,
        );
        // Intentionally NOT rethrown — logout must always complete.
      }
    }

    _fcmToken = null;
  }

  /// Request notification permissions (iOS)
  Future<bool> requestPermission() async {
    final messaging = _messaging;
    if (messaging == null) {
      // Test paths that don't drive the permission flow can leave the
      // FirebaseMessaging reference null. Treat as denied so callers
      // don't fall into a non-deterministic state.
      return false;
    }
    try {
      final settings = await messaging.requestPermission(
        alert: true,
        badge: true,
        sound: true,
        provisional: false,
      );

      final isAuthorized =
          settings.authorizationStatus == AuthorizationStatus.authorized;

      return isAuthorized;
    } catch (e, stackTrace) {
      _logger?.error(
        'FCM requestPermission threw — treating as denied',
        extra: {'error': e.toString()},
        stackTrace: stackTrace,
      );
      return false;
    }
  }

  /// Cleanup - cancel subscriptions
  void dispose() {
    _tokenRefreshSubscription?.cancel();
    _tokenRefreshSubscription = null;
  }
}
