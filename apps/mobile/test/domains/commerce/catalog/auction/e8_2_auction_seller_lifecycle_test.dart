// E8.2 — Mobile auction-detail seller user-axis lifecycle ingestion tests.
//
// Mirrors the listing E8.2 test file. Scope is pinned to three seams:
//   1) the wire shape parser that walks `auction.seller.user.lifecycle`
//      into AuctionDto.sellerUserLifecycle,
//   2) the mapper that converts the wire string into the canonical
//      ContentLifecycle enum on the Auction entity,
//   3) the axis-boundary contract — top-level `auction.seller.lifecycle`
//      MUST NOT be read on this surface.

import 'package:flutter_test/flutter_test.dart';

import 'package:labuda/domains/commerce/catalog/auction/data/dto/auction_dto.dart';
import 'package:labuda/domains/commerce/catalog/auction/data/mappers/auction_mapper.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';

Map<String, dynamic> _baseAuctionJson({Map<String, dynamic>? auction}) {
  final base = <String, dynamic>{
    'id': '00000000-0000-0000-0000-000000000001',
    'seller_id': '00000000-0000-0000-0000-000000000002',
    'title': 'Showa Auction',
    'images': <String>[],
    'start_price': 1000000,
    'bid_increment': 50000,
    'current_highest_bid': 1000000,
    'total_bids': 0,
    'minimum_bid': 1050000,
    'start_time': '2026-01-01T00:00:00.000Z',
    'end_time': '2026-01-02T00:00:00.000Z',
    'time_remaining_seconds': 86400,
    'status': 'active',
    'auto_extend': false,
    'auto_extend_minutes': 10,
    'auto_extend_count': 0,
    'remaining_extensions': 3,
    'views_count': 0,
    'watchers_count': 0,
    'can_bid': true,
    'can_buy_now': false,
    'created_at': '2026-01-01T00:00:00.000Z',
    'updated_at': '2026-01-01T00:00:00.000Z',
    'seller_username': 'alice',
    'seller_farm_name': 'Acme Farm',
    'seller_avatar_url': null,
  };
  if (auction != null) {
    base['auction'] = auction;
  }
  return base;
}

void main() {
  group('E8.2 — AuctionDto.sellerUserLifecycle wire extraction', () {
    test('absent auction.seller.user → sellerUserLifecycle null', () {
      final dto = AuctionDto.fromJson(_baseAuctionJson());
      expect(dto.sellerUserLifecycle, isNull);
    });

    test('nested user.lifecycle="active" → "active"', () {
      final dto = AuctionDto.fromJson(
        _baseAuctionJson(
          auction: {
            'id': '00000000-0000-0000-0000-000000000001',
            'title': 'Showa Auction',
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
      final dto = AuctionDto.fromJson(
        _baseAuctionJson(
          auction: {
            'id': '00000000-0000-0000-0000-000000000001',
            'title': 'Showa Auction',
            'seller': {
              'user': {
                'id': '00000000-0000-0000-0000-000000000002',
                'username': 'bob',
                'lifecycle': 'unavailable',
              },
              'farm_name': 'Acme Farm',
              'lifecycle': null,
            },
          },
        ),
      );
      expect(dto.sellerUserLifecycle, 'unavailable');
    });

    test('nested user.lifecycle="removed" → "removed"', () {
      final dto = AuctionDto.fromJson(
        _baseAuctionJson(
          auction: {
            'id': '00000000-0000-0000-0000-000000000001',
            'title': 'Showa Auction',
            'seller': {
              'user': {
                'id': '00000000-0000-0000-0000-000000000002',
                'username': 'ghost',
                'lifecycle': 'removed',
              },
              'farm_name': null,
              'lifecycle': null,
            },
          },
        ),
      );
      expect(dto.sellerUserLifecycle, 'removed');
    });

    test('AXIS BOUNDARY: top-level seller.lifecycle MUST NOT influence '
        'sellerUserLifecycle', () {
      final dto = AuctionDto.fromJson(
        _baseAuctionJson(
          auction: {
            'id': '00000000-0000-0000-0000-000000000001',
            'title': 'Showa Auction',
            'seller': {
              'user': {
                'id': '00000000-0000-0000-0000-000000000002',
                'username': 'alice',
                'lifecycle': 'active',
              },
              'farm_name': 'Acme Farm',
              // Out-of-doctrine emission — MUST be ignored.
              'lifecycle': 'unavailable',
            },
          },
        ),
      );
      expect(dto.sellerUserLifecycle, 'active');
    });
  });

  group('E8.2 — Mapper threads sellerUserLifecycle into Auction entity', () {
    test('null wire → ContentLifecycle.unavailable (FAIL CLOSED)', () {
      final dto = AuctionDto.fromJson(_baseAuctionJson());
      final entity = AuctionMapper.toEntity(dto);
      expect(entity.sellerUserLifecycle, ContentLifecycle.unavailable);
    });

    test('"active" wire → ContentLifecycle.active', () {
      final dto = AuctionDto.fromJson(
        _baseAuctionJson(
          auction: {
            'id': '00000000-0000-0000-0000-000000000001',
            'title': 'Showa Auction',
            'seller': {
              'user': {
                'id': '00000000-0000-0000-0000-000000000002',
                'username': 'alice',
                'lifecycle': 'active',
              },
            },
          },
        ),
      );
      final entity = AuctionMapper.toEntity(dto);
      expect(entity.sellerUserLifecycle, ContentLifecycle.active);
    });

    test('"unavailable" wire → ContentLifecycle.unavailable', () {
      final dto = AuctionDto.fromJson(
        _baseAuctionJson(
          auction: {
            'id': '00000000-0000-0000-0000-000000000001',
            'title': 'Showa Auction',
            'seller': {
              'user': {
                'id': '00000000-0000-0000-0000-000000000002',
                'username': 'bob',
                'lifecycle': 'unavailable',
              },
            },
          },
        ),
      );
      final entity = AuctionMapper.toEntity(dto);
      expect(entity.sellerUserLifecycle, ContentLifecycle.unavailable);
    });

    test('"removed" wire → ContentLifecycle.removed', () {
      final dto = AuctionDto.fromJson(
        _baseAuctionJson(
          auction: {
            'id': '00000000-0000-0000-0000-000000000001',
            'title': 'Showa Auction',
            'seller': {
              'user': {
                'id': '00000000-0000-0000-0000-000000000002',
                'username': 'ghost',
                'lifecycle': 'removed',
              },
            },
          },
        ),
      );
      final entity = AuctionMapper.toEntity(dto);
      expect(entity.sellerUserLifecycle, ContentLifecycle.removed);
    });

    test('unknown wire → ContentLifecycle.unavailable (FAIL CLOSED)', () {
      final dto = AuctionDto.fromJson(
        _baseAuctionJson(
          auction: {
            'id': '00000000-0000-0000-0000-000000000001',
            'title': 'Showa Auction',
            'seller': {
              'user': {
                'id': '00000000-0000-0000-0000-000000000002',
                'username': 'alice',
                'lifecycle': 'shadowbanned',
              },
            },
          },
        ),
      );
      final entity = AuctionMapper.toEntity(dto);
      expect(entity.sellerUserLifecycle, ContentLifecycle.unavailable);
    });
  });
}
