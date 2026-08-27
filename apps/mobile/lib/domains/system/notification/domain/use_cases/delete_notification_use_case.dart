/// Delete Notification Use Case
///
/// Deletes a single notification from user's notification list.
///
/// Size: < 50 lines (per GUIDELINES)
library;

// Dart
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/system/notification/domain/repositories/i_notification_repository.dart';

class DeleteNotificationUseCase {
  final INotificationRepository repository;

  DeleteNotificationUseCase({required this.repository});

  /// Execute use case
  ///
  /// Returns `Result<void>` indicating success or failure
  Future<Result<void>> call({required String notificationId}) async {
    return await repository.deleteNotification(notificationId: notificationId);
  }
}
