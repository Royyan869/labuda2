import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

String _source(String relativePath) {
  return File(relativePath).readAsStringSync().replaceAll('\r\n', '\n');
}

const _activeDetailInventory = <String>[
  'lib/domains/commerce/catalog/shared/presentation/widgets/commerce_detail_primitives.dart',
  'lib/domains/commerce/catalog/for_sale/presentation/screens/for_sale_detail_screen.dart',
  'lib/domains/commerce/catalog/auction/presentation/screens/auction_detail_screen.dart',
  'lib/domains/commerce/catalog/auction/presentation/widgets/detail/auction_action_modal.dart',
  'lib/domains/commerce/catalog/auction/presentation/widgets/detail/auction_bid_history.dart',
  'lib/domains/commerce/catalog/auction/presentation/widgets/detail/auction_bid_position_indicator.dart',
  'lib/domains/commerce/catalog/auction/presentation/widgets/detail/auction_bid_section.dart',
  'lib/domains/commerce/catalog/auction/presentation/widgets/detail/auction_claim_shipping_modal.dart',
  'lib/domains/commerce/catalog/auction/presentation/widgets/detail/auction_countdown_timer.dart',
  'lib/domains/commerce/catalog/auction/presentation/widgets/detail/auction_detail_bottom_bar.dart',
  'lib/domains/commerce/catalog/auction/presentation/widgets/detail/auction_detail_header.dart',
  'lib/domains/commerce/catalog/auction/presentation/widgets/detail/auction_detail_info.dart',
  'lib/domains/commerce/catalog/auction/presentation/widgets/detail/auction_recommendations_section.dart',
  'lib/domains/commerce/catalog/auction/presentation/widgets/detail/auction_seller_card.dart',
  'lib/domains/commerce/catalog/auction/presentation/widgets/detail/auction_seller_settlement_monitor.dart',
];

void main() {
  test('shared scaffold authority stays centralized', () {
    final listingDetail = _source(
      'lib/domains/commerce/catalog/for_sale/presentation/screens/for_sale_detail_screen.dart',
    );
    final auctionDetail = _source(
      'lib/domains/commerce/catalog/auction/presentation/screens/auction_detail_screen.dart',
    );

    // Both detail screens use their own Scaffold
    expect(listingDetail, contains('return Scaffold('));
    expect(auctionDetail, contains('return Scaffold('));
  });

  test(
    'active detail inventory excludes prohibited controllers',
    () {
      for (final path in _activeDetailInventory) {
        final source = _source(path);
        expect(source, isNot(contains('Chewie')), reason: path);
        expect(source, isNot(contains('VideoPlayerController')), reason: path);
        expect(source, isNot(contains('UniqueKey(')), reason: path);

        if (path !=
            'lib/domains/commerce/catalog/shared/presentation/widgets/commerce_detail_primitives.dart') {
          expect(source, isNot(contains('PageController(')), reason: path);
        }
      }
    },
  );

  test('seller identity adapter remains a thin domain wrapper', () {
    final listingDetail = _source(
      'lib/domains/commerce/catalog/for_sale/presentation/screens/for_sale_detail_screen.dart',
    );
    final auctionAdapter = _source(
      'lib/domains/commerce/catalog/auction/presentation/widgets/detail/auction_seller_card.dart',
    );

    // Both surfaces use buildCommerceSellerIdentity for canonical identity resolution
    expect(listingDetail, contains('buildCommerceSellerIdentity('));
    expect(auctionAdapter, contains('buildCommerceSellerIdentity('));
  });

  test('loading error redaction and sticky-action authority stay in source', () {
    final listingDetail = _source(
      'lib/domains/commerce/catalog/for_sale/presentation/screens/for_sale_detail_screen.dart',
    );
    final auctionDetail = _source(
      'lib/domains/commerce/catalog/auction/presentation/screens/auction_detail_screen.dart',
    );
    final shared = _source(
      'lib/domains/commerce/catalog/shared/presentation/widgets/commerce_detail_primitives.dart',
    );

    expect(listingDetail, contains('loading:'));
    expect(listingDetail, contains('error:'));
    expect(listingDetail, contains('publicRedactionLabel'));
    expect(auctionDetail, contains('_buildLoadingScaffold'));
    expect(auctionDetail, contains('_buildErrorScaffold'));
    expect(auctionDetail, contains('_buildNotFoundScaffold'));
    expect(shared, isNot(contains('sellerTrustLifecycle')));
    expect(shared, isNot(contains('winnerId')));
    expect(shared, isNot(contains('isAvailable')));
  });

  test('detail screens use canonical scroll patterns', () {
    final listingDetail = _source(
      'lib/domains/commerce/catalog/for_sale/presentation/screens/for_sale_detail_screen.dart',
    );
    final auctionDetail = _source(
      'lib/domains/commerce/catalog/auction/presentation/screens/auction_detail_screen.dart',
    );

    // ForSale uses SingleChildScrollView for scrolling
    expect(listingDetail, contains('SingleChildScrollView('));
    // Auction uses CustomScrollView for scrolling
    expect(auctionDetail, contains('CustomScrollView('));
  });
}
