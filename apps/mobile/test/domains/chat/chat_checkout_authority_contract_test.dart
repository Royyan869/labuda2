// Chat checkout-authority contract — Owner rule: chat is a display layer.
//
// Chat must NEVER build a checkout route or resolve a physical product id:
//   - zero '/checkout/' route construction anywhere in the chat domain;
//   - the FOR-SALE path forwards to the commerce-owned intent
//     (openForSaleCheckout / CheckoutIntent);
//   - the AUCTION winner path forwards to the SAME shared Checkout through
//     the commerce-owned auction intent (openAuctionCheckout /
//     AuctionCheckoutIntent(bidWin: true)), carrying the quote + conversation
//     provenance.
//
// The obsolete claim flow (AuctionClaimShippingModal + claimAuction →
// POST /auctions/:id/claim) is PURGED: bid-win settlement is POST /orders
// via the shared Checkout. These contracts make any resurrection fail CI.

import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

List<File> _dartFiles(String dir) {
  final root = Directory(dir);
  if (!root.existsSync()) {
    // Thrown instead of expect(): this helper also runs in main() body,
    // outside any test zone.
    throw StateError('checkout-authority scan scope missing: $dir');
  }
  return root
      .listSync(recursive: true)
      .whereType<File>()
      .where((f) => f.path.endsWith('.dart'))
      .toList();
}

void main() {
  final chatFiles = _dartFiles('lib/domains/chat');
  final chatScreen = File(
    'lib/domains/chat/chat/presentation/screens/chat_detail_screen.dart',
  ).readAsStringSync();

  group('Chat never owns checkout identity (Owner rule)', () {
    test('scan scope is real (positive proof the scan covers code)', () {
      expect(chatFiles, isNotEmpty);
      expect(
        chatFiles.any(
          (f) => f.path.endsWith('chat_detail_screen.dart'),
        ),
        isTrue,
      );
    });

    test('chat builds zero checkout routes', () {
      final offenders = chatFiles
          .where((f) => f.readAsStringSync().contains("'/checkout/"))
          .map((f) => f.path)
          .toList();
      expect(
        offenders,
        isEmpty,
        reason:
            'Checkout route construction is commerce-owned — forward an '
            'intent instead of building the route.',
      );
    });

    test('the for-sale path forwards to the commerce checkout intent', () {
      expect(chatScreen, contains('openForSaleCheckout'));
      expect(chatScreen, contains('CheckoutIntent('));
      // The distinct-ID discipline lives in commerce: chat carries no productId.
      expect(chatScreen, isNot(contains('target.productId')));
    });

    test('the auction winner path forwards through the shared bid-win checkout', () {
      // ONE PURCHASE FUNNEL: chat resolves NOTHING — the commerce auction
      // intent resolves the live auction, product id and trust gate, and
      // builds the canonical checkout route with the bid-win discriminator.
      expect(chatScreen, contains('openAuctionCheckout'));
      expect(chatScreen, contains('AuctionCheckoutIntent('));
      expect(chatScreen, contains('bidWin: true'));
      // Quote + conversation provenance travel WITH the intent (the backend
      // consumes the quote via the ONE ShippingQuote authority inside order
      // creation).
      expect(chatScreen, contains('shippingQuoteId'));
      expect(chatScreen, contains('chatId'));
    });

    test('chat never resurrects the purged claim flow', () {
      expect(
        chatScreen,
        isNot(contains('AuctionClaimShippingModal')),
        reason: 'the claim modal was purged with the claim flow',
      );
      expect(
        chatScreen,
        isNot(contains('claimAuction')),
        reason: 'the claim RPC was purged with the claim flow',
      );
    });

    test('chat resolves no physical product id of its own', () {
      for (final file in chatFiles) {
        expect(
          file.readAsStringSync(),
          isNot(contains('resolveAuctionProductId')),
          reason: '${file.path} must not resolve auction product ids',
        );
      }
      expect(chatScreen, isNot(contains('target.productId')));
    });
  });
}
