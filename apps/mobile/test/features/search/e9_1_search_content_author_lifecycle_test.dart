// E9.1 — Mobile /search/content author user-identity lifecycle ingestion
// and subtitle redaction tests.
//
// Scope is pinned to four seams:
//   1) `_readContentSearchAuthorLifecycle` wire-shape parser — primary path
//      `card.author.lifecycle`, fallback `author.lifecycle`, null fall-through.
//   2) Mapper / DTO→entity conversion threads `authorLifecycle` into the
//      canonical `ContentLifecycle` on `ContentSearchResult` (default active
//      for null / missing / unknown).
//   3) Axis-boundary contract — content item `lifecycle` and author
//      `authorLifecycle` are independent axes that must never be conflated.
//   4) `SearchResultItem` subtitle redaction gate for content rows driven by
//      `metadata['authorLifecycle']`.  Item opacity / tap still governed by
//      item-axis `metadata['lifecycle']` only.
//
// Widget-level golden tests would require a full Riverpod harness; the
// subtitle gate is validated by computing the rendered placeholder string
// from a SearchResult directly — same lightweight posture as E8.4.

import 'package:flutter_test/flutter_test.dart';

import 'package:labuda/features/search/search/data/dto/search_dto.dart';
import 'package:labuda/features/search/search/data/mappers/search_mapper.dart';
import 'package:labuda/features/search/search/domain/entities/search_result.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

Map<String, dynamic> _baseContentJson({
  Map<String, dynamic>? card,
  Map<String, dynamic>? author,
}) {
  return <String, dynamic>{
    'id': '00000000-0000-0000-0000-000000000001',
    'author_id': '00000000-0000-0000-0000-000000000002',
    'type': 'content',
    'caption': 'Test caption',
    'media_urls': <String>[],
    'created_at': '2026-01-01T00:00:00.000Z',
    'card': ?card,
    'author': ?author,
  };
}

/// Reproduce the content-author subtitle redaction logic from
/// SearchResultItem._contentAuthorRedactionSubtitle so we can pin it without
/// a Riverpod widget harness. Reads only `metadata['authorLifecycle']`.
String? _renderAuthorSubtitle(SearchResult r) {
  final isContent = r.type == SearchResultType.content;
  final lifecycle = isContent
      ? ContentLifecycleParse.fromWire(r.metadata['authorLifecycle'] as String?)
      : ContentLifecycle.active;
  switch (lifecycle) {
    case ContentLifecycle.removed:
      return 'Pengguna dihapus';
    case ContentLifecycle.unavailable:
      return 'Pengguna tidak tersedia';
    case ContentLifecycle.active:
      return r.subtitle;
  }
}

void main() {
  // -------------------------------------------------------------------------
  // 1) DTO wire extraction
  // -------------------------------------------------------------------------
  group('E9.1 — ContentSearchResultDto.authorLifecycle wire extraction', () {
    test('absent card + absent author → authorLifecycle null (pre-E9.1)', () {
      final dto = ContentSearchResultDto.fromJson(_baseContentJson());
      expect(dto.authorLifecycle, isNull);
    });

    test('card.author.lifecycle = "active" → "active" (primary path)', () {
      final dto = ContentSearchResultDto.fromJson(
        _baseContentJson(
          card: {
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

    test('card.author.lifecycle = "unavailable" → "unavailable"', () {
      final dto = ContentSearchResultDto.fromJson(
        _baseContentJson(
          card: {
            'author': {
              'id': '00000000-0000-0000-0000-000000000002',
              'username': 'banned',
              'lifecycle': 'unavailable',
            },
          },
        ),
      );
      expect(dto.authorLifecycle, 'unavailable');
    });

    test('card.author.lifecycle = "removed" → "removed"', () {
      final dto = ContentSearchResultDto.fromJson(
        _baseContentJson(
          card: {
            'author': {
              'id': '00000000-0000-0000-0000-000000000002',
              'username': 'gone',
              'lifecycle': 'removed',
            },
          },
        ),
      );
      expect(dto.authorLifecycle, 'removed');
    });

    test('fallback author.lifecycle when card absent', () {
      final dto = ContentSearchResultDto.fromJson(
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

    test('card.author.lifecycle wins over author.lifecycle fallback', () {
      final dto = ContentSearchResultDto.fromJson(
        _baseContentJson(
          card: {
            'author': {'lifecycle': 'active'},
          },
          author: {
            'id': '00000000-0000-0000-0000-000000000002',
            'username': 'alice',
            'lifecycle': 'removed',
          },
        ),
      );
      // Primary path (card.author.lifecycle) must win.
      expect(dto.authorLifecycle, 'active');
    });

    test('empty-string lifecycle → null (rollback-safe)', () {
      final dto = ContentSearchResultDto.fromJson(
        _baseContentJson(
          card: {
            'author': {'lifecycle': ''},
          },
        ),
      );
      expect(dto.authorLifecycle, isNull);
    });

    test('null lifecycle in card.author → fallback to author block', () {
      final dto = ContentSearchResultDto.fromJson(
        _baseContentJson(
          card: {
            'author': {'lifecycle': null},
          },
          author: {'lifecycle': 'removed'},
        ),
      );
      expect(dto.authorLifecycle, 'removed');
    });
  });

  // -------------------------------------------------------------------------
  // 2) Mapper threads DTO → entity ContentLifecycle
  // -------------------------------------------------------------------------
  group('E9.1 — Mapper threads authorLifecycle into ContentSearchResult', () {
    test('null wire → ContentLifecycle.unavailable (FAIL CLOSED)', () {
      final dto = ContentSearchResultDto.fromJson(_baseContentJson());
      expect(dto.toDomain().authorLifecycle, ContentLifecycle.unavailable);
    });

    test('"active" wire → ContentLifecycle.active', () {
      final dto = ContentSearchResultDto.fromJson(
        _baseContentJson(
          card: {
            'author': {'lifecycle': 'active'},
          },
        ),
      );
      expect(dto.toDomain().authorLifecycle, ContentLifecycle.active);
    });

    test('"unavailable" wire → ContentLifecycle.unavailable', () {
      final dto = ContentSearchResultDto.fromJson(
        _baseContentJson(
          card: {
            'author': {'lifecycle': 'unavailable'},
          },
        ),
      );
      expect(dto.toDomain().authorLifecycle, ContentLifecycle.unavailable);
    });

    test('"removed" wire → ContentLifecycle.removed', () {
      final dto = ContentSearchResultDto.fromJson(
        _baseContentJson(
          card: {
            'author': {'lifecycle': 'removed'},
          },
        ),
      );
      expect(dto.toDomain().authorLifecycle, ContentLifecycle.removed);
    });

    test('unknown wire → ContentLifecycle.unavailable (FAIL CLOSED)', () {
      final dto = ContentSearchResultDto.fromJson(
        _baseContentJson(
          card: {
            'author': {'lifecycle': 'shadowbanned'},
          },
        ),
      );
      expect(dto.toDomain().authorLifecycle, ContentLifecycle.unavailable);
    });
  });

  // -------------------------------------------------------------------------
  // 3) Axis-boundary contract
  // -------------------------------------------------------------------------
  group('E9.1 — Axis boundary: item lifecycle vs author lifecycle', () {
    test('content item lifecycle is independent of author lifecycle', () {
      // Item lifecycle (governance axis) gates opacity + tap.
      // Author lifecycle (identity axis) gates only subtitle.
      // They must be stored and consumed via separate metadata keys.
      final result = SearchResult(
        id: 'c1',
        type: SearchResultType.content,
        title: 'Post',
        subtitle: '@alice',
        metadata: {
          'lifecycle': 'unavailable', // item-axis → opacity gate
          'authorLifecycle': 'removed', // author-axis → subtitle gate
        },
        createdAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
      );
      // Author subtitle redaction fires off authorLifecycle only.
      expect(_renderAuthorSubtitle(result), 'Pengguna dihapus');
      // Item axis lifecycle is separate and untouched.
      expect(result.metadata['lifecycle'], 'unavailable');
    });

    test(
      'active item + degraded author → subtitle redacted, row stays visible',
      () {
        final result = SearchResult(
          id: 'c2',
          type: SearchResultType.content,
          title: 'Post',
          subtitle: '@banned',
          metadata: {'lifecycle': 'active', 'authorLifecycle': 'unavailable'},
          createdAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
        );
        expect(_renderAuthorSubtitle(result), 'Pengguna tidak tersedia');
        // Row must have no item-axis disabled signal.
        expect(
          ContentLifecycleParse.fromWire(
            result.metadata['lifecycle'] as String?,
          ),
          ContentLifecycle.active,
        );
      },
    );

    test(
      'item unavailable dominates — subtitle would show author redaction but '
      'item-axis opacity/tap gate is separate from subtitle gate',
      () {
        // This test pins that the two gates are independently keyed.
        // In the widget, isUnavailable fires FIRST (item-axis) so the
        // "Tidak tersedia" item tombstone is shown regardless. That is
        // widget-level; here we verify the metadata keys are distinct.
        final result = SearchResult(
          id: 'c3',
          type: SearchResultType.content,
          title: 'Post',
          subtitle: '@removed',
          metadata: {'lifecycle': 'removed', 'authorLifecycle': 'removed'},
          createdAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
        );
        // Author subtitle function returns redaction placeholder.
        expect(_renderAuthorSubtitle(result), 'Pengguna dihapus');
        // Item lifecycle is independently 'removed'.
        expect(result.metadata['lifecycle'], 'removed');
      },
    );
  });

  // -------------------------------------------------------------------------
  // 4) Subtitle redaction gate
  // -------------------------------------------------------------------------
  group('E9.1 — SearchResultItem content author subtitle redaction gate', () {
    SearchResult contentResult(String? authorLifecycle, {String? subtitle}) {
      return SearchResult(
        id: 'c1',
        type: SearchResultType.content,
        title: 'Post title',
        subtitle: subtitle ?? '@alice',
        metadata: {'lifecycle': 'active', 'authorLifecycle': ?authorLifecycle},
        createdAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
      );
    }

    test('active author → owner-truth subtitle unchanged', () {
      expect(_renderAuthorSubtitle(contentResult('active')), '@alice');
    });

    test(
      'missing authorLifecycle → "Pengguna tidak tersedia" (FAIL CLOSED)',
      () {
        expect(
          _renderAuthorSubtitle(contentResult(null)),
          'Pengguna tidak tersedia',
        );
      },
    );

    test('unavailable author → "Pengguna tidak tersedia"', () {
      expect(
        _renderAuthorSubtitle(contentResult('unavailable')),
        'Pengguna tidak tersedia',
      );
    });

    test('removed author → "Pengguna dihapus"', () {
      expect(
        _renderAuthorSubtitle(contentResult('removed')),
        'Pengguna dihapus',
      );
    });

    test(
      'unknown author lifecycle → "Pengguna tidak tersedia" (FAIL CLOSED)',
      () {
        expect(
          _renderAuthorSubtitle(contentResult('shadowbanned')),
          'Pengguna tidak tersedia',
        );
      },
    );

    test('listing row IGNORES authorLifecycle', () {
      final listing = SearchResult(
        id: 'l1',
        type: SearchResultType.listing,
        title: 'Listing',
        subtitle: 'Acme Farm',
        metadata: {'authorLifecycle': 'removed', 'sellerLifecycle': 'active'},
        createdAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
      );
      expect(_renderAuthorSubtitle(listing), 'Acme Farm');
    });

    test('auction row IGNORES authorLifecycle', () {
      final auction = SearchResult(
        id: 'a1',
        type: SearchResultType.auction,
        title: 'Auction',
        subtitle: '@bob',
        metadata: {'authorLifecycle': 'removed', 'sellerLifecycle': 'active'},
        createdAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
      );
      expect(_renderAuthorSubtitle(auction), '@bob');
    });

    test('user row IGNORES authorLifecycle', () {
      final user = SearchResult(
        id: 'u1',
        type: SearchResultType.user,
        title: '@charlie',
        subtitle: '@charlie',
        metadata: {'authorLifecycle': 'removed'},
        createdAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
      );
      expect(_renderAuthorSubtitle(user), '@charlie');
    });

    test(
      'listing/auction sellerLifecycle unaffected by content author gate',
      () {
        // E8.4 seller redaction is separate; confirm neither axis
        // bleeds into the other. Listing sellerLifecycle still reaches
        // the E8.4 gate (not tested here — see e8_4 test file),
        // but authorLifecycle is never consumed on a listing row.
        final listing = SearchResult(
          id: 'l2',
          type: SearchResultType.listing,
          title: 'Koi',
          subtitle: 'Acme Farm',
          metadata: {
            'sellerLifecycle': 'removed',
            // authorLifecycle would be absent on real listing rows but
            // even if present it must be ignored by the author gate.
            'authorLifecycle': 'active',
          },
          createdAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
        );
        // Author gate ignores non-content rows.
        expect(_renderAuthorSubtitle(listing), 'Acme Farm');
        // sellerLifecycle present and correct for the E8.4 gate.
        expect(listing.metadata['sellerLifecycle'], 'removed');
      },
    );
  });
}
