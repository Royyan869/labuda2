import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

/// PASS_21B: audits whether the live "Buy Now" button on the auction detail
/// screen reaches `AuctionRepository.buyNow()` — a datasource stub that
/// always throws `UnsupportedError` (locked in by
/// auction_contract_p1_test.dart's "unsupported endpoints" test). It does
/// not: `_handleBuyNow` routes to the generic checkout screen instead. This
/// test locks that routing in as a regression guard — if a future change
/// wires the live button back to the unsupported repository method, this
/// test fails loudly instead of shipping a guaranteed-broken buy-now button.
void main() {
  test(
    'AuctionDetailScreen buy-now handler routes to checkout, not the unsupported AuctionRepository.buyNow() stub',
    () {
      final source = File(
        'lib/domains/commerce/catalog/auction/presentation/screens/auction_detail_screen.dart',
      ).readAsStringSync().replaceAll('\r\n', '\n');

      // Live path: navigates to the generic checkout screen with
      // source_type=auction context, same as any other order-creation flow.
      expect(source, contains("context.push(\n      '/checkout/"));
      expect(source, contains('auction_id='));

      // Must NOT call the repository/notifier method backed by the
      // always-throwing datasource stub.
      expect(source, isNot(contains('.buyNow(')));
      expect(
        source,
        isNot(contains('auctionNotifierProvider.notifier).buyNow')),
      );
    },
  );
}
