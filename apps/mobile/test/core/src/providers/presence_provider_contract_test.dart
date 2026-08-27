import 'dart:async';

import 'package:fake_async/fake_async.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/core/src/providers/presence_provider.dart'
    as core_presence;
import 'package:labuda/core/websocket/websocket_message.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:labuda/shared/providers/authenticated_account_provider.dart'
    show authenticatedUserProvider;

AuthUser? _currentFakeAuthUser;

class _NoopLogger implements ILoggerService {
  @override
  Future<Result<void>> debug(String message, {Map<String, dynamic>? extra}) =>
      Future.value(Result.success(null));
  @override
  Future<Result<void>> info(String message, {Map<String, dynamic>? extra}) =>
      Future.value(Result.success(null));
  @override
  Future<Result<void>> warning(String message, {Map<String, dynamic>? extra}) =>
      Future.value(Result.success(null));
  @override
  Future<Result<void>> error(
    String message, {
    Map<String, dynamic>? extra,
    StackTrace? stackTrace,
  }) => Future.value(Result.success(null));
  @override
  Future<Result<void>> fatal(
    String message, {
    Map<String, dynamic>? extra,
    StackTrace? stackTrace,
  }) => Future.value(Result.success(null));
  @override
  Future<Result<void>> logUserAction(
    String action, {
    String? userId,
    Map<String, dynamic>? parameters,
  }) => Future.value(Result.success(null));
  @override
  Future<Result<void>> logPerformance(
    String operation, {
    required Duration duration,
    Map<String, dynamic>? metrics,
  }) => Future.value(Result.success(null));
  @override
  Future<Result<void>> logSecurityEvent(
    String event, {
    String? userId,
    String? severity,
    Map<String, dynamic>? details,
  }) => Future.value(Result.success(null));
  @override
  Future<Result<void>> logApiCall(
    String endpoint, {
    required String method,
    required int statusCode,
    required Duration duration,
    Map<String, dynamic>? requestData,
    Map<String, dynamic>? responseData,
  }) => Future.value(Result.success(null));
  @override
  Future<Result<void>> setLogLevel(LogLevel level) =>
      Future.value(Result.success(null));
  @override
  Future<Result<void>> clearLogs() => Future.value(Result.success(null));
  @override
  Future<Result<List<LogEntry>>> getLogs({
    LogLevel? minLevel,
    DateTime? startDate,
    DateTime? endDate,
    int? limit,
  }) => Future.value(Result.success(const <LogEntry>[]));
  @override
  Future<void> debugSync(String userId) async {}
  @override
  Future<void> debugSyncSuccess(String userId) async {}
  @override
  Future<void> debugSyncFailed(String userId, String? errorMessage) async {}
  @override
  Future<void> debugCallingGetCurrentUser() async {}
  @override
  Future<void> debugGetCurrentUserSuccess(
    String userId,
    bool isEmailVerified,
  ) async {}
  @override
  Future<void> debugGetCurrentUserFailed(
    String userId,
    String? errorMessage,
  ) async {}
  @override
  Future<void> debugSyncException(
    String userId,
    String errorMessage,
    String stackTrace,
  ) async {}
  @override
  Future<void> debugRouterCheck(
    String userId,
    bool isEmailVerified,
    String location,
    bool isVerificationRoute,
  ) async {}
  @override
  Future<void> log(String message, {LogLevel level = LogLevel.debug}) async {}
}

class _RecordingWebSocketService extends WebSocketService {
  _RecordingWebSocketService() : super(baseUrl: 'ws://example.invalid');

  final StreamController<WebSocketMessage> _messages =
      StreamController<WebSocketMessage>.broadcast(sync: true);
  final StreamController<ConnectionState> _connection =
      StreamController<ConnectionState>.broadcast(sync: true);

  final List<WebSocketMessage> sentMessages = <WebSocketMessage>[];
  final List<String> events = <String>[];

  Completer<void>? leaveCompleter;
  bool failLeave = false;

  @override
  Stream<WebSocketMessage> get messages => _messages.stream;

  @override
  Stream<ConnectionState> get connectionState => _connection.stream;

  void emitConnected() => _connection.add(ConnectionState.connected);
  void emitDisconnected() => _connection.add(ConnectionState.disconnected);
  void emitPresence(WebSocketMessage message) => _messages.add(message);
  void clearEvents() {
    sentMessages.clear();
    events.clear();
  }

  @override
  Future<void> send(WebSocketMessage message, {bool requireAck = false}) async {
    sentMessages.add(message);
    events.add(message.type);
    if (message.type == MessageType.presenceLeave) {
      if (failLeave) throw StateError('transport failure');
      if (leaveCompleter != null) {
        return leaveCompleter!.future;
      }
    }
  }

  @override
  Future<void> disconnect() async {
    events.add('disconnect');
  }
}

AuthUser _authUser(String id, {String username = 'presence-user'}) {
  return AuthUser(
    id: id,
    createdAt: DateTime.utc(2026, 6, 1),
    updatedAt: DateTime.utc(2026, 6, 1),
    email: '$username@example.com',
    username: username,
    isEmailVerified: true,
    roles: const [UserRole.user],
    provider: ShonaAuthProvider.email,
    accountStatus: AccountStatus.active,
  );
}

ProviderContainer _container(_RecordingWebSocketService ws) {
  return ProviderContainer(
    overrides: [
      authenticatedUserProvider.overrideWith((ref) => _currentFakeAuthUser),
      webSocketServiceProvider.overrideWithValue(ws),
      loggerServiceProvider.overrideWithValue(_NoopLogger()),
    ],
  );
}

void _setAuth(ProviderContainer container, AuthUser? user) {
  _currentFakeAuthUser = user;
  container.refresh(authenticatedUserProvider);
}

Future<void> _drain() async {
  await Future<void>.delayed(Duration.zero);
  await Future<void>.delayed(Duration.zero);
}

String _uuid(int suffix) {
  return '123e4567-e89b-12d3-a456-${suffix.toRadixString(16).padLeft(12, '0')}';
}

void main() {
  group('PresenceManager contract', () {
    test(
      'versioning keeps higher version, ignores equal/lower, and keeps users independent',
      () async {
        final ws = _RecordingWebSocketService();
        final container = _container(ws);
        addTearDown(container.dispose);

        _setAuth(container, _authUser(_uuid(1), username: 'principal'));
        container.read(core_presence.presenceManagerProvider);
        ws.emitConnected();
        await _drain();
        ws.clearEvents();

        const userA = '123e4567-e89b-12d3-a456-426614174100';
        const userB = '123e4567-e89b-12d3-a456-426614174101';
        final manager = container.read(
          core_presence.presenceManagerProvider.notifier,
        );
        final subscription = manager.acquire({userA, userB});
        addTearDown(subscription.dispose);
        await _drain();
        ws.clearEvents();

        ws.emitPresence(
          WebSocketMessage(
            type: MessageType.presenceSnapshot,
            from: 'server',
            data: {
              'states': [
                {
                  'user_id': userA,
                  'is_online': true,
                  'last_seen_at': '2026-07-29T10:00:00Z',
                  'version': 1,
                },
                {'user_id': userB, 'is_online': false, 'version': 1},
              ],
            },
          ),
        );
        await _drain();
        expect(
          container
              .read(core_presence.presenceManagerProvider)
              .records[userA]
              ?.version,
          1,
        );
        expect(
          container
              .read(core_presence.presenceManagerProvider)
              .records[userB]
              ?.isOnline,
          isFalse,
        );

        ws.emitPresence(
          WebSocketMessage(
            type: MessageType.presenceChanged,
            from: 'server',
            data: {
              'user_id': userA,
              'is_online': false,
              'last_seen_at': '2026-07-29T10:05:00Z',
              'version': 2,
            },
          ),
        );
        await _drain();
        expect(
          container
              .read(core_presence.presenceManagerProvider)
              .records[userA]
              ?.version,
          2,
        );
        expect(
          container
              .read(core_presence.presenceManagerProvider)
              .records[userA]
              ?.isOnline,
          isFalse,
        );

        ws.emitPresence(
          WebSocketMessage(
            type: MessageType.presenceChanged,
            from: 'server',
            data: {'user_id': userA, 'is_online': true, 'version': 2},
          ),
        );
        await _drain();
        expect(
          container
              .read(core_presence.presenceManagerProvider)
              .records[userA]
              ?.isOnline,
          isFalse,
        );

        ws.emitPresence(
          WebSocketMessage(
            type: MessageType.presenceChanged,
            from: 'server',
            data: {'user_id': userA, 'is_online': true, 'version': 1},
          ),
        );
        await _drain();
        expect(
          container
              .read(core_presence.presenceManagerProvider)
              .records[userA]
              ?.version,
          2,
        );
        expect(
          container
              .read(core_presence.presenceManagerProvider)
              .records[userB]
              ?.isOnline,
          isFalse,
        );
      },
    );

    test(
      'logout and account switch clear state and stale frames are ignored',
      () async {
        final ws = _RecordingWebSocketService();
        final container = _container(ws);
        addTearDown(container.dispose);

        _setAuth(container, _authUser(_uuid(2), username: 'principal-a'));
        container.read(core_presence.presenceManagerProvider);
        ws.emitConnected();
        await _drain();
        ws.clearEvents();

        const watchedUser = '123e4567-e89b-12d3-a456-426614174102';
        final handle = container
            .read(core_presence.presenceManagerProvider.notifier)
            .acquire({watchedUser});
        addTearDown(handle.dispose);
        await _drain();

        ws.emitPresence(
          WebSocketMessage(
            type: MessageType.presenceChanged,
            from: 'server',
            data: {'user_id': watchedUser, 'is_online': true, 'version': 1},
          ),
        );
        await _drain();
        expect(
          container
              .read(core_presence.presenceManagerProvider)
              .records[watchedUser]
              ?.isOnline,
          isTrue,
        );

        _setAuth(container, null);
        await _drain();
        expect(
          container.read(core_presence.presenceManagerProvider).records,
          isEmpty,
        );

        ws.clearEvents();
        await container
            .read(core_presence.presenceManagerProvider.notifier)
            .prepareForLogout();
        await _drain();
        expect(
          ws.events,
          isEmpty,
          reason: 'repeated logout must not emit duplicate leave',
        );

        ws.emitPresence(
          WebSocketMessage(
            type: MessageType.presenceChanged,
            from: 'server',
            data: {'user_id': watchedUser, 'is_online': false, 'version': 2},
          ),
        );
        await _drain();
        expect(
          container.read(core_presence.presenceManagerProvider).records,
          isEmpty,
        );

        _setAuth(container, _authUser(_uuid(3), username: 'principal-b'));
        await _drain();
        expect(
          container.read(core_presence.presenceManagerProvider).records,
          isEmpty,
        );
      },
    );

    test(
      'account switch sends old-principal leave and ignores stale principal frames',
      () async {
        final ws = _RecordingWebSocketService();
        final container = _container(ws);
        addTearDown(container.dispose);

        final oldPrincipal = _authUser(_uuid(2), username: 'principal-a');
        final newPrincipal = _authUser(_uuid(3), username: 'principal-b');

        _setAuth(container, oldPrincipal);
        container.read(core_presence.presenceManagerProvider);
        ws.emitConnected();
        await _drain();
        ws.clearEvents();

        _setAuth(container, newPrincipal);
        await _drain();

        expect(ws.events, isNotEmpty);
        expect(ws.events.first, MessageType.presenceLeave);
        expect(ws.events, contains(MessageType.presenceLeave));
        expect(
          container.read(core_presence.presenceManagerProvider).records,
          isEmpty,
        );

        ws.emitPresence(
          WebSocketMessage(
            type: MessageType.presenceChanged,
            from: 'server',
            data: {'user_id': oldPrincipal.id, 'is_online': true, 'version': 2},
          ),
        );
        await _drain();
        expect(
          container.read(core_presence.presenceManagerProvider).records,
          isEmpty,
        );
      },
    );

    test(
      'subscription registry deduplicates, batches to 200, and fails closed',
      () async {
        final ws = _RecordingWebSocketService();
        final container = _container(ws);
        addTearDown(container.dispose);

        _setAuth(container, _authUser(_uuid(4), username: 'principal'));
        container.read(core_presence.presenceManagerProvider);
        ws.emitConnected();
        await _drain();
        ws.clearEvents();

        const watchedUser = '123e4567-e89b-12d3-a456-426614174103';
        final first = container
            .read(core_presence.presenceManagerProvider.notifier)
            .acquire({watchedUser});
        await _drain();
        expect(
          ws.sentMessages.where((m) => m.type == MessageType.presenceSubscribe),
          hasLength(1),
        );

        final second = container
            .read(core_presence.presenceManagerProvider.notifier)
            .acquire({watchedUser});
        await _drain();
        expect(
          ws.sentMessages.where((m) => m.type == MessageType.presenceSubscribe),
          hasLength(1),
        );

        await first.dispose();
        await _drain();
        expect(
          ws.sentMessages.where(
            (m) => m.type == MessageType.presenceUnsubscribe,
          ),
          isEmpty,
        );

        await second.dispose();
        await _drain();
        expect(
          ws.sentMessages.where(
            (m) => m.type == MessageType.presenceUnsubscribe,
          ),
          hasLength(1),
        );

        ws.clearEvents();
        final duplicateIds = <String>[watchedUser, watchedUser];
        final duplicate = container
            .read(core_presence.presenceManagerProvider.notifier)
            .acquire(duplicateIds.toSet());
        await _drain();
        expect(
          ws.sentMessages.where((m) => m.type == MessageType.presenceSubscribe),
          hasLength(1),
        );
        await duplicate.dispose();
        await _drain();
        expect(
          ws.sentMessages.where(
            (m) => m.type == MessageType.presenceUnsubscribe,
          ),
          hasLength(1),
        );

        ws.clearEvents();
        final empty = container
            .read(core_presence.presenceManagerProvider.notifier)
            .acquire(const <String>{});
        await empty.dispose();
        await _drain();
        expect(ws.sentMessages, isEmpty);

        expect(
          () => container
              .read(core_presence.presenceManagerProvider.notifier)
              .acquire({'not-a-uuid'}),
          throwsFormatException,
        );
        expect(ws.sentMessages, isEmpty);

        final ids = List<String>.generate(201, (index) => _uuid(1000 + index));
        final batchHandle = container
            .read(core_presence.presenceManagerProvider.notifier)
            .acquire(ids.toSet());
        addTearDown(batchHandle.dispose);
        await _drain();

        final subscribeMessages = ws.sentMessages
            .where((message) => message.type == MessageType.presenceSubscribe)
            .toList();
        expect(subscribeMessages, hasLength(2));
        expect(subscribeMessages.first.data['user_ids'], hasLength(200));
        expect(subscribeMessages.last.data['user_ids'], hasLength(1));
      },
    );

    test(
      'background preserves desired subscriptions, reconnect replays once, and leave failures stay bounded',
      () {
        fakeAsync((async) {
          final ws = _RecordingWebSocketService();
          final container = _container(ws);
          addTearDown(container.dispose);

          _setAuth(container, _authUser(_uuid(5), username: 'principal'));
          container.read(core_presence.presenceManagerProvider);
          ws.emitConnected();
          async.flushMicrotasks();
          ws.clearEvents();

          const watchedUser = '123e4567-e89b-12d3-a456-426614174104';
          final handle = container
              .read(core_presence.presenceManagerProvider.notifier)
              .acquire({watchedUser});
          addTearDown(handle.dispose);
          async.flushMicrotasks();

          expect(
            ws.sentMessages.where(
              (m) => m.type == MessageType.presenceSubscribe,
            ),
            hasLength(1),
          );

          container
              .read(core_presence.presenceManagerProvider.notifier)
              .setForeground(false);
          async.flushMicrotasks();
          expect(
            ws.events.where(
              (event) => event == MessageType.presenceUnsubscribe,
            ),
            isEmpty,
          );

          ws.emitDisconnected();
          ws.emitConnected();
          async.flushMicrotasks();
          expect(
            ws.events.where((event) => event == MessageType.presenceResume),
            isEmpty,
            reason: 'reconnect while background should not resume again',
          );
          expect(
            ws.events.where((event) => event == MessageType.presencePause),
            hasLength(2),
            reason: 'background reconnect should reassert the pause contract',
          );

          container
              .read(core_presence.presenceManagerProvider.notifier)
              .setForeground(true);
          async.flushMicrotasks();
          expect(
            ws.events.where((event) => event == MessageType.presenceResume),
            hasLength(1),
          );

          ws.clearEvents();
          async.flushMicrotasks();
          expect(ws.events, isEmpty);
          ws.leaveCompleter = Completer<void>();
          final logoutFuture = container
              .read(core_presence.presenceManagerProvider.notifier)
              .prepareForLogout();
          async.elapse(const Duration(seconds: 4));
          async.flushMicrotasks();
          expect(logoutFuture, completes);
          expect(ws.events, contains(MessageType.presenceLeave));
          expect(ws.events.last, MessageType.presenceLeave);
          expect(
            container.read(core_presence.presenceManagerProvider).records,
            isEmpty,
          );

          ws.clearEvents();
          ws.failLeave = true;
          _setAuth(container, _authUser(_uuid(6), username: 'principal-2'));
          container
              .read(core_presence.presenceManagerProvider.notifier)
              .prepareForLogout();
          async.flushMicrotasks();
          expect(ws.events, contains(MessageType.presenceLeave));
          expect(ws.events.last, MessageType.presenceLeave);
          expect(
            container.read(core_presence.presenceManagerProvider).records,
            isEmpty,
          );
        });
      },
    );
  });
}
