import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/commerce/catalog/auction/data/dto/auction_dto.dart';
import 'package:labuda/domains/commerce/catalog/auction/data/mappers/auction_mapper.dart';
import 'package:labuda/domains/social/content/domain/entities/content.dart';

/// CONVERGED media contract (typed-media alignment with for_sale):
/// the backend auction detail wire now projects Product.MediaURLs through
/// commerce/shared MediaWireItems, emitting a typed `media` block with the
/// SAME shape as for_sale detail (id, type, url, position, thumbnail_url,
/// width, height, duration, created_at).
///
/// Authority order: typed `media` rows (when present) carry canonical
/// metadata (type/dimensions/thumbnail); flat `media_urls`/`images` remain
/// the universal fallback for list payloads.
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
    'current_bid': 1500000,
    'current_winner_id': null,
    'start_at': '2026-07-28T00:00:00.000Z',
    'end_at': '2026-07-29T00:00:00.000Z',
    'status': 'active',
    'created_at': '2026-07-28T00:00:00.000Z',
    'updated_at': '2026-07-28T00:00:00.000Z',
  };
}

void main() {
  test('AuctionDto parses typed media rows into mediaItems', () {
    final dto = AuctionDto.fromJson(_auctionJson());

    expect(dto.mediaItems, hasLength(2));
    expect(dto.mediaItems[0].id, 'media-image-a');
    expect(
      dto.mediaItems[0].url,
      'https://cdn.example.com/auctions/image-a.jpg',
    );
    expect(dto.mediaItems[0].isVideo, isFalse);
    expect(dto.mediaItems[1].id, 'media-video-b');
    expect(dto.mediaItems[1].isVideo, isTrue);
    expect(
      dto.mediaItems[1].thumbnailUrl,
      'https://cdn.example.com/auctions/video-b-poster.jpg',
    );
    // Flat normalization stays available for widgets that want raw URLs.
    expect(dto.images, [
      'https://legacy.example.com/legacy-first.jpg',
      'https://legacy.example.com/legacy-second.mp4',
    ]);
  });

  test('AuctionMapper prefers typed media rows (video type preserved)', () {
    final dto = AuctionDto.fromJson(_auctionJson());
    final entity = AuctionMapper.toEntity(dto);

    expect(entity.media, hasLength(2));
    expect(
      entity.media[0].originalUrl,
      'https://cdn.example.com/auctions/image-a.jpg',
    );
    expect(entity.media[0].type, MediaType.image);
    expect(
      entity.media[1].originalUrl,
      'https://cdn.example.com/auctions/video-b.mp4',
    );
    // Typed rows carry the canonical type — no fabrication needed.
    expect(entity.media[1].type, MediaType.video);
  });

  test('AuctionDto falls back to legacy media_urls when media rows absent', () {
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
      'current_bid': 1500000,
      'current_winner_id': null,
      'start_at': '2026-07-28T00:00:00.000Z',
      'end_at': '2026-07-29T00:00:00.000Z',
      'status': 'active',
      'created_at': '2026-07-28T00:00:00.000Z',
      'updated_at': '2026-07-28T00:00:00.000Z',
    });

    expect(dto.images, [
      'https://legacy.example.com/image-a.jpg',
      'https://legacy.example.com/video-b.mp4',
    ]);
    expect(dto.mediaItems, isEmpty);

    // Fallback mapper path: flat URLs become image-only MediaEntities.
    final entity = AuctionMapper.toEntity(dto);
    expect(entity.media, hasLength(2));
    expect(entity.media[0].type, MediaType.image);
    expect(entity.media[1].type, MediaType.image);
  });
}
