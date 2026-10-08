// Chat checkout-authority contract — Owner rule: chat is a display layer.
//
// Chat must NEVER build a checkout route or resolve a physical product id:
//   - zero '/checkout/' route construction anywhere in the chat domain;
//   - the FOR-SALE path forwards to the commerce-owned intent
//     (openForSaleCheckout / CheckoutIntent);
//   - the AUCTION winner path forwards to the canonical winner CLAIM flow
//     (AuctionClaimShippingModal + auctionNotifierProvider.claimAuction →
//     POST /auctions/:id/claim), carrying the quote + conversation provenance.
//
// The obsolete auction buy-now intent (openAuctionCheckout / AuctionCheckoutIntent)
// is NOT a chat entry point: it belongs to auction-detail buy-now only. Routing a
// chat auction through it is the architecture that was deleted — these contracts
// make its resurrection fail CI.

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
            'intent/claim instead of building the route.',
      );
    });

    test('the for-sale path forwards to the commerce checkout intent', () {
      expect(chatScreen, contains('openForSaleCheckout'));
      expect(chatScreen, contains('CheckoutIntent('));
      // The distinct-ID discipline lives in commerce: chat carries no productId.
      expect(chatScreen, isNot(contains('target.productId')));
    });

    test('the auction winner path forwards through the canonical claim flow', () {
      // Canonical flow: resolve the auction from Commerce, seed the
      // Commerce-owned claim modal, and forward the explicit claim intent.
      expect(chatScreen, contains('auctionDetailProvider'));
      expect(chatScreen, contains('AuctionClaimShippingModal.show'));
      expect(chatScreen, contains('auctionNotifierProvider'));
      expect(chatScreen, contains('.claimAuction('));
      // Quote + conversation provenance travel WITH the claim (the backend
      // consumes the quote via the ONE ShippingQuote authority).
      expect(chatScreen, contains('shippingQuoteId'));
      expect(chatScreen, contains('chatId'));
    });

    test('chat never routes auctions through the obsolete buy-now intent', () {
      expect(
        chatScreen,
        isNot(contains('openAuctionCheckout')),
        reason:
            'Auction buy-now (openAuctionCheckout) is auction-detail only. The '
            'chat auction winner path is the canonical claim flow — a chat '
            'route to the buy-now intent is the deleted architecture.',
      );
      expect(
        chatScreen,
        isNot(contains('AuctionCheckoutIntent')),
        reason: 'obsolete chat auction buy-now intent',
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
