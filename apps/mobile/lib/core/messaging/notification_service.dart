/// Core Notification Service Interface
///
/// Pure interface without Firebase dependencies.
/// Implementation will be provided by FcmServiceImpl.
///
/// **Location:** lib/core/messaging/notification_service.dart
/// **Implementation:** lib/core/messaging/fcm_service_impl.dart
///
/// **IMPORTANT:** Firebase SDK is ONLY allowed in fcm_service_impl.dart
/// All other files must use this interface.
abstract class NotificationService {
  // ==================== FCM Operations ====================

  /// Request notification permission.
  Future<bool> requestPermission();

  /// Get current FCM token.
  Future<String?> getToken();

  /// Subscribe to a topic.
  Future<void> subscribeToTopic(String topic);

  /// Unsubscribe from a topic.
  Future<void> unsubscribeFromTopic(String topic);

  /// Stream of incoming messages when app is in foreground.
  Stream<Map<String, dynamic>> get onMessage;

  /// Stream of messages when app is opened from notification.
  Stream<Map<String, dynamic>> get onMessageOpened;

  /// Initialize notification service for a user.
  Future<void> initialize({required String userId});

  /// Cleanup notification service on logout.
  Future<void> cleanup({required String userId});

  // ==================== Data Operations ====================
  // These methods handle Firestore operations for notifications.
  // Firestore serves as the event store for Cloud Functions triggers.

  /// Get user notification preferences from Firestore.
  ///
  /// Returns Map containing preference data with keys:
  /// - pushEnabled, orderNotifications, chatNotifications, etc.
  Future<Map<String, dynamic>?> getPreferences({required String userId});

  /// Save notification to Firestore.
  ///
  /// This triggers Cloud Functions to send FCM push notification.
  /// [data] must contain: id, userId, type, title, body, isRead, createdAt
  Future<void> saveNotification(Map<String, dynamic> data);

  /// Get user's FCM token from Firestore.
  Future<String?> getUserToken({required String userId});

  /// Update user notification preferences in Firestore.
  Future<void> updatePreferences({
    required String userId,
    required Map<String, dynamic> preferences,
  });
}
