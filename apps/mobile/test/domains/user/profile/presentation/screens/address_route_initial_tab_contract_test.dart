import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

void main() {
  group('Canonical address route initial-tab contract', () {
    test('Shipping Options opens Sender Address tab', () {
      final source = File(
        'lib/domains/user/preference/seller/presentation/screens/seller_shipping_screen.dart',
      ).readAsStringSync();

      expect(
        source,
        contains('RoutePaths.addressesWithInitialTab(AddressInitialTab.sender)'),
      );
    });

    test('Create Listing opens Sender Address tab', () {
      final source = File(
        'lib/domains/commerce/catalog/listing/presentation/screens/create_listing_screen.dart',
      ).readAsStringSync();

      expect(
        source,
        contains('RoutePaths.addressesWithInitialTab(AddressInitialTab.sender)'),
      );
    });

    test('Create Auction opens Sender Address tab', () {
      final source = File(
        'lib/domains/commerce/catalog/auction/presentation/screens/create_auction_screen.dart',
      ).readAsStringSync();

      expect(
        source,
        contains('RoutePaths.addressesWithInitialTab(AddressInitialTab.sender)'),
      );
    });
  });
}
