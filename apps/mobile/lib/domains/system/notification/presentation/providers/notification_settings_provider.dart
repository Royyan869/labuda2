/// Notification Settings Provider
///
/// Provider untuk user notification preferences.
///
/// Size: < 150 lines (per GUIDELINES)
library;

/// Use case providers (pure Riverpod)

// Dart
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/domains/system/notification/domain/entities/notification_preference_entity.dart';
import 'package:labuda/domains/system/notification/domain/use_cases/get_preferences_use_case.dart';
import 'package:labuda/domains/system/notification/domain/use_cases/update_preferences_use_case.dart';
import 'package:labuda/domains/system/notification/data/notification_providers.dart';

final getPreferencesUseCaseProvider = Provider<GetPreferencesUseCase>((ref) {
  final repository = ref.watch(notificationRepositoryProvider);
  return GetPreferencesUseCase(repository: repository);
});

final updatePreferencesUseCaseProvider = Provider<UpdatePreferencesUseCase>((
  ref,
) {
  final repository = ref.watch(notificationRepositoryProvider);
  return UpdatePreferencesUseCase(repository: repository);
});

/// Notification preferences provider
final notificationPreferencesProvider =
    FutureProvider.family<NotificationPreferenceEntity, String>((
      ref,
      userId,
    ) async {
      final useCase = ref.watch(getPreferencesUseCaseProvider);

      final result = await useCase(userId: userId);
      return result.fold(
        (error) => throw Exception(error),
        (preferences) => preferences,
      );
    });

/// Update preferences action
final updateNotificationPreferencesProvider =
    Provider<Future<void> Function(NotificationPreferenceEntity)>((ref) {
      final useCase = ref.watch(updatePreferencesUseCaseProvider);

      return (NotificationPreferenceEntity preferences) async {
        final result = await useCase(preferences: preferences);
        result.fold((error) => throw Exception(error), (_) {
          // Invalidate preferences provider to refresh
          ref.invalidate(notificationPreferencesProvider);
        });
      };
    });
