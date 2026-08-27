import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/data/dto/for_sale_dto.dart';
import 'package:labuda/domains/commerce/catalog/shared/data/dto/commerce_media_request_dto.dart';

void main() {
  group('Create listing contract', () {
    test(
      'CreateForSaleRequestDto serializes typed media instead of media_urls',
      () {
        final dto = CreateForSaleRequestDto(
          title: 'Kohaku Listing',
          description: 'Listing request payload',
          price: 1500000,
          quantity: 2,
          media: [
            CommerceMediaRequestDto.image(
              url: 'https://cdn.example.com/listing-a.jpg',
              width: 1280,
              height: 960,
            ).copyWith(position: 0),
            CommerceMediaRequestDto.video(
              url: 'https://cdn.example.com/listing-b.mp4',
              width: 1920,
              height: 1080,
              duration: 24222,
              thumbnailUrl: 'https://cdn.example.com/listing-b-poster.jpg',
            ).copyWith(position: 1),
          ],
          mediaUrls: const ['https://cdn.example.com/legacy.jpg'],
          visibility: 'private',
        );

        final json = dto.toJson();

        expect(json['title'], 'Kohaku Listing');
        expect(json['visibility'], 'private');
        expect(json['media'], hasLength(2));
        expect(json.containsKey('media_urls'), isFalse);
        expect(json['media'][0], containsPair('type', 'image'));
        expect(json['media'][0], containsPair('position', 0));
        expect(
          json['media'][1],
          containsPair(
            'thumbnail_url',
            'https://cdn.example.com/listing-b-poster.jpg',
          ),
        );
        expect(json['media'][1], containsPair('position', 1));
      },
    );

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
