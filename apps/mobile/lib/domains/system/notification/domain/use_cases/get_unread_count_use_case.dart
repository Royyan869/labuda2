/// Get Unread Count Use Case
///
/// Stream-based use case untuk mendapatkan jumlah unread notifications.
/// Returns real-time count yang auto-update.
///
/// Size: < 100 lines (per GUIDELINES)
library;

// Dart
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/system/notification/domain/repositories/i_notification_repository.dart';

class GetUnreadCountUseCase {
  final INotificationRepository repository;

  GetUnreadCountUseCase({required this.repository});

  /// Execute use case
  ///
  /// Returns stream of unread notification count yang auto-update.
  Stream<Result<int>> call({required String userId}) {
    return repository.getUnreadCount(userId: userId);
  }
}
