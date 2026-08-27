import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/data/dto/for_sale_dto.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/data/mappers/for_sale_dto_mapper.dart';
import 'package:labuda/domains/social/content/domain/entities/content.dart';

Map<String, dynamic> _listingJson() {
  return <String, dynamic>{
    'id': 'listing-typed-media',
    'seller_id': 'seller-1',
    'title': 'Mixed typed listing',
    'description': 'A listing with canonical typed media rows',
    'media_urls': <String>[
      'https://legacy.example.com/legacy-first.jpg',
      'https://legacy.example.com/legacy-second.mp4',
    ],
    'media': <Map<String, dynamic>>[
      <String, dynamic>{
        'id': 'media-image-a',
        'url': 'https://cdn.example.com/products/image-a.jpg',
        'type': 'image',
        'position': 0,
        'created_at': '2026-07-28T00:00:00.000Z',
      },
      <String, dynamic>{
        'id': 'media-video-b',
        'url': 'https://cdn.example.com/products/video-b.mp4',
        'type': 'video',
        'position': 1,
        'created_at': '2026-07-28T00:00:00.000Z',
        'thumbnail_url': 'https://cdn.example.com/products/video-b-poster.jpg',
      },
    ],
    'price': 1500000,
    'quantity': 1,
    'negotiation_enabled': false,
    'visibility': 'public',
    'status': 'active',
    'created_at': '2026-07-28T00:00:00.000Z',
    'updated_at': '2026-07-28T00:00:00.000Z',
  };
}

void main() {
  test('ForSaleResponseDto prefers typed media over legacy media_urls', () {
    final dto = ForSaleResponseDto.fromJson(_listingJson());

    expect(dto.media, hasLength(2));
    expect(dto.media[0].id, 'media-image-a');
    expect(dto.media[0].type, 'image');
    expect(dto.media[1].id, 'media-video-b');
    expect(dto.media[1].type, 'video');
    expect(dto.mediaUrls, [
      'https://cdn.example.com/products/image-a.jpg',
      'https://cdn.example.com/products/video-b.mp4',
    ]);
  });

  test('ForSaleDtoMapper preserves typed media ordering and entity types', () {
    final dto = ForSaleResponseDto.fromJson(_listingJson());
    final entity = ForSaleDtoMapper.toEntity(dto);

    expect(entity.media, hasLength(2));
    expect(
      entity.media[0].originalUrl,
      'https://cdn.example.com/products/image-a.jpg',
    );
    expect(entity.media[0].type, MediaType.image);
    expect(
      entity.media[0].createdAt,
      DateTime.parse('2026-07-28T00:00:00.000Z'),
    );

    expect(
      entity.media[1].originalUrl,
      'https://cdn.example.com/products/video-b.mp4',
    );
    expect(entity.media[1].type, MediaType.video);
    expect(
      entity.media[1].createdAt,
      DateTime.parse('2026-07-28T00:00:00.000Z'),
    );
    expect(
      entity.media[1].posterUrl,
      'https://cdn.example.com/products/video-b-poster.jpg',
    );
    expect(
      entity.media[1].variants['thumbnail'],
      'https://cdn.example.com/products/video-b-poster.jpg',
    );
  });

  test(
    'ForSaleResponseDto falls back to legacy media_urls when typed media is absent',
    () {
      final dto = ForSaleResponseDto.fromJson(<String, dynamic>{
        'id': 'listing-legacy-media',
        'seller_id': 'seller-1',
        'title': 'Legacy listing',
        'description': 'Legacy media only',
        'media_urls': <String>[
          'https://legacy.example.com/image-a.jpg',
          'https://legacy.example.com/video-b.mp4',
        ],
        'price': 1500000,
        'quantity': 1,
        'visibility': 'public',
        'status': 'active',
        'created_at': '2026-07-28T00:00:00.000Z',
        'updated_at': '2026-07-28T00:00:00.000Z',
      });

      expect(dto.media, hasLength(2));
      expect(dto.media[0].type, 'image');
      expect(dto.media[1].type, 'video');
      expect(dto.mediaUrls, [
        'https://legacy.example.com/image-a.jpg',
        'https://legacy.example.com/video-b.mp4',
      ]);
    },
  );
}
