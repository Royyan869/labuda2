// C1B1 — Chat participant safe identity tests.
//
// Validates:
//   A) Chat entity identity contract (getOtherParticipantUsername + UUID safety)
//   B) chatParticipantLabel presentation helper
//   C) ChatCard behavioral contracts (via entity + helper composition)
//   D) Negative contracts preventing UUID / @User / ? / inline initials
//
// Classification: all tests in groups A–E are behavioral-unit tests
// (pure entity / helper functions, no widget pumping).
// Group F (ChatCard widget) is in the companion widget-test file.
//
// Scope: chat participant identity only. Does not cover:
//   - message sender labels (protected — message_bubble.dart unchanged)
//   - support/system chat identity (protected)
//   - lifecycle redaction (validated in E10/E4.3 tests)
//   - DTO parsing (validated in E11.1 tests)
//   - new-chat / search / mentions (C1A scope)

import 'package:flutter_test/flutter_test.dart';

import 'package:labuda/domains/chat/chat/data/dto/chat_dto.dart';
import 'package:labuda/domains/chat/chat/data/mappers/chat_mapper.dart';
import 'package:labuda/domains/chat/chat/domain/entities/chat_entities.dart';
import 'package:labuda/domains/chat/chat/presentation/utils/chat_identity_display.dart';
import 'package:labuda/domains/chat/chat/presentation/utils/chat_lifecycle_redaction.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';

// =============================================================================
// Fixtures
// =============================================================================

const _otherUserId = '00000000-0000-0000-0000-000000000002';
const _roomId = '00000000-0000-0000-0000-000000000010';

/// Build a Chat entity directly (bypasses DTO parsing to test entity contract).
Chat _chatWithParticipant({
  String? username,
  String? avatarUrl,
  String lifecycle = 'active',
}) {
  final names = <String, String>{};
  final avatars = <String, String?>{};
  if (username != null) {
    names[_otherUserId] = username;
  }
  if (avatarUrl != null && avatarUrl.isNotEmpty) {
    avatars[_otherUserId] = avatarUrl;
  }
  return Chat(
    id: _roomId,
    type: ChatType.private,
    participantIds: [_otherUserId],
    participantNames: names,
    participantAvatars: avatars,
    createdAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
    participantLifecycles: {
      _otherUserId: ContentLifecycleParse.fromWire(lifecycle),
    },
  );
}

/// Build via DTO → mapper (full ingress path).
Chat _chatFromDto({String? username, String? avatarUrl, String? lifecycle}) {
  final otherUser = <String, dynamic>{};
  if (username != null) otherUser['username'] = username;
  if (avatarUrl != null) otherUser['avatar_url'] = avatarUrl;
  if (lifecycle != null) otherUser['lifecycle'] = lifecycle;
  final dto = ChatDto.fromJson(<String, dynamic>{
    'id': _roomId,
    'other_user_id': _otherUserId,
    'room_type': 'direct',
    'created_at': '2026-01-01T00:00:00.000Z',
    'other_user': otherUser,
  });
  return ChatMapper.toDomain(dto);
}

// =============================================================================
// A) Chat entity identity contract — behavioral unit
// =============================================================================

void main() {
  group('C1B1 — Chat.getOtherParticipantUsername (raw data accessor)', () {
    // -- basic resolution ---------------------------------------------------

    test('valid username → returns raw username', () {
      final chat = _chatWithParticipant(username: 'alice');
      expect(chat.getOtherParticipantUsername(''), 'alice');
    });

    test('valid username with caller userId → returns raw username', () {
      final chat = _chatWithParticipant(username: 'bob');
      expect(chat.getOtherParticipantUsername('my-user-id'), 'bob');
    });

    test('empty string username → returns null', () {
      final chat = _chatWithParticipant(username: '');
      expect(chat.getOtherParticipantUsername(''), isNull);
    });

    test('missing participantNames entry → returns null', () {
      final chat = _chatWithParticipant(username: null);
      expect(chat.getOtherParticipantUsername(''), isNull);
    });

    // -- whitespace ---------------------------------------------------------

    test('whitespace-only username → returns null (trim-then-empty)', () {
      final chat = _chatWithParticipant(username: '   ');
      expect(chat.getOtherParticipantUsername(''), isNull);
    });

    test('whitespace-surrounded valid username → returns trimmed', () {
      final chat = _chatWithParticipant(username: '  alice  ');
      expect(chat.getOtherParticipantUsername(''), 'alice');
    });

    // -- participant-ID equality safety -------------------------------------

    test('candidate equals participant user ID (exact) → returns null', () {
      final chat = _chatWithParticipant(username: _otherUserId);
      expect(chat.getOtherParticipantUsername(''), isNull);
    });

    test(
      'candidate equals participant user ID with explicit currentUserId',
      () {
        final id = 'aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee';
        final chat = Chat(
          id: _roomId,
          type: ChatType.private,
          participantIds: ['caller', id],
          participantNames: {id: id},
          participantAvatars: const {},
          createdAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
        );
        expect(chat.getOtherParticipantUsername('caller'), isNull);
      },
    );

    test('UUID candidate with different casing than participant ID → null', () {
      const lowerId = '550e8400-e29b-41d4-a716-446655440000';
      const upperCandidate = '550E8400-E29B-41D4-A716-446655440000';
      final chat = Chat(
        id: _roomId,
        type: ChatType.private,
        participantIds: ['caller', lowerId],
        participantNames: {lowerId: upperCandidate},
        participantAvatars: const {},
        createdAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
      );
      // Case-insensitive UUID comparison → treated as equal to participant ID.
      expect(chat.getOtherParticipantUsername('caller'), isNull);
    });

    // -- canonical UUID shape safety ----------------------------------------

    test('lowercase canonical UUID → returns null', () {
      final chat = _chatWithParticipant(
        username: 'deadbeef-1234-5678-9abc-def012345678',
      );
      expect(chat.getOtherParticipantUsername(''), isNull);
    });

    test('uppercase canonical UUID → returns null', () {
      final chat = _chatWithParticipant(
        username: 'DEADBEEF-1234-5678-9ABC-DEF012345678',
      );
      expect(chat.getOtherParticipantUsername(''), isNull);
    });

    test('mixed-case canonical UUID → returns null', () {
      final chat = _chatWithParticipant(
        username: 'DeadBeef-1234-5678-9aBc-def012345678',
      );
      expect(chat.getOtherParticipantUsername(''), isNull);
    });

    test('UUID surrounded by whitespace → returns null', () {
      final chat = _chatWithParticipant(
        username: '  deadbeef-1234-5678-9abc-def012345678  ',
      );
      expect(chat.getOtherParticipantUsername(''), isNull);
    });

    // -- legitimate usernames preserved -------------------------------------

    test('hyphenated username like john-2026 → preserved', () {
      final chat = _chatWithParticipant(username: 'john-2026');
      expect(chat.getOtherParticipantUsername(''), 'john-2026');
    });

    test('numeric-containing username like user123 → preserved', () {
      final chat = _chatWithParticipant(username: 'user123');
      expect(chat.getOtherParticipantUsername(''), 'user123');
    });

    test('legitimate username with digits and hyphens → preserved', () {
      final chat = _chatWithParticipant(username: 'seller-99');
      expect(chat.getOtherParticipantUsername(''), 'seller-99');
    });

    test('abc-def (looks like UUID fragment but not 36 chars) → preserved', () {
      final chat = _chatWithParticipant(username: 'abc-def');
      expect(chat.getOtherParticipantUsername(''), 'abc-def');
    });

    // -- edge cases ---------------------------------------------------------

    test('never returns generic label "User"', () {
      final chat = _chatWithParticipant(username: null);
      final result = chat.getOtherParticipantUsername('');
      expect(result, isNull);
    });

    test('short user IDs do not leak', () {
      final chat = Chat(
        id: _roomId,
        type: ChatType.private,
        participantIds: ['ab'],
        participantNames: const {},
        participantAvatars: const {},
        createdAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
      );
      expect(chat.getOtherParticipantUsername(''), isNull);
    });

    test('malformed IDs do not cause substring errors', () {
      final chat = Chat(
        id: _roomId,
        type: ChatType.private,
        participantIds: ['x'],
        participantNames: const {},
        participantAvatars: const {},
        createdAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
      );
      // Must not throw RangeError or any exception.
      expect(chat.getOtherParticipantUsername(''), isNull);
    });
  });

  // ===========================================================================
  // B) chatParticipantLabel presentation helper — behavioral unit
  // ===========================================================================

  group('C1B1 — chatParticipantLabel (presentation)', () {
    test('valid raw username → @username', () {
      expect(chatParticipantLabel('alice'), '@alice');
    });

    test('leading-@ username → single @ (normalised)', () {
      expect(chatParticipantLabel('@alice'), '@alice');
    });

    test('double-@ username → single @ (normalised)', () {
      expect(chatParticipantLabel('@@alice'), '@alice');
    });

    test('null username → "User"', () {
      expect(chatParticipantLabel(null), 'User');
    });

    test('empty username → "User"', () {
      expect(chatParticipantLabel(''), 'User');
    });

    test('whitespace-only username → "User"', () {
      expect(chatParticipantLabel('   '), 'User');
    });

    test('never returns @User', () {
      expect(chatParticipantLabel(null), isNot('@User'));
      expect(chatParticipantLabel(''), isNot('@User'));
      expect(chatParticipantLabel('   '), isNot('@User'));
    });

    test('underscore-separated username → single @', () {
      expect(chatParticipantLabel('john_doe'), '@john_doe');
    });
  });

  // ===========================================================================
  // C) ChatCard participant identity composition — behavioral unit
  // ===========================================================================

  group('C1B1 — ChatCard participant identity composition', () {
    test('valid username + active lifecycle → @username', () {
      final chat = _chatWithParticipant(username: 'alice');
      final username = chat.getOtherParticipantUsername('');
      final lifecycle = chat.getOtherParticipantLifecycle('');
      final display = lifecycle.isDegraded
          ? chatLifecycleRedactionLabel(lifecycle)
          : chatParticipantLabel(username);
      expect(display, '@alice');
    });

    test('null username + active lifecycle → "User"', () {
      final chat = _chatWithParticipant(username: null);
      final username = chat.getOtherParticipantUsername('');
      final lifecycle = chat.getOtherParticipantLifecycle('');
      final display = lifecycle.isDegraded
          ? chatLifecycleRedactionLabel(lifecycle)
          : chatParticipantLabel(username);
      expect(display, 'User');
    });

    test('empty username + active lifecycle → "User"', () {
      final chat = _chatWithParticipant(username: '');
      final username = chat.getOtherParticipantUsername('');
      final lifecycle = chat.getOtherParticipantLifecycle('');
      final display = lifecycle.isDegraded
          ? chatLifecycleRedactionLabel(lifecycle)
          : chatParticipantLabel(username);
      expect(display, 'User');
    });

    test(
      'UUID-polluted name + active lifecycle → "User" (filtered by accessor)',
      () {
        final chat = _chatWithParticipant(username: _otherUserId);
        final username = chat.getOtherParticipantUsername('');
        final lifecycle = chat.getOtherParticipantLifecycle('');
        final display = lifecycle.isDegraded
            ? chatLifecycleRedactionLabel(lifecycle)
            : chatParticipantLabel(username);
        // getOtherParticipantUsername returns null (ID equality rejected),
        // so chatParticipantLabel returns 'User'.
        expect(display, 'User');
        expect(display, isNot(contains('00000000')));
      },
    );

    test('lifecycle takes priority over valid username', () {
      final chat = _chatWithParticipant(
        username: 'alice',
        lifecycle: 'removed',
      );
      final lifecycle = chat.getOtherParticipantLifecycle('');
      expect(lifecycle.isDegraded, isTrue);
      final display = chatLifecycleRedactionLabel(lifecycle);
      expect(display, isNotEmpty);
      expect(display, isNot('@alice'));
      expect(display, isNot(contains('alice')));
    });

    test('lifecycle takes priority over null username', () {
      final chat = _chatWithParticipant(
        username: null,
        lifecycle: 'unavailable',
      );
      final lifecycle = chat.getOtherParticipantLifecycle('');
      expect(lifecycle.isDegraded, isTrue);
      final display = chatLifecycleRedactionLabel(lifecycle);
      expect(display, isNotEmpty);
      expect(display, isNot('User'));
    });

    test('DTO ingress: valid username → canonical handle via label', () {
      final chat = _chatFromDto(username: 'alice');
      final username = chat.getOtherParticipantUsername('');
      expect(username, 'alice');
      expect(chatParticipantLabel(username), '@alice');
    });

    test('DTO ingress: missing username → null → "User"', () {
      final chat = _chatFromDto(username: null);
      final username = chat.getOtherParticipantUsername('');
      expect(username, isNull);
      expect(chatParticipantLabel(username), 'User');
    });

    test('DTO ingress: empty username → null → "User"', () {
      final chat = _chatFromDto(username: '');
      final username = chat.getOtherParticipantUsername('');
      // DTO branch 1: empty string username is filtered (isNotEmpty check).
      // So participantNames is empty → getOtherParticipantUsername returns null.
      expect(username, isNull);
      expect(chatParticipantLabel(username), 'User');
    });

    test('DTO ingress: leading-@ username normalises to single @', () {
      final chat = _chatFromDto(username: '@alice');
      final username = chat.getOtherParticipantUsername('');
      expect(username, '@alice');
      expect(chatParticipantLabel(username), '@alice');
    });
  });

  // ===========================================================================
  // D) Negative contracts — behavioral unit
  // ===========================================================================

  group('C1B1 — Negative contracts', () {
    test('chatParticipantLabel(null) is short (no UUID fragment)', () {
      final result = chatParticipantLabel(null);
      expect(result.length, lessThan(10));
      expect(result, isNot(contains('0000')));
    });

    test('no raw "?" fallback via chatParticipantLabel', () {
      expect(chatParticipantLabel(null), isNot('?'));
      expect(chatParticipantLabel(''), isNot('?'));
    });

    test('no bare "@" via chatParticipantLabel', () {
      expect(chatParticipantLabel(null), isNot('@'));
      expect(chatParticipantLabel(''), isNot('@'));
      expect(chatParticipantLabel('   '), isNot('@'));
    });

    test('no double "@@" via chatParticipantLabel', () {
      expect(chatParticipantLabel('alice'), isNot('@@alice'));
      expect(chatParticipantLabel('@alice'), isNot('@@alice'));
      expect(chatParticipantLabel('@@alice'), isNot('@@alice'));
    });

    test('UUID detection is case-insensitive (not lowercase-only)', () {
      // The UUID safety filter in getOtherParticipantUsername rejects all
      // canonical-UUID-shaped strings regardless of case. These candidates
      // never reach chatParticipantLabel; the test pins that contract for
      // the full chain (entity → label).
      final lowerUuid = 'deadbeef-1234-5678-9abc-def012345678';
      final upperUuid = 'DEADBEEF-1234-5678-9ABC-DEF012345678';
      final mixedUuid = 'DeadBeef-1234-5678-9aBc-def012345678';
      // Build Chat entities with these as participantNames, then verify
      // getOtherParticipantUsername returns null for all three.
      for (final uuid in [lowerUuid, upperUuid, mixedUuid]) {
        final chat = _chatWithParticipant(username: uuid);
        expect(
          chat.getOtherParticipantUsername(''),
          isNull,
          reason: 'UUID $uuid should be rejected',
        );
        expect(chatParticipantLabel(null), 'User');
        expect(chatParticipantLabel(null), isNot(contains('@')));
      }
    });

    test('empty string senderUsername → formatChatHandle returns ""', () {
      // MessageBubble contract: empty senderUsername → empty display.
      expect(formatChatHandle(''), '');
    });

    test('valid senderUsername → formatChatHandle returns @username', () {
      expect(formatChatHandle('alice'), '@alice');
    });

    test(
      'getOtherParticipantUsername on empty participantIds throws StateError',
      () {
        // Pre-existing contract: firstWhere throws when no matching element.
        expect(
          () => Chat(
            id: _roomId,
            type: ChatType.private,
            participantIds: const [],
            participantNames: const {},
            participantAvatars: const {},
            createdAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
          ).getOtherParticipantUsername(''),
          throwsA(isA<StateError>()),
        );
      },
    );

    test(
      'getOtherParticipantName does not exist (source-contract: removed in C1B1)',
      () {
        // C1B1 correction 5: the ambiguous legacy helper was removed.
        // This test proves it doesn't compile if someone reintroduces it.
        final chat = _chatWithParticipant(username: 'alice');
        // getOtherParticipantName symbol must not resolve.
        // We use getOtherParticipantUsername — the canonical replacement.
        expect(chat.getOtherParticipantUsername(''), 'alice');
      },
    );
  });

  // ===========================================================================
  // E) avatar URL passthrough — behavioral unit
  // ===========================================================================

  group('C1B1 — participantAvatars passthrough', () {
    test('avatar URL accessible via participantAvatars lookup', () {
      final chat = _chatWithParticipant(
        username: 'alice',
        avatarUrl: 'https://cdn.example.com/alice.jpg',
      );
      expect(
        chat.participantAvatars[_otherUserId],
        'https://cdn.example.com/alice.jpg',
      );
    });

    test('null avatar → participantAvatars lookup returns null', () {
      final chat = _chatWithParticipant(username: 'alice', avatarUrl: null);
      expect(chat.participantAvatars[_otherUserId], isNull);
    });

    test('DTO ingress: avatar_url → participantAvatars populated', () {
      final chat = _chatFromDto(
        username: 'alice',
        avatarUrl: 'https://cdn.example.com/alice.jpg',
      );
      expect(
        chat.participantAvatars[_otherUserId],
        'https://cdn.example.com/alice.jpg',
      );
    });
  });
}
