import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/websocket/websocket_message.dart';
import 'package:labuda/domains/chat/chat/data/dto/chat_room_event_dto.dart';
import 'package:labuda/domains/chat/chat/data/dto/message_dto.dart';

void main() {
  test('WebSocketMessage.fromJson parses canonical backend chat envelope', () {
    final frame = {
      'id': 'evt-1',
      'type': 'chat.message.sent',
      'timestamp': '2026-06-01T00:00:00Z',
      'from': 'server',
      'data': {'room_id': 'room-1', 'message_id': 'msg-1'},
    };

    final message = WebSocketMessage.fromJson(frame);

    expect(message.id, 'evt-1');
    expect(message.type, 'chat.message.sent');
    expect(message.from, 'server');
    expect(message.data['room_id'], 'room-1');
    expect(message.data['message_id'], 'msg-1');
  });

  test('WebSocketEventType maps chat.message.sent as messageNew', () {
    final type = WebSocketEventType.fromString('chat.message.sent');
    expect(type, WebSocketEventType.messageNew);
  });

  test('WebSocketEventType treats legacy message.updated as unknown', () {
    expect(
      WebSocketEventType.fromString('message.updated'),
      WebSocketEventType.unknown,
    );
  });

  test('WebSocketEventType maps hide/restore chat signals', () {
    expect(
      WebSocketEventType.fromString('chat.message.hidden'),
      WebSocketEventType.messageHidden,
    );
    expect(
      WebSocketEventType.fromString('chat.message.restored'),
      WebSocketEventType.messageRestored,
    );
  });

  test('WebSocketEventType maps room summary signals', () {
    expect(
      WebSocketEventType.fromString('chat.room.created'),
      WebSocketEventType.roomCreated,
    );
    expect(
      WebSocketEventType.fromString('chat.room.updated'),
      WebSocketEventType.roomUpdated,
    );
  });

  test('WebSocketEventType treats message.deleted as unknown', () {
    expect(
      WebSocketEventType.fromString('message.deleted'),
      WebSocketEventType.unknown,
    );
  });

  test('WebSocketEventType treats chat.room.removed as unknown', () {
    expect(
      WebSocketEventType.fromString('chat.room.removed'),
      WebSocketEventType.unknown,
    );
  });

  test('ChatRoomEventDto parses created payload with null last_message', () {
    final event = WebSocketEventDto.fromJson({
      'id': 'evt-room-created',
      'type': 'chat.room.created',
      'timestamp': '2026-06-01T00:00:00Z',
      'from': 'server',
      'payload': {
        'room_id': 'room-1',
        'room_type': 'direct',
        'other_user_id': 'user-2',
        'other_user': {
          'id': 'user-2',
          'username': 'alice',
          'avatar_url': 'https://cdn.example/avatar.png',
          'lifecycle': 'active',
        },
        'context': {'kind': 'listing', 'id': 'listing-1'},
        'context_set_by': 'user-1',
        'linked_order_id': 'order-1',
        'last_message': null,
        'unread_count': 0,
        'created_at': '2026-06-01T00:00:00Z',
        'updated_at': '2026-06-01T00:00:00Z',
        'last_message_at': '2026-06-01T00:00:00Z',
      },
    });

    final room = ChatRoomEventDto.fromWebSocketEvent(event);

    expect(room.eventType, WebSocketEventType.roomCreated);
    expect(room.roomId, 'room-1');
    expect(room.roomType, 'direct');
    expect(room.otherUserId, 'user-2');
    expect(room.otherUser, isNotNull);
    expect(room.otherUser!.username, 'alice');
    expect(room.otherUser!.lifecycle, 'active');
    expect(room.context, isNotNull);
    expect(room.lastMessage, isNull);
    expect(room.unreadCount, 0);
  });

  test('ChatRoomEventDto parses updated payload with visible last_message', () {
    final event = WebSocketEventDto.fromJson({
      'id': 'evt-room-updated',
      'type': 'chat.room.updated',
      'timestamp': '2026-06-01T00:00:01Z',
      'from': 'server',
      'payload': {
        'room_id': 'room-1',
        'room_type': 'direct',
        'other_user_id': 'user-2',
        'other_user': {'id': 'user-2', 'username': 'alice'},
        'last_message': {
          'id': 'msg-1',
          'room_id': 'room-1',
          'sender_id': 'user-2',
          'message_type': 'text',
          'body': 'hello',
          'created_at': '2026-06-01T00:00:01Z',
        },
        'unread_count': 3,
        'created_at': '2026-06-01T00:00:00Z',
        'updated_at': '2026-06-01T00:00:01Z',
        'last_message_at': '2026-06-01T00:00:01Z',
      },
    });

    final room = ChatRoomEventDto.fromWebSocketEvent(event);

    expect(room.eventType, WebSocketEventType.roomUpdated);
    expect(room.lastMessage, isNotNull);
    expect(room.lastMessage!.body, 'hello');
    expect(room.lastMessage!.isHidden, isFalse);
  });

  test('ChatRoomEventDto parses hidden tombstone without body leak', () {
    final event = WebSocketEventDto.fromJson({
      'id': 'evt-room-hidden',
      'type': 'chat.room.updated',
      'timestamp': '2026-06-01T00:00:02Z',
      'from': 'server',
      'payload': {
        'room_id': 'room-1',
        'room_type': 'direct',
        'other_user_id': 'user-2',
        'last_message': {
          'id': 'msg-hidden',
          'room_id': 'room-1',
          'sender_id': 'user-2',
          'message_type': 'text',
          'is_hidden': true,
          'created_at': '2026-06-01T00:00:02Z',
        },
        'unread_count': 1,
        'created_at': '2026-06-01T00:00:00Z',
        'updated_at': '2026-06-01T00:00:02Z',
        'last_message_at': '2026-06-01T00:00:02Z',
      },
    });

    final room = ChatRoomEventDto.fromWebSocketEvent(event);

    expect(room.lastMessage, isNotNull);
    expect(room.lastMessage!.isHidden, isTrue);
    expect(room.lastMessage!.body, isNull);
    expect(room.lastMessage!.attachmentJson, isNull);
  });
}
