import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/features/search/search/data/dto/search_dto.dart';
import 'package:labuda/features/search/search/data/mappers/search_mapper.dart';

Map<String, dynamic> _baseContentJson({
  String caption = 'Caption text',
  List<String> mediaUrls = const [],
  Map<String, dynamic>? resourceProjection,
}) {
  final json = <String, dynamic>{
    'id': 'content-1',
    'author_id': 'author-1',
    'author_username': 'alice',
    'author_avatar_url': null,
    'type': 'post',
    'caption': caption,
    'media_urls': mediaUrls,
    'created_at': '2026-06-02T10:00:00.000Z',
  };
  if (resourceProjection != null) {
    json['resource_projection'] = resourceProjection;
  }
  return json;
}

Map<String, dynamic> _fixedPriceSaleProjection({
  required String resourceId,
  required String title,
  required String thumbnail,
}) {
  return <String, dynamic>{
    'state': 'LIVE',
    'resource_type': 'fixed_price_sale',
    'resource_id': resourceId,
    'fixed_price_sale': <String, dynamic>{
      'title': title,
      'media': <Map<String, dynamic>>[],
      'thumbnail_url': thumbnail,
      'price': 1500000,
      'status': 'active',
      'quantity_available': 3,
      'can_interact': true,
      'seller': <String, dynamic>{
        'user': <String, dynamic>{
          'id': 'seller-1',
          'username': 'seller',
        },
      },
    },
  };
}

Map<String, dynamic> _auctionProjection({
  required String resourceId,
  required String title,
  required String thumbnail,
}) {
  return <String, dynamic>{
    'state': 'LIVE',
    'resource_type': 'auction',
    'resource_id': resourceId,
    'auction': <String, dynamic>{
      'title': title,
      'media': <Map<String, dynamic>>[],
      'thumbnail_url': thumbnail,
      'lifecycle': 'active',
      'current_bid': 1750000,
      'buy_now_price': 2500000,
      'end_at': '2026-08-10T10:00:00.000Z',
      'can_interact': true,
      'seller': <String, dynamic>{
        'user': <String, dynamic>{
          'id': 'seller-1',
          'username': 'seller',
        },
      },
    },
  };
}

Map<String, dynamic> _profileProjection({
  required String resourceId,
  required String username,
  required String avatarUrl,
}) {
  return <String, dynamic>{
    'state': 'LIVE',
    'resource_type': 'profile',
    'resource_id': resourceId,
    'profile': <String, dynamic>{
      'username': username,
      'avatar_url': avatarUrl,
      'lifecycle': 'active',
    },
  };
}

void main() {
  test('fixed_price_sale resource_projection search keeps projection title and thumbnail', () {
    final dto = ContentSearchResultDto.fromJson(
      _baseContentJson(
        caption: '',
        resourceProjection: _fixedPriceSaleProjection(
          resourceId: 'fixed-price-sale-1',
          title: 'FixedPriceSale Title',
          thumbnail: 'https://img.example/fixed-price-sale.jpg',
        ),
      ),
    );
    expect(dto.resourceProjection, isNotNull);
    expect(dto.resourceProjection!.titleText, 'FixedPriceSale Title');
    expect(dto.resourceProjection!.imageUrl, 'https://img.example/fixed-price-sale.jpg');

    final entity = dto.toDomain();
    expect(entity.title, 'FixedPriceSale Title');
    expect(entity.thumbnailUrl, 'https://img.example/fixed-price-sale.jpg');
  });

  test('auction resource_projection search now projects auction preview data', () {
    final dto = ContentSearchResultDto.fromJson(
      _baseContentJson(
        caption: '',
        resourceProjection: _auctionProjection(
          resourceId: 'auction-1',
          title: 'Auction Title',
          thumbnail: 'https://img.example/auction.jpg',
        ),
      ),
    );

    expect(dto.resourceProjection, isNotNull);
    expect(dto.resourceProjection!.titleText, 'Auction Title');
    expect(dto.resourceProjection!.imageUrl, 'https://img.example/auction.jpg');

    final entity = dto.toDomain();
    expect(entity.title, 'Auction Title');
    expect(entity.thumbnailUrl, 'https://img.example/auction.jpg');
  });

  test('profile resource_projection search now projects profile preview data', () {
    final dto = ContentSearchResultDto.fromJson(
      _baseContentJson(
        caption: '',
        resourceProjection: _profileProjection(
          resourceId: 'profile-1',
          username: 'profile-title',
          avatarUrl: 'https://img.example/profile.jpg',
        ),
      ),
    );

    expect(dto.resourceProjection, isNotNull);
    expect(dto.resourceProjection!.titleText, '@profile-title');
    expect(dto.resourceProjection!.imageUrl, 'https://img.example/profile.jpg');

    final entity = dto.toDomain();
    expect(entity.title, '@profile-title');
    expect(entity.thumbnailUrl, 'https://img.example/profile.jpg');
  });

  test('normal content search stays caption-first', () {
    final dto = ContentSearchResultDto.fromJson(
      _baseContentJson(
        caption: 'Hello world\nSecond line',
        mediaUrls: ['https://img.example/content.jpg'],
      ),
    );

    final entity = dto.toDomain();
    expect(entity.title, 'Hello world');
    expect(entity.thumbnailUrl, 'https://img.example/content.jpg');
  });

  test('missing resource_projection content stays on generic post fallback', () {
    final dto = ContentSearchResultDto.fromJson(
      _baseContentJson(caption: '', mediaUrls: const []),
    );

    final entity = dto.toDomain();
    expect(entity.title, 'Content');
    expect(entity.thumbnailUrl, isNull);
  });
}
