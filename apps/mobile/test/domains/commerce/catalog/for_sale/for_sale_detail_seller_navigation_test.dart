import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/domain/domain.dart';
import 'package:labuda/domains/commerce/catalog/shared/presentation/widgets/commerce_detail_primitives.dart';
import 'package:labuda/shared/models/seller_identity_data.dart';
import 'package:labuda/shared/shared.dart';

class _NavSpy extends Fake implements NavigationHandler {
  String? userId;

  @override
  void navigateToUserProfile(String userId) {
    this.userId = userId;
  }
}

ForSale _listing() {
  final now = DateTime.utc(2026, 7, 24);
  return ForSale(
    forSaleId: 'listing-1',
    productId: 'product-1',
    title: 'Showa Koi 30cm',
    description: 'Premium showa',
    price: 1500000,
    stock: 1,
    sellerId: 'seller-1',
    sellerUsername: 'seller_user',
    sellerFarmName: 'Acme Farm',
    sellerAvatar: null,
    status: ForSaleStatus.active,
    visibility: ForSaleVisibility.public,
    createdAt: now,
    updatedAt: now,
  );
}

Widget _wrap(_NavSpy nav) {
  final listing = _listing();
  final identity = SellerIdentityData(
    userId: listing.sellerId,
    username: listing.sellerUsername,
    storeName: listing.sellerFarmName,
    avatarUrl: listing.sellerAvatar,
    isSeller: true,
  );
  return ProviderScope(
    overrides: [navigationHandlerProvider.overrideWith((ref) => nav)],
    child: MaterialApp(
      home: Scaffold(
        body: CommerceDetailSellerIdentityCard(
          identity: identity,
          isDegraded: false,
          redactionLabel: 'unused',
          sellerTier: null,
          onTap: () => nav.navigateToUserProfile(listing.sellerId),
        ),
      ),
    ),
  );
}

void main() {
  testWidgets('seller identity tap navigates by durable seller id', (
    tester,
  ) async {
    final nav = _NavSpy();

    await tester.pumpWidget(_wrap(nav));
    await tester.pumpAndSettle();

    expect(find.text('@seller_user'), findsOneWidget);

    await tester.tap(find.text('@seller_user'));
    await tester.pumpAndSettle();

    expect(nav.userId, 'seller-1');
    expect(nav.userId, isNot('seller_user'));
    expect(nav.userId, isNot(contains('@')));
  });

  test(
    'listing detail screen threads route observer and shared gallery wiring',
    () {
      final source = File(
        'lib/domains/commerce/catalog/for_sale/presentation/screens/for_sale_detail_screen.dart',
      ).readAsStringSync();

      expect(source, contains('screenViewRouteObserverProvider'));
      expect(source, contains('CommerceDetailMediaGallery('));
      expect(source, contains('routeObserver: routeObserver'));
      expect(source, contains('media: listing.media'));
      expect(source, contains('logicalCacheKeyBuilder: (media, index)'));
      expect(source, isNot(contains('VideoPlayerController')));
      expect(source, isNot(contains('Chewie')));
    },
  );
}
