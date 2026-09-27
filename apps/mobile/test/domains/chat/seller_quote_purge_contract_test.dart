// Seller-quote purge contract — Owner decision 2026-09-26.
//
// The chat seller-quote feature (special price offer from seller to buyer,
// Pass 1D-F1 era) was deleted TO ROOT — not just the UI:
//   - composer props (onSendQuote / onStartNegotiation / onBuyNow),
//   - chat_seller_quote_cta_test.dart,
//   - checkout's QUOTE_UNAVAILABLE / 'seller quote' runtime branch,
//   - CheckoutHonestyMessages.quoteUnavailable*,
//   - every doc/comment reference in chat + checkout.
//
// What shares the name but is NOT this feature and must stay alive:
//   - auction settlement seller quote in the backend (SellerQuoteProvided),
//   - negotiation ("Kirim Tawaran" dialog — nego authority, Owner O4),
//   - shipping quote (ongkir — separate feature).
//
// These negative contracts make resurrection fail CI: any future agent that
// reintroduces seller-quote symbols in chat or checkout breaks this test.
// (CTA nego / "Beli Sekarang" contracts are intentionally NOT part of this
// file — they await the Owner's separate card-CTA decision.)

import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

/// Seller-quote symbols that must never reappear in chat or checkout code.
/// Comments are included on purpose: doc rot is how dead features come back.
final RegExp _forbidden = RegExp(
  r'onSendQuote|onStartNegotiation|onBuyNow|seller[\s_]?quote|quoteUnavailable|QUOTE_UNAVAILABLE',
  caseSensitive: false,
);

List<File> _dartFiles(String dir) {
  final root = Directory(dir);
  if (!root.existsSync()) {
    // Thrown instead of expect(): this helper also runs in main() body,
    // outside any test zone.
    throw StateError('seller-quote scan scope missing: $dir');
  }
  return root
      .listSync(recursive: true)
      .whereType<File>()
      .where((f) => f.path.endsWith('.dart'))
      .toList();
}

List<String> _violations(List<File> files) {
  final hits = <String>[];
  for (final file in files) {
    final lines = file.readAsStringSync().split('\n');
    for (var i = 0; i < lines.length; i++) {
      if (_forbidden.hasMatch(lines[i])) {
        hits.add('${file.path}:${i + 1}: ${lines[i].trim()}');
      }
    }
  }
  return hits;
}

void main() {
  final chatLibFiles = _dartFiles('lib/domains/chat');
  final checkoutLibFiles =
      _dartFiles('lib/domains/commerce/transaction/checkout');

  group('Seller quote purged to root (Owner 2026-09-26)', () {
    test('scan scope is real (positive proof the scan covers code)', () {
      expect(chatLibFiles, isNotEmpty);
      expect(checkoutLibFiles, isNotEmpty);
      // The composer must be inside the scanned scope.
      expect(
        chatLibFiles.any((f) => f.path.endsWith('chat_input_area.dart')),
        isTrue,
      );
    });

    test('chat domain contains zero seller-quote symbols', () {
      expect(
        _violations(chatLibFiles),
        isEmpty,
        reason:
            'Seller quote is deleted to root — do not reintroduce composer '
            'quote/buy/nego CTA props or seller-quote code in the chat domain.',
      );
    });

    test('checkout contains zero seller-quote symbols', () {
      expect(
        _violations(checkoutLibFiles),
        isEmpty,
        reason:
            "Checkout's seller-quote branch was dead (backend never emits "
            'QUOTE_UNAVAILABLE / "seller quote") and was purged — do not '
            'reintroduce it.',
      );
    });
  });

  group('Neighbors that share the name but stay alive', () {
    test('negotiation offer dialog (Kirim Tawaran) still exists — nego is '
        'NOT seller quote', () {
      final screen = File(
        'lib/domains/chat/chat/presentation/screens/chat_detail_screen.dart',
      ).readAsStringSync();
      expect(screen, contains('Kirim Tawaran'));
    });

    test('shipping quote (ongkir) still exists — separate feature', () {
      final repo = File(
        'lib/domains/chat/chat/data/repositories/chat_repository_impl.dart',
      ).readAsStringSync();
      expect(repo, contains('createShippingQuote'));
    });
  });
}
