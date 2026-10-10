// ForSale certificate contract — seller-declared certificates must reach the
// buyer-facing detail surface.
//
// Business truth (owner): a certificate is a plain statement the seller makes
// about the fish (breeder, contest, import, health) so buyers know the fish is
// certified. It is product content like breeder/bloodline — never a document
// upload, and never a hidden write-only field.
//
// Backend authority: `certificates` is persisted on Product and emitted on the
// forSale seller/detail projection. This test proves the mobile read model no
// longer drops that slot (the entity previously had no field and the shared
// detail section hardcoded an empty list, so the value could be written but
// never seen).
//
// Auction pins the identical content contract through `KoiDetails.certificates`
// in auction_detail_screen_runtime_test.dart.
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/domains/commerce/catalog/for_sale/data/dto/for_sale_dto.dart';
import 'package:hishumi/domains/commerce/catalog/for_sale/data/mappers/for_sale_dto_mapper.dart';
import 'package:hishumi/domains/commerce/catalog/shared/presentation/widgets/commerce_common_product_detail_section.dart';

Map<String, dynamic> _detailJson() {
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
  test('detail wire maps seller-declared certificates into the read model', () {
    final payload = _detailJson()
      ..['certificates'] = <String>['breeder', 'import'];

    final dto = ForSaleResponseDto.fromJson(payload);
    final entity = ForSaleDtoMapper.toEntity(dto);

    expect(entity.certificates, const ['breeder', 'import']);
  });

  test('a payload without certificates stays an empty declaration', () {
    final dto = ForSaleResponseDto.fromJson(_detailJson());
    final entity = ForSaleDtoMapper.toEntity(dto);

    expect(entity.certificates, isEmpty);
  });

  testWidgets('the shared detail section renders the canonical labels', (
    tester,
  ) async {
    final payload = _detailJson()
      ..['certificates'] = <String>['import', 'health'];
    final entity = ForSaleDtoMapper.toEntity(
      ForSaleResponseDto.fromJson(payload),
    );

    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: CommerceCommonProductDetailSection(
            title: 'Detail Produk',
            data: CommerceCommonProductDetailsData.fromForSale(entity),
          ),
        ),
      ),
    );

    expect(find.text('Sertifikat'), findsOneWidget);
    expect(find.text('Import, Kesehatan'), findsOneWidget);
  });

  testWidgets('the retired ownership value never renders a label', (
    tester,
  ) async {
    final payload = _detailJson()..['certificates'] = <String>['ownership'];
    final entity = ForSaleDtoMapper.toEntity(
      ForSaleResponseDto.fromJson(payload),
    );

    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: CommerceCommonProductDetailSection(
            title: 'Detail Produk',
            data: CommerceCommonProductDetailsData.fromForSale(entity),
          ),
        ),
      ),
    );

    expect(find.text('Kepemilikan'), findsNothing);
    // No declaration to show at all -> the row is absent, not empty-labelled.
    expect(find.text('Sertifikat'), findsNothing);
  });
}
