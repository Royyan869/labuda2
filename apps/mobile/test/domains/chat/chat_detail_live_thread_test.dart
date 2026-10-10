import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/core/common/result.dart';
import 'package:hishumi/domains/chat/chat/data/chat_providers.dart';
import 'package:hishumi/domains/chat/chat/data/dto/chat_room_event_dto.dart';
import 'package:hishumi/domains/chat/chat/data/dto/message_dto.dart';
import 'package:hishumi/domains/chat/chat/domain/entities/chat_entities.dart';
import 'package:hishumi/domains/chat/chat/domain/repositories/chat_repository.dart';
import 'package:hishumi/domains/chat/chat/presentation/providers/chat_notifier.dart';
import 'package:hishumi/domains/chat/chat/presentation/providers/chat_state.dart';
import 'package:hishumi/domains/system/notification/data/notification_providers.dart';
import 'package:hishumi/domains/system/notification/domain/repositories/i_notification_repository.dart';
import 'package:hishumi/shared/providers/auth_status_providers.dart';

// ============================================================================
// CHAT_DOMAIN_P1A — LIVE THREAD AUTHORITY
//
// Canonical authority for an OPEN thread: `chat.room.updated` (user-targeted,
// viewer-scoped) → REST re-fetch. The WS frame never carries message bodies
// (ADR-005), and moderation tombstones have no distinct payload, so the
// thread always re-reads the canonical REST list instead of patching state
// from an event body.
// ============================================================================

const _roomId = 'room_1';
const _me = 'me';
const _other = 'other';

class _LiveThreadRepo implements ChatRepository {
  _LiveThreadRepo(this.events);

  final Stream<ChatRoomEventDto> events;

  /// Newest-first, exactly like the canonical REST page shape.
  List<Message> serverMessages = const [];
  bool failReads = false;
  final List<String> markedRooms = [];
  int getMessagesCalls = 0;

  @override
  Stream<ChatRoomEventDto> watchChatRoomEvents() => events;

  @override
  Future<Result<List<Message>>> getMessages({
    required String chatId,
    required String userId,
    int page = 1,
    int limit = 50,
    DateTime? cursorCreatedAt,
    String? cursorId,
  }) async {
    getMessagesCalls++;
    if (failReads) return Result.error('refresh failed');
    return Result.success(serverMessages);
  }

  @override
  Future<Result<bool>> markMessagesAsRead({
    required String chatId,
    required String userId,
    List<String>? messageIds,
  }) async {
    markedRooms.add(chatId);
    return Result.success(true);
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _NoOpNotificationRepository implements INotificationRepository {
  @override
  dynamic noSuchMethod(Invocation invocation) => null;
}

Message _message(String id, {String senderId = _other}) => Message(
  id: id,
  chatId: _roomId,
  senderId: senderId,
  senderName: senderId == _me ? 'Me' : 'Other',
  content: 'body-$id',
  // Deterministic, ascending with the m<number> suffix.
  createdAt: DateTime.utc(2026, 6, 1, int.parse(id.substring(1))),
);

ChatRoomLastMessageDto _lastMessage(Message message) =>
    ChatRoomLastMessageDto(
      id: message.id,
      roomId: message.chatId,
      senderId: message.senderId,
      messageType: 'text',
      body: message.content,
      isHidden: false,
      createdAt: message.createdAt,
    );

ChatRoomEventDto _roomEvent({
  required String roomId,
  required int unreadCount,
  Message? lastMessage,
  String? linkedOrderId,
}) => ChatRoomEventDto(
  eventType: WebSocketEventType.roomUpdated,
  roomId: roomId,
  roomType: 'direct',
  otherUserId: _other,
  linkedOrderId: linkedOrderId,
  lastMessage: lastMessage == null ? null : _lastMessage(lastMessage),
  unreadCount: unreadCount,
  createdAt: DateTime.utc(2026, 6, 1),
  updatedAt: DateTime.utc(2026, 6, 2),
  lastMessageAt: DateTime.utc(2026, 6, 2),
);

void main() {
  late StreamController<ChatRoomEventDto> events;
  late _LiveThreadRepo repo;
  late ProviderContainer container;
  late ProviderSubscription<ChatDetailState> sub;

  setUp(() {
    events = StreamController<ChatRoomEventDto>.broadcast();
    repo = _LiveThreadRepo(events.stream);
    container = ProviderContainer(
      overrides: [
        chatRepositoryProvider.overrideWithValue(repo),
        currentUserIdProvider.overrideWith((ref) => _me),
        notificationRepositoryProvider.overrideWithValue(
          _NoOpNotificationRepository(),
        ),
      ],
    );
    sub = container.listen(
      chatDetailProvider(_roomId),
      (_, __) {},
      fireImmediately: true,
    );
  });

  tearDown(() async {
    sub.close();
    container.dispose();
    await events.close();
  });

  test(
    'incoming message signal refreshes the open thread from REST',
    () async {
      repo.serverMessages = [_message('m1')];
      await container.read(chatDetailProvider(_roomId).notifier).loadMessages(
        _me,
      );
      expect(
        container.read(chatDetailProvider(_roomId)).messages.map((m) => m.id),
        ['m1'],
      );

      repo.serverMessages = [_message('m2'), _message('m1')];
      final callsBefore = repo.getMessagesCalls;
      events.add(_roomEvent(roomId: _roomId, unreadCount: 0));
      await pumpEventQueue();

      expect(repo.getMessagesCalls, callsBefore + 1);
      expect(
        container.read(chatDetailProvider(_roomId)).messages.map((m) => m.id),
        ['m2', 'm1'],
      );
    },
  );

  test(
    'refresh never collapses already-paginated history',
    () async {
      repo.serverMessages = [_message('m3'), _message('m2'), _message('m1')];
      await container.read(chatDetailProvider(_roomId).notifier).loadMessages(
        _me,
      );
      expect(
        container
            .read(chatDetailProvider(_roomId))
            .messages
            .map((m) => m.id)
            .toList(),
        ['m3', 'm2', 'm1'],
      );

      // The canonical first page now only carries the newest message; the
      // locally paginated tail must survive the merge.
      repo.serverMessages = [_message('m4')];
      events.add(_roomEvent(roomId: _roomId, unreadCount: 0));
      await pumpEventQueue();

      expect(
        container
            .read(chatDetailProvider(_roomId))
            .messages
            .map((m) => m.id)
            .toList(),
        ['m4', 'm3', 'm2', 'm1'],
      );
    },
  );

  test(
    'a repeated signal never duplicates messages',
    () async {
      repo.serverMessages = [_message('m2'), _message('m1')];
      await container.read(chatDetailProvider(_roomId).notifier).loadMessages(
        _me,
      );

      events.add(_roomEvent(roomId: _roomId, unreadCount: 0));
      events.add(_roomEvent(roomId: _roomId, unreadCount: 0));
      await pumpEventQueue();

      expect(
        container
            .read(chatDetailProvider(_roomId))
            .messages
            .map((m) => m.id)
            .toList(),
        ['m2', 'm1'],
      );
    },
  );

  test(
    'signals for other rooms leave this thread untouched',
    () async {
      repo.serverMessages = [_message('m1')];
      await container.read(chatDetailProvider(_roomId).notifier).loadMessages(
        _me,
      );
      final callsBefore = repo.getMessagesCalls;

      events.add(_roomEvent(roomId: 'room_other', unreadCount: 5));
      await pumpEventQueue();

      expect(repo.getMessagesCalls, callsBefore);
      expect(
        container.read(chatDetailProvider(_roomId)).messages.map((m) => m.id),
        ['m1'],
      );
    },
  );

  test(
    'incoming message while open clears unread through the read authority',
    () async {
      repo.serverMessages = [_message('m1')];
      await container.read(chatDetailProvider(_roomId).notifier).loadMessages(
        _me,
      );

      repo.serverMessages = [_message('m2'), _message('m1')];
      events.add(
        _roomEvent(roomId: _roomId, unreadCount: 2, lastMessage: _message('m2')),
      );
      await pumpEventQueue();

      expect(repo.markedRooms, [_roomId]);
      expect(
        container.read(chatDetailProvider(_roomId)).messages.map((m) => m.id),
        ['m2', 'm1'],
      );
    },
  );

  test(
    'a failing refresh keeps the last known thread instead of wiping it',
    () async {
      repo.serverMessages = [_message('m1')];
      await container.read(chatDetailProvider(_roomId).notifier).loadMessages(
        _me,
      );

      repo.failReads = true;
      events.add(_roomEvent(roomId: _roomId, unreadCount: 0));
      await pumpEventQueue();

      expect(
        container.read(chatDetailProvider(_roomId)).messages.map((m) => m.id),
        ['m1'],
      );
    },
  );

  test(
    'own outgoing message does not trigger a read marking',
    () async {
      repo.serverMessages = [_message('m1')];
      await container.read(chatDetailProvider(_roomId).notifier).loadMessages(
        _me,
      );

      final mine = _message('m2', senderId: _me);
      repo.serverMessages = [mine, _message('m1')];
      events.add(
        _roomEvent(roomId: _roomId, unreadCount: 1, lastMessage: mine),
      );
      await pumpEventQueue();

      expect(repo.markedRooms, isEmpty);
    },
  );
}
