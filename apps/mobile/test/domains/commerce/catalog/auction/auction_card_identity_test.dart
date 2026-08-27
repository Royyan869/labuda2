import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/domain.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/widgets/auction_card.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/shared/governance/seller_inactive_badge.dart';

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
    startTime: DateTime.parse('2026-01-01T00:00:00.000Z'),
    endTime: DateTime.parse('2026-01-02T00:00:00.000Z'),
    status: AuctionStatus.active,
    totalBidders: 0,
    totalWatchers: 0,
    totalViews: 0,
    createdAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
  );
}

void main() {
  testWidgets('Auction Card renders @username then store_name', (tester) async {
    await tester.pumpWidget(_wrap(_auction()));

    final usernameFinder = find.text('@yayan');
    final storeFinder = find.text('Farm Koi Nusantara');

    expect(usernameFinder, findsOneWidget);
    expect(storeFinder, findsOneWidget);
    expect(
      tester.getTopLeft(usernameFinder).dy,
      lessThan(tester.getTopLeft(storeFinder).dy),
    );
  });

  testWidgets('Store missing fallback renders @username only', (tester) async {
    await tester.pumpWidget(_wrap(_auction(sellerFarmName: null)));

    expect(find.text('@yayan'), findsOneWidget);
    expect(find.text('Farm Koi Nusantara'), findsNothing);
  });

  testWidgets('Degraded lifecycle redaction overrides identity', (
    tester,
  ) async {
    await tester.pumpWidget(
      _wrap(_auction(userLifecycle: ContentLifecycle.unavailable)),
    );

    expect(find.text('Pengguna tidak tersedia'), findsOneWidget);
    expect(find.text('@yayan'), findsNothing);
    expect(find.text('Farm Koi Nusantara'), findsNothing);
  });

  testWidgets('Removed lifecycle redaction overrides identity', (tester) async {
    await tester.pumpWidget(
      _wrap(_auction(userLifecycle: ContentLifecycle.removed)),
    );

    expect(find.text('Pengguna dihapus'), findsOneWidget);
    expect(find.text('@yayan'), findsNothing);
    expect(find.text('Farm Koi Nusantara'), findsNothing);
  });

  testWidgets('Inactive seller badge still renders when trust is degraded', (
    tester,
  ) async {
    await tester.pumpWidget(
      _wrap(_auction(trustLifecycle: ContentLifecycle.unavailable)),
    );

    expect(find.byType(SellerInactiveBadge), findsOneWidget);
  });
}
