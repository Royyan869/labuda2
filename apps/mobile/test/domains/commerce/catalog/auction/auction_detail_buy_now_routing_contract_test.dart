import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

/// PASS_21B + single-funnel contract: the live "Buy Now" button on the auction
/// detail screen must NOT call `AuctionRepository.buyNow()` — a datasource
/// stub that always throws `UnsupportedError` (locked in by
/// auction_contract_p1_test.dart's "unsupported endpoints" test). It also must
/// not build the checkout route itself: checkout opens through the
/// commerce-owned intent (openAuctionCheckout), so product-id resolution, the
/// seller trust gate and the route shape live in exactly one place.
void main() {
  final source = File(
    'lib/domains/commerce/catalog/auction/presentation/screens/auction_detail_screen.dart',
  ).readAsStringSync().replaceAll('\r\n', '\n');

  test(
    'buy-now delegates to the commerce checkout intent (no second builder)',
    () {
      // Live path: forwards an intent to the generic checkout screen with
      // source_type=auction context — the intent owns the route shape.
      expect(source, contains('openAuctionCheckout('));
      expect(source, contains('AuctionCheckoutIntent('));
      expect(source, isNot(contains("'/checkout/")));
    },
  );

  test(
    'buy-now is not wired to the unsupported AuctionRepository.buyNow() stub',
    () {
      expect(source, isNot(contains('.buyNow(')));
      expect(
        source,
        isNot(contains('auctionNotifierProvider.notifier).buyNow')),
      );
    },
  );
}
