import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/data/dto/for_sale_dto.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/data/mappers/for_sale_dto_mapper.dart';
import 'package:labuda/domains/social/content/domain/entities/content.dart';

Map<String, dynamic> _listingJson() {
  return <String, dynamic>{
    'id': 'forSale-typed-media',
    'seller_id': 'seller-1',
    'title': 'Mixed typed forSale',
    'description': 'A forSale with canonical typed media rows',
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
  test('ForSaleResponseDto parses typed media items and preserves legacy media_urls', () {
    final dto = ForSaleResponseDto.fromJson(_listingJson());

    expect(dto.mediaItems, hasLength(2));
    expect(dto.mediaItems[0].id, 'media-image-a');
    expect(dto.mediaItems[0].type, 'image');
    expect(dto.mediaItems[1].id, 'media-video-b');
    expect(dto.mediaItems[1].type, 'video');
    // mediaUrls preserves the legacy media_urls field independently.
    expect(dto.mediaUrls, [
      'https://legacy.example.com/legacy-first.jpg',
      'https://legacy.example.com/legacy-second.mp4',
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

  test('empty-string thumbnail_url normalizes to null and does not shadow originalUrl', () {
    // Backend renders thumbnail_url as "" when the media row has no
    // thumbnail (stringValue(nil)). The DTO must normalize empty → null
    // and the mapper must not synthesize a blank 'thumbnail' variant that
    // would shadow originalUrl via variants['thumbnail'] ?? originalUrl.
    final dto = ForSaleResponseDto.fromJson(<String, dynamic>{
      'id': 'forSale-empty-thumb',
      'seller_id': 'seller-1',
      'title': 'No thumb',
      'description': '',
      'media': <Map<String, dynamic>>[
        <String, dynamic>{
          'id': 'media-no-thumb',
          'url': 'https://cdn.example.com/products/no-thumb.jpg',
          'type': 'image',
          'position': 0,
          'created_at': '2026-07-28T00:00:00.000Z',
          'thumbnail_url': '',
        },
      ],
      'price': 1500000,
      'quantity': 1,
      'visibility': 'public',
      'status': 'active',
      'created_at': '2026-07-28T00:00:00.000Z',
      'updated_at': '2026-07-28T00:00:00.000Z',
    });

    expect(dto.mediaItems, hasLength(1));
    expect(dto.mediaItems[0].thumbnailUrl, isNull);

    final entity = ForSaleDtoMapper.toEntity(dto);
    expect(entity.media, hasLength(1));
    expect(entity.media[0].variants, isEmpty);
    // thumbnailUrl falls back to originalUrl instead of the blank string.
    expect(
      entity.media[0].thumbnailUrl,
      'https://cdn.example.com/products/no-thumb.jpg',
    );
  });

  test(
    'ForSaleResponseDto preserves legacy media_urls separately from typed mediaItems',
    () {
      final dto = ForSaleResponseDto.fromJson(<String, dynamic>{
        'id': 'forSale-legacy-media',
        'seller_id': 'seller-1',
        'title': 'Legacy forSale',
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

      // When no typed 'media' array is present, mediaItems stays empty.
      expect(dto.mediaItems, isEmpty);
      // Legacy media_urls are preserved in the mediaUrls field.
      expect(dto.mediaUrls, [
        'https://legacy.example.com/image-a.jpg',
        'https://legacy.example.com/video-b.mp4',
      ]);
    },
  );
}
