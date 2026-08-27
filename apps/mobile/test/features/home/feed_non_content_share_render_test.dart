import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/features/home/domain/entities/feed_item.dart';
import 'package:labuda/features/home/presentation/providers/feed_renderers.dart';
import 'package:labuda/shared/object/object_preview_provider.dart';
import 'package:labuda/shared/object/presentation/widgets/object_preview_card.dart';
import 'package:labuda/domains/social/content/domain/entities/content_resource_projection.dart';
import 'package:labuda/domains/social/content/presentation/widgets/content_resource_projection_card.dart';
import 'package:labuda/shared/widgets/repost_attribution_bar.dart';

Widget _wrap(Widget child) {
  return ProviderScope(
    overrides: [
      objectPreviewProvider.overrideWith((ref, reference) async => null),
    ],
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
    'resource_type': 'fixed_price_sale',
    'resource_id': resourceId,
    'fixed_price_sale': <String, dynamic>{
      'title': title,
      'media': <Map<String, dynamic>>[],
      'thumbnail_url': thumbnailUrl,
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
  required String thumbnailUrl,
}) {
  return <String, dynamic>{
    'state': 'LIVE',
    'resource_type': 'auction',
    'resource_id': resourceId,
    'auction': <String, dynamic>{
      'title': title,
      'media': <Map<String, dynamic>>[],
      'thumbnail_url': thumbnailUrl,
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
              'shareReference': <String, dynamic>{
                'targetType': 'fixed_price_sale',
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
    await tester.pumpAndSettle();

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
            id: 'listing-share-1',
            content: 'listing share content',
            type: FeedItemType.content,
            additionalData: {
              'resourceProjection': ContentResourceProjection.fromJson(
                _fixedPriceSaleProjection(
                  resourceId: 'sale-1',
                  title: 'listing share',
                  thumbnailUrl: 'https://example.com/listing.jpg',
                ),
              ),
            },
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byType(RepostAttributionBar), findsNothing);
    expect(find.byType(ObjectPreviewCard), findsNothing);
    expect(find.byType(ContentResourceProjectionCard), findsOneWidget);
    expect(find.text('listing share'), findsOneWidget);
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
              'resourceProjection': ContentResourceProjection.fromJson(
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
    await tester.pumpAndSettle();

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
              'resourceProjection': ContentResourceProjection.fromJson(
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
    await tester.pumpAndSettle();

    expect(find.byType(RepostAttributionBar), findsNothing);
    expect(find.byType(ObjectPreviewCard), findsNothing);
    expect(find.byType(ContentResourceProjectionCard), findsOneWidget);
    expect(find.text('@profile share'), findsOneWidget);
  });
}
