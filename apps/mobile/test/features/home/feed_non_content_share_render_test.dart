import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/features/home/domain/entities/feed_item.dart';
import 'package:labuda/features/home/presentation/providers/feed_renderers.dart';
import 'package:labuda/shared/object/presentation/widgets/object_preview_card.dart';
import 'package:labuda/shared/domain/entities/resource_projection.dart';
import 'package:labuda/domains/social/content/presentation/widgets/content_resource_projection_card.dart';
import 'package:labuda/domains/user/identity/authentication/presentation/providers/auth_controller.dart';
import 'package:labuda/domains/user/identity/authentication/presentation/providers/auth_state.dart';
import 'package:labuda/shared/widgets/repost_attribution_bar.dart';

/// Guest auth state: the feed footer reads the auth controller, and a guest
/// state keeps this harness free of the apiClient wiring main.dart owns.
class _GuestAuthController extends AuthController {
  @override
  AuthState build() => const AuthState.unauthenticated();
}

Widget _wrap(Widget child) {
  return ProviderScope(
    overrides: [authControllerProvider.overrideWith(_GuestAuthController.new)],
    child: MaterialApp(
      home: Scaffold(
        body: SingleChildScrollView(child: child),
      ),
    ),
  );
}

FeedItem _baseItem({
  required String id,
  required String content,
  required FeedItemType type,
  required Map<String, dynamic> additionalData,
}) {
  return FeedItem(
    id: id,
    content: content,
    authorId: 'author-1',
    authorUsername: 'author',
    type: type,
    createdAt: DateTime.utc(2026, 6, 2, 10, 0),
    additionalData: {
      'title': content,
      'caption': content,
      'status': 'active',
      ...additionalData,
    },
  );
}

Map<String, dynamic> _fixedPriceSaleProjection({
  required String resourceId,
  required String title,
  required String thumbnailUrl,
}) {
  return <String, dynamic>{
    'state': 'LIVE',
    'resource_type': 'for_sale',
    'resource_id': resourceId,
    'canonical_url': '/for-sale/$resourceId',
    'viewer_capabilities': <String, dynamic>{
      'can_view': true,
      'can_interact': true,
      'blocked_by_tombstone': false,
    },
    'commerce_actions': <String, dynamic>{
      'role': 'buyer',
      'can_chat': true,
      'can_negotiate': true,
      'can_buy': true,
      'can_bid': false,
      'can_manage': false,
    },
    'for_sale': <String, dynamic>{
      'title': title,
      'media': <Map<String, dynamic>>[
        {'url': thumbnailUrl, 'kind': 'image'},
      ],
      'thumbnail_url': thumbnailUrl,
      'price': {'amount': 1500000, 'currency': 'IDR'},
      'status': 'active',
      'quantity_available': 3,
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
  required String thumbnailUrl,
}) {
  return <String, dynamic>{
    'state': 'LIVE',
    'resource_type': 'auction',
    'resource_id': resourceId,
    'canonical_url': '/auction/$resourceId',
    'viewer_capabilities': <String, dynamic>{
      'can_view': true,
      'can_interact': true,
      'blocked_by_tombstone': false,
    },
    'commerce_actions': <String, dynamic>{
      'role': 'buyer',
      'can_chat': true,
      'can_negotiate': false,
      'can_buy': false,
      'can_bid': true,
      'can_manage': false,
    },
    'auction': <String, dynamic>{
      'title': title,
      'media': <Map<String, dynamic>>[],
      'thumbnail_url': thumbnailUrl,
      'lifecycle': 'active',
      'current_bid': 1750000,
      'buy_now_price': 2500000,
      'end_at': '2026-08-10T10:00:00.000Z',
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
    'canonical_url': '/user/$resourceId',
    'viewer_capabilities': <String, dynamic>{
      'can_view': true,
      'can_interact': false,
      'blocked_by_tombstone': false,
    },
    'profile': <String, dynamic>{
      'username': username,
      'avatar_url': avatarUrl,
      'lifecycle': 'active',
    },
  };
}

void main() {
  testWidgets('legacy share payload does not render an object preview card', (
    tester,
  ) async {
    await tester.pumpWidget(
      _wrap(
        FeedCard(
          item: _baseItem(
            id: 'legacy-share-1',
            content: 'legacy share content',
            type: FeedItemType.content,
            additionalData: {
              'shareReference': <String, dynamic>{                 'targetType': 'for_sale',
                'targetId': 'sale-1',
                'preview': <String, dynamic>{
                  'title': 'legacy preview',
                  'imageUrl': 'https://example.com/legacy.jpg',
                },
              },
            },
          ),
        ),
      ),
    );
    // Never pumpAndSettle: the projection card's AppImage shimmer skeletons
    // animate forever by design — bounded pump instead (test follows codebase).
    await tester.pump(const Duration(milliseconds: 400));

    expect(find.byType(RepostAttributionBar), findsNothing);
    expect(find.byType(ObjectPreviewCard), findsNothing);
    expect(find.text('legacy share content'), findsOneWidget);
    expect(find.text('Sedang Mencari Koi'), findsNothing);
    expect(find.text('Tawarkan Ikan'), findsNothing);
  });

  testWidgets('fixed price sale resource projection renders canonical card', (
    tester,
  ) async {
    await tester.pumpWidget(
      _wrap(
        FeedCard(
          item: _baseItem(
            id: 'forSale-share-1',
            content: 'forSale share content',
            type: FeedItemType.content,
            additionalData: {
              'resourceProjection': ResourceProjection.fromJson(
                _fixedPriceSaleProjection(
                  resourceId: 'sale-1',
                  title: 'forSale share',
                  thumbnailUrl: 'https://example.com/forSale.jpg',
                ),
              ),
            },
          ),
        ),
      ),
    );
    // Never pumpAndSettle: the projection card's AppImage shimmer skeletons
    // animate forever by design — bounded pump instead (test follows codebase).
    await tester.pump(const Duration(milliseconds: 400));

    expect(find.byType(RepostAttributionBar), findsNothing);
    expect(find.byType(ObjectPreviewCard), findsNothing);
    expect(find.byType(ContentResourceProjectionCard), findsOneWidget);
    expect(find.text('forSale share'), findsOneWidget);
  });

  testWidgets('auction resource projection renders canonical card', (
    tester,
  ) async {
    await tester.pumpWidget(
      _wrap(
        FeedCard(
          item: _baseItem(
            id: 'auction-share-1',
            content: 'auction share content',
            type: FeedItemType.content,
            additionalData: {
              'resourceProjection': ResourceProjection.fromJson(
                _auctionProjection(
                  resourceId: 'auction-1',
                  title: 'auction share',
                  thumbnailUrl: 'https://example.com/auction.jpg',
                ),
              ),
            },
          ),
        ),
      ),
    );
    // Never pumpAndSettle: the projection card's AppImage shimmer skeletons
    // animate forever by design — bounded pump instead (test follows codebase).
    await tester.pump(const Duration(milliseconds: 400));

    expect(find.byType(RepostAttributionBar), findsNothing);
    expect(find.byType(ObjectPreviewCard), findsNothing);
    expect(find.byType(ContentResourceProjectionCard), findsOneWidget);
    expect(find.text('auction share'), findsOneWidget);
  });

  testWidgets('profile resource projection renders canonical card', (
    tester,
  ) async {
    await tester.pumpWidget(
      _wrap(
        FeedCard(
          item: _baseItem(
            id: 'profile-share-1',
            content: 'profile share content',
            type: FeedItemType.content,
            additionalData: {
              'resourceProjection': ResourceProjection.fromJson(
                _profileProjection(
                  resourceId: 'profile-1',
                  username: 'profile share',
                  avatarUrl: 'https://example.com/profile.jpg',
                ),
              ),
            },
          ),
        ),
      ),
    );
    // Never pumpAndSettle: the projection card's AppImage shimmer skeletons
    // animate forever by design — bounded pump instead (test follows codebase).
    await tester.pump(const Duration(milliseconds: 400));

    expect(find.byType(RepostAttributionBar), findsNothing);
    expect(find.byType(ObjectPreviewCard), findsNothing);
    expect(find.byType(ContentResourceProjectionCard), findsOneWidget);
    expect(find.text('@profile share'), findsOneWidget);
  });
}
