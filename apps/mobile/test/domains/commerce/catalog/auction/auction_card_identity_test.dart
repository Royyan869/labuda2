import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/domains/commerce/catalog/auction/domain/domain.dart';
import 'package:hishumi/domains/commerce/catalog/auction/presentation/widgets/auction_card.dart';
import 'package:hishumi/domains/commerce/catalog/shared/presentation/widgets/commerce_marketplace_primitives.dart';
import 'package:hishumi/shared/governance/content_lifecycle.dart';
import 'package:hishumi/shared/governance/seller_inactive_badge.dart';

/// CARD IDENTITY CONTRACT (owner decision 2026-09-27):
///
/// The Auction discovery card NEVER renders seller identity — no @username,
/// no store name, no redaction label, no seller-trust badge. The countdown /
/// status chip is an OVERLAY on the media (not a row under it), so the card
/// frame is identical to ForSaleCard and reusable by the promotion grid.
Widget _wrap(Auction auction) {
  return MaterialApp(
    home: Scaffold(
      body: ListView(
        children: [AuctionCard(auction: auction, onTap: () {})],
      ),
    ),
  );
}

Auction _auction({
  String? sellerUsername = 'yayan',
  String? sellerFarmName = 'Farm Koi Nusantara',
  ContentLifecycle userLifecycle = ContentLifecycle.active,
  ContentLifecycle trustLifecycle = ContentLifecycle.active,
  DateTime? startTime,
  DateTime? endTime,
}) {
  return Auction(
    id: 'auction-1',
    sellerId: 'seller-1',
    sellerUsername: sellerUsername,
    sellerFarmName: sellerFarmName,
    sellerUserLifecycle: userLifecycle,
    sellerTrustLifecycle: trustLifecycle,
    title: 'Sanke Auction',
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
    startTime: startTime ?? DateTime.parse('2026-01-01T00:00:00.000Z'),
    endTime: endTime ?? DateTime.parse('2026-01-02T00:00:00.000Z'),
    status: AuctionStatus.active,
    createdAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
  );
}

void main() {
  testWidgets('Auction Card renders title, bid and countdown — no author', (
    tester,
  ) async {
    await tester.pumpWidget(_wrap(_auction()));
    await tester.pumpAndSettle();

    expect(find.text('Sanke Auction'), findsOneWidget);
    expect(find.textContaining('Rp'), findsWidgets);
    expect(find.byType(CommerceMarketplaceCardShell), findsOneWidget);

    // Exactly one chip: the countdown overlay on the media. No badge row.
    expect(find.byType(CommerceMarketplaceCardBadge), findsOneWidget);

    expect(find.text('@yayan'), findsNothing);
    expect(find.text('Farm Koi Nusantara'), findsNothing);
    expect(find.textContaining('yayan'), findsNothing);
  });

  testWidgets('Degraded lifecycle never leaks identity or redaction label', (
    tester,
  ) async {
    await tester.pumpWidget(
      _wrap(_auction(userLifecycle: ContentLifecycle.unavailable)),
    );
    await tester.pumpAndSettle();

    expect(find.text('Sanke Auction'), findsOneWidget);
    expect(find.text('@yayan'), findsNothing);
    expect(find.text('Farm Koi Nusantara'), findsNothing);
    expect(find.text('Pengguna tidak tersedia'), findsNothing);
  });

  testWidgets('Removed lifecycle redaction never reaches the card', (
    tester,
  ) async {
    await tester.pumpWidget(
      _wrap(_auction(userLifecycle: ContentLifecycle.removed)),
    );
    await tester.pumpAndSettle();

    expect(find.text('Pengguna dihapus'), findsNothing);
    expect(find.text('@yayan'), findsNothing);
    expect(find.text('Farm Koi Nusantara'), findsNothing);
  });

  testWidgets(
    'Seller trust badge is a detail-surface concern, not a card one',
    (tester) async {
      await tester.pumpWidget(
        _wrap(_auction(trustLifecycle: ContentLifecycle.unavailable)),
      );
      await tester.pumpAndSettle();

      expect(find.byType(SellerInactiveBadge), findsNothing);
    },
  );
}
