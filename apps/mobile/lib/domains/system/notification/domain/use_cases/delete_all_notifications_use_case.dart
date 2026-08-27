/// Delete All Notifications Use Case
///
/// Deletes all notifications for a user.
///
/// Size: < 50 lines (per GUIDELINES)
library;

// Dart
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/system/notification/domain/repositories/i_notification_repository.dart';

class DeleteAllNotificationsUseCase {
  final INotificationRepository repository;

  DeleteAllNotificationsUseCase({required this.repository});

  /// Execute use case
  ///
  /// Returns `Result<void>` indicating success or failure
  Future<Result<void>> call({required String userId}) async {
    return await repository.deleteAllNotifications(userId: userId);
  }
}
