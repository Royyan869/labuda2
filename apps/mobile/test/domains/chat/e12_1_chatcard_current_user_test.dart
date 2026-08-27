// E12.1 — ChatCard current-user injection fix tests.
//
// Validates that entity helpers and Message.isFromUser behave correctly when
// the real authenticated currentUserId (from currentUserIdProvider) is used
// instead of the old _getCurrentUserId() placeholder that returned '' for all
// branch-1 rooms (participantIds.length == 1).
//
// Branch-1 topology: participantIds = [otherUserId] (self excluded).
// With a real selfUserId, every entity helper must still resolve the correct
// OTHER participant — they do, because firstWhere(id != selfId) on a
// single-element list returns the one element when it differs from selfId.
//
// The primary fix is Message.isFromUser: with '' it was always false; with
// the real selfUserId it correctly returns true for self-authored messages,
// enabling the "You: " prefix in the room list last-message preview.
//
// Scope: entity helpers + Message.isFromUser.
// Widget layer (ChatCard itself) requires a full Riverpod harness and is not
// covered here — the logic under test is pure Dart.
// E11.1 + E10 remain the authorities for participant display and lifecycle.

import 'package:flutter_test/flutter_test.dart';

import 'package:labuda/domains/chat/chat/data/dto/chat_dto.dart';
import 'package:labuda/domains/chat/chat/data/mappers/chat_mapper.dart';
import 'package:labuda/domains/chat/chat/domain/entities/chat_entities.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

const _selfUserId = '00000000-0000-0000-0000-000000000001';
const _otherUserId = '00000000-0000-0000-0000-000000000002';

/// Branch-1 JSON payload (triggered by other_user_id key).
/// participantIds will be [_otherUserId]; self is excluded by the backend.
Map<String, dynamic> _roomJson({
  Map<String, dynamic>? otherUser,
  String roomType = 'direct',
}) => <String, dynamic>{
  'id': '00000000-0000-0000-0000-000000000010',
  'other_user_id': _otherUserId,
  'room_type': roomType,
  'created_at': '2026-01-01T00:00:00.000Z',
  'other_user': ?otherUser,
};

/// Build a Chat with a last message authored by [senderId].
Chat _chatWithLastMessage({
  required String senderId,
  String content = 'hello',
  Map<String, String>? participantNames,
  Map<String, ContentLifecycle>? participantLifecycles,
}) {
  return Chat(
    id: '00000000-0000-0000-0000-000000000010',
    type: ChatType.private,
    participantIds: const [_otherUserId],
    participantNames: participantNames ?? const {_otherUserId: 'alice'},
    participantAvatars: const {_otherUserId: null},
    createdAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
    participantLifecycles: participantLifecycles ?? const {},
    lastMessage: Message(
      id: 'msg-1',
      chatId: '00000000-0000-0000-0000-000000000010',
      senderId: senderId,
      senderName: senderId == _selfUserId ? 'me' : 'alice',
      content: content,
      type: MessageType.text,
      createdAt: DateTime.parse('2026-01-01T00:00:01.000Z'),
      status: MessageStatus.sent,
    ),
  );
}

/// Simulate ChatCard._buildLastMessage "You: " logic — pure Dart, no widget harness.
String _lastMessageLabel(Chat chat, String currentUserId) {
  if (chat.lastMessage == null) return 'No messages yet';
  final msg = chat.lastMessage!;
  final prefix = msg.isFromUser(currentUserId) ? 'You: ' : '';
  return '$prefix${msg.content}';
}

// ---------------------------------------------------------------------------

void main() {
  // -------------------------------------------------------------------------
  // 1) Entity helpers with real currentUserId (branch-1 topology)
  // -------------------------------------------------------------------------
  group(
    'E12.1 — Entity helpers with real currentUserId (branch-1 topology)',
    () {
      late Chat chat;

      setUp(() {
        final dto = ChatDto.fromJson(
          _roomJson(
            otherUser: {
              'id': _otherUserId,
              'username': 'alice',
              'avatar_url': 'https://cdn.example.com/alice.jpg',
              'lifecycle': 'active',
            },
          ),
        );
        chat = ChatMapper.toDomain(dto);
      });

      test('getOtherParticipantId(selfId) returns otherUserId', () {
        expect(chat.getOtherParticipantId(_selfUserId), _otherUserId);
      });

      test('getOtherParticipantName(selfId) returns real username', () {
        expect(chat.getOtherParticipantName(_selfUserId), 'alice');
      });

      test(
        'getOtherParticipantLifecycle(selfId) returns correct lifecycle',
        () {
          expect(
            chat.getOtherParticipantLifecycle(_selfUserId),
            ContentLifecycle.active,
          );
        },
      );

      test('participantAvatars[otherUserId] accessible', () {
        expect(
          chat.participantAvatars[_otherUserId],
          'https://cdn.example.com/alice.jpg',
        );
      });

      test('degraded lifecycle redacted via real selfId', () {
        final dto = ChatDto.fromJson(
          _roomJson(otherUser: {'username': 'alice', 'lifecycle': 'removed'}),
        );
        final degradedChat = ChatMapper.toDomain(dto);
        expect(
          degradedChat.getOtherParticipantLifecycle(_selfUserId).isDegraded,
          isTrue,
        );
      });
    },
  );

  // -------------------------------------------------------------------------
  // 2) Entity helpers with '' fallback — regression anchor
  // -------------------------------------------------------------------------
  group('E12.1 — Entity helpers with empty-string fallback remain correct', () {
    // With participantIds = [otherUserId] and currentUserId = '',
    // firstWhere(id != '') returns otherUserId. This is coincidentally correct
    // but fragile — the fix eliminates the need for this coincidence.
    // Tests kept as regression anchors for the graceful-fallback contract.
    test('getOtherParticipantId("") still returns otherUserId', () {
      final dto = ChatDto.fromJson(_roomJson(otherUser: {'username': 'alice'}));
      final chat = ChatMapper.toDomain(dto);
      expect(chat.getOtherParticipantId(''), _otherUserId);
    });

    test('getOtherParticipantName("") still returns real username', () {
      final dto = ChatDto.fromJson(_roomJson(otherUser: {'username': 'alice'}));
      final chat = ChatMapper.toDomain(dto);
      expect(chat.getOtherParticipantName(''), 'alice');
    });
  });

  // -------------------------------------------------------------------------
  // 3) "You: " prefix — the primary fix
  // -------------------------------------------------------------------------
  group('E12.1 — Last message "You: " prefix with real currentUserId', () {
    test('self-authored message → "You: hello"', () {
      final chat = _chatWithLastMessage(senderId: _selfUserId);
      expect(_lastMessageLabel(chat, _selfUserId), 'You: hello');
    });

    test('other-authored message → "hello" (no prefix)', () {
      final chat = _chatWithLastMessage(senderId: _otherUserId);
      expect(_lastMessageLabel(chat, _selfUserId), 'hello');
    });

    test(
      'self-authored + empty-string fallback → "hello" (pre-fix behavior preserved as fallback)',
      () {
        // With '' as currentUserId, isFromUser(_selfUserId) is never true.
        // This is the old broken behavior, now a documented fallback state.
        final chat = _chatWithLastMessage(senderId: _selfUserId);
        expect(_lastMessageLabel(chat, ''), 'hello');
      },
    );

    test('other-authored + empty-string fallback → "hello" (unchanged)', () {
      final chat = _chatWithLastMessage(senderId: _otherUserId);
      expect(_lastMessageLabel(chat, ''), 'hello');
    });

    test('no last message → "No messages yet"', () {
      final chat = Chat(
        id: '00000000-0000-0000-0000-000000000010',
        type: ChatType.private,
        participantIds: const [_otherUserId],
        participantNames: const {_otherUserId: 'alice'},
        participantAvatars: const {},
        createdAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
      );
      expect(_lastMessageLabel(chat, _selfUserId), 'No messages yet');
    });
  });

  // -------------------------------------------------------------------------
  // 4) Message.isFromUser — unit
  // -------------------------------------------------------------------------
  group('E12.1 — Message.isFromUser unit', () {
    late Message selfMsg;
    late Message otherMsg;

    setUp(() {
      selfMsg = Message(
        id: 'msg-self',
        chatId: 'chat-1',
        senderId: _selfUserId,
        senderName: 'me',
        content: 'hi',
        type: MessageType.text,
        createdAt: DateTime.parse('2026-01-01T00:00:01.000Z'),
        status: MessageStatus.sent,
      );
      otherMsg = Message(
        id: 'msg-other',
        chatId: 'chat-1',
        senderId: _otherUserId,
        senderName: 'alice',
        content: 'hi back',
        type: MessageType.text,
        createdAt: DateTime.parse('2026-01-01T00:00:02.000Z'),
        status: MessageStatus.sent,
      );
    });

    test('selfMsg.isFromUser(selfId) → true', () {
      expect(selfMsg.isFromUser(_selfUserId), isTrue);
    });

    test('otherMsg.isFromUser(selfId) → false', () {
      expect(otherMsg.isFromUser(_selfUserId), isFalse);
    });

    test(
      'selfMsg.isFromUser("") → false (empty string never matches UUID)',
      () {
        expect(selfMsg.isFromUser(''), isFalse);
      },
    );

    test('otherMsg.isFromUser("") → false', () {
      expect(otherMsg.isFromUser(''), isFalse);
    });
  });

  // -------------------------------------------------------------------------
  // 5) Support room — entity parity unchanged
  // -------------------------------------------------------------------------
  group('E12.1 — Support room entity parity', () {
    test('support room DTO parse is not affected by fix', () {
      final dto = ChatDto.fromJson(
        _roomJson(
          roomType: 'support',
          otherUser: {'username': 'admin_user', 'lifecycle': 'active'},
        ),
      );
      expect(dto.type, 'support');
      // participantIds and names still populated correctly
      expect(dto.participantIds, contains(_otherUserId));
      expect(dto.participantNames[_otherUserId], 'admin_user');
    });

    test(
      'support room last message + real selfId: "You: " works for self-authored',
      () {
        final chat = Chat(
          id: '00000000-0000-0000-0000-000000000011',
          type: ChatType.support,
          participantIds: const [_otherUserId],
          participantNames: const {},
          participantAvatars: const {},
          createdAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
          lastMessage: Message(
            id: 'msg-s',
            chatId: '00000000-0000-0000-0000-000000000011',
            senderId: _selfUserId,
            senderName: 'me',
            content: 'need help',
            type: MessageType.text,
            createdAt: DateTime.parse('2026-01-01T00:00:01.000Z'),
            status: MessageStatus.sent,
          ),
        );
        expect(_lastMessageLabel(chat, _selfUserId), 'You: need help');
      },
    );
  });
}
