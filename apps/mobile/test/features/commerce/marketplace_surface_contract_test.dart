import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

String _source(String relativePath) {
  return File(relativePath).readAsStringSync().replaceAll('\r\n', '\n');
}

/// CANONICAL DESIGN (owner-locked):
///
/// 1. Every PUBLIC commerce discovery surface renders through the shared
///    `CommerceMarketplaceGrid` (2 columns, e-commerce layout) — a surface may
///    not own its own list widget.
/// 2. Every public commerce card is a `CommerceMarketplaceCardShell` wrapper —
///    a channel may fill slots, never re-implement the frame.
/// 3. The dormant auction browse screen (placeholder cards, English copy,
///    unrouted) is deleted, not left lying around to be resurrected.
const _publicSurfaces = <String>[
  'lib/features/marketplace/presentation/widgets/marketplace_for_sale_tab.dart',
  'lib/features/marketplace/presentation/widgets/marketplace_auction_tab.dart',
  'lib/domains/user/preference/seller/presentation/widgets/profile_store_tab.dart',
  'lib/domains/commerce/catalog/for_sale/presentation/screens/for_sale_list_screen.dart',
];

const _publicCards = <String>[
  'lib/domains/commerce/catalog/for_sale/presentation/widgets/for_sale_card.dart',
  'lib/domains/commerce/catalog/auction/presentation/widgets/auction_card.dart',
];

void main() {
  test('public marketplace surfaces use the shared grid primitive', () {
    for (final path in _publicSurfaces) {
      final source = _source(path);
      expect(source, contains('CommerceMarketplaceGrid('), reason: path);
      expect(source, isNot(contains('ListView.builder(')), reason: path);
      expect(source, isNot(contains('SliverList(')), reason: path);
    }
  });

  test('public commerce cards wrap the shared card shell', () {
    for (final path in _publicCards) {
      final source = _source(path);
      expect(source, contains('CommerceMarketplaceCardShell('), reason: path);
      expect(
        source,
        contains('CommerceCardSellerMetadata('),
        reason: '$path must use the shared seller block',
      );
      // The card may not own its frame or its media badge stack.
      expect(source, isNot(contains('return Card(')), reason: path);
      expect(source, isNot(contains('Positioned(')), reason: path);
    }
  });

  test('home promo shelf has been purged (no Sedang Laku shelf)', () {
    expect(
      File(
        'lib/features/home/presentation/widgets/commerce_preview_section.dart',
      ).existsSync(),
      isFalse,
      reason:
          'CommercePreviewSection must be purged — Home is social-only until promotion is active',
    );
  });

  test('dormant auction browse screen has been removed', () {
    expect(
      File(
        'lib/domains/commerce/catalog/auction/presentation/screens/auction_list_screen.dart',
      ).existsSync(),
      isFalse,
      reason:
          'Unrouted placeholder browse surface — auction discovery lives in MarketplaceAuctionTab',
    );
  });
}
