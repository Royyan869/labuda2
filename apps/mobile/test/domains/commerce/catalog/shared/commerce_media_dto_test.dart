import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/commerce/catalog/shared/data/dto/commerce_media_dto.dart';
import 'package:labuda/domains/social/content/domain/entities/content.dart';

void main() {
  test('CommerceMediaDto parses typed media metadata and variants', () {
    final dto = CommerceMediaDto.fromJson(<String, dynamic>{
      'id': 'media-video-b',
      'url': 'https://cdn.example.com/products/video-b.mp4',
      'type': 'video',
      'position': 1,
      'created_at': '2026-07-28T00:00:00.000Z',
      'blurhash': 'LEHV6nWB2yk8pyo0adR*.7kCMdnj',
      'duration': 42,
      'width': 1920,
      'height': 1080,
      'variants': <String, dynamic>{
        'thumbnail': 'https://cdn.example.com/products/video-b-poster.jpg',
      },
    });

    expect(dto.id, 'media-video-b');
    expect(dto.url, 'https://cdn.example.com/products/video-b.mp4');
    expect(dto.type, 'video');
    expect(dto.position, 1);
    expect(dto.createdAt, DateTime.parse('2026-07-28T00:00:00.000Z'));
    expect(dto.blurhash, 'LEHV6nWB2yk8pyo0adR*.7kCMdnj');
    expect(dto.duration, 42);
    expect(dto.width, 1920);
    expect(dto.height, 1080);
    expect(
      dto.variants['thumbnail'],
      'https://cdn.example.com/products/video-b-poster.jpg',
    );
  });

  test('CommerceMediaDto legacy URL infers stable video identity', () {
    final dto = CommerceMediaDto.legacyUrl(
      'https://cdn.example.com/products/fish-clip.mp4',
      position: 3,
      createdAt: DateTime.utc(2026, 7, 28),
    );

    expect(dto.id, 'fish-clip');
    expect(dto.type, 'video');
    expect(dto.position, 3);
    expect(dto.createdAt, DateTime.utc(2026, 7, 28));

    final entity = dto.toEntity();
    expect(entity.id, 'fish-clip');
    expect(
      entity.originalUrl,
      'https://cdn.example.com/products/fish-clip.mp4',
    );
    expect(entity.type, MediaType.video);
    expect(entity.position, 3);
    expect(entity.createdAt, DateTime.utc(2026, 7, 28));
  });

  test(
    'CommerceMediaDto fallback entity creation keeps zero timestamp inert',
    () {
      final dto = CommerceMediaDto.fromJson(<String, dynamic>{
        'url': 'https://cdn.example.com/products/showa.jpg',
        'position': 0,
      });

      final entity = dto.toEntity();
      expect(entity.id, 'showa');
      expect(entity.type, MediaType.image);
      expect(
        entity.createdAt,
        DateTime.fromMillisecondsSinceEpoch(0, isUtc: true),
      );
    },
  );
}
