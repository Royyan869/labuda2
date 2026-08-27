import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

String _source(String relativePath) {
  return File(relativePath).readAsStringSync().replaceAll('\r\n', '\n');
}

void main() {
  test('public marketplace surfaces use the shared grid primitive', () {
    final paths = <String>[
      'lib/features/explore/presentation/widgets/explore_for_sale_tab.dart',
      'lib/features/explore/presentation/widgets/explore_auction_tab.dart',
      'lib/domains/user/preference/seller/presentation/widgets/profile_store_tab.dart',
      'lib/domains/commerce/catalog/for_sale/presentation/screens/for_sale_list_screen.dart',
    ];

    for (final path in paths) {
      final source = _source(path);
      expect(source, contains('CommerceMarketplaceGrid('), reason: path);
      expect(source, isNot(contains('ListView.builder(')), reason: path);
    }
  });

  test('home promo shelf uses the shared compact marketplace card shell', () {
    final source = _source(
      'lib/features/home/presentation/widgets/commerce_preview_section.dart',
    );

    expect(source, contains('CommerceMarketplaceCardShell('));
    expect(source, contains('CommerceMarketplaceCardMedia('));
    expect(source, contains('CommerceMarketplaceCardBadge('));
    expect(source, isNot(contains('Image.network(')));
  });

  test('dormant auction browse screen has been removed', () {
    expect(
      File(
        'lib/domains/commerce/catalog/auction/presentation/screens/auction_list_screen.dart',
      ).existsSync(),
      isFalse,
    );
  });
}
