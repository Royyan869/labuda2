/// Notification List Provider
///
/// ONE-SHOT canonical projection of GET /notifications.
///
/// Periodic refresh is NOT owned here: the ONE notification reconciliation
/// cadence (NotificationInitializer, 10s) invalidates this provider, which
/// re-reads the canonical backend page. Filtering is presentation-level over
/// this authority's value.
///
/// Size: < 150 lines (per GUIDELINES)
library;

/// Use case providers (pure Riverpod)

// Dart
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:hishumi/domains/system/notification/domain/entities/notification_entity.dart';
import 'package:hishumi/domains/system/notification/domain/use_cases/delete_all_notifications_use_case.dart';
import 'package:hishumi/domains/system/notification/domain/use_cases/delete_notification_use_case.dart';
import 'package:hishumi/domains/system/notification/domain/use_cases/delete_read_notifications_use_case.dart';
import 'package:hishumi/domains/system/notification/domain/use_cases/get_notifications_use_case.dart';
import 'package:hishumi/domains/system/notification/domain/use_cases/mark_all_as_read_use_case.dart';
import 'package:hishumi/domains/system/notification/domain/use_cases/mark_as_read_use_case.dart';
import 'package:hishumi/domains/system/notification/data/notification_providers.dart';
import 'package:hishumi/domains/system/notification/presentation/providers/unread_count_provider.dart';

final getNotificationsUseCaseProvider = Provider<GetNotificationsUseCase>((
  ref,
) {
  final repository = ref.watch(notificationRepositoryProvider);
  return GetNotificationsUseCase(repository: repository);
});

final markAsReadUseCaseProvider = Provider<MarkAsReadUseCase>((ref) {
  final repository = ref.watch(notificationRepositoryProvider);
  return MarkAsReadUseCase(repository: repository);
});

final markAllAsReadUseCaseProvider = Provider<MarkAllAsReadUseCase>((ref) {
  final repository = ref.watch(notificationRepositoryProvider);
  return MarkAllAsReadUseCase(repository: repository);
});

final deleteNotificationUseCaseProvider = Provider<DeleteNotificationUseCase>((
  ref,
) {
  final repository = ref.watch(notificationRepositoryProvider);
  return DeleteNotificationUseCase(repository: repository);
});

final deleteAllNotificationsUseCaseProvider =
    Provider<DeleteAllNotificationsUseCase>((ref) {
      final repository = ref.watch(notificationRepositoryProvider);
      return DeleteAllNotificationsUseCase(repository: repository);
    });

final deleteReadNotificationsUseCaseProvider =
    Provider<DeleteReadNotificationsUseCase>((ref) {
      final repository = ref.watch(notificationRepositoryProvider);
      return DeleteReadNotificationsUseCase(repository: repository);
    });

/// Notification list provider — one-shot canonical read per build.
final notificationListProvider = FutureProvider.family<
  List<NotificationEntity>,
  String
>((ref, userId) async {
  final useCase = ref.watch(getNotificationsUseCaseProvider);

  final result = await useCase(userId: userId, limit: 20);
  return result.fold(
    (error) => throw Exception(error),
    (notifications) => notifications,
  );
});

// =============================================================================
// MUTATION CLOSURES — backend response is authority
//
// Every successful mutation returns the canonical post-mutation
// `unread_count` from the backend (Task 1 contract). On success the closure
// invalidates BOTH the unread-count provider and the list provider so the
// badge and the list immediately re-read that same canonical authority —
// no local decrement, no optimistic arithmetic, no waiting for the next
// poll tick. On failure nothing is invalidated: the badge and the list keep
// showing the last known backend state.
// =============================================================================

/// THE canonical notification reconciliation path (Phase 4.3).
///
/// Invalidates the two canonical notification providers so they re-read
/// backend state (GET /notifications/unread-count, GET /notifications).
/// Every convergence trigger — realtime events, mutation closures, chat-room
/// read, app resume, WebSocket reconnect — routes through THIS function.
/// No payload, no arithmetic, no local authority.
///
/// The invalidate callback is typed `dynamic` because the parameter type of
/// `Ref.invalidate`/`WidgetRef.invalidate` (ProviderOrFamily) is not part of
/// the public Flutter Riverpod export surface in this version; the tear-off
/// is safely assignable via dynamic contravariance and both the Ref and
/// WidgetRef worlds call the same single implementation.
void reconcileNotificationState({
  required void Function(dynamic) invalidate,
  required String userId,
}) {
  if (userId.isEmpty) return; // fail closed without an authenticated scope
  invalidate(unreadCountProvider(userId));
  invalidate(notificationListProvider(userId));
}

/// Converges count + list state after a successful mutation whose backend
/// response carried the canonical post-mutation unread count.
void _convergeAfterMutation(Ref ref, String userId) {
  reconcileNotificationState(
    invalidate: (provider) => ref.invalidate(provider),
    userId: userId,
  );
}

/// Mark notification as read.
///
/// Signature: (notificationId, userId). Success applies the backend
/// response count via canonical re-read; failure throws and changes nothing.
final markNotificationAsReadProvider =
    Provider<Future<void> Function(String, String)>((ref) {
      final useCase = ref.watch(markAsReadUseCaseProvider);

      return (String notificationId, String userId) async {
        final result = await useCase(notificationId: notificationId);
        result.fold((error) => throw Exception(error), (unreadCount) {
          _convergeAfterMutation(ref, userId);
        });
      };
    });

/// Mark all notifications as read.
///
/// Success sets the badge to the backend response count (normally 0) via
/// canonical re-read; failure throws and never forces the badge to 0.
final markAllNotificationsAsReadProvider =
    Provider<Future<void> Function(String)>((ref) {
      final useCase = ref.watch(markAllAsReadUseCaseProvider);

      return (String userId) async {
        final result = await useCase(userId: userId);
        result.fold((error) => throw Exception(error), (unreadCount) {
          _convergeAfterMutation(ref, userId);
        });
      };
    });

/// Delete notification.
///
/// Signature: (notificationId, userId). Success applies the backend response
/// count (honoured even when the deleted item was already read); failure
/// throws and changes nothing.
final deleteNotificationProvider =
    Provider<Future<void> Function(String, String)>((ref) {
      final useCase = ref.watch(deleteNotificationUseCaseProvider);

      return (String notificationId, String userId) async {
        final result = await useCase(notificationId: notificationId);
        result.fold((error) => throw Exception(error), (unreadCount) {
          _convergeAfterMutation(ref, userId);
        });
      };
    });

/// Delete all notifications.
///
/// Success converges count + list to the post-deletion backend state;
/// failure throws and changes nothing.
final deleteAllNotificationsProvider =
    Provider<Future<void> Function(String)>((ref) {
      final useCase = ref.watch(deleteAllNotificationsUseCaseProvider);

      return (String userId) async {
        final result = await useCase(userId: userId);
        result.fold((error) => throw Exception(error), (_) {
          _convergeAfterMutation(ref, userId);
        });
      };
    });

/// Delete read notifications.
///
/// Returns the deleted count (existing contract). Success also converges
/// count + list; failure throws and changes nothing.
final deleteReadNotificationsProvider =
    Provider<Future<int> Function(String)>((ref) {
      final useCase = ref.watch(deleteReadNotificationsUseCaseProvider);

      return (String userId) async {
        final result = await useCase(userId: userId);
        return result.fold((error) => throw Exception(error), (deletedCount) {
          _convergeAfterMutation(ref, userId);
          return deletedCount;
        });
      };
    });
