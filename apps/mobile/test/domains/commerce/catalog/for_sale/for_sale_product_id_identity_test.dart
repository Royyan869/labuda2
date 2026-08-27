import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/data/dto/for_sale_dto.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/data/mappers/for_sale_dto_mapper.dart';

Map<String, dynamic> _baseListingJson({
  required String saleId,
  String? productId,
}) {
  return <String, dynamic>{
    'id': saleId,
    if (productId != null) 'product_id': productId,
    'seller_id': '33333333-3333-3333-3333-333333333333',
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
  };
}

void main() {
  group('ForSaleResponseDto product identity', () {
    test('maps backend id and product_id to distinct fields', () {
      const forSaleId = '22222222-2222-2222-2222-222222222222';
      const productId = '11111111-1111-1111-1111-111111111111';

      final dto = ForSaleResponseDto.fromJson(
        _baseListingJson(saleId: forSaleId, productId: productId),
      );

      expect(dto.id, forSaleId);
      expect(dto.productId, productId);

      final entity = ForSaleDtoMapper.toEntity(dto);
      expect(entity.forSaleId, forSaleId);
      expect(entity.productId, productId);
      expect(entity.productId, isNot(equals(entity.forSaleId)));
    });

    test('missing product_id stays null instead of falling back to id', () {
      const forSaleId = '22222222-2222-2222-2222-222222222222';

      final dto = ForSaleResponseDto.fromJson(
        _baseListingJson(saleId: forSaleId),
      );

      expect(dto.id, forSaleId);
      expect(dto.productId, isNull);

      final entity = ForSaleDtoMapper.toEntity(dto);
      expect(entity.forSaleId, forSaleId);
      expect(entity.productId, isNull);
    });
  });
}
