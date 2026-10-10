// TASK 4 — Chat Badge Initial Load & Resume Sync.
//
// SessionLifecycleObserver is the single lifecycle owner of the canonical
// chat unread state (chatListProvider):
//   - authenticated scope boot → initialize once per user (Home badge works
//     without opening the Chat List);
//   - real background→foreground resume → refresh from canonical REST.
// The badge and every other consumer only READ that one state.

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/domains/chat/chat/data/dto/chat_room_event_dto.dart';
import 'package:hishumi/domains/chat/chat/domain/entities/chat_entities.dart';
import 'package:hishumi/domains/chat/chat/domain/repositories/chat_repository.dart';
import 'package:hishumi/domains/chat/chat/presentation/providers/chat_providers.dart';

const _userId = 'user-1';

class _RecordingAuthController extends AuthController {
  @override
  AuthState build() => AuthState.authenticated(
    AuthUser(
      id: _userId,
      email: 'test@example.com',
      username: 'testuser',
      isEmailVerified: true,
      createdAt: DateTime.utc(2026, 1, 1),
      updatedAt: DateTime.utc(2026, 1, 1),
      roles: const [UserRole.user],
      provider: AuthProvider.email,
    ),
    emailVerified: true,
  );

  @override
  Future<void> forceRefreshAuthState() async {}
}

class _RecordingWebSocketService extends WebSocketService {
  _RecordingWebSocketService() : super(baseUrl: 'ws://example.invalid');

  int reconnectCalls = 0;

  @override
  void reconnectNow() => reconnectCalls++;
}

/// Scripted canonical chat backend: serves whatever room list the current
/// scenario needs and records every getUserChats call.
class _ScriptedChatRepo implements ChatRepository {
  List<Chat> serverChats = const [];
  int getUserChatsCalls = 0;

  @override
  Future<Result<List<Chat>>> getUserChats({
    required String userId,
    int page = 1,
    int limit = 20,
  }) async {
    getUserChatsCalls++;
    return Result.success(serverChats);
  }

  @override
  Stream<ChatRoomEventDto> watchChatRoomEvents() =>
      const Stream<ChatRoomEventDto>.empty();

  @override
  dynamic noSuchMethod(Invocation invocation) => null;
}

Chat _room(String id, {int unread = 0}) => Chat(
  id: id,
  participantIds: const [_userId, 'other'],
  participantNames: const {'other': 'Other'},
  participantAvatars: const {'other': null},
  createdAt: DateTime.utc(2026, 1, 1),
  unreadCount: unread,
);

Future<(SessionLifecycleObserverState, ProviderContainer, _ScriptedChatRepo)>
_pumpHarness(WidgetTester tester, {required List<Chat> serverChats}) async {
  final repo = _ScriptedChatRepo()..serverChats = serverChats;
  final socket = _RecordingWebSocketService();

  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        authControllerProvider.overrideWith(() => _RecordingAuthController()),
        webSocketServiceProvider.overrideWithValue(socket),
        chatRepositoryProvider.overrideWithValue(repo),
      ],
      child: const MaterialApp(
        home: SessionLifecycleObserver(
          child: Scaffold(body: Text('body')),
        ),
      ),
    ),
  );
  await tester.pumpAndSettle();

  final container = ProviderScope.containerOf(
    tester.element(find.byType(SessionLifecycleObserver)),
  );
  final state = tester.state<SessionLifecycleObserverState>(
    find.byType(SessionLifecycleObserver),
  );
  return (state, container, repo);
}

void main() {
  testWidgets('Case 1: authenticated Home has the unread count without opening Chat List', (
    tester,
  ) async {
    // Canonical backend: one room with 3 unread messages.
    final (_, container, repo) = await _pumpHarness(
      tester,
      serverChats: [
        _room('r1', unread: 3),
        _room('r2', unread: 0),
      ],
    );

    expect(repo.getUserChatsCalls, greaterThanOrEqualTo(1),
        reason: 'the authenticated scope must initialize the canonical chat state');

    // Home badge source — the SAME fold Chat consumers use.
    expect(container.read(totalUnreadCountProvider), 3,
        reason: 'Home Chat badge must show the unread count without the Chat List ever opening');
  });

  testWidgets('Case 2: zero unread keeps the badge hidden at zero', (tester) async {
    final (_, container, _) = await _pumpHarness(
      tester,
      serverChats: [_room('r1', unread: 0)],
    );

    expect(container.read(totalUnreadCountProvider), 0);
  });

  testWidgets('Case 3: resume reconciles a new unread that arrived while backgrounded', (
    tester,
  ) async {
    final (_, container, repo) = await _pumpHarness(
      tester,
      serverChats: [_room('r1', unread: 0)],
    );
    expect(container.read(totalUnreadCountProvider), 0);

    // While backgrounded a new message arrives on the server.
    repo.serverChats = [_room('r1', unread: 1)];
    final callsBefore = repo.getUserChatsCalls;

    // Drive a REAL background→foreground cycle through the production seam.
    final observer = tester.state<SessionLifecycleObserverState>(
      find.byType(SessionLifecycleObserver),
    );
    observer.handleLifecycle(AppLifecycleState.paused);
    observer.handleLifecycle(AppLifecycleState.resumed);
    await tester.pumpAndSettle();

    expect(repo.getUserChatsCalls, callsBefore + 1,
        reason: 'resume must re-read the canonical chat state exactly once');
    expect(container.read(totalUnreadCountProvider), 1,
        reason: 'the badge must show the unread that arrived while backgrounded');
  });

  testWidgets('Case 4: resume reconciles a read-state change made elsewhere', (
    tester,
  ) async {
    final (_, container, repo) = await _pumpHarness(
      tester,
      serverChats: [_room('r1', unread: 1)],
    );
    expect(container.read(totalUnreadCountProvider), 1);

    // While backgrounded the message is read on another device.
    repo.serverChats = [_room('r1', unread: 0)];

    final observer = tester.state<SessionLifecycleObserverState>(
      find.byType(SessionLifecycleObserver),
    );
    observer.handleLifecycle(AppLifecycleState.paused);
    observer.handleLifecycle(AppLifecycleState.resumed);
    await tester.pumpAndSettle();

    expect(container.read(totalUnreadCountProvider), 0,
        reason: 'the badge must drop when the canonical read state changed elsewhere');
  });

  testWidgets('Case 5: one canonical authority — the badge is a fold of chatListProvider', (
    tester,
  ) async {
    final (_, container, _) = await _pumpHarness(
      tester,
      serverChats: [
        _room('r1', unread: 2),
        _room('r2', unread: 1),
      ],
    );

    final chatListState = container.read(chatListProvider);
    final foldedSum = chatListState.chats.fold<int>(
      0,
      (sum, chat) => sum + chat.roomUnreadCount,
    );
    expect(container.read(totalUnreadCountProvider), foldedSum,
        reason: 'the badge must equal the fold of the ONE canonical chat list state');
    expect(container.read(totalUnreadCountProvider), 3);
  });

  testWidgets('Case 6: resume still reconnects the realtime socket alongside the chat refresh', (
    tester,
  ) async {
    final repo = _ScriptedChatRepo()..serverChats = [_room('r1')];
    final socket = _RecordingWebSocketService();

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          authControllerProvider.overrideWith(() => _RecordingAuthController()),
          webSocketServiceProvider.overrideWithValue(socket),
          chatRepositoryProvider.overrideWithValue(repo),
        ],
        child: const MaterialApp(
          home: SessionLifecycleObserver(
            child: Scaffold(body: Text('body')),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    final observer = tester.state<SessionLifecycleObserverState>(
      find.byType(SessionLifecycleObserver),
    );
    observer.handleLifecycle(AppLifecycleState.paused);
    observer.handleLifecycle(AppLifecycleState.resumed);
    await tester.pumpAndSettle();

    expect(socket.reconnectCalls, 1,
        reason: 'the existing realtime reconnect must remain intact on resume');
  });

  testWidgets('a focus blip does NOT re-fetch the chat state', (tester) async {
    final (_, _, repo) = await _pumpHarness(tester, serverChats: [_room('r1')]);
    final callsAfterBoot = repo.getUserChatsCalls;

    final observer = tester.state<SessionLifecycleObserverState>(
      find.byType(SessionLifecycleObserver),
    );
    observer.handleLifecycle(AppLifecycleState.inactive);
    observer.handleLifecycle(AppLifecycleState.resumed);
    await tester.pumpAndSettle();

    expect(repo.getUserChatsCalls, callsAfterBoot,
        reason: 'a window focus blip is not a resume and must not re-fetch');
  });
}
