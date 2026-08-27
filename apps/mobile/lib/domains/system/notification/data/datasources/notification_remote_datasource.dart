// Dart
// FIRESTORE SUNSET (2025-02-20): Firestore removed - now uses Backend API
//
// Tier 3 (Runtime Honesty): the FCM token methods
// `saveUserToken` / `deleteUserToken` now delegate to the canonical
// [NotificationApiDatasource] (POST/DELETE /notifications/fcm/token)
// when an `apiDatasource` is provided. Without it (legacy callers),
// they degrade gracefully and log a warning — they no longer silently
// no-op. The other methods on this class remain compatibility stubs
// (separately deprecated) and are intentionally out of Tier 3 scope.

import 'package:labuda/core/api/platform/platform_io.dart'
    if (dart.library.html) 'package:labuda/core/api/platform/platform_web.dart';
import 'package:labuda/core/src/interfaces/services/i_logger_service.dart';
import 'package:labuda/domains/system/notification/data/datasources/notification_api_datasource.dart';
import 'package:labuda/domains/system/notification/data/models/api/notification_api_models.dart';

/// Notification Remote Datasource
///
/// FIRESTORE SUNSET (2025-02-20): All operations moved to Backend API.
/// This class is kept for interface compatibility.
/// NO business logic, hanya CRUD operations via Backend API.
///
/// Tier 3 closed: FCM token persistence (`saveUserToken` /
/// `deleteUserToken`) now reaches the canonical
/// `NotificationApiDatasource` instead of being a silent no-op.
class NotificationRemoteDatasource {
  final NotificationApiDatasource? _apiDatasource;
  final ILoggerService? _logger;

  NotificationRemoteDatasource({
    NotificationApiDatasource? apiDatasource,
    ILoggerService? logger,
  }) : _apiDatasource = apiDatasource,
       _logger = logger;

  // RECOVERY: Stub methods to fix compilation errors
  Stream<List<dynamic>> getNotifications({
    required String userId,
    int limit = 20,
  }) {
    return Stream.value([]);
  }

  Future<void> markAsRead({required String notificationId}) async {}

  Future<void> markAsReadByEntity({
    required String userId,
    required String entityType,
    required String entityId,
  }) async {
    // TODO: Implement API call to mark notifications as read by entity
    // This will be integrated when the backend API is ready
  }

  Future<void> markAllAsRead({required String userId}) async {}

  Stream<int> getUnreadCount({required String userId}) {
    return Stream.value(0);
  }

  Future<Map<String, dynamic>> getPreferences({required String userId}) async {
    return {};
  }

  Future<void> updatePreferences({
    required String userId,
    required Map<String, dynamic> preferences,
  }) async {}

  Future<void> deleteNotification({required String notificationId}) async {}

  Future<void> deleteAllNotifications({required String userId}) async {}

  Future<int> deleteReadNotifications({required String userId}) async {
    return 0;
  }

  /// Register the device's FCM token with the backend so push
  /// notifications can be routed to this device. Delegates to the
  /// canonical `POST /notifications/fcm/token`.
  ///
  /// The current FCM token is reused as the stable `deviceId` — FCM
  /// guarantees the token is unique per app install, and using it here
  /// gives us a deterministic key to pass back to
  /// `removeFCMToken(deviceId)` on logout / token rotation without
  /// having to maintain a separate persistent device-id store.
  ///
  /// When [_apiDatasource] is null (legacy bootstrap) we log a warning
  /// and return — the call no longer pretends to have succeeded.
  /// Errors are rethrown so [FCMTokenManager] can apply its own
  /// retry/backoff policy.
  Future<void> saveUserToken({
    required String userId,
    required String token,
  }) async {
    final api = _apiDatasource;
    if (api == null) {
      _logger?.warning(
        'FCM saveUserToken: no NotificationApiDatasource wired — '
        'backend registration skipped (push will not arrive on this device)',
      );
      return;
    }
    try {
      final platform = _resolvePlatform();
      final result = await api.registerFCMToken(
        RegisterFCMTokenRequest(
          token: token,
          platform: platform,
          // FCM token reused as deviceId so the matching
          // removeFCMToken(deviceId) call has a precise target.
          deviceId: token,
        ),
      );
      _logger?.info(
        'FCM saveUserToken: backend registration OK '
        '(id=${result.id}, platform=$platform)',
      );
    } catch (e, stackTrace) {
      _logger?.error(
        'FCM saveUserToken: backend registration failed',
        extra: {'error': e.toString(), 'userId': userId},
        stackTrace: stackTrace,
      );
      rethrow;
    }
  }

  /// Deregister an FCM token from the backend so push notifications
  /// stop being routed to this device — critical on logout to avoid
  /// cross-account push leakage. Delegates to
  /// `DELETE /notifications/fcm/token?device_id=...`.
  ///
  /// When [_apiDatasource] is null (legacy bootstrap) we log a
  /// warning instead of silently returning. Errors are rethrown so
  /// [FCMTokenManager.deleteToken] can decide how to handle them
  /// (logout must always complete, so it currently logs-and-continues).
  Future<void> deleteUserToken({
    required String userId,
    required String token,
  }) async {
    final api = _apiDatasource;
    if (api == null) {
      _logger?.warning(
        'FCM deleteUserToken: no NotificationApiDatasource wired — '
        'backend deregistration skipped (orphan token may remain server-side)',
      );
      return;
    }
    try {
      // The FCM token was registered as the deviceId in saveUserToken,
      // so it is also the precise key to remove.
      await api.removeFCMToken(token);
      _logger?.info(
        'FCM deleteUserToken: backend deregistration OK (userId=$userId)',
      );
    } catch (e, stackTrace) {
      _logger?.error(
        'FCM deleteUserToken: backend deregistration failed',
        extra: {'error': e.toString(), 'userId': userId},
        stackTrace: stackTrace,
      );
      rethrow;
    }
  }

  String _resolvePlatform() {
    if (platformDetector.isIOS) return 'ios';
    if (platformDetector.isAndroid) return 'android';
    return 'web';
  }
}
