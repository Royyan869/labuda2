/// Delete Read Notifications Use Case
///
/// Deletes all read notifications for a user.
/// Returns the count of deleted notifications.
///
/// Size: < 50 lines (per GUIDELINES)
library;

// Dart
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/system/notification/domain/repositories/i_notification_repository.dart';

class DeleteReadNotificationsUseCase {
  final INotificationRepository repository;

  DeleteReadNotificationsUseCase({required this.repository});

  /// Execute use case
  ///
  /// Returns `Result<int>` with count of deleted notifications
  Future<Result<int>> call({required String userId}) async {
    return await repository.deleteReadNotifications(userId: userId);
  }
}
