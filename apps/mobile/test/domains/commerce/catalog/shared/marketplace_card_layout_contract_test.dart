import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/domains/commerce/catalog/auction/domain/domain.dart';
import 'package:hishumi/domains/commerce/catalog/auction/presentation/widgets/auction_card.dart';
import 'package:hishumi/domains/commerce/catalog/for_sale/domain/domain.dart';
import 'package:hishumi/domains/commerce/catalog/for_sale/presentation/widgets/for_sale_card.dart';
import 'package:hishumi/domains/commerce/catalog/shared/presentation/widgets/commerce_marketplace_metrics.dart';
import 'package:hishumi/domains/commerce/catalog/shared/presentation/widgets/commerce_marketplace_primitives.dart';
import 'package:hishumi/shared/governance/content_lifecycle.dart';

/// MARKETPLACE CARD LAYOUT CONTRACT (owner decision 2026-09-27).
///
/// Why this test exists: earlier "fixes" only moved magic numbers around, so
/// the design drifted back. These assertions lock the decisions:
///
///  1. Geometry lives in [CommerceMarketplaceMetrics] — radius 12, grid edge
///     12, gap 8, content rhythm 8, media 4:5 contain, title one line;
///  2. Neither channel overflows its grid cell, at any tested width and at
///     accessibility text scaling;
///  3. For Sale and Auction cards render IDENTICAL geometry — the promotion
///     grid reuses this card shape for both channels, so any divergence here
///     breaks promoted placements too.

class _Snapshot {
  const _Snapshot({
    required this.exception,
    required this.cardSize,
    required this.cardTopLeft,
    required this.titleTopLeft,
    required this.valueTopLeft,
  });

  final Object? exception;
  final Size cardSize;
  final Offset cardTopLeft;
  final Offset titleTopLeft;
  final Offset valueTopLeft;
}

ForSale _forSale({required String title}) {
  return ForSale(
    forSaleId: 'forSale-1',
    title: title,
    description: 'Premium showa',
    price: 1500000,
    stock: 1,
    sellerId: 'seller-1',
    sellerUsername: 'yayan',
    sellerFarmName: 'Farm Koi Nusantara',
    sellerUserLifecycle: ContentLifecycle.active,
    sellerTrustLifecycle: ContentLifecycle.active,
    status: ForSaleStatus.active,
    visibility: ForSaleVisibility.public,
    createdAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
    updatedAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
  );
}

Auction _auction({required String title}) {
  return Auction(
    id: 'auction-1',
    sellerId: 'seller-1',
    sellerUsername: 'yayan',
    sellerFarmName: 'Farm Koi Nusantara',
    sellerUserLifecycle: ContentLifecycle.active,
    sellerTrustLifecycle: ContentLifecycle.active,
    title: title,
    description: 'Live auction',
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

Future<_Snapshot> _pumpGrid(
  WidgetTester tester, {
  required Widget card,
  required Finder cardFinder,
  required Finder titleFinder,
  required Finder valueFinder,
  double scale = 1.0,
  Size surface = const Size(360, 640),
}) async {
  await tester.binding.setSurfaceSize(surface);
  addTearDown(() => tester.binding.setSurfaceSize(null));

  await tester.pumpWidget(
    MaterialApp(
      home: MediaQuery(
        data: MediaQueryData(
          size: surface,
          textScaler: TextScaler.linear(scale),
        ),
        child: Scaffold(
          body: CustomScrollView(
            slivers: [
              CommerceMarketplaceGrid(
                itemCount: 1,
                itemBuilder: (context, index) => card,
              ),
            ],
          ),
        ),
      ),
    ),
  );
  await tester.pumpAndSettle();

  final exception = tester.takeException();
  final cardBox = tester.renderObject<RenderBox>(cardFinder);
  final titleBox = tester.renderObject<RenderBox>(titleFinder);
  final valueBox = tester.renderObject<RenderBox>(valueFinder);

  return _Snapshot(
    exception: exception,
    cardSize: cardBox.size,
    cardTopLeft: cardBox.localToGlobal(Offset.zero),
    titleTopLeft: titleBox.localToGlobal(Offset.zero),
    valueTopLeft: valueBox.localToGlobal(Offset.zero),
  );
}

void main() {
  test('geometry tokens are the owner-locked values', () {
    expect(CommerceMarketplaceMetrics.cardRadius, 12);
    expect(CommerceMarketplaceMetrics.gridEdgePadding, 12);
    expect(CommerceMarketplaceMetrics.gridGap, 8);
    expect(CommerceMarketplaceMetrics.contentPadding, 8);
    expect(CommerceMarketplaceMetrics.contentGap, 8);
    expect(CommerceMarketplaceMetrics.titleMaxLines, 1);
    expect(CommerceMarketplaceMetrics.mediaAspectRatio, 4 / 5);
  });

  test('grid and card shell read their defaults from the tokens', () {
    final grid = CommerceMarketplaceGrid(
      itemCount: 0,
      itemBuilder: (context, index) => const SizedBox.shrink(),
    );
    expect(grid.padding, const EdgeInsets.fromLTRB(12, 12, 12, 16));
    expect(grid.crossAxisSpacing, CommerceMarketplaceMetrics.gridGap);
    expect(grid.mainAxisSpacing, CommerceMarketplaceMetrics.gridGap);
    expect(grid.childAspectRatio, CommerceMarketplaceMetrics.childAspectRatio);

    const shell = CommerceMarketplaceCardShell(
      media: CommerceMarketplaceCardMedia(
        imageUrl: null,
        fallback: SizedBox.shrink(),
      ),
      title: 'Title',
      value: CommerceMarketplaceCardValue(value: 'Rp 1'),
    );
    expect(shell.borderRadius, const BorderRadius.all(Radius.circular(12)));
    expect(shell.contentPadding, const EdgeInsets.all(8));
    expect(shell.titleMaxLines, 1);

    const media = CommerceMarketplaceCardMedia(
      imageUrl: null,
      fallback: SizedBox.shrink(),
    );
    expect(media.aspectRatio, 4 / 5);
    expect(media.fit, BoxFit.contain);
  });

  test('shared primitives carry no stray geometry literals', () {
    final primitives = _readSource(
      'lib/domains/commerce/catalog/shared/presentation/'
      'widgets/commerce_marketplace_primitives.dart',
    );
    expect(primitives, isNot(contains('Radius.circular(16)')));
    expect(primitives, isNot(contains('EdgeInsets.fromLTRB(8, 8, 8, 16)')));
    expect(primitives, isNot(contains('BoxFit.cover')));
    expect(primitives, isNot(contains('childAspectRatio = 0.53')));
  });

  testWidgets('no channel overflows its grid cell at any tested scale', (
    tester,
  ) async {
    const surfaces = <Size>[
      Size(360, 640),
      Size(500, 800),
      Size(700, 900),
      Size(1200, 1600),
    ];
    const scales = <double>[1.0, 1.3];

    const longTitle =
        'Sankei Kohaku Gin Rin Kanoko Premium Kolam Empat Musim '
        'Ukuran 48cm';

    for (final scale in scales) {
      for (final surface in surfaces) {
        final forSale = await _pumpGrid(
          tester,
          card: ForSaleCard(
            forSale: _forSale(title: longTitle),
            onTap: () {},
          ),
          cardFinder: find.byType(ForSaleCard),
          titleFinder: find.text(longTitle),
          valueFinder: find.textContaining('Rp'),
          scale: scale,
          surface: surface,
        );
        expect(
          forSale.exception,
          isNull,
          reason: 'ForSale overflow @${surface.width}dp scale $scale',
        );

        final auction = await _pumpGrid(
          tester,
          card: AuctionCard(
            auction: _auction(title: longTitle),
            onTap: () {},
          ),
          cardFinder: find.byType(AuctionCard),
          titleFinder: find.text(longTitle),
          valueFinder: find.textContaining('Rp'),
          scale: scale,
          surface: surface,
        );
        expect(
          auction.exception,
          isNull,
          reason: 'Auction overflow @${surface.width}dp scale $scale',
        );
      }
    }
  });

  testWidgets('For Sale and Auction cards share identical geometry', (
    tester,
  ) async {
    const title = 'Showa Koi 30cm';

    for (final scale in <double>[1.0, 1.3]) {
      final forSale = await _pumpGrid(
        tester,
        card: ForSaleCard(
          forSale: _forSale(title: title),
          onTap: () {},
        ),
        cardFinder: find.byType(ForSaleCard),
        titleFinder: find.text(title),
        valueFinder: find.textContaining('Rp'),
        scale: scale,
      );
      final auction = await _pumpGrid(
        tester,
        card: AuctionCard(
          auction: _auction(title: title),
          onTap: () {},
        ),
        cardFinder: find.byType(AuctionCard),
        titleFinder: find.text(title),
        valueFinder: find.textContaining('Rp'),
        scale: scale,
      );

      expect(forSale.exception, isNull, reason: 'scale $scale');
      expect(auction.exception, isNull, reason: 'scale $scale');
      expect(auction.cardSize, forSale.cardSize, reason: 'scale $scale');
      expect(
        auction.titleTopLeft,
        forSale.titleTopLeft,
        reason: 'title must sit at the same spot in both channels',
      );
      expect(
        auction.valueTopLeft,
        forSale.valueTopLeft,
        reason: 'value must sit at the same spot in both channels',
      );
    }
  });
}

String _readSource(String relativePath) {
  return File(relativePath).readAsStringSync().replaceAll('\r\n', '\n');
}
