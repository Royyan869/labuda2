import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/data/dto/for_sale_dto.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/data/mappers/for_sale_dto_mapper.dart';

Map<String, dynamic> _baseListingJson({
  required String saleId,
  required String sellerId,
  required String flatSellerUsername,
  required String flatSellerFarmName,
  required String flatSellerAvatarUrl,
  required Map<String, dynamic> fixedPriceSale,
}) {
  return <String, dynamic>{
    'id': saleId,
    'seller_id': sellerId,
    'title': 'Showa Koi 30cm',
    'description': 'Premium showa',
    'media_urls': <String>[],
    'listing_type': 'fixed_price',
    'price': 1500000,
    'quantity': 1,
    'negotiation_enabled': false,
    'visibility': 'public',
    'status': 'active',
    'seller_username': flatSellerUsername,
    'seller_farm_name': flatSellerFarmName,
    'seller_avatar_url': flatSellerAvatarUrl,
    'fixed_price_sale': fixedPriceSale,
    'created_at': '2026-07-24T00:00:00.000Z',
    'updated_at': '2026-07-24T00:00:00.000Z',
  };
}

void main() {
  test('nested fixed-price seller card wins over flat seller fields', () {
    final dto = ForSaleResponseDto.fromJson(
      _baseListingJson(
        saleId: 'listing-1',
        sellerId: 'seller-1',
        flatSellerUsername: 'flat_should_not_win',
        flatSellerFarmName: 'Flat Farm',
        flatSellerAvatarUrl: 'https://example.com/flat-avatar.jpg',
        fixedPriceSale: <String, dynamic>{
          'seller': <String, dynamic>{
            'user': <String, dynamic>{
              'username': 'user_deadbeef',
              'avatar_url': 'https://example.com/nested-avatar.jpg',
              'lifecycle': 'active',
            },
            'farm_name': 'Nested Farm',
            'avatar_url': 'https://example.com/nested-avatar.jpg',
            'lifecycle': 'active',
            'tier': 'pro',
          },
        },
      ),
    );

    expect(dto.sellerUsername, 'user_deadbeef');
    expect(dto.sellerFarmName, 'Nested Farm');
    expect(dto.sellerAvatarUrl, 'https://example.com/nested-avatar.jpg');

    final entity = ForSaleDtoMapper.toEntity(dto);
    expect(entity.sellerUsername, 'user_deadbeef');
    expect(entity.sellerFarmName, 'Nested Farm');
    expect(entity.sellerAvatar, 'https://example.com/nested-avatar.jpg');
  });

  test('listing response parses public origin and shipping summaries', () {
    final json = _baseListingJson(
      saleId: 'listing-3',
      sellerId: 'seller-3',
      flatSellerUsername: 'seller_user',
      flatSellerFarmName: 'Acme Farm',
      flatSellerAvatarUrl: 'https://example.com/avatar.jpg',
      fixedPriceSale: <String, dynamic>{
        'seller': <String, dynamic>{
          'user': <String, dynamic>{
            'username': 'seller_user',
            'avatar_url': 'https://example.com/avatar.jpg',
            'lifecycle': 'active',
          },
          'farm_name': 'Acme Farm',
          'avatar_url': 'https://example.com/avatar.jpg',
          'lifecycle': 'active',
          'tier': 'pro',
        },
      },
    );
    json['seller_identity'] = <String, dynamic>{
      'store_name': 'Acme Farm',
      'store_image_url': 'https://example.com/store.jpg',
      'username': 'seller_user',
      'avatar_url': 'https://example.com/avatar.jpg',
      'public_origin_line': 'Magelang, Jawa Tengah',
    };
    json['origin'] = 'Kecamatan, Kota, Provinsi';
    json['shipping_options'] = <Map<String, dynamic>>[
      <String, dynamic>{
        'id': 'ship-1',
        'name': 'JNE',
        'transport_type': 'express',
      },
    ];

    final dto = ForSaleResponseDto.fromJson(json);

    expect(dto.sellerIdentity, isNotNull);
    expect(dto.sellerIdentity!.publicOriginLine, 'Magelang, Jawa Tengah');
    expect(dto.origin, 'Kecamatan, Kota, Provinsi');
    expect(dto.shippingSetups, hasLength(1));
    expect(dto.shippingSetups.first.id, 'ship-1');
    expect(dto.shippingSetups.first.name, 'JNE');
    expect(dto.shippingSetups.first.transportType, 'express');
    expect(dto.shippingSetupIds, ['ship-1']);

    final entity = ForSaleDtoMapper.toEntity(dto);
    expect(entity.sellerIdentity, isNotNull);
    expect(entity.sellerIdentity!.publicOriginLine, 'Magelang, Jawa Tengah');
    expect(entity.origin, 'Kecamatan, Kota, Provinsi');
    expect(entity.shippingSetups, hasLength(1));
    expect(entity.shippingSetups.first.id, 'ship-1');
    expect(entity.shippingSetups.first.name, 'JNE');
    expect(entity.shippingSetups.first.transportType, 'express');
  });

  test(
    'blank nested seller username does not fall back to flat seller fields',
    () {
      final dto = ForSaleResponseDto.fromJson(
        _baseListingJson(
          saleId: 'listing-2',
          sellerId: 'seller-2',
          flatSellerUsername: 'flat_should_not_win',
          flatSellerFarmName: 'Flat Farm',
          flatSellerAvatarUrl: 'https://example.com/flat-avatar.jpg',
          fixedPriceSale: <String, dynamic>{
            'seller': <String, dynamic>{
              'user': <String, dynamic>{
                'username': '',
                'avatar_url': '',
                'lifecycle': 'unavailable',
              },
              'farm_name': 'Nested Farm',
              'avatar_url': '',
              'lifecycle': 'active',
            },
          },
        ),
      );

      expect(dto.sellerUsername, isNull);
      expect(dto.sellerFarmName, 'Nested Farm');
      expect(dto.sellerAvatarUrl, isNull);

      final entity = ForSaleDtoMapper.toEntity(dto);
      expect(entity.sellerUsername, isNull);
      expect(entity.sellerFarmName, 'Nested Farm');
      expect(entity.sellerAvatar, isNull);
    },
  );
}
