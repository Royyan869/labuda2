// Auction detail wire — buyer-facing ORIGIN contract.
//
// Backend authority (GET /api/v1/auctions → auctionToResponseWithSeller and
// GET /api/v1/auctions/:id → auctionToDetailResponseWithSeller): the canonical
// auction payload carries the shared Product content block
// (shared.ProductContentWireKeys) plus seller scalars / viewer_capabilities.
// The DETAIL payload additionally carries `public_origin_line` — the
// buyer-facing origin summary of the account's primary address
// ("City, Province").
//
// CANONICAL TRUTH:
//   - There is no product-level origin address. Every product's origin is the
//     seller account's primary address, resolved by the backend at read time.
//   - `public_origin_line` is the ONE origin transport, and it is DETAIL-ONLY:
//     backend-redacted to city + province. A rendered full address string
//     (`origin`) and `shipping_options` are NOT emitted; shipping for an
//     auction is resolved at CLAIM time.
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
    'preparation_time': '1_3_days',
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
  });

  test(
    'illegal origin/shipping_keys on the wire are ignored (no phantom model)',
    () {
      // Payload that illegally smuggles origin/shipping keys — the canonical
      // auction backend never emits these.
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
      // rendered origin/shipping surface from the illegal keys.
      expect(entity.publicOriginLine, isNull);
    },
  );

  test('canonical detail wire maps the buyer-facing origin summary', () {
    final payload = _canonicalDetailJson()
      ..['public_origin_line'] = 'Magelang, Jawa Tengah';

    final dto = AuctionDto.fromJson(payload);
    final entity = AuctionMapper.toEntity(dto);

    expect(entity.publicOriginLine, 'Magelang, Jawa Tengah');
  });

  test('discovery payloads hide the origin instead of fabricating one', () {
    final dto = AuctionDto.fromJson(_canonicalDetailJson());
    final entity = AuctionMapper.toEntity(dto);

    // The wire slot is detail-only; a list payload omits it and the read model
    // keeps a null slot (the seller card then hides the line).
    expect(entity.publicOriginLine, isNull);
  });
}
