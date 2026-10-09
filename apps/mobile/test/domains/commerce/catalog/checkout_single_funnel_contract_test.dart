// Checkout single-funnel contract — Owner rule: one door per surface, and it
// lives in commerce.
//
//   - for_sale → openForSaleCheckout
//     (lib/domains/commerce/catalog/for_sale/presentation/checkout_intent.dart)
//   - auction  → openAuctionCheckout
//     (lib/domains/commerce/catalog/auction/presentation/checkout_intent.dart)
//
// No screen may build a '/checkout/' route itself, and the for-sale detail CTA
// must keep forwarding the deal binding (viewer_negotiation_id) through the
// intent. A second builder, or a silently dropped binding, fails CI here.

import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

const _catalogDir = 'lib/domains/commerce/catalog';
const _forSaleDetail =
    'lib/domains/commerce/catalog/for_sale/presentation/screens/for_sale_detail_screen.dart';
const _auctionDetail =
    'lib/domains/commerce/catalog/auction/presentation/screens/auction_detail_screen.dart';

List<File> _dartFiles(String dir) {
  final root = Directory(dir);
  if (!root.existsSync()) {
    // Thrown instead of expect(): this helper also runs in main() body,
    // outside any test zone.
    throw StateError('single-funnel scan scope missing: $dir');
  }
  return root
      .listSync(recursive: true)
      .whereType<File>()
      .where((f) => f.path.endsWith('.dart'))
      .toList();
}

String _norm(String path) => path.replaceAll('\\', '/');

void main() {
  final catalogFiles = _dartFiles(_catalogDir);
  final forSaleDetail = File(_forSaleDetail).readAsStringSync();
  final auctionDetail = File(_auctionDetail).readAsStringSync();

  group('Checkout single funnel (commerce catalog)', () {
    test('scan scope is real (positive proof the scan covers code)', () {
      expect(catalogFiles, isNotEmpty);
      expect(
        catalogFiles.any((f) => _norm(f.path).endsWith('/for_sale/presentation/checkout_intent.dart')),
        isTrue,
      );
    });

    test('only the two commerce intents build checkout routes', () {
      final builders = catalogFiles
          .where((f) => f.readAsStringSync().contains("'/checkout/"))
          .map((f) => _norm(f.path))
          .toList()
        ..sort();

      expect(
        builders,
        [
          'lib/domains/commerce/catalog/auction/presentation/checkout_intent.dart',
          'lib/domains/commerce/catalog/for_sale/presentation/checkout_intent.dart',
        ],
        reason:
            'A checkout route may only be constructed by the commerce intent — '
            'screens forward an intent instead of building the route.',
      );
    });

    test('for-sale detail CTA forwards the deal binding through the intent', () {
      expect(forSaleDetail, contains('openForSaleCheckout('));
      expect(forSaleDetail, contains('CheckoutIntent('));
      // Deal price, not list: the binding must travel through the intent.
      expect(
        forSaleDetail,
        contains('negotiationId: forSale.viewerNegotiationId'),
      );
      expect(forSaleDetail, isNot(contains("'/checkout/")));
    });

    test('auction detail forwards BOTH purchase modes through the one intent', () {
      expect(auctionDetail, contains('openAuctionCheckout('));
      // Buy-now: active-auction purchase via the same shared checkout.
      expect(
        auctionDetail,
        contains('AuctionCheckoutIntent(auctionId: auction.id)'),
      );
      // Bid-win: the winner completing the settlement window — SAME checkout,
      // bid_win discriminator. No claim flow exists.
      expect(
        auctionDetail,
        contains('AuctionCheckoutIntent(auctionId: auction.id, bidWin: true)'),
      );
      expect(auctionDetail, isNot(contains("'/checkout/")));
      expect(auctionDetail, isNot(contains('AuctionClaimShippingModal')));
      expect(auctionDetail, isNot(contains('claimAuction')));
    });

    test('the claim design is dead across the whole catalog', () {
      final offenders = catalogFiles
          .where((f) {
            final src = f.readAsStringSync();
            return src.contains('AuctionClaimShippingModal') ||
                src.contains('claimAuction') ||
                src.contains('/auctions/') && src.contains('/claim');
          })
          .map((f) => _norm(f.path))
          .toList();
      expect(
        offenders,
        isEmpty,
        reason:
            'bid-win settlement is POST /orders via the shared Checkout — '
            'no claim modal, claim RPC or claim route may exist in the '
            'catalog domain.',
      );
    });
  });
}
