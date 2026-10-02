import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/data/dto/for_sale_dto.dart';

void main() {
  group('Create forSale contract', () {
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
