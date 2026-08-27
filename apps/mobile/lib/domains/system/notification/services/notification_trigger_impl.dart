/// Notification Trigger Implementation
///
/// Implements INotificationTrigger untuk digunakan oleh modul lain.
///
/// **NOTIFICATION CREATION DISABLED:** Firestore writes disabled.
/// Notification creation moved to backend. Use backend API to send notifications.
///
/// **IMPORTANT:** This file does NOT import Firebase SDK directly.
/// All Firebase operations are abstracted through NotificationService interface.
library;

// Dart
import 'package:labuda/core/core.dart' hide NotificationType;
import 'package:labuda/core/interfaces/i_notification_trigger.dart';

class NotificationTriggerImpl implements INotificationTrigger {
  final NotificationService notificationService;

  NotificationTriggerImpl({required this.notificationService});

  @override
  Future<Result<void>> sendNotification({
    required String userId,
    required NotificationType type,
    required String title,
    required String body,
    Map<String, dynamic>? data,
  }) async {
    // TODO: migrate to backend notification API
    // Notification creation moved to backend. Use backend API to send notifications.
    return Result.success(null);
  }

  @override
  Future<Result<void>> sendNotificationBatch({
    required List<String> userIds,
    required NotificationType type,
    required String title,
    required String body,
    Map<String, dynamic>? data,
  }) async {
    // TODO: migrate to backend notification API
    // Notification creation moved to backend. Use backend API to send notifications.
    return Result.success(null);
  }
}
