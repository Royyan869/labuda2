import 'package:labuda/core/interfaces/i_notification_trigger.dart'
    show NotificationType;
import 'package:labuda/core/common/result.dart';

/// Old notification service interface
///
/// NOTE: NotificationType is now imported from i_notification_trigger.dart
/// to maintain single source of truth.
abstract class INotificationService {
  Future<Result<void>> initialize();
  Future<Result<String?>> getDeviceToken();
  Future<Result<void>> subscribeToTopic(String topic);
  Future<Result<void>> unsubscribeFromTopic(String topic);
  Future<Result<void>> requestPermission();
  Future<Result<List<NotificationEntity>>> getNotifications({
    int page = 1,
    int limit = 20,
  });
  Future<Result<NotificationEntity>> getNotificationById(String notificationId);
  Future<Result<void>> markAsRead(String notificationId);
  Future<Result<void>> markAllAsRead();
  Future<Result<void>> deleteNotification(String notificationId);
  Future<Result<void>> clearAllNotifications();
  Future<Result<NotificationSettings>> getNotificationSettings();
  Future<Result<void>> updateNotificationSettings(
    NotificationSettings settings,
  );
  Stream<NotificationEntity> get onNotificationReceived;
  Stream<NotificationEntity> get onNotificationOpened;
}

abstract class NotificationEntity {
  String get id;
  String get userId;
  NotificationType get type;
  String get title;
  String get body;
  String? get imageUrl;
  Map<String, dynamic>? get data;
  bool get isRead;
  DateTime get createdAt;
  DateTime? get readAt;
}

abstract class NotificationSettings {
  bool get enabled;
  bool get pushEnabled;
  bool get emailEnabled;
  bool get smsEnabled;
  NotificationPreferences get preferences;
}

abstract class NotificationPreferences {
  bool get newMessages;
  bool get newFollowers;
  bool get collectionUpdates;
  bool get auctionUpdates;
  bool get paymentUpdates;
  bool get marketingUpdates;
  bool get systemUpdates;
}
