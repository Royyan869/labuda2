import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/domains/commerce/catalog/for_sale/domain/domain.dart';
import 'package:hishumi/domains/commerce/catalog/for_sale/presentation/widgets/for_sale_card.dart';
import 'package:hishumi/domains/commerce/catalog/shared/presentation/widgets/commerce_marketplace_primitives.dart';
import 'package:hishumi/shared/governance/content_lifecycle.dart';
import 'package:hishumi/shared/governance/seller_inactive_badge.dart';

/// CARD IDENTITY CONTRACT (owner decision 2026-09-27):
///
/// The For Sale discovery card NEVER renders seller identity — no @username,
/// no store name, no redaction label, no seller-trust badge. Identity and
/// governance live on the detail surface and in search. This keeps the card
/// frame identical to AuctionCard so the promotion grid can reuse it.
Widget _wrap(ForSale forSale) {
  return MaterialApp(
    home: Scaffold(
      body: ListView(
        children: [ForSaleCard(forSale: forSale, onTap: () {})],
      ),
    ),
  );
}

ForSale _listing({
  String? sellerUsername = 'yayan',
  String? sellerFarmName = 'Farm Koi Nusantara',
  ContentLifecycle userLifecycle = ContentLifecycle.active,
  ContentLifecycle trustLifecycle = ContentLifecycle.active,
}) {
  return ForSale(
    forSaleId: 'forSale-1',
    title: 'Showa Koi 30cm',
    description: 'Premium showa',
    price: 1500000,
    stock: 1,
    sellerId: 'seller-1',
    sellerUsername: sellerUsername,
    sellerFarmName: sellerFarmName,
    sellerUserLifecycle: userLifecycle,
    sellerTrustLifecycle: trustLifecycle,
    status: ForSaleStatus.active,
    visibility: ForSaleVisibility.public,
    createdAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
    updatedAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
  );
}

void main() {
  testWidgets('ForSale Card renders title, price and media — no author', (
    tester,
  ) async {
    await tester.pumpWidget(_wrap(_listing()));
    await tester.pumpAndSettle();

    expect(find.text('Showa Koi 30cm'), findsOneWidget);
    expect(find.textContaining('Rp'), findsWidgets);
    expect(find.byType(CommerceMarketplaceCardShell), findsOneWidget);

    expect(find.text('@yayan'), findsNothing);
    expect(find.text('Farm Koi Nusantara'), findsNothing);
    expect(find.textContaining('yayan'), findsNothing);
  });

  testWidgets('Degraded lifecycle never leaks identity or redaction label', (
    tester,
  ) async {
    await tester.pumpWidget(
      _wrap(_listing(userLifecycle: ContentLifecycle.unavailable)),
    );
    await tester.pumpAndSettle();

    expect(find.text('Showa Koi 30cm'), findsOneWidget);
    expect(find.text('@yayan'), findsNothing);
    expect(find.text('Farm Koi Nusantara'), findsNothing);
    expect(find.text('Pengguna tidak tersedia'), findsNothing);
  });

  testWidgets('Removed lifecycle redaction never reaches the card', (
    tester,
  ) async {
    await tester.pumpWidget(
      _wrap(_listing(userLifecycle: ContentLifecycle.removed)),
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
        _wrap(_listing(trustLifecycle: ContentLifecycle.unavailable)),
      );
      await tester.pumpAndSettle();

      expect(find.byType(SellerInactiveBadge), findsNothing);
    },
  );
}
