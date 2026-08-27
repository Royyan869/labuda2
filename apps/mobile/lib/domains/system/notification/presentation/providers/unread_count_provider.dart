/// Unread Count Provider
///
/// Stream provider untuk real-time unread notification count.
///
/// Size: < 100 lines (per GUIDELINES)
library;

/// Use case provider (pure Riverpod)

// Dart
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/domains/system/notification/domain/use_cases/get_unread_count_use_case.dart';
import 'package:labuda/domains/system/notification/data/notification_providers.dart';

final getUnreadCountUseCaseProvider = Provider<GetUnreadCountUseCase>((ref) {
  final repository = ref.watch(notificationRepositoryProvider);
  return GetUnreadCountUseCase(repository: repository);
});

/// Unread count stream provider
final unreadCountProvider = StreamProvider.family<int, String>((ref, userId) {
  final useCase = ref.watch(getUnreadCountUseCaseProvider);

  return useCase(userId: userId).map((result) {
    return result.fold(
      (error) => 0, // Return 0 on error
      (count) => count,
    );
  });
});
