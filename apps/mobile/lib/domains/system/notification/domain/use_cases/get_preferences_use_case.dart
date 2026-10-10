/// Get Preferences Use Case
///
/// Get user notification preferences.
///
/// Size: < 100 lines (per GUIDELINES)
library;

// Dart
import 'package:hishumi/core/core.dart';
import 'package:hishumi/domains/system/notification/domain/entities/notification_preference_entity.dart';
import 'package:hishumi/domains/system/notification/domain/repositories/i_notification_repository.dart';

class GetPreferencesUseCase {
  final INotificationRepository repository;

  GetPreferencesUseCase({required this.repository});

  /// Execute use case
  ///
  /// Gets notification preferences untuk user.
  Future<Result<NotificationPreferenceEntity>> call({required String userId}) {
    return repository.getPreferences(userId: userId);
  }
}
