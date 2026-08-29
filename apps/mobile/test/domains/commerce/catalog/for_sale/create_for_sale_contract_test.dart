import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/data/dto/for_sale_dto.dart';

void main() {
  group('Create listing contract', () {
    test(
      'CreateForSaleRequestDto keeps legacy media_urls when typed media is absent',
      () {
        final dto = CreateForSaleRequestDto(
          title: 'Legacy Listing',
          description: 'Legacy payload',
          price: 1500000,
          quantity: 1,
          mediaUrls: const ['https://legacy.example.com/a.jpg'],
        );

        final json = dto.toJson();

        expect(json['media_urls'], const ['https://legacy.example.com/a.jpg']);
        expect(json.containsKey('media'), isFalse);
      },
    );
  });
}
