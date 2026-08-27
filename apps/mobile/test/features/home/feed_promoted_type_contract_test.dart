import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/features/home/domain/entities/feed_item.dart';
import 'package:labuda/features/home/presentation/providers/feed_renderers.dart';
import 'package:visibility_detector/visibility_detector.dart';

class _FactoryHost extends ConsumerWidget {
  final FeedItem item;

  const _FactoryHost({required this.item});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return MaterialApp(
      home: Scaffold(body: FeedCardFactory.buildCardForFeedItem(item, ref)),
    );
  }
}

FeedItem _baseItem({
  required String id,
  required String content,
  required FeedItemType type,
  Map<String, dynamic> additionalData = const {},
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

Future<void> _pumpFactory(WidgetTester tester, FeedItem item) async {
  await tester.pumpWidget(ProviderScope(child: _FactoryHost(item: item)));
  await tester.pumpAndSettle();
}

void main() {
  setUp(() {
    VisibilityDetectorController.instance.updateInterval = Duration.zero;
  });

  testWidgets('promoted listing renders PromotedListingCard', (tester) async {
    await _pumpFactory(
      tester,
      _baseItem(
        id: 'promo-listing-1',
        content: 'promoted listing',
        type: FeedItemType.promotedListing,
        additionalData: const {
          'promotionInstanceId': 'pi-1',
          'targetType': 'listing',
          'fixedPriceSaleId': 'listing-1',
          'pricePerUnit': 150000,
          'imageUrl': 'https://example.com/listing.jpg',
          'sellerUsername': 'seller_user',
          'sellerFarmName': 'Farm Name',
          'title': 'promoted listing',
        },
      ),
    );

    expect(find.byType(PromotedListingCard), findsOneWidget);
    expect(find.byType(PromotedAuctionCard), findsNothing);
    expect(find.byType(PromotedExternalCard), findsNothing);
    expect(find.byType(FeedCard), findsNothing);
  });

  testWidgets('promoted auction renders PromotedAuctionCard', (tester) async {
    await _pumpFactory(
      tester,
      _baseItem(
        id: 'promo-auction-1',
        content: 'promoted auction',
        type: FeedItemType.promotedAuction,
        additionalData: const {
          'promotionInstanceId': 'pi-2',
          'targetType': 'auction',
          'auctionId': 'auction-1',
          'startPrice': 250000,
          'currentBid': 300000,
          'bidCount': 4,
          'status': 'active',
          'endAt': '2026-06-02T12:00:00Z',
          'imageUrl': 'https://example.com/auction.jpg',
          'sellerUsername': 'seller_user',
          'sellerFarmName': 'Farm Name',
          'title': 'promoted auction',
        },
      ),
    );

    expect(find.byType(PromotedAuctionCard), findsOneWidget);
    expect(find.byType(PromotedListingCard), findsNothing);
    expect(find.byType(PromotedExternalCard), findsNothing);
    expect(find.byType(FeedCard), findsNothing);
  });

  testWidgets('promoted external renders PromotedExternalCard', (tester) async {
    await _pumpFactory(
      tester,
      _baseItem(
        id: 'promo-external-1',
        content: 'promoted external',
        type: FeedItemType.promotedExternal,
        additionalData: const {
          'promotionInstanceId': 'pi-3',
          'targetType': 'external_product',
          'externalUrl': 'https://example.com/product',
          'externalMediaUrl': 'https://example.com/product.jpg',
          'title': 'promoted external',
        },
      ),
    );

    expect(find.byType(PromotedExternalCard), findsOneWidget);
    expect(find.byType(PromotedListingCard), findsNothing);
    expect(find.byType(PromotedAuctionCard), findsNothing);
    expect(find.byType(FeedCard), findsNothing);
  });

  testWidgets('promoted listing renders split seller identity', (tester) async {
    await _pumpFactory(
      tester,
      _baseItem(
        id: 'promo-listing-2',
        content: 'promoted listing 2',
        type: FeedItemType.promotedListing,
        additionalData: const {
          'promotionInstanceId': 'pi-4',
          'targetType': 'listing',
          'fixedPriceSaleId': 'listing-2',
          'pricePerUnit': 150000,
          'imageUrl': 'https://example.com/listing.jpg',
          'sellerUsername': 'seller_user',
          'sellerFarmName': 'Farm Name',
          'title': 'promoted listing 2',
        },
      ),
    );

    expect(find.text('@seller_user • Farm Name'), findsOneWidget);
  });

  testWidgets('promoted listing renders split seller identity only', (
    tester,
  ) async {
    await _pumpFactory(
      tester,
      _baseItem(
        id: 'promo-listing-3',
        content: 'promoted listing 3',
        type: FeedItemType.promotedListing,
        additionalData: const {
          'promotionInstanceId': 'pi-5',
          'targetType': 'listing',
          'fixedPriceSaleId': 'listing-3',
          'pricePerUnit': 150000,
          'imageUrl': 'https://example.com/listing.jpg',
          'sellerUsername': 'seller_user',
          'title': 'promoted listing 3',
        },
      ),
    );

    expect(find.text('@seller_user'), findsOneWidget);
  });

  testWidgets('universal content renders through FeedCard', (tester) async {
    await _pumpFactory(
      tester,
      _baseItem(
        id: 'content-1',
        content: 'normal content',
        type: FeedItemType.content,
      ),
    );

    expect(find.byType(FeedCard), findsOneWidget);
    expect(find.text('normal content'), findsOneWidget);
  });
}
