// Notification API Datasource
// HTTP operations for Notification domain

// External
import 'package:hishumi/core/api/api.dart';

// Internal
import 'package:hishumi/domains/system/notification/data/models/api/notification_api_models.dart';

class NotificationApiDatasource {
  final ApiClient _apiClient;

  NotificationApiDatasource(this._apiClient);

  // ============================================================================
  // Notification Operations
  // ============================================================================

  /// List user's notifications with pagination
  /// Query params: limit, offset
  Future<ListNotificationsResponse> listNotifications({
    int? page,
    int? perPage,
    bool? unreadOnly,
  }) async {
    final params = <String, dynamic>{};
    final safePage = (page != null && page > 0) ? page : 1;
    final safePerPage = (perPage != null && perPage > 0) ? perPage : 20;
    params['limit'] = safePerPage;
    params['offset'] = (safePage - 1) * safePerPage;
    // Backend doesn't support unread_only query yet; keep argument for API
    // compatibility with existing repository callers.
    if (unreadOnly != null) params['unread_only'] = unreadOnly;

    final response = await _apiClient.get(
      '/notifications',
      queryParameters: params,
    );
    return ListNotificationsResponse.fromJson(response.data['data']);
  }

  /// Get a specific notification by ID.
  /// Backend has no GET /notifications/:id route; fetch from list and match.
  Future<NotificationResponse> getNotification(String notificationId) async {
    final response = await listNotifications(
      page: 1,
      perPage: 100,
      unreadOnly: null,
    );
    final match = response.notifications.where((n) => n.id == notificationId);
    if (match.isEmpty) {
      throw const NotFoundException(
        message: 'Notification not found',
        code: 'NOTIFICATION_NOT_FOUND',
      );
    }
    return match.first;
  }

  /// Mark specific notifications as read.
  /// Returns the canonical post-mutation `unread_count` from the LAST
  /// mutation response (backend contract from Task 1).
  Future<int> markNotificationsAsRead(List<String> notificationIds) async {
    var unreadCount = 0;
    for (final id in notificationIds) {
      final response = await _apiClient.post('/notifications/$id/read', data: {});
      unreadCount = _readMutationUnreadCount(response);
    }
    return unreadCount;
  }

  /// Mark all notifications as read.
  /// Returns the canonical post-mutation `unread_count` (normally 0).
  Future<int> markAllNotificationsAsRead() async {
    final response = await _apiClient.post('/notifications/read-all', data: {});
    return _readMutationUnreadCount(response);
  }

  /// Mark notifications as read by entity type and entity ID
  /// Used for cross-domain sync (e.g., chat read → chat notifications read)
  Future<void> markAsReadByEntity({
    required String entityType,
    required String entityId,
  }) async {
    await _apiClient.post(
      '/notifications/read-by-entity',
      data: MarkAsReadByEntityRequest(
        entityType: entityType,
        entityId: entityId,
      ).toJson(),
    );
  }

  /// Delete a notification.
  /// Returns the canonical post-mutation `unread_count` from the response.
  Future<int> deleteNotification(String notificationId) async {
    final response = await _apiClient.delete('/notifications/$notificationId');
    return _readMutationUnreadCount(response);
  }

  /// Extracts the canonical post-mutation unread count from a mutation
  /// response envelope (`data.unread_count`).
  int _readMutationUnreadCount(dynamic response) {
    final payload = response.data['data'] as Map<String, dynamic>? ?? const {};
    return MutationUnreadCountResponse.fromJson(payload).unreadCount;
  }

  /// Get unread notification count.
  /// Canonical backend endpoint: GET /notifications/unread-count -> {count}
  Future<UnreadCountResponse> getUnreadCount() async {
    final response = await _apiClient.get('/notifications/unread-count');
    final payload = response.data['data'] as Map<String, dynamic>? ?? const {};
    return UnreadCountResponse.fromJson(payload);
  }

  // ============================================================================
  // FCM Token Management
  // ============================================================================

  /// Register FCM token
  Future<FCMTokenResponse> registerFCMToken(
    RegisterFCMTokenRequest request,
  ) async {
    final response = await _apiClient.post(
      '/notifications/fcm-token',
      data: request.toJson(),
    );
    final payload = response.data['data'] as Map<String, dynamic>? ?? const {};
    return FCMTokenResponse.fromJson(payload);
  }

  /// Remove FCM token by device ID
  Future<void> removeFCMToken(String deviceId) async {
    await _apiClient.delete(
      '/notifications/fcm-token',
      queryParameters: {'device_id': deviceId},
    );
  }

  /// List user's FCM tokens (not supported by canonical backend routes).
  Future<ListFCMTokensResponse> listFCMTokens() async {
    throw UnsupportedError(
      'GET /notifications/fcm/tokens is not available in canonical backend routes.',
    );
  }

  // ============================================================================
  // Notification Preferences
  // ============================================================================

  /// Get user's notification preferences (not supported by canonical backend routes).
  Future<NotificationPreferencesResponse> getPreferences() async {
    throw UnsupportedError(
      'GET /notifications/preferences is not available in canonical backend routes.',
    );
  }

  /// Update notification preferences (not supported by canonical backend routes).
  Future<NotificationPreferencesResponse> updatePreferences(
    UpdateNotificationPreferencesRequest request,
  ) async {
    throw UnsupportedError(
      'PUT /notifications/preferences is not available in canonical backend routes.',
    );
  }

  // ============================================================================
  // Push Notification Sending (for internal use / triggers)
  // ============================================================================

  /// Send push notification to single user
  Future<void> sendPushNotification(SendPushNotificationRequest request) async {
    throw UnsupportedError(
      'POST /notifications/push is not available in canonical backend routes.',
    );
  }

  /// Send batch push notification
  Future<void> sendBatchPushNotification(SendBatchPushRequest request) async {
    throw UnsupportedError(
      'POST /notifications/push/batch is not available in canonical backend routes.',
    );
  }
}
