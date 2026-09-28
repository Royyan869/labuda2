import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

String _source(String relativePath) {
  return File(relativePath).readAsStringSync().replaceAll('\r\n', '\n');
}

void main() {
  test('auction card renders media with the channel-identical frame', () {
    final source = _source(
      'lib/domains/commerce/catalog/auction/presentation/widgets/auction_card.dart',
    );

    // The card is a thin wrapper: frame lives in the shared commerce
    // primitives so For Sale, Auction and the promotion grid stay identical.
    expect(source, contains('CommerceMarketplaceCardShell('));
    expect(source, isNot(contains('return Card(')));

    // Owner decision 2026-09-27: no seller block on discovery cards, and the
    // countdown/status chip is a MEDIA overlay — never a row under the media.
    expect(source, isNot(contains('CommerceCardSellerMetadata(')));
    expect(source, isNot(contains('sellerUsername')));
    expect(source, contains('overlay:'));
    expect(source, isNot(contains('badges:')));
  });

  test('auction detail header renders media without raw video controllers', () {
    final source = _source(
      'lib/domains/commerce/catalog/auction/presentation/widgets/detail/auction_detail_header.dart',
    );

    expect(source, isNot(contains('VideoPlayerController')));
    expect(source, isNot(contains('Chewie')));
  });

  test('for sale detail screen uses MediaCarouselWidget for gallery', () {
    final source = _source(
      'lib/domains/commerce/catalog/for_sale/presentation/screens/for_sale_detail_screen.dart',
    );

    expect(source, contains('MediaCarouselWidget('));
    expect(source, isNot(contains('Image.network(')));
  });

  test('auction detail screen avoids raw Image.network in active detail UI', () {
    final source = _source(
      'lib/domains/commerce/catalog/auction/presentation/screens/auction_detail_screen.dart',
    );

    expect(source, isNot(contains('Image.network(')));
    expect(source, contains('AuctionDetailHeader('));
  });

  test('auction detail screen uses auction-specific handlers', () {
    final source = _source(
      'lib/domains/commerce/catalog/auction/presentation/screens/auction_detail_screen.dart',
    );
    expect(source, contains('AuctionDetailHandlers('));
    expect(source, contains('AuctionDetailBottomBar('));
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
