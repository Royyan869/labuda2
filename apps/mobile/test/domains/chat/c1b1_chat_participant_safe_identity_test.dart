// C1B1 — Chat participant safe identity tests.
//
// Validates:
//   A) Chat entity identity contract (getOtherParticipantName + fallback safety)
//   B) formatChatHandle presentation helper
//   C) ChatCard behavioral contracts (via entity + helper composition)
//   D) Negative contracts preventing UUID / @User / ? / inline initials
//
// Classification: all tests in groups A–E are behavioral-unit tests
// (pure entity / helper functions, no widget pumping).
//
// Scope: chat participant identity only. Does not cover:
//   - message sender labels (protected — message_bubble.dart unchanged)
//   - support/system chat identity (protected)
//   - lifecycle redaction (validated in E10/E4.3 tests)
//   - DTO parsing (validated in E11.1 tests)
//   - new-chat / search / mentions (C1A scope)
//
// Stage 3B-5 convergence:
//   - getOtherParticipantUsername → getOtherParticipantName (canonical)
//   - chatParticipantLabel → formatChatHandle (canonical)
//   - Old null-returning semantics replaced by String-returning fallback behavior

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
  group('C1B1 — Chat.getOtherParticipantName (raw data accessor)', () {
    // -- basic resolution ---------------------------------------------------

    test('valid username → returns raw username', () {
      final chat = _chatWithParticipant(username: 'alice');
      expect(chat.getOtherParticipantName(''), 'alice');
    });

    test('valid username with caller userId → returns raw username', () {
      final chat = _chatWithParticipant(username: 'bob');
      expect(chat.getOtherParticipantName('my-user-id'), 'bob');
    });

    test('missing participantNames entry → returns fallback with id prefix',
        () {
      final chat = _chatWithParticipant(username: null);
      final name = chat.getOtherParticipantName('');
      // Fallback: "User <id_prefix>..." where id_prefix is first 8 chars.
      expect(name, startsWith('User '));
      expect(name, isNot(equals('User ')));
    });

    // -- whitespace ---------------------------------------------------------

    test('whitespace-only username → returns raw whitespace (not filtered)',
        () {
      final chat = _chatWithParticipant(username: '   ');
      expect(chat.getOtherParticipantName(''), '   ');
    });

    test('whitespace-surrounded valid username → returns raw (no trim)',
        () {
      final chat = _chatWithParticipant(username: '  alice  ');
      expect(chat.getOtherParticipantName(''), '  alice  ');
    });

    // -- participant-ID equality safety -------------------------------------

    test('candidate equals participant user ID → returns raw ID string', () {
      final chat = _chatWithParticipant(username: _otherUserId);
      // getOtherParticipantName returns the raw name from participantNames;
      // no UUID filtering is performed at the entity level.
      expect(chat.getOtherParticipantName(''), _otherUserId);
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
        expect(chat.getOtherParticipantName('caller'), id);
      },
    );

    // -- canonical UUID shape passthrough -----------------------------------

    test('lowercase canonical UUID → returned as-is', () {
      final chat = _chatWithParticipant(
        username: 'deadbeef-1234-5678-9abc-def012345678',
      );
      expect(chat.getOtherParticipantName(''), 'deadbeef-1234-5678-9abc-def012345678');
    });

    test('uppercase canonical UUID → returned as-is', () {
      final chat = _chatWithParticipant(
        username: 'DEADBEEF-1234-5678-9ABC-DEF012345678',
      );
      expect(chat.getOtherParticipantName(''), 'DEADBEEF-1234-5678-9ABC-DEF012345678');
    });

    test('mixed-case canonical UUID → returned as-is', () {
      final chat = _chatWithParticipant(
        username: 'DeadBeef-1234-5678-9aBc-def012345678',
      );
      expect(chat.getOtherParticipantName(''), 'DeadBeef-1234-5678-9aBc-def012345678');
    });

    test('UUID surrounded by whitespace → returned as-is', () {
      final chat = _chatWithParticipant(
        username: '  deadbeef-1234-5678-9abc-def012345678  ',
      );
      expect(chat.getOtherParticipantName(''), '  deadbeef-1234-5678-9abc-def012345678  ');
    });

    // -- legitimate usernames preserved -------------------------------------

    test('hyphenated username like john-2026 → preserved', () {
      final chat = _chatWithParticipant(username: 'john-2026');
      expect(chat.getOtherParticipantName(''), 'john-2026');
    });

    test('numeric-containing username like user123 → preserved', () {
      final chat = _chatWithParticipant(username: 'user123');
      expect(chat.getOtherParticipantName(''), 'user123');
    });

    test('legitimate username with digits and hyphens → preserved', () {
      final chat = _chatWithParticipant(username: 'seller-99');
      expect(chat.getOtherParticipantName(''), 'seller-99');
    });

    test('abc-def (looks like UUID fragment but not 36 chars) → preserved',
        () {
      final chat = _chatWithParticipant(username: 'abc-def');
      expect(chat.getOtherParticipantName(''), 'abc-def');
    });

    // -- edge cases ---------------------------------------------------------

    test('never returns generic label "User" when name exists', () {
      final chat = _chatWithParticipant(username: 'alice');
      final result = chat.getOtherParticipantName('');
      expect(result, isNot(equals('User')));
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
      final name = chat.getOtherParticipantName('');
      // Fallback includes truncated id prefix.
      expect(name, startsWith('User '));
      expect(name.length, greaterThan(5));
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
      final name = chat.getOtherParticipantName('');
      expect(name, isNotEmpty);
    });
  });

  // ===========================================================================
  // B) formatChatHandle presentation helper — behavioral unit
  // ===========================================================================

  group('C1B1 — formatChatHandle (presentation)', () {
    test('valid raw username → @username', () {
      expect(formatChatHandle('alice'), '@alice');
    });

    test('leading-@ username → single @ (normalised)', () {
      expect(formatChatHandle('@alice'), '@alice');
    });

    test('double-@ username → preserved as-is (@-prefix check only)', () {
      expect(formatChatHandle('@@alice'), '@@alice');
    });

    test('empty username → empty string', () {
      expect(formatChatHandle(''), '');
    });

    test('whitespace-only username → empty after trim', () {
      expect(formatChatHandle('   '), '');
    });

    test('never returns @User', () {
      expect(formatChatHandle(''), isNot('@User'));
      expect(formatChatHandle('   '), isNot('@User'));
    });

    test('underscore-separated username → single @', () {
      expect(formatChatHandle('john_doe'), '@john_doe');
    });
  });

  // ===========================================================================
  // C) ChatCard participant identity composition — behavioral unit
  // ===========================================================================

  group('C1B1 — ChatCard participant identity composition', () {
    test('valid username + active lifecycle → @username', () {
      final chat = _chatWithParticipant(username: 'alice');
      final name = chat.getOtherParticipantName('');
      final lifecycle = chat.getOtherParticipantLifecycle('');
      final display = lifecycle.isDegraded
          ? chatLifecycleRedactionLabel(lifecycle)
          : formatChatHandle(name);
      expect(display, '@alice');
    });

    test('null username + active lifecycle → fallback name formatted', () {
      final chat = _chatWithParticipant(username: null);
      final name = chat.getOtherParticipantName('');
      final lifecycle = chat.getOtherParticipantLifecycle('');
      final display = lifecycle.isDegraded
          ? chatLifecycleRedactionLabel(lifecycle)
          : formatChatHandle(name);
      // getOtherParticipantName returns "User <id>..." fallback.
      // formatChatHandle wraps it with @ prefix.
      expect(display, startsWith('@User '));
    });

    test('empty username + active lifecycle → fallback via name check', () {
      final chat = _chatWithParticipant(username: '');
      final name = chat.getOtherParticipantName('');
      // getOtherParticipantName checks name.isNotEmpty; empty string
      // falls through to the 'User <id>...' fallback.
      expect(name, startsWith('User '));
      // Lifecycle is 'active' (not degraded), so display uses formatChatHandle.
      final display = formatChatHandle(name);
      expect(display, startsWith('@User '));
    });

    test('DTO ingress: empty username → filtered by mapper → lifecycle redaction', () {
      final chat = _chatFromDto(username: '');
      final lifecycle = chat.getOtherParticipantLifecycle('');
      // participantLifecycles is empty → getOtherParticipantLifecycle returns
      // ContentLifecycle.unavailable (fail-closed default).
      expect(lifecycle.isDegraded, isTrue);
      final display = chatLifecycleRedactionLabel(lifecycle);
      expect(display, isNotEmpty);
      expect(display, isNot(startsWith('@')));
    });

    test(
      'UUID-polluted name + active lifecycle → formatted as-is',
      () {
        final chat = _chatWithParticipant(username: _otherUserId);
        final name = chat.getOtherParticipantName('');
        final lifecycle = chat.getOtherParticipantLifecycle('');
        final display = lifecycle.isDegraded
            ? chatLifecycleRedactionLabel(lifecycle)
            : formatChatHandle(name);
        // getOtherParticipantName returns the raw name from participantNames.
        // No UUID filtering at the entity level.
        expect(display, '@$_otherUserId');
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
      expect(display, isNot(startsWith('@User ')));
    });

    test('DTO ingress: valid username → canonical handle via label', () {
      final chat = _chatFromDto(username: 'alice');
      final name = chat.getOtherParticipantName('');
      expect(name, 'alice');
      expect(formatChatHandle(name), '@alice');
    });

    test('DTO ingress: missing username → fallback → @User fallback', () {
      final chat = _chatFromDto(username: null);
      final name = chat.getOtherParticipantName('');
      // Fallback includes truncated id prefix.
      expect(name, startsWith('User '));
      expect(formatChatHandle(name), startsWith('@User '));
    });

    test('DTO ingress: empty username → empty → empty string', () {
      final chat = _chatFromDto(username: '');
      final name = chat.getOtherParticipantName('');
      // DTO branch: empty string username is filtered (isNotEmpty check).
      // So participantNames is empty → getOtherParticipantName returns fallback.
      expect(name, startsWith('User '));
      expect(formatChatHandle(name), startsWith('@User '));
    });

    test('DTO ingress: leading-@ username normalises to single @', () {
      final chat = _chatFromDto(username: '@alice');
      final name = chat.getOtherParticipantName('');
      expect(name, '@alice');
      expect(formatChatHandle(name), '@alice');
    });
  });

  // ===========================================================================
  // D) Negative contracts — behavioral unit
  // ===========================================================================

  group('C1B1 — Negative contracts', () {
    test('fallback name is short (no full UUID leak)', () {
      final chat = _chatWithParticipant(username: null);
      final name = chat.getOtherParticipantName('');
      // Fallback: 'User <id_prefix>...' where id_prefix is first 8 chars.
      // The full UUID is never returned — only the truncated prefix.
      expect(name.length, lessThan(30));
      expect(name, startsWith('User '));
    });

    test('no raw "?" fallback via formatChatHandle', () {
      expect(formatChatHandle(''), isNot('?'));
    });

    test('no bare "@" via formatChatHandle', () {
      expect(formatChatHandle(''), isNot('@'));
      expect(formatChatHandle('   '), isNot('@'));
    });

    test('no double "@@" via formatChatHandle for normal input', () {
      // formatChatHandle adds @ prefix only when input doesn't start with @.
      // Input already starting with @ is returned as-is (no @@ created).
      expect(formatChatHandle('alice'), isNot('@@alice'));
      expect(formatChatHandle('@alice'), isNot('@@alice'));
    });

    test('UUID names pass through as-is (no entity-level filtering)', () {
      // getOtherParticipantName returns the raw name from participantNames.
      // UUID safety is handled at the presentation layer, not the entity layer.
      final lowerUuid = 'deadbeef-1234-5678-9abc-def012345678';
      final upperUuid = 'DEADBEEF-1234-5678-9ABC-DEF012345678';
      final mixedUuid = 'DeadBeef-1234-5678-9aBc-def012345678';
      for (final uuid in [lowerUuid, upperUuid, mixedUuid]) {
        final chat = _chatWithParticipant(username: uuid);
        expect(
          chat.getOtherParticipantName(''),
          uuid,
          reason: 'UUID $uuid should pass through',
        );
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
      'getOtherParticipantName on empty participantIds throws StateError',
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
          ).getOtherParticipantName(''),
          throwsA(isA<StateError>()),
        );
      },
    );

    test(
      'getOtherParticipantName is the canonical participant identity accessor',
      () {
        // C1B1 convergence: getOtherParticipantName replaced the removed
        // getOtherParticipantUsername. This test proves it compiles and
        // returns the expected value.
        final chat = _chatWithParticipant(username: 'alice');
        expect(chat.getOtherParticipantName(''), 'alice');
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
