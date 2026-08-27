// Notification Repository Implementation
// Implements INotificationRepository dengan error handling dan Result wrapping.

// Dart
import 'dart:async';

// External
import 'package:labuda/core/core.dart'
    hide NotificationEntity, NotificationType;

// Internal
import 'package:labuda/domains/system/notification/data/datasources/notification_remote_datasource.dart';
import 'package:labuda/domains/system/notification/data/models/notification_model.dart';
import 'package:labuda/domains/system/notification/data/models/notification_preference_model.dart';
import 'package:labuda/domains/system/notification/domain/entities/notification_entity.dart';
import 'package:labuda/domains/system/notification/domain/entities/notification_preference_entity.dart';
import 'package:labuda/domains/system/notification/domain/repositories/i_notification_repository.dart';

/// Stub extension for Map toEntity - RECOVERY
/// BATCH N2: Removed listingRecommendations and auctionNotifications (ghost features)
extension NotificationPreferenceMapExtension on Map<String, dynamic> {
  NotificationPreferenceEntity toPreferenceEntity() {
    return NotificationPreferenceEntity(
      userId: this['userId'] as String? ?? '',
      pushEnabled: this['pushEnabled'] as bool? ?? true,
      orderNotifications: this['orderNotifications'] as bool? ?? true,
      chatNotifications: this['chatNotifications'] as bool? ?? true,
      securityAlerts: this['securityAlerts'] as bool? ?? true,
      marketingNotifications: this['marketingNotifications'] as bool? ?? false,
    );
  }
}

/// Notification Repository Implementation
///
/// Implements INotificationRepository dengan error handling dan Result wrapping.
///
/// Size: < 250 lines (per GUIDELINES)
class NotificationRepositoryImpl implements INotificationRepository {
  final NotificationRemoteDatasource remoteDatasource;

  NotificationRepositoryImpl({required this.remoteDatasource});

  @override
  Stream<Result<List<NotificationEntity>>> getNotifications({
    required String userId,
    int limit = 20,
  }) {
    return remoteDatasource
        .getNotifications(userId: userId, limit: limit)
        .transform(
          StreamTransformer<
            List<NotificationModel>,
            Result<List<NotificationEntity>>
          >.fromHandlers(
            handleData: (models, sink) {
              try {
                final entities = models
                    .map((model) => model.toEntity())
                    .toList();
                sink.add(Result.success(entities));
              } catch (e) {
                sink.add(Result<List<NotificationEntity>>.error(e.toString()));
              }
            },
            handleError: (error, _, sink) {
              // Emit error Result instead of breaking stream
              sink.add(
                Result<List<NotificationEntity>>.error(error.toString()),
              );
            },
          ),
        );
  }

  @override
  Future<Result<void>> markAsRead({required String notificationId}) async {
    try {
      await remoteDatasource.markAsRead(notificationId: notificationId);
      return Result.success(null);
    } catch (e) {
      return Result.error(e.toString());
    }
  }

  @override
  Future<Result<void>> markAsReadByEntity({
    required String userId,
    required String entityType,
    required String entityId,
  }) async {
    try {
      await remoteDatasource.markAsReadByEntity(
        userId: userId,
        entityType: entityType,
        entityId: entityId,
      );
      return Result.success(null);
    } catch (e) {
      return Result.error(e.toString());
    }
  }

  @override
  Future<Result<void>> markAllAsRead({required String userId}) async {
    try {
      await remoteDatasource.markAllAsRead(userId: userId);
      return Result.success(null);
    } catch (e) {
      return Result.error(e.toString());
    }
  }

  @override
  Stream<Result<int>> getUnreadCount({required String userId}) {
    return remoteDatasource
        .getUnreadCount(userId: userId)
        .transform(
          StreamTransformer<int, Result<int>>.fromHandlers(
            handleData: (count, sink) {
              sink.add(Result.success(count));
            },
            handleError: (error, stackTrace, sink) {
              // Emit error Result instead of breaking stream
              sink.add(Result<int>.error(error.toString()));
            },
          ),
        );
  }

  @override
  Future<Result<NotificationPreferenceEntity>> getPreferences({
    required String userId,
  }) async {
    try {
      final map = await remoteDatasource.getPreferences(userId: userId);
      return Result.success(map.toPreferenceEntity());
    } catch (e) {
      return Result.error(e.toString());
    }
  }

  @override
  Future<Result<void>> updatePreferences({
    required NotificationPreferenceEntity preferences,
  }) async {
    try {
      final model = NotificationPreferenceModel.fromEntity(preferences);
      await remoteDatasource.updatePreferences(
        userId: preferences.userId,
        preferences: model.toJson(),
      );
      return Result.success(null);
    } catch (e) {
      return Result.error(e.toString());
    }
  }

  @override
  Future<Result<void>> deleteNotification({
    required String notificationId,
  }) async {
    try {
      await remoteDatasource.deleteNotification(notificationId: notificationId);
      return Result.success(null);
    } catch (e) {
      return Result.error(e.toString());
    }
  }

  @override
  Future<Result<void>> deleteAllNotifications({required String userId}) async {
    try {
      await remoteDatasource.deleteAllNotifications(userId: userId);
      return Result.success(null);
    } catch (e) {
      return Result.error(e.toString());
    }
  }

  @override
  Future<Result<int>> deleteReadNotifications({required String userId}) async {
    try {
      final count = await remoteDatasource.deleteReadNotifications(
        userId: userId,
      );
      return Result.success(count);
    } catch (e) {
      return Result.error(e.toString());
    }
  }
}
