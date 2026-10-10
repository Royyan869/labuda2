/// Delete Notification Use Case
///
/// Deletes a single notification from user's notification list.
///
/// Size: < 50 lines (per GUIDELINES)
library;

// Dart
import 'package:hishumi/core/core.dart';
import 'package:hishumi/domains/system/notification/domain/repositories/i_notification_repository.dart';

class DeleteNotificationUseCase {
  final INotificationRepository repository;

  DeleteNotificationUseCase({required this.repository});

  /// Execute use case
  ///
  /// Returns `Result<int>` carrying the canonical post-mutation
  /// `unread_count` from the backend.
  Future<Result<int>> call({required String notificationId}) async {
    return await repository.deleteNotification(notificationId: notificationId);
  }
}
