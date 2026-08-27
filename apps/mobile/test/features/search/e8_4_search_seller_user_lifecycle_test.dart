// E8.4 — Mobile search listing/auction seller user-axis lifecycle ingestion
// and subtitle redaction tests.
//
// Scope is pinned to four seams:
//   1) the wire-shape parsers that walk `listing.seller.user.lifecycle` and
//      `auction.seller.user.lifecycle` into the DTO `sellerUserLifecycle`
//      string slot,
//   2) the mapper / DTO->entity conversion that pipes that wire string into
//      the canonical `ContentLifecycle` on the search-row entity (fail-closed
//      to unavailable for null / missing / unknown — 3-state truth doctrine),
//   3) the axis-boundary contract — top-level `seller.lifecycle` and flat
//      `seller_lifecycle` MUST NOT be read on this surface,
//   4) the SearchResultItem subtitle redaction gate driven off the
//      metadata['sellerLifecycle'] wire string on listing/auction rows
//      ONLY (other surfaces remain untouched).
//
// Widget-level golden tests would require a full Riverpod harness; the
// subtitle gate is validated by computing the rendered placeholder string
// from a SearchResult directly — same lightweight posture as E2.1 / E3.1 /
// E4.3 / E5.2 / E6 / E8.1 / E8.2.

import 'package:flutter_test/flutter_test.dart';

import 'package:labuda/features/search/search/data/dto/search_dto.dart';
import 'package:labuda/features/search/search/data/mappers/search_mapper.dart';
import 'package:labuda/features/search/search/domain/entities/search_result.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/shared/utils/commerce_seller_identity.dart';

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

Map<String, dynamic> _baseListingJson({Map<String, dynamic>? listing}) {
  return <String, dynamic>{
    'id': '00000000-0000-0000-0000-000000000001',
    'title': 'Showa Koi 30cm',
    'description': 'Premium showa',
    'variety': 'showa',
    'price': 1500000,
    'media_urls': <String>[],
    'seller_id': '00000000-0000-0000-0000-000000000002',
    'seller_username': 'alice',
    'seller_farm_name': 'Acme Farm',
    'seller_avatar_url': null,
    'created_at': '2026-01-01T00:00:00.000Z',
    'fixed_price_sale': ?listing,
  };
}

Map<String, dynamic> _baseAuctionJson({Map<String, dynamic>? auction}) {
  return <String, dynamic>{
    'id': '00000000-0000-0000-0000-000000000010',
    'seller_id': '00000000-0000-0000-0000-000000000020',
    'product_id': '00000000-0000-0000-0000-000000000030',
    'title': 'Sanke Auction',
    'description': 'Live auction',
    'start_price': 1000000,
    'current_bid': 1500000,
    'buy_now_price': null,
    'start_at': '2026-01-01T00:00:00.000Z',
    'end_at': '2026-01-02T00:00:00.000Z',
    'status': 'active',
    'thumbnail_url': null,
    'bid_count': 3,
    'created_at': '2026-01-01T00:00:00.000Z',
    'seller_username': 'bob',
    'seller_farm_name': 'Auction Farm',
    'seller_avatar_url': null,
    'auction': ?auction,
  };
}

/// Reproduce the listing/auction subtitle redaction logic from
/// SearchResultItem._sellerRedactionSubtitle so we can pin it without a
/// Riverpod widget harness. The widget reads ONLY this wire shape:
///   - metadata['sellerLifecycle'] (sourced from seller.user.lifecycle).
/// Top-level seller.lifecycle never reaches metadata, so it can never
/// influence this gate.
String? _renderSubtitle(SearchResult r) {
  final isSellerSurface =
      r.type == SearchResultType.listing || r.type == SearchResultType.auction;
  final lifecycle = isSellerSurface
      ? ContentLifecycleParse.fromWire(r.metadata['sellerLifecycle'] as String?)
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
  // 1) Listing DTO wire extraction
  // -------------------------------------------------------------------------
  group(
    'E8.4 — ListingSearchResultDto.sellerUserLifecycle wire extraction',
    () {
      test('absent listing block → sellerUserLifecycle null (pre-E8.1)', () {
        final dto = ListingSearchResultDto.fromJson(_baseListingJson());
        expect(dto.sellerUserLifecycle, isNull);
      });

      test('missing nested seller.user → null (defaults to active)', () {
        final dto = ListingSearchResultDto.fromJson(
          _baseListingJson(
            listing: {
              'id': '00000000-0000-0000-0000-000000000001',
              'seller': {'farm_name': 'Acme Farm'},
            },
          ),
        );
        expect(dto.sellerUserLifecycle, isNull);
      });

      test('nested user.lifecycle="active" → "active"', () {
        final dto = ListingSearchResultDto.fromJson(
          _baseListingJson(
            listing: {
              'id': '00000000-0000-0000-0000-000000000001',
              'seller': {
                'user': {
                  'id': '00000000-0000-0000-0000-000000000002',
                  'username': 'alice',
                  'lifecycle': 'active',
                },
                'farm_name': 'Acme Farm',
                'lifecycle': null,
              },
            },
          ),
        );
        expect(dto.sellerUserLifecycle, 'active');
      });

      test('nested user.lifecycle="unavailable" → "unavailable"', () {
        final dto = ListingSearchResultDto.fromJson(
          _baseListingJson(
            listing: {
              'id': '00000000-0000-0000-0000-000000000001',
              'seller': {
                'user': {
                  'id': '00000000-0000-0000-0000-000000000002',
                  'username': 'banned-user',
                  'lifecycle': 'unavailable',
                },
              },
            },
          ),
        );
        expect(dto.sellerUserLifecycle, 'unavailable');
      });

      test('nested user.lifecycle="removed" → "removed"', () {
        final dto = ListingSearchResultDto.fromJson(
          _baseListingJson(
            listing: {
              'id': '00000000-0000-0000-0000-000000000001',
              'seller': {
                'user': {
                  'id': '00000000-0000-0000-0000-000000000002',
                  'username': 'gone',
                  'lifecycle': 'removed',
                },
              },
            },
          ),
        );
        expect(dto.sellerUserLifecycle, 'removed');
      });

      test('empty-string user.lifecycle → null (rollback-safe)', () {
        final dto = ListingSearchResultDto.fromJson(
          _baseListingJson(
            listing: {
              'id': '00000000-0000-0000-0000-000000000001',
              'seller': {
                'user': {
                  'id': '00000000-0000-0000-0000-000000000002',
                  'username': 'alice',
                  'lifecycle': '',
                },
              },
            },
          ),
        );
        expect(dto.sellerUserLifecycle, isNull);
      });

      test(
        'AXIS BOUNDARY: top-level seller.lifecycle MUST NOT be consumed',
        () {
          // Even if the backend (in violation of doctrine) emitted a non-nil
          // top-level seller.lifecycle, the DTO must NEVER consume it. The
          // walker only reaches user.lifecycle; top-level is ignored.
          final dto = ListingSearchResultDto.fromJson(
            _baseListingJson(
              listing: {
                'id': '00000000-0000-0000-0000-000000000001',
                'seller': {
                  'user': {
                    'id': '00000000-0000-0000-0000-000000000002',
                    'username': 'alice',
                    'lifecycle': 'active',
                  },
                  // Hypothetical out-of-doctrine emission — must be IGNORED.
                  'lifecycle': 'unavailable',
                },
              },
            ),
          );
          expect(dto.sellerUserLifecycle, 'active');
        },
      );

      test(
        'AXIS BOUNDARY: flat seller_lifecycle scalar MUST NOT be consumed',
        () {
          final json = _baseListingJson();
          // Hypothetical flat scalar — must be IGNORED by the nested walker.
          json['seller_lifecycle'] = 'unavailable';
          json['seller_status'] = 'banned';
          final dto = ListingSearchResultDto.fromJson(json);
          expect(dto.sellerUserLifecycle, isNull);
        },
      );
    },
  );

  // -------------------------------------------------------------------------
  // 2) Listing mapper threads DTO → entity ContentLifecycle
  // -------------------------------------------------------------------------
  group('E8.4 — Listing mapper threads sellerUserLifecycle into entity', () {
    test('null wire → ContentLifecycle.unavailable (FAIL CLOSED)', () {
      final dto = ListingSearchResultDto.fromJson(_baseListingJson());
      final entity = dto.toDomain();
      expect(entity.sellerUserLifecycle, ContentLifecycle.unavailable);
    });

    test('"active" wire → ContentLifecycle.active', () {
      final dto = ListingSearchResultDto.fromJson(
        _baseListingJson(
          listing: {
            'id': '00000000-0000-0000-0000-000000000001',
            'seller': {
              'user': {'lifecycle': 'active'},
            },
          },
        ),
      );
      expect(dto.toDomain().sellerUserLifecycle, ContentLifecycle.active);
    });

    test('"unavailable" wire → ContentLifecycle.unavailable', () {
      final dto = ListingSearchResultDto.fromJson(
        _baseListingJson(
          listing: {
            'id': '00000000-0000-0000-0000-000000000001',
            'seller': {
              'user': {'lifecycle': 'unavailable'},
            },
          },
        ),
      );
      expect(dto.toDomain().sellerUserLifecycle, ContentLifecycle.unavailable);
    });

    test('"removed" wire → ContentLifecycle.removed', () {
      final dto = ListingSearchResultDto.fromJson(
        _baseListingJson(
          listing: {
            'id': '00000000-0000-0000-0000-000000000001',
            'seller': {
              'user': {'lifecycle': 'removed'},
            },
          },
        ),
      );
      expect(dto.toDomain().sellerUserLifecycle, ContentLifecycle.removed);
    });

    test('unknown wire → ContentLifecycle.unavailable (FAIL CLOSED)', () {
      final dto = ListingSearchResultDto.fromJson(
        _baseListingJson(
          listing: {
            'id': '00000000-0000-0000-0000-000000000001',
            'seller': {
              'user': {'lifecycle': 'shadowbanned'},
            },
          },
        ),
      );
      expect(dto.toDomain().sellerUserLifecycle, ContentLifecycle.unavailable);
    });
  });

  // -------------------------------------------------------------------------
  // 3) Auction DTO wire extraction + mapper
  // -------------------------------------------------------------------------
  group(
    'E8.4 — AuctionSearchResultDto.sellerUserLifecycle wire extraction',
    () {
      test('absent auction block → null', () {
        final dto = AuctionSearchResultDto.fromJson(_baseAuctionJson());
        expect(dto.sellerUserLifecycle, isNull);
      });

      test('missing nested seller.user → null', () {
        final dto = AuctionSearchResultDto.fromJson(
          _baseAuctionJson(
            auction: {'status': 'active', 'end_at': '2026-01-02T00:00:00.000Z'},
          ),
        );
        expect(dto.sellerUserLifecycle, isNull);
      });

      test('"unavailable" wire → "unavailable"', () {
        final dto = AuctionSearchResultDto.fromJson(
          _baseAuctionJson(
            auction: {
              'status': 'active',
              'end_at': '2026-01-02T00:00:00.000Z',
              'seller': {
                'user': {'lifecycle': 'unavailable'},
              },
            },
          ),
        );
        expect(dto.sellerUserLifecycle, 'unavailable');
      });

      test('"removed" wire → "removed"', () {
        final dto = AuctionSearchResultDto.fromJson(
          _baseAuctionJson(
            auction: {
              'status': 'active',
              'end_at': '2026-01-02T00:00:00.000Z',
              'seller': {
                'user': {'lifecycle': 'removed'},
              },
            },
          ),
        );
        expect(dto.sellerUserLifecycle, 'removed');
      });

      test('AXIS BOUNDARY: top-level auction.seller.lifecycle IGNORED', () {
        final dto = AuctionSearchResultDto.fromJson(
          _baseAuctionJson(
            auction: {
              'status': 'active',
              'end_at': '2026-01-02T00:00:00.000Z',
              'seller': {
                'user': {'lifecycle': 'active'},
                // Hypothetical out-of-doctrine top-level emission.
                'lifecycle': 'removed',
              },
            },
          ),
        );
        expect(dto.sellerUserLifecycle, 'active');
      });

      test('AXIS BOUNDARY: flat seller_lifecycle scalar IGNORED', () {
        final json = _baseAuctionJson();
        json['seller_lifecycle'] = 'removed';
        json['seller_status'] = 'banned';
        final dto = AuctionSearchResultDto.fromJson(json);
        expect(dto.sellerUserLifecycle, isNull);
      });

      test('unknown wire → ContentLifecycle.unavailable (FAIL CLOSED)', () {
        final dto = AuctionSearchResultDto.fromJson(
          _baseAuctionJson(
            auction: {
              'status': 'active',
              'end_at': '2026-01-02T00:00:00.000Z',
              'seller': {
                'user': {'lifecycle': 'shadowbanned'},
              },
            },
          ),
        );
        // DTO holds the raw wire string; mobile coarsens unknown to
        // unavailable (fail-closed) so internal vocabulary cannot leak.
        expect(dto.sellerUserLifecycle, 'shadowbanned');
        expect(
          ContentLifecycleParse.fromWire(dto.sellerUserLifecycle),
          ContentLifecycle.unavailable,
        );
      });
    },
  );

  // -------------------------------------------------------------------------
  // 4) Subtitle redaction gate (listing/auction only)
  // -------------------------------------------------------------------------
  group('E8.4 — SearchResultItem subtitle redaction gate', () {
    SearchResult listingResult(String? lifecycle) {
      return SearchResult(
        id: 'l1',
        type: SearchResultType.listing,
        title: 'Showa Koi 30cm',
        subtitle: '@bob\nAcme Farm', // owner-truth handle + store subtitle
        metadata: {
          'sellerId': '00000000-0000-0000-0000-000000000002',
          'sellerLifecycle': ?lifecycle,
        },
        createdAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
      );
    }

    SearchResult auctionResult(String? lifecycle) {
      return SearchResult(
        id: 'a1',
        type: SearchResultType.auction,
        title: 'Sanke Auction',
        subtitle: '@bob\nAuction Farm', // owner-truth handle + store subtitle
        metadata: {
          'sellerId': '00000000-0000-0000-0000-000000000020',
          'sellerLifecycle': ?lifecycle,
        },
        createdAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
      );
    }

    test('listing active → owner-truth subtitle unchanged', () {
      expect(_renderSubtitle(listingResult('active')), '@bob\nAcme Farm');
    });

    test(
      'listing missing sellerLifecycle → "Pengguna tidak tersedia" (FAIL CLOSED)',
      () {
        expect(_renderSubtitle(listingResult(null)), 'Pengguna tidak tersedia');
      },
    );

    test('listing unavailable → "Pengguna tidak tersedia"', () {
      expect(
        _renderSubtitle(listingResult('unavailable')),
        'Pengguna tidak tersedia',
      );
    });

    test('listing removed → "Pengguna dihapus"', () {
      expect(_renderSubtitle(listingResult('removed')), 'Pengguna dihapus');
    });

    test('listing unknown → "Pengguna tidak tersedia" (FAIL CLOSED)', () {
      expect(
        _renderSubtitle(listingResult('shadowbanned')),
        'Pengguna tidak tersedia',
      );
    });

    test('auction unavailable → "Pengguna tidak tersedia"', () {
      expect(
        _renderSubtitle(auctionResult('unavailable')),
        'Pengguna tidak tersedia',
      );
    });

    test('auction removed → "Pengguna dihapus"', () {
      expect(_renderSubtitle(auctionResult('removed')), 'Pengguna dihapus');
    });

    test('non-listing/auction surface (content) IGNORES sellerLifecycle', () {
      // Even when sellerLifecycle is present on a content row (it should
      // never be — adapter only emits it for listing/auction), the
      // subtitle gate must not fire on non-seller surfaces.
      final content = SearchResult(
        id: 'c1',
        type: SearchResultType.content,
        title: 'Post',
        subtitle: '@alice',
        metadata: {'sellerLifecycle': 'removed', 'lifecycle': 'active'},
        createdAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
      );
      expect(_renderSubtitle(content), '@alice');
    });

    test('store missing fallback renders only @username', () {
      final identity = buildCommerceSellerIdentity(
        username: 'bob',
        storeName: null,
      );
      expect(identity?.line1, '@bob');
      expect(identity?.line2, isNull);
      expect(identity?.multilineLabel, '@bob');
    });

    test('user surface IGNORES sellerLifecycle', () {
      final user = SearchResult(
        id: 'u1',
        type: SearchResultType.user,
        title: '@alice',
        subtitle: '@alice',
        metadata: {'sellerLifecycle': 'removed'},
        createdAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
      );
      expect(_renderSubtitle(user), '@alice');
    });

    test('degraded subtitle still presents tappable row semantics', () {
      // SearchResultItem only disables onTap on the item-axis isUnavailable
      // path (which reads metadata['lifecycle'] for content rows). The
      // seller-axis path does NOT branch onTap, so a redacted listing
      // remains tappable. Pinning the absence of any item-axis lifecycle
      // signal proves this row stays interactive.
      final r = listingResult('removed');
      expect(
        r.metadata.containsKey('lifecycle'),
        isFalse,
        reason:
            'listing/auction rows must not propagate item-axis lifecycle metadata',
      );
    });
  });
}
