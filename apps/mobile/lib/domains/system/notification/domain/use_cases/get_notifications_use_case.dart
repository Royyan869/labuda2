/// Get Notifications Use Case
///
/// One-shot use case untuk mendapatkan daftar notifications (canonical
/// backend read). Periodic refresh dimiliki oleh NOTIFICATION
/// RECONCILIATION CADENCE di NotificationInitializer — bukan use case ini.
///
/// Size: < 150 lines (per GUIDELINES)
library;

// Dart
import 'package:hishumi/core/core.dart' hide NotificationEntity;
import 'package:hishumi/domains/system/notification/domain/entities/notification_entity.dart';
import 'package:hishumi/domains/system/notification/domain/repositories/i_notification_repository.dart';

class GetNotificationsUseCase {
  final INotificationRepository repository;

  GetNotificationsUseCase({required this.repository});

  /// Execute use case — ONE-SHOT canonical page read.
  ///
  /// Gunakan limit untuk pagination (default 20).
  Future<Result<List<NotificationEntity>>> call({
    required String userId,
    int limit = 20,
  }) {
    return repository.getNotifications(userId: userId, limit: limit);
  }
}
