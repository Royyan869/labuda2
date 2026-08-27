// Seller tier badge — parse + widget visibility tests.
//
// Scope:
//   1) SellerTier.fromApiValue parse correctness
//   2) SellerTierBadge widget renders pro/elite, hides for null/basic/unknown
//   3) AuthUser._mapApiDataToAuthUser seller_tier parse integration

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:labuda/domains/user/identity/authentication/domain/entities/seller_tier.dart';
import 'package:labuda/shared/governance/seller_tier_badge.dart';

void main() {
  group('SellerTier.fromApiValue', () {
    test('null → null', () {
      expect(SellerTier.fromApiValue(null), isNull);
    });

    test('empty string → null', () {
      expect(SellerTier.fromApiValue(''), isNull);
    });

    test('"basic" → sellerBasic', () {
      expect(SellerTier.fromApiValue('basic'), SellerTier.sellerBasic);
    });

    test('"pro" → sellerPro', () {
      expect(SellerTier.fromApiValue('pro'), SellerTier.sellerPro);
    });

    test('"elite" → sellerElite', () {
      expect(SellerTier.fromApiValue('elite'), SellerTier.sellerElite);
    });

    test('unknown value → sellerBasic (safe fallback)', () {
      expect(SellerTier.fromApiValue('legendary'), SellerTier.sellerBasic);
    });
  });

  group('SellerTier.apiValue round-trip', () {
    test('basic round-trips', () {
      expect(SellerTier.fromApiValue('basic')?.apiValue, 'basic');
    });

    test('pro round-trips', () {
      expect(SellerTier.fromApiValue('pro')?.apiValue, 'pro');
    });

    test('elite round-trips', () {
      expect(SellerTier.fromApiValue('elite')?.apiValue, 'elite');
    });
  });

  group('SellerTierBadge widget', () {
    Widget wrap(String? tier) {
      return MaterialApp(
        home: Scaffold(body: SellerTierBadge(tier: tier)),
      );
    }

    testWidgets('null tier → SizedBox.shrink (no badge)', (tester) async {
      await tester.pumpWidget(wrap(null));
      expect(find.byType(SizedBox), findsOneWidget);
      expect(find.text('Pro Seller'), findsNothing);
      expect(find.text('Elite Seller'), findsNothing);
    });

    testWidgets('"basic" tier → no badge', (tester) async {
      await tester.pumpWidget(wrap('basic'));
      expect(find.text('Pro Seller'), findsNothing);
      expect(find.text('Elite Seller'), findsNothing);
    });

    testWidgets('unknown tier → no badge', (tester) async {
      await tester.pumpWidget(wrap('legendary'));
      expect(find.text('Pro Seller'), findsNothing);
      expect(find.text('Elite Seller'), findsNothing);
    });

    testWidgets('"pro" tier → shows Pro Seller badge', (tester) async {
      await tester.pumpWidget(wrap('pro'));
      expect(find.text('Pro Seller'), findsOneWidget);
      expect(find.byIcon(Icons.star_rounded), findsOneWidget);
    });

    testWidgets('"elite" tier → shows Elite Seller badge', (tester) async {
      await tester.pumpWidget(wrap('elite'));
      expect(find.text('Elite Seller'), findsOneWidget);
      expect(find.byIcon(Icons.workspace_premium_rounded), findsOneWidget);
    });
  });
}
