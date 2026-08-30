// E6 — Mobile content-detail author lifecycle ingestion + redaction tests.
//
// Scope is pinned to two seams:
//   1) the wire shape parser that walks `card.author.lifecycle` (with the
//      top-level `author.lifecycle` fallback) into ContentDto.authorLifecycle,
//   2) the ContentLifecycle parser that maps wire strings into the canonical
//      enum used by the redaction gate in content_detail_screen.
//
// Widget-level golden tests for the rendered author redaction would require
// a full Riverpod harness and existing test infrastructure that is not
// present in this slice; pure-data testing keeps the contract pinned
// without dragging in a new harness — same posture as E2.1 / E3.1 / E4.3 /
// E5.2.

import 'package:flutter_test/flutter_test.dart';

import 'package:labuda/domains/social/content/data/dto/content_dto.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';

Map<String, dynamic> _baseContentJson({
  Map<String, dynamic>? card,
  Map<String, dynamic>? author,
}) {
  return <String, dynamic>{
    'id': '00000000-0000-0000-0000-000000000001',
    'caption': 'hello world',
    'author_id': '00000000-0000-0000-0000-000000000002',
    'author_username': 'alice',
    'author_avatar': null,
    'author_city': null,
    'author_province': null,
    'status': 'active',
    'lifecycle': 'active',
    'visibility': 'public',
    'media_urls': <String>[],
    'media': <Map<String, dynamic>>[],
    'tags': <String>[],
    'tagged_users': <String>[],
    'location': null,
    'engagement': <String, dynamic>{
      'likeCount': 0,
      'commentCount': 0,
    },
    'moderation_info': null,
    'budget_min': null,
    'budget_max': null,
    'deadline': null,
    'published_at': null,
    'scheduled_at': null,
    'created_at': '2026-01-01T00:00:00.000Z',
    'updated_at': '2026-01-01T00:00:00.000Z',
    'is_liked': null,
    'is_saved': null,
    'original_author_id': null,
    'share_reference': null,
    if (card != null) 'card': card,
    if (author != null) 'author': author,
  };
}

void main() {
  group('E6 — ContentDto authorLifecycle wire extraction', () {
    test('absent card → authorLifecycle null', () {
      final dto = ContentDto.fromJson(_baseContentJson());
      expect(dto.authorLifecycle, isNull);
    });

    test('card.author.lifecycle="active" → authorLifecycle "active"', () {
      final dto = ContentDto.fromJson(
        _baseContentJson(
          card: {
            'id': '00000000-0000-0000-0000-000000000001',
            'type': 'post',
            'author': {
              'id': '00000000-0000-0000-0000-000000000002',
              'username': 'alice',
              'lifecycle': 'active',
            },
          },
        ),
      );
      expect(dto.authorLifecycle, 'active');
    });

    test(
      'card.author.lifecycle="unavailable" → authorLifecycle "unavailable"',
      () {
        final dto = ContentDto.fromJson(
          _baseContentJson(
            card: {
              'id': '00000000-0000-0000-0000-000000000001',
              'type': 'post',
              'author': {
                'id': '00000000-0000-0000-0000-000000000002',
                'username': 'bob',
                'lifecycle': 'unavailable',
              },
            },
          ),
        );
        expect(dto.authorLifecycle, 'unavailable');
      },
    );

    test('card.author.lifecycle="removed" → authorLifecycle "removed"', () {
      final dto = ContentDto.fromJson(
        _baseContentJson(
          card: {
            'id': '00000000-0000-0000-0000-000000000001',
            'type': 'post',
            'author': {
              'id': '00000000-0000-0000-0000-000000000002',
              'username': 'ghost',
              'lifecycle': 'removed',
            },
          },
        ),
      );
      expect(dto.authorLifecycle, 'removed');
    });

    test('top-level author.lifecycle is honoured as fallback', () {
      final dto = ContentDto.fromJson(
        _baseContentJson(
          author: {
            'id': '00000000-0000-0000-0000-000000000002',
            'username': 'alice',
            'lifecycle': 'unavailable',
          },
        ),
      );
      expect(dto.authorLifecycle, 'unavailable');
    });

    test('card.author.lifecycle wins over top-level author.lifecycle', () {
      final dto = ContentDto.fromJson(
        _baseContentJson(
          card: {
            'id': '00000000-0000-0000-0000-000000000001',
            'type': 'post',
            'author': {
              'id': '00000000-0000-0000-0000-000000000002',
              'username': 'alice',
              'lifecycle': 'active',
            },
          },
          author: {
            'id': '00000000-0000-0000-0000-000000000002',
            'username': 'alice',
            'lifecycle': 'unavailable',
          },
        ),
      );
      expect(dto.authorLifecycle, 'active');
    });

    test('empty-string lifecycle → null (rollback-safe)', () {
      final dto = ContentDto.fromJson(
        _baseContentJson(
          card: {
            'id': '00000000-0000-0000-0000-000000000001',
            'type': 'post',
            'author': {
              'id': '00000000-0000-0000-0000-000000000002',
              'username': 'alice',
              'lifecycle': '',
            },
          },
        ),
      );
      expect(dto.authorLifecycle, isNull);
    });
  });

  group(
    'E6 — ContentLifecycleParse converts authorLifecycle wire correctly',
    () {
      test('null wire → unavailable (FAIL CLOSED)', () {
        expect(
          ContentLifecycleParse.fromWire(null),
          ContentLifecycle.unavailable,
        );
      });

      test('"active" → active', () {
        expect(
          ContentLifecycleParse.fromWire('active'),
          ContentLifecycle.active,
        );
      });

      test('"unavailable" → unavailable', () {
        expect(
          ContentLifecycleParse.fromWire('unavailable'),
          ContentLifecycle.unavailable,
        );
      });

      test('"removed" → removed', () {
        expect(
          ContentLifecycleParse.fromWire('removed'),
          ContentLifecycle.removed,
        );
      });

      test('unknown wire → unavailable (FAIL CLOSED)', () {
        expect(
          ContentLifecycleParse.fromWire('shadowbanned'),
          ContentLifecycle.unavailable,
        );
      });
    },
  );
}
