// Auction detail wire — origin/shipping NEGATIVE CONTRACT + Product address
// POSITIVE CONTRACT.
//
// Backend authority (GET /api/v1/auctions → auctionToResponseWithSeller and
// GET /api/v1/auctions/:id → auctionToDetailResponseWithSeller): the canonical
// auction payload carries the shared Product content block
// (shared.ProductContentWireKeys = title, description, media, media_urls,
// variety, size_cm, age_months, gender, breeder, bloodline, certificates,
// farm_address_id, preparation_time, preparation_note) plus seller scalars /
// viewer_capabilities. It does NOT emit `origin` (a rendered address string)
// or `shipping_options`.
//
// CANONICAL TRUTH (Product = single content authority):
//   - `farm_address_id` IS Product content. The backend accepts it on create
//     (CreateAuctionRequest.farm_address_id) and emits it on every read
//     payload, exactly like for_sale. The Auction read model therefore maps
//     it — an always-null slot would be a lie about the payload it received.
//   - Shipping for an auction is resolved at CLAIM time through the canonical
//     shipping domain (checkDeliveryAvailability → /auctions/:id/claim with
//     address_id + shipping_option_id). The read model carries only the
//     address ID — never a rendered origin/shipping surface.
//
// These tests pin that: absence stays absent, the address ID maps honestly
// (for_sale parity), and even an illegal payload that smuggles `origin` /
// `shipping_options` is ignored (no model, no exception).
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

    // Absence stays absent: a payload without farm_address_id yields a null
    // slot — no default, no fabrication.
    expect(entity.farmAddressId, isNull);
  });

  test('canonical farm_address_id maps into the read model (for_sale parity)', () {
    final payload = _canonicalDetailJson()
      ..['farm_address_id'] = 'address-1';

    final dto = AuctionDto.fromJson(payload);
    final entity = AuctionMapper.toEntity(dto);

    // Product address is canonical content on BOTH sale channels; the read
    // model records what the wire actually said.
    expect(entity.farmAddressId, 'address-1');
  });

  test(
    'illegal origin/shipping_keys on the wire are ignored (no phantom model)',
    () {
      // Payload that illegally smuggles origin/shipping keys — the canonical
      // auction backend never emits these.
      final payload = _canonicalDetailJson()
        ..['origin'] = 'Kecamatan, Kota, Provinsi'
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
      expect(entity.farmAddressId, isNull);
    },
  );
}
