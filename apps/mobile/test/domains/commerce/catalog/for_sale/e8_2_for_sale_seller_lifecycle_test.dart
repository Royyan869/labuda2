// E8.2 — Mobile listing-detail seller user-axis lifecycle ingestion tests.
//
// Scope is pinned to three seams:
//   1) the wire shape parser that walks `listing.seller.user.lifecycle`
//      into ForSaleResponseDto.sellerUserLifecycle,
//   2) the mapper that converts the wire string into the canonical
//      ContentLifecycle enum on the Listing entity,
//   3) the axis-boundary contract — top-level `listing.seller.lifecycle`
//      MUST NOT be read on this surface.
//
// Widget-level golden tests for the rendered seller redaction would
// require a full Riverpod harness; pure-data testing keeps the contract
// pinned without dragging in a new harness — same posture as E2.1 / E3.1 /
// E4.3 / E5.2 / E6 / E8.1.

import 'package:flutter_test/flutter_test.dart';

import 'package:labuda/domains/commerce/catalog/for_sale/data/dto/for_sale_dto.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/data/mappers/for_sale_dto_mapper.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';

Map<String, dynamic> _baseListingJson({Map<String, dynamic>? listing}) {
  return <String, dynamic>{
    'id': '00000000-0000-0000-0000-000000000001',
    'seller_id': '00000000-0000-0000-0000-000000000002',
    'title': 'Showa Koi 30cm',
    'description': 'Premium showa',
    'media_urls': <String>[],
    'listing_type': 'fixed_price',
    'price': 1500000,
    'quantity': 1,
    'negotiation_enabled': false,
    'visibility': 'public',
    'status': 'active',
    'created_at': '2026-01-01T00:00:00.000Z',
    'updated_at': '2026-01-01T00:00:00.000Z',
    'seller_username': 'alice',
    'seller_farm_name': 'Acme Farm',
    'seller_avatar_url': null,
    if (listing != null) 'listing': listing,
  };
}

void main() {
  group('E8.2 — ForSaleResponseDto.sellerUserLifecycle wire extraction', () {
    test('absent listing.seller.user → sellerUserLifecycle null', () {
      final dto = ForSaleResponseDto.fromJson(_baseListingJson());
      expect(dto.sellerUserLifecycle, isNull);
    });

    test('nested user.lifecycle="active" → "active"', () {
      final dto = ForSaleResponseDto.fromJson(
        _baseListingJson(
          listing: {
            'id': '00000000-0000-0000-0000-000000000001',
            'title': 'Showa Koi 30cm',
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
      final dto = ForSaleResponseDto.fromJson(
        _baseListingJson(
          listing: {
            'id': '00000000-0000-0000-0000-000000000001',
            'title': 'Showa Koi 30cm',
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
      final dto = ForSaleResponseDto.fromJson(
        _baseListingJson(
          listing: {
            'id': '00000000-0000-0000-0000-000000000001',
            'title': 'Showa Koi 30cm',
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

    test('empty-string user.lifecycle → null (rollback-safe)', () {
      final dto = ForSaleResponseDto.fromJson(
        _baseListingJson(
          listing: {
            'id': '00000000-0000-0000-0000-000000000001',
            'title': 'Showa Koi 30cm',
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

    test('AXIS BOUNDARY: top-level seller.lifecycle MUST NOT influence '
        'sellerUserLifecycle', () {
      // Even if backend (in violation of doctrine) emitted a non-nil
      // top-level seller.lifecycle, the DTO must NEVER consume it.
      // The walker only reaches user.lifecycle; top-level is ignored.
      final dto = ForSaleResponseDto.fromJson(
        _baseListingJson(
          listing: {
            'id': '00000000-0000-0000-0000-000000000001',
            'title': 'Showa Koi 30cm',
            'seller': {
              'user': {
                'id': '00000000-0000-0000-0000-000000000002',
                'username': 'alice',
                'lifecycle': 'active',
              },
              'farm_name': 'Acme Farm',
              // Hypothetical out-of-doctrine emission — must be IGNORED.
              'lifecycle': 'unavailable',
            },
          },
        ),
      );
      // sellerUserLifecycle reflects ONLY user.lifecycle = "active".
      expect(dto.sellerUserLifecycle, 'active');
    });
  });

  group('E8.2 — Mapper threads sellerUserLifecycle into Listing entity', () {
    test('null wire → ContentLifecycle.unavailable (FAIL CLOSED)', () {
      final dto = ForSaleResponseDto.fromJson(_baseListingJson());
      final entity = ForSaleDtoMapper.toEntity(dto);
      expect(entity.sellerUserLifecycle, ContentLifecycle.unavailable);
    });

    test('"active" wire → ContentLifecycle.active', () {
      final dto = ForSaleResponseDto.fromJson(
        _baseListingJson(
          listing: {
            'id': '00000000-0000-0000-0000-000000000001',
            'title': 'Showa Koi 30cm',
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
      final entity = ForSaleDtoMapper.toEntity(dto);
      expect(entity.sellerUserLifecycle, ContentLifecycle.active);
    });

    test('"unavailable" wire → ContentLifecycle.unavailable', () {
      final dto = ForSaleResponseDto.fromJson(
        _baseListingJson(
          listing: {
            'id': '00000000-0000-0000-0000-000000000001',
            'title': 'Showa Koi 30cm',
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
      final entity = ForSaleDtoMapper.toEntity(dto);
      expect(entity.sellerUserLifecycle, ContentLifecycle.unavailable);
    });

    test('"removed" wire → ContentLifecycle.removed', () {
      final dto = ForSaleResponseDto.fromJson(
        _baseListingJson(
          listing: {
            'id': '00000000-0000-0000-0000-000000000001',
            'title': 'Showa Koi 30cm',
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
      final entity = ForSaleDtoMapper.toEntity(dto);
      expect(entity.sellerUserLifecycle, ContentLifecycle.removed);
    });

    test('unknown wire → ContentLifecycle.unavailable (FAIL CLOSED)', () {
      final dto = ForSaleResponseDto.fromJson(
        _baseListingJson(
          listing: {
            'id': '00000000-0000-0000-0000-000000000001',
            'title': 'Showa Koi 30cm',
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
      final entity = ForSaleDtoMapper.toEntity(dto);
      expect(entity.sellerUserLifecycle, ContentLifecycle.unavailable);
    });
  });
}
