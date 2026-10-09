// TASK 6A — Mobile `notification.created` WebSocket Consumer.
//
// Drives the PRODUCTION consumer (notificationRealtimeSyncProvider — the
// single WS listener kept alive by the app-root NotificationInitializer)
// through the existing WebSocketService envelope, and proves:
//   - the event is an invalidation/wake-up signal ONLY: canonical
//     unreadCountProvider re-fetches (GET /unread-count);
//   - the event's unread_count is NEVER applied as state (Case 2 critical);
//   - notificationListProvider re-fetches so new notifications appear;
//   - malformed frames / stream errors never kill the subscription;
//   - chat WS events are untouched by the notification consumer;
//   - exactly ONE WS subscription and ONE reconciliation per event.
//
// NOTE: written as plain `test()` + ProviderContainer — the same proven
// mechanism as the Task 2 notification convergence suite in this domain.
import 'dart:async';
import 'dart:io';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart' hide NotificationEntity;
import 'package:labuda/core/websocket/websocket_message.dart';
import 'package:labuda/domains/system/notification/data/notification_providers.dart';
import 'package:labuda/domains/system/notification/domain/entities/notification_entity.dart';
import 'package:labuda/domains/system/notification/domain/repositories/i_notification_repository.dart';
import 'package:labuda/domains/system/notification/presentation/providers/notification_list_provider.dart';
import 'package:labuda/domains/system/notification/presentation/providers/notification_realtime_sync_provider.dart';
import 'package:labuda/domains/system/notification/presentation/providers/unread_count_provider.dart';
import 'package:labuda/shared/providers/auth_status_providers.dart';

const _userId = 'u1';

/// Scripted canonical backend: serves the CURRENT canonical unread/list state
/// and records every subscription so invalidation-driven refetches are
/// observable. The event payload never touches this fake — only canonical
/// reads do.
class _ScriptedNotificationRepository implements INotificationRepository {
  _ScriptedNotificationRepository({this.unread = 0});

  int unread;
  List<NotificationEntity> notifications = const [];
  int countSubscriptions = 0;
  int listCalls = 0;

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
    return Result.success(notifications);
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => null;
}

class _FakeWebSocketService extends WebSocketService {
  _FakeWebSocketService() : super(baseUrl: 'ws://example.invalid');

  int wsSubscriptions = 0;

  late final StreamController<WebSocketMessage> _controller =
      StreamController<WebSocketMessage>.broadcast(
        onListen: () => wsSubscriptions++,
      );

  @override
  Stream<WebSocketMessage> get messages => _controller.stream;

  void emit(WebSocketMessage message) => _controller.add(message);

  void emitStreamError(Object error) => _controller.addError(error);

  void disposeController() => _controller.close();
}

NotificationEntity _notification(String id) => NotificationEntity(
  id: id,
  userId: _userId,
  type: NotificationType.orderCreated,
  title: 'T$id',
  body: 'B$id',
  isRead: false,
  createdAt: DateTime.utc(2026, 1, 1),
);

WebSocketMessage _notificationCreated({Map<String, dynamic>? data}) =>
    WebSocketMessage(
      id: 'evt-1',
      type: 'notification.created',
      from: 'server',
      data:
          data ??
          {
            'notification_id': 'n1',
            'type': 'chat_message',
            'unread_count': 99, // deliberately stale/misleading hint
          },
    );

WebSocketMessage _notificationUpdated() => WebSocketMessage(
  id: 'evt-upd',
  type: 'notification.updated',
  from: 'server',
  // Stale/misleading hint: the canonical backend is the only truth.
  data: const {'unread_count': 99},
);

({ProviderContainer container, _ScriptedNotificationRepository repo, _FakeWebSocketService socket})
_harness({_ScriptedNotificationRepository? repo}) {
  final scripted = repo ?? _ScriptedNotificationRepository();
  final socket = _FakeWebSocketService();
  final container = ProviderContainer(
    overrides: [
      webSocketServiceProvider.overrideWithValue(socket),
      notificationRepositoryProvider.overrideWithValue(scripted),
      currentUserIdProvider.overrideWith((ref) => _userId),
    ],
  );
  addTearDown(container.dispose);
  addTearDown(socket.disposeController);
  return (container: container, repo: scripted, socket: socket);
}

/// Activates the production consumer and the two canonical consumers the
/// Home badge / list watch — exactly like the live app tree does.
Future<void> _activate(ProviderContainer container) async {
  container.listen(notificationRealtimeSyncProvider, (_, _) {});
  container.listen(unreadCountProvider(_userId), (_, _) {});
  container.listen(notificationListProvider(_userId), (_, _) {});
  await pumpEventQueue();
}

void main() {
  test('Case 1: notification.created triggers canonical count refresh', () async {
    final (:container, :repo, :socket) = _harness();
    await _activate(container);
    expect(repo.countSubscriptions, 1);
    expect(container.read(unreadCountProvider(_userId)).value, 0);

    // Canonical backend now has one unread; the event arrives.
    repo.unread = 1;
    socket.emit(_notificationCreated());
    await pumpEventQueue();

    expect(repo.countSubscriptions, 2,
        reason: 'the event must invalidate the provider → immediate refetch');
    expect(container.read(unreadCountProvider(_userId)).value, 1,
        reason: 'badge must show the CANONICAL count, without polling timers');
  });

  test('Case 2 (critical): stale event unread_count is NEVER authority', () async {
    final (:container, :repo, :socket) = _harness(
      repo: _ScriptedNotificationRepository(unread: 2),
    );
    await _activate(container);
    expect(container.read(unreadCountProvider(_userId)).value, 2);

    // Event lies: unread_count = 99. Canonical backend says 2.
    socket.emit(_notificationCreated());
    await pumpEventQueue();

    expect(container.read(unreadCountProvider(_userId)).value, 2,
        reason: 'the event hint (99) must never become the badge state');
    expect(repo.countSubscriptions, 2,
        reason: 'convergence must come from the canonical re-read');
  });

  test('Case 3: notification list converges after the event', () async {
    final (:container, :repo, :socket) = _harness();
    await _activate(container);
    expect(repo.listCalls, 1);
    expect(container.read(notificationListProvider(_userId)).value, isEmpty);

    // Canonical backend now carries a new notification.
    repo.notifications = [_notification('n1')];
    socket.emit(_notificationCreated());
    await pumpEventQueue();

    expect(repo.listCalls, 2,
        reason: 'the list provider must refetch without the screen reopening');
    final list = container.read(notificationListProvider(_userId)).value;
    expect(list, isNotNull);
    expect(list!.map((n) => n.id), contains('n1'));
  });

  test('Case 4: malformed frames never kill the subscription', () async {
    final (:container, :repo, :socket) = _harness();
    await _activate(container);
    final subsBefore = repo.countSubscriptions;

    // Stream-level parse error (WebSocketService surfaces bad frames this way).
    socket.emitStreamError(FormatException('bad frame'));
    await pumpEventQueue();

    // A malformed DATA map on a typed event must not crash either.
    socket.emit(_notificationCreated(data: {'garbage': true}));
    await pumpEventQueue();
    expect(repo.countSubscriptions, subsBefore + 1,
        reason: 'reconciliation is data-shape independent and stays alive');

    // A later valid event still reconciles — the listener survived.
    socket.emit(_notificationCreated());
    await pumpEventQueue();
    expect(repo.countSubscriptions, subsBefore + 2);
  });

  test('Case 5: chat WS events do not trigger notification reconciliation', () async {
    final (:container, :repo, :socket) = _harness();
    await _activate(container);
    final countBefore = repo.countSubscriptions;
    final listBefore = repo.listCalls;

    socket.emit(
      WebSocketMessage(
        id: 'c1',
        type: 'chat.room.updated',
        from: 'server',
        data: {'room_id': 'r1', 'unread_count': 3},
      ),
    );
    socket.emit(
      WebSocketMessage(
        id: 'c2',
        type: 'chat.message.sent',
        from: 'server',
        data: {'room_id': 'r1', 'message_id': 'm1'},
      ),
    );
    await pumpEventQueue();

    expect(repo.countSubscriptions, countBefore,
        reason: 'chat events must not touch the notification consumer');
    expect(repo.listCalls, listBefore);
  });

  test('Case 6: one WS subscription, one reconciliation per event', () async {
    final (:container, :repo, :socket) = _harness();
    await _activate(container);
    expect(socket.wsSubscriptions, 1,
        reason: 'the consumer owns exactly ONE WS listener');

    final subsBefore = repo.countSubscriptions;
    socket.emit(_notificationCreated());
    await pumpEventQueue();
    expect(repo.countSubscriptions, subsBefore + 1,
        reason: 'one event → one logical reconciliation (no duplicate listener)');

    // Extra consumers of the canonical providers add no WS subscription and
    // no extra reconciliation.
    container.listen(unreadCountProvider(_userId), (_, _) {});
    container.listen(notificationRealtimeSyncProvider, (_, _) {});
    expect(socket.wsSubscriptions, 1);
    socket.emit(_notificationCreated());
    await pumpEventQueue();
    expect(repo.countSubscriptions, subsBefore + 2);
    expect(socket.wsSubscriptions, 1);
  });

  test('Case 7: notification.updated invalidates canonically (stale hint ignored)', () async {
    final (:container, :repo, :socket) = _harness(
      repo: _ScriptedNotificationRepository(unread: 2),
    );
    await _activate(container);
    expect(container.read(unreadCountProvider(_userId)).value, 2);
    final countBefore = repo.countSubscriptions;
    final listBefore = repo.listCalls;

    // A committed mutation (mark-read/delete/chat-sync) on another device:
    // the event lies with unread_count=99; canonical backend says 2.
    socket.emit(_notificationUpdated());
    await pumpEventQueue();

    expect(repo.countSubscriptions, countBefore + 1,
        reason: 'notification.updated must invalidate the count provider');
    expect(repo.listCalls, listBefore + 1,
        reason: 'notification.updated must invalidate the list provider');
    expect(container.read(unreadCountProvider(_userId)).value, 2,
        reason: 'the stale event hint must never become the badge state');
  });

  test('production wiring: the app-root initializer keeps the consumer alive', () {
    final source = File(
      'lib/domains/system/notification/presentation/widgets/notification_initializer.dart',
    ).readAsStringSync();
    expect(
      source.contains('ref.watch(notificationRealtimeSyncProvider)'),
      isTrue,
      reason:
          'NotificationInitializer must watch the realtime sync provider so '
          'the single WS listener survives for the authenticated session',
    );
  });
}
