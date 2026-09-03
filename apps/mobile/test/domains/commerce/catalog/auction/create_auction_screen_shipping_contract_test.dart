import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

/// PASS_18E: locks the create-auction screen's shipping-option contract —
/// backend requires `shipping_setup_ids` with at least one entry, and
/// there was previously no UI path to collect it at all (PASS_18D finding),
/// making mobile auction creation always fail with 400. Follows the same
/// source-text contract convention as the sibling timing contract test
/// (no existing widget-pump harness exists for this screen to extend).
void main() {
  test(
    'CreateAuctionScreen reuses SellerShippingSetupsSelector and validates a selection before submit',
    () {
      final source = File(
        'lib/domains/commerce/catalog/auction/presentation/screens/create_auction_screen.dart',
      ).readAsStringSync();

      // Reuses the existing reusable selector rather than a bespoke widget.
      expect(
        source,
        contains(
          "import 'package:labuda/domains/commerce/transaction/shipping/presentation/widgets/seller_shipping_options_selector.dart';",
        ),
      );
      expect(source, contains('SellerShippingSetupsSelector('));
      expect(source, contains('_selectedShippingSetupIds'));

      // Submit is blocked when nothing is selected.
      expect(source, contains('_selectedShippingSetupIds.isEmpty'));

      // Selected IDs actually reach the notifier call.
      expect(source, contains('shippingSetupIds: _selectedShippingSetupIds'));
    },
  );

  test(
    'CreateAuctionScreen has no edit-mode surface (PASS_21C: removed dead auctionToEdit branch)',
    () {
      final source = File(
        'lib/domains/commerce/catalog/auction/presentation/screens/create_auction_screen.dart',
      ).readAsStringSync();

      // auctionToEdit/_isEditMode were dead end-to-end (no route ever passed
      // an auction to edit, and no repository method had any other caller
      // either) and were removed outright rather than kept as an unreachable
      // fail-closed stub. Regression guard against silently reintroducing a
      // half-wired edit surface. (The class-level history comment above is
      // allowed to mention the removed name in prose.)
      expect(source, isNot(contains('final Auction? auctionToEdit')));
      expect(source, isNot(contains('widget.auctionToEdit')));
      expect(source, isNot(contains('bool get _isEditMode')));
    },
  );
}
