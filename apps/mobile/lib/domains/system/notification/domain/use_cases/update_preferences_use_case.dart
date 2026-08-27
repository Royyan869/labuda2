/// Update Preferences Use Case
///
/// Update user notification preferences.
///
/// Size: < 100 lines (per GUIDELINES)
library;

// Dart
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/system/notification/domain/entities/notification_preference_entity.dart';
import 'package:labuda/domains/system/notification/domain/repositories/i_notification_repository.dart';

class UpdatePreferencesUseCase {
  final INotificationRepository repository;

  UpdatePreferencesUseCase({required this.repository});

  /// Execute use case
  ///
  /// Updates notification preferences untuk user.
  Future<Result<void>> call({
    required NotificationPreferenceEntity preferences,
  }) {
    return repository.updatePreferences(preferences: preferences);
  }
}
