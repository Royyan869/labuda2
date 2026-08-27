// E11.1 — Chat room list participant display fix tests.
//
// Validates that ChatDto.fromJson branch 1 (triggered by `other_user_id`)
// correctly extracts other_user.username and other_user.avatar_url into
// participantNames and participantAvatars.
//
// Pre-E11.1 those two maps were hardcoded to {}, causing ChatCard to fall back
// to "User <uuid>..." for every private/negotiation room row. The backend has
// emitted other_user.username and other_user.avatar_url since E4.2; this fix
// consumes those fields at the DTO seam.
//
// Scope: DTO parse seam (branch 1) + mapper/entity transparency.
// E10 remains the authority for participantLifecycles.
// ChatCard / ChatMapper / Chat entity: no changes — transparent.

import 'package:flutter_test/flutter_test.dart';

import 'package:labuda/domains/chat/chat/data/dto/chat_dto.dart';
import 'package:labuda/domains/chat/chat/data/mappers/chat_mapper.dart';

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

const _otherUserId = '00000000-0000-0000-0000-000000000002';

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

// ---------------------------------------------------------------------------

void main() {
  // -------------------------------------------------------------------------
  // 1) participantNames population
  // -------------------------------------------------------------------------
  group('E11.1 — ChatDto branch 1: participantNames', () {
    test(
      'other_user with username → participantNames[otherUserId] = username',
      () {
        final dto = ChatDto.fromJson(
          _roomJson(
            otherUser: {
              'id': _otherUserId,
              'username': 'alice',
              'avatar_url': null,
              'lifecycle': 'active',
            },
          ),
        );
        expect(dto.participantNames[_otherUserId], 'alice');
      },
    );

    test('absent other_user block → participantNames empty', () {
      final dto = ChatDto.fromJson(_roomJson());
      expect(dto.participantNames, isEmpty);
    });

    test(
      'other_user present but username empty string → participantNames empty',
      () {
        final dto = ChatDto.fromJson(_roomJson(otherUser: {'username': ''}));
        expect(dto.participantNames, isEmpty);
      },
    );

    test('other_user present but username null → participantNames empty', () {
      final dto = ChatDto.fromJson(_roomJson(otherUser: {'username': null}));
      expect(dto.participantNames, isEmpty);
    });

    test(
      'other_user present but username key absent → participantNames empty',
      () {
        final dto = ChatDto.fromJson(
          _roomJson(otherUser: {'lifecycle': 'active'}),
        );
        expect(dto.participantNames, isEmpty);
      },
    );

    test('username is non-string type → participantNames empty', () {
      final dto = ChatDto.fromJson(_roomJson(otherUser: {'username': 42}));
      expect(dto.participantNames, isEmpty);
    });
  });

  // -------------------------------------------------------------------------
  // 2) participantAvatars population
  // -------------------------------------------------------------------------
  group('E11.1 — ChatDto branch 1: participantAvatars', () {
    test(
      'other_user with avatar_url → participantAvatars[otherUserId] = url',
      () {
        final dto = ChatDto.fromJson(
          _roomJson(
            otherUser: {
              'username': 'alice',
              'avatar_url': 'https://cdn.example.com/alice.jpg',
            },
          ),
        );
        expect(
          dto.participantAvatars[_otherUserId],
          'https://cdn.example.com/alice.jpg',
        );
      },
    );

    test('other_user with null avatar_url → no avatar entry in map', () {
      final dto = ChatDto.fromJson(
        _roomJson(otherUser: {'username': 'alice', 'avatar_url': null}),
      );
      expect(dto.participantAvatars, isEmpty);
    });

    test('other_user with missing avatar_url key → no avatar entry', () {
      final dto = ChatDto.fromJson(_roomJson(otherUser: {'username': 'alice'}));
      expect(dto.participantAvatars, isEmpty);
    });

    test('other_user with empty string avatar_url → no avatar entry', () {
      final dto = ChatDto.fromJson(
        _roomJson(otherUser: {'username': 'alice', 'avatar_url': ''}),
      );
      expect(dto.participantAvatars, isEmpty);
    });

    test('absent other_user block → participantAvatars empty', () {
      final dto = ChatDto.fromJson(_roomJson());
      expect(dto.participantAvatars, isEmpty);
    });
  });

  // -------------------------------------------------------------------------
  // 3) Co-extraction: all three fields from the same other_user block
  // -------------------------------------------------------------------------
  group('E11.1 — other_user block: name + avatar + lifecycle co-extracted', () {
    test('full card → name + avatar + lifecycle all populated', () {
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
      expect(dto.participantNames[_otherUserId], 'alice');
      expect(
        dto.participantAvatars[_otherUserId],
        'https://cdn.example.com/alice.jpg',
      );
      expect(dto.participantLifecycles[_otherUserId], 'active');
    });

    test('unavailable lifecycle still co-extracted alongside name', () {
      final dto = ChatDto.fromJson(
        _roomJson(
          otherUser: {
            'username': 'alice',
            'avatar_url': null,
            'lifecycle': 'unavailable',
          },
        ),
      );
      expect(dto.participantNames[_otherUserId], 'alice');
      expect(dto.participantLifecycles[_otherUserId], 'unavailable');
    });

    test(
      'missing lifecycle + present name → name populated, lifecycle absent',
      () {
        final dto = ChatDto.fromJson(
          _roomJson(otherUser: {'username': 'alice'}),
        );
        expect(dto.participantNames[_otherUserId], 'alice');
        expect(dto.participantLifecycles, isEmpty);
      },
    );
  });

  // -------------------------------------------------------------------------
  // 4) Chat entity: getOtherParticipantName resolves correctly
  // -------------------------------------------------------------------------
  group('E11.1 — Chat entity name resolution', () {
    test(
      'populated participantNames → returns username (no uuid fallback)',
      () {
        final dto = ChatDto.fromJson(
          _roomJson(otherUser: {'username': 'alice', 'lifecycle': 'active'}),
        );
        final chat = ChatMapper.toDomain(dto);
        // currentUserId = '' matches ChatCard._getCurrentUserId behaviour for
        // branch-1 rooms (participantIds has 1 element, length != 2 → '').
        expect(chat.getOtherParticipantName(''), 'alice');
      },
    );

    test('empty participantNames → fallback "User <uuid>..." still works', () {
      // Regression anchor: the fallback must continue to function for
      // rooms where other_user is absent (pre-E4.2 / rollback).
      final dto = ChatDto.fromJson(_roomJson());
      final chat = ChatMapper.toDomain(dto);
      expect(
        chat.getOtherParticipantName(''),
        'User ${_otherUserId.substring(0, 8)}...',
      );
    });

    test('avatar accessible via participantAvatars on the entity', () {
      final dto = ChatDto.fromJson(
        _roomJson(
          otherUser: {
            'username': 'alice',
            'avatar_url': 'https://cdn.example.com/alice.jpg',
          },
        ),
      );
      final chat = ChatMapper.toDomain(dto);
      expect(
        chat.participantAvatars[_otherUserId],
        'https://cdn.example.com/alice.jpg',
      );
    });

    test('null avatar_url → participantAvatars lookup returns null', () {
      final dto = ChatDto.fromJson(
        _roomJson(otherUser: {'username': 'alice', 'avatar_url': null}),
      );
      final chat = ChatMapper.toDomain(dto);
      // Map is empty; lookup returns null — same runtime result as null value.
      expect(chat.participantAvatars[_otherUserId], isNull);
    });
  });

  // -------------------------------------------------------------------------
  // 5) Negotiation room type parity
  // -------------------------------------------------------------------------
  group('E11.1 — Negotiation room type parity with direct', () {
    test('negotiation room with other_user → participantNames populated', () {
      final dto = ChatDto.fromJson(
        _roomJson(
          roomType: 'negotiation',
          otherUser: {'username': 'bob', 'lifecycle': 'active'},
        ),
      );
      expect(dto.type, 'private');
      expect(dto.participantNames[_otherUserId], 'bob');
    });
  });

  // -------------------------------------------------------------------------
  // 6) Support room: DTO parse identical to direct (widget bypass independent)
  // -------------------------------------------------------------------------
  group('E11.1 — Support room DTO branch', () {
    test('support room with other_user → participantNames populated in DTO', () {
      // ChatCard._buildSupportChatCard ignores participantNames/Avatars and
      // renders the hardcoded "Support" label regardless — this test only
      // confirms the DTO parse is consistent; widget behaviour is out of scope.
      final dto = ChatDto.fromJson(
        _roomJson(roomType: 'support', otherUser: {'username': 'admin_user'}),
      );
      expect(dto.type, 'support');
      expect(dto.participantNames[_otherUserId], 'admin_user');
    });
  });

  // -------------------------------------------------------------------------
  // 7) No crash on malformed inputs
  // -------------------------------------------------------------------------
  group('E11.1 — Null-tolerance / no crash', () {
    test('other_user is not a Map (string) → no throw, maps empty', () {
      final dto = ChatDto.fromJson(<String, dynamic>{
        'id': '00000000-0000-0000-0000-000000000010',
        'other_user_id': _otherUserId,
        'room_type': 'direct',
        'created_at': '2026-01-01T00:00:00.000Z',
        'other_user': 'unexpected-string',
      });
      expect(dto.participantNames, isEmpty);
      expect(dto.participantAvatars, isEmpty);
    });

    test('other_user_id is null → participantIds empty, maps empty', () {
      final dto = ChatDto.fromJson(<String, dynamic>{
        'id': '00000000-0000-0000-0000-000000000010',
        'other_user_id': null,
        'room_type': 'direct',
        'created_at': '2026-01-01T00:00:00.000Z',
        'other_user': {'username': 'alice'},
      });
      expect(dto.participantIds, isEmpty);
      expect(dto.participantNames, isEmpty);
      expect(dto.participantAvatars, isEmpty);
    });
  });
}
