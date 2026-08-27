import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

String _source(String relativePath) {
  return File(relativePath).readAsStringSync().replaceAll('\r\n', '\n');
}

String _block(String source, String startMarker, String endMarker) {
  final start = source.indexOf(startMarker);
  final end = source.indexOf(endMarker, start + startMarker.length);
  expect(start, isNonNegative, reason: 'missing $startMarker');
  expect(end, isNonNegative, reason: 'missing $endMarker');
  return source.substring(start, end);
}

void main() {
  test('auction card uses the shared marketplace media contract', () {
    final source = _source(
      'lib/domains/commerce/catalog/auction/presentation/widgets/auction_card.dart',
    );

    expect(source, contains('CommerceMarketplaceCardMedia('));
    expect(source, contains('auctionMediaLogicalKey('));
    expect(source, isNot(contains('Image.network(')));
  });

  test('auction recommendations keep the stable image primitive', () {
    final source = _source(
      'lib/domains/commerce/catalog/auction/presentation/widgets/detail/auction_recommendations_section.dart',
    );

    expect(source, contains('StableNetworkImage('));
    expect(source, isNot(contains('Image.network(')));
  });

  test('auction detail header uses the shared media gallery primitive', () {
    final source = _source(
      'lib/domains/commerce/catalog/auction/presentation/widgets/detail/auction_detail_header.dart',
    );

    expect(source, contains('CommerceDetailMediaGallery('));
    expect(source, contains('media: auction.media'));
    expect(source, contains('logicalCacheKeyBuilder: (media, index)'));
    expect(source, contains('routeObserver: routeObserver'));
    expect(source, isNot(contains('Image.network(')));
  });

  test('listing detail screen uses the shared media gallery primitive', () {
    final source = _source(
      'lib/domains/commerce/catalog/listing/presentation/screens/listing_detail_screen.dart',
    );

    expect(source, contains('CommerceDetailMediaGallery('));
    expect(source, contains('routeObserver: routeObserver'));
    expect(source, isNot(contains('Image.network(')));
  });

  test('auction detail screen avoids raw Image.network in active detail UI', () {
    final source = _source(
      'lib/domains/commerce/catalog/auction/presentation/screens/auction_detail_screen.dart',
    );

    expect(source, isNot(contains('Image.network(')));
    expect(source, contains('screenViewRouteObserverProvider'));
    expect(source, contains('AuctionDetailHeader('));
    expect(source, contains('routeObserver: routeObserver'));
  });

  test(
    'promoted auction feed block uses shared marketplace media and keys',
    () {
      final source = _source(
        'lib/features/home/presentation/providers/feed_renderers.dart',
      );
      expect(source, contains('promo-auction-'));
      final block = _block(
        source,
        'class PromotedAuctionCard extends ConsumerWidget',
        'class PromotedExternalCard extends ConsumerWidget',
      );

      expect(block, contains('CommerceMarketplaceCardMedia('));
      expect(block, contains('auctionMediaLogicalKey('));
      expect(block, isNot(contains('sellerUsername')));
      expect(block, isNot(contains('sellerFarmName')));
      expect(block, isNot(contains('Image.network(')));
    },
  );

  test('auction card call sites and detail header use stable item keys', () {
    expect(
      _source(
        'lib/features/explore/presentation/widgets/explore_auction_tab.dart',
      ),
      contains("ValueKey('auction-card-"),
    );
    expect(
      _source(
        'lib/domains/user/preference/seller/presentation/widgets/profile_store_tab.dart',
      ),
      contains("ValueKey('auction-card-"),
    );
    expect(
      _source(
        'lib/domains/commerce/catalog/auction/presentation/screens/auction_detail_screen.dart',
      ),
      contains('auction-detail-header-'),
    );
  });

  test('auction detail header and screen remain free of raw video controllers', () {
    final header = _source(
      'lib/domains/commerce/catalog/auction/presentation/widgets/detail/auction_detail_header.dart',
    );
    final screen = _source(
      'lib/domains/commerce/catalog/auction/presentation/screens/auction_detail_screen.dart',
    );

    expect(header, isNot(contains('VideoPlayerController')));
    expect(header, isNot(contains('Chewie')));
    expect(header, isNot(contains('autoPlay')));
    expect(header, isNot(contains('looping')));
    expect(screen, isNot(contains('VideoPlayerController')));
    expect(screen, isNot(contains('Chewie')));
  });
}
