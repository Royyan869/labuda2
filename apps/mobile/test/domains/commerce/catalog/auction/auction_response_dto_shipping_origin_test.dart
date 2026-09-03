import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/commerce/catalog/auction/data/dto/auction_dto.dart';
import 'package:labuda/domains/commerce/catalog/auction/data/mappers/auction_mapper.dart';

Map<String, dynamic> _baseAuctionJson({
  required String auctionId,
  required String sellerId,
}) {
  return <String, dynamic>{
    'id': auctionId,
    'seller_id': sellerId,
    'product_id': 'product-1',
    'title': 'Showa Koi Auction',
    'description': 'Premium showa',
    'media_urls': <String>[],
    'variety': 'Showa',
    'size_cm': 28,
    'age_months': 18,
    'gender': 'female',
    'breeder': 'Akira',
    'bloodline': 'Matsunosuke',
    'certificates': <String>['breeder', 'health'],
    'farm_address_id': 'address-1',
    'preparation_time': 'immediate',
    'preparation_note': 'Pickup ready',
    'origin': 'Kecamatan, Kota, Provinsi',
    'shipping_options': <Map<String, dynamic>>[
      <String, dynamic>{
        'id': 'ship-1',
        'name': 'JNE',
        'transport_type': 'express',
      },
    ],
    'start_price': 500000,
    'bid_increment': 25000,
    'buy_now_price': 800000,
    'current_bid': 500000,
    'total_bids': 0,
    'status': 'active',
    'auto_extend': false,
    'auto_extend_minutes': 10,
    'auto_extend_count': 0,
    'remaining_extensions': 3,
    'views_count': 0,
    'watchers_count': 0,
    'can_bid': true,
    'can_buy_now': true,
    'created_at': '2026-07-24T00:00:00.000Z',
    'updated_at': '2026-07-24T00:00:00.000Z',
    'start_at': '2026-07-24T00:00:00.000Z',
    'end_at': '2026-07-25T00:00:00.000Z',
    'time_remaining_seconds': 86400,
  };
}

void main() {
  test('auction response parses public origin and shipping summaries', () {
    final dto = AuctionDto.fromJson(
      _baseAuctionJson(auctionId: 'auction-1', sellerId: 'seller-1'),
    );

    expect(dto.origin, 'Kecamatan, Kota, Provinsi');
    expect(dto.shippingSetups, hasLength(1));
    expect(dto.shippingSetups.first.id, 'ship-1');
    expect(dto.shippingSetups.first.name, 'JNE');
    expect(dto.shippingSetups.first.transportType, 'express');
    expect(dto.shippingSetupIds, ['ship-1']);

    final entity = AuctionMapper.toEntity(dto);
    expect(entity.origin, 'Kecamatan, Kota, Provinsi');
    expect(entity.shippingSetups, hasLength(1));
    expect(entity.shippingSetups.first.id, 'ship-1');
    expect(entity.shippingSetups.first.name, 'JNE');
    expect(entity.shippingSetups.first.transportType, 'express');
  });
}
