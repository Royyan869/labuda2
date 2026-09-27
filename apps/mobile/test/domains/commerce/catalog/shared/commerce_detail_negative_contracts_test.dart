import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

String _source(String relativePath) {
  return File(relativePath).readAsStringSync().replaceAll('\r\n', '\n');
}

const _forSaleDetailScreen =
    'lib/domains/commerce/catalog/for_sale/presentation/screens/for_sale_detail_screen.dart';
const _auctionDetailScreen =
    'lib/domains/commerce/catalog/auction/presentation/screens/auction_detail_screen.dart';
const _auctionSellerCard =
    'lib/domains/commerce/catalog/auction/presentation/widgets/detail/auction_seller_card.dart';
const _auctionDetailInfo =
    'lib/domains/commerce/catalog/auction/presentation/widgets/detail/auction_detail_info.dart';
const _auctionDetailHeader =
    'lib/domains/commerce/catalog/auction/presentation/widgets/detail/auction_detail_header.dart';
const _detailSellerCardAuthority =
    'lib/domains/commerce/catalog/shared/presentation/widgets/commerce_detail_seller_card.dart';
const _detailStates =
    'lib/domains/commerce/catalog/shared/presentation/widgets/commerce_detail_states.dart';
const _commonProductSection =
    'lib/domains/commerce/catalog/shared/presentation/widgets/commerce_common_product_detail_section.dart';

const _activeDetailInventory = <String>[
  'lib/domains/commerce/catalog/shared/presentation/widgets/commerce_detail_primitives.dart',
  _detailStates,
  _detailSellerCardAuthority,
  _commonProductSection,
  _forSaleDetailScreen,
  _auctionDetailScreen,
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
  test('both detail screens keep their own canonical scaffold', () {
    // Each channel composes its Scaffold (app-bar actions and bottom bar are
    // channel-owned); the STATE surfaces below are the shared part.
    expect(_source(_forSaleDetailScreen), contains('return Scaffold('));
    expect(_source(_auctionDetailScreen), contains('return Scaffold('));
  });

  test('both sale channels compose the same canonical detail skeleton', () {
    final listing = _source(_forSaleDetailScreen);
    final auction = _source(_auctionDetailScreen);

    // 1. Shared state authority — loading / error / notFound vocabulary is
    //    never channel-owned.
    expect(listing, contains('CommerceDetailStates.'));
    expect(auction, contains('CommerceDetailStates.'));

    // 2. Shared app bar.
    expect(listing, contains('AppBarCustom('));
    expect(auction, contains('AppBarCustom('));

    // 3. Shared scroll pattern — the old channel divergence
    //    (SingleChildScrollView vs CustomScrollView) is purged.
    expect(listing, contains('CustomScrollView('));
    expect(auction, contains('CustomScrollView('));
    expect(listing, isNot(contains('SingleChildScrollView(')));
    expect(auction, isNot(contains('SingleChildScrollView(')));

    // 4. Shared seller render authority — ForSale consumes it directly,
    //    Auction through its thin channel wrapper.
    expect(listing, contains('CommerceDetailSellerCard('));
    expect(_source(_auctionSellerCard), contains('CommerceDetailSellerCard('));

    // 5. Shared Product content section — ForSale consumes it directly,
    //    Auction through its channel info widget.
    expect(listing, contains('CommerceCommonProductDetailSection('));
    expect(
      _source(_auctionDetailInfo),
      contains('CommerceCommonProductDetailSection('),
    );

    // 6. Canonical media block — same carousel, same 4/3 aspect, edge to edge.
    expect(listing, contains('MediaCarouselWidget('));
    expect(listing, contains('aspectRatio: 4 / 3'));
    expect(_source(_auctionDetailHeader), contains('MediaCarouselWidget('));
    expect(_source(_auctionDetailHeader), contains('aspectRatio: 4 / 3'));
  });

  test('seller identity resolution has ONE authority for both channels', () {
    final authority = _source(_detailSellerCardAuthority);
    expect(authority, contains('buildCommerceSellerIdentity('));
    expect(authority, contains('publicRedactionLabel'));

    // Channel screens resolve no identity and redact on their own — they
    // delegate to the shared authority.
    expect(
      _source(_forSaleDetailScreen),
      isNot(contains('buildCommerceSellerIdentity(')),
    );
    expect(
      _source(_forSaleDetailScreen),
      isNot(contains('publicRedactionLabel')),
    );
    expect(
      _source(_auctionSellerCard),
      isNot(contains('buildCommerceSellerIdentity(')),
    );

    // Both channels RENDER the shared authority.
    expect(_source(_forSaleDetailScreen), contains('CommerceDetailSellerCard('));
    expect(_source(_auctionSellerCard), contains('CommerceDetailSellerCard('));
  });

  test('state handlers stay in the channel screens', () {
    final listing = _source(_forSaleDetailScreen);
    final auction = _source(_auctionDetailScreen);
    final primitives = _source(
      'lib/domains/commerce/catalog/shared/presentation/widgets/commerce_detail_primitives.dart',
    );

    // Each channel maps its provider states onto the ONE shared authority —
    // a channel may translate its own error copy, but never own a renderer.
    expect(listing, contains('CommerceDetailStates.'));
    expect(auction, contains('CommerceDetailStates.'));
    expect(listing, contains('loading:'));
    expect(listing, contains('error:'));
    expect(auction, contains('error:'));

    // Negative proof: a channel never renders its own state surface. The
    // only spinner in the auction screen is the in-action busy dialog, and
    // neither screen draws an error/empty icon — CommerceDetailStates owns them.
    expect(listing, isNot(contains('CircularProgressIndicator')));
    expect(listing, isNot(contains('Icons.error')));
    expect(auction, isNot(contains('Icons.error')));
    expect(auction, isNot(contains('Icons.search_off')));

    // Primitives stay presentation-only: no lifecycle/transaction facts.
    expect(primitives, isNot(contains('sellerTrustLifecycle')));
    expect(primitives, isNot(contains('winnerId')));
    expect(primitives, isNot(contains('isAvailable')));
  });

  test('active detail inventory excludes prohibited controllers', () {
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
  });
}
