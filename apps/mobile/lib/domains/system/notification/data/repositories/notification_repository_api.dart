// Notification Repository API Implementation
// Implements INotificationRepository using HTTP API

// Dart
import 'dart:async';

// External
import 'package:dio/dio.dart';
import 'package:labuda/core/core.dart' hide NotificationEntity;

// Internal
import 'package:labuda/domains/system/notification/data/datasources/notification_api_datasource.dart';
import 'package:labuda/domains/system/notification/data/mappers/notification_api_mapper.dart';
import 'package:labuda/domains/system/notification/domain/entities/notification_entity.dart';
import 'package:labuda/domains/system/notification/domain/entities/notification_preference_entity.dart';
import 'package:labuda/domains/system/notification/domain/repositories/i_notification_repository.dart';

class NotificationRepositoryApi implements INotificationRepository {
  final NotificationApiDatasource _datasource;
  final ApiClient _apiClient;

  NotificationRepositoryApi(this._datasource, this._apiClient);

  // ============================================================================
  // Notification Operations
  // ============================================================================

  @override
  Stream<Result<List<NotificationEntity>>> getNotifications({
    required String userId,
    int limit = 20,
  }) async* {
    // Polling implementation (10s interval)
    // WebSocket will be implemented in Phase 3D
    while (true) {
      try {
        final response = await _datasource.listNotifications(
          perPage: limit,
          unreadOnly: false,
        );
        final entities = NotificationApiMapper.toEntityList(
          response.notifications,
        );
        yield Result.success(entities);
      } on DioException catch (e) {
        final exception = _apiClient.extractException(e);
        yield Result.error(exception.message);
      } catch (e) {
        yield Result.error('Failed to get notifications: $e');
      }
      await Future.delayed(const Duration(seconds: 10));
    }
  }

  @override
  Future<Result<void>> markAsRead({required String notificationId}) async {
    try {
      await _datasource.markNotificationsAsRead([notificationId]);
      return Result.success(null);
    } on DioException catch (e) {
      final exception = _apiClient.extractException(e);
      return Result.error(exception.message);
    } catch (e) {
      return Result.error('Failed to mark notification as read: $e');
    }
  }

  @override
  Future<Result<void>> markAllAsRead({required String userId}) async {
    try {
      await _datasource.markAllNotificationsAsRead();
      return Result.success(null);
    } on DioException catch (e) {
      final exception = _apiClient.extractException(e);
      return Result.error(exception.message);
    } catch (e) {
      return Result.error('Failed to mark all notifications as read: $e');
    }
  }

  @override
  Future<Result<void>> markAsReadByEntity({
    required String userId,
    required String entityType,
    required String entityId,
  }) async {
    try {
      await _datasource.markAsReadByEntity(
        entityType: entityType,
        entityId: entityId,
      );
      return Result.success(null);
    } on DioException catch (e) {
      final exception = _apiClient.extractException(e);
      return Result.error(exception.message);
    } catch (e) {
      return Result.error('Failed to mark notifications as read by entity: $e');
    }
  }

  @override
  Stream<Result<int>> getUnreadCount({required String userId}) async* {
    // Polling implementation (10s interval)
    // WebSocket will be implemented in Phase 3D
    while (true) {
      try {
        final unread = await _datasource.getUnreadCount();
        yield Result.success(unread.count);
      } on DioException catch (e) {
        final exception = _apiClient.extractException(e);
        yield Result.error(exception.message);
      } catch (e) {
        yield Result.error('Failed to get unread count: $e');
      }
      await Future.delayed(const Duration(seconds: 10));
    }
  }

  // ============================================================================
  // Notification Preferences
  // ============================================================================

  @override
  Future<Result<NotificationPreferenceEntity>> getPreferences({
    required String userId,
  }) async {
    try {
      final response = await _datasource.getPreferences();
      final entity = NotificationApiMapper.toPreferenceEntity(response, userId);
      return Result.success(entity);
    } on DioException catch (e) {
      final exception = _apiClient.extractException(e);
      return Result.error(exception.message);
    } catch (e) {
      return Result.error('Failed to get notification preferences: $e');
    }
  }

  @override
  Future<Result<void>> updatePreferences({
    required NotificationPreferenceEntity preferences,
  }) async {
    try {
      final request = NotificationApiMapper.toUpdateRequest(preferences);
      await _datasource.updatePreferences(request);
      return Result.success(null);
    } on DioException catch (e) {
      final exception = _apiClient.extractException(e);
      return Result.error(exception.message);
    } catch (e) {
      return Result.error('Failed to update notification preferences: $e');
    }
  }

  // ============================================================================
  // Deletion Operations
  // ============================================================================

  @override
  Future<Result<void>> deleteNotification({
    required String notificationId,
  }) async {
    try {
      await _datasource.deleteNotification(notificationId);
      return Result.success(null);
    } on DioException catch (e) {
      final exception = _apiClient.extractException(e);
      return Result.error(exception.message);
    } catch (e) {
      return Result.error('Failed to delete notification: $e');
    }
  }

  @override
  Future<Result<void>> deleteAllNotifications({required String userId}) async {
    try {
      // Backend doesn't have bulk delete endpoint yet
      // Get all notifications and delete one by one
      final response = await _datasource.listNotifications(
        perPage: 1000, // Large limit to get all
        unreadOnly: false,
      );

      // Delete each notification
      for (final notification in response.notifications) {
        await _datasource.deleteNotification(notification.id);
      }

      return Result.success(null);
    } on DioException catch (e) {
      final exception = _apiClient.extractException(e);
      return Result.error(exception.message);
    } catch (e) {
      return Result.error('Failed to delete all notifications: $e');
    }
  }

  @override
  Future<Result<int>> deleteReadNotifications({required String userId}) async {
    try {
      // Backend doesn't have bulk delete endpoint yet
      // Get all notifications and delete read ones
      final response = await _datasource.listNotifications(
        perPage: 1000, // Large limit to get all
        unreadOnly: false,
      );

      // Filter and delete read notifications
      final readNotifications = response.notifications
          .where((notification) => notification.isRead)
          .toList();

      for (final notification in readNotifications) {
        await _datasource.deleteNotification(notification.id);
      }

      return Result.success(readNotifications.length);
    } on DioException catch (e) {
      final exception = _apiClient.extractException(e);
      return Result.error(exception.message);
    } catch (e) {
      return Result.error('Failed to delete read notifications: $e');
    }
  }
}
