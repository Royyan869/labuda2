/// Notification Repository Interface
///
/// Defines contract untuk data operations pada notifications.
/// Implementation ada di data layer (notification_repository_impl.dart).
///
/// Size: < 100 lines (per GUIDELINES)
library;

// Dart
import 'package:labuda/core/core.dart' hide NotificationEntity;
import 'package:labuda/domains/system/notification/domain/entities/notification_entity.dart';
import 'package:labuda/domains/system/notification/domain/entities/notification_preference_entity.dart';

abstract class INotificationRepository {
  /// Get notifications untuk user (real-time stream)
  ///
  /// Returns stream of notifications yang auto-update dari Firestore.
  /// Supports pagination dengan limit parameter.
  Stream<Result<List<NotificationEntity>>> getNotifications({
    required String userId,
    int limit = 20,
  });

  /// Mark single notification as read
  Future<Result<void>> markAsRead({required String notificationId});

  /// Mark notifications as read by entity type and entity ID
  ///
  /// Used for chat-notification sync: when user reads chat messages,
  /// mark corresponding chat notifications as read.
  Future<Result<void>> markAsReadByEntity({
    required String userId,
    required String entityType,
    required String entityId,
  });

  /// Mark all notifications as read untuk user
  Future<Result<void>> markAllAsRead({required String userId});

  /// Get unread notification count
  ///
  /// Returns real-time stream of unread count.
  Stream<Result<int>> getUnreadCount({required String userId});

  /// Get user notification preferences
  Future<Result<NotificationPreferenceEntity>> getPreferences({
    required String userId,
  });

  /// Update user notification preferences
  Future<Result<void>> updatePreferences({
    required NotificationPreferenceEntity preferences,
  });

  /// Delete single notification
  Future<Result<void>> deleteNotification({required String notificationId});

  /// Delete all notifications untuk user
  Future<Result<void>> deleteAllNotifications({required String userId});

  /// Delete all read notifications untuk user
  ///
  /// Returns the count of deleted notifications
  Future<Result<int>> deleteReadNotifications({required String userId});
}
