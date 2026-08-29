import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/social/content/domain/entities/content_resource_projection.dart';

Map<String, dynamic> _profileProjection({
  String state = 'LIVE',
  String resourceId = 'profile-1',
  String username = 'alice',
}) {
  final json = <String, dynamic>{
    'state': state,
    'resource_type': 'profile',
    'resource_id': resourceId,
  };
  if (state == 'LIVE') {
    json['profile'] = <String, dynamic>{
      'username': username,
      'avatar_url': 'https://example.com/profile.jpg',
      'lifecycle': 'active',
    };
  }
  return json;
}

Map<String, dynamic> _contentProjection({
  String state = 'LIVE',
  String resourceId = 'content-1',
  String caption = 'Hello content',
}) {
  final json = <String, dynamic>{
    'state': state,
    'resource_type': 'content',
    'resource_id': resourceId,
  };
  if (state == 'LIVE') {
    json['content'] = <String, dynamic>{
      'caption': caption,
      'media': <Map<String, dynamic>>[],
      'lifecycle': 'active',
      'created_at': '2026-08-10T00:00:00.000Z',
      'author': <String, dynamic>{
        'id': 'author-1',
        'username': 'alice',
        'avatar_url': null,
        'lifecycle': 'active',
      },
    };
  }
  return json;
}

Map<String, dynamic> _fixedPriceSaleProjection({
  String state = 'LIVE',
  String resourceId = 'sale-1',
  String title = 'Produk Dijual',
}) {
  final json = <String, dynamic>{
    'state': state,
    'resource_type': 'fixed_price_sale',
    'resource_id': resourceId,
  };
  if (state == 'LIVE') {
    json['fixed_price_sale'] = <String, dynamic>{
      'title': title,
      'media': <Map<String, dynamic>>[],
      'price': 125000,
      'status': 'active',
      'quantity_available': 1,
      'can_interact': true,
      'seller': <String, dynamic>{
        'user': <String, dynamic>{
          'id': 'seller-1',
          'username': 'seller',
          'avatar_url': null,
          'lifecycle': 'active',
        },
        'farm_name': 'Koi Farm',
        'avatar_url': null,
        'lifecycle': 'active',
      },
    };
  }
  return json;
}

Map<String, dynamic> _auctionProjection({
  String state = 'LIVE',
  String resourceId = 'auction-1',
  String title = 'Lelang Koi',
}) {
  final json = <String, dynamic>{
    'state': state,
    'resource_type': 'auction',
    'resource_id': resourceId,
  };
  if (state == 'LIVE') {
    json['auction'] = <String, dynamic>{
      'title': title,
      'media': <Map<String, dynamic>>[],
      'end_at': '2026-08-11T00:00:00.000Z',
      'lifecycle': 'active',
      'can_interact': true,
      'seller': <String, dynamic>{
        'user': <String, dynamic>{
          'id': 'seller-1',
          'username': 'seller',
          'avatar_url': null,
          'lifecycle': 'active',
        },
        'farm_name': 'Koi Farm',
        'avatar_url': null,
        'lifecycle': 'active',
      },
    };
  }
  return json;
}

void main() {
  test('supports LIVE and TOMBSTONE projections for all four resource types', () {
    final cases = <({
      Map<String, dynamic> json,
      ContentResourceProjectionType type,
      String path,
      String title,
    })>[
      (json: _profileProjection(), type: ContentResourceProjectionType.profile, path: '/user/profile-1', title: '@alice'),
      (json: _contentProjection(), type: ContentResourceProjectionType.content, path: '/content/content-1', title: 'Hello content'),
      (json: _fixedPriceSaleProjection(), type: ContentResourceProjectionType.fixedPriceSale, path: '/for-sale/sale-1', title: 'Produk Dijual'),
      (json: _auctionProjection(), type: ContentResourceProjectionType.auction, path: '/auction/auction-1', title: 'Lelang Koi'),
    ];

    for (final tc in cases) {
      final projection = ContentResourceProjection.fromJson(tc.json);
      expect(projection.state, ContentResourceProjectionState.live);
      expect(projection.resourceType, tc.type);
      expect(projection.isLive, isTrue);
      expect(projection.isTombstone, isFalse);
      expect(projection.canonicalPath, tc.path);
      expect(projection.titleText, tc.title);
      expect(projection.typeLabel, tc.type.displayLabel);
    }

    final tombstoneCases = <({
      Map<String, dynamic> json,
      ContentResourceProjectionType type,
      String path,
    })>[
      (json: _profileProjection(state: 'TOMBSTONE'), type: ContentResourceProjectionType.profile, path: '/user/profile-1'),
      (json: _contentProjection(state: 'TOMBSTONE'), type: ContentResourceProjectionType.content, path: '/content/content-1'),
      (json: _fixedPriceSaleProjection(state: 'TOMBSTONE'), type: ContentResourceProjectionType.fixedPriceSale, path: '/for-sale/sale-1'),
      (json: _auctionProjection(state: 'TOMBSTONE'), type: ContentResourceProjectionType.auction, path: '/auction/auction-1'),
    ];

    for (final tc in tombstoneCases) {
      final projection = ContentResourceProjection.fromJson(tc.json);
      expect(projection.state, ContentResourceProjectionState.tombstone);
      expect(projection.resourceType, tc.type);
      expect(projection.isLive, isFalse);
      expect(projection.isTombstone, isTrue);
      expect(projection.canonicalPath, tc.path);
      expect(projection.titleText, contains('tidak tersedia'));
      expect(projection.imageUrl, isNull);
      expect(projection.valueText, isNull);
    }
  });

  test('unknown state, type, and malformed payloads fail closed', () {
    expect(
      () => ContentResourceProjection.fromJson(
        <String, dynamic>{
          'state': 'BROKEN',
          'resource_type': 'profile',
          'resource_id': 'profile-1',
          'profile': <String, dynamic>{
            'username': 'alice',
            'lifecycle': 'active',
          },
        },
      ),
      throwsFormatException,
    );

    expect(
      () => ContentResourceProjection.fromJson(
        <String, dynamic>{
          'state': 'LIVE',
          'resource_type': 'broken',
          'resource_id': 'profile-1',
          'profile': <String, dynamic>{
            'username': 'alice',
            'lifecycle': 'active',
          },
        },
      ),
      throwsFormatException,
    );

    expect(
      () => ContentResourceProjection.fromJson(
        <String, dynamic>{
          'state': 'LIVE',
          'resource_type': 'profile',
          'resource_id': 'profile-1',
        },
      ),
      throwsFormatException,
    );

    expect(
      () => ContentResourceProjection.fromJson(
        <String, dynamic>{
          'state': 'TOMBSTONE',
          'resource_type': 'auction',
          'resource_id': 'auction-1',
          'auction': <String, dynamic>{
            'title': 'Lelang Koi',
          },
        },
      ),
      throwsFormatException,
    );
  });

  test('source has no FALLBACK_ALLOWED or legacy->canonical conversion', () {
    final source = File(
      'lib/domains/social/content/domain/entities/content_resource_projection.dart',
    ).readAsStringSync();

    expect(source.contains('FALLBACK_ALLOWED'), isFalse);
    expect(source.contains('legacy->canonical'), isFalse);
  });
}
