// Stage 2 seller tier badge — listing/auction lifecycle-gate tests.
//
// Scope:
//   1) SellerTierBadge dark-mode color variant (no crash, renders correctly).
//   2) AuctionSellerCard renders tier badge when all gates pass.
//   3) AuctionSellerCard hides badge when user identity is degraded.
//   4) AuctionSellerCard hides badge when seller trust is unavailable.
//   5) AuctionSellerCard hides badge when tier is null/basic.
//   6) Seller identity row still visible when badge is suppressed.

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:labuda/domains/commerce/catalog/auction/domain/domain.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/widgets/detail/auction_seller_card.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/shared/governance/seller_tier_badge.dart';

// ── Helpers ──────────────────────────────────────────────────────────────────

Widget _wrapBadge(String? tier, {bool dark = false}) {
  return MaterialApp(
    theme: dark ? ThemeData.dark() : ThemeData.light(),
    home: Scaffold(body: SellerTierBadge(tier: tier)),
  );
}

Auction _auction({
  ContentLifecycle userLifecycle = ContentLifecycle.active,
  ContentLifecycle trustLifecycle = ContentLifecycle.active,
  String? sellerTier,
}) {
  return Auction(
    id: 'auction-1',
    sellerId: 'seller-1',
    sellerUsername: 'seller',
    sellerFarmName: 'Test Farm',
    sellerUserLifecycle: userLifecycle,
    sellerTrustLifecycle: trustLifecycle,
    sellerTier: sellerTier,
    title: 'Test Auction',
    description: '',
    koiDetails: KoiDetails(
      variety: 'Kohaku',
      sizeInCm: 0,
      ageInMonths: 0,
      gender: 'unknown',
      certificates: const [],
    ),
    openingBid: 100,
    currentBid: 100,
    bidIncrement: 10,
    startTime: DateTime.now(),
    endTime: DateTime.now().add(const Duration(days: 1)),
    status: AuctionStatus.active,
    totalBidders: 0,
    totalWatchers: 0,
    totalViews: 0,
    createdAt: DateTime.now(),
  );
}

Widget _wrapCard(Auction auction) {
  return MaterialApp(
    home: Scaffold(body: AuctionSellerCard(auction: auction)),
  );
}

// ── Tests ─────────────────────────────────────────────────────────────────────

void main() {
  group('SellerTierBadge dark mode', () {
    testWidgets('pro badge renders in dark mode without crash', (tester) async {
      await tester.pumpWidget(_wrapBadge('pro', dark: true));
      expect(find.byIcon(Icons.star_rounded), findsOneWidget);
    });

    testWidgets('elite badge renders in dark mode without crash', (
      tester,
    ) async {
      await tester.pumpWidget(_wrapBadge('elite', dark: true));
      expect(find.byIcon(Icons.workspace_premium_rounded), findsOneWidget);
    });

    testWidgets('null tier in dark mode → SizedBox.shrink', (tester) async {
      await tester.pumpWidget(_wrapBadge(null, dark: true));
      // SizedBox.shrink is the fallback; no icon present
      expect(find.byIcon(Icons.star_rounded), findsNothing);
      expect(find.byIcon(Icons.workspace_premium_rounded), findsNothing);
    });
  });

  group('AuctionSellerCard tier badge', () {
    testWidgets('shows Pro badge when all gates pass', (tester) async {
      await tester.pumpWidget(_wrapCard(_auction(sellerTier: 'pro')));
      expect(find.byType(SellerTierBadge), findsOneWidget);
      expect(find.byIcon(Icons.star_rounded), findsOneWidget);
    });

    testWidgets('shows Elite badge when all gates pass', (tester) async {
      await tester.pumpWidget(_wrapCard(_auction(sellerTier: 'elite')));
      expect(find.byType(SellerTierBadge), findsOneWidget);
      expect(find.byIcon(Icons.workspace_premium_rounded), findsOneWidget);
    });

    testWidgets('hides badge when user identity is unavailable (suspended)', (
      tester,
    ) async {
      await tester.pumpWidget(
        _wrapCard(
          _auction(
            userLifecycle: ContentLifecycle.unavailable,
            sellerTier: 'pro',
          ),
        ),
      );
      expect(find.byIcon(Icons.star_rounded), findsNothing);
    });

    testWidgets('hides badge when user identity is removed (banned/deleted)', (
      tester,
    ) async {
      await tester.pumpWidget(
        _wrapCard(
          _auction(
            userLifecycle: ContentLifecycle.removed,
            sellerTier: 'elite',
          ),
        ),
      );
      expect(find.byIcon(Icons.workspace_premium_rounded), findsNothing);
    });

    testWidgets(
      'hides badge when trust lifecycle is unavailable (expired sub)',
      (tester) async {
        await tester.pumpWidget(
          _wrapCard(
            _auction(
              trustLifecycle: ContentLifecycle.unavailable,
              sellerTier: 'pro',
            ),
          ),
        );
        expect(find.byIcon(Icons.star_rounded), findsNothing);
      },
    );

    testWidgets('hides badge when sellerTier is null', (tester) async {
      await tester.pumpWidget(_wrapCard(_auction(sellerTier: null)));
      expect(find.byIcon(Icons.star_rounded), findsNothing);
      expect(find.byIcon(Icons.workspace_premium_rounded), findsNothing);
    });

    testWidgets('hides badge for basic tier', (tester) async {
      await tester.pumpWidget(_wrapCard(_auction(sellerTier: 'basic')));
      expect(find.byIcon(Icons.star_rounded), findsNothing);
      expect(find.byIcon(Icons.workspace_premium_rounded), findsNothing);
    });

    testWidgets(
      'seller identity row still visible when badge suppressed (trust degraded)',
      (tester) async {
        await tester.pumpWidget(
          _wrapCard(
            _auction(
              trustLifecycle: ContentLifecycle.unavailable,
              sellerTier: 'elite',
            ),
          ),
        );
        // Handle line remains visible even when badge is hidden.
        expect(find.text('@seller'), findsOneWidget);
        // Store line remains visible when present.
        expect(find.text('Test Farm'), findsOneWidget);
      },
    );

    testWidgets('renders @username first and store name second', (
      tester,
    ) async {
      await tester.pumpWidget(_wrapCard(_auction(sellerTier: 'pro')));
      expect(find.text('@seller'), findsOneWidget);
      expect(find.text('Test Farm'), findsOneWidget);
    });
  });
}
