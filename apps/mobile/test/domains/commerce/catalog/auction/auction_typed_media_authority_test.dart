import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/commerce/catalog/auction/data/dto/auction_dto.dart';
import 'package:labuda/domains/commerce/catalog/auction/data/mappers/auction_mapper.dart';
import 'package:labuda/domains/social/content/domain/entities/content.dart';

Map<String, dynamic> _auctionJson() {
  return <String, dynamic>{
    'id': 'auction-typed-media',
    'seller_id': 'seller-1',
    'title': 'Mixed typed auction',
    'description': 'An auction with canonical typed media rows',
    'media_urls': <String>[
      'https://legacy.example.com/legacy-first.jpg',
      'https://legacy.example.com/legacy-second.mp4',
    ],
    'media': <Map<String, dynamic>>[
      <String, dynamic>{
        'id': 'media-image-a',
        'url': 'https://cdn.example.com/auctions/image-a.jpg',
        'type': 'image',
        'position': 0,
        'created_at': '2026-07-28T00:00:00.000Z',
      },
      <String, dynamic>{
        'id': 'media-video-b',
        'url': 'https://cdn.example.com/auctions/video-b.mp4',
        'type': 'video',
        'position': 1,
        'created_at': '2026-07-28T00:00:00.000Z',
        'thumbnail_url': 'https://cdn.example.com/auctions/video-b-poster.jpg',
      },
    ],
    'start_price': 1500000,
    'bid_increment': 50000,
    'current_highest_bid': 1500000,
    'total_bids': 0,
    'minimum_bid': 1500000,
    'start_at': '2026-07-28T00:00:00.000Z',
    'end_at': '2026-07-29T00:00:00.000Z',
    'time_remaining_seconds': 86400,
    'status': 'active',
    'auto_extend': false,
    'auto_extend_minutes': 10,
    'auto_extend_count': 0,
    'remaining_extensions': 3,
    'views_count': 0,
    'watchers_count': 0,
    'can_bid': true,
    'can_buy_now': false,
    'created_at': '2026-07-28T00:00:00.000Z',
    'updated_at': '2026-07-28T00:00:00.000Z',
  };
}

void main() {
  test('AuctionDto prefers typed media over legacy media_urls', () {
    final dto = AuctionDto.fromJson(_auctionJson());

    expect(dto.media, hasLength(2));
    expect(dto.media[0].id, 'media-image-a');
    expect(dto.media[0].type, 'image');
    expect(dto.media[1].id, 'media-video-b');
    expect(dto.media[1].type, 'video');
    expect(dto.images, [
      'https://cdn.example.com/auctions/image-a.jpg',
      'https://cdn.example.com/auctions/video-b.mp4',
    ]);
  });

  test('AuctionMapper preserves typed media ordering and entity types', () {
    final dto = AuctionDto.fromJson(_auctionJson());
    final entity = AuctionMapper.toEntity(dto);

    expect(entity.media, hasLength(2));
    expect(
      entity.media[0].originalUrl,
      'https://cdn.example.com/auctions/image-a.jpg',
    );
    expect(entity.media[0].type, MediaType.image);
    expect(
      entity.media[0].createdAt,
      DateTime.parse('2026-07-28T00:00:00.000Z'),
    );

    expect(
      entity.media[1].originalUrl,
      'https://cdn.example.com/auctions/video-b.mp4',
    );
    expect(entity.media[1].type, MediaType.video);
    expect(
      entity.media[1].createdAt,
      DateTime.parse('2026-07-28T00:00:00.000Z'),
    );
    expect(
      entity.media[1].posterUrl,
      'https://cdn.example.com/auctions/video-b-poster.jpg',
    );
    expect(
      entity.media[1].variants['thumbnail'],
      'https://cdn.example.com/auctions/video-b-poster.jpg',
    );
  });

  test(
    'AuctionDto falls back to legacy media_urls when typed media is absent',
    () {
      final dto = AuctionDto.fromJson(<String, dynamic>{
        'id': 'auction-legacy-media',
        'seller_id': 'seller-1',
        'title': 'Legacy auction',
        'description': 'Legacy media only',
        'media_urls': <String>[
          'https://legacy.example.com/image-a.jpg',
          'https://legacy.example.com/video-b.mp4',
        ],
        'start_price': 1500000,
        'bid_increment': 50000,
        'current_highest_bid': 1500000,
        'total_bids': 0,
        'minimum_bid': 1500000,
        'start_at': '2026-07-28T00:00:00.000Z',
        'end_at': '2026-07-29T00:00:00.000Z',
        'time_remaining_seconds': 86400,
        'status': 'active',
        'auto_extend': false,
        'auto_extend_minutes': 10,
        'auto_extend_count': 0,
        'remaining_extensions': 3,
        'views_count': 0,
        'watchers_count': 0,
        'can_bid': true,
        'can_buy_now': false,
        'created_at': '2026-07-28T00:00:00.000Z',
        'updated_at': '2026-07-28T00:00:00.000Z',
      });

      expect(dto.media, hasLength(2));
      expect(dto.media[0].type, 'image');
      expect(dto.media[1].type, 'video');
      expect(dto.images, [
        'https://legacy.example.com/image-a.jpg',
        'https://legacy.example.com/video-b.mp4',
      ]);
    },
  );
}
