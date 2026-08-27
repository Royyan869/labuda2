/// Notification List Provider
///
/// Stream-based provider untuk daftar notifications.
/// Auto-updates dari API (polling).
///
/// Size: < 150 lines (per GUIDELINES)
library;

/// Use case providers (pure Riverpod)

// Dart
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/domains/system/notification/domain/entities/notification_entity.dart';
import 'package:labuda/domains/system/notification/domain/use_cases/delete_all_notifications_use_case.dart';
import 'package:labuda/domains/system/notification/domain/use_cases/delete_notification_use_case.dart';
import 'package:labuda/domains/system/notification/domain/use_cases/delete_read_notifications_use_case.dart';
import 'package:labuda/domains/system/notification/domain/use_cases/get_notifications_use_case.dart';
import 'package:labuda/domains/system/notification/domain/use_cases/mark_all_as_read_use_case.dart';
import 'package:labuda/domains/system/notification/domain/use_cases/mark_as_read_use_case.dart';
import 'package:labuda/domains/system/notification/data/notification_providers.dart';

final getNotificationsUseCaseProvider = Provider<GetNotificationsUseCase>((
  ref,
) {
  final repository = ref.watch(notificationRepositoryProvider);
  return GetNotificationsUseCase(repository: repository);
});

final markAsReadUseCaseProvider = Provider<MarkAsReadUseCase>((ref) {
  final repository = ref.watch(notificationRepositoryProvider);
  return MarkAsReadUseCase(repository: repository);
});

final markAllAsReadUseCaseProvider = Provider<MarkAllAsReadUseCase>((ref) {
  final repository = ref.watch(notificationRepositoryProvider);
  return MarkAllAsReadUseCase(repository: repository);
});

final deleteNotificationUseCaseProvider = Provider<DeleteNotificationUseCase>((
  ref,
) {
  final repository = ref.watch(notificationRepositoryProvider);
  return DeleteNotificationUseCase(repository: repository);
});

final deleteAllNotificationsUseCaseProvider =
    Provider<DeleteAllNotificationsUseCase>((ref) {
      final repository = ref.watch(notificationRepositoryProvider);
      return DeleteAllNotificationsUseCase(repository: repository);
    });

final deleteReadNotificationsUseCaseProvider =
    Provider<DeleteReadNotificationsUseCase>((ref) {
      final repository = ref.watch(notificationRepositoryProvider);
      return DeleteReadNotificationsUseCase(repository: repository);
    });

/// Notification list stream provider
final notificationListProvider =
    StreamProvider.family<List<NotificationEntity>, String>((ref, userId) {
      final useCase = ref.watch(getNotificationsUseCaseProvider);

      return useCase(userId: userId, limit: 20).map((result) {
        return result.fold(
          (error) => throw Exception(error),
          (notifications) => notifications,
        );
      });
    });

/// Mark notification as read
final markNotificationAsReadProvider = Provider<Future<void> Function(String)>((
  ref,
) {
  final useCase = ref.watch(markAsReadUseCaseProvider);

  return (String notificationId) async {
    final result = await useCase(notificationId: notificationId);
    result.fold((error) => throw Exception(error), (_) => null);
  };
});

/// Mark all notifications as read
final markAllNotificationsAsReadProvider =
    Provider<Future<void> Function(String)>((ref) {
      final useCase = ref.watch(markAllAsReadUseCaseProvider);

      return (String userId) async {
        final result = await useCase(userId: userId);
        result.fold((error) => throw Exception(error), (_) => null);
      };
    });

/// Delete notification
final deleteNotificationProvider = Provider<Future<void> Function(String)>((
  ref,
) {
  final useCase = ref.watch(deleteNotificationUseCaseProvider);

  return (String notificationId) async {
    final result = await useCase(notificationId: notificationId);
    result.fold((error) => throw Exception(error), (_) => null);
  };
});

/// Delete all notifications for user
final deleteAllNotificationsProvider = Provider<Future<void> Function(String)>((
  ref,
) {
  final useCase = ref.watch(deleteAllNotificationsUseCaseProvider);

  return (String userId) async {
    final result = await useCase(userId: userId);
    result.fold((error) => throw Exception(error), (_) => null);
  };
});

/// Delete read notifications for user
///
/// Returns the count of deleted notifications
final deleteReadNotificationsProvider = Provider<Future<int> Function(String)>((
  ref,
) {
  final useCase = ref.watch(deleteReadNotificationsUseCaseProvider);

  return (String userId) async {
    final result = await useCase(userId: userId);
    return result.fold((error) => throw Exception(error), (count) => count);
  };
});
