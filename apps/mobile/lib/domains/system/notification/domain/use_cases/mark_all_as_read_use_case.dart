/// Mark All As Read Use Case
///
/// Mark all notifications untuk user sebagai read.
///
/// Size: < 100 lines (per GUIDELINES)
library;

// Dart
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/system/notification/domain/repositories/i_notification_repository.dart';

class MarkAllAsReadUseCase {
  final INotificationRepository repository;

  MarkAllAsReadUseCase({required this.repository});

  /// Execute use case
  ///
  /// Marks semua notifications untuk userId sebagai read.
  Future<Result<void>> call({required String userId}) {
    return repository.markAllAsRead(userId: userId);
  }
}
