import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/domain.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/widgets/detail/auction_detail_header.dart';
import 'package:labuda/domains/social/content/domain/entities/content.dart';
import 'package:labuda/shared/widgets/stable_network_image.dart';

Widget _wrap(Auction auction) {
  return MaterialApp(
    home: Scaffold(body: AuctionDetailHeader(auction: auction)),
  );
}

Auction _auction({required List<MediaEntity> media}) {
  return Auction(
    id: 'auction-1',
    sellerId: 'seller-1',
    sellerUsername: 'yayan',
    sellerFarmName: 'Farm Koi Nusantara',
    title: 'Sanke Auction',
    description: 'Live auction',
    media: media,
    koiDetails: const KoiDetails(
      variety: 'Kohaku',
      sizeInCm: 0,
      ageInMonths: 0,
      gender: 'unknown',
      certificates: [],
    ),
    openingBid: 1000000,
    currentBid: 1500000,
    bidIncrement: 50000,
    startTime: DateTime.parse('2026-01-01T00:00:00.000Z'),
    endTime: DateTime.parse('2026-01-02T00:00:00.000Z'),
    status: AuctionStatus.active,
    createdAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
  );
}

String _resolveImageUrl(ImageProvider<Object> provider) {
  final resolved = provider is ResizeImage ? provider.imageProvider : provider;
  return (resolved as NetworkImage).url;
}

void main() {
  testWidgets('Auction detail header renders media in declared order', (
    tester,
  ) async {
    const firstUrl =
        'https://cdn.example.com/auctions/auction-1-first.jpg?X-Amz-Signature=one';
    const secondUrl =
        'https://cdn.example.com/auctions/auction-1-second.jpg?X-Amz-Signature=two';

    await tester.pumpWidget(
      _wrap(
        _auction(
          media: [
            MediaEntity(
              id: 'first',
              originalUrl: firstUrl,
              type: MediaType.image,
              createdAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
            ),
            MediaEntity(
              id: 'second',
              originalUrl: secondUrl,
              type: MediaType.image,
              createdAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
            ),
          ],
        ),
      ),
    );

    final pageView = find.byType(PageView);
    expect(pageView, findsOneWidget);

    var images = tester.widgetList<Image>(find.byType(Image)).toList();
    expect(images, hasLength(1));
    expect(_resolveImageUrl(images.single.image), firstUrl);

    await tester.drag(pageView, const Offset(-400, 0));
    await tester.pumpAndSettle();

    images = tester.widgetList<Image>(find.byType(Image)).toList();
    expect(images, hasLength(1));
    expect(_resolveImageUrl(images.single.image), secondUrl);
    expect(find.text('Sanke Auction'), findsOneWidget);
  });

  testWidgets('Auction detail header preserves page controller on refresh', (
    tester,
  ) async {
    const firstUrl =
        'https://cdn.example.com/auctions/auction-1-first.jpg?X-Amz-Signature=one';
    const secondUrl =
        'https://cdn.example.com/auctions/auction-1-second.jpg?X-Amz-Signature=two';
    const firstUrlUpdated =
        'https://cdn.example.com/auctions/auction-1-first.jpg?X-Amz-Signature=updated';
    const secondUrlUpdated =
        'https://cdn.example.com/auctions/auction-1-second.jpg?X-Amz-Signature=updated';

    await tester.pumpWidget(
      _wrap(
        _auction(
          media: [
            MediaEntity(
              id: 'first',
              originalUrl: firstUrl,
              type: MediaType.image,
              createdAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
            ),
            MediaEntity(
              id: 'second',
              originalUrl: secondUrl,
              type: MediaType.image,
              createdAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
            ),
          ],
        ),
      ),
    );

    final controllerBefore = tester
        .widget<PageView>(find.byType(PageView))
        .controller;
    expect(find.byType(StableNetworkImage), findsOneWidget);

    await tester.drag(find.byType(PageView), const Offset(-400, 0));
    await tester.pumpAndSettle();

    await tester.pumpWidget(
      _wrap(
        _auction(
          media: [
            MediaEntity(
              id: 'first',
              originalUrl: firstUrlUpdated,
              type: MediaType.image,
              createdAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
            ),
            MediaEntity(
              id: 'second',
              originalUrl: secondUrlUpdated,
              type: MediaType.image,
              createdAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
            ),
          ],
        ),
      ),
    );
    await tester.pumpAndSettle();

    final controllerAfter = tester
        .widget<PageView>(find.byType(PageView))
        .controller;
    expect(identical(controllerBefore, controllerAfter), isTrue);

    final images = tester.widgetList<Image>(find.byType(Image)).toList();
    expect(images, hasLength(1));
    expect(_resolveImageUrl(images.single.image), secondUrlUpdated);
  });
}
