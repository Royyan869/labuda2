import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/commerce/catalog/auction/data/dto/auction_dto.dart';
import 'package:labuda/domains/commerce/catalog/auction/data/mappers/auction_mapper.dart';

Map<String, dynamic> _baseAuctionPayload({
  required Map<String, dynamic> sellerIdentity,
  required Map<String, dynamic> legacySeller,
}) {
  return {
    'id': 'auction-1',
    'seller_id': 'seller-1',
    'product_id': 'product-1',
    'title': 'Auction title',
    'description': 'Auction description',
    'images': const ['images/auction/item.jpg'],
    'category': null,
    'condition': null,
    'start_price': 100000,
    'bid_increment': 5000,
    'buy_now_price': null,
    'current_highest_bid': 100000,
    'highest_bidder_id': null,
    'total_bids': 0,
    'minimum_bid': 100000,
    'start_at': '2026-07-26T12:20:13+07:00',
    'end_at': '2026-07-31T12:20:13+07:00',
    'time_remaining_seconds': 0,
    'status': 'active',
    'auto_extend': false,
    'auto_extend_minutes': 10,
    'auto_extend_count': 0,
    'remaining_extensions': 3,
    'views_count': 0,
    'watchers_count': 0,
    'can_bid': true,
    'can_buy_now': false,
    'created_at': '2026-07-26T12:20:13+07:00',
    'updated_at': '2026-07-26T12:20:13+07:00',
    'seller_identity': sellerIdentity,
    'auction': {
      'id': 'auction-1',
      'title': 'Auction title',
      'thumbnail_url': null,
      'current_bid': null,
      'buy_now_price': null,
      'end_at': '2026-07-31T12:20:13+07:00',
      'lifecycle': 'active',
      'seller': legacySeller,
    },
  };
}

void main() {
  test('Auction detail canonical seller_identity survives DTO and mapper', () {
    final dto = AuctionDto.fromJson(
      _baseAuctionPayload(
        sellerIdentity: {
          'store_name': 'Qiqi Store',
          'store_image_url': 'images/stores/store.jpg',
          'username': 'qiqijho',
          'avatar_url': 'images/avatars/user.jpg',
          'public_origin_line': 'Magelang, Jawa Tengah',
        },
        legacySeller: {
          'user': {
            'id': 'seller-1',
            'username': 'legacy_user',
            'avatar_url': 'images/avatars/legacy-user.jpg',
            'lifecycle': 'active',
          },
          'farm_name': 'Legacy Store',
          'avatar_url': 'images/stores/legacy-store.jpg',
          'lifecycle': 'active',
        },
      ),
    );
    final entity = AuctionMapper.toEntity(dto);

    expect(dto.sellerIdentity, isNotNull);
    expect(dto.sellerIdentity!.storeName, 'Qiqi Store');
    expect(dto.sellerIdentity!.storeImageUrl, 'images/stores/store.jpg');
    expect(dto.sellerIdentity!.username, 'qiqijho');
    expect(dto.sellerIdentity!.avatarUrl, 'images/avatars/user.jpg');
    expect(dto.sellerIdentity!.publicOriginLine, 'Magelang, Jawa Tengah');
    expect(dto.sellerIdentity!.storeName, isNot('Legacy Store'));
    expect(
      dto.sellerIdentity!.storeImageUrl,
      isNot('images/stores/legacy-store.jpg'),
    );
    expect(
      dto.sellerIdentity!.avatarUrl,
      isNot('images/avatars/legacy-user.jpg'),
    );

    expect(entity.sellerIdentity, isNotNull);
    expect(entity.sellerIdentity!.storeName, 'Qiqi Store');
    expect(
      entity.sellerIdentity!.normalizedStoreImageUrl,
      'images/stores/store.jpg',
    );
    expect(
      entity.sellerIdentity!.normalizedAvatarUrl,
      'images/avatars/user.jpg',
    );
    expect(entity.sellerIdentity!.username, 'qiqijho');
    expect(entity.sellerIdentity!.handle, '@qiqijho');
    expect(entity.sellerIdentity!.publicOriginLine, 'Magelang, Jawa Tengah');
  });

  test('Auction detail canonical seller_identity keeps null visuals null', () {
    final dto = AuctionDto.fromJson(
      _baseAuctionPayload(
        sellerIdentity: {
          'store_name': 'Qiqi Store',
          'store_image_url': null,
          'username': 'qiqijho',
          'avatar_url': null,
          'public_origin_line': 'Magelang, Jawa Tengah',
        },
        legacySeller: {
          'user': {
            'id': 'seller-1',
            'username': 'legacy_user',
            'avatar_url': 'images/avatars/legacy-user.jpg',
            'lifecycle': 'active',
          },
          'farm_name': 'Legacy Store',
          'avatar_url': 'images/stores/legacy-store.jpg',
          'lifecycle': 'active',
        },
      ),
    );
    final entity = AuctionMapper.toEntity(dto);

    expect(dto.sellerIdentity, isNotNull);
    expect(dto.sellerIdentity!.storeName, 'Qiqi Store');
    expect(dto.sellerIdentity!.storeImageUrl, isNull);
    expect(dto.sellerIdentity!.avatarUrl, isNull);
    expect(dto.sellerIdentity!.publicOriginLine, 'Magelang, Jawa Tengah');
    expect(entity.sellerIdentity, isNotNull);
    expect(entity.sellerIdentity!.storeName, 'Qiqi Store');
    expect(entity.sellerIdentity!.normalizedStoreImageUrl, isNull);
    expect(entity.sellerIdentity!.normalizedAvatarUrl, isNull);
    expect(dto.sellerCard?.farmName, 'Legacy Store');
    expect(dto.sellerCard?.user.avatarUrl, 'images/avatars/legacy-user.jpg');
    expect(entity.sellerIdentity!.publicOriginLine, 'Magelang, Jawa Tengah');
  });
}
