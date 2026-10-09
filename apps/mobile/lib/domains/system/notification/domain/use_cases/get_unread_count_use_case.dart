/// Get Unread Count Use Case
///
/// One-shot use case untuk mendapatkan jumlah unread notifications
/// (canonical backend read). Periodic refresh dimiliki oleh NOTIFICATION
/// RECONCILIATION CADENCE di NotificationInitializer — bukan use case ini.
///
/// Size: < 100 lines (per GUIDELINES)
library;

// Dart
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/system/notification/domain/repositories/i_notification_repository.dart';

class GetUnreadCountUseCase {
  final INotificationRepository repository;

  GetUnreadCountUseCase({required this.repository});

  /// Execute use case — ONE-SHOT canonical count read.
  Future<Result<int>> call({required String userId}) {
    return repository.getUnreadCount(userId: userId);
  }
}
