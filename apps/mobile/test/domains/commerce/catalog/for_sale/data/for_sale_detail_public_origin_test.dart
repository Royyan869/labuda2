// for_sale detail wire — buyer-facing ORIGIN contract.
//
// Backend authority (GET /api/v1/for-sales/:id →
// forSaleToDetailResponseWithViewerCapabilities): the DETAIL payload carries
// `public_origin_line`, the buyer-facing origin summary of the listing's
// sender address ("City, Province"), resolved from Product.FarmAddressID (the
// seller's primary sender address as fallback) and already redacted to
// city + province. Discovery/list payloads do NOT carry it.
//
// Auction pins the identical contract in
// auction_response_dto_shipping_origin_test.dart; both channels must bind the
// SAME wire slot so the shared CommerceDetailSellerCard renders one truth.
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/data/dto/for_sale_dto.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/data/mappers/for_sale_dto_mapper.dart';

Map<String, dynamic> _canonicalDetailJson() {
  return <String, dynamic>{
    'id': 'for-sale-1',
    'product_id': 'product-1',
    'seller_id': 'seller-1',
    'title': 'Kohaku 30cm',
    'description': 'Premium kohaku',
    'media_urls': <String>['https://cdn.example.com/koi.jpg'],
    'price': 750000,
    'quantity': 1,
    'status': 'active',
    'created_at': '2026-09-24T00:00:00.000Z',
    'updated_at': '2026-09-24T00:00:00.000Z',
  };
}

void main() {
  test('detail wire maps the buyer-facing origin summary', () {
    final payload = _canonicalDetailJson()
      ..['public_origin_line'] = 'Magelang, Jawa Tengah';

    final dto = ForSaleResponseDto.fromJson(payload);
    final entity = ForSaleDtoMapper.toEntity(dto);

    expect(entity.publicOriginLine, 'Magelang, Jawa Tengah');
  });

  test('discovery payloads hide the origin instead of fabricating one', () {
    final dto = ForSaleResponseDto.fromJson(_canonicalDetailJson());
    final entity = ForSaleDtoMapper.toEntity(dto);

    expect(entity.publicOriginLine, isNull);
  });

  test('a full street address is not an accepted origin transport', () {
    final payload = _canonicalDetailJson()
      ..['origin'] = 'Jl. Rahasia 1, Kecamatan Borobudur, Magelang, Jawa Tengah';

    final dto = ForSaleResponseDto.fromJson(payload);
    final entity = ForSaleDtoMapper.toEntity(dto);

    expect(entity.publicOriginLine, isNull);
  });
}
