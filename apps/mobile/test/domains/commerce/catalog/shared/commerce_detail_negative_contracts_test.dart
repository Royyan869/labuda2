import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

String _source(String relativePath) {
  return File(relativePath).readAsStringSync().replaceAll('\r\n', '\n');
}

const _activeDetailInventory = <String>[
  'lib/domains/commerce/catalog/shared/presentation/widgets/commerce_detail_primitives.dart',
  'lib/domains/commerce/catalog/listing/presentation/screens/listing_detail_screen.dart',
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
      'lib/domains/commerce/catalog/listing/presentation/screens/listing_detail_screen.dart',
    );
    final auctionDetail = _source(
      'lib/domains/commerce/catalog/auction/presentation/screens/auction_detail_screen.dart',
    );
    final shared = _source(
      'lib/domains/commerce/catalog/shared/presentation/widgets/commerce_detail_primitives.dart',
    );

    expect(listingDetail, contains('CommerceDetailShell('));
    expect(auctionDetail, contains('CommerceDetailShell('));
    expect(listingDetail, isNot(contains('CommerceDetailScaffold(')));
    expect(auctionDetail, isNot(contains('CommerceDetailScaffold(')));
    expect(listingDetail, isNot(contains('return Scaffold(')));
    expect(auctionDetail, isNot(contains('return Scaffold(')));
    expect(shared, contains('return Scaffold('));
    expect(shared, contains('CommerceDetailStickyActionBar('));
    expect(shared, contains('MediaViewerWidget('));
    expect(shared, contains('SafeArea(top: false'));
  });

  test(
    'active detail inventory excludes prohibited literals and controllers',
    () {
      for (final path in _activeDetailInventory) {
        final source = _source(path);
        expect(source, isNot(contains('Colors.white')), reason: path);
        expect(source, isNot(contains('Colors.black')), reason: path);
        expect(source, isNot(contains('Color(0x')), reason: path);
        expect(source, isNot(contains('Chewie')), reason: path);
        expect(source, isNot(contains('VideoPlayerController')), reason: path);
        expect(source, isNot(contains('UniqueKey(')), reason: path);
        expect(source, isNot(contains('Image.network(')), reason: path);

        if (path !=
            'lib/domains/commerce/catalog/shared/presentation/widgets/commerce_detail_primitives.dart') {
          expect(source, isNot(contains('PageController(')), reason: path);
        }
      }
    },
  );

  test('seller identity adapter remains a thin domain wrapper', () {
    final listingDetail = _source(
      'lib/domains/commerce/catalog/listing/presentation/screens/listing_detail_screen.dart',
    );
    final auctionAdapter = _source(
      'lib/domains/commerce/catalog/auction/presentation/widgets/detail/auction_seller_card.dart',
    );

    expect(listingDetail, contains('CommerceDetailSellerIdentityCard('));
    expect(listingDetail, contains('SellerIdentityData('));

    expect(auctionAdapter, contains('CommerceDetailSellerIdentityCard('));
    expect(auctionAdapter, contains('SellerIdentityData('));
    expect(auctionAdapter, isNot(contains('SellerIdentityView(')));
    expect(auctionAdapter, isNot(contains('SellerDualAvatar(')));
    expect(auctionAdapter, isNot(contains('StableNetworkImage(')));
    expect(auctionAdapter, isNot(contains('Image.network(')));
  });

  test('loading error redaction and sticky-action authority stay in source', () {
    final listingDetail = _source(
      'lib/domains/commerce/catalog/listing/presentation/screens/listing_detail_screen.dart',
    );
    final auctionDetail = _source(
      'lib/domains/commerce/catalog/auction/presentation/screens/auction_detail_screen.dart',
    );
    final auctionBottomBar = _source(
      'lib/domains/commerce/catalog/auction/presentation/widgets/detail/auction_detail_bottom_bar.dart',
    );
    final shared = _source(
      'lib/domains/commerce/catalog/shared/presentation/widgets/commerce_detail_primitives.dart',
    );

    expect(listingDetail, contains('loading:'));
    expect(listingDetail, contains('error:'));
    expect(listingDetail, contains('publicRedactionLabel'));
    expect(listingDetail, contains('CommerceDetailRoleAwareActionBar('));
    expect(listingDetail, isNot(contains('_resolveCapabilities(')));
    expect(auctionDetail, contains('_buildLoadingScaffold'));
    expect(auctionDetail, contains('_buildErrorScaffold'));
    expect(auctionDetail, contains('_buildNotFoundScaffold'));
    expect(auctionBottomBar, contains('CommerceDetailRoleAwareActionBar('));
    expect(auctionBottomBar, isNot(contains('_fallbackCapabilities(')));
    expect(listingDetail, contains('CommerceDetailStickyActionBar('));
    expect(shared, isNot(contains('sellerTrustLifecycle')));
    expect(shared, isNot(contains('winnerId')));
    expect(shared, isNot(contains('isAvailable')));
  });

  test('detail screens no longer hand-roll their outer scroll scaffold', () {
    final listingDetail = _source(
      'lib/domains/commerce/catalog/listing/presentation/screens/listing_detail_screen.dart',
    );
    final auctionDetail = _source(
      'lib/domains/commerce/catalog/auction/presentation/screens/auction_detail_screen.dart',
    );

    expect(listingDetail, isNot(contains('return CommerceDetailScaffold(')));
    expect(auctionDetail, isNot(contains('return CommerceDetailScaffold(')));
    expect(listingDetail, isNot(contains('SingleChildScrollView(')));
    expect(auctionDetail, isNot(contains('CustomScrollView(')));
    expect(auctionDetail, isNot(contains('AuctionDetailHeader(')));
    expect(auctionDetail, isNot(contains('AuctionSellerCard(')));
  });
}
