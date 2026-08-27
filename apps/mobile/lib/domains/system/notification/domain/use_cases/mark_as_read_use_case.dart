/// Mark As Read Use Case
///
/// Mark single notification as read.
///
/// Size: < 100 lines (per GUIDELINES)
library;

// Dart
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/system/notification/domain/repositories/i_notification_repository.dart';

class MarkAsReadUseCase {
  final INotificationRepository repository;

  MarkAsReadUseCase({required this.repository});

  /// Execute use case
  ///
  /// Marks notification dengan ID tertentu sebagai read.
  Future<Result<void>> call({required String notificationId}) {
    return repository.markAsRead(notificationId: notificationId);
  }
}
