// Auction detail wire — origin/shipping NEGATIVE CONTRACT.
//
// Backend authority (GET /api/v1/auctions/:id →
// auctionToDetailResponseWithSeller): the canonical auction detail projection
// carries Product content (title, description, media_urls, variety, size_cm,
// age_months, gender, breeder, bloodline, certificates, preparation_time,
// preparation_note) plus seller_identity / viewer_capabilities. It does NOT
// emit `origin`, `shipping_options`, or `farm_address_id`.
//
// Shipping for an auction is resolved at CLAIM time through the canonical
// shipping domain (checkDeliveryAvailability → /auctions/:id/claim with
// address_id + shipping_option_id), not through detail-surface data. So the
// Auction read model must never grow an origin/shipping surface — those were
// phantom expectations carried only by a stale test.
//
// These tests pin that: the canonical payload parses with no fabricated
// origin/shipping values, and even an illegal payload that smuggles
// origin/shipping keys is ignored (no model, no exception).
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/commerce/catalog/auction/data/dto/auction_dto.dart';
import 'package:labuda/domains/commerce/catalog/auction/data/mappers/auction_mapper.dart';

Map<String, dynamic> _canonicalDetailJson() {
  return <String, dynamic>{
    'id': 'auction-1',
    'seller_id': 'seller-1',
    'product_id': 'product-1',
    'title': 'Showa Koi Auction',
    'description': 'Premium showa',
    'media_urls': <String>['https://cdn.example.com/koi.jpg'],
    'variety': 'Showa',
    'size_cm': 28,
    'age_months': 18,
    'gender': 'female',
    'breeder': 'Akira',
    'bloodline': 'Matsunosuke',
    'certificates': <String>['breeder', 'health'],
    'preparation_time': 'immediate',
    'preparation_note': 'Pickup ready',
    'start_price': 500000,
    'bid_increment': 25000,
    'buy_now_price': 800000,
    'current_bid': 500000,
    'total_bids': 0,
    'status': 'active',
    'created_at': '2026-07-24T00:00:00.000Z',
    'updated_at': '2026-07-24T00:00:00.000Z',
    'start_at': '2026-07-24T00:00:00.000Z',
    'end_at': '2026-07-25T00:00:00.000Z',
  };
}

void main() {
  test('canonical auction detail wire carries no origin/shipping values', () {
    final dto = AuctionDto.fromJson(_canonicalDetailJson());
    final entity = AuctionMapper.toEntity(dto);

    // Canonical Product content IS preserved (replacement proof — the read
    // model keeps the detail contract it actually receives).
    expect(entity.koiDetails.variety, 'Showa');
    expect(entity.koiDetails.sizeInCm, 28.0);
    expect(entity.preparationTime, isNotNull);

    // No fabricated origin/shipping surface: the read model's only
    // location-related slots stay absent on the canonical payload.
    expect(entity.location, isNull);
    expect(entity.farmAddressId, isNull);
  });

  test(
    'illegal origin/shipping_keys on the wire are ignored (no phantom model)',
    () {
      // Payload that illegally smuggles origin/shipping keys — the canonical
      // auction detail backend never emits these.
      final payload = _canonicalDetailJson()
        ..['origin'] = 'Kecamatan, Kota, Provinsi'
        ..['farm_address_id'] = 'address-1'
        ..['shipping_options'] = <Map<String, dynamic>>[
          <String, dynamic>{
            'id': 'ship-1',
            'name': 'JNE',
            'transport_type': 'express',
          },
        ];

      final dto = AuctionDto.fromJson(payload);
      final entity = AuctionMapper.toEntity(dto);

      // Parsed without exception, and the read model does NOT adopt any
      // origin/shipping state from the illegal keys.
      expect(entity.location, isNull);
      expect(entity.farmAddressId, isNull);
    },
  );
}