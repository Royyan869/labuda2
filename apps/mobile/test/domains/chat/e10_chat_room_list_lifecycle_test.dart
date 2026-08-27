// E10 — Chat room list participant lifecycle ingestion and render gate tests.
//
// Scope is pinned to four seams:
//   1) `_readParticipantLifecycles` wire extraction — primary path
//      `other_user.lifecycle` (room-list wire slot), fallback
//      `participant_lifecycles` map, null/empty fall-through.
//   2) Mapper/DTO→entity conversion threads participantLifecycles into the
//      canonical ContentLifecycle on Chat (fail-closed to unavailable for
//      null / missing / unknown — 3-state truth doctrine).
//   3) Axis-boundary contract — room isActive/status (room-axis) and
//      participant lifecycle (identity-axis) are independent; never conflated.
//   4) Chat card render gate — degraded participant triggers redaction
//      placeholder; support rooms bypass the gate; room stays tappable.
//
// Widget-level golden tests would require a full Riverpod harness; the
// render gate is validated by computing the displayed label from a Chat
// entity directly — same lightweight posture as E8.4 / E9.1.
//
// Runtime proof: backend activation confirmed via E4.2 (2026-05-13).
// Mobile render gate active via E4.3 (2026-05-13).

import 'package:flutter_test/flutter_test.dart';

import 'package:labuda/domains/chat/chat/data/dto/chat_dto.dart';
import 'package:labuda/domains/chat/chat/data/mappers/chat_mapper.dart';
import 'package:labuda/domains/chat/chat/domain/entities/chat_entities.dart';
import 'package:labuda/domains/chat/chat/presentation/utils/chat_lifecycle_redaction.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

const _otherUserId = '00000000-0000-0000-0000-000000000002';

Map<String, dynamic> _baseRoomJson({
  Map<String, dynamic>? otherUser,
  Map<String, dynamic>? participantLifecycles,
  String roomType = 'direct',
}) {
  return <String, dynamic>{
    'id': '00000000-0000-0000-0000-000000000010',
    'other_user_id': _otherUserId,
    'room_type': roomType,
    'created_at': '2026-01-01T00:00:00.000Z',
    'other_user': ?otherUser,
    'participant_lifecycles': ?participantLifecycles,
  };
}

/// Direct chat with a known participant lifecycle.
Chat _directChat(String otherUserId, {String? participantLifecycle}) {
  return Chat(
    id: '00000000-0000-0000-0000-000000000010',
    type: ChatType.private,
    participantIds: [otherUserId],
    participantNames: {otherUserId: '@alice'},
    participantAvatars: {otherUserId: null},
    createdAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
    participantLifecycles: participantLifecycle != null
        ? {otherUserId: ContentLifecycleParse.fromWire(participantLifecycle)}
        : const {},
  );
}

/// Support chat (no private participants).
Chat _supportChat() {
  return Chat(
    id: '00000000-0000-0000-0000-000000000011',
    type: ChatType.support,
    participantIds: const [],
    participantNames: const {},
    participantAvatars: const {},
    createdAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
  );
}

/// Reproduce the ChatCard._buildPrivateChatCard render-gate logic so the
/// subtitle redaction contract can be pinned without a Riverpod widget harness.
/// Returns 'support-bypass' for support rooms to make that branch testable.
String _renderParticipantLabel(Chat chat, String currentUserId) {
  if (chat.isSupportChat) return 'support-bypass';
  final lifecycle = chat.getOtherParticipantLifecycle(currentUserId);
  if (!lifecycle.isDegraded) return chat.getOtherParticipantName(currentUserId);
  return chatLifecycleRedactionLabel(lifecycle);
}

void main() {
  // -------------------------------------------------------------------------
  // 1) DTO wire extraction
  // -------------------------------------------------------------------------
  group('E10 — ChatDto participantLifecycles wire extraction', () {
    test('absent other_user → empty map (pre-E4.2 / rollback)', () {
      final dto = ChatDto.fromJson(_baseRoomJson());
      expect(dto.participantLifecycles, isEmpty);
    });

    test('other_user.lifecycle = "active" → {otherUserId: "active"}', () {
      final dto = ChatDto.fromJson(
        _baseRoomJson(
          otherUser: {
            'id': _otherUserId,
            'username': 'alice',
            'lifecycle': 'active',
          },
        ),
      );
      expect(dto.participantLifecycles[_otherUserId], 'active');
    });

    test('other_user.lifecycle = "unavailable"', () {
      final dto = ChatDto.fromJson(
        _baseRoomJson(otherUser: {'lifecycle': 'unavailable'}),
      );
      expect(dto.participantLifecycles[_otherUserId], 'unavailable');
    });

    test('other_user.lifecycle = "removed"', () {
      final dto = ChatDto.fromJson(
        _baseRoomJson(otherUser: {'lifecycle': 'removed'}),
      );
      expect(dto.participantLifecycles[_otherUserId], 'removed');
    });

    test('participant_lifecycles map fallback (no other_user block)', () {
      final dto = ChatDto.fromJson(
        _baseRoomJson(participantLifecycles: {_otherUserId: 'unavailable'}),
      );
      expect(dto.participantLifecycles[_otherUserId], 'unavailable');
    });

    test('empty-string lifecycle in other_user → not in map', () {
      final dto = ChatDto.fromJson(_baseRoomJson(otherUser: {'lifecycle': ''}));
      expect(dto.participantLifecycles, isEmpty);
    });

    test('null lifecycle in other_user → not in map', () {
      final dto = ChatDto.fromJson(
        _baseRoomJson(otherUser: {'lifecycle': null}),
      );
      expect(dto.participantLifecycles, isEmpty);
    });
  });

  // -------------------------------------------------------------------------
  // 2) Mapper threads DTO → entity ContentLifecycle
  // -------------------------------------------------------------------------
  group('E10 — Mapper threads participantLifecycles into Chat entity', () {
    test(
      'empty map → getParticipantLifecycle returns unavailable (FAIL CLOSED)',
      () {
        final dto = ChatDto.fromJson(_baseRoomJson());
        final chat = ChatMapper.toDomain(dto);
        expect(
          chat.getParticipantLifecycle(_otherUserId),
          ContentLifecycle.unavailable,
        );
      },
    );

    test('"active" → ContentLifecycle.active', () {
      final dto = ChatDto.fromJson(
        _baseRoomJson(otherUser: {'lifecycle': 'active'}),
      );
      final chat = ChatMapper.toDomain(dto);
      expect(
        chat.getParticipantLifecycle(_otherUserId),
        ContentLifecycle.active,
      );
    });

    test('"unavailable" → ContentLifecycle.unavailable', () {
      final dto = ChatDto.fromJson(
        _baseRoomJson(otherUser: {'lifecycle': 'unavailable'}),
      );
      final chat = ChatMapper.toDomain(dto);
      expect(
        chat.getParticipantLifecycle(_otherUserId),
        ContentLifecycle.unavailable,
      );
    });

    test('"removed" → ContentLifecycle.removed', () {
      final dto = ChatDto.fromJson(
        _baseRoomJson(otherUser: {'lifecycle': 'removed'}),
      );
      final chat = ChatMapper.toDomain(dto);
      expect(
        chat.getParticipantLifecycle(_otherUserId),
        ContentLifecycle.removed,
      );
    });

    test('unknown wire → ContentLifecycle.unavailable (FAIL CLOSED)', () {
      final dto = ChatDto.fromJson(
        _baseRoomJson(otherUser: {'lifecycle': 'shadowbanned'}),
      );
      final chat = ChatMapper.toDomain(dto);
      expect(
        chat.getParticipantLifecycle(_otherUserId),
        ContentLifecycle.unavailable,
      );
    });
  });

  // -------------------------------------------------------------------------
  // 3) Axis boundary: participant lifecycle vs room state
  // -------------------------------------------------------------------------
  group('E10 — Axis boundary: participant lifecycle vs room isActive/status', () {
    test('room isActive=false does not affect participant lifecycle', () {
      final chat = Chat(
        id: 'r1',
        type: ChatType.private,
        participantIds: [_otherUserId],
        participantNames: const {},
        participantAvatars: const {},
        createdAt: DateTime.now(),
        isActive: false,
        participantLifecycles: {_otherUserId: ContentLifecycle.removed},
      );
      // Participant lifecycle is independently "removed".
      expect(
        chat.getParticipantLifecycle(_otherUserId),
        ContentLifecycle.removed,
      );
      // Room isActive is independently false (room-axis, not identity-axis).
      expect(chat.isActive, isFalse);
    });

    test('room blocked status does not affect participant lifecycle', () {
      final chat = Chat(
        id: 'r2',
        type: ChatType.private,
        participantIds: [_otherUserId],
        participantNames: const {},
        participantAvatars: const {},
        createdAt: DateTime.now(),
        status: ChatStatus.blocked,
        participantLifecycles: {_otherUserId: ContentLifecycle.active},
      );
      // Participant lifecycle is independently active even when room is blocked.
      expect(
        chat.getParticipantLifecycle(_otherUserId),
        ContentLifecycle.active,
      );
      expect(chat.status, ChatStatus.blocked);
    });

    test(
      'active participant + unavailable room status coexist independently',
      () {
        final chat = Chat(
          id: 'r3',
          type: ChatType.private,
          participantIds: [_otherUserId],
          participantNames: const {},
          participantAvatars: const {},
          createdAt: DateTime.now(),
          isActive: false,
          participantLifecycles: {_otherUserId: ContentLifecycle.unavailable},
        );
        expect(
          chat.getParticipantLifecycle(_otherUserId),
          ContentLifecycle.unavailable,
        );
        expect(chat.isActive, isFalse);
      },
    );
  });

  // -------------------------------------------------------------------------
  // 4) Render gate
  // -------------------------------------------------------------------------
  group('E10 — ChatCard participant lifecycle render gate', () {
    test('active participant → real display name unchanged', () {
      final chat = _directChat(_otherUserId, participantLifecycle: 'active');
      // currentUserId = '' matches how ChatCard._getCurrentUserId behaves for
      // room-list items (participantIds has 1 element, length != 2 → returns '').
      expect(_renderParticipantLabel(chat, ''), '@alice');
    });

    test('absent lifecycle → "Pengguna tidak tersedia" (FAIL CLOSED)', () {
      final chat = _directChat(_otherUserId);
      expect(_renderParticipantLabel(chat, ''), 'Pengguna tidak tersedia');
    });

    test('unavailable participant → "Pengguna tidak tersedia"', () {
      final chat = _directChat(
        _otherUserId,
        participantLifecycle: 'unavailable',
      );
      expect(_renderParticipantLabel(chat, ''), 'Pengguna tidak tersedia');
    });

    test('removed participant → "Pengguna dihapus"', () {
      final chat = _directChat(_otherUserId, participantLifecycle: 'removed');
      expect(_renderParticipantLabel(chat, ''), 'Pengguna dihapus');
    });

    test('unknown lifecycle → "Pengguna tidak tersedia" (FAIL CLOSED)', () {
      final chat = _directChat(
        _otherUserId,
        participantLifecycle: 'shadowbanned',
      );
      expect(_renderParticipantLabel(chat, ''), 'Pengguna tidak tersedia');
    });

    test('support room bypasses participant lifecycle gate', () {
      final chat = _supportChat();
      expect(_renderParticipantLabel(chat, 'any-user-id'), 'support-bypass');
    });

    test('room stays tappable regardless of participant lifecycle', () {
      // ChatCard.onTap is never gated by participant lifecycle state.
      // Slot-persistence: the room row stays visible and tappable even when
      // the other participant is banned/deleted. Only the identity display
      // collapses to the redaction placeholder.
      final chat = _directChat(_otherUserId, participantLifecycle: 'removed');
      // Room-axis isActive is the tap gate; it is independent of participant lifecycle.
      expect(chat.isActive, isTrue);
    });

    test('last message field is unaffected by participant lifecycle', () {
      // The chat card always renders the last message text regardless of
      // participant lifecycle (slot-persistence on the message display too).
      final chat = Chat(
        id: '00000000-0000-0000-0000-000000000010',
        type: ChatType.private,
        participantIds: [_otherUserId],
        participantNames: {_otherUserId: '@alice'},
        participantAvatars: {_otherUserId: null},
        createdAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
        participantLifecycles: {_otherUserId: ContentLifecycle.removed},
        lastMessage: Message(
          id: 'msg-1',
          chatId: '00000000-0000-0000-0000-000000000010',
          senderId: _otherUserId,
          senderName: '@alice',
          content: 'hello',
          type: MessageType.text,
          createdAt: DateTime.parse('2026-01-01T00:00:01.000Z'),
          status: MessageStatus.sent,
        ),
      );
      // Participant degraded but last message is still present.
      expect(chat.lastMessage, isNotNull);
      expect(chat.lastMessage!.content, 'hello');
    });
  });
}
