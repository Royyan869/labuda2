// PHASE 4.3 — Notification Resume & Reconnect Reconciliation (Gate B).
//
// Drives the PRODUCTION SessionLifecycleObserver against the canonical
// notification reconciliation path and proves:
//   B1/B5 resume (socket healthy) reconciles immediately;
//   B2/B6 reconnect (disconnected→connected) reconciles exactly once;
//   B3/B4 missed created/mutation events recover via canonical re-read;
//   initial connecting→connected does NOT reconcile (bootstrap keeps its
//     existing initial loading — no duplicate GET storm);
//   B7 account switch reconciles ONLY the current user;
//   B8 resume+reconnect coincidence produces exactly ONE reconciliation.
import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart' hide NotificationEntity, ConnectionState;
// Explicit alias: ConnectionState is ambiguous between the async enum and the
// WebSocket enum; the socket lifecycle contract uses the WebSocket one.
import 'package:labuda/core/websocket/websocket_service.dart' as websocket;
import 'package:labuda/domains/system/notification/data/notification_providers.dart';
import 'package:labuda/domains/system/notification/domain/entities/notification_entity.dart';
import 'package:labuda/domains/system/notification/domain/repositories/i_notification_repository.dart';
import 'package:labuda/domains/system/notification/presentation/providers/notification_list_provider.dart';
import 'package:labuda/domains/system/notification/presentation/providers/unread_count_provider.dart';
import 'package:labuda/shared/providers/auth_status_providers.dart';

const _user1 = 'user-1';
const _user2 = 'user-2';

/// Scripted canonical backend: per-user unread values + a fetch log so every
/// reconciliation (invalidate → immediate canonical GET) is observable.
class _ScriptedNotificationRepository implements INotificationRepository {
  final Map<String, int> unreadByUser = {};
  final List<String> countFetches = [];

  @override
  Future<Result<int>> getUnreadCount({required String userId}) async {
    countFetches.add(userId);
    return Result.success(unreadByUser[userId] ?? 0);
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

/// Fake socket exposing the PRODUCTION connection lifecycle contract with a
/// controllable state stream (create-once semantics preserved).
class _FakeWebSocketService extends WebSocketService {
  _FakeWebSocketService() : super(baseUrl: 'ws://example.invalid');

  final _stateController =
      StreamController<websocket.ConnectionState>.broadcast();
  bool connected = false;
  int reconnectNowCalls = 0;

  @override
  Stream<websocket.ConnectionState> get connectionState =>
      _stateController.stream;

  @override
  bool get isConnected => connected;

  @override
  void reconnectNow() => reconnectNowCalls++;

  void emitState(websocket.ConnectionState state) {
    connected = state == websocket.ConnectionState.connected;
    _stateController.add(state);
  }
}

AuthState _authFor(String userId) => AuthState.authenticated(
  AuthUser(
    id: userId,
    email: '$userId@test.invalid',
    username: userId,
    isEmailVerified: true,
    createdAt: DateTime.utc(2026, 1, 1),
    updatedAt: DateTime.utc(2026, 1, 1),
    roles: const [UserRole.user],
    provider: AuthProvider.email,
  ),
  emailVerified: true,
);

class _MutableAuthController extends AuthController {
  _MutableAuthController(this._current);

  AuthState _current;

  @override
  AuthState build() => _current;

  void switchTo(AuthState next) {
    _current = next;
    state = next;
  }

  @override
  Future<void> forceRefreshAuthState() async {}
}

Future<
  (
    SessionLifecycleObserverState,
    ProviderContainer,
    _ScriptedNotificationRepository,
    _FakeWebSocketService,
    _MutableAuthController,
  )
>
_pumpHarness(
  WidgetTester tester, {
  String initialUser = _user1,
  Map<String, int> unread = const <String, int>{},
}) async {
  final repo = _ScriptedNotificationRepository()
    ..unreadByUser.addAll(unread);
  final socket = _FakeWebSocketService();
  final auth = _MutableAuthController(_authFor(initialUser));

  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        webSocketServiceProvider.overrideWithValue(socket),
        notificationRepositoryProvider.overrideWithValue(repo),
        authControllerProvider.overrideWith(() => auth),
        currentUserIdProvider.overrideWith((ref) {
          final s = ref.watch(authControllerProvider);
          return s is AuthStateAuthenticated ? s.user.id : '';
        }),
      ],
      child: const MaterialApp(
        home: SessionLifecycleObserver(child: SizedBox.shrink()),
      ),
    ),
  );
  await tester.pump(const Duration(milliseconds: 50));

  final container = ProviderScope.containerOf(
    tester.element(find.byType(SessionLifecycleObserver)),
  );
  // Activate the canonical badge/list consumers exactly like the Home badge
  // does, establishing the initial state (the existing initial loading).
  container.listen(unreadCountProvider(initialUser), (_, _) {});
  container.listen(notificationListProvider(initialUser), (_, _) {});
  await tester.pump(const Duration(milliseconds: 50));

  final state = tester.state<SessionLifecycleObserverState>(
    find.byType(SessionLifecycleObserver),
  );
  return (state, container, repo, socket, auth);
}

void main() {
  testWidgets('B1/B5: resume with a healthy socket reconciles immediately', (
    tester,
  ) async {
    final (state, container, repo, socket, _) = await _pumpHarness(
      tester,
      unread: {_user1: 1},
    );
    await tester.pump(const Duration(milliseconds: 20));
    expect(container.read(unreadCountProvider(_user1)).value, 1);

    // The socket is healthy for this case (no disconnect happened).
    socket.emitState(websocket.ConnectionState.connected);
    await tester.pump(const Duration(milliseconds: 20));

    // Canonical backend changes to 2 while "backgrounded".
    repo.unreadByUser[_user1] = 2;
    final fetchesBefore = repo.countFetches.length;

    state.handleLifecycle(AppLifecycleState.paused);
    state.handleLifecycle(AppLifecycleState.resumed);
    await tester.pump(const Duration(milliseconds: 50));

    expect(repo.countFetches.length, fetchesBefore + 1,
        reason: 'resume must reconcile via ONE canonical re-read');
    expect(repo.countFetches.last, _user1);
    expect(container.read(unreadCountProvider(_user1)).value, 2,
        reason: 'badge must show canonical state without waiting for polling');
    expect(socket.reconnectNowCalls, 1,
        reason: 'existing realtime resume behavior stays intact');
  });

  testWidgets(
    'B2/B3/B4/B6: reconnect (disconnected→connected) reconciles exactly once; '
    'initial connecting→connected does not',
    (tester) async {
      final (state, container, repo, socket, _) = await _pumpHarness(
        tester,
      );
      repo.unreadByUser[_user1] = 1;
      await tester.pump(const Duration(milliseconds: 20));
      final fetchesAfterActivation = repo.countFetches.length;

      // Initial connect: connecting → connected must NOT reconcile
      // (bootstrap keeps the existing initial notification loading).
      socket.emitState(websocket.ConnectionState.connecting);
      socket.emitState(websocket.ConnectionState.connected);
      await tester.pump(const Duration(milliseconds: 20));
      expect(repo.countFetches.length, fetchesAfterActivation,
          reason: 'initial connect must not duplicate the initial loading');

      // Disconnect window: a created event AND a mutation event are both
      // missed (B3/B4) — canonical state changes from 1 to 3 meanwhile.
      repo.unreadByUser[_user1] = 3;
      socket.emitState(websocket.ConnectionState.disconnected);
      await tester.pump(const Duration(milliseconds: 20));
      expect(repo.countFetches.length, fetchesAfterActivation,
          reason: 'going down alone must not reconcile');

      // Reconnect without any resume (B6): reconnecting → connected.
      socket.emitState(websocket.ConnectionState.reconnecting);
      socket.emitState(websocket.ConnectionState.connected);
      await tester.pump(const Duration(milliseconds: 50));

      expect(repo.countFetches.length, fetchesAfterActivation + 1,
        reason: 'reconnect must reconcile exactly once via the same path');
      expect(container.read(unreadCountProvider(_user1)).value, 3,
        reason: 'missed created/mutation events recover via canonical re-read');
      expect(state, isNotNull);
    },
  );

  testWidgets('B7: account switch reconciles only the CURRENT user', (
    tester,
  ) async {
    final (state, container, repo, socket, auth) = await _pumpHarness(
      tester,
    );
    repo.unreadByUser[_user1] = 5;
    repo.unreadByUser[_user2] = 7;
    await tester.pump(const Duration(milliseconds: 20));
    final user1FetchesBeforeSwitch = repo.countFetches
        .where((u) => u == _user1)
        .length;

    // Logout/switch: new authenticated identity.
    auth.switchTo(_authFor(_user2));
    await tester.pump(const Duration(milliseconds: 50));
    container.listen(unreadCountProvider(_user2), (_, _) {});
    await tester.pump(const Duration(milliseconds: 50));

    state.handleLifecycle(AppLifecycleState.paused);
    state.handleLifecycle(AppLifecycleState.resumed);
    await tester.pump(const Duration(milliseconds: 50));

    expect(
      repo.countFetches.where((u) => u == _user1).length,
      user1FetchesBeforeSwitch,
      reason: 'the old user must never be fetched after the switch',
    );
    expect(repo.countFetches.last, _user2,
        reason: 'reconciliation must use the CURRENT authenticated user');
    expect(container.read(unreadCountProvider(_user2)).value, 7);
  });

  testWidgets(
    'B8: resume while the socket is down defers to the reconnect trigger — '
    'exactly ONE reconciliation for the coincidence',
    (tester) async {
      final (state, container, repo, socket, _) = await _pumpHarness(
        tester,
      );
      repo.unreadByUser[_user1] = 1;
      await tester.pump(const Duration(milliseconds: 20));
      final fetchesAfterActivation = repo.countFetches.length;

      // Socket dies in the background; canonical state moves to 4.
      socket.emitState(websocket.ConnectionState.disconnected);
      repo.unreadByUser[_user1] = 4;
      await tester.pump(const Duration(milliseconds: 20));

      // Resume with the socket still down: reconnectNow is scheduled, and
      // the resume trigger defers the reconcile to the reconnect trigger.
      state.handleLifecycle(AppLifecycleState.paused);
      state.handleLifecycle(AppLifecycleState.resumed);
      await tester.pump(const Duration(milliseconds: 50));
      expect(socket.reconnectNowCalls, 1);
      expect(repo.countFetches.length, fetchesAfterActivation,
        reason: 'resume with a dead socket must not double-reconcile');

      // The reconnect completes → ONE reconcile via the same canonical path.
      socket.emitState(websocket.ConnectionState.connected);
      await tester.pump(const Duration(milliseconds: 50));
      expect(repo.countFetches.length, fetchesAfterActivation + 1,
        reason: 'the coincidence must produce exactly one reconciliation');
      expect(container.read(unreadCountProvider(_user1)).value, 4);
    },
  );
}
