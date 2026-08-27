import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/domain/domain.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/widgets/for_sale_card.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/shared/governance/seller_inactive_badge.dart';

Widget _wrap(ForSale listing) {
  return MaterialApp(
    home: Scaffold(
      body: ListView(
        children: [ForSaleCard(listing: listing, onTap: () {})],
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
    forSaleId: 'listing-1',
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
  testWidgets('Listing Card renders @username then store_name', (tester) async {
    await tester.pumpWidget(_wrap(_listing()));

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
    await tester.pumpWidget(_wrap(_listing(sellerFarmName: null)));

    expect(find.text('@yayan'), findsOneWidget);
    expect(find.text('Farm Koi Nusantara'), findsNothing);
  });

  testWidgets('Degraded lifecycle redaction overrides identity', (
    tester,
  ) async {
    await tester.pumpWidget(
      _wrap(_listing(userLifecycle: ContentLifecycle.unavailable)),
    );

    expect(find.text('Pengguna tidak tersedia'), findsOneWidget);
    expect(find.text('@yayan'), findsNothing);
    expect(find.text('Farm Koi Nusantara'), findsNothing);
  });

  testWidgets('Removed lifecycle redaction overrides identity', (tester) async {
    await tester.pumpWidget(
      _wrap(_listing(userLifecycle: ContentLifecycle.removed)),
    );

    expect(find.text('Pengguna dihapus'), findsOneWidget);
    expect(find.text('@yayan'), findsNothing);
    expect(find.text('Farm Koi Nusantara'), findsNothing);
  });

  testWidgets('Inactive seller badge still renders when trust is degraded', (
    tester,
  ) async {
    await tester.pumpWidget(
      _wrap(_listing(trustLifecycle: ContentLifecycle.unavailable)),
    );

    expect(find.byType(SellerInactiveBadge), findsOneWidget);
  });
}
