import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:labuda/core/common/result.dart';
import 'package:labuda/domains/chat/chat/data/chat_providers.dart';
import 'package:labuda/domains/chat/chat/data/dto/chat_room_event_dto.dart';
import 'package:labuda/domains/chat/chat/data/dto/message_dto.dart';
import 'package:labuda/domains/chat/chat/domain/entities/chat_entities.dart';
import 'package:labuda/domains/chat/chat/domain/repositories/chat_repository.dart';
import 'package:labuda/domains/chat/chat/presentation/providers/chat_notifier.dart';
import 'package:labuda/domains/chat/chat/presentation/providers/chat_state.dart';

Message _lastMessage(
  String roomId,
  String messageId,
  String senderId,
  DateTime createdAt, {
  bool isHidden = false,
  String content = 'preview',
}) {
  return Message(
    id: messageId,
    chatId: roomId,
    senderId: senderId,
    senderName: '',
    content: content,
    isHidden: isHidden,
    type: MessageType.text,
    createdAt: createdAt,
    status: MessageStatus.sent,
    deletedBy: const [],
  );
}

Chat _chat({
  required String roomId,
  required String otherUserId,
  required String otherUsername,
  required DateTime createdAt,
  required DateTime updatedAt,
  required DateTime lastMessageAt,
  required int unreadCount,
  Message? lastMessage,
  String? linkedOrderId,
  ChatType type = ChatType.private,
}) {
  return Chat(
    id: roomId,
    type: type,
    participantIds: [otherUserId],
    participantNames: {otherUserId: otherUsername},
    participantAvatars: const {},
    lastMessage: lastMessage,
    createdAt: createdAt,
    updatedAt: updatedAt,
    unreadCount: unreadCount,
    linkedOrderId: linkedOrderId,
  );
}

Map<String, dynamic> _roomEventPayload({
  required String roomId,
  required String otherUserId,
  required String otherUsername,
  required DateTime createdAt,
  required DateTime updatedAt,
  required DateTime lastMessageAt,
  required int unreadCount,
  String? linkedOrderId,
  Map<String, dynamic>? lastMessage,
}) {
  return <String, dynamic>{
    'room_id': roomId,
    'room_type': 'direct',
    'other_user_id': otherUserId,
    'other_user': <String, dynamic>{
      'id': otherUserId,
      'username': otherUsername,
      'display_name': 'Display $otherUsername',
      'avatar_url': 'https://example.com/$otherUserId.png',
      'lifecycle': 'active',
    },
    if (linkedOrderId != null) 'linked_order_id': linkedOrderId,
    if (lastMessage != null) 'last_message': lastMessage,
    'unread_count': unreadCount,
    'created_at': createdAt.toIso8601String(),
    'updated_at': updatedAt.toIso8601String(),
    'last_message_at': lastMessageAt.toIso8601String(),
  };
}

Map<String, dynamic> _lastMessagePayload({
  required String messageId,
  required String roomId,
  required String senderId,
  required String messageType,
  required DateTime createdAt,
  String? body,
  bool isHidden = false,
}) {
  return <String, dynamic>{
    'id': messageId,
    'room_id': roomId,
    'sender_id': senderId,
    'message_type': messageType,
    if (body != null) 'body': body,
    'is_hidden': isHidden,
    'created_at': createdAt.toIso8601String(),
  };
}

class _FakeChatRepository implements ChatRepository {
  final List<Chat> initialChats;
  final StreamController<ChatRoomEventDto> events =
      StreamController<ChatRoomEventDto>.broadcast();
  int getUserChatsCalls = 0;

  _FakeChatRepository(this.initialChats);

  @override
  Future<Result<List<Chat>>> getUserChats({
    required String userId,
    int page = 1,
    int limit = 20,
  }) async {
    getUserChatsCalls++;
    return Result.success(List<Chat>.from(initialChats));
  }

  @override
  Stream<ChatRoomEventDto> watchChatRoomEvents() => events.stream;

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

ProviderSubscription<ChatListState> _keepChatListAlive(
  ProviderContainer container,
) {
  return container.listen(chatListProvider, (_, __) {}, fireImmediately: true);
}

void main() {
  group('ChatListNotifier room-event consumer', () {
    test(
      'room.created inserts a new row and duplicate events do not create a second row',
      () async {
        final roomA = _chat(
          roomId: 'room_a',
          otherUserId: 'user_a',
          otherUsername: 'alice',
          createdAt: DateTime.utc(2026, 6, 2, 10, 0),
          updatedAt: DateTime.utc(2026, 6, 2, 10, 0),
          lastMessageAt: DateTime.utc(2026, 6, 2, 10, 0),
          unreadCount: 1,
          lastMessage: _lastMessage(
            'room_a',
            'msg_a',
            'user_a',
            DateTime.utc(2026, 6, 2, 10, 0),
          ),
        );
        final repo = _FakeChatRepository([roomA]);
        final container = ProviderContainer(
          overrides: [chatRepositoryProvider.overrideWithValue(repo)],
        );
        final chatListSubscription = _keepChatListAlive(container);
        addTearDown(chatListSubscription.close);
        addTearDown(container.dispose);

        final notifier = container.read(chatListProvider.notifier);
        await notifier.loadChats('user_me');

        repo.events.add(
          ChatRoomEventDto.fromJson(
            _roomEventPayload(
              roomId: 'room_b',
              otherUserId: 'user_b',
              otherUsername: 'bob',
              createdAt: DateTime.utc(2026, 6, 2, 10, 5),
              updatedAt: DateTime.utc(2026, 6, 2, 10, 5),
              lastMessageAt: DateTime.utc(2026, 6, 2, 10, 5),
              unreadCount: 3,
              linkedOrderId: 'order_b',
            ),
            eventType: WebSocketEventType.roomCreated,
          ),
        );

        await Future<void>.delayed(Duration.zero);

        var state = container.read(chatListProvider);
        expect(state.chats, hasLength(2));
        expect(state.chats.first.id, 'room_b');
        expect(state.chats.first.linkedOrderId, 'order_b');
        expect(state.chats.first.roomUnreadCount, 3);

        repo.events.add(
          ChatRoomEventDto.fromJson(
            _roomEventPayload(
              roomId: 'room_b',
              otherUserId: 'user_b',
              otherUsername: 'bob',
              createdAt: DateTime.utc(2026, 6, 2, 10, 5),
              updatedAt: DateTime.utc(2026, 6, 2, 10, 6),
              lastMessageAt: DateTime.utc(2026, 6, 2, 10, 5),
              unreadCount: 4,
              linkedOrderId: 'order_b',
            ),
            eventType: WebSocketEventType.roomUpdated,
          ),
        );

        await Future<void>.delayed(Duration.zero);

        state = container.read(chatListProvider);
        expect(state.chats, hasLength(2));
        expect(state.chats.where((chat) => chat.id == 'room_b'), hasLength(1));
        expect(state.chats.first.id, 'room_b');
        expect(state.chats.first.roomUnreadCount, 4);
      },
    );

    test(
      'room.updated preserves position when last_message_at is unchanged, but reorders when a new message arrives',
      () async {
        final roomA = _chat(
          roomId: 'room_a',
          otherUserId: 'user_a',
          otherUsername: 'alice',
          createdAt: DateTime.utc(2026, 6, 2, 10, 0),
          updatedAt: DateTime.utc(2026, 6, 2, 10, 0),
          lastMessageAt: DateTime.utc(2026, 6, 2, 10, 10),
          unreadCount: 1,
          lastMessage: _lastMessage(
            'room_a',
            'msg_a',
            'user_a',
            DateTime.utc(2026, 6, 2, 10, 10),
          ),
        );
        final roomB = _chat(
          roomId: 'room_b',
          otherUserId: 'user_b',
          otherUsername: 'bob',
          createdAt: DateTime.utc(2026, 6, 2, 10, 0),
          updatedAt: DateTime.utc(2026, 6, 2, 10, 0),
          lastMessageAt: DateTime.utc(2026, 6, 2, 10, 5),
          unreadCount: 1,
          lastMessage: _lastMessage(
            'room_b',
            'msg_b',
            'user_b',
            DateTime.utc(2026, 6, 2, 10, 5),
          ),
        );
        final repo = _FakeChatRepository([roomA, roomB]);
        final container = ProviderContainer(
          overrides: [chatRepositoryProvider.overrideWithValue(repo)],
        );
        final chatListSubscription = _keepChatListAlive(container);
        addTearDown(chatListSubscription.close);
        addTearDown(container.dispose);

        final notifier = container.read(chatListProvider.notifier);
        await notifier.loadChats('user_me');

        var state = container.read(chatListProvider);
        expect(state.chats.map((chat) => chat.id), ['room_a', 'room_b']);

        repo.events.add(
          ChatRoomEventDto.fromJson(
            _roomEventPayload(
              roomId: 'room_b',
              otherUserId: 'user_b',
              otherUsername: 'bob',
              createdAt: DateTime.utc(2026, 6, 2, 10, 0),
              updatedAt: DateTime.utc(2026, 6, 2, 10, 7),
              lastMessageAt: DateTime.utc(2026, 6, 2, 10, 5),
              unreadCount: 0,
              lastMessage: _lastMessagePayload(
                messageId: 'msg_b',
                roomId: 'room_b',
                senderId: 'user_b',
                messageType: 'text',
                createdAt: DateTime.utc(2026, 6, 2, 10, 5),
                body: 'read-state only',
              ),
            ),
            eventType: WebSocketEventType.roomUpdated,
          ),
        );

        await Future<void>.delayed(Duration.zero);

        state = container.read(chatListProvider);
        expect(state.chats.map((chat) => chat.id), ['room_a', 'room_b']);
        expect(state.chats.last.roomUnreadCount, 0);

        repo.events.add(
          ChatRoomEventDto.fromJson(
            _roomEventPayload(
              roomId: 'room_b',
              otherUserId: 'user_b',
              otherUsername: 'bob',
              createdAt: DateTime.utc(2026, 6, 2, 10, 0),
              updatedAt: DateTime.utc(2026, 6, 2, 10, 11),
              lastMessageAt: DateTime.utc(2026, 6, 2, 10, 11),
              unreadCount: 1,
              lastMessage: _lastMessagePayload(
                messageId: 'msg_b2',
                roomId: 'room_b',
                senderId: 'user_b',
                messageType: 'text',
                createdAt: DateTime.utc(2026, 6, 2, 10, 11),
                body: 'new message',
              ),
            ),
            eventType: WebSocketEventType.roomUpdated,
          ),
        );

        await Future<void>.delayed(Duration.zero);

        state = container.read(chatListProvider);
        expect(state.chats.map((chat) => chat.id), ['room_b', 'room_a']);
        expect(state.chats.first.roomUnreadCount, 1);
      },
    );

    test(
      'created room with null last_message and hidden tombstone room.updated are handled safely',
      () async {
        final roomA = _chat(
          roomId: 'room_a',
          otherUserId: 'user_a',
          otherUsername: 'alice',
          createdAt: DateTime.utc(2026, 6, 2, 10, 0),
          updatedAt: DateTime.utc(2026, 6, 2, 10, 0),
          lastMessageAt: DateTime.utc(2026, 6, 2, 10, 0),
          unreadCount: 1,
        );
        final repo = _FakeChatRepository([roomA]);
        final container = ProviderContainer(
          overrides: [chatRepositoryProvider.overrideWithValue(repo)],
        );
        final chatListSubscription = _keepChatListAlive(container);
        addTearDown(chatListSubscription.close);
        addTearDown(container.dispose);

        final notifier = container.read(chatListProvider.notifier);
        await notifier.loadChats('user_me');

        repo.events.add(
          ChatRoomEventDto.fromJson(
            _roomEventPayload(
              roomId: 'room_new',
              otherUserId: 'user_new',
              otherUsername: 'newbie',
              createdAt: DateTime.utc(2026, 6, 2, 10, 1),
              updatedAt: DateTime.utc(2026, 6, 2, 10, 1),
              lastMessageAt: DateTime.utc(2026, 6, 2, 10, 1),
              unreadCount: 0,
            ),
            eventType: WebSocketEventType.roomCreated,
          ),
        );

        await Future<void>.delayed(Duration.zero);

        var state = container.read(chatListProvider);
        expect(state.chats.any((chat) => chat.id == 'room_new'), isTrue);
        final createdRoom = state.chats.firstWhere(
          (chat) => chat.id == 'room_new',
        );
        expect(createdRoom.lastMessage, isNull);

        repo.events.add(
          ChatRoomEventDto.fromJson(
            _roomEventPayload(
              roomId: 'room_a',
              otherUserId: 'user_a',
              otherUsername: 'alice',
              createdAt: DateTime.utc(2026, 6, 2, 10, 0),
              updatedAt: DateTime.utc(2026, 6, 2, 10, 2),
              lastMessageAt: DateTime.utc(2026, 6, 2, 10, 2),
              unreadCount: 1,
              lastMessage: _lastMessagePayload(
                messageId: 'msg_hidden',
                roomId: 'room_a',
                senderId: 'user_a',
                messageType: 'text',
                createdAt: DateTime.utc(2026, 6, 2, 10, 2),
                isHidden: true,
              ),
            ),
            eventType: WebSocketEventType.roomUpdated,
          ),
        );

        await Future<void>.delayed(Duration.zero);

        state = container.read(chatListProvider);
        final hiddenRoom = state.chats.firstWhere(
          (chat) => chat.id == 'room_a',
        );
        expect(hiddenRoom.lastMessage, isNotNull);
        expect(hiddenRoom.lastMessage!.isHidden, isTrue);
        expect(hiddenRoom.lastMessage!.content, isEmpty);
        expect(hiddenRoom.lastMessage!.mediaUrls, isEmpty);
      },
    );

    test(
      'REST refresh still replaces the list snapshot and malformed stream errors do not mutate state',
      () async {
        final roomA = _chat(
          roomId: 'room_a',
          otherUserId: 'user_a',
          otherUsername: 'alice',
          createdAt: DateTime.utc(2026, 6, 2, 10, 0),
          updatedAt: DateTime.utc(2026, 6, 2, 10, 0),
          lastMessageAt: DateTime.utc(2026, 6, 2, 10, 0),
          unreadCount: 1,
        );
        final roomB = _chat(
          roomId: 'room_b',
          otherUserId: 'user_b',
          otherUsername: 'bob',
          createdAt: DateTime.utc(2026, 6, 2, 10, 5),
          updatedAt: DateTime.utc(2026, 6, 2, 10, 5),
          lastMessageAt: DateTime.utc(2026, 6, 2, 10, 5),
          unreadCount: 2,
        );
        final repo = _FakeChatRepository([roomA]);
        final container = ProviderContainer(
          overrides: [chatRepositoryProvider.overrideWithValue(repo)],
        );
        final chatListSubscription = _keepChatListAlive(container);
        addTearDown(chatListSubscription.close);
        addTearDown(container.dispose);

        final notifier = container.read(chatListProvider.notifier);
        await notifier.loadChats('user_me');

        repo.events.add(
          ChatRoomEventDto.fromJson(
            _roomEventPayload(
              roomId: 'room_b',
              otherUserId: 'user_b',
              otherUsername: 'bob',
              createdAt: DateTime.utc(2026, 6, 2, 10, 5),
              updatedAt: DateTime.utc(2026, 6, 2, 10, 6),
              lastMessageAt: DateTime.utc(2026, 6, 2, 10, 6),
              unreadCount: 2,
            ),
            eventType: WebSocketEventType.roomCreated,
          ),
        );

        await Future<void>.delayed(Duration.zero);

        var state = container.read(chatListProvider);
        expect(state.chats, hasLength(2));

        repo.initialChats
          ..clear()
          ..add(roomA);
        repo.initialChats.add(roomB);
        await notifier.loadChats('user_me');

        state = container.read(chatListProvider);
        expect(state.chats, hasLength(2));
        expect(state.chats.map((chat) => chat.id), ['room_b', 'room_a']);

        final before = state.chats;
        repo.events.addError(StateError('malformed room frame'));
        await Future<void>.delayed(Duration.zero);

        state = container.read(chatListProvider);
        expect(state.chats, before);
      },
    );

    test('room events do not affect message-detail state', () async {
      final roomA = _chat(
        roomId: 'room_a',
        otherUserId: 'user_a',
        otherUsername: 'alice',
        createdAt: DateTime.utc(2026, 6, 2, 10, 0),
        updatedAt: DateTime.utc(2026, 6, 2, 10, 0),
        lastMessageAt: DateTime.utc(2026, 6, 2, 10, 0),
        unreadCount: 1,
      );
      final repo = _FakeChatRepository([roomA]);
      final container = ProviderContainer(
        overrides: [chatRepositoryProvider.overrideWithValue(repo)],
      );
      final chatListSubscription = _keepChatListAlive(container);
      addTearDown(chatListSubscription.close);
      addTearDown(container.dispose);

      final listNotifier = container.read(chatListProvider.notifier);
      await listNotifier.loadChats('user_me');

      final detailStateBefore = container.read(chatDetailProvider('room_a'));
      expect(detailStateBefore.messages, isEmpty);

      repo.events.add(
        ChatRoomEventDto.fromJson(
          _roomEventPayload(
            roomId: 'room_a',
            otherUserId: 'user_a',
            otherUsername: 'alice',
            createdAt: DateTime.utc(2026, 6, 2, 10, 0),
            updatedAt: DateTime.utc(2026, 6, 2, 10, 1),
            lastMessageAt: DateTime.utc(2026, 6, 2, 10, 1),
            unreadCount: 0,
          ),
          eventType: WebSocketEventType.roomUpdated,
        ),
      );

      await Future<void>.delayed(Duration.zero);

      final detailStateAfter = container.read(chatDetailProvider('room_a'));
      expect(detailStateAfter.messages, isEmpty);
    });
  });
}
