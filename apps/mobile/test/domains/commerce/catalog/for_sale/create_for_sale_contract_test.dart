import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/domains/commerce/catalog/for_sale/data/dto/for_sale_dto.dart';

void main() {
  group('Create forSale contract', () {
    test('preserves canonical quantity and negotiation combinations', () {
      final cases = [
        (quantity: 1, negotiation: false),
        (quantity: 10, negotiation: false),
        (quantity: 1, negotiation: true),
        (quantity: 10, negotiation: true),
      ];

      for (final value in cases) {
        final json = CreateForSaleRequestDto(
          title: 'ForSale',
          description: 'Description',
          price: 1500000,
          quantity: value.quantity,
          negotiationEnabled: value.negotiation,
        ).toJson();

        expect(json['quantity'], value.quantity);
        expect(json['negotiation_enabled'], value.negotiation);
      }
    });

    test(
      'CreateForSaleRequestDto sends typed media[] and never legacy media_urls',
      () {
        final dto = CreateForSaleRequestDto(
          title: 'Typed ForSale',
          description: 'Typed payload',
          price: 1500000,
          quantity: 1,
          media: const [
            {
              'type': 'image',
              'url': 'https://legacy.example.com/a.jpg',
              'blurhash': 'LKO2?U%2Tw=w]~RBVZRi};RPxuwH',
            },
          ],
        );

        final json = dto.toJson();

        expect(json['media'], const [
          {
            'type': 'image',
            'url': 'https://legacy.example.com/a.jpg',
            'blurhash': 'LKO2?U%2Tw=w]~RBVZRi};RPxuwH',
          },
        ]);
        expect(json.containsKey('media_urls'), isFalse);
      },
    );
  });
}
