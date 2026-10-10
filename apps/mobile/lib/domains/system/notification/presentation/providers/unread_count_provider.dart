/// Unread Count Provider
///
/// ONE-SHOT canonical projection of GET /notifications/unread-count.
///
/// Periodic refresh is NOT owned here: the ONE notification reconciliation
/// cadence (NotificationInitializer, 10s) invalidates this provider, which
/// re-reads the canonical backend count. Primary paths (realtime events,
/// resume/reconnect, mutations) invalidate through the same canonical
/// [reconcileNotificationState] function.
///
/// Size: < 100 lines (per GUIDELINES)
library;

/// Use case provider (pure Riverpod)

// Dart
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:hishumi/domains/system/notification/domain/use_cases/get_unread_count_use_case.dart';
import 'package:hishumi/domains/system/notification/data/notification_providers.dart';

final getUnreadCountUseCaseProvider = Provider<GetUnreadCountUseCase>((ref) {
  final repository = ref.watch(notificationRepositoryProvider);
  return GetUnreadCountUseCase(repository: repository);
});

/// Unread count provider — one-shot canonical read per build.
///
/// ERROR SEMANTICS: a failed fetch THROWS — it is never folded into a
/// number. `0` is only ever a backend-confirmed value (CountUnread
/// authority). On a subsequent failed rebuild the provider enters
/// AsyncError while Riverpod retains the previous successful `.value`, so
/// consumers rendering `value` keep the last backend-confirmed count; an
/// initial failure yields `value == null` (no fabricated number).
final unreadCountProvider = FutureProvider.family<int, String>((
  ref,
  userId,
) async {
  final useCase = ref.watch(getUnreadCountUseCaseProvider);

  final result = await useCase(userId: userId);
  return result.fold(
    (error) => throw Exception(error),
    (count) => count,
  );
});
