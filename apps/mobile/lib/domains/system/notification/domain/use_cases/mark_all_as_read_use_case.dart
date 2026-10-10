/// Mark All As Read Use Case
///
/// Mark all notifications untuk user sebagai read.
///
/// Size: < 100 lines (per GUIDELINES)
library;

// Dart
import 'package:hishumi/core/core.dart';
import 'package:hishumi/domains/system/notification/domain/repositories/i_notification_repository.dart';

class MarkAllAsReadUseCase {
  final INotificationRepository repository;

  MarkAllAsReadUseCase({required this.repository});

  /// Execute use case
  ///
  /// Marks semua notifications untuk userId sebagai read.
  /// Returns the canonical post-mutation `unread_count` from the backend.
  Future<Result<int>> call({required String userId}) {
    return repository.markAllAsRead(userId: userId);
  }
}
