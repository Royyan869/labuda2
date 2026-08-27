import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/domains/chat/chat/domain/entities/chat_resource_projection.dart';
import 'package:labuda/domains/chat/chat/presentation/widgets/chat_resource_projection_card.dart';
import 'package:labuda/domains/commerce/catalog/shared/presentation/widgets/commerce_marketplace_primitives.dart';

Map<String, dynamic> _profileLiveJson({
  String resourceId = 'user-resource-1',
  String username = 'alice',
  String? avatarUrl = 'https://cdn.example.test/alice.png',
  String? storeName = 'Toko Alice',
  bool canInteract = false,
  String lifecycle = 'active',
}) {
  final profile = <String, dynamic>{
    'username': username,
    'is_seller': true,
    'lifecycle': lifecycle,
  };
  if (avatarUrl != null) {
    profile['avatar_url'] = avatarUrl;
  }
  if (storeName != null) {
    profile['store_name'] = storeName;
  }

  return {
    'state': 'LIVE',
    'resource_type': 'profile',
    'resource_id': resourceId,
    'canonical_url': '/user/$resourceId',
    'viewer_capabilities': {
      'can_view': true,
      'can_interact': canInteract,
      'blocked_by_tombstone': false,
    },
    'profile': profile,
  };
}

Map<String, dynamic> _profileTombstoneJson() {
  return {
    'state': 'TOMBSTONE',
    'resource_type': 'profile',
    'viewer_capabilities': {
      'can_view': false,
      'can_interact': false,
      'blocked_by_tombstone': true,
    },
  };
}

Map<String, dynamic> _contentLiveJson({Map<String, dynamic>? nestedResource}) {
  final content = <String, dynamic>{
    'caption': 'Konten utama',
    'media': [
      {'url': 'https://cdn.example.test/content-1.jpg', 'kind': 'image'},
    ],
    'lifecycle': 'active',
    'created_at': '2026-08-08T10:11:12Z',
    'author': {
      'id': 'author-1',
      'username': 'author_user',
      'avatar_url': 'https://cdn.example.test/author.png',
      'lifecycle': 'active',
    },
  };
  if (nestedResource != null) {
    content['nested_resource'] = nestedResource;
  }

  return {
    'state': 'LIVE',
    'resource_type': 'content',
    'resource_id': 'content-resource-1',
    'canonical_url': '/content/content-resource-1',
    'viewer_capabilities': {
      'can_view': true,
      'can_interact': false,
      'blocked_by_tombstone': false,
    },
    'content': content,
  };
}

Map<String, dynamic> _contentTombstoneJson() {
  return {
    'state': 'TOMBSTONE',
    'resource_type': 'content',
    'viewer_capabilities': {
      'can_view': false,
      'can_interact': false,
      'blocked_by_tombstone': true,
    },
  };
}

Map<String, dynamic> _fpsLiveJson({
  String resourceId = 'fps-resource-1',
  required bool canBuy,
  bool canChat = true,
  bool canNegotiate = false,
  bool canManage = true,
  String status = 'available',
  int quantityAvailable = 4,
}) {
  return {
    'state': 'LIVE',
    'resource_type': 'for_sale',
    'resource_id': resourceId,
    'canonical_url': '/for-sale/$resourceId',
    'viewer_capabilities': {
      'can_view': true,
      'can_interact': canBuy || canNegotiate,
      'blocked_by_tombstone': false,
    },
    'commerce_actions': {
      'role': 'buyer',
      'can_chat': canChat,
      'can_negotiate': canNegotiate,
      'can_buy': canBuy,
      'can_bid': false,
      'can_manage': canManage,
    },
    'for_sale': {
      'title': 'Koi Premium',
      'image_url': 'https://cdn.example.test/fps.jpg',
      'price': {'amount': 1250000, 'currency': 'IDR'},
      'status': status,
      'seller': {
        'user': {
          'id': 'seller-1',
          'username': 'seller_user',
          'avatar_url': 'https://cdn.example.test/seller.png',
          'lifecycle': 'active',
        },
        'farm_name': 'Tambak Alice',
        'avatar_url': 'https://cdn.example.test/seller.png',
        'lifecycle': 'active',
      },
      'quantity_available': quantityAvailable,
    },
  };
}

Map<String, dynamic> _fpsTombstoneJson() {
  return {
    'state': 'TOMBSTONE',
    'resource_type': 'for_sale',
    'viewer_capabilities': {
      'can_view': false,
      'can_interact': false,
      'blocked_by_tombstone': true,
    },
  };
}

Map<String, dynamic> _auctionLiveJson({
  String resourceId = 'auction-resource-1',
  required bool canBid,
  bool canBuy = false,
  bool canChat = true,
  bool canManage = true,
  String endAt = '2026-08-10T12:34:56Z',
}) {
  return {
    'state': 'LIVE',
    'resource_type': 'auction',
    'resource_id': resourceId,
    'canonical_url': '/auction/$resourceId',
    'viewer_capabilities': {
      'can_view': true,
      'can_interact': canBid || canBuy,
      'blocked_by_tombstone': false,
    },
    'commerce_actions': {
      'role': 'buyer',
      'can_chat': canChat,
      'can_negotiate': false,
      'can_buy': canBuy,
      'can_bid': canBid,
      'can_manage': canManage,
    },
    'auction': {
      'title': 'Auction Premium',
      'thumbnail_url': 'https://cdn.example.test/auction.jpg',
      'current_bid': 1450000,
      'buy_now_price': 1750000,
      'end_at': endAt,
      'lifecycle': 'active',
      'seller': {
        'user': {
          'id': 'seller-2',
          'username': 'auction_seller',
          'avatar_url': 'https://cdn.example.test/auction-seller.png',
          'lifecycle': 'active',
        },
        'farm_name': 'Auction Farm',
        'avatar_url': 'https://cdn.example.test/auction-seller.png',
        'lifecycle': 'active',
      },
    },
  };
}

Map<String, dynamic> _auctionTombstoneJson() {
  return {
    'state': 'TOMBSTONE',
    'resource_type': 'auction',
    'viewer_capabilities': {
      'can_view': false,
      'can_interact': false,
      'blocked_by_tombstone': true,
    },
  };
}

ChatResourceProjection _parseProjection(Map<String, dynamic> json) {
  return ChatResourceProjection.fromJson(json);
}

Widget _projectionCard(ChatResourceProjection projection) {
  return MaterialApp(
    home: Scaffold(
      body: SingleChildScrollView(
        child: ChatResourceProjectionCard(resourceProjection: projection),
      ),
    ),
  );
}

void main() {
  group('strict union negative cases', () {
    test('LIVE missing resource_id is rejected', () {
      expect(
        () => _parseProjection({..._profileLiveJson(), 'resource_id': ''}),
        throwsFormatException,
      );
    });

    test('LIVE missing required payload is rejected', () {
      expect(
        () => _parseProjection({..._profileLiveJson(), 'profile': null}),
        throwsFormatException,
      );
    });

    test('Profile projection with Content payload is rejected', () {
      expect(
        () => _parseProjection({
          ..._profileLiveJson(),
          'content': {
            'caption': 'wrong payload',
            'media': [],
            'lifecycle': 'active',
            'created_at': '2026-08-08T10:11:12Z',
            'author': {'id': 'author-1', 'username': 'author_user'},
          },
        }),
        throwsFormatException,
      );
    });

    test('FPS LIVE missing commerce_actions is rejected', () {
      expect(
        () => _parseProjection({
          ..._fpsLiveJson(canBuy: true),
          'commerce_actions': null,
        }),
        throwsFormatException,
      );
    });

    test('Auction LIVE missing commerce_actions is rejected', () {
      expect(
        () => _parseProjection({
          ..._auctionLiveJson(canBid: true),
          'commerce_actions': null,
        }),
        throwsFormatException,
      );
    });

    test('multiple LIVE payloads present is rejected', () {
      expect(
        () => _parseProjection({
          ..._profileLiveJson(),
          'content': _contentLiveJson()['content'],
        }),
        throwsFormatException,
      );
    });

    test('TOMBSTONE containing resource_id is rejected', () {
      expect(
        () => _parseProjection({
          ..._profileTombstoneJson(),
          'resource_id': 'user-resource-1',
        }),
        throwsFormatException,
      );
    });

    test('TOMBSTONE containing canonical_url is rejected', () {
      expect(
        () => _parseProjection({
          ..._profileTombstoneJson(),
          'canonical_url': '/user/user-resource-1',
        }),
        throwsFormatException,
      );
    });

    test('TOMBSTONE containing payload is rejected', () {
      expect(
        () => _parseProjection({
          ..._profileTombstoneJson(),
          'profile': _profileLiveJson()['profile'],
        }),
        throwsFormatException,
      );
    });

    test('TOMBSTONE containing commerce_actions is rejected', () {
      expect(
        () => _parseProjection({
          ..._profileTombstoneJson(),
          'commerce_actions': {
            'role': 'buyer',
            'can_chat': true,
            'can_negotiate': false,
            'can_buy': false,
            'can_bid': false,
            'can_manage': false,
          },
        }),
        throwsFormatException,
      );
    });

    test('unknown projection state is rejected', () {
      expect(
        () =>
            _parseProjection({..._profileLiveJson(), 'state': 'UNKNOWN_STATE'}),
        throwsFormatException,
      );
    });

    test('unknown resource type is rejected', () {
      expect(
        () =>
            _parseProjection({..._profileLiveJson(), 'resource_type': 'order'}),
        throwsFormatException,
      );
    });
  });

  group('round trip', () {
    test('profile LIVE round trips canonically', () {
      final json = _profileLiveJson();
      final projection = _parseProjection(json);

      expect(projection.toJson(), json);
      expect(projection.resourceType, ChatResourceType.profile);
      expect(projection.state, ChatResourceProjectionState.live);
    });

    test('content LIVE round trips canonically', () {
      final json = _contentLiveJson(
        nestedResource: {
          'resource_type': 'profile',
          'resource_id': 'profile-nested-1',
        },
      );
      final projection = _parseProjection(json);

      expect(projection.toJson(), json);
      expect(projection.resourceType, ChatResourceType.content);
      expect(projection.state, ChatResourceProjectionState.live);
    });

    test('fixed price sale LIVE round trips canonically', () {
      final json = _fpsLiveJson(canBuy: true, canNegotiate: false);
      final projection = _parseProjection(json);

      expect(projection.toJson(), json);
      expect(projection.resourceType, ChatResourceType.forSale);
      expect(projection.state, ChatResourceProjectionState.live);
    });

    test('auction LIVE round trips canonically', () {
      final json = _auctionLiveJson(canBid: true, canBuy: false);
      final projection = _parseProjection(json);

      expect(projection.toJson(), json);
      expect(projection.resourceType, ChatResourceType.auction);
      expect(projection.state, ChatResourceProjectionState.live);
    });
  });

  group('profile/content/fps/auction render authority', () {
    testWidgets('Profile LIVE renders backend identity and can navigate', (
      tester,
    ) async {
      final projection = _parseProjection(_profileLiveJson());
      final router = GoRouter(
        initialLocation: '/',
        routes: [
          GoRoute(
            path: '/',
            builder: (context, state) => Scaffold(
              body: SingleChildScrollView(
                child: ChatResourceProjectionCard(
                  resourceProjection: projection,
                ),
              ),
            ),
          ),
          GoRoute(
            path: '/user/user-resource-1',
            builder: (context, state) =>
                const Scaffold(body: Text('profile destination')),
          ),
        ],
      );

      await tester.pumpWidget(MaterialApp.router(routerConfig: router));
      await tester.pumpAndSettle();

      expect(find.text('@alice'), findsOneWidget);
      expect(find.text('Toko Alice'), findsOneWidget);
      expect(find.text('Profil'), findsWidgets);
      expect(find.text('LIVE'), findsWidgets);

      await tester.tap(find.byType(CommerceMarketplaceCardShell));
      await tester.pumpAndSettle();

      expect(find.text('profile destination'), findsOneWidget);
    });

    testWidgets('Profile TOMBSTONE is privacy-safe and cannot navigate', (
      tester,
    ) async {
      final projection = _parseProjection(_profileTombstoneJson());
      await tester.pumpWidget(_projectionCard(projection));

      final shell = tester.widget<CommerceMarketplaceCardShell>(
        find.byType(CommerceMarketplaceCardShell),
      );
      expect(shell.onTap, isNull);
      expect(find.text('Profil tidak tersedia'), findsOneWidget);
      expect(find.text('Tidak dapat ditampilkan'), findsOneWidget);
    });

    testWidgets('Content LIVE renders canonical payload and nested indicator', (
      tester,
    ) async {
      final projection = _parseProjection(
        _contentLiveJson(
          nestedResource: {
            'resource_type': 'profile',
            'resource_id': 'profile-nested-1',
          },
        ),
      );
      await tester.pumpWidget(_projectionCard(projection));

      expect(find.text('Konten utama'), findsOneWidget);
      expect(find.text('@author_user'), findsOneWidget);
      expect(projection.compactPreviewText, 'Konten utama');
      expect(
        projection.toJson(),
        _contentLiveJson(
          nestedResource: {
            'resource_type': 'profile',
            'resource_id': 'profile-nested-1',
          },
        ),
      );
    });

    testWidgets('Content TOMBSTONE is privacy-safe', (tester) async {
      final projection = _parseProjection(_contentTombstoneJson());
      await tester.pumpWidget(_projectionCard(projection));

      expect(find.text('Konten tidak tersedia'), findsOneWidget);
      expect(find.text('Tidak dapat ditampilkan'), findsOneWidget);
    });

    testWidgets('FPS backend capability flags drive the UI', (tester) async {
      final projection = _parseProjection(
        _fpsLiveJson(canBuy: true, canNegotiate: false, canChat: true),
      );
      await tester.pumpWidget(_projectionCard(projection));

      expect(find.text('Beli'), findsOneWidget);
      expect(find.text('Chat'), findsOneWidget);
      expect(find.text('Nego'), findsNothing);
      expect(find.text('Kelola'), findsOneWidget);
      expect(find.text('LIVE'), findsWidgets);
    });

    testWidgets('FPS TOMBSTONE cannot navigate', (tester) async {
      final projection = _parseProjection(_fpsTombstoneJson());
      await tester.pumpWidget(_projectionCard(projection));

      final shell = tester.widget<CommerceMarketplaceCardShell>(
        find.byType(CommerceMarketplaceCardShell),
      );
      expect(shell.onTap, isNull);
    });

    testWidgets('Auction backend capability flags drive the UI', (
      tester,
    ) async {
      final projection = _parseProjection(
        _auctionLiveJson(canBid: true, canBuy: false, canChat: true),
      );
      await tester.pumpWidget(_projectionCard(projection));

      expect(find.text('Bid'), findsOneWidget);
      expect(find.text('Chat'), findsOneWidget);
      expect(find.text('Beli'), findsNothing);
      expect(find.text('LIVE'), findsWidgets);
    });

    testWidgets('Auction TOMBSTONE cannot navigate', (tester) async {
      final projection = _parseProjection(_auctionTombstoneJson());
      await tester.pumpWidget(_projectionCard(projection));

      final shell = tester.widget<CommerceMarketplaceCardShell>(
        find.byType(CommerceMarketplaceCardShell),
      );
      expect(shell.onTap, isNull);
    });
  });
}
