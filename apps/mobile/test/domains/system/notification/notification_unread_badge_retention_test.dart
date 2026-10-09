// FIX — FALSE-ZERO NOTIFICATION UNREAD BADGE.
//
// Proves the Owner decision end-to-end against production code:
//   a failed unread-count fetch NEVER becomes a confirmed `0`; the badge
//   keeps the LAST BACKEND-CONFIRMED count; `0` appears only when the
//   backend (CountUnread authority) confirms it; an initial failure invents
//   nothing; values never leak across accounts.
//
// Cases (per task spec):
//   A initial success 3        → 3
//   B subsequent error         → stays 3 (never 0)
//   C error then success 5     → 5
//   D backend-confirmed 0      → 0
//   E initial error            → value null (no fabricated number)
//   isolation                  → user B initial error never sees user A's 5
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart' hide NotificationEntity;
import 'package:labuda/domains/system/notification/data/notification_providers.dart';
import 'package:labuda/domains/system/notification/domain/entities/notification_entity.dart';
import 'package:labuda/domains/system/notification/domain/repositories/i_notification_repository.dart';
import 'package:labuda/domains/system/notification/presentation/providers/unread_count_provider.dart';
import 'package:labuda/domains/system/notification/presentation/widgets/notification_badge_widget.dart';
import 'package:labuda/shared/widgets/count_badge.dart';

const _uidA = 'user-a';
const _uidB = 'user-b';

/// Scripted canonical backend: serves the CURRENT unread state or fails on
/// demand. No arithmetic authority lives here — it only reports what the
/// "backend" says.
class _ScriptedRepository implements INotificationRepository {
  _ScriptedRepository({this.unread = 0});

  int unread;
  bool failCount = false;
  int countCalls = 0;

  @override
  Future<Result<int>> getUnreadCount({required String userId}) async {
    countCalls++;
    if (failCount) return Result.error('boom-unread-count');
    return Result.success(unread);
  }

  @override
  Future<Result<List<NotificationEntity>>> getNotifications({
    required String userId,
    int limit = 20,
  }) async {
    return Result.success(const <NotificationEntity>[]);
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => null;
}

({ProviderContainer container, _ScriptedRepository repo})
_harness({int? initialUnread}) {
  final repo = _ScriptedRepository(unread: initialUnread ?? 0);
  final container = ProviderContainer(
    overrides: [notificationRepositoryProvider.overrideWithValue(repo)],
  );
  addTearDown(container.dispose);
  return (container: container, repo: repo);
}

void main() {
  group('false-zero fix — provider value semantics', () {
    test('Case A: initial success 3 → value 3', () async {
      final (:container, :repo) = _harness(initialUnread: 3);
      container.listen(unreadCountProvider(_uidA), (_, _) {});
      await pumpEventQueue();

      expect(repo.countCalls, 1);
      expect(container.read(unreadCountProvider(_uidA)).value, 3);
      expect(container.read(unreadCountProvider(_uidA)).hasError, isFalse);
    });

    test('Case B: subsequent error keeps last confirmed 3 — never 0', () async {
      final (:container, :repo) = _harness(initialUnread: 3);
      container.listen(unreadCountProvider(_uidA), (_, _) {});
      await pumpEventQueue();
      expect(container.read(unreadCountProvider(_uidA)).value, 3);

      // Backend now fails; the cadence/realtime/mutation path invalidates.
      repo.failCount = true;
      container.invalidate(unreadCountProvider(_uidA));
      await pumpEventQueue();

      final state = container.read(unreadCountProvider(_uidA));
      expect(state.hasError, isTrue, reason: 'the failure must surface as error');
      expect(state.value, 3,
          reason: 'the last backend-confirmed count must be retained');
      expect(state.value, isNot(0),
          reason: 'a failed fetch must never be presented as confirmed 0');
    });

    test('Case C: success after failure updates to 5', () async {
      final (:container, :repo) = _harness(initialUnread: 3);
      container.listen(unreadCountProvider(_uidA), (_, _) {});
      await pumpEventQueue();

      repo.failCount = true;
      container.invalidate(unreadCountProvider(_uidA));
      await pumpEventQueue();
      expect(container.read(unreadCountProvider(_uidA)).value, 3);

      repo.failCount = false;
      repo.unread = 5;
      container.invalidate(unreadCountProvider(_uidA));
      await pumpEventQueue();

      final state = container.read(unreadCountProvider(_uidA));
      expect(state.hasError, isFalse);
      expect(state.value, 5, reason: 'recovery must apply the fresh authority');
    });

    test('Case D: backend-confirmed 0 becomes 0 (not the old value)', () async {
      final (:container, :repo) = _harness(initialUnread: 3);
      container.listen(unreadCountProvider(_uidA), (_, _) {});
      await pumpEventQueue();
      expect(container.read(unreadCountProvider(_uidA)).value, 3);

      // The user actually read everything — CountUnread confirms 0.
      repo.unread = 0;
      container.invalidate(unreadCountProvider(_uidA));
      await pumpEventQueue();

      final state = container.read(unreadCountProvider(_uidA));
      expect(state.hasError, isFalse);
      expect(state.value, 0,
          reason: 'a backend-confirmed 0 must win over the retained 3');
    });

    test('Case E: initial failure invents nothing (value stays null)', () async {
      final (:container, :repo) = _harness();
      repo.failCount = true;
      container.listen(unreadCountProvider(_uidA), (_, _) {});
      await pumpEventQueue();

      final state = container.read(unreadCountProvider(_uidA));
      expect(state.hasError, isTrue);
      expect(state.value, isNull,
          reason: 'without a prior confirmed read there is no last-known '
              'value to retain — and none may be fabricated');
    });

    test('account isolation: user B initial error never sees user A 5', () async {
      final (:container, :repo) = _harness(initialUnread: 5);
      // Both scopes active at once — the real badge situation right after an
      // account switch rebuilds the consumer under user B while user A's
      // family instance may still be lingering in the container.
      container.listen(unreadCountProvider(_uidA), (_, _) {});
      await pumpEventQueue();
      expect(container.read(unreadCountProvider(_uidA)).value, 5);

      // B's fresh family instance starts and its FIRST read fails.
      repo.failCount = true;
      container.listen(unreadCountProvider(_uidB), (_, _) {});
      await pumpEventQueue();

      final bState = container.read(unreadCountProvider(_uidB));
      expect(bState.value, isNull,
          reason: 'user B must never inherit user A last-known count');
      expect(bState.hasError, isTrue);
      // A's own scope still holds its own 5 — instances are per-user keyed.
      expect(container.read(unreadCountProvider(_uidA)).value, 5,
          reason: 'family instances are isolated per userId');
    });
  });

  group('false-zero fix — badge rendering', () {
    Future<void> pumpBadge(
      WidgetTester tester,
      _ScriptedRepository repo,
      String userId,
    ) async {
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            notificationRepositoryProvider.overrideWithValue(repo),
          ],
          child: MaterialApp(
            home: NotificationBadgeWidget(
              userId: userId,
              child: const Icon(Icons.notifications_outlined),
            ),
          ),
        ),
      );
      await tester.pump();
      await tester.pump();
    }

    testWidgets('initial error shows the plain icon — no fabricated 0 badge', (
      tester,
    ) async {
      final repo = _ScriptedRepository()..failCount = true;
      await pumpBadge(tester, repo, _uidA);

      expect(find.byType(CountBadgeOverlay), findsNothing,
          reason: 'an unread-count failure must never render a badge');
      expect(find.byIcon(Icons.notifications_outlined), findsOneWidget);
    });

    testWidgets(
      'confirmed 3 renders the badge; failed refresh KEEPS it at 3',
      (tester) async {
        final repo = _ScriptedRepository(unread: 3);
        await pumpBadge(tester, repo, _uidA);
        expect(find.byType(CountBadgeOverlay), findsOneWidget);
        expect(find.text('3'), findsOneWidget);

        // A subsequent failed cadence/realtime refresh must not drop the
        // badge NOR turn it into a confirmed 0.
        repo.failCount = true;
        final container = ProviderScope.containerOf(
          tester.element(find.byType(NotificationBadgeWidget)),
        );
        container.invalidate(unreadCountProvider(_uidA));
        await tester.pump();
        await tester.pump();

        expect(find.byType(CountBadgeOverlay), findsOneWidget,
            reason: 'the last confirmed count stays visible on failure');
        expect(find.text('3'), findsOneWidget);
        expect(find.text('0'), findsNothing);
      },
    );

    testWidgets('backend-confirmed 0 renders no badge number (plain icon)', (
      tester,
    ) async {
      final repo = _ScriptedRepository(unread: 0);
      await pumpBadge(tester, repo, _uidA);

      // CountBadgeOverlay(0) renders the plain child internally — the
      // invariant is that NO badge number is presented as confirmed state.
      expect(find.text('0'), findsNothing);
      expect(find.byIcon(Icons.notifications_outlined), findsOneWidget);
    });
  });

  group('false-zero fix — no new authority', () {
    test('provider holds no arithmetic — a failed fetch is an error, not a '
        'computed number', () async {
      final (:container, :repo) = _harness(initialUnread: 7);
      container.listen(unreadCountProvider(_uidA), (_, _) {});
      await pumpEventQueue();

      repo.failCount = true;
      container.invalidate(unreadCountProvider(_uidA));
      await pumpEventQueue();

      final state = container.read(unreadCountProvider(_uidA));
      // The retained 7 is Riverpod's previous AsyncValue — not a local
      // counter, not arithmetic, not a fallback default.
      expect(state.value, 7);
      expect(state.hasError, isTrue,
          reason: 'the failure is observable as an error state');
    });
  });
}
