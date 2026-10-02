// Chat checkout-authority contract — Owner rule: chat is a display layer.
//
// Chat must NEVER resolve commerce identity or build a checkout route:
//   - zero '/checkout/' route construction in the chat domain,
//   - zero checkout identity resolution (auctionDetailProvider /
//     resolveAuctionProductId / manual product-id plumbing),
//   - every surface forwards to the commerce-owned intent:
//     openForSaleCheckout (for_sale) and openAuctionCheckout (auction).
//
// These negative contracts make resurrection fail CI: any future change that
// reintroduces route building or product-id resolution in chat breaks here.

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
            'intent (openForSaleCheckout / openAuctionCheckout) instead.',
      );
    });

    test('chat resolves zero commerce checkout identity', () {
      final offenders = chatFiles
          .where(
            (f) =>
                f.readAsStringSync().contains('auctionDetailProvider') ||
                f.readAsStringSync().contains('resolveAuctionProductId'),
          )
          .map((f) => f.path)
          .toList();
      expect(
        offenders,
        isEmpty,
        reason:
            'Product id / listing resolution belongs to the commerce intent, '
            'not to chat.',
      );
    });

    test('both surfaces forward to a commerce-owned intent', () {
      expect(chatScreen, contains('openForSaleCheckout'));
      expect(chatScreen, contains('openAuctionCheckout'));
      expect(chatScreen, contains('AuctionCheckoutIntent'));
      // The distinct-ID discipline lives in commerce now: the product id is
      // resolved there, so chat carries no productId field on its target.
      expect(chatScreen, isNot(contains('target.productId')));
    });
  });
}
