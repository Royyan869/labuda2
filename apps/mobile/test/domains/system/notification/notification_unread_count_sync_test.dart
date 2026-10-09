// TASK 2 — Mobile Notification Read/Unread Count Sync.
//
// Proves the critical rule: after a SUCCESSFUL mutation, the mobile unread
// count and the notification list converge immediately to the backend's
// canonical post-mutation state — without waiting for the 10s poll tick, and
// without any local decrement. A FAILED mutation changes nothing.
//
// Architecture under test (production code):
//   mutation closure (notification_list_provider.dart)
//     → repository returns Result<int> = response.unread_count (Task 1 contract)
//     → success: invalidate unreadCountProvider + notificationListProvider
//     → failure: throw, no invalidation, no state change
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/common/result.dart';
import 'package:labuda/domains/system/notification/data/notification_providers.dart';
import 'package:labuda/domains/system/notification/domain/entities/notification_entity.dart';
import 'package:labuda/domains/system/notification/domain/entities/notification_preference_entity.dart';
import 'package:labuda/domains/system/notification/domain/repositories/i_notification_repository.dart';
import 'package:labuda/domains/system/notification/presentation/providers/notification_list_provider.dart';
import 'package:labuda/domains/system/notification/presentation/providers/unread_count_provider.dart';
import 'package:labuda/domains/system/notification/presentation/widgets/notification_badge_widget.dart';
import 'package:labuda/shared/widgets/count_badge.dart';

const _uid = 'u1';

/// Scripted canonical backend.
///
/// Mirrors the Task 1 backend truth: a mutation response's `unread_count`
/// and the canonical GET /notifications/unread-count always agree, because
/// both read the same database state AFTER the mutation. Tests script the
/// backend transition via [nextMutationCount].
class _ScriptedBackend implements INotificationRepository {
  _ScriptedBackend({this.unread = 3});

  /// Canonical unread count served by getUnreadCount.
  int unread;

  /// The count the next mutation returns — and the state the "backend"
  /// moves to. Null = mutation returns the current state unchanged
  /// (e.g. marking an already-read notification).
  int? nextMutationCount;

  bool failMutations = false;

  int listCalls = 0;
  int countSubscriptions = 0;

  Future<Result<int>> _mutate() async {
    if (failMutations) return Result.error('boom-mutation');
    final count = nextMutationCount ?? unread;
    unread = count;
    nextMutationCount = null;
    return Result.success(count);
  }

  @override
  Future<Result<int>> getUnreadCount({required String userId}) async {
    countSubscriptions++;
    return Result.success(unread);
  }

  @override
  Future<Result<List<NotificationEntity>>> getNotifications({
    required String userId,
    int limit = 20,
  }) async {
    listCalls++;
    return Result.success(const <NotificationEntity>[]);
  }

  @override
  Future<Result<int>> markAsRead({required String notificationId}) => _mutate();

  @override
  Future<Result<int>> markAllAsRead({required String userId}) => _mutate();

  @override
  Future<Result<int>> deleteNotification({required String notificationId}) =>
      _mutate();

  @override
  Future<Result<void>> markAsReadByEntity({
    required String userId,
    required String entityType,
    required String entityId,
  }) async => Result.success(null);

  @override
  Future<Result<int>> deleteReadNotifications({required String userId}) async =>
      Result.success(0);

  @override
  Future<Result<void>> deleteAllNotifications({required String userId}) async =>
      Result.success(null);

  @override
  Future<Result<NotificationPreferenceEntity>> getPreferences({
    required String userId,
  }) => throw UnimplementedError();

  @override
  Future<Result<void>> updatePreferences({
    required NotificationPreferenceEntity preferences,
  }) => throw UnimplementedError();
}

ProviderContainer _container(_ScriptedBackend backend) {
  final container = ProviderContainer(
    overrides: [notificationRepositoryProvider.overrideWithValue(backend)],
  );
  addTearDown(container.dispose);
  return container;
}

/// Activates (listens to) both canonical providers and waits for their first
/// emission. Riverpod only builds a FutureProvider while it has a listener —
/// so `listen` (not bare `read(.future)`) is the activation call.
Future<void> _prime(
  ProviderContainer container,
  _ScriptedBackend backend,
) async {
  container.listen(unreadCountProvider(_uid), (_, _) {});
  container.listen(notificationListProvider(_uid), (_, _) {});
  await pumpEventQueue();
  expect(container.read(unreadCountProvider(_uid)).value, backend.unread);
  expect(backend.countSubscriptions, 1);
  expect(backend.listCalls, 1);
}

void main() {
  group('Task 2 — mutation success converges count + list immediately', () {
    test('Case 1: mark one read — count 3 → 2, list re-read, no poll wait', () async {
      final backend = _ScriptedBackend(unread: 3)..nextMutationCount = 2;
      final container = _container(backend);
      await _prime(container, backend);
      final countSubsBefore = backend.countSubscriptions;
      final listCallsBefore = backend.listCalls;

      await container.read(markNotificationAsReadProvider)('n1', _uid);
      await pumpEventQueue();

      expect(container.read(unreadCountProvider(_uid)).value, 2);
      expect(backend.countSubscriptions, countSubsBefore + 1,
          reason: 'count provider must re-read the canonical authority immediately');
      expect(backend.listCalls, listCallsBefore + 1,
          reason: 'list must be invalidated and re-read immediately');
    });

    test('Case 2: mark already-read — count stays 3, no local decrement', () async {
      final backend = _ScriptedBackend(unread: 3); // nextMutationCount null → unchanged
      final container = _container(backend);
      await _prime(container, backend);

      await container.read(markNotificationAsReadProvider)('n1', _uid);
      await pumpEventQueue();

      expect(container.read(unreadCountProvider(_uid)).value, 3,
          reason: 'backend reported 3 — mobile must never decrement locally');
    });

    test('Case 3: mark all read — badge immediately 0, list re-read', () async {
      final backend = _ScriptedBackend(unread: 4)..nextMutationCount = 0;
      final container = _container(backend);
      await _prime(container, backend);
      final listCallsBefore = backend.listCalls;

      await container.read(markAllNotificationsAsReadProvider)(_uid);
      await pumpEventQueue();

      expect(container.read(unreadCountProvider(_uid)).value, 0);
      expect(backend.listCalls, listCallsBefore + 1);
    });

    test('Case 4: delete unread — count 3 → 2, list re-read', () async {
      final backend = _ScriptedBackend(unread: 3)..nextMutationCount = 2;
      final container = _container(backend);
      await _prime(container, backend);
      final listCallsBefore = backend.listCalls;

      await container.read(deleteNotificationProvider)('n1', _uid);
      await pumpEventQueue();

      expect(container.read(unreadCountProvider(_uid)).value, 2);
      expect(backend.listCalls, listCallsBefore + 1,
          reason: 'deleted item must disappear from the list via re-read');
    });

    test('Case 5: delete already-read — count stays 3, list still re-read', () async {
      final backend = _ScriptedBackend(unread: 3);
      final container = _container(backend);
      await _prime(container, backend);
      final listCallsBefore = backend.listCalls;

      await container.read(deleteNotificationProvider)('n1', _uid);
      await pumpEventQueue();

      expect(container.read(unreadCountProvider(_uid)).value, 3,
          reason: 'response count is honoured even when the item was read');
      expect(backend.listCalls, listCallsBefore + 1,
          reason: 'the deleted row must still leave the list');
    });
  });

  group('Task 2 — mutation failure never fabricates state', () {
    test('Case 6: failed mark-read — count untouched, no provider re-read', () async {
      final backend = _ScriptedBackend(unread: 3)..failMutations = true;
      final container = _container(backend);
      await _prime(container, backend);
      final countSubsBefore = backend.countSubscriptions;
      final listCallsBefore = backend.listCalls;

      await expectLater(
        container.read(markNotificationAsReadProvider)('n1', _uid),
        throwsA(isA<Exception>()),
      );

      expect(container.read(unreadCountProvider(_uid)).value, 3,
          reason: 'badge must keep the last known backend state');
      expect(backend.countSubscriptions, countSubsBefore,
          reason: 'a failed mutation must not trigger a count re-read');
      expect(backend.listCalls, listCallsBefore,
          reason: 'a failed mutation must not refresh the list');
    });

    test('Case 6: failed mark-all — badge is never forced to 0', () async {
      final backend = _ScriptedBackend(unread: 3)..failMutations = true;
      final container = _container(backend);
      await _prime(container, backend);

      await expectLater(
        container.read(markAllNotificationsAsReadProvider)(_uid),
        throwsA(isA<Exception>()),
      );

      expect(container.read(unreadCountProvider(_uid)).value, 3);
    });

    test('Case 6: failed delete — count and list untouched', () async {
      final backend = _ScriptedBackend(unread: 3)..failMutations = true;
      final container = _container(backend);
      await _prime(container, backend);
      final listCallsBefore = backend.listCalls;

      await expectLater(
        container.read(deleteNotificationProvider)('n1', _uid),
        throwsA(isA<Exception>()),
      );

      expect(container.read(unreadCountProvider(_uid)).value, 3);
      expect(backend.listCalls, listCallsBefore);
    });
  });

  group('Task 2 — badge UI convergence (production widget)', () {
    testWidgets('badge reflects the backend post-mutation count immediately', (
      tester,
    ) async {
      final backend = _ScriptedBackend(unread: 3)..nextMutationCount = 2;

      late WidgetRef capturedRef;
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            notificationRepositoryProvider.overrideWithValue(backend),
          ],
          child: MaterialApp(
            home: Scaffold(
              body: Consumer(
                builder: (context, ref, _) {
                  capturedRef = ref;
                  return NotificationBadgeWidget(
                    userId: _uid,
                    child: const SizedBox.shrink(),
                  );
                },
              ),
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();

      // Prime: badge shows the canonical 3.
      final overlay = tester.widget<CountBadgeOverlay>(
        find.byType(CountBadgeOverlay),
      );
      expect(overlay.count, 3);

      // Successful mark-read via the production closure.
      await capturedRef.read(markNotificationAsReadProvider)('n1', _uid);
      await tester.pumpAndSettle();

      final converged = tester.widget<CountBadgeOverlay>(
        find.byType(CountBadgeOverlay),
      );
      expect(converged.count, 2,
          reason: 'badge must show the backend post-mutation count without polling');
    });
  });
}
