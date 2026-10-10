import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/shared/domain/entities/resource_projection.dart';

/// MOBILE CANONICAL PROJECTION RATCHET.
///
/// The mobile client mirrors one envelope for every surface:
/// `commerceshared.ResourceProjection` on the backend is parsed by exactly one
/// Dart type (`lib/shared/domain/entities/resource_projection.dart`). These
/// tests pin the canonical wire and the facts that keep the two former
/// per-surface parsers dead.

Map<String, dynamic> _liveViewerCapabilities({
  bool canInteract = false,
}) => <String, dynamic>{
  'can_view': true,
  'can_interact': canInteract,
  'blocked_by_tombstone': false,
};

Map<String, dynamic> _tombstoneViewerCapabilities() => <String, dynamic>{
  'can_view': false,
  'can_interact': false,
  'blocked_by_tombstone': true,
};

Map<String, dynamic> _profileProjection({
  String state = 'LIVE',
  String resourceId = 'profile-1',
  String username = 'alice',
}) {
  if (state == 'TOMBSTONE') {
    return <String, dynamic>{
      'state': state,
      'resource_type': 'profile',
      'resource_id': resourceId,
      'viewer_capabilities': _tombstoneViewerCapabilities(),
    };
  }
  return <String, dynamic>{
    'state': state,
    'resource_type': 'profile',
    'resource_id': resourceId,
    'canonical_url': '/user/$resourceId',
    'viewer_capabilities': _liveViewerCapabilities(),
    'profile': <String, dynamic>{
      'username': username,
      'avatar_url': 'https://example.com/profile.jpg',
      'lifecycle': 'active',
    },
  };
}

Map<String, dynamic> _contentProjection({
  String state = 'LIVE',
  String resourceId = 'content-1',
  String caption = 'Hello content',
}) {
  if (state == 'TOMBSTONE') {
    return <String, dynamic>{
      'state': state,
      'resource_type': 'content',
      'resource_id': resourceId,
      'viewer_capabilities': _tombstoneViewerCapabilities(),
    };
  }
  return <String, dynamic>{
    'state': state,
    'resource_type': 'content',
    'resource_id': resourceId,
    'canonical_url': '/content/$resourceId',
    'viewer_capabilities': _liveViewerCapabilities(),
    'content': <String, dynamic>{
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
    },
  };
}

Map<String, dynamic> _fixedPriceSaleProjection({
  String state = 'LIVE',
  String resourceId = 'sale-1',
  String title = 'Produk Dijual',
}) {
  if (state == 'TOMBSTONE') {
    return <String, dynamic>{
      'state': state,
      'resource_type': 'for_sale',
      'resource_id': resourceId,
      'viewer_capabilities': _tombstoneViewerCapabilities(),
    };
  }
  return <String, dynamic>{
    'state': state,
    'resource_type': 'for_sale',
    'resource_id': resourceId,
    'canonical_url': '/for-sale/$resourceId',
    'viewer_capabilities': _liveViewerCapabilities(canInteract: true),
    'for_sale': <String, dynamic>{
      'title': title,
      // Money is an object {amount, currency} — the scalar price is dead.
      'media': <Map<String, dynamic>>[
        {'url': 'https://example.com/sale.jpg', 'kind': 'image'},
      ],
      'price': {'amount': 125000, 'currency': 'IDR'},
      'status': 'active',
      'quantity_available': 1,
      'negotiation_enabled': true,
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
    },
  };
}

Map<String, dynamic> _auctionProjection({
  String state = 'LIVE',
  String resourceId = 'auction-1',
  String title = 'Lelang Koi',
}) {
  if (state == 'TOMBSTONE') {
    return <String, dynamic>{
      'state': state,
      'resource_type': 'auction',
      'resource_id': resourceId,
      'viewer_capabilities': _tombstoneViewerCapabilities(),
    };
  }
  return <String, dynamic>{
    'state': state,
    'resource_type': 'auction',
    'resource_id': resourceId,
    'canonical_url': '/auction/$resourceId',
    'viewer_capabilities': _liveViewerCapabilities(canInteract: true),
    'auction': <String, dynamic>{
      'title': title,
      'media': <Map<String, dynamic>>[],
      'thumbnail_url': 'https://example.com/auction.jpg',
      'end_at': '2026-08-11T00:00:00.000Z',
      'lifecycle': 'active',
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
    },
  };
}

void main() {
  test('supports LIVE and TOMBSTONE projections for all four resource types', () {
    final cases =
        <
          ({
            Map<String, dynamic> json,
            ResourceProjectionType type,
            String path,
            String title,
          })
        >[
          (
            json: _profileProjection(),
            type: ResourceProjectionType.profile,
            path: '/user/profile-1',
            title: '@alice',
          ),
          (
            json: _contentProjection(),
            type: ResourceProjectionType.content,
            path: '/content/content-1',
            title: 'Hello content',
          ),
          (
            json: _fixedPriceSaleProjection(),
            type: ResourceProjectionType.fixedPriceSale,
            path: '/for-sale/sale-1',
            title: 'Produk Dijual',
          ),
          (
            json: _auctionProjection(),
            type: ResourceProjectionType.auction,
            path: '/auction/auction-1',
            title: 'Lelang Koi',
          ),
        ];

    for (final tc in cases) {
      final projection = ResourceProjection.fromJson(tc.json);
      expect(projection.state, ResourceProjectionState.live);
      expect(projection.resourceType, tc.type);
      expect(projection.isLive, isTrue);
      expect(projection.isTombstone, isFalse);
      expect(projection.canonicalPath, tc.path);
      expect(projection.canonicalUrl, tc.json['canonical_url']);
      expect(projection.titleText, tc.title);
      expect(projection.typeLabel, tc.type.displayLabel);
      expect(projection.resourceId, isNotEmpty);
      expect(projection.payload, isNotNull);
      expect(projection.payload!.resourceType, tc.type);
    }

    // The canonical money object is carried by the envelope on LIVE.
    final sale = ResourceProjection.fromJson(_fixedPriceSaleProjection());
    final salePayload = sale.payload! as ForSaleLivePayload;
    expect(salePayload.price.amount, 125000);
    expect(salePayload.price.currency, 'IDR');
    // Discovery renders the money; the chat card owns the opposite policy.
    expect(sale.primaryImageUrl, 'https://example.com/sale.jpg');

    final tombstoneCases =
        <
          ({
            Map<String, dynamic> json,
            ResourceProjectionType type,
            String path,
          })
        >[
          (
            json: _profileProjection(state: 'TOMBSTONE'),
            type: ResourceProjectionType.profile,
            path: '/user/profile-1',
          ),
          (
            json: _contentProjection(state: 'TOMBSTONE'),
            type: ResourceProjectionType.content,
            path: '/content/content-1',
          ),
          (
            json: _fixedPriceSaleProjection(state: 'TOMBSTONE'),
            type: ResourceProjectionType.fixedPriceSale,
            path: '/for-sale/sale-1',
          ),
          (
            json: _auctionProjection(state: 'TOMBSTONE'),
            type: ResourceProjectionType.auction,
            path: '/auction/auction-1',
          ),
        ];

    for (final tc in tombstoneCases) {
      final projection = ResourceProjection.fromJson(tc.json);
      expect(projection.state, ResourceProjectionState.tombstone);
      expect(projection.resourceType, tc.type);
      expect(projection.isLive, isFalse);
      expect(projection.isTombstone, isTrue);
      expect(projection.canonicalPath, tc.path);
      expect(projection.titleText, contains('tidak tersedia'));
      expect(projection.primaryImageUrl, isNull);
      expect(projection.payload, isNull);
      expect(projection.canonicalUrl, isNull);
      // Canonical contract: identity survives death — dedup/audit still work.
      expect(projection.resourceId, isNotEmpty);
    }
  });

  test('unknown state, type, and malformed payloads fail closed', () {
    expect(
      () => ResourceProjection.fromJson(
        _profileProjection()..['state'] = 'BROKEN',
      ),
      throwsFormatException,
    );

    expect(
      () => ResourceProjection.fromJson(
        _profileProjection()..['resource_type'] = 'broken',
      ),
      throwsFormatException,
    );

    // LIVE without viewer_capabilities is not a canonical envelope.
    expect(
      () => ResourceProjection.fromJson(
        _profileProjection()..remove('viewer_capabilities'),
      ),
      throwsFormatException,
    );

    // Missing payload for a LIVE state.
    expect(
      () => ResourceProjection.fromJson(
        _profileProjection()..remove('profile'),
      ),
      throwsFormatException,
    );

    // TOMBSTONE carrying a payload.
    expect(
      () => ResourceProjection.fromJson(
        _auctionProjection(state: 'TOMBSTONE')..['auction'] = {'title': 'x'},
      ),
      throwsFormatException,
    );

    // TOMBSTONE without identity.
    expect(
      () => ResourceProjection.fromJson(
        _fixedPriceSaleProjection(state: 'TOMBSTONE')..remove('resource_id'),
      ),
      throwsFormatException,
    );

    // Legacy payload shape (payload-level can_interact + scalar price) is dead.
    expect(
      () => ResourceProjection.fromJson(<String, dynamic>{
        'state': 'LIVE',
        'resource_type': 'for_sale',
        'resource_id': 'sale-legacy-1',
        'canonical_url': '/for-sale/sale-legacy-1',
        'for_sale': <String, dynamic>{
          'title': 'Legacy',
          'media': <Map<String, dynamic>>[],
          'price': 125000,
          'status': 'active',
          'quantity_available': 1,
          'can_interact': true,
          'seller': <String, dynamic>{
            'user': <String, dynamic>{
              'id': 'seller-1',
              'username': 'seller',
            },
          },
        },
      }),
      throwsFormatException,
    );
  });

  test('the two legacy per-surface parsers no longer exist', () {
    // One wire, one parser: a second Dart envelope must not be resurrected.
    expect(
      File(
        'lib/domains/chat/chat/domain/entities/chat_resource_projection.dart',
      ).existsSync(),
      isFalse,
    );
    expect(
      File(
        'lib/domains/social/content/domain/entities/content_resource_projection.dart',
      ).existsSync(),
      isFalse,
    );
  });

  test('the canonical entity has no fallback and no payload-level policy', () {
    const path = 'lib/shared/domain/entities/resource_projection.dart';
    final source = File(path).readAsStringSync();

    expect(source.contains('FALLBACK_ALLOWED'), isFalse);
    expect(source.contains('legacy->canonical'), isFalse);

    // Capabilities live on the envelope only. A payload-level
    // `can_interact` (or the dead singular `image_url`) would serialize into
    // the payload sub-map — structural proof, not a source scan.
    for (final json in [
      ResourceProjection.fromJson(_fixedPriceSaleProjection()).toJson(),
      ResourceProjection.fromJson(_auctionProjection()).toJson(),
      ResourceProjection.fromJson(_profileProjection()).toJson(),
      ResourceProjection.fromJson(_contentProjection()).toJson(),
    ]) {
      final payloadKey = json['resource_type'] as String;
      final payload = json[payloadKey] as Map<String, dynamic>;
      expect(payload.containsKey('can_interact'), isFalse);
      expect(payload.containsKey('image_url'), isFalse);
      expect(json['viewer_capabilities'], isA<Map<String, dynamic>>());
    }

    // The money is a canonical object, never a scalar.
    final sale =
        ResourceProjection.fromJson(_fixedPriceSaleProjection()).toJson()['for_sale']
            as Map<String, dynamic>;
    expect(sale['price'], isA<Map<String, dynamic>>());
  });
}
