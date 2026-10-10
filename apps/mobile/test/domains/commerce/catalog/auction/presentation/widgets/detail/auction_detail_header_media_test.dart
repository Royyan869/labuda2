import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/domains/commerce/catalog/auction/domain/domain.dart';
import 'package:hishumi/domains/commerce/catalog/auction/presentation/widgets/detail/auction_detail_header.dart';
import 'package:hishumi/domains/social/content/domain/entities/content.dart';
import 'package:hishumi/shared/widgets/media_carousel_widget.dart';
import 'package:hishumi/shared/widgets/media_viewer_widget.dart';

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
  // The canonical carousel loads through CachedNetworkImage; only legacy
  // paths produce a raw NetworkImage.
  if (resolved is CachedNetworkImageProvider) return resolved.url;
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

    // Only the current page is built initially; it is the FIRST declared item.
    var images = tester.widgetList<Image>(find.byType(Image)).toList();
    expect(images, hasLength(1));
    expect(_resolveImageUrl(images.first.image), firstUrl);

    final controller = tester.widget<PageView>(pageView).controller;
    expect(controller, isNotNull);

    await tester.drag(pageView, const Offset(-900, 0));
    // Bounded pumps: the carousel's loading shimmer never settles under
    // fake async.
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 600));

    // The pager reaches the second declared item.
    expect(controller!.page, closeTo(1.0, 0.01));
    images = tester.widgetList<Image>(find.byType(Image)).toList();
    expect(images, isNotEmpty);
    final urls = images.map((i) => _resolveImageUrl(i.image)).toList();
    expect(urls, contains(secondUrl));
    // The title is NOT part of the media block — both detail channels render
    // it as the first body item (canonical detail skeleton).
    expect(find.byType(MediaCarouselWidget), findsOneWidget);
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
    expect(find.byType(MediaCarouselWidget), findsOneWidget);

    await tester.drag(find.byType(PageView), const Offset(-900, 0));
    // Bounded pumps: the carousel's loading shimmer never settles under
    // fake async.
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 600));

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
    // Bounded pumps: the carousel's loading shimmer never settles under
    // fake async.
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 600));

    final controllerAfter = tester
        .widget<PageView>(find.byType(PageView))
        .controller;
    expect(identical(controllerBefore, controllerAfter), isTrue);
    // The refresh keeps the pager on the second declared item.
    expect(controllerAfter?.page, closeTo(1.0, 0.01));

    // Every rendered page picked up the refreshed URLs.
    final images = tester.widgetList<Image>(find.byType(Image)).toList();
    expect(images, isNotEmpty);
    final urls = images.map((i) => _resolveImageUrl(i.image)).toSet();
    for (final url in urls) {
      expect(url, anyOf(firstUrlUpdated, secondUrlUpdated));
    }
    expect(urls, contains(secondUrlUpdated));
  });

  testWidgets('Detail gallery matches the card contract and opens fullscreen',
      (tester) async {
    const firstUrl = 'https://cdn.example.com/auctions/cover.jpg';

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
          ],
        ),
      ),
    );
    await tester.pump();

    // Card ≡ detail: 4:5 contain, koi never cropped.
    final carousel = tester.widget<MediaCarouselWidget>(
      find.byType(MediaCarouselWidget),
    );
    expect(carousel.aspectRatio, 4 / 5);
    expect(carousel.fit, BoxFit.contain);

    // Tap opens the canonical fullscreen viewer.
    await tester.tap(find.byType(MediaCarouselWidget));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 300));

    expect(find.byType(MediaViewerWidget), findsOneWidget);
  });
}
