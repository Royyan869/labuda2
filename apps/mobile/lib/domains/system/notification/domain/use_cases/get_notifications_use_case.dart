/// Get Notifications Use Case
///
/// Stream-based use case untuk mendapatkan daftar notifications.
/// Supports real-time updates dari Firestore dengan pagination.
///
/// Size: < 150 lines (per GUIDELINES)
library;

// Dart
import 'package:labuda/core/core.dart' hide NotificationEntity;
import 'package:labuda/domains/system/notification/domain/entities/notification_entity.dart';
import 'package:labuda/domains/system/notification/domain/repositories/i_notification_repository.dart';

class GetNotificationsUseCase {
  final INotificationRepository repository;

  GetNotificationsUseCase({required this.repository});

  /// Execute use case
  ///
  /// Returns stream of notifications yang auto-update.
  /// Gunakan limit untuk pagination (default 20).
  Stream<Result<List<NotificationEntity>>> call({
    required String userId,
    int limit = 20,
  }) {
    return repository.getNotifications(userId: userId, limit: limit);
  }
}
